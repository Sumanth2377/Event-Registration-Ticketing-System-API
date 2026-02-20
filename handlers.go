package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Handlers struct {
	DB *DB
}

// SendJSON is a helper for sending JSON responses
func SendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error in real app, but for now we just return
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Request/Response DTOs
type CreateEventRequest struct {
	Name       string `json:"name"`
	TotalSpots int    `json:"total_spots"`
}

type RegisterRequest struct {
	Email          string `json:"email"`
	IdempotencyKey string `json:"idempotency_key"`
}

// HandleCreateEvent handles POST /events
func (h *Handlers) HandleCreateEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method Not Allowed"})
		return
	}

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	if req.Name == "" || req.TotalSpots <= 0 {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid name or total_spots"})
		return
	}

	evt, err := h.DB.CreateEvent(r.Context(), req.Name, req.TotalSpots)
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	SendJSON(w, http.StatusCreated, evt)
}

// HandleListEvents handles GET /events
func (h *Handlers) HandleListEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method Not Allowed"})
		return
	}

	events, err := h.DB.ListEvents(r.Context())
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Returning an empty array instead of null if no events
	if events == nil {
		events = []Event{}
	}

	SendJSON(w, http.StatusOK, events)
}

// HandleRegister handles POST /events/{id}/register
func (h *Handlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method Not Allowed"})
		return
	}

	// Extract {id} manually since we are using Go 1.22's exact match or manual parsing.
	// Go 1.22 NewServeMux handles wildcard routes: "POST /events/{id}/register"
	idStr := r.PathValue("id")
	if idStr == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing event ID"})
		return
	}
	eventID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid event ID format"})
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	if req.Email == "" || req.IdempotencyKey == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Email and idempotency_key are required"})
		return
	}

	ticketID, err := h.DB.RegisterForEvent(r.Context(), eventID, req.Email, req.IdempotencyKey)
	if err != nil {
		if errors.Is(err, ErrSoldOut) {
			SendJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrAlreadyRegistered) {
			SendJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}

		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error during registration"})
		return
	}

	SendJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "Seat reserved! Please confirm within 5 minutes.",
		"ticket_id": ticketID,
	})
}

// HandleConfirm handles POST /tickets/{id}/confirm
func (h *Handlers) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method Not Allowed"})
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing ticket ID"})
		return
	}
	ticketID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid ticket ID format"})
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	if req.Email == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Email is required to confirm"})
		return
	}

	err = h.DB.ConfirmReservation(r.Context(), ticketID, req.Email)
	if err != nil {
		SendJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	SendJSON(w, http.StatusOK, map[string]string{"message": "Ticket successfully confirmed"})
}
