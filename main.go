package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Setup structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// We can pass DSN from command line
	dsn := flag.String("dsn", "file:events.db?cache=shared&mode=rwc", "SQLite DSN")
	port := flag.String("port", ":8080", "Server Port")
	flag.Parse()

	// Initialize Database
	db, err := NewDB(*dsn)
	if err != nil {
		slog.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}

	// Important: We use a short timeout for schema init to avoid pulling down the server on boot
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.InitSchema(ctx); err != nil {
		slog.Error("failed to initialize schema", "error", err)
		os.Exit(1)
	}
	slog.Info("database schema initialized")

	// Context for background workers, cancelled on graceful shutdown
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel() // Ensure worker context is cancelled on main exit

	// Background Worker for Reclaiming Seats
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				slog.Info("reclaim expired seats worker stopping")
				return
			case <-ticker.C:
				reclaimed, err := db.ReclaimExpiredSeats(context.Background())
				if err != nil {
					slog.Error("failed reclaimed seats worker", "error", err)
				} else if reclaimed > 0 {
					slog.Info("reclaimed expired seats", "count", reclaimed)
				}
			}
		}
	}()

	// Set up Handlers
	h := &Handlers{DB: db}

	// Standard Library Router
	mux := http.NewServeMux()

	// Create Event (Protected: Organizer/Admin)
	mux.Handle("POST /events", RBACMiddleware("organizer")(http.HandlerFunc(h.HandleCreateEvent)))

	// List Events (Public)
	mux.HandleFunc("GET /events", h.HandleListEvents)

	// Register (Protected: User)
	mux.Handle("POST /events/{id}/register", RBACMiddleware("user")(http.HandlerFunc(h.HandleRegister)))

	// Confirm (Protected: User)
	mux.Handle("POST /tickets/{id}/confirm", RBACMiddleware("user")(http.HandlerFunc(h.HandleConfirm)))

	// Apply Global Middlewares
	var handler http.Handler = mux
	handler = RateLimitMiddleware(handler)
	handler = LoggingMiddleware(handler)
	handler = RecoveryMiddleware(handler)

	// Configure Server with Timeouts
	server := &http.Server{
		Addr:         *port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful Shutdown Setup
	go func() {
		slog.Info("server starting", "port", *port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// 5 seconds to finish in-flight requests
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(ctxShutdown); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	// Close DB connection last
	if err := db.Close(); err != nil {
		slog.Error("failed to close db", "error", err)
	}

	slog.Info("server exited cleanly")
}
