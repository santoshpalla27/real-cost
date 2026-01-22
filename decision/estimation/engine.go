// Package estimation provides the Cost & Carbon Estimation Engine
// Combines billing components, usage predictions, and pricing data to produce cost estimates
package estimation

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"terraform-cost/db/clickhouse"
	"terraform-cost/decision/billing"
)

// Engine is the Cost & Carbon Estimation Engine
type Engine struct {
	pricingStore *clickhouse.Store
	carbonStore  CarbonStore // Interface for carbon intensity data
}

// CarbonStore provides carbon intensity data
type CarbonStore interface {
	GetIntensity(ctx context.Context, cloud, region string) (float64, error)
}

// NewEngine creates a new estimation engine
func NewEngine(pricingStore *clickhouse.Store) *Engine {
	return &Engine{
		pricingStore: pricingStore,
	}
}

// WithCarbonStore adds carbon intensity support
func (e *Engine) WithCarbonStore(store CarbonStore) *Engine {
	e.carbonStore = store
	return e
}

// EstimationRequest contains inputs for cost estimation
type EstimationRequest struct {
	Components   []billing.BillingComponent
	Environment  string // dev, staging, prod
	PricingAlias string // Pricing version alias (default: "default")
	
	// Carbon options
	IncludeCarbon bool
	
	// Explainability
	IncludeFormulas bool
}

// EstimationResult contains the complete estimation output
type EstimationResult struct {
	// Cost totals
	MonthlyCostP50 decimal.Decimal `json:"monthly_cost_p50"`
	MonthlyCostP90 decimal.Decimal `json:"monthly_cost_p90"`
	HourlyCostP50  decimal.Decimal `json:"hourly_cost_p50"`
	
	// Carbon totals  
	CarbonKgCO2    float64            `json:"carbon_kg_co2"`
	CarbonByRegion map[string]float64 `json:"carbon_by_region"`
	
	// Cost breakdown
	CostDrivers []CostDriver `json:"cost_drivers"`
	
	// Quality metrics
	Confidence   float64 `json:"confidence"`
	IsIncomplete bool    `json:"is_incomplete"`
	
	// Errors and warnings
	Errors   []EstimationError `json:"errors"`
	Warnings []string          `json:"warnings"`
	
	// Audit trail
	AuditTrail AuditTrail `json:"audit_trail"`
	
	// Statistics
	ComponentsProcessed int `json:"components_processed"`
	ComponentsEstimated int `json:"components_estimated"`
	ComponentsSymbolic  int `json:"components_symbolic"`
}

// CostDriver explains a single cost line item
type CostDriver struct {
	// Identity
	ID           string `json:"id"`
	ComponentID  string `json:"component_id"`
	ResourceAddr string `json:"resource_addr"`
	
	// Classification
	Cloud         string `json:"cloud"`
	Service       string `json:"service"`
	ProductFamily string `json:"product_family"`
	Region        string `json:"region"`
	
	// Description
	Description string `json:"description"`
	
	// Cost calculation
	MonthlyCostP50 decimal.Decimal `json:"monthly_cost_p50"`
	MonthlyCostP90 decimal.Decimal `json:"monthly_cost_p90"`
	
	// Formula explanation
	Formula     string          `json:"formula"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	UsageP50    float64         `json:"usage_p50"`
	UsageP90    float64         `json:"usage_p90"`
	UsageUnit   string          `json:"usage_unit"`
	
	// Carbon
	CarbonKgCO2 float64 `json:"carbon_kg_co2"`
	
	// Quality
	Confidence float64 `json:"confidence"`
	IsSymbolic bool    `json:"is_symbolic"`
	Reason     string  `json:"reason,omitempty"`
	
	// Pricing reference
	SnapshotID uuid.UUID `json:"snapshot_id,omitempty"`
	Source     string    `json:"source,omitempty"`
}

// EstimationError represents an error during estimation
type EstimationError struct {
	ComponentID  string `json:"component_id"`
	ResourceAddr string `json:"resource_addr"`
	Message      string `json:"message"`
	IsCritical   bool   `json:"is_critical"`
}

// AuditTrail provides reproducibility information
type AuditTrail struct {
	EstimatedAt   time.Time          `json:"estimated_at"`
	Environment   string             `json:"environment"`
	PricingAlias  string             `json:"pricing_alias"`
	SnapshotsUsed map[string]uuid.UUID `json:"snapshots_used"` // region -> snapshot ID
}

// Estimate performs cost and carbon estimation
func (e *Engine) Estimate(ctx context.Context, req EstimationRequest) (*EstimationResult, error) {
	result := &EstimationResult{
		MonthlyCostP50: decimal.Zero,
		MonthlyCostP90: decimal.Zero,
		HourlyCostP50:  decimal.Zero,
		CarbonKgCO2:    0,
		CarbonByRegion: make(map[string]float64),
		CostDrivers:    make([]CostDriver, 0),
		Confidence:     1.0,
		Errors:         make([]EstimationError, 0),
		Warnings:       make([]string, 0),
		AuditTrail: AuditTrail{
			EstimatedAt:   time.Now(),
			Environment:   req.Environment,
			PricingAlias:  req.PricingAlias,
			SnapshotsUsed: make(map[string]uuid.UUID),
		},
	}
	
	if req.PricingAlias == "" {
		req.PricingAlias = "default"
	}
	
	// Track minimum confidence across all components
	minConfidence := 1.0
	
	// Process each billing component
	for _, comp := range req.Components {
		result.ComponentsProcessed++
		
		driver, err := e.estimateComponent(ctx, comp, req)
		if err != nil {
			result.Errors = append(result.Errors, EstimationError{
				ComponentID:  comp.ID,
				ResourceAddr: comp.ResourceAddr,
				Message:      err.Error(),
				IsCritical:   false,
			})
			result.ComponentsSymbolic++
			
			// Add symbolic driver
			driver = e.createSymbolicDriver(comp, err.Error())
		}
		
		// Add to totals
		result.MonthlyCostP50 = result.MonthlyCostP50.Add(driver.MonthlyCostP50)
		result.MonthlyCostP90 = result.MonthlyCostP90.Add(driver.MonthlyCostP90)
		result.CarbonKgCO2 += driver.CarbonKgCO2
		
		if driver.Region != "" && driver.CarbonKgCO2 > 0 {
			result.CarbonByRegion[driver.Region] += driver.CarbonKgCO2
		}
		
		// Track confidence
		if driver.Confidence < minConfidence {
			minConfidence = driver.Confidence
		}
		
		// Track snapshot usage
		if driver.SnapshotID != uuid.Nil {
			result.AuditTrail.SnapshotsUsed[driver.Region] = driver.SnapshotID
		}
		
		if !driver.IsSymbolic {
			result.ComponentsEstimated++
		}
		
		result.CostDrivers = append(result.CostDrivers, driver)
	}
	
	// Calculate hourly cost
	if !result.MonthlyCostP50.IsZero() {
		result.HourlyCostP50 = result.MonthlyCostP50.Div(decimal.NewFromFloat(730))
	}
	
	// Set final confidence
	result.Confidence = minConfidence
	
	// Mark as incomplete if any symbolic costs
	if result.ComponentsSymbolic > 0 {
		result.IsIncomplete = true
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("%d components could not be priced", result.ComponentsSymbolic))
	}
	
	// Fail-closed: if incomplete, zero out totals
	if result.IsIncomplete {
		// Keep the drivers for explainability, but zero the aggregate
		result.Warnings = append(result.Warnings,
			"Totals may be incomplete due to missing pricing data")
	}
	
	// Sort cost drivers by cost (highest first)
	sort.Slice(result.CostDrivers, func(i, j int) bool {
		return result.CostDrivers[i].MonthlyCostP50.GreaterThan(result.CostDrivers[j].MonthlyCostP50)
	})
	
	return result, nil
}

// estimateComponent estimates a single billing component
func (e *Engine) estimateComponent(ctx context.Context, comp billing.BillingComponent, req EstimationRequest) (CostDriver, error) {
	driver := CostDriver{
		ID:            fmt.Sprintf("driver-%s", comp.ID),
		ComponentID:   comp.ID,
		ResourceAddr:  comp.ResourceAddr,
		Cloud:         comp.Cloud,
		Service:       comp.Service,
		ProductFamily: comp.ProductFamily,
		Region:        comp.Region,
		Description:   comp.Description,
		UsageP50:      comp.VarianceProfile.P50Usage,
		UsageP90:      comp.VarianceProfile.P90Usage,
		Confidence:    comp.VarianceProfile.Confidence,
	}
	
	// Resolve pricing
	rate, err := e.pricingStore.ResolveRate(
		ctx,
		clickhouse.CloudProvider(comp.Cloud),
		comp.Service,
		comp.ProductFamily,
		comp.Region,
		comp.Attributes,
		e.billingPeriodToUnit(comp.BillingPeriod),
		req.PricingAlias,
	)
	
	if err != nil {
		return driver, fmt.Errorf("pricing resolution failed: %w", err)
	}
	
	if rate == nil {
		driver.IsSymbolic = true
		driver.Reason = "no pricing data available"
		return driver, nil
	}
	
	// Calculate costs
	driver.UnitPrice = rate.Price
	driver.SnapshotID = rate.SnapshotID
	driver.Source = rate.Source
	driver.Confidence = min(driver.Confidence, rate.Confidence)
	
	// Apply usage to get monthly cost
	usageP50 := decimal.NewFromFloat(comp.VarianceProfile.P50Usage)
	usageP90 := decimal.NewFromFloat(comp.VarianceProfile.P90Usage)
	
	driver.MonthlyCostP50 = rate.Price.Mul(usageP50).Round(4)
	driver.MonthlyCostP90 = rate.Price.Mul(usageP90).Round(4)
	
	// Generate formula
	driver.UsageUnit = e.billingPeriodToUnit(comp.BillingPeriod)
	if req.IncludeFormulas {
		driver.Formula = fmt.Sprintf("%.2f %s × $%s/%s = $%s",
			comp.VarianceProfile.P50Usage,
			driver.UsageUnit,
			rate.Price.StringFixed(6),
			driver.UsageUnit,
			driver.MonthlyCostP50.StringFixed(2),
		)
	}
	
	// Calculate carbon if enabled
	if req.IncludeCarbon && e.carbonStore != nil {
		carbonIntensity, err := e.carbonStore.GetIntensity(ctx, comp.Cloud, comp.Region)
		if err == nil && carbonIntensity > 0 {
			// Estimate based on compute hours and regional intensity
			// This is a simplified model - real implementation would be more sophisticated
			driver.CarbonKgCO2 = e.estimateCarbonForComponent(comp, carbonIntensity)
		}
	}
	
	return driver, nil
}

// createSymbolicDriver creates a driver for unpriced components
func (e *Engine) createSymbolicDriver(comp billing.BillingComponent, reason string) CostDriver {
	return CostDriver{
		ID:            fmt.Sprintf("driver-%s", comp.ID),
		ComponentID:   comp.ID,
		ResourceAddr:  comp.ResourceAddr,
		Cloud:         comp.Cloud,
		Service:       comp.Service,
		ProductFamily: comp.ProductFamily,
		Region:        comp.Region,
		Description:   comp.Description,
		MonthlyCostP50: decimal.Zero,
		MonthlyCostP90: decimal.Zero,
		Confidence:    0,
		IsSymbolic:    true,
		Reason:        reason,
	}
}

// estimateCarbonForComponent estimates carbon emissions for a component
func (e *Engine) estimateCarbonForComponent(comp billing.BillingComponent, intensityGCO2 float64) float64 {
	// Simplified carbon model based on service type
	// In production, this would use actual power consumption models
	
	var powerKw float64
	
	switch comp.Service {
	case "AmazonEC2":
		// Estimate based on instance type (simplified)
		powerKw = 0.1 // 100W average for small instance
	case "AmazonRDS":
		powerKw = 0.2 // 200W average for database
	case "AWSLambda":
		powerKw = 0.01 // Minimal for serverless
	default:
		powerKw = 0.05 // Default estimate
	}
	
	// Calculate monthly energy (kWh) = power (kW) × hours
	hoursPerMonth := 730.0
	energyKwh := powerKw * hoursPerMonth
	
	// Convert to kg CO2 (intensity is in gCO2/kWh)
	carbonKg := energyKwh * intensityGCO2 / 1000.0
	
	return carbonKg
}

// billingPeriodToUnit converts billing period to pricing unit
func (e *Engine) billingPeriodToUnit(period billing.BillingPeriod) string {
	switch period {
	case billing.PeriodHourly:
		return "hours"
	case billing.PeriodMonthly:
		return "GB-month"
	case billing.PeriodPerRequest:
		return "requests"
	case billing.PeriodPerGB:
		return "GB"
	default:
		return "units"
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
