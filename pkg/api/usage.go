// Package api defines usage prediction types.
package api

// UsagePrediction represents predicted resource usage with uncertainty.
type UsagePrediction struct {
	ComponentID string   `json:"component_id"`
	Metric      string   `json:"metric"`
	Unit        string   `json:"unit"`
	P50         float64  `json:"p50"`
	P90         float64  `json:"p90"`
	Confidence  float64  `json:"confidence"`
	Assumptions []string `json:"assumptions"`
}

// UsageResult holds all predictions for a request.
type UsageResult struct {
	Predictions       []UsagePrediction `json:"predictions"`
	AverageConfidence float64           `json:"average_confidence"`
	Environment       string            `json:"environment"`
}
