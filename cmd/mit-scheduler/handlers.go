package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Handler handles HTTP requests for the scheduler
type Handler struct {
	scheduler *Scheduler
	db        *DB
	logger    *log.Logger
}

// NewHandler creates a new handler instance
func NewHandler(scheduler *Scheduler, db *DB, logger *log.Logger) *Handler {
	return &Handler{
		scheduler: scheduler,
		db:        db,
		logger:    logger,
	}
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := h.scheduler.Status()

	dbHealthy := false
	if h.db != nil {
		if err := h.db.Ping(); err == nil {
			dbHealthy = true
		}
	}

	response := map[string]interface{}{
		"service":           "mit-scheduler",
		"status":            "healthy",
		"scheduler_running": status.Running,
		"database_healthy":  dbHealthy,
	}

	respondJSON(w, http.StatusOK, response)
}

// GetSchedulerStatus handles GET /scheduler/status
func (h *Handler) GetSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	status := h.scheduler.Status()
	respondJSON(w, http.StatusOK, status)
}

// TriggerScheduler handles POST /scheduler/trigger
func (h *Handler) TriggerScheduler(w http.ResponseWriter, r *http.Request) {
	h.logger.Println("Manual scheduler trigger requested")

	result, err := h.scheduler.TriggerManual()
	if err != nil {
		h.logger.Printf("Error during manual trigger: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to trigger scheduler", "TRIGGER_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Scheduler triggered successfully",
		"processed":  result.Processed,
		"successful": result.Successful,
		"failed":     result.Failed,
		"duration":   result.Duration.String(),
		"jobs":       result.Jobs,
	})
}

// ListJobs handles GET /scheduler/jobs
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Default limit
	limit := 50

	jobs, err := h.db.ListJobs(ctx, limit)
	if err != nil {
		h.logger.Printf("Error listing jobs: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list jobs", "LIST_JOBS_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// GetJob handles GET /scheduler/jobs/{id}
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, err := h.db.GetJob(ctx, jobID)
	if err != nil {
		h.logger.Printf("Error getting job %s: %v", jobID, err)
		respondError(w, http.StatusNotFound, "Job not found", "JOB_NOT_FOUND")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// ListRetries handles GET /scheduler/retries
func (h *Handler) ListRetries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get status filter from query
	status := r.URL.Query().Get("status")
	if status == "" {
		status = RetryStatusPending
	}

	retries, err := h.db.ListRetries(ctx, status, 100)
	if err != nil {
		h.logger.Printf("Error listing retries: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list retries", "LIST_RETRIES_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"retries": retries,
		"total":   len(retries),
		"status":  status,
	})
}

// GetRetry handles GET /scheduler/retries/{id}
func (h *Handler) GetRetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	retryID := vars["id"]

	retry, err := h.db.GetRetryEntry(ctx, retryID)
	if err != nil {
		h.logger.Printf("Error getting retry %s: %v", retryID, err)
		respondError(w, http.StatusNotFound, "Retry not found", "RETRY_NOT_FOUND")
		return
	}

	respondJSON(w, http.StatusOK, retry)
}

// RetryNow handles POST /scheduler/retries/{id}/retry-now
func (h *Handler) RetryNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	retryID := vars["id"]

	h.logger.Printf("Immediate retry requested for %s", retryID)

	// Get the retry entry
	entry, err := h.db.GetRetryEntry(ctx, retryID)
	if err != nil {
		h.logger.Printf("Error getting retry %s: %v", retryID, err)
		respondError(w, http.StatusNotFound, "Retry not found", "RETRY_NOT_FOUND")
		return
	}

	if entry.Status != RetryStatusPending {
		respondError(w, http.StatusBadRequest, "Retry is not in pending status", "INVALID_RETRY_STATUS")
		return
	}

	// Process the retry immediately
	result, err := h.scheduler.executor.ProcessRetry(ctx, entry)
	if err != nil {
		h.logger.Printf("Error processing retry %s: %v", retryID, err)
		respondError(w, http.StatusInternalServerError, "Failed to process retry", "RETRY_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success":        result.Success,
		"retry_id":       retryID,
		"subscription_id": entry.SubscriptionID,
		"transaction_id": result.TransactionID,
		"processor_used": result.ProcessorUsed,
		"error_code":     result.ErrorCode,
		"error_message":  result.ErrorMessage,
	})
}

// CancelRetry handles POST /scheduler/retries/{id}/cancel
func (h *Handler) CancelRetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	retryID := vars["id"]

	h.logger.Printf("Cancel retry requested for %s", retryID)

	// Get the retry entry first to verify it exists and is cancellable
	entry, err := h.db.GetRetryEntry(ctx, retryID)
	if err != nil {
		h.logger.Printf("Error getting retry %s: %v", retryID, err)
		respondError(w, http.StatusNotFound, "Retry not found", "RETRY_NOT_FOUND")
		return
	}

	if entry.Status != RetryStatusPending && entry.Status != RetryStatusProcessing {
		respondError(w, http.StatusBadRequest, "Retry cannot be canceled", "INVALID_RETRY_STATUS")
		return
	}

	// Cancel the retry
	if err := h.db.CancelRetry(ctx, retryID); err != nil {
		h.logger.Printf("Error canceling retry %s: %v", retryID, err)
		respondError(w, http.StatusInternalServerError, "Failed to cancel retry", "CANCEL_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"retry_id":       retryID,
		"subscription_id": entry.SubscriptionID,
		"message":        "Retry canceled successfully",
	})
}

// GetRetryStats handles GET /scheduler/stats
func (h *Handler) GetRetryStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.db.GetRetryStats(ctx)
	if err != nil {
		h.logger.Printf("Error getting retry stats: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get stats", "STATS_FAILED")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message, code string) {
	respondJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}
