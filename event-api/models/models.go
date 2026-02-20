package models

import "time"

// Event represents an event that users can register for.
type Event struct {
	ID             int       `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Capacity       int       `json:"capacity"`
	AvailableSpots int       `json:"available_spots"`
	Date           time.Time `json:"date"`
}

// Registration represents a user's booking for an event.
type Registration struct {
	ID             int       `json:"id"`
	EventID        int       `json:"event_id"`
	UserName       string    `json:"user_name"`
	UserEmail      string    `json:"user_email"`
	RegisteredDate time.Time `json:"registered_date"`
}
