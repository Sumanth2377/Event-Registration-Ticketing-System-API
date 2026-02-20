package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestOptimisticConcurrency(t *testing.T) {
	// Create a temporary database for testing
	t.Log("Setting up temporary SQLite database...")
	dbPath := "test_concurrent.db"
	_ = os.Remove(dbPath) // Ensure clean slate
	defer os.Remove(dbPath)

	db, err := NewDB(fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbPath))
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.InitSchema(ctx); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	// 1. Create an event with exactly 5 capacity
	totalCapacity := 5
	event, err := db.CreateEvent(ctx, "The Big GopherCon", totalCapacity)
	if err != nil {
		t.Fatalf("Failed to create test event: %v", err)
	}

	t.Logf("Created Event ID: %d with Capacity: %d", event.ID, event.TotalSpots)

	// 2. Launch 100 Goroutines to fight for the 5 spots!
	numRequests := 100
	var successCount int32
	var soldOutCount int32
	var errorCount int32

	var wg sync.WaitGroup
	wg.Add(numRequests)

	t.Logf("Firing %d concurrent registration requests for %d spots...", numRequests, totalCapacity)

	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			defer wg.Done()

			email := fmt.Sprintf("gopher%d@example.com", requestID)
			idempotencyKey := fmt.Sprintf("key_%d", requestID)

			_, err := db.RegisterForEvent(ctx, event.ID, email, idempotencyKey)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, ErrSoldOut) {
				atomic.AddInt32(&soldOutCount, 1)
			} else {
				t.Logf("Unexpected error for request %d: %v", requestID, err)
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	// Wait for all 100 Goroutines to finish
	wg.Wait()

	t.Logf("Results -> Successes: %d | Sold Out: %d | Errors: %d", successCount, soldOutCount, errorCount)

	// 3. Verify exactly 5 succeeded and exactly 95 got "Sold Out"
	if successCount != int32(totalCapacity) {
		t.Errorf("Expected exactly %d successes, but got %d", totalCapacity, successCount)
	}
	if soldOutCount != int32(numRequests-totalCapacity) {
		t.Errorf("Expected exactly %d sold out errors, but got %d", numRequests-totalCapacity, soldOutCount)
	}
	if errorCount != 0 {
		t.Errorf("Expected 0 unexpected errors, but got %d", errorCount)
	}

	// 4. Double check the database records directly
	events, err := db.ListEvents(ctx)
	if err != nil {
		t.Fatalf("Failed to list events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, found %d", len(events))
	}

	if events[0].AvailableSpots != 0 {
		t.Errorf("Expected 0 available spots remaining in DB, but got %d", events[0].AvailableSpots)
	}

	// 5. Query total tickets sold
	var totalTicketsSold int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tickets WHERE event_id = ?", event.ID).Scan(&totalTicketsSold)
	if err != nil {
		t.Fatalf("Failed to count tickets: %v", err)
	}

	if totalTicketsSold != totalCapacity {
		t.Errorf("Expected exactly %d ticket rows in DB, but got %d", totalCapacity, totalTicketsSold)
	}

	t.Log("✅ Concurrency Test Passed! Zero Race Conditions. Database Atomicity Verified.")
}
