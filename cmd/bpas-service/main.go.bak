package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

type BPASService struct {
	mu           sync.RWMutex
	rules        []RoutingRule
	configPath   string
	lastModified time.Time
	stats        BPASStats
}

type BPASStats struct {
	TotalEvaluations      int            `json:"total_evaluations"`
	RuleHits              map[string]int `json:"rule_hits"`
	ProcessorDistribution map[string]int `json:"processor_distribution"`
	LastConfigReload      time.Time      `json:"last_config_reload"`
	ConfigReloadCount     int            `json:"config_reload_count"`
	AverageEvalTime       float64        `json:"average_eval_time_ms"`
}

type RoutingRule struct {
	Name            string                 `yaml:"name" json:"name"`
	Priority        int                    `yaml:"priority" json:"priority"`
	ConditionType   string                 `yaml:"condition_type" json:"condition_type"`
	ConditionValue  map[string]interface{} `yaml:"condition_value" json:"condition_value"`
	TargetProcessor string                 `yaml:"target_processor" json:"target_processor"`
	Percentage      int                    `yaml:"percentage" json:"percentage"`
	IsActive        bool                   `yaml:"is_active" json:"is_active"`
	Description     string                 `yaml:"description,omitempty" json:"description,omitempty"`
	CreatedAt       time.Time              `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt       time.Time              `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type RoutingConfig struct {
	Version      string        `yaml:"version"`
	LastUpdated  time.Time     `yaml:"last_updated"`
	RoutingRules []RoutingRule `yaml:"routing_rules"`
	DefaultRule  RoutingRule   `yaml:"default_rule"`
}

type EvaluationRequest struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Marketplace string  `json:"marketplace,omitempty"`
	UserTier    string  `json:"user_tier,omitempty"`
	UserID      string  `json:"user_id,omitempty"`
	ClientID    string  `json:"client_id,omitempty"`
}

type EvaluationResponse struct {
	Success         bool          `json:"success"`
	TargetProcessor string        `json:"target_processor"`
	RuleMatched     string        `json:"rule_matched"`
	RulePriority    int           `json:"rule_priority"`
	Confidence      float64       `json:"confidence"`
	Alternatives    []Alternative `json:"alternatives,omitempty"`
	EvaluationTime  float64       `json:"evaluation_time_ms"`
	ErrorMessage    string        `json:"error_message,omitempty"`
}

type Alternative struct {
	Processor string  `json:"processor"`
	Weight    float64 `json:"weight"`
	Reason    string  `json:"reason"`
}

type RulesListResponse struct {
	Rules       []RoutingRule `json:"rules"`
	TotalRules  int           `json:"total_rules"`
	ActiveRules int           `json:"active_rules"`
	LastReload  time.Time     `json:"last_reload"`
}

func NewBPASService(configPath string) *BPASService {
	service := &BPASService{
		configPath: configPath,
		stats: BPASStats{
			RuleHits:              make(map[string]int),
			ProcessorDistribution: make(map[string]int),
		},
	}

	if err := service.loadConfig(); err != nil {
		log.Printf("Warning: Failed to load initial config: %v", err)
		service.loadDefaultConfig()
	}

	return service
}

func (b *BPASService) loadConfig() error {
	configFile := filepath.Join(b.configPath, "routing-rules.yaml")
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config RoutingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Sort rules by priority (lower number = higher priority)
	sort.Slice(config.RoutingRules, func(i, j int) bool {
		return config.RoutingRules[i].Priority < config.RoutingRules[j].Priority
	})

	b.rules = config.RoutingRules
	b.lastModified = time.Now()
	b.stats.LastConfigReload = time.Now()
	b.stats.ConfigReloadCount++

	log.Printf("Loaded %d routing rules from config", len(b.rules))
	return nil
}

func (b *BPASService) loadDefaultConfig() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Default routing rules if config file is not available
	b.rules = []RoutingRule{
		{
			Name:            "high_value_transactions",
			Priority:        1,
			ConditionType:   "amount_threshold",
			ConditionValue:  map[string]interface{}{"amount": 1000.0, "operator": "greater_than"},
			TargetProcessor: "processor_a",
			Percentage:      100,
			IsActive:        true,
			Description:     "Route high-value transactions to primary processor",
			CreatedAt:       time.Now(),
		},
		{
			Name:            "euro_transactions",
			Priority:        2,
			ConditionType:   "currency",
			ConditionValue:  map[string]interface{}{"currencies": []string{"EUR", "GBP"}},
			TargetProcessor: "processor_b",
			Percentage:      100,
			IsActive:        true,
			Description:     "Route EUR/GBP to multi-currency processor",
			CreatedAt:       time.Now(),
		},
		{
			Name:            "default_primary_split",
			Priority:        10,
			ConditionType:   "percentage",
			ConditionValue:  map[string]interface{}{},
			TargetProcessor: "processor_a",
			Percentage:      70,
			IsActive:        true,
			Description:     "Default 70% to primary processor",
			CreatedAt:       time.Now(),
		},
		{
			Name:            "default_secondary_split",
			Priority:        11,
			ConditionType:   "percentage",
			ConditionValue:  map[string]interface{}{},
			TargetProcessor: "processor_b",
			Percentage:      30,
			IsActive:        true,
			Description:     "Default 30% to secondary processor",
			CreatedAt:       time.Now(),
		},
	}

	b.lastModified = time.Now()
	log.Println("Loaded default routing rules")
}

func (b *BPASService) evaluateRouting(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req EvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.Amount < 0 {
		response := EvaluationResponse{
			Success:      false,
			ErrorMessage: "Amount must be positive",
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.Currency == "" {
		req.Currency = "USD" // Default currency
	}

	b.mu.Lock()
	b.stats.TotalEvaluations++
	b.mu.Unlock()

	// Evaluate routing rules
	processor, rule, confidence := b.evaluateRules(&req)

	// Record statistics
	b.mu.Lock()
	if rule != nil {
		b.stats.RuleHits[rule.Name]++
	}
	b.stats.ProcessorDistribution[processor]++

	// Update average evaluation time
	evalTime := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds
	if b.stats.AverageEvalTime == 0 {
		b.stats.AverageEvalTime = evalTime
	} else {
		b.stats.AverageEvalTime = (b.stats.AverageEvalTime + evalTime) / 2
	}
	b.mu.Unlock()

	// Build response
	response := EvaluationResponse{
		Success:         true,
		TargetProcessor: processor,
		Confidence:      confidence,
		EvaluationTime:  evalTime,
	}

	if rule != nil {
		response.RuleMatched = rule.Name
		response.RulePriority = rule.Priority
	}

	// Add alternatives for transparency
	response.Alternatives = b.getAlternatives(&req, processor)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *BPASService) evaluateRules(req *EvaluationRequest) (string, *RoutingRule, float64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Check rules in priority order
	for _, rule := range b.rules {
		if !rule.IsActive {
			continue
		}

		if b.matchesRule(req, &rule) {
			// For percentage-based rules, apply the percentage check
			if rule.ConditionType == "percentage" {
				if rand.Intn(100) < rule.Percentage {
					return rule.TargetProcessor, &rule, 1.0
				}
				continue // Try next rule if percentage doesn't match
			}

			// For other rule types, return immediately if matched
			confidence := b.calculateConfidence(&rule, req)
			return rule.TargetProcessor, &rule, confidence
		}
	}

	// Fallback to processor_a if no rules match
	return "processor_a", nil, 0.5
}

func (b *BPASService) matchesRule(req *EvaluationRequest, rule *RoutingRule) bool {
	switch rule.ConditionType {
	case "amount_threshold":
		return b.matchesAmountThreshold(req, rule)
	case "currency":
		return b.matchesCurrency(req, rule)
	case "marketplace":
		return b.matchesMarketplace(req, rule)
	case "user_tier":
		return b.matchesUserTier(req, rule)
	case "percentage":
		return true // Always matches, but percentage is applied in evaluateRules
	case "client_id":
		return b.matchesClientID(req, rule)
	default:
		return false
	}
}

func (b *BPASService) matchesAmountThreshold(req *EvaluationRequest, rule *RoutingRule) bool {
	amount, ok := rule.ConditionValue["amount"].(float64)
	if !ok {
		return false
	}

	operator, ok := rule.ConditionValue["operator"].(string)
	if !ok {
		operator = "greater_than"
	}

	switch operator {
	case "greater_than":
		return req.Amount > amount
	case "less_than":
		return req.Amount < amount
	case "equals":
		return req.Amount == amount
	case "greater_equal":
		return req.Amount >= amount
	case "less_equal":
		return req.Amount <= amount
	default:
		return false
	}
}

func (b *BPASService) matchesCurrency(req *EvaluationRequest, rule *RoutingRule) bool {
	currencies, ok := rule.ConditionValue["currencies"].([]interface{})
	if !ok {
		return false
	}

	for _, currency := range currencies {
		if currencyStr, ok := currency.(string); ok && currencyStr == req.Currency {
			return true
		}
	}
	return false
}

func (b *BPASService) matchesMarketplace(req *EvaluationRequest, rule *RoutingRule) bool {
	marketplaces, ok := rule.ConditionValue["marketplaces"].([]interface{})
	if !ok {
		return false
	}

	for _, marketplace := range marketplaces {
		if marketplaceStr, ok := marketplace.(string); ok && marketplaceStr == req.Marketplace {
			return true
		}
	}
	return false
}

func (b *BPASService) matchesUserTier(req *EvaluationRequest, rule *RoutingRule) bool {
	tiers, ok := rule.ConditionValue["tiers"].([]interface{})
	if !ok {
		return false
	}

	for _, tier := range tiers {
		if tierStr, ok := tier.(string); ok && tierStr == req.UserTier {
			return true
		}
	}
	return false
}

func (b *BPASService) matchesClientID(req *EvaluationRequest, rule *RoutingRule) bool {
	clientIDs, ok := rule.ConditionValue["client_ids"].([]interface{})
	if !ok {
		return false
	}

	for _, clientID := range clientIDs {
		if clientIDStr, ok := clientID.(string); ok && clientIDStr == req.ClientID {
			return true
		}
	}
	return false
}

func (b *BPASService) calculateConfidence(rule *RoutingRule, req *EvaluationRequest) float64 {
	// Simple confidence calculation based on rule specificity
	confidence := 0.5

	if rule.ConditionType == "amount_threshold" {
		confidence = 0.9
	} else if rule.ConditionType == "currency" {
		confidence = 0.8
	} else if rule.ConditionType == "marketplace" {
		confidence = 0.7
	} else if rule.ConditionType == "percentage" {
		confidence = 0.6
	}

	return confidence
}

func (b *BPASService) getAlternatives(req *EvaluationRequest, selectedProcessor string) []Alternative {
	alternatives := []Alternative{}

	if selectedProcessor != "processor_a" {
		alternatives = append(alternatives, Alternative{
			Processor: "processor_a",
			Weight:    0.7,
			Reason:    "Primary processor with faster response time",
		})
	}

	if selectedProcessor != "processor_b" {
		alternatives = append(alternatives, Alternative{
			Processor: "processor_b",
			Weight:    0.3,
			Reason:    "Secondary processor with multi-currency support",
		})
	}

	return alternatives
}

func (b *BPASService) getRules(w http.ResponseWriter, r *http.Request) {
	b.mu.RLock()
	rules := make([]RoutingRule, len(b.rules))
	copy(rules, b.rules)
	lastReload := b.stats.LastConfigReload
	b.mu.RUnlock()

	activeCount := 0
	for _, rule := range rules {
		if rule.IsActive {
			activeCount++
		}
	}

	response := RulesListResponse{
		Rules:       rules,
		TotalRules:  len(rules),
		ActiveRules: activeCount,
		LastReload:  lastReload,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *BPASService) reloadConfig(w http.ResponseWriter, r *http.Request) {
	if err := b.loadConfig(); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	b.mu.RLock()
	reloadCount := b.stats.ConfigReloadCount
	ruleCount := len(b.rules)
	b.mu.RUnlock()

	response := map[string]interface{}{
		"success":      true,
		"message":      "Configuration reloaded successfully",
		"rules_loaded": ruleCount,
		"reload_count": reloadCount,
		"timestamp":    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *BPASService) updateRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ruleName := vars["name"]

	var updatedRule RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&updatedRule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Find and update the rule
	for i, rule := range b.rules {
		if rule.Name == ruleName {
			updatedRule.Name = ruleName // Ensure name doesn't change
			updatedRule.UpdatedAt = time.Now()
			b.rules[i] = updatedRule

			// Re-sort rules by priority
			sort.Slice(b.rules, func(i, j int) bool {
				return b.rules[i].Priority < b.rules[j].Priority
			})

			response := map[string]interface{}{
				"success":   true,
				"message":   "Rule updated successfully",
				"rule":      updatedRule,
				"timestamp": time.Now(),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	http.Error(w, "Rule not found", http.StatusNotFound)
}

func (b *BPASService) getStats(w http.ResponseWriter, r *http.Request) {
	b.mu.RLock()
	stats := b.stats
	rulesCount := len(b.rules)
	b.mu.RUnlock()

	response := map[string]interface{}{
		"service_name": "bpas_service",
		"stats":        stats,
		"total_rules":  rulesCount,
		"timestamp":    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (b *BPASService) health(w http.ResponseWriter, r *http.Request) {
	b.mu.RLock()
	rulesCount := len(b.rules)
	lastReload := b.stats.LastConfigReload
	b.mu.RUnlock()

	response := map[string]interface{}{
		"service":            "bpas-service",
		"status":             "healthy",
		"timestamp":          time.Now(),
		"version":            "1.0.0",
		"rules_loaded":       rulesCount,
		"last_config_reload": lastReload,
		"capabilities":       []string{"dynamic_routing", "rule_evaluation", "config_reload", "percentage_splits"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Test endpoint for rule evaluation
func (b *BPASService) testRule(w http.ResponseWriter, r *http.Request) {
	amountStr := r.URL.Query().Get("amount")
	currency := r.URL.Query().Get("currency")
	marketplace := r.URL.Query().Get("marketplace")

	amount := 100.0
	if amountStr != "" {
		if parsed, err := strconv.ParseFloat(amountStr, 64); err == nil {
			amount = parsed
		}
	}

	if currency == "" {
		currency = "USD"
	}

	req := EvaluationRequest{
		Amount:      amount,
		Currency:    currency,
		Marketplace: marketplace,
	}

	processor, rule, confidence := b.evaluateRules(&req)

	response := map[string]interface{}{
		"test_input":       req,
		"result_processor": processor,
		"confidence":       confidence,
	}

	if rule != nil {
		response["matched_rule"] = rule.Name
		response["rule_priority"] = rule.Priority
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	configPath := "/app/configs"
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		configPath = path
	}

	service := NewBPASService(configPath)
	r := mux.NewRouter()

	// Core BPAS endpoints
	r.HandleFunc("/bpas/evaluate", service.evaluateRouting).Methods("POST")
	r.HandleFunc("/bpas/rules", service.getRules).Methods("GET")
	r.HandleFunc("/bpas/rules/{name}", service.updateRule).Methods("PUT")
	r.HandleFunc("/bpas/reload", service.reloadConfig).Methods("POST")

	// Testing endpoints
	r.HandleFunc("/bpas/test", service.testRule).Methods("GET")

	// Admin endpoints
	r.HandleFunc("/admin/stats", service.getStats).Methods("GET")

	// Health check
	r.HandleFunc("/health", service.health).Methods("GET")

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	log.Println("BPAS Service starting on port 8003")
	log.Printf("Configuration path: %s", configPath)
	log.Println("Core endpoints:")
	log.Println("   POST /bpas/evaluate")
	log.Println("   GET /bpas/rules")
	log.Println("   PUT /bpas/rules/{name}")
	log.Println("   POST /bpas/reload")
	log.Println("   GET /bpas/test?amount=1000&currency=EUR")

	log.Fatal(http.ListenAndServe(":8003", r))
}
