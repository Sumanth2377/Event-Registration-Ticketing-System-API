# Technical Specifications Prompt (Senior Architect Design Document)

*The following is the structured Product Requirements prompt used as the overarching architectural guideline for this application. It demonstrates the technical depth used to prompt a mature Golang system.*

> **System Architecture & Implementation Prompt:**
> "Act as a Senior Backend Staff Engineer. Design and implement a production-grade Event Registration & Ticketing REST API in Go. You must strictly use the standard library `net/http` from Go 1.22+ to handle multiplexing, bypassing bloated third-party routers. The persistence layer must be SQLite using `modernc.org/sqlite` (pure Go port) to guarantee zero-configuration compilation across cross-platform CI pipelines.
> 
> **1. Concurrency Defense (The Core Requirement):**
> Your core concurrency strategy must mathematically and flawlessly prevent overbooking. Do NOT use `sync.Mutex`. Instead, implement an Optimistic Concurrency Control (OCC) pattern. Decrement ticket capacity utilizing a conditional SQL `UPDATE events SET available_spots = available_spots - 1 WHERE available_spots > 0` bound strictly within an ACID `sql.Tx` transaction. Enforce database integrity using `CHECK (available_spots >= 0)` and `UNIQUE(event_id, email)` constraints to make the schema self-defending.
> 
> **2. The 'Perfect Trio' Product Requirements:**
> Elevate the system from a basic API to a mature Commerce Platform by implementing these three required features:
> - **State-Machine Ticketing (Seat Expiry):** Tickets must initially insert with `status='reserved'` and an `expires_at` column set 5 minutes into the future. Write a dedicated background Goroutine, attached to the main context, that sweeps the database every 10 seconds. It must atomically find expired rows, mark them `cancelled`, and dynamically return their spots to the parent event. Implement a `POST /tickets/{id}/confirm` checkout endpoint to lock the ticket status to `confirmed`.
> - **Role-Based Access Control (RBAC):** Build an `RBACMiddleware()` that validates standard requests. Enforce that `POST /events` requires an `organizer` role, while registration flows require a `user` role. Emit strict `HTTP 403 Forbidden` on violations.
> - **Anti-Bot Defense (Rate Limiting):** Build a `RateLimitMiddleware()` utilizing an in-memory, Mutex-bound sliding token bucket. Throttle IP addresses to exactly 5 requests per 10-second window, returning `HTTP 429 Too Many Requests`.
> 
> **3. Enterprise Operations:** 
> Implement standard REST semantics natively (201 Created, 400 Bad Request, 403 Forbidden, 409 Conflict for sold-out/duplicate states). Include `idempotency_key` logic within the `tickets` schema to neutralize network retry double-billing loops. Implement `os/signal` termination interception, granting in-flight HTTP connections exactly 5 seconds to drain or rollback cleanly before container shutdown. Finally, wrap all routes in a Go 1.21+ `log/slog` structured JSON telemetry logger and a Top-Level Panic `RecoveryMiddleware`.
>
> **4. Testing Parity:**
> Provide a `concurrent_test.go` suite using `sync.WaitGroup` and `sync/atomic`. It must simulate 100 heavily concurrent Goroutine HTTP requests simultaneously fighting for exactly 5 distinct database spots. Assert that exactly 5 succeed, exactly 95 fail gracefully with a specific error, and zero memory race conditions or SQLite deadlocks occur."
