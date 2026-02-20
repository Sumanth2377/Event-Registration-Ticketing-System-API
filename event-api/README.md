# Event Registration & Ticketing API

This repository contains the solution for **Capstone Project 5**: Event Registration & Ticketing System API. 
The API is built using Go (Golang) and uses SQLite for zero-configuration, self-contained persistence.

## Features & Highlights
- **Zero-Dependency Core**: The API routing uses the newly introduced Go 1.22 `ServeMux` features (`POST /path`, `r.PathValue`).
- **Pure Go SQLite**: Uses `modernc.org/sqlite` instead of `mattn/go-sqlite3` so you don't need CGO or a C compiler (like GCC/MinGW) installed on your system. It works out of the box on Windows, Mac, and Linux!
- **Concurrency Bulletproof**: Handled the critical constraint of concurrent bookings to prevent overbooking using Optimistic Concurrency Control (Atomic DB Updates).

## Requirements
- Go 1.22 or higher

## Setup & Running
1. Clone the repository and navigate to the directory.
2. Download dependencies:
   ```bash
   go mod download
   ```
3. Run the application:
   ```bash
   go run main.go
   ```
   The server will start on `http://localhost:8080`. The database `events.db` will be auto-created in the root directory.

## Running the Concurrency Test
We have a dedicated test that actively tries to break the system by launching **100 concurrent users** trying to book an event that only has **5 spots**.
To run the test and verify that exactly 5 succeed and 95 fail gracefully without race conditions:
```bash
go test -v ./...
```

## API Documentation

### 1. Create an Event
- **Endpoint:** `POST /events`
- **Request Body:**
  ```json
  {
      "title": "Golang Concurrency Workshop",
      "description": "Learn how to handle race conditions",
      "capacity": 50,
      "date": "2026-12-01T10:00:00Z"
  }
  ```

### 2. Browse Events
- **Endpoint:** `GET /events`
- **Response:**
  ```json
  [
      {
          "id": 1,
          "title": "Golang Concurrency Workshop",
          "description": "Learn how to handle race conditions",
          "capacity": 50,
          "available_spots": 50,
          "date": "2026-12-01T10:00:00Z"
      }
  ]
  ```

### 3. Register for an Event
- **Endpoint:** `POST /events/{id}/register`
- **Request Body:**
  ```json
  {
      "user_name": "John Doe",
      "user_email": "john@example.com"
  }
  ```
- **Responses:**
  - `201 Created`: Successfully registered.
  - `409 Conflict`: Event is sold out or does not exist.
