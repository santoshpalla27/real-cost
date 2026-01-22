// Package api defines billing component types.
package api

// BillingComponent represents an atomic billable unit.
type BillingComponent struct {
	ID              string          `json:"id"`
	ResourceID      string          `json:"resource_id"`
	Type            ComponentType   `json:"type"`
	UsageType       UsageType       `json:"usage_type"`
	Lifecycle       Lifecycle       `json:"lifecycle"`
	VarianceProfile VarianceProfile `json:"variance_profile"`
	Dependencies    []string        `json:"dependencies,omitempty"`
	Attributes      map[string]any  `json:"attributes"`
	MappingError    *MappingError   `json:"mapping_error,omitempty"`
}

// ComponentType represents the billing category.
type ComponentType string

const (
	ComponentTypeCompute ComponentType = "compute"
	ComponentTypeStorage ComponentType = "storage"
	ComponentTypeNetwork ComponentType = "network"
	ComponentTypeData    ComponentType = "data"
	ComponentTypeOther   ComponentType = "other"
)

// UsageType represents how the resource is billed.
type UsageType string

const (
	UsageTypeOnDemand  UsageType = "on_demand"
	UsageTypeReserved  UsageType = "reserved"
	UsageTypeSpot      UsageType = "spot"
	UsageTypeSavings   UsageType = "savings_plan"
	UsageTypeProvisioned UsageType = "provisioned"
)

// Lifecycle represents billing frequency.
type Lifecycle string

const (
	LifecycleHourly  Lifecycle = "hourly"
	LifecycleMonthly Lifecycle = "monthly"
	LifecycleOneTime Lifecycle = "one_time"
	LifecyclePerUnit Lifecycle = "per_unit"
)

// VarianceProfile describes expected usage variability.
type VarianceProfile struct {
	Pattern    string  `json:"pattern"`    // stable, bursty, predictable
	Seasonality string `json:"seasonality"` // none, daily, weekly, monthly
	Confidence float64 `json:"confidence"`
}

// MappingError indicates a resource couldn't be fully mapped.
type MappingError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}


// SemanticResult holds the output of semantic processing.
type SemanticResult struct {
	Components    []BillingComponent `json:"components"`
	MappingErrors []MappingError     `json:"mapping_errors,omitempty"`
}
