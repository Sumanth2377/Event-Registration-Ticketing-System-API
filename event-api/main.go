package main

import (
	"event-api/db"
	"event-api/handlers"
	"log"
	"net/http"
)

func main() {
	log.Println("Initializing database...")
	err := db.InitDB("events.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /events", handlers.CreateEvent)
	mux.HandleFunc("GET /events", handlers.GetEvents)
	mux.HandleFunc("POST /events/{id}/register", handlers.RegisterForEvent)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
