package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	logger := log.New(os.Stdout, "[MIT-SCHEDULER] ", log.LstdFlags|log.Lshortfile)

	// Get database connection string
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	}

	// Initialize database connection
	db, err := NewDB(dbURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	logger.Println("Database connection established")

	// Ensure scheduler tables exist
	ctx := context.Background()
	if err := db.EnsureJobsTableExists(ctx); err != nil {
		logger.Printf("Warning: Failed to ensure jobs table exists: %v", err)
	}
	if err := db.EnsureRetryTableExists(ctx); err != nil {
		logger.Printf("Warning: Failed to ensure retry_queue table exists: %v", err)
	}

	// Initialize scheduler with default config
	config := DefaultSchedulerConfig()

	// Allow config override via environment
	if interval := os.Getenv("SCHEDULER_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			config.TickInterval = d
		}
	}

	// Get subscription service URL
	subscriptionServiceURL := os.Getenv("SUBSCRIPTION_SERVICE_URL")
	if subscriptionServiceURL == "" {
		subscriptionServiceURL = "http://localhost:8002"
	}

	// Initialize executor
	executor := NewExecutor(db, subscriptionServiceURL, logger)

	scheduler := NewScheduler(db, executor, config, logger)

	// Initialize handlers
	handler := NewHandler(scheduler, db, logger)

	// Setup routes
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Scheduler endpoints
	r.HandleFunc("/scheduler/status", handler.GetSchedulerStatus).Methods("GET")
	r.HandleFunc("/scheduler/trigger", handler.TriggerScheduler).Methods("POST")
	r.HandleFunc("/scheduler/jobs", handler.ListJobs).Methods("GET")
	r.HandleFunc("/scheduler/jobs/{id}", handler.GetJob).Methods("GET")

	// Retry queue endpoints
	r.HandleFunc("/scheduler/retries", handler.ListRetries).Methods("GET")
	r.HandleFunc("/scheduler/retries/{id}", handler.GetRetry).Methods("GET")
	r.HandleFunc("/scheduler/retries/{id}/retry-now", handler.RetryNow).Methods("POST")
	r.HandleFunc("/scheduler/retries/{id}/cancel", handler.CancelRetry).Methods("POST")
	r.HandleFunc("/scheduler/stats", handler.GetRetryStats).Methods("GET")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":8004",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start scheduler in background
	scheduler.Start()

	// Handle graceful shutdown
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Println("Server is shutting down...")

		// Stop scheduler first
		scheduler.Stop()

		// Shutdown HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	logger.Println("MIT Scheduler starting on port 8004")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on :8004: %v\n", err)
	}

	<-done
	logger.Println("Server stopped")
}
