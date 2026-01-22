// Package api defines the shared request/response contracts for all services.
package api

// ParseRequest is the input for the ingestion service.
type ParseRequest struct {
	PlanJSON []byte `json:"plan_json"`
}

// SemanticRequest is the input for the semantic engine.
type SemanticRequest struct {
	Graph *InfrastructureGraph `json:"graph"`
}

// UsageRequest is the input for the usage prediction engine.
type UsageRequest struct {
	Components  []BillingComponent `json:"components"`
	Environment string             `json:"environment"`
}

// PriceRequest is the input for the pricing engine.
type PriceRequest struct {
	Components    []BillingComponent `json:"components"`
	Region        string             `json:"region"`
	EffectiveDate *string            `json:"effective_date,omitempty"`
}

// EstimateRequest combines all inputs for full estimation.
type EstimateRequest struct {
	PlanJSON    []byte `json:"plan_json,omitempty"`
	Graph       *InfrastructureGraph `json:"graph,omitempty"`
	Environment string `json:"environment"`
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
