// Package estimation provides cost and carbon estimation.
package estimation

import (
	"sort"

	"github.com/santoshpalla27/fiac-platform/internal/carbon"
	"github.com/santoshpalla27/fiac-platform/internal/pricing"
	"github.com/santoshpalla27/fiac-platform/pkg/api"
	"github.com/santoshpalla27/fiac-platform/pkg/confidence"
)

// Calculator combines semantics, usage, and pricing into estimates.
type Calculator struct{}

// Result is the estimation output.
type Result struct {
	TotalCost       api.CostRange         `json:"total_cost"`
	TotalCarbon     api.CarbonEstimate    `json:"total_carbon"`
	Drivers         []api.CostDriver      `json:"drivers"`
	ConfidenceScore float64               `json:"confidence_score"`
	IsIncomplete    bool                  `json:"is_incomplete"`
	Errors          []api.EstimationError `json:"errors,omitempty"`
	Lineage         []api.LineageItem     `json:"lineage,omitempty"`
}

func NewCalculator() *Calculator { return &Calculator{} }

func (c *Calculator) Calculate(req api.EstimateRequest) (*Result, error) {
	return &Result{
		TotalCost:    api.CostRange{Currency: "USD"},
		IsIncomplete: true,
	}, nil
}

func (c *Calculator) CalculateFromComponents(
	semantics *api.SemanticResult,
	usage *api.UsageResult,
	prices *pricing.PriceResult,
) (*Result, error) {
	result := &Result{
		TotalCost: api.CostRange{Currency: "USD"},
		Drivers:   []api.CostDriver{},
		Lineage:   []api.LineageItem{},
		Errors:    []api.EstimationError{},
	}

	if len(semantics.MappingErrors) > 0 {
		result.IsIncomplete = true
		for _, err := range semantics.MappingErrors {
			result.Errors = append(result.Errors, api.EstimationError{
				Code: err.Code, Message: err.Message, Recoverable: err.Recoverable,
			})
		}
		return result, nil
	}

	usageMap := make(map[string]api.UsagePrediction)
	for _, p := range usage.Predictions {
		usageMap[p.ComponentID] = p
	}
	priceMap := make(map[string]pricing.ResolvedPrice)
	for _, p := range prices.Prices {
		priceMap[p.ComponentID] = p
	}

	var confidences []float64
	var totalP50, totalP90, totalCarbon float64

	for _, comp := range semantics.Components {
		u, hasU := usageMap[comp.ID]
		p, hasP := priceMap[comp.ID]
		if !hasU || !hasP {
			continue
		}

		p50Cost := u.P50 * p.PricePerUnit
		p90Cost := u.P90 * p.PricePerUnit
		totalP50 += p50Cost
		totalP90 += p90Cost
		totalCarbon += u.P50 * p.CarbonIntensity
		confidences = append(confidences, u.Confidence)

		result.Lineage = append(result.Lineage, api.LineageItem{
			ResourceID: comp.ResourceID, Component: string(comp.Type),
			SKU: p.SKUID, Price: p.PricePerUnit, Unit: p.Unit,
			Quantity: u.P50, MonthlyCost: p50Cost, Explanation: p.Explanation,
		})
	}

	result.TotalCost.P50, result.TotalCost.P90 = totalP50, totalP90
	result.TotalCarbon.KgCO2e = totalCarbon
	if len(confidences) > 0 {
		result.ConfidenceScore = confidence.Aggregate(confidences)
	}
	result.Drivers = c.extractDrivers(result.Lineage, totalP50)
	return result, nil
}

func (c *Calculator) extractDrivers(lineage []api.LineageItem, total float64) []api.CostDriver {
	if total == 0 {
		return nil
	}
	costs := make(map[string]float64)
	for _, item := range lineage {
		costs[item.ResourceID] += item.MonthlyCost
	}
	var drivers []api.CostDriver
	for id, cost := range costs {
		drivers = append(drivers, api.CostDriver{
			ResourceID: id, Name: id, MonthlyCost: cost, Percentage: (cost / total) * 100,
		})
	}
	sort.Slice(drivers, func(i, j int) bool { return drivers[i].MonthlyCost > drivers[j].MonthlyCost })
	if len(drivers) > 5 {
		drivers = drivers[:5]
	}
	return drivers
}

func (c *Calculator) EstimateCarbon(region string, hours, gbMonths float64) api.CarbonEstimate {
	compute := carbon.ComputeCarbon{Region: region, Hours: hours}
	storage := carbon.StorageCarbon{Region: region, GBMonths: gbMonths}
	return api.CarbonEstimate{
		KgCO2e: compute.EstimateKgCO2e() + storage.EstimateKgCO2e(),
		Confidence: 0.7, Region: region,
	}
}
