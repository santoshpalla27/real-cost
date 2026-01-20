package api

import (
	"github.com/futuristic-iac/pkg/focus"
)

// BillingComponent represents an atomic billable unit derived from a Resource.
// It encodes financial intent and variance.
type BillingComponent struct {
	ResourceAddress string   `json:"resource_address"`
	ComponentType   string   `json:"component_type"` // compute, storage, network, license
	Provider        string   `json:"provider"`
	UsageType       string   `json:"usage_type"`       // on_demand, spot, reserved
	Lifecycle       string   `json:"lifecycle"`        // persistent, ephemeral
	VarianceProfile string   `json:"variance_profile"` // static, autoscaled, request_driven
	Dependencies    []string `json:"dependencies"`     // IDs of other BillingComponents
	
	// LookupAttributes are used by the Pricing Engine to find the matching SKU.
	LookupAttributes map[string]string `json:"lookup_attributes"`

	// Resolved Pricing (filled by Estimation Core)
	PricingRef *focus.PricingItem `json:"pricing_ref,omitempty"`

	// Mapping Metadata
	MappingError string `json:"mapping_error,omitempty"` // If mapping failed/incomplete
}

// UsageForecastRequest is the input to the Usage Engine.
type UsageForecastRequest struct {
	ResourceID   string `json:"resource_id"`
	Class        string `json:"class"` // api, batch, stateful
	Environment  string `json:"env"`   // prod, dev, staging
	ResourceShape string `json:"shape"` // e.g. t3.medium
}

// UsageForecast represents the predicted usage for a specific component.
type UsageForecast struct {
	ResourceAddress string `json:"resource_address"`
	Metric          string `json:"metric"` // e.g., "cpu_hours", "gb_months"
	
	MonthlyForecast struct {
		P50 float64 `json:"p50"`
		P90 float64 `json:"p90"`
	} `json:"monthly_forecast"`

	Confidence  float64  `json:"confidence"`
	Assumptions []string `json:"assumptions"`
}

// EstimationResult represents the final cost and carbon output.
type EstimationResult struct {
	TotalMonthlyCost struct {
		P50 float64 `json:"p50"`
		P90 float64 `json:"p90"`
	} `json:"total_monthly_cost"`

	Carbon struct {
		KgCO2e          float64 `json:"kgco2e"`
		RegionIntensity string  `json:"region_intensity"` // low, medium, high
	} `json:"carbon"`

	Drivers []CostDriver `json:"drivers"`

	// Safety & Correctness Fields
	ConfidenceScore float64           `json:"confidence_score"` // 0.0 to 1.0
	IsIncomplete    bool              `json:"is_incomplete"`    // True if any component failed usage/pricing/mapping
	Errors          []string          `json:"errors"`           // Deprecated: simplistic list
	DetailedErrors  []EstimationError `json:"detailed_errors"`  // Structured errors
}

// EstimationError provides detailed context on failures
type EstimationError struct {
	Component string `json:"component"`
	Severity  string `json:"severity"` // critical | warning
	Message   string `json:"message"`
}

const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
)

// CostDriver explains a single line item contribution.
type CostDriver struct {
	Component string  `json:"component"` // e.g. "aws_instance.web.compute"
	Reason    string  `json:"reason"`    // e.g. "on_demand_prod"
	P50Cost   float64 `json:"p50_cost"`
	P90Cost   float64 `json:"p90_cost"`
	Formula   string  `json:"formula"`   // e.g. "730h * $0.05/h"
}
