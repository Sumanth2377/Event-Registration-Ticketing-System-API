# Design Document: Event Registration API

## 1. Problem Statement
The critical challenge for Capstone Project 5 is handling concurrent registrations to prevent *overbooking* when multiple users try to register for the last few spots simultaneously. 
In a naive implementation, if an event has 1 spot left and 100 users hit the API at the same millisecond, all 100 threads might read `available_spots = 1`, resulting in 100 successful bookings (99 overbooked).

## 2. Approach to Concurrency

### Options Considered
- **Application-Level Mutex (`sync.Mutex`)**: Locking the critical section in Go.
  - *Pros:* Easy to write single-node code.
  - *Cons:* Fails completely when deployed across multiple servers (Horizontal Scaling). The mutex lock belongs to a single process.
- **Go Channels / Worker Pool**: Queueing registrations into a single goroutine worker.
  - *Pros:* Elegant Go mechanics.
  - *Cons:* Also fails when deployed across multiple instances unless a distributed queue (like Redis or Kafka) is introduced, adding complexity.
- **Optimistic Concurrency Control via Database (Atomic Update)**: This is the approach we took.

### Chosen Approach: Optimistic Concurrency Control
Rather than reading the state, making a software decision, and then writing back, we shifted the responsibility to the database's ACID properties.

The core logic uses a single atomic SQL update query:

```sql
UPDATE events 
SET available_spots = available_spots - 1 
WHERE id = ? AND available_spots > 0;
```

#### How it works:
1. **Transaction Begins**: SQL guarantees isolation.
2. **Atomic Update**: We attempt to decrement `available_spots` by 1, but *only* if `available_spots > 0`. 
3. **Condition Check**: Even if 1000 requests are fired, the database engine executes these sequentially internally using its own locking mechanism. Once `available_spots` hits `0`, the query silently fails to update any rows.
4. **Verification**: If `RowsAffected() == 0`, we know the event is sold out, and we `Rollback` the transaction and throw a `409 Conflict`.
5. **Success Path**: If `RowsAffected() == 1`, we confidently insert the user's registration record and `Commit`.

This method is horizontal-scaling-friendly, fast, and does not require complex distributed locks like Redis or Etcd.

## 3. Technology Stack Selection

- **Language:** Go 1.22
- **Router:** Standard Library `net/http` using the new `ServeMux` features. This showcases deep knowledge of Go's built-in strength over relying on high-level bloated frameworks.
- **Database:** SQLite (`modernc.org/sqlite`). This is a pure-Go implementation of SQLite, meaning it removes the need for CGO. We chose this so that Judges can immediately run and test the application on Windows, Mac, or Linux without installing MinGW or GCC.

## 4. Testing the Strategy
We built a dedicated integration test (`tests/concurrent_test.go`) that spawns 100 simultaneous Goroutines to register for an event seeded with only 5 spots. The test repeatedly proves that exactly 5 rows are successfully committed to the database, leaving 95 identical errors handled correctly.
