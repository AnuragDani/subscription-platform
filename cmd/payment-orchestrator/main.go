package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	ws "github.com/AnuragDani/subscription-platform/internal/websocket"
)

type PaymentOrchestrator struct {
	db           *DB
	cache        *RedisClient
	processorA   *ProcessorClient
	processorB   *ProcessorClient
	bpasClient   *BPASClient
	tokenManager *TokenManager
	wsHub        *ws.Hub
	events       *EventEmitter
}

func main() {
	log.Println("🚀 Payment Orchestrator starting on port 8001")

	// Initialize configuration
	cfg := LoadConfig()

	// Connect to database
	db, err := NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Connect to Redis
	cache, err := NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer cache.Close()

	// Initialize processor clients
	processorA := NewProcessorClient("processor_a", cfg.ProcessorAURL)
	processorB := NewProcessorClient("processor_b", cfg.ProcessorBURL)

	// Initialize BPAS client
	bpasClient := NewBPASClient(cfg.BPASServiceURL)

	// Initialize token manager
	tokenManager := NewTokenManager(cfg.NetworkTokenURL, processorA, processorB)

	// Initialize WebSocket hub
	logger := log.New(os.Stdout, "[WS-HUB] ", log.LstdFlags)
	wsHub := ws.NewHub(logger)
	go wsHub.Run()
	log.Println("WebSocket hub started")

	// Initialize event emitter
	eventEmitter := NewEventEmitter(wsHub)

	// Create orchestrator
	orchestrator := &PaymentOrchestrator{
		db:           db,
		cache:        cache,
		processorA:   processorA,
		processorB:   processorB,
		bpasClient:   bpasClient,
		tokenManager: tokenManager,
		wsHub:        wsHub,
		events:       eventEmitter,
	}

	// Setup routes
	r := mux.NewRouter()
	r.HandleFunc("/health", orchestrator.healthCheck).Methods("GET")
	r.HandleFunc("/ws", wsHub.ServeWs).Methods("GET")
	r.HandleFunc("/ws/stats", orchestrator.wsStats).Methods("GET")
	r.HandleFunc("/orchestrator/charge", orchestrator.processCharge).Methods("POST")
	r.HandleFunc("/orchestrator/refund", orchestrator.processRefund).Methods("POST")
	r.HandleFunc("/admin/stats", orchestrator.getStats).Methods("GET")
	r.HandleFunc("/stats/transactions", orchestrator.getTransactionStats).Methods("GET")
	r.HandleFunc("/stats/processors", orchestrator.getProcessorStats).Methods("GET")
	r.HandleFunc("/internal/events", orchestrator.handleInternalEvent).Methods("POST")

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:         ":8001",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("Payment Orchestrator listening on port 8001")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}

func (o *PaymentOrchestrator) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"service":   "payment-orchestrator",
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"dependencies": map[string]string{
			"database":    o.checkDatabaseHealth(),
			"redis":       o.checkRedisHealth(),
			"processor_a": o.checkProcessorHealth(o.processorA),
			"processor_b": o.checkProcessorHealth(o.processorB),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (o *PaymentOrchestrator) checkDatabaseHealth() string {
	if err := o.db.Ping(); err != nil {
		return "unhealthy"
	}
	return "healthy"
}

func (o *PaymentOrchestrator) checkRedisHealth() string {
	ctx := context.Background()
	if err := o.cache.Ping(ctx); err != nil {
		return "unhealthy"
	}
	return "healthy"
}

func (o *PaymentOrchestrator) checkProcessorHealth(client *ProcessorClient) string {
	if client.IsHealthy() {
		return "healthy"
	}
	return "unhealthy"
}

// wsStats returns WebSocket hub statistics
func (o *PaymentOrchestrator) wsStats(w http.ResponseWriter, r *http.Request) {
	stats := o.wsHub.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// InternalEventRequest represents an internal event from other services
type InternalEventRequest struct {
	Type  string      `json:"type"`
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// handleInternalEvent receives events from other services and broadcasts them
func (o *PaymentOrchestrator) handleInternalEvent(w http.ResponseWriter, r *http.Request) {
	var req InternalEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Broadcast the event to all WebSocket clients
	o.wsHub.BroadcastEvent(req.Type, req.Event, req.Data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"client_count": o.wsHub.ClientCount(),
	})
}
