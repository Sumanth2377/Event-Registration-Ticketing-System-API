package tests

import (
	"database/sql"
	"event-api/db"
	"event-api/models"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestConcurrentRegistration simulates 100 users trying to register for an event with only 5 spots.
func TestConcurrentRegistration(t *testing.T) {
	// Initialize a temporary in-memory database for testing
	err := db.InitDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.DB.Close()

	// 1. Create an event with exactly 5 capacity
	event := models.Event{
		Title:          "Golang Concurrency Workshop",
		Description:    "Learn how to handle race conditions",
		Capacity:       5, // Only 5 spots available
		AvailableSpots: 5,
		Date:           time.Now().Add(48 * time.Hour),
	}

	eventID, err := db.CreateEvent(event)
	if err != nil {
		t.Fatalf("Failed to create test event: %v", err)
	}

	// 2. Prepare 100 concurrent workers
	concurrencyLevel := 100
	var wg sync.WaitGroup

	successCount := 0
	failureCount := 0
	var mu sync.Mutex // Mutex just to safely increment counters in the test

	log.Printf("Starting %d concurrent registration requests for Event ID %d with 5 spots...", concurrencyLevel, eventID)

	// Launch 100 goroutines simultaneously
	for i := 0; i < concurrencyLevel; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			reg := models.Registration{
				EventID:   int(eventID),
				UserName:  fmt.Sprintf("User %d", userID),
				UserEmail: fmt.Sprintf("user%d@example.com", userID),
			}

			// Core test: Call the RegisterUser function which contains our atomic DB update
			err := db.RegisterUser(reg)

			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failureCount++
			}
			mu.Unlock()

		}(i)
	}

	// Wait for all requests to finish
	wg.Wait()

	log.Printf("Test Completed. Successes: %d, Failures: %d", successCount, failureCount)

	// 3. Assertions
	if successCount != 5 {
		t.Errorf("Expected exactly 5 successful registrations, got %d", successCount)
	}

	if failureCount != 95 {
		t.Errorf("Expected exactly 95 failed registrations, got %d", failureCount)
	}

	// Verify the database state
	var availableSpots int
	err = db.DB.QueryRow("SELECT available_spots FROM events WHERE id = ?", eventID).Scan(&availableSpots)
	if err != nil {
		t.Fatalf("Failed to query event: %v", err)
	}

	if availableSpots != 0 {
		t.Errorf("Expected available_spots to be 0, got %d", availableSpots)
	}

	// Verify exactly 5 registrations were inserted
	var registrationCount int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM registrations WHERE event_id = ?", eventID).Scan(&registrationCount)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Failed to count registrations: %v", err)
	}

	if registrationCount != 5 {
		t.Errorf("Expected exactly 5 registration records, got %d", registrationCount)
	}
}
