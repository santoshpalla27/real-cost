// Package api defines estimation result types.
package api

// EstimationResult is the final output of the estimation pipeline.
type EstimationResult struct {
	TotalCost       CostRange        `json:"total_cost"`
	TotalCarbon     CarbonEstimate   `json:"total_carbon"`
	Drivers         []CostDriver     `json:"drivers"`
	ConfidenceScore float64          `json:"confidence_score"`
	IsIncomplete    bool             `json:"is_incomplete"`
	Errors          []EstimationError `json:"errors,omitempty"`
	Lineage         []LineageItem    `json:"lineage,omitempty"`
}

// CostRange represents cost with uncertainty bounds.
type CostRange struct {
	P50      float64 `json:"p50"`
	P90      float64 `json:"p90"`
	Currency string  `json:"currency"`
}

// CarbonEstimate represents carbon footprint.
type CarbonEstimate struct {
	KgCO2e     float64 `json:"kg_co2e"`
	Confidence float64 `json:"confidence"`
	Region     string  `json:"region"`
}

// CostDriver represents a major cost contributor.
type CostDriver struct {
	ResourceID  string  `json:"resource_id"`
	Name        string  `json:"name"`
	MonthlyCost float64 `json:"monthly_cost"`
	Percentage  float64 `json:"percentage"`
}

// EstimationError records issues during estimation.
type EstimationError struct {
	ResourceID  string `json:"resource_id"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// LineageItem provides explainability for a cost decision.
type LineageItem struct {
	ResourceID  string `json:"resource_id"`
	Component   string `json:"component"`
	SKU         string `json:"sku"`
	Price       float64 `json:"price"`
	Unit        string `json:"unit"`
	Quantity    float64 `json:"quantity"`
	MonthlyCost float64 `json:"monthly_cost"`
	Explanation string `json:"explanation"`
}
