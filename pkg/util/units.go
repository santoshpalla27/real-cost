package util

import (
	"fmt"
	"strings"
)

// NormalizeUnitFactor returns the multiplier to convert 'usageUnits' into 'pricingUnits'.
// Example: Usage="GB-Mpth", Price="GB-Mo" -> 1.0
// Example: Usage="Hours", Price="Hrs" -> 1.0
func NormalizeUnitFactor(usageUnit string, pricingUnit string) (float64, error) {
	u := strings.ToLower(usageUnit)
	p := strings.ToLower(pricingUnit)

	// Direct match
	if u == p {
		return 1.0, nil
	}

	// Aliases
	if (u == "hours" || u == "hr") && (p == "hours" || p == "hrs") {
		return 1.0, nil
	}
	
	// In the EC2 EBS example:
	// Usage Forecast for storage is often just "Exists" (Hours)
	// But Pricing is "GB-Mo".
	// Semantic Engine passes "size_gb" in attributes.
	// So the COST formula is: Size(GB) * Time(MonthFraction) * Price(Per-GB-Mo).
	// This logic belongs in the Estimation Core, but we need a helper to detect unit mismatch.
	
	return 0.0, fmt.Errorf("incompatible units: usage=%s, pricing=%s", usageUnit, pricingUnit)
}
