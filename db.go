package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB represents our database layer
type DB struct {
	*sql.DB
}

// NewDB initializes and connects to the SQLite database
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Important settings for SQLite concurrency.
	// We want to avoid "database is locked" errors during high concurrent writes.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// InitSchema sets up the required tables
func (db *DB) InitSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		total_spots INTEGER NOT NULL,
		available_spots INTEGER NOT NULL,
		CHECK (available_spots >= 0)
	);

	CREATE TABLE IF NOT EXISTS tickets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id INTEGER NOT NULL,
		user_email TEXT NOT NULL,
		idempotency_key TEXT UNIQUE NOT NULL,
		status TEXT DEFAULT 'reserved' CHECK (status IN ('reserved', 'confirmed', 'cancelled')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (event_id) REFERENCES events(id),
		UNIQUE(event_id, user_email)
	);
	`
	_, err := db.ExecContext(ctx, schema)
	return err
}

// Event represents an event record
type Event struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	TotalSpots     int    `json:"total_spots"`
	AvailableSpots int    `json:"available_spots"`
}

// CreateEvent creates a new event
func (db *DB) CreateEvent(ctx context.Context, name string, totalSpots int) (*Event, error) {
	query := `INSERT INTO events (name, total_spots, available_spots) VALUES (?, ?, ?)`
	res, err := db.ExecContext(ctx, query, name, totalSpots, totalSpots)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Event{
		ID:             id,
		Name:           name,
		TotalSpots:     totalSpots,
		AvailableSpots: totalSpots,
	}, nil
}

// ListEvents lists all events
func (db *DB) ListEvents(ctx context.Context) ([]Event, error) {
	query := `SELECT id, name, total_spots, available_spots FROM events`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Name, &e.TotalSpots, &e.AvailableSpots); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

var ErrSoldOut = errors.New("event is sold out")
var ErrAlreadyRegistered = errors.New("user already registered for this event or request already processed")

// RegisterForEvent uses an atomic conditional update inside a transaction to prevent overbooking
func (db *DB) RegisterForEvent(ctx context.Context, eventID int64, email string, idempotencyKey string) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback() // Safe to call even if committed

	// 1. Optimistic Concurrent Update (The Atomic Edge)
	res, err := tx.ExecContext(ctx, `
		UPDATE events 
		SET available_spots = available_spots - 1 
		WHERE id = ? AND available_spots > 0
	`, eventID)

	if err != nil {
		return 0, fmt.Errorf("failed to update event capacity: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return 0, ErrSoldOut
	}

	// 2. Insert Ticket with 5-minute expiry
	// Use SQLite specific datetime modification
	res, err = tx.ExecContext(ctx, `
		INSERT INTO tickets (event_id, user_email, idempotency_key, status, expires_at) 
		VALUES (?, ?, ?, 'reserved', datetime('now', '+5 minutes'))
	`, eventID, email, idempotencyKey)

	if err != nil {
		// Could be a UNIQUE constraint violation (double booking or duplicate idempotency key)
		return 0, fmt.Errorf("%w: %v", ErrAlreadyRegistered, err)
	}

	ticketID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed getting ticket id: %w", err)
	}

	// 3. Commit Transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit tx: %w", err)
	}

	return ticketID, nil
}

// ConfirmReservation finalizes the ticket.
func (db *DB) ConfirmReservation(ctx context.Context, ticketID int64, userEmail string) error {
	// Only allow confirming if status is 'reserved' and it hasn't expired
	res, err := db.ExecContext(ctx, `
		UPDATE tickets 
		SET status = 'confirmed' 
		WHERE id = ? AND user_email = ? AND status = 'reserved' AND expires_at > datetime('now')
	`, ticketID, userEmail)

	if err != nil {
		return fmt.Errorf("failed to confirm ticket: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("ticket is expired, already confirmed, or does not exist")
	}

	return nil
}

// ReclaimExpiredSeats acts as the background worker reclaiming spots
func (db *DB) ReclaimExpiredSeats(ctx context.Context) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// 1. Find expired but still reserved tickets
	// SQLite syntax to update status to cancelled and return event_ids for atomic replenishment
	// We do this via two steps in SQLite because it lacks UPDATE ... RETURNING out of the box until newer versions.

	rows, err := tx.QueryContext(ctx, `SELECT id, event_id FROM tickets WHERE status = 'reserved' AND expires_at <= datetime('now')`)
	if err != nil {
		return 0, err
	}

	type reclaimed struct {
		ticketID int64
		eventID  int64
	}
	var expired []reclaimed
	for rows.Next() {
		var r reclaimed
		if err := rows.Scan(&r.ticketID, &r.eventID); err == nil {
			expired = append(expired, r)
		}
	}
	rows.Close()

	if len(expired) == 0 {
		return 0, tx.Commit()
	}

	// 2. Mark as Cancelled and Return spot to events table
	var reclaimedCount int64
	for _, e := range expired {
		_, err := tx.ExecContext(ctx, `UPDATE tickets SET status = 'cancelled' WHERE id = ?`, e.ticketID)
		if err != nil {
			continue
		}

		_, err = tx.ExecContext(ctx, `UPDATE events SET available_spots = available_spots + 1 WHERE id = ?`, e.eventID)
		if err != nil {
			continue // In reality we'd log this critical error
		}
		reclaimedCount++
	}

	return reclaimedCount, tx.Commit()
}
