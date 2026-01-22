package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	logger := log.New(os.Stdout, "[subscription-service] ", log.LstdFlags)

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://payment_user:payment_pass@localhost:5432/payment_db?sslmode=disable"
	}

	// Get Payment Orchestrator URL from environment
	orchestratorURL := os.Getenv("ORCHESTRATOR_URL")
	if orchestratorURL == "" {
		orchestratorURL = "http://localhost:8001"
	}

	// Connect to database
	db, err := NewDB(dbURL)
	if err != nil {
		logger.Printf("Warning: Failed to connect to database: %v", err)
		logger.Println("Starting in limited mode (no database)")
		db = nil
	} else {
		defer db.Close()
		logger.Println("Connected to database")
	}

	// Create handlers
	handler := NewHandler(db, logger)
	billingHandler := NewBillingHandler(db, orchestratorURL, logger)

	// Setup router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", handler.Health).Methods("GET")

	// Plans endpoints
	r.HandleFunc("/plans", handler.ListPlans).Methods("GET")
	r.HandleFunc("/plans/{id}", handler.GetPlan).Methods("GET")

	// Subscriptions endpoints
	r.HandleFunc("/subscriptions", handler.CreateSubscription).Methods("POST")
	r.HandleFunc("/subscriptions", handler.ListSubscriptions).Methods("GET")
	r.HandleFunc("/subscriptions/{id}", handler.GetSubscription).Methods("GET")
	r.HandleFunc("/subscriptions/{id}/cancel", handler.CancelSubscription).Methods("PUT")
	r.HandleFunc("/subscriptions/{id}/upgrade", handler.UpgradeSubscription).Methods("PUT")
	r.HandleFunc("/subscriptions/{id}/downgrade", handler.DowngradeSubscription).Methods("PUT")

	// Billing endpoints (Commit 1.3)
	r.HandleFunc("/subscriptions/{id}/charge", billingHandler.ChargeSubscription).Methods("POST")
	r.HandleFunc("/subscriptions/{id}/invoices", billingHandler.ListInvoices).Methods("GET")

	// Stats endpoint
	r.HandleFunc("/stats/subscriptions", handler.GetSubscriptionStats).Methods("GET")

	// Start server
	addr := ":8002"
	logger.Printf("Subscription Service starting on %s", addr)
	logger.Printf("Payment Orchestrator URL: %s", orchestratorURL)
	logger.Fatal(http.ListenAndServe(addr, r))
}
