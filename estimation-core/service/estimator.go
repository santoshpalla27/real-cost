package service

import (
	"encoding/json"
	"fmt"
	"bytes"
	"net/http"
	"time"

	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/focus"
	"github.com/futuristic-iac/pkg/platform"
)

// Estimator orchestrates the pricing and usage data to form a Cost DAG.
type Estimator struct {
	UsageURL   string
	PricingURL string
	Client     *platform.HTTPClient
}

func NewEstimator(usageURL, pricingURL string) *Estimator {
	return &Estimator{
		UsageURL:   usageURL,
		PricingURL: pricingURL,
		Client:     platform.NewHTTPClient(3, 2*time.Second), // 3 retries, 2s timeout
	}
}

func (e *Estimator) Estimate(components []api.BillingComponent) (*api.EstimationResult, error) {
	result := &api.EstimationResult{
		TotalMonthlyCost: struct {
			P50 float64 `json:"p50"`
			P90 float64 `json:"p90"`
		}{0, 0},
		Carbon: struct {
			KgCO2e          float64 `json:"kgco2e"`
			RegionIntensity string  `json:"region_intensity"`
		}{0, "medium"},
		Drivers:        []api.CostDriver{},
		DetailedErrors: []api.EstimationError{},
		Errors:         []string{}, // Backward comp
	}

	globalConfidence := 1.0
	isIncomplete := false

	// --- STAGE 1: Resolution & Data Gathering ---
	type ResolvedComponent struct {
		Component api.BillingComponent
		Forecast  *api.UsageForecast
		Price     *focus.PricingItem
	}
	
	resolved := []ResolvedComponent{}

	for _, comp := range components {
		rc := ResolvedComponent{Component: comp}

		// 0. Check Semantic Mapping vs Policy
		if comp.MappingError != "" {
			err := api.EstimationError{
				Component: comp.ResourceAddress,
				Severity:  api.SeverityCritical, // Semantic failure is critical
				Message:   fmt.Sprintf("Mapping Failed: %s", comp.MappingError),
			}
			result.DetailedErrors = append(result.DetailedErrors, err)
			result.Errors = append(result.Errors, err.Message)
			isIncomplete = true
			globalConfidence *= 0.0 // Invalidates confidence
			continue
		}

		// 1. Usage Resolution
		forecast, err := e.getUsageForecast(comp)
		if err != nil {
			estErr := api.EstimationError{
				Component: comp.ResourceAddress,
				Severity:  api.SeverityCritical,
				Message:   fmt.Sprintf("Usage Forecast Failed: %v", err),
			}
			result.DetailedErrors = append(result.DetailedErrors, estErr)
			result.Errors = append(result.Errors, estErr.Message)
			isIncomplete = true
			globalConfidence *= 0.5
			continue
		}
		rc.Forecast = forecast
		globalConfidence *= forecast.Confidence

		// 2. Pricing Resolution
		price, err := e.getPrice(comp)
		if err != nil {
			var sev string = api.SeverityCritical
			// If Component says "usage_type: free_tier" maybe warning? 
			// For now, treat missing price as critical.
			
			estErr := api.EstimationError{
				Component: comp.ResourceAddress,
				Severity:  sev,
				Message:   fmt.Sprintf("Pricing Lookup Failed: %v", err),
			}
			result.DetailedErrors = append(result.DetailedErrors, estErr)
			result.Errors = append(result.Errors, estErr.Message)
			
			// Add 0-cost driver so it appears in list
			result.Drivers = append(result.Drivers, api.CostDriver{
				Component: comp.ResourceAddress,
				Reason:    "Price Missing",
				P50Cost:   0, P90Cost: 0,
			})
			
			isIncomplete = true
			globalConfidence *= 0.5 // Heavy penalty
			continue
		}
		rc.Price = price
		
		resolved = append(resolved, rc)
	}

	// --- STAGE 2: Cost Calculation ---
	for _, rc := range resolved {
		comp := rc.Component
		forecast := rc.Forecast
		price := rc.Price

		units := forecast.MonthlyForecast.P50
		unitLabel := "units"

		// Unit Normalization Logic
		if price.Unit == "GB-Mo" {
			sizeStr, ok := comp.LookupAttributes["size_gb"]
			if !ok {
				estErr := api.EstimationError{
					Component: comp.ResourceAddress,
					Severity:  api.SeverityCritical,
					Message:   "GB-Mo price requires size_gb attribute",
				}
				result.DetailedErrors = append(result.DetailedErrors, estErr)
				result.Errors = append(result.Errors, estErr.Message)
				isIncomplete = true
				continue
			}
			var size float64
			fmt.Sscanf(sizeStr, "%f", &size)
			months := forecast.MonthlyForecast.P50 / 730.0
			units = size * months
			unitLabel = fmt.Sprintf("GB-Mo (%.1f GB * %.1f Mo)", size, months)
		} else if price.Unit == "Hrs" || price.Unit == "Hrs" {
			unitLabel = "Hrs"
		}

		p50Cost := price.PricePerUnit * units
		p90Cost := price.PricePerUnit * (units * (forecast.MonthlyForecast.P90 / forecast.MonthlyForecast.P50))

		driver := api.CostDriver{
			Component: comp.ResourceAddress,
			Reason:    fmt.Sprintf("%s (%s)", comp.UsageType, price.SkuID),
			P50Cost:   p50Cost,
			P90Cost:   p90Cost,
			Formula:   fmt.Sprintf("%.2f %s * $%.4f", units, unitLabel, price.PricePerUnit),
		}

		result.Drivers = append(result.Drivers, driver)
		result.TotalMonthlyCost.P50 += p50Cost
		result.TotalMonthlyCost.P90 += p90Cost
		
		// Carbon
		carbon := price.CarbonIntensity * forecast.MonthlyForecast.P50 // gCO2e
		result.Carbon.KgCO2e += carbon / 1000.0
	}
	
	result.IsIncomplete = isIncomplete
	result.ConfidenceScore = globalConfidence
	
	result.IsIncomplete = isIncomplete
	result.ConfidenceScore = globalConfidence
	
	// CRITICAL CHECK: If any critical error exists, Confidence = 0.0
	for _, e := range result.DetailedErrors {
		if e.Severity == api.SeverityCritical {
			result.ConfidenceScore = 0.0
			break
		}
	}

	// Fail-Closed Aggregation: If incomplete, totals must be 0 to prevent downstream misuse.
	if result.IsIncomplete {
		result.TotalMonthlyCost.P50 = 0
		result.TotalMonthlyCost.P90 = 0
		result.Carbon.KgCO2e = 0
	}

	return result, nil
}

func (e *Estimator) getUsageForecast(comp api.BillingComponent) (*api.UsageForecast, error) {
	req := api.UsageForecastRequest{
		Component: comp,
	}
	body, _ := json.Marshal(req)
	
	resp, err := e.Client.PostJSON(e.UsageURL, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage service error: %d", resp.StatusCode)
	}
	
	var f api.UsageForecast
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (e *Estimator) getPrice(comp api.BillingComponent) (*focus.PricingItem, error) {
	req := struct {
		Provider   string            `json:"provider"`
		Attributes map[string]string `json:"attributes"`
		Timestamp  string            `json:"timestamp"`
	}{
		Provider:   comp.Provider,
		Attributes: comp.LookupAttributes,
		Timestamp:  time.Now().Format(time.RFC3339),
	}
	
	body, _ := json.Marshal(req)
	resp, err := e.Client.PostJSON(e.PricingURL, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing service error: %d", resp.StatusCode)
	}
	
	var p focus.PricingItem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
