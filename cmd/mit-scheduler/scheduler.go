package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// Scheduler manages recurring billing jobs
type Scheduler struct {
	db            *DB
	executor      *Executor
	config        *SchedulerConfig
	logger        *log.Logger
	running       bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	lastRun       *time.Time
	nextRun       *time.Time
	processedLast int
	lastResult    *BatchResult
}

// NewScheduler creates a new scheduler instance
func NewScheduler(db *DB, executor *Executor, config *SchedulerConfig, logger *log.Logger) *Scheduler {
	return &Scheduler{
		db:       db,
		executor: executor,
		config:   config,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the scheduler background processing
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Printf("Starting scheduler with tick interval: %v, batch size: %d",
		s.config.TickInterval, s.config.BatchSize)

	s.wg.Add(1)
	go s.run()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	s.logger.Println("Stopping scheduler, waiting for current batch to complete...")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Println("Scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	// Calculate next run time
	next := time.Now().Add(s.config.TickInterval)
	s.mu.Lock()
	s.nextRun = &next
	s.mu.Unlock()

	for {
		select {
		case <-s.stopCh:
			s.logger.Println("Scheduler received stop signal")
			return
		case <-ticker.C:
			if s.config.Enabled {
				s.tick()
			}

			// Update next run time
			next := time.Now().Add(s.config.TickInterval)
			s.mu.Lock()
			s.nextRun = &next
			s.mu.Unlock()
		}
	}
}

// tick executes one scheduling cycle
func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := time.Now()
	s.mu.Lock()
	s.lastRun = &now
	s.mu.Unlock()

	s.logger.Println("Scheduler tick: checking for due subscriptions and retries...")

	// Get subscriptions due for billing
	subscriptions, err := s.db.GetSubscriptionsDue(ctx, s.config.BatchSize)
	if err != nil {
		s.logger.Printf("Error getting due subscriptions: %v", err)
	} else {
		s.logger.Printf("Scheduler tick: found %d subscriptions due for billing", len(subscriptions))
	}

	// Get due retries
	retries, err := s.db.GetDueRetries(ctx, s.config.BatchSize)
	if err != nil {
		s.logger.Printf("Error getting due retries: %v", err)
	} else {
		s.logger.Printf("Scheduler tick: found %d retries due for processing", len(retries))
	}

	totalProcessed := len(subscriptions) + len(retries)
	s.mu.Lock()
	s.processedLast = totalProcessed
	s.mu.Unlock()

	if totalProcessed == 0 {
		return
	}

	// Process subscriptions
	var billingResult *BatchResult
	if len(subscriptions) > 0 {
		billingResult = s.executor.ExecuteBatch(ctx, subscriptions)
		s.logger.Printf("Billing completed: processed=%d, successful=%d, failed=%d",
			billingResult.Processed, billingResult.Successful, billingResult.Failed)
	}

	// Process retries
	var retryResult *BatchResult
	if len(retries) > 0 {
		retryResult = s.executor.ExecuteRetryBatch(ctx, retries)
		s.logger.Printf("Retries completed: processed=%d, successful=%d, failed=%d",
			retryResult.Processed, retryResult.Successful, retryResult.Failed)
	}

	// Combine results
	combinedResult := &BatchResult{}
	if billingResult != nil {
		combinedResult.Processed += billingResult.Processed
		combinedResult.Successful += billingResult.Successful
		combinedResult.Failed += billingResult.Failed
		combinedResult.Jobs = append(combinedResult.Jobs, billingResult.Jobs...)
	}
	if retryResult != nil {
		combinedResult.Processed += retryResult.Processed
		combinedResult.Successful += retryResult.Successful
		combinedResult.Failed += retryResult.Failed
		combinedResult.Jobs = append(combinedResult.Jobs, retryResult.Jobs...)
	}

	s.mu.Lock()
	s.lastResult = combinedResult
	s.mu.Unlock()

	s.logger.Printf("Scheduler tick completed: total_processed=%d, successful=%d, failed=%d",
		combinedResult.Processed, combinedResult.Successful, combinedResult.Failed)
}

// Status returns the current scheduler status
func (s *Scheduler) Status() *SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalJobs := 0
	if s.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if count, err := s.db.GetJobCount(ctx); err == nil {
			totalJobs = count
		}
	}

	return &SchedulerStatus{
		Running:       s.running,
		LastRun:       s.lastRun,
		NextRun:       s.nextRun,
		ProcessedLast: s.processedLast,
		TotalJobs:     totalJobs,
		TickInterval:  s.config.TickInterval.String(),
	}
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerManual manually triggers a scheduler tick (for testing)
func (s *Scheduler) TriggerManual() (*BatchResult, error) {
	s.logger.Println("Manual scheduler trigger initiated")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get subscriptions due for billing
	subscriptions, err := s.db.GetSubscriptionsDue(ctx, s.config.BatchSize)
	if err != nil {
		return nil, err
	}

	s.logger.Printf("Manual trigger: found %d subscriptions due for billing", len(subscriptions))

	if len(subscriptions) == 0 {
		return &BatchResult{}, nil
	}

	// Execute batch through executor
	result := s.executor.ExecuteBatch(ctx, subscriptions)

	now := time.Now()
	s.mu.Lock()
	s.lastRun = &now
	s.processedLast = result.Processed
	s.lastResult = result
	s.mu.Unlock()

	return result, nil
}

// GetLastResult returns the last batch execution result
func (s *Scheduler) GetLastResult() *BatchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastResult
}
