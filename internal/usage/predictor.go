// Package usage provides predictive usage estimation with uncertainty.
package usage

import (
	"github.com/santoshpalla27/fiac-platform/pkg/api"
	"github.com/santoshpalla27/fiac-platform/pkg/confidence"
)

// Predictor generates usage predictions for billing components.
type Predictor struct {
	profiles map[string]UsageProfile
}

// UsageProfile defines expected usage patterns for an environment.
type UsageProfile struct {
	Name              string
	UtilizationFactor float64 // 0-1, percentage of capacity used
	GrowthFactor      float64 // Monthly growth rate
	VarianceFactor    float64 // How much P90 differs from P50
	BaseConfidence    float64
}

// NewPredictor creates a predictor with default profiles.
func NewPredictor() *Predictor {
	return &Predictor{
		profiles: map[string]UsageProfile{
			"dev": {
				Name:              "Development",
				UtilizationFactor: 0.2,  // 20% utilization
				GrowthFactor:      0.0,  // No growth assumption
				VarianceFactor:    1.3,  // 30% variance
				BaseConfidence:    0.7,
			},
			"staging": {
				Name:              "Staging",
				UtilizationFactor: 0.4,
				GrowthFactor:      0.05,
				VarianceFactor:    1.4,
				BaseConfidence:    0.65,
			},
			"prod": {
				Name:              "Production",
				UtilizationFactor: 0.7,
				GrowthFactor:      0.1,
				VarianceFactor:    1.5,
				BaseConfidence:    0.6,
			},
		},
	}
}

// Predict generates usage predictions for components.
func (p *Predictor) Predict(components []api.BillingComponent, environment string) *api.UsageResult {
	profile, exists := p.profiles[environment]
	if !exists {
		profile = p.profiles["dev"] // Conservative default
	}

	result := &api.UsageResult{
		Predictions: []api.UsagePrediction{},
		Environment: environment,
	}

	var confidenceSum float64
	for _, comp := range components {
		prediction := p.predictComponent(comp, profile)
		result.Predictions = append(result.Predictions, prediction)
		confidenceSum += prediction.Confidence
	}

	if len(result.Predictions) > 0 {
		result.AverageConfidence = confidenceSum / float64(len(result.Predictions))
	}

	return result
}

func (p *Predictor) predictComponent(comp api.BillingComponent, profile UsageProfile) api.UsagePrediction {
	prediction := api.UsagePrediction{
		ComponentID: comp.ID,
		Assumptions: []string{},
	}

	// Apply base confidence from variance profile
	baseConf := comp.VarianceProfile.Confidence
	if baseConf == 0 {
		baseConf = profile.BaseConfidence
	}

	// Combine with profile confidence
	prediction.Confidence = confidence.Aggregate([]float64{baseConf, profile.BaseConfidence})

	switch comp.Type {
	case api.ComponentTypeCompute:
		prediction = p.predictCompute(comp, profile, prediction)
	case api.ComponentTypeStorage:
		prediction = p.predictStorage(comp, profile, prediction)
	case api.ComponentTypeNetwork:
		prediction = p.predictNetwork(comp, profile, prediction)
	case api.ComponentTypeData:
		prediction = p.predictData(comp, profile, prediction)
	default:
		prediction.Metric = "units"
		prediction.Unit = "units"
		prediction.P50 = 1
		prediction.P90 = 1
		prediction.Confidence = confidence.LowConfidence
		prediction.Assumptions = append(prediction.Assumptions, "unknown-component-type")
	}

	return prediction
}

func (p *Predictor) predictCompute(comp api.BillingComponent, profile UsageProfile, pred api.UsagePrediction) api.UsagePrediction {
	pred.Metric = "hours"
	pred.Unit = "hours/month"

	// Base: 730 hours/month (full month)
	hoursPerMonth := 730.0

	switch comp.UsageType {
	case api.UsageTypeOnDemand:
		// Apply utilization factor
		pred.P50 = hoursPerMonth * profile.UtilizationFactor
		pred.P90 = hoursPerMonth * profile.UtilizationFactor * profile.VarianceFactor
		pred.Assumptions = append(pred.Assumptions, 
			"base-730-hours",
			"utilization-"+profile.Name,
		)
	case api.UsageTypeReserved:
		// Reserved instances run full time
		pred.P50 = hoursPerMonth
		pred.P90 = hoursPerMonth
		pred.Confidence = confidence.HighConfidence
		pred.Assumptions = append(pred.Assumptions, "reserved-full-utilization")
	case api.UsageTypeSpot:
		// Spot instances have interruption risk
		pred.P50 = hoursPerMonth * 0.85
		pred.P90 = hoursPerMonth * 0.95
		pred.Confidence = confidence.Decay(pred.Confidence, 1)
		pred.Assumptions = append(pred.Assumptions, "spot-interruption-risk")
	default:
		pred.P50 = hoursPerMonth
		pred.P90 = hoursPerMonth
	}

	return pred
}

func (p *Predictor) predictStorage(comp api.BillingComponent, profile UsageProfile, pred api.UsagePrediction) api.UsagePrediction {
	pred.Metric = "gb_months"
	pred.Unit = "GB-months"

	// Storage is provisioned, so prediction is based on provisioned size
	size := getFloatAttr(comp.Attributes, "volume_size", 100)

	pred.P50 = size
	pred.P90 = size * (1 + profile.GrowthFactor) // Account for potential growth
	pred.Confidence = confidence.HighConfidence
	pred.Assumptions = append(pred.Assumptions, "provisioned-storage", "monthly-growth-possible")

	return pred
}

func (p *Predictor) predictNetwork(comp api.BillingComponent, profile UsageProfile, pred api.UsagePrediction) api.UsagePrediction {
	pred.Metric = "gb_transfer"
	pred.Unit = "GB/month"

	// Network is very hard to predict, use conservative estimates
	baseGB := 10.0 // 10 GB base

	switch profile.Name {
	case "Production":
		baseGB = 100.0
	case "Staging":
		baseGB = 50.0
	}

	pred.P50 = baseGB * profile.UtilizationFactor
	pred.P90 = baseGB * profile.UtilizationFactor * 2.0 // High variance
	pred.Confidence = confidence.LowConfidence
	pred.Assumptions = append(pred.Assumptions, 
		"network-heuristic",
		"high-variance-warning",
	)

	return pred
}

func (p *Predictor) predictData(comp api.BillingComponent, profile UsageProfile, pred api.UsagePrediction) api.UsagePrediction {
	pred.Metric = "requests"
	pred.Unit = "requests/month"

	// Data/request-based components are usage-dependent
	baseRequests := 10000.0

	switch profile.Name {
	case "Production":
		baseRequests = 1000000.0
	case "Staging":
		baseRequests = 100000.0
	}

	pred.P50 = baseRequests * profile.UtilizationFactor
	pred.P90 = baseRequests * profile.VarianceFactor
	pred.Confidence = confidence.MinConfidence
	pred.Assumptions = append(pred.Assumptions,
		"request-heuristic",
		"requires-historical-data",
	)

	return pred
}

func getFloatAttr(attrs map[string]any, key string, defaultVal float64) float64 {
	if v, ok := attrs[key].(float64); ok {
		return v
	}
	if v, ok := attrs[key].(int); ok {
		return float64(v)
	}
	return defaultVal
}
