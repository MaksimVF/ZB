package models

import (
	"encoding/json"
	"math"
	"time"
)

// PricingType represents pricing structure for a model
type PricingType struct {
	Input  float64 `json:"input" redis:"input"`   // Cost per input token
	Output float64 `json:"output" redis:"output"` // Cost per output token
	Embed  float64 `json:"embed" redis:"embed"`   // Cost per embed token
}

// PricingModel represents a model's pricing information
type PricingModel struct {
	ID        string                 `json:"id" redis:"id"`
	Name      string                 `json:"name" redis:"name"`
	Provider  string                 `json:"provider" redis:"provider"`
	Pricing   PricingType            `json:"pricing" redis:"pricing"`
	Active    bool                   `json:"active" redis:"active"`
	CreatedAt time.Time              `json:"created_at" redis:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" redis:"updated_at"`
	Source    string                 `json:"source" redis:"source"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" redis:"metadata,omitempty"`
}

// GetTokenPrice returns the price for a specific token type
func (p *PricingModel) GetTokenPrice(tokenType string) float64 {
	switch tokenType {
	case "input":
		return p.Pricing.Input
	case "output":
		return p.Pricing.Output
	case "embed":
		return p.Pricing.Embed
	default:
		return 0.0
	}
}

// CalculateCost calculates the total cost for given token usage
func (p *PricingModel) CalculateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) * p.Pricing.Input
	outputCost := float64(outputTokens) * p.Pricing.Output
	return roundToCents(inputCost + outputCost)
}

// Validate validates pricing model data
func (p *PricingModel) Validate() error {
	if p.ID == "" {
		return ErrRequired("ID")
	}
	if p.Name == "" {
		return ErrRequired("Name")
	}
	if p.Provider == "" {
		return ErrRequired("Provider")
	}
	if p.Pricing.Input < 0 || p.Pricing.Output < 0 || p.Pricing.Embed < 0 {
		return ErrInvalid("Pricing values must be non-negative")
	}
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler
func (p *PricingModel) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (p *PricingModel) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}

// PricingInfo represents all pricing information
type PricingInfo struct {
	Models      []string                 `json:"models"`
	Pricing     map[string]*PricingModel `json:"pricing"`
	LastUpdated time.Time                `json:"last_updated"`
	Source      string                   `json:"source"`
	TotalModels int                      `json:"total_models"`
}

// UpdatePricingRequest represents a request to update pricing
type UpdatePricingRequest struct {
	ModelID   string      `json:"model_id" binding:"required"`
	Name      string      `json:"name" binding:"required"`
	Provider  string      `json:"provider" binding:"required"`
	Pricing   PricingType `json:"pricing" binding:"required"`
	Active    bool        `json:"active" binding:"required"`
	Source    string      `json:"source"`
	UpdatedBy string      `json:"updated_by"`
}

// BulkUpdatePricingRequest represents a request to update multiple pricing models
type BulkUpdatePricingRequest struct {
	Updates   map[string]PricingUpdate `json:"updates" binding:"required"`
	Source    string                   `json:"source"`
	UpdatedBy string                   `json:"updated_by"`
}

// PricingUpdate represents a pricing update for a single model
type PricingUpdate struct {
	Name     string      `json:"name"`
	Provider string      `json:"provider"`
	Pricing  PricingType `json:"pricing"`
	Active   bool        `json:"active"`
}

// BulkUpdatePricingResponse represents the response from a bulk pricing update
type BulkUpdatePricingResponse struct {
	TotalModels  int                         `json:"total_models"`
	SuccessCount int                         `json:"success_count"`
	ErrorCount   int                         `json:"error_count"`
	Results      map[string]BulkUpdateResult `json:"results"`
	UpdatedAt    time.Time                   `json:"updated_at"`
	Source       string                      `json:"source"`
}

// BulkUpdateResult represents the result of updating a single pricing model
type BulkUpdateResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CalculateCostRequest represents a request to calculate cost
type CalculateCostRequest struct {
	ModelID      string `json:"model_id" binding:"required"`
	InputTokens  int    `json:"input_tokens" binding:"required,min=0"`
	OutputTokens int    `json:"output_tokens" binding:"required,min=0"`
}

// CalculateCostResponse represents the response with cost calculation
type CalculateCostResponse struct {
	ModelID      string  `json:"model_id"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
}

// ProvidersResponse represents available providers and models
type ProvidersResponse struct {
	Providers      map[string][]string `json:"providers"`
	TotalProviders int                 `json:"total_providers"`
	TotalModels    int                 `json:"total_models"`
}

// PricingHistoryEntry represents a pricing change history entry
type PricingHistoryEntry struct {
	ModelID    string      `json:"model_id"`
	OldPricing PricingType `json:"old_pricing"`
	NewPricing PricingType `json:"new_pricing"`
	UpdatedBy  string      `json:"updated_by"`
	Timestamp  time.Time   `json:"timestamp"`
	Source     string      `json:"source"`
	Reason     string      `json:"reason,omitempty"`
}

// ExternalPricingData represents pricing data from external source
type ExternalPricingData struct {
	Source string          `json:"source"`
	Models []ExternalModel `json:"models"`
}

// ExternalModel represents a model from external source
type ExternalModel struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Provider string      `json:"provider"`
	Pricing  PricingType `json:"pricing"`
	Active   bool        `json:"active"`
}

// PricingSearchRequest represents a search request for pricing models
type PricingSearchRequest struct {
	Provider string  `json:"provider"`
	Active   *bool   `json:"active"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
	Limit    int     `json:"limit"`
	Offset   int     `json:"offset"`
}

// PricingSearchResponse represents the response to a pricing search
type PricingSearchResponse struct {
	Models  []*PricingModel `json:"models"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"has_more"`
}

// PricingNotification represents a notification about pricing changes
type PricingNotification struct {
	Type       string        `json:"type"` // "created", "updated", "deleted"
	ModelID    string        `json:"model_id"`
	Pricing    *PricingModel `json:"pricing,omitempty"`
	OldPricing *PricingModel `json:"old_pricing,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// PricingComparison represents a comparison between two pricing models
type PricingComparison struct {
	Model1        *PricingModel `json:"model1"`
	Model2        *PricingModel `json:"model2"`
	InputDiff     float64       `json:"input_diff"`
	OutputDiff    float64       `json:"output_diff"`
	EmbedDiff     float64       `json:"embed_diff"`
	InputPercent  float64       `json:"input_percent"`
	OutputPercent float64       `json:"output_percent"`
	EmbedPercent  float64       `json:"embed_percent"`
}

// PricingStatistics represents pricing-related statistics
type PricingStatistics struct {
	TotalModels    int                   `json:"total_models"`
	ActiveModels   int                   `json:"active_models"`
	Providers      map[string]int        `json:"providers"`
	AvgInputPrice  float64               `json:"avg_input_price"`
	AvgOutputPrice float64               `json:"avg_output_price"`
	AvgEmbedPrice  float64               `json:"avg_embed_price"`
	MostExpensive  *PricingModel         `json:"most_expensive"`
	Cheapest       *PricingModel         `json:"cheapest"`
	RecentChanges  []PricingHistoryEntry `json:"recent_changes"`
	GeneratedAt    time.Time             `json:"generated_at"`
}

// Helper functions

// roundToCents rounds a float to the nearest cent
func roundToCents(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// IsActive returns true if the pricing model is active
func (p *PricingModel) IsActive() bool {
	return p.Active
}

// IsProvider checks if the model is from a specific provider
func (p *PricingModel) IsProvider(provider string) bool {
	return p.Provider == provider
}

// GetTotalPrice returns the sum of all pricing values
func (p *PricingModel) GetTotalPrice() float64 {
	return p.Pricing.Input + p.Pricing.Output + p.Pricing.Embed
}

// ComparePrices compares this pricing model with another
func (p *PricingModel) ComparePrices(other *PricingModel) *PricingComparison {
	if other == nil {
		return nil
	}

	inputDiff := p.Pricing.Input - other.Pricing.Input
	outputDiff := p.Pricing.Output - other.Pricing.Output
	embedDiff := p.Pricing.Embed - other.Pricing.Embed

	var inputPercent, outputPercent, embedPercent float64
	if other.Pricing.Input > 0 {
		inputPercent = (inputDiff / other.Pricing.Input) * 100
	}
	if other.Pricing.Output > 0 {
		outputPercent = (outputDiff / other.Pricing.Output) * 100
	}
	if other.Pricing.Embed > 0 {
		embedPercent = (embedDiff / other.Pricing.Embed) * 100
	}

	return &PricingComparison{
		Model1:        p,
		Model2:        other,
		InputDiff:     roundToCents(inputDiff),
		OutputDiff:    roundToCents(outputDiff),
		EmbedDiff:     roundToCents(embedDiff),
		InputPercent:  roundToCents(inputPercent),
		OutputPercent: roundToCents(outputPercent),
		EmbedPercent:  roundToCents(embedPercent),
	}
}

// CreatePricing creates a new pricing model
func CreatePricing(id, name, provider string, pricing PricingType) *PricingModel {
	return &PricingModel{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Pricing:   pricing,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    "manual",
	}
}

// UpdatePricing updates an existing pricing model
func (p *PricingModel) UpdatePricing(name, provider string, pricing PricingType, active bool) {
	p.Name = name
	p.Provider = provider
	p.Pricing = pricing
	p.Active = active
	p.UpdatedAt = time.Now()
}

// String returns a string representation of the pricing model
func (p *PricingModel) String() string {
	return p.ID
}

// Equals compares two pricing models
func (p *PricingModel) Equals(other *PricingModel) bool {
	if other == nil {
		return false
	}
	return p.ID == other.ID &&
		p.Name == other.Name &&
		p.Provider == other.Provider &&
		p.Pricing.Input == other.Pricing.Input &&
		p.Pricing.Output == other.Pricing.Output &&
		p.Pricing.Embed == other.Pricing.Embed &&
		p.Active == other.Active
}

// PricingDiff represents the difference between two pricing states
type PricingDiff struct {
	ModelID    string      `json:"model_id"`
	Field      string      `json:"field"`
	OldValue   interface{} `json:"old_value"`
	NewValue   interface{} `json:"new_value"`
	ChangeType string      `json:"change_type"` // "added", "removed", "modified"
}

// GetPricingDiff returns the differences between two pricing models
func (p *PricingModel) GetPricingDiff(old *PricingModel) []PricingDiff {
	var diffs []PricingDiff

	if old == nil {
		// New pricing model - all fields are added
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "model",
			OldValue:   nil,
			NewValue:   p,
			ChangeType: "added",
		})
		return diffs
	}

	// Compare ID
	if p.ID != old.ID {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "id",
			OldValue:   old.ID,
			NewValue:   p.ID,
			ChangeType: "modified",
		})
	}

	// Compare Name
	if p.Name != old.Name {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "name",
			OldValue:   old.Name,
			NewValue:   p.Name,
			ChangeType: "modified",
		})
	}

	// Compare Provider
	if p.Provider != old.Provider {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "provider",
			OldValue:   old.Provider,
			NewValue:   p.Provider,
			ChangeType: "modified",
		})
	}

	// Compare Active
	if p.Active != old.Active {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "active",
			OldValue:   old.Active,
			NewValue:   p.Active,
			ChangeType: "modified",
		})
	}

	// Compare Pricing
	if p.Pricing.Input != old.Pricing.Input {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "pricing.input",
			OldValue:   old.Pricing.Input,
			NewValue:   p.Pricing.Input,
			ChangeType: "modified",
		})
	}

	if p.Pricing.Output != old.Pricing.Output {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "pricing.output",
			OldValue:   old.Pricing.Output,
			NewValue:   p.Pricing.Output,
			ChangeType: "modified",
		})
	}

	if p.Pricing.Embed != old.Pricing.Embed {
		diffs = append(diffs, PricingDiff{
			ModelID:    p.ID,
			Field:      "pricing.embed",
			OldValue:   old.Pricing.Embed,
			NewValue:   p.Pricing.Embed,
			ChangeType: "modified",
		})
	}

	return diffs
}
