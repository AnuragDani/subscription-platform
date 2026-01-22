package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	db     *DB
	logger *log.Logger
}

// NewHandler creates a new handler with dependencies
func NewHandler(db *DB, logger *log.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string, code string) {
	respondJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// ============== Plan Handlers ==============

// ListPlans handles GET /plans
func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	plans, err := h.db.ListPlans(ctx, false)
	if err != nil {
		h.logger.Printf("Error listing plans: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list plans", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusOK, PlanListResponse{
		Plans: plans,
		Total: len(plans),
	})
}

// GetPlan handles GET /plans/{id}
func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	plan, err := h.db.GetPlan(ctx, id)
	if err != nil {
		if err == ErrPlanNotFound {
			respondError(w, http.StatusNotFound, "Plan not found", "PLAN_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get plan", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusOK, plan)
}

// ============== Subscription Handlers ==============

// CreateSubscription handles POST /subscriptions
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", "INVALID_REQUEST")
		return
	}

	if err := req.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
		return
	}

	sub, err := h.db.CreateSubscription(ctx, &req)
	if err != nil {
		if err == ErrPlanNotFound {
			respondError(w, http.StatusBadRequest, "Plan not found", "PLAN_NOT_FOUND")
			return
		}
		h.logger.Printf("Error creating subscription: %v (request: %+v)", err, req)
		respondError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}

	// Get subscription with plan details
	subWithPlan, err := h.db.GetSubscriptionWithPlan(ctx, sub.ID)
	if err != nil {
		h.logger.Printf("Error getting subscription with plan: %v", err)
		respondJSON(w, http.StatusCreated, sub)
		return
	}

	h.logger.Printf("Created subscription %s for user %s on plan %s", sub.ID, sub.UserID, sub.PlanID)
	respondJSON(w, http.StatusCreated, SubscriptionResponse{
		SubscriptionWithPlan: subWithPlan,
		Message:              "Subscription created successfully",
	})
}

// GetSubscription handles GET /subscriptions/{id}
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	subWithPlan, err := h.db.GetSubscriptionWithPlan(ctx, id)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get subscription", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusOK, subWithPlan)
}

// ListSubscriptions handles GET /subscriptions
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	status := r.URL.Query().Get("status")

	subs, err := h.db.ListSubscriptions(ctx, userID, status)
	if err != nil {
		h.logger.Printf("Error listing subscriptions: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list subscriptions", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusOK, SubscriptionListResponse{
		Subscriptions: subs,
		Total:         len(subs),
	})
}

// CancelSubscription handles PUT /subscriptions/{id}/cancel
func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	sub, err := h.db.CancelSubscription(ctx, id)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		h.logger.Printf("Error canceling subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to cancel subscription", "INTERNAL_ERROR")
		return
	}

	subWithPlan, _ := h.db.GetSubscriptionWithPlan(ctx, sub.ID)

	h.logger.Printf("Canceled subscription %s", id)
	respondJSON(w, http.StatusOK, SubscriptionResponse{
		SubscriptionWithPlan: subWithPlan,
		Message:              "Subscription will be canceled at the end of the current period",
	})
}

// UpgradeSubscription handles PUT /subscriptions/{id}/upgrade
func (h *Handler) UpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", "INVALID_REQUEST")
		return
	}

	if req.PlanID == "" {
		respondError(w, http.StatusBadRequest, "plan_id is required", "VALIDATION_ERROR")
		return
	}

	// Get current subscription
	currentSub, err := h.db.GetSubscription(ctx, id)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get subscription", "INTERNAL_ERROR")
		return
	}

	// Get current and new plans
	currentPlan, err := h.db.GetPlan(ctx, currentSub.PlanID)
	if err != nil {
		h.logger.Printf("Error getting current plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get current plan", "INTERNAL_ERROR")
		return
	}

	newPlan, err := h.db.GetPlan(ctx, req.PlanID)
	if err != nil {
		if err == ErrPlanNotFound {
			respondError(w, http.StatusBadRequest, "New plan not found", "PLAN_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting new plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get new plan", "INTERNAL_ERROR")
		return
	}

	// Verify this is an upgrade (new plan costs more)
	if newPlan.Amount <= currentPlan.Amount {
		respondError(w, http.StatusBadRequest, "New plan must be higher value than current plan. Use downgrade endpoint instead.", "INVALID_UPGRADE")
		return
	}

	// Calculate proration
	prorationAmount := calculateProration(currentSub, currentPlan, newPlan)

	// Update subscription
	now := time.Now()
	var newPeriodEnd time.Time
	if newPlan.Interval == IntervalMonthly {
		newPeriodEnd = now.AddDate(0, 1, 0)
	} else {
		newPeriodEnd = now.AddDate(1, 0, 0)
	}

	sub, err := h.db.UpdateSubscription(ctx, id, map[string]interface{}{
		"plan_id":              req.PlanID,
		"amount":               float64(newPlan.Amount) / 100,
		"billing_cycle":        newPlan.Interval,
		"current_period_start": now,
		"current_period_end":   newPeriodEnd,
		"next_billing_date":    newPeriodEnd,
	})

	if err != nil {
		h.logger.Printf("Error upgrading subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to upgrade subscription", "INTERNAL_ERROR")
		return
	}

	subWithPlan, _ := h.db.GetSubscriptionWithPlan(ctx, sub.ID)

	h.logger.Printf("Upgraded subscription %s from %s to %s", id, currentSub.PlanID, req.PlanID)
	respondJSON(w, http.StatusOK, SubscriptionResponse{
		SubscriptionWithPlan: subWithPlan,
		ProrationAmount:      prorationAmount,
		Message:              "Subscription upgraded successfully",
	})
}

// DowngradeSubscription handles PUT /subscriptions/{id}/downgrade
func (h *Handler) DowngradeSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", "INVALID_REQUEST")
		return
	}

	if req.PlanID == "" {
		respondError(w, http.StatusBadRequest, "plan_id is required", "VALIDATION_ERROR")
		return
	}

	// Get current subscription
	currentSub, err := h.db.GetSubscription(ctx, id)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			respondError(w, http.StatusNotFound, "Subscription not found", "SUBSCRIPTION_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get subscription", "INTERNAL_ERROR")
		return
	}

	// Get current and new plans
	currentPlan, err := h.db.GetPlan(ctx, currentSub.PlanID)
	if err != nil {
		h.logger.Printf("Error getting current plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get current plan", "INTERNAL_ERROR")
		return
	}

	newPlan, err := h.db.GetPlan(ctx, req.PlanID)
	if err != nil {
		if err == ErrPlanNotFound {
			respondError(w, http.StatusBadRequest, "New plan not found", "PLAN_NOT_FOUND")
			return
		}
		h.logger.Printf("Error getting new plan: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get new plan", "INTERNAL_ERROR")
		return
	}

	// Verify this is a downgrade (new plan costs less)
	if newPlan.Amount >= currentPlan.Amount {
		respondError(w, http.StatusBadRequest, "New plan must be lower value than current plan. Use upgrade endpoint instead.", "INVALID_DOWNGRADE")
		return
	}

	// Downgrade takes effect at end of current period
	sub, err := h.db.UpdateSubscription(ctx, id, map[string]interface{}{
		"plan_id":       req.PlanID,
		"amount":        float64(newPlan.Amount) / 100,
		"billing_cycle": newPlan.Interval,
	})

	if err != nil {
		h.logger.Printf("Error downgrading subscription: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to downgrade subscription", "INTERNAL_ERROR")
		return
	}

	subWithPlan, _ := h.db.GetSubscriptionWithPlan(ctx, sub.ID)

	h.logger.Printf("Downgraded subscription %s from %s to %s", id, currentSub.PlanID, req.PlanID)
	respondJSON(w, http.StatusOK, SubscriptionResponse{
		SubscriptionWithPlan: subWithPlan,
		Message:              "Subscription downgraded. New plan takes effect at end of current period.",
	})
}

// GetSubscriptionStats handles GET /stats/subscriptions
func (h *Handler) GetSubscriptionStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.db.GetSubscriptionStats(ctx)
	if err != nil {
		h.logger.Printf("Error getting subscription stats: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get stats", "INTERNAL_ERROR")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "healthy"
	if h.db != nil {
		if err := h.db.Ping(); err != nil {
			dbStatus = "unhealthy"
		}
	}

	response := map[string]interface{}{
		"service":   "subscription-service",
		"status":    "healthy",
		"database":  dbStatus,
		"timestamp": time.Now(),
	}

	respondJSON(w, http.StatusOK, response)
}

// calculateProration calculates the proration amount for plan changes
func calculateProration(sub *Subscription, currentPlan, newPlan *Plan) int64 {
	if sub.CurrentPeriodEnd == nil || sub.CurrentPeriodStart == nil {
		return 0
	}

	// Calculate days remaining in current period
	now := time.Now()
	periodEnd := *sub.CurrentPeriodEnd
	periodStart := *sub.CurrentPeriodStart

	if now.After(periodEnd) {
		return 0
	}

	totalDays := periodEnd.Sub(periodStart).Hours() / 24
	remainingDays := periodEnd.Sub(now).Hours() / 24

	if totalDays <= 0 {
		return 0
	}

	// Calculate prorated credit from current plan
	dailyRateCurrent := float64(currentPlan.Amount) / totalDays
	creditAmount := dailyRateCurrent * remainingDays

	// Calculate prorated charge for new plan
	dailyRateNew := float64(newPlan.Amount) / totalDays
	chargeAmount := dailyRateNew * remainingDays

	// Proration is the difference (positive means customer owes more)
	return int64(chargeAmount - creditAmount)
}
