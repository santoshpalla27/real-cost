package service

import (
	"github.com/futuristic-iac/pkg/api"
)

func Predict(req api.UsageForecastRequest) (*api.UsageForecast, error) {
	// Simple heuristic: 730 hours/month for everything unless specified
	// In reality, this would query Prometheus or CloudWatch history.
	
	usage := 730.0 // Default for always-on compute
	metric := "hours"
	confidence := 0.8
	
	if req.Component.Provider == "aws" {
		if req.Component.UsageType == "compute" {
			// EC2: Time-based
			usage = 730.0
			metric = "hours"
			confidence = 0.95
		} else if req.Component.UsageType == "storage" {
			// EBS: Size-based (allocated static), priced in GB-Mo.
			// The Estimator expects "units" to be multiplied by price.
			// Price is per GB-Mo. Size is in attributes.
			// If we return usage=1.0 "months", Estimator will do size * 1.0 = size.
			
			usage = 1.0
			metric = "months"
			confidence = 0.9
		}
	}

	return &api.UsageForecast{
		MonthlyForecast: struct {
			P50 float64 `json:"p50"`
			P90 float64 `json:"p90"`
		}{
			P50: usage,
			P90: usage, // Flat forecast for MVP
		},
		Metric:     metric, 
		Confidence: confidence,
	}, nil
}
