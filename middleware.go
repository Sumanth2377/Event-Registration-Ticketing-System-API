package main

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// responseWriter is a minimal wrapper for http.ResponseWriter that allows the
// written HTTP status code to be captured for logging.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

// LoggingMiddleware logs the incoming HTTP request & its duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := wrapResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
		)
	})
}

// RBACMiddleware demonstrates Role-Based Access Control.
func RBACMiddleware(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mock check: in reality this parses a JWT role claim
			role := r.Header.Get("X-Role")
			if role == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Unauthorized: Missing X-Role header"}`))
				return
			}

			if role != requiredRole && role != "admin" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error": "Forbidden: Insufficient privileges"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware provides a basic per-IP token bucket/window for bot defense.
func RateLimitMiddleware(next http.Handler) http.Handler {
	// Simple fixed window rate limiter (e.g. 5 requests per 10 seconds per IP)
	// In production, use Redis to share state across server instances.
	var (
		mu        sync.Mutex
		visitors  = make(map[string]int)
		lastReset = time.Now()
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()

		// Reset window every 10 seconds
		if time.Since(lastReset) > 10*time.Second {
			visitors = make(map[string]int)
			lastReset = time.Now()
		}

		ip := r.RemoteAddr // In prod, rely on X-Forwarded-For usually

		if visitors[ip] >= 5 {
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "Too Many Requests"}`))
			return
		}

		visitors[ip]++
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// RecoveryMiddleware gracefully handles panics to prevent server crashes.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"trace", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Internal Server Error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
