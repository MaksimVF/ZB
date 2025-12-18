package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// ModelSpecificStrategy selects based on model-specific criteria
type ModelSpecificStrategy struct{}

// NewModelSpecificStrategy creates a new model-specific strategy
func NewModelSpecificStrategy() *ModelSpecificStrategy {
	return &ModelSpecificStrategy{}
}

// Name returns the strategy name
func (s *ModelSpecificStrategy) Name() string {
	return "model_specific"
}

// SelectHead selects based on model-specific criteria from metadata
func (s *ModelSpecificStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// For now, just return the first head
	// In production, this would use model-specific criteria from metadata
	return &heads[0]
}