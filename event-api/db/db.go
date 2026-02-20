package db

import (
	"database/sql"
	"errors"
	"event-api/models"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB initializes the SQLite database and creates necessary tables
func InitDB(dataSourceName string) error {
	var err error
	DB, err = sql.Open("sqlite", dataSourceName)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	return createTables()
}

func createTables() error {
	createEventsTable := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT,
		capacity INTEGER NOT NULL,
		available_spots INTEGER NOT NULL,
		date DATETIME NOT NULL
	);`

	createRegistrationsTable := `
	CREATE TABLE IF NOT EXISTS registrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id INTEGER NOT NULL,
		user_name TEXT NOT NULL,
		user_email TEXT NOT NULL,
		registered_date DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(event_id) REFERENCES events(id)
	);`

	_, err := DB.Exec(createEventsTable)
	if err != nil {
		return fmt.Errorf("could not create events table: %v", err)
	}

	_, err = DB.Exec(createRegistrationsTable)
	if err != nil {
		return fmt.Errorf("could not create registrations table: %v", err)
	}

	return nil
}

// CreateEvent inserts a new event into the database
func CreateEvent(e models.Event) (int64, error) {
	stmt, err := DB.Prepare("INSERT INTO events(title, description, capacity, available_spots, date) VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(e.Title, e.Description, e.Capacity, e.Capacity, e.Date)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// GetEvents retrieves all events
func GetEvents() ([]models.Event, error) {
	rows, err := DB.Query("SELECT id, title, description, capacity, available_spots, date FROM events")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Capacity, &e.AvailableSpots, &e.Date); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// RegisterUser handles the concurrent registration logic using atomic updates.
func RegisterUser(registration models.Registration) error {
	// Optimization: Start a transaction
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	// The Critical Concurrency Step: Optimistic Concurrency Control using Atomic DB Update.
	// This ensures that even if 1000 requests happen simultaneously exactly here,
	// only the ones where available_spots > 0 will succeed.
	res, err := tx.Exec("UPDATE events SET available_spots = available_spots - 1 WHERE id = ? AND available_spots > 0", registration.EventID)
	if err != nil {
		tx.Rollback()
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}

	// If no rows were affected, the event is either sold out or doesn't exist.
	if rowsAffected == 0 {
		tx.Rollback()
		return errors.New("event is sold out or does not exist")
	}

	// Insert the registration record
	stmt, err := tx.Prepare("INSERT INTO registrations(event_id, user_name, user_email) VALUES(?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(registration.EventID, registration.UserName, registration.UserEmail)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	log.Printf("Successfully registered user %s for event %d\n", registration.UserEmail, registration.EventID)
	return nil
}
