// Package pricing provides SKU price resolution with carbon intensity.
package pricing

import (
	"fmt"
	"strings"
	"time"

	"github.com/santoshpalla27/fiac-platform/pkg/api"
)

// PriceEntry represents a pricing record.
type PriceEntry struct {
	SKUID           string
	Provider        string
	Region          string
	ResourceType    string
	Unit            string
	PricePerUnit    float64
	EffectiveDate   time.Time
	CarbonIntensity float64 // gCO2e per unit
	Attributes      map[string]string
}

// PriceResult holds resolved prices.
type PriceResult struct {
	Prices   []ResolvedPrice `json:"prices"`
	Warnings []string        `json:"warnings,omitempty"`
}

// ResolvedPrice represents a matched price.
type ResolvedPrice struct {
	ComponentID     string  `json:"component_id"`
	SKUID           string  `json:"sku_id"`
	PricePerUnit    float64 `json:"price_per_unit"`
	Unit            string  `json:"unit"`
	CarbonIntensity float64 `json:"carbon_intensity"`
	Explanation     string  `json:"explanation"`
}

// Resolver matches billing components to prices.
type Resolver struct {
	store *PriceStore
}

// NewResolver creates a price resolver with stub data.
func NewResolver() *Resolver {
	return &Resolver{
		store: NewPriceStore(),
	}
}

// Resolve finds prices for billing components.
func (r *Resolver) Resolve(components []api.BillingComponent, region string, effectiveDate *string) (*PriceResult, error) {
	result := &PriceResult{
		Prices:   []ResolvedPrice{},
		Warnings: []string{},
	}

	for _, comp := range components {
		price, err := r.resolveComponent(comp, region)
		if err != nil {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("Price not found for %s: %s", comp.ID, err.Error()))
			continue
		}
		result.Prices = append(result.Prices, price)
	}

	return result, nil
}

func (r *Resolver) resolveComponent(comp api.BillingComponent, region string) (ResolvedPrice, error) {
	// Build SKU lookup key
	skuKey := r.buildSKUKey(comp)
	
	entry, exists := r.store.Get(skuKey, region)
	if !exists {
		// Try with default region
		entry, exists = r.store.Get(skuKey, "us-east-1")
		if !exists {
			return ResolvedPrice{}, fmt.Errorf("no price found for SKU: %s in region: %s", skuKey, region)
		}
	}

	return ResolvedPrice{
		ComponentID:     comp.ID,
		SKUID:           entry.SKUID,
		PricePerUnit:    entry.PricePerUnit,
		Unit:            entry.Unit,
		CarbonIntensity: entry.CarbonIntensity,
		Explanation:     fmt.Sprintf("Matched %s at $%.6f/%s", entry.SKUID, entry.PricePerUnit, entry.Unit),
	}, nil
}

func (r *Resolver) buildSKUKey(comp api.BillingComponent) string {
	switch comp.Type {
	case api.ComponentTypeCompute:
		if instanceType, ok := comp.Attributes["instance_type"].(string); ok {
			return fmt.Sprintf("ec2:%s:ondemand", instanceType)
		}
		if instanceClass, ok := comp.Attributes["instance_class"].(string); ok {
			engine := getStrAttr(comp.Attributes, "engine", "mysql")
			return fmt.Sprintf("rds:%s:%s:ondemand", engine, instanceClass)
		}
		return "compute:generic"
	
	case api.ComponentTypeStorage:
		volumeType := getStrAttr(comp.Attributes, "volume_type", "gp3")
		return fmt.Sprintf("ebs:%s:storage", volumeType)
	
	case api.ComponentTypeNetwork:
		return "ec2:data-transfer:out"
	
	default:
		return "generic:unit"
	}
}

func getStrAttr(attrs map[string]any, key, defaultVal string) string {
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return defaultVal
}

// PriceStore is an in-memory price database (stub implementation).
type PriceStore struct {
	entries map[string]map[string]PriceEntry // sku -> region -> entry
}

// NewPriceStore creates a store with stub AWS pricing data.
func NewPriceStore() *PriceStore {
	store := &PriceStore{
		entries: make(map[string]map[string]PriceEntry),
	}
	
	// Add stub EC2 pricing
	store.addEntry(PriceEntry{
		SKUID:           "ec2:t3.micro:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.0104,
		CarbonIntensity: 0.005,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ec2:t3.small:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.0208,
		CarbonIntensity: 0.01,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ec2:t3.medium:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.0416,
		CarbonIntensity: 0.02,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ec2:t3.large:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.0832,
		CarbonIntensity: 0.04,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ec2:m5.large:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.096,
		CarbonIntensity: 0.05,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ec2:m5.xlarge:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.192,
		CarbonIntensity: 0.10,
	})

	// Add EBS pricing
	store.addEntry(PriceEntry{
		SKUID:           "ebs:gp3:storage",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "storage",
		Unit:            "GB-month",
		PricePerUnit:    0.08,
		CarbonIntensity: 0.001,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ebs:gp2:storage",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "storage",
		Unit:            "GB-month",
		PricePerUnit:    0.10,
		CarbonIntensity: 0.001,
	})
	store.addEntry(PriceEntry{
		SKUID:           "ebs:io1:storage",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "storage",
		Unit:            "GB-month",
		PricePerUnit:    0.125,
		CarbonIntensity: 0.002,
	})

	// Add network pricing
	store.addEntry(PriceEntry{
		SKUID:           "ec2:data-transfer:out",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "network",
		Unit:            "GB",
		PricePerUnit:    0.09,
		CarbonIntensity: 0.0005,
	})

	// Add RDS pricing
	store.addEntry(PriceEntry{
		SKUID:           "rds:mysql:db.t3.micro:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.017,
		CarbonIntensity: 0.008,
	})
	store.addEntry(PriceEntry{
		SKUID:           "rds:mysql:db.t3.small:ondemand",
		Provider:        "aws",
		Region:          "us-east-1",
		ResourceType:    "compute",
		Unit:            "hours",
		PricePerUnit:    0.034,
		CarbonIntensity: 0.016,
	})

	return store
}

func (s *PriceStore) addEntry(entry PriceEntry) {
	if _, exists := s.entries[entry.SKUID]; !exists {
		s.entries[entry.SKUID] = make(map[string]PriceEntry)
	}
	s.entries[entry.SKUID][entry.Region] = entry
}

// Get retrieves a price entry for a SKU and region.
func (s *PriceStore) Get(sku, region string) (PriceEntry, bool) {
	regions, exists := s.entries[sku]
	if !exists {
		// Try partial match
		for k, r := range s.entries {
			if matchSKU(sku, k) {
				if entry, ok := r[region]; ok {
					return entry, true
				}
				if entry, ok := r["us-east-1"]; ok {
					return entry, true
				}
			}
		}
		return PriceEntry{}, false
	}
	
	entry, exists := regions[region]
	if !exists {
		// Fall back to us-east-1
		entry, exists = regions["us-east-1"]
	}
	return entry, exists
}

func matchSKU(query, stored string) bool {
	// Simple prefix matching
	return stored == query || 
		   (len(query) > 0 && len(stored) > 0 && 
		    (query[:min(len(query), 10)] == stored[:min(len(stored), 10)] ||
		     strings.Contains(stored, strings.Split(query, ":")[0])))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
