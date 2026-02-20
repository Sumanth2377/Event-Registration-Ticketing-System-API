package handlers

import (
	"encoding/json"
	"event-api/db"
	"event-api/models"
	"net/http"
	"strconv"
)

// CreateEvent handles POST /events
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	id, err := db.CreateEvent(event)
	if err != nil {
		http.Error(w, "Failed to create event", http.StatusInternalServerError)
		return
	}

	event.ID = int(id)
	event.AvailableSpots = event.Capacity

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

// GetEvents handles GET /events
func GetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	events, err := db.GetEvents()
	if err != nil {
		http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}

	// Make sure we don't return null for an empty array
	if events == nil {
		events = []models.Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// RegisterForEvent handles POST /events/{id}/register
// Now using Go 1.22 ServeMux so we can access PathValue.
func RegisterForEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	eventIDStr := r.PathValue("id")
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		http.Error(w, "Invalid event ID format", http.StatusBadRequest)
		return
	}

	var reg models.Registration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	reg.EventID = eventID

	// Attempt consistent registration via atomic update
	err = db.RegisterUser(reg)
	if err != nil {
		// Differentiate between sold out and other errors
		if err.Error() == "event is sold out or does not exist" {
			http.Error(w, err.Error(), http.StatusConflict) // 409 Conflict
		} else {
			http.Error(w, "Failed to register for event: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Successfully registered!",
	})
}
