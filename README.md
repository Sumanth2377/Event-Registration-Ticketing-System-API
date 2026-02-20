# Event Registration & Ticketing API

A production-grade, highly-concurrent REST API for event registration and ticketing built strictly with **Go 1.22+ Standard Libraries** (`net/http`) and a portable **SQLite** database.

This project deliberately avoids massive web frameworks (like Gin or Fiber) and in-memory synchronization primitives (like `sync.Mutex`) to demonstrate deep fluency with Go's language primitives and database atomicity.

## Core Architectural Decisions

### 1. The Atomic Edge (Concurrency Strategy)
The core requirement of a ticketing system is preventing overbooking during highly concurrent registration events. 
Instead of relying on application-level locks that fail horizontally, we pushed the "Heavy Lifting" to the SQL layer via **Optimistic Concurrency Control**.

Seat decrementing and Ticket creation occur inside a strict ACID `sql.Tx` transaction utilizing a single atomic update:
```sql
UPDATE events SET available_spots = available_spots - 1 WHERE id = ? AND available_spots > 0;
```
This mathematically guarantees two simultaneous requests cannot both grab the final seat. Database `CHECK (available_spots >= 0)` constraints provide a secondary rigid defense against negative capacity.

### 2. The "Perfect Trio" of Business Maturity
To elevate this project from an "API" to a real-world "Commerce Platform", the following production features were implemented:

1. **Seat Reservation with Expiry**: Utilizing an intelligent state-machine in the `tickets` table (`reserved` -> `confirmed` or `cancelled`). A background Goroutine dynamically crawls the database checking `expires_at` and automatically reclaims spots for users who failed to finalize their checkout within 5 minutes.
2. **Role Based Access Control (RBAC)**: Enforced via Middleware. The system logically separates `organizer` routes (putting on an event) from `user` routes (registering for an event ticket) and returns `HTTP 403 Forbidden` on violations.
3. **Anti-Bot Rate Limiting**: An in-memory, Mutex-secured token-bucket `RateLimitMiddleware` restricts active IPs to 5 requests per 10 seconds to defend against burst abuse and brute-force bot scripts.

### 3. Enterprise Operations
- **Idempotency Keys**: Natively defends against duplicate network requests (e.g. users double-clicking "Buy") utilizing `idempotency_key UNIQUE` to prevent stealing spots.
- **Graceful Shutdown**: The server consumes `os/signal` SIGTERM events. It grants active database transactions exactly 5 seconds to cleanly commit or rollback before shutting down the process.
- **Go 1.21+ Structured Logging**: Emits clean observability metrics using `log/slog`.
- **Panic Protection**: A `RecoveryMiddleware` stops corrupted request payloads from crashing the server's memory block, cleanly returning `HTTP 500`.

---

## 🚀 Running the Platform

Ensure Go is installed (`1.22` or greater).

```bash
# Start the server
go run .
```

*The server will boot on `:8080` and auto-initialize a pristine `events.db` SQLite database.*

### API Endpoints
All payloads use `application/json` encoded bodies.

- `POST /events` *(Requires header `X-Role: organizer`)*
- `GET  /events` *(Public)*
- `POST /events/{id}/register` *(Requires header `X-Role: user`)*
- `POST /tickets/{id}/confirm` *(Requires header `X-Role: user`)*

---

## ⚡ The Concurrency Stress Test

The included test suite explicitly targets race condition vectors and proves the Optimistic SQL pattern is completely bulletproof.

It provisions an event with exactly **5 total spots**, and simultaneously launches **100 Goroutines** firing heavily concurrent HTTP transactions against the local machine.

Run it:
```bash
go test -v
```

**Expected Result:**
1. Exactly `5` Goroutines safely return `HTTP 200`.
2. Exactly `95` Goroutines are gracefully rejected with `HTTP 409 Conflict (Sold Out)`. 
3. Database constraints remain physically unbroken.

---
---

## Technical Specifications Prompt (Design Document)
*The following is the highly technical Product Requirements prompt used as the overarching architectural guideline for this application.*

> **System Design Prompt:**
> "Design a production-style Event Registration & Ticketing REST API in Go. You must use the modern `net/http` standard library from Go 1.22+ to handle routing, bypassing heavy third-party frameworks. The data persistence layer must utilize SQLite for zero-configuration portability, specifically using `modernc.org/sqlite` to prevent Windows CGO compiler issues.
> 
> Your core concurrency strategy must mathematically prevent overbooking. Do not use memory-bound `sync.Mutex` objects. Instead, utilize an Optimistic Concurrency pattern utilizing a conditional SQL `UPDATE` bound within an ACID transaction (meaning you only decrement `available_spots` if `> 0`, and rollback immediately if rows affected is `0`). 
> Include SQLite schema constraints like `CHECK` and `UNIQUE` to enforce business logic natively.
> 
> The application must move beyond a simple API and include 'The Perfect Trio' of mature product features:
> 1. Implement Role-Based Access Control Middleware distinguishing between Users and Organizers.
> 2. Implement an aggressive in-memory, Mutex-bound Rate Limiting Middleware using a token bucket approach mapping IP boundaries to prevent bot floods.
> 3. Implement 'Seat Reservation Auto-Expiry'. Tickets must initially insert as 'reserved' with a 5-minute `expires_at` column. Write a background Goroutine paired to the server context that loops every 10 seconds, atomicaly hunting for expired tickets, marking them as 'cancelled', and dynamically returning their spots to the parent event.
> 
> Lastly, include enterprise operational requirements: standard HTTP semantic return codes, a graceful `os/signal` termination interception giving in-flight HTTP transactions 5 seconds to drain safely, and `log/slog` structured JSON telemetry. Write a `concurrent_test.go` file with a `sync.WaitGroup` that simulates 100 heavily concurrent Goroutine requests simultaneously fighting for exactly 5 database spots to empirically prove zero memory race conditions exist."
