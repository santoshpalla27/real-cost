package focus

import "time"

// PricingItem represents a normalized pricing record following FOCUS-like semantics.
type PricingItem struct {
	SkuID           string                 `json:"sku_id"`
	Provider        string                 `json:"provider"`         // AWS, GCP, Azure
	Service         string                 `json:"service"`          // AmazonEC2
	Family          string                 `json:"family,omitempty"` // Compute Instance
	Region          string                 `json:"region"`
	Unit            string                 `json:"unit"`
	PricePerUnit    float64                `json:"price_per_unit"`
	Currency        string                 `json:"currency"`
	EffectiveDate   time.Time              `json:"effective_date"`
	CarbonIntensity float64                `json:"carbon_intensity"` // gCO2e/unit
	Attributes      map[string]interface{} `json:"attributes"`       // Searchable attributes

	// Explanation Fields
	MatchConfidence float64  `json:"match_confidence"` // 0.0 - 1.0
	MatchedKeys     []string `json:"matched_keys"`
	IgnoredKeys     []string `json:"ignored_keys"`
}
