package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(connectionString string) (*DB, error) {
	conn, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping checks if the database is accessible
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// ============== Plan Operations ==============

// GetPlan retrieves a plan by ID
func (db *DB) GetPlan(ctx context.Context, id string) (*Plan, error) {
	query := `
		SELECT id, name, display_name, amount, currency, interval,
			   trial_days, features, is_active, created_at, updated_at
		FROM plans WHERE id = $1`

	var p Plan
	var features []byte

	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.Amount, &p.Currency, &p.Interval,
		&p.TrialDays, &features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	p.Features = features
	return &p, nil
}

// ListPlans retrieves all active plans
func (db *DB) ListPlans(ctx context.Context, includeInactive bool) ([]Plan, error) {
	query := `
		SELECT id, name, display_name, amount, currency, interval,
			   trial_days, features, is_active, created_at, updated_at
		FROM plans`

	if !includeInactive {
		query += " WHERE is_active = true"
	}

	query += " ORDER BY amount ASC"

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		var p Plan
		var features []byte

		err := rows.Scan(
			&p.ID, &p.Name, &p.DisplayName, &p.Amount, &p.Currency, &p.Interval,
			&p.TrialDays, &features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan plan: %w", err)
		}

		p.Features = features
		plans = append(plans, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating plans: %w", err)
	}

	return plans, nil
}

// ============== Subscription Operations ==============

// CreateSubscription creates a new subscription
func (db *DB) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*Subscription, error) {
	// Get the plan first
	plan, err := db.GetPlan(ctx, req.PlanID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Calculate period dates
	var periodEnd time.Time
	if plan.Interval == IntervalMonthly {
		periodEnd = now.AddDate(0, 1, 0)
	} else {
		periodEnd = now.AddDate(1, 0, 0)
	}

	// Handle trial period
	var trialStart, trialEnd *time.Time
	var status string
	var nextBillingDate time.Time

	if plan.TrialDays > 0 {
		ts := now
		te := now.AddDate(0, 0, plan.TrialDays)
		trialStart = &ts
		trialEnd = &te
		status = SubscriptionStatusTrialing
		nextBillingDate = te
	} else {
		status = SubscriptionStatusActive
		nextBillingDate = periodEnd
	}

	// Parse user_id - if it's not a valid UUID, generate one using the string as seed
	var userUUID uuid.UUID
	userUUID, err = uuid.Parse(req.UserID)
	if err != nil {
		// Generate a deterministic UUID from the user string
		userUUID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(req.UserID))
	}

	// Handle payment_method_id - can be null if not provided or invalid
	var paymentMethodID interface{}
	if req.PaymentMethodID != "" {
		pmUUID, parseErr := uuid.Parse(req.PaymentMethodID)
		if parseErr != nil {
			// Generate deterministic UUID or set to null
			paymentMethodID = nil
		} else {
			paymentMethodID = pmUUID
		}
	}

	query := `
		INSERT INTO subscriptions (
			user_id, plan_id, payment_method_id, status, amount, currency,
			billing_cycle, current_period_start, current_period_end,
			next_billing_date, trial_start, trial_end, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`

	var returnedID string
	err = db.conn.QueryRowContext(ctx, query,
		userUUID, req.PlanID, paymentMethodID, status,
		float64(plan.Amount)/100, plan.Currency, plan.Interval,
		now, periodEnd, nextBillingDate, trialStart, trialEnd, now, now,
	).Scan(&returnedID)

	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	return db.GetSubscription(ctx, returnedID)
}

// GetSubscription retrieves a subscription by ID
func (db *DB) GetSubscription(ctx context.Context, id string) (*Subscription, error) {
	query := `
		SELECT id, user_id, plan_id, COALESCE(payment_method_id::text, ''), status, amount, currency,
			   billing_cycle, current_period_start, current_period_end,
			   next_billing_date, cancel_at_period_end, canceled_at,
			   trial_start, trial_end, created_at, updated_at
		FROM subscriptions WHERE id = $1`

	var s Subscription
	var paymentMethodID string
	var amount float64

	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.UserID, &s.PlanID, &paymentMethodID, &s.Status, &amount, &s.Currency,
		&s.BillingCycle, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.NextBillingDate, &s.CancelAtPeriodEnd, &s.CanceledAt,
		&s.TrialStart, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	if paymentMethodID != "" {
		s.PaymentMethodID = paymentMethodID
	}

	// Convert amount from dollars to cents
	s.Amount = int64(amount * 100)

	return &s, nil
}

// GetSubscriptionWithPlan retrieves a subscription with its plan details
func (db *DB) GetSubscriptionWithPlan(ctx context.Context, id string) (*SubscriptionWithPlan, error) {
	sub, err := db.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}

	plan, err := db.GetPlan(ctx, sub.PlanID)
	if err != nil && err != ErrPlanNotFound {
		return nil, err
	}

	return &SubscriptionWithPlan{
		Subscription: *sub,
		Plan:         plan,
	}, nil
}

// ListSubscriptions retrieves subscriptions with optional filters
func (db *DB) ListSubscriptions(ctx context.Context, userID string, status string) ([]SubscriptionWithPlan, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, COALESCE(s.payment_method_id::text, ''), s.status,
			   s.amount, s.currency, s.billing_cycle, s.current_period_start, s.current_period_end,
			   s.next_billing_date, s.cancel_at_period_end, s.canceled_at,
			   s.trial_start, s.trial_end, s.created_at, s.updated_at
		FROM subscriptions s
		WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	if userID != "" {
		query += fmt.Sprintf(" AND s.user_id = $%d", argNum)
		args = append(args, userID)
		argNum++
	}

	if status != "" {
		query += fmt.Sprintf(" AND s.status = $%d", argNum)
		args = append(args, status)
		argNum++
	}

	query += " ORDER BY s.created_at DESC"

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []SubscriptionWithPlan
	planCache := make(map[string]*Plan)

	for rows.Next() {
		var s Subscription
		var paymentMethodID string
		var amount float64

		err := rows.Scan(
			&s.ID, &s.UserID, &s.PlanID, &paymentMethodID, &s.Status,
			&amount, &s.Currency, &s.BillingCycle, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
			&s.NextBillingDate, &s.CancelAtPeriodEnd, &s.CanceledAt,
			&s.TrialStart, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}

		if paymentMethodID != "" {
			s.PaymentMethodID = paymentMethodID
		}

		// Convert amount from dollars to cents
		s.Amount = int64(amount * 100)

		// Get plan from cache or database
		var plan *Plan
		if cached, ok := planCache[s.PlanID]; ok {
			plan = cached
		} else {
			plan, _ = db.GetPlan(ctx, s.PlanID)
			planCache[s.PlanID] = plan
		}

		subscriptions = append(subscriptions, SubscriptionWithPlan{
			Subscription: s,
			Plan:         plan,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscriptions: %w", err)
	}

	return subscriptions, nil
}

// UpdateSubscription updates a subscription
func (db *DB) UpdateSubscription(ctx context.Context, id string, updates map[string]interface{}) (*Subscription, error) {
	if len(updates) == 0 {
		return db.GetSubscription(ctx, id)
	}

	// Build dynamic update query
	query := "UPDATE subscriptions SET updated_at = NOW()"
	args := []interface{}{}
	argNum := 1

	for key, value := range updates {
		query += fmt.Sprintf(", %s = $%d", key, argNum)
		args = append(args, value)
		argNum++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argNum)
	args = append(args, id)

	result, err := db.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrSubscriptionNotFound
	}

	return db.GetSubscription(ctx, id)
}

// UpdateSubscriptionStatus updates the status of a subscription
func (db *DB) UpdateSubscriptionStatus(ctx context.Context, id string, status string) (*Subscription, error) {
	return db.UpdateSubscription(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// CancelSubscription marks a subscription for cancellation at period end
func (db *DB) CancelSubscription(ctx context.Context, id string) (*Subscription, error) {
	now := time.Now()
	return db.UpdateSubscription(ctx, id, map[string]interface{}{
		"cancel_at_period_end": true,
		"canceled_at":          now,
	})
}

// GetSubscriptionsDue retrieves subscriptions due for billing
func (db *DB) GetSubscriptionsDue(ctx context.Context, limit int) ([]Subscription, error) {
	query := `
		SELECT id, user_id, plan_id, COALESCE(payment_method_id::text, ''), status, amount, currency,
			   billing_cycle, current_period_start, current_period_end,
			   next_billing_date, cancel_at_period_end, canceled_at,
			   trial_start, trial_end, created_at, updated_at
		FROM subscriptions
		WHERE status = 'active'
		  AND next_billing_date <= NOW()
		  AND cancel_at_period_end = false
		ORDER BY next_billing_date ASC
		LIMIT $1`

	rows, err := db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get due subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var s Subscription
		var paymentMethodID string
		var amount float64

		err := rows.Scan(
			&s.ID, &s.UserID, &s.PlanID, &paymentMethodID, &s.Status, &amount, &s.Currency,
			&s.BillingCycle, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
			&s.NextBillingDate, &s.CancelAtPeriodEnd, &s.CanceledAt,
			&s.TrialStart, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}

		if paymentMethodID != "" {
			s.PaymentMethodID = paymentMethodID
		}

		s.Amount = int64(amount * 100)
		subscriptions = append(subscriptions, s)
	}

	return subscriptions, rows.Err()
}

// GetSubscriptionStats retrieves subscription statistics
func (db *DB) GetSubscriptionStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'active') as active_count,
			COUNT(*) FILTER (WHERE status = 'past_due') as past_due_count,
			COUNT(*) FILTER (WHERE status = 'canceled') as canceled_count,
			COUNT(*) FILTER (WHERE status = 'trialing') as trialing_count,
			COALESCE(SUM(amount) FILTER (WHERE status = 'active' AND billing_cycle = 'monthly'), 0) as mrr,
			COALESCE(SUM(amount) FILTER (WHERE status = 'active' AND billing_cycle = 'yearly'), 0) / 12 as arr_monthly
		FROM subscriptions`

	var stats struct {
		ActiveCount   int64
		PastDueCount  int64
		CanceledCount int64
		TrialingCount int64
		MRR           float64
		ARRMonthly    float64
	}

	err := db.conn.QueryRowContext(ctx, query).Scan(
		&stats.ActiveCount, &stats.PastDueCount, &stats.CanceledCount,
		&stats.TrialingCount, &stats.MRR, &stats.ARRMonthly,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get subscription stats: %w", err)
	}

	return map[string]interface{}{
		"active":     stats.ActiveCount,
		"past_due":   stats.PastDueCount,
		"canceled":   stats.CanceledCount,
		"trialing":   stats.TrialingCount,
		"mrr":        stats.MRR + stats.ARRMonthly,
		"total":      stats.ActiveCount + stats.PastDueCount + stats.CanceledCount + stats.TrialingCount,
	}, nil
}

// AdvanceSubscriptionPeriod moves the subscription to the next billing period
func (db *DB) AdvanceSubscriptionPeriod(ctx context.Context, id string) (*Subscription, error) {
	sub, err := db.GetSubscription(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var newPeriodEnd time.Time

	if sub.BillingCycle == IntervalMonthly {
		newPeriodEnd = now.AddDate(0, 1, 0)
	} else {
		newPeriodEnd = now.AddDate(1, 0, 0)
	}

	return db.UpdateSubscription(ctx, id, map[string]interface{}{
		"current_period_start": now,
		"current_period_end":   newPeriodEnd,
		"next_billing_date":    newPeriodEnd,
		"status":               SubscriptionStatusActive,
	})
}

// Helper function to get default plans as JSON for seeding
func GetDefaultPlans() []Plan {
	basicFeatures, _ := json.Marshal([]string{
		"Up to 1,000 transactions/month",
		"Email support",
		"Basic analytics",
		"1 team member",
	})

	proFeatures, _ := json.Marshal([]string{
		"Up to 10,000 transactions/month",
		"Priority support",
		"Advanced analytics",
		"5 team members",
		"Custom webhooks",
	})

	enterpriseFeatures, _ := json.Marshal([]string{
		"Unlimited transactions",
		"24/7 dedicated support",
		"Real-time analytics",
		"Unlimited team members",
		"Custom integrations",
		"SLA guarantee",
	})

	return []Plan{
		{ID: PlanBasicMonthly, Name: "basic", DisplayName: "Basic Monthly", Amount: 2900, Currency: "USD", Interval: IntervalMonthly, TrialDays: 14, Features: basicFeatures, IsActive: true},
		{ID: PlanProMonthly, Name: "pro", DisplayName: "Pro Monthly", Amount: 7900, Currency: "USD", Interval: IntervalMonthly, TrialDays: 14, Features: proFeatures, IsActive: true},
		{ID: PlanEnterpriseMonthly, Name: "enterprise", DisplayName: "Enterprise Monthly", Amount: 19900, Currency: "USD", Interval: IntervalMonthly, TrialDays: 30, Features: enterpriseFeatures, IsActive: true},
		{ID: PlanBasicYearly, Name: "basic", DisplayName: "Basic Annual", Amount: 29000, Currency: "USD", Interval: IntervalYearly, TrialDays: 14, Features: basicFeatures, IsActive: true},
		{ID: PlanProYearly, Name: "pro", DisplayName: "Pro Annual", Amount: 79000, Currency: "USD", Interval: IntervalYearly, TrialDays: 14, Features: proFeatures, IsActive: true},
		{ID: PlanEnterpriseYearly, Name: "enterprise", DisplayName: "Enterprise Annual", Amount: 199000, Currency: "USD", Interval: IntervalYearly, TrialDays: 30, Features: enterpriseFeatures, IsActive: true},
	}
}
