package processor

import (
	"context"
	"fmt"
	"time"
)

// ProcessorInterface defines the interface all processors must implement
type ProcessorInterface interface {
	Charge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error)
	Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
	Tokenize(ctx context.Context, req *TokenizeRequest) (*TokenizeResponse, error)
	Health(ctx context.Context) (*HealthResponse, error)
	GetStats(ctx context.Context) (*StatsResponse, error)
	IsHealthy(ctx context.Context) bool
	GetName() string
	GetBaseURL() string
}

// ProcessorConfig holds configuration for a processor
type ProcessorConfig struct {
	Name       string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// ProcessorFactory manages processor instances
type ProcessorFactory struct {
	processors map[string]ProcessorInterface
	configs    map[string]*ProcessorConfig
}

// NewProcessorFactory creates a new processor factory
func NewProcessorFactory() *ProcessorFactory {
	return &ProcessorFactory{
		processors: make(map[string]ProcessorInterface),
		configs:    make(map[string]*ProcessorConfig),
	}
}

// RegisterProcessor adds a processor configuration
func (f *ProcessorFactory) RegisterProcessor(config *ProcessorConfig) {
	f.configs[config.Name] = config
}

// GetProcessor returns a processor client by name, creating it if necessary
func (f *ProcessorFactory) GetProcessor(name string) (ProcessorInterface, error) {
	// Return existing processor if already created
	if processor, exists := f.processors[name]; exists {
		return processor, nil
	}

	// Get configuration
	config, exists := f.configs[name]
	if !exists {
		return nil, fmt.Errorf("processor %s not configured", name)
	}

	// Create new processor client
	client := NewClient(config.Name, config.BaseURL, config.Timeout)
	f.processors[name] = client

	return client, nil
}

// GetAllProcessors returns all configured processors
func (f *ProcessorFactory) GetAllProcessors() ([]ProcessorInterface, error) {
	var processors []ProcessorInterface

	for name := range f.configs {
		processor, err := f.GetProcessor(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get processor %s: %w", name, err)
		}
		processors = append(processors, processor)
	}

	return processors, nil
}

// GetProcessorNames returns a list of all configured processor names
func (f *ProcessorFactory) GetProcessorNames() []string {
	var names []string
	for name := range f.configs {
		names = append(names, name)
	}
	return names
}

// GetHealthyProcessors returns only the processors that are currently healthy
func (f *ProcessorFactory) GetHealthyProcessors(ctx context.Context) ([]ProcessorInterface, error) {
	allProcessors, err := f.GetAllProcessors()
	if err != nil {
		return nil, err
	}

	var healthyProcessors []ProcessorInterface
	for _, processor := range allProcessors {
		if processor.IsHealthy(ctx) {
			healthyProcessors = append(healthyProcessors, processor)
		}
	}

	return healthyProcessors, nil
}

// CheckAllHealth checks the health of all processors and returns a summary
func (f *ProcessorFactory) CheckAllHealth(ctx context.Context) (map[string]bool, error) {
	healthStatus := make(map[string]bool)

	for name := range f.configs {
		processor, err := f.GetProcessor(name)
		if err != nil {
			healthStatus[name] = false
			continue
		}

		healthStatus[name] = processor.IsHealthy(ctx)
	}

	return healthStatus, nil
}

// GetProcessorStats returns statistics for all processors
func (f *ProcessorFactory) GetProcessorStats(ctx context.Context) (map[string]*StatsResponse, error) {
	stats := make(map[string]*StatsResponse)

	for name := range f.configs {
		processor, err := f.GetProcessor(name)
		if err != nil {
			continue
		}

		processorStats, err := processor.GetStats(ctx)
		if err != nil {
			continue
		}

		stats[name] = processorStats
	}

	return stats, nil
}

// DefaultProcessorFactory creates a factory with default processor configurations
func DefaultProcessorFactory() *ProcessorFactory {
	factory := NewProcessorFactory()

	// Register Processor A (Primary)
	factory.RegisterProcessor(&ProcessorConfig{
		Name:       "processor_a",
		BaseURL:    "http://mock-processor-a:8101",
		Timeout:    5 * time.Second,
		MaxRetries: 2,
	})

	// Register Processor B (Secondary/Backup)
	factory.RegisterProcessor(&ProcessorConfig{
		Name:       "processor_b",
		BaseURL:    "http://mock-processor-b:8102",
		Timeout:    5 * time.Second,
		MaxRetries: 2,
	})

	return factory
}

// ProcessorFromConfig creates a processor factory from environment or config
func ProcessorFromConfig(processorAURL, processorBURL string) *ProcessorFactory {
	factory := NewProcessorFactory()

	if processorAURL != "" {
		factory.RegisterProcessor(&ProcessorConfig{
			Name:       "processor_a",
			BaseURL:    processorAURL,
			Timeout:    5 * time.Second,
			MaxRetries: 2,
		})
	}

	if processorBURL != "" {
		factory.RegisterProcessor(&ProcessorConfig{
			Name:       "processor_b",
			BaseURL:    processorBURL,
			Timeout:    5 * time.Second,
			MaxRetries: 2,
		})
	}

	return factory
}
