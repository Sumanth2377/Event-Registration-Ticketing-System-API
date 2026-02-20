# Event Registration API Summary

A production-grade, concurrency-safe REST API built strictly with standard Go (`net/http`) and a portable SQLite database. Its architecture solves "Ticketing Overbooking" at the database level rather than relying on language-level mutex locks.

## Core Setup & The Atomic Edge
- **Optimistic Concurrency:** Avoids "Read-Calculate-Write" memory race conditions entirely by utilizing a single atomic `UPDATE events SET available_spots = available_spots - 1 WHERE available_spots > 0` wrapped safely inside an ACID `sql.Tx` transaction.
- **Data Integrity:** `CHECK (available_spots >= 0)` constraints strictly guarantee the DB cannot be oversold, even if application bugs arise.
- **Stress-Tested:** `concurrent_test.go` synchronously fires 100 Goroutines attempting to register for 5 spots to empirically prove zero concurrency race conditions.

## The "Perfect Trio" of Business Maturity
1. **Seat Reservation with Expiry:** Creating a ticket only *reserves* a seat for 5 minutes (`expires_at`), preventing checkout hoarding. A background Goroutine dynamically crawls the database every 10 seconds and automatically increments event capacity back up if users fail to `POST /tickets/{id}/confirm` in time.
2. **Role-Based Access Control (RBAC):** Custom Middleware validates `X-Role` headers. Only `organizer` accounts can create events, while `user` accounts are strictly limited to registration flows (`HTTP 403 Forbidden`).
3. **Anti-Bot Rate Limiting:** An in-memory, Mutex-secured Token Bucket restricts IPs to 5 requests per 10 seconds, returning `HTTP 429 Too Many Requests` on burst abuse.

## Enterprise Observability
- **Idempotency Keys:** Safely neutralizing network retries using `idempotency_key UNIQUE` in SQLite, meaning accidental double-clicks from users won't steal two seats.
- **Graceful Shutdown:** Implements `os/signal` termination interception. When the server goes down, it gives active threads a 5-second grace period to cleanly finish or rollback gracefully.
- **Structured JSON Logging:** Modern observability standards met utilizing Go 1.21's new `log/slog`.
- **Panic Protection:** Top-level Panic `RecoveryMiddleware` safely catches corrupted HTTP requests, preventing violent memory crashes and returning a safe `HTTP 500`.
