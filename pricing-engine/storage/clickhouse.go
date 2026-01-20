package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/futuristic-iac/pkg/focus"
)

package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/futuristic-iac/pkg/focus"
)

// ClickHouseStore manages pricing data.
// In a real build, we would import "github.com/ClickHouse/clickhouse-go/v2" 
// and embed driver.Conn. For this stage, we simulate the SQL generation 
// to ensure the architectural logic (Append-Only) is correct.
type ClickHouseStore struct {
	// Conn clickhouse.Conn 
}

func NewClickHouseStore() *ClickHouseStore {
	return &ClickHouseStore{}
}

// Ingest generates the SQL for batch insertion.
func (s *ClickHouseStore) Ingest(ctx context.Context, items []focus.PricingItem) error {
	// Real implementation would use:
	// batch, err := s.Conn.PrepareBatch(ctx, "INSERT INTO pricing")
	// for _, item := range items { batch.Append(...) }
	// return batch.Send()
	
	fmt.Printf("SQL [Simulated]: INSERT INTO pricing (sku_id, provider, effective_date, price, unit, attributes) VALUES (... %d items ...)\n", len(items))
	return nil
}

// GetPrice executes the Time-Travel query.
func (s *ClickHouseStore) GetPrice(ctx context.Context, provider string, attributes map[string]string, effectiveTime time.Time) (*focus.PricingItem, error) {
	// 1. Build Attribute Filter (JSON based in ClickHouse)
	// Clause: "attributes['instance_type'] = 't3.medium'"
	// In ClickHouse: "visitParamExtractString(attributes, 'instance_type') = 't3.medium'"
	// Or utilizing a Map column: "attributes['instance_type'] = 't3.medium'"
	
	whereParts := []string{"provider = ?"}
	args := []interface{}{provider}
	
	// Query Explanation Metadata
	matchReasons := []string{fmt.Sprintf("provider=%s", provider)}

	for k, v := range attributes {
		// sanitizing key for safety in demo
		if isValidKey(k) {
			whereParts = append(whereParts, fmt.Sprintf("attributes['%s'] = ?", k))
			args = append(args, v)
			matchReasons = append(matchReasons, fmt.Sprintf("%s=%s", k, v))
		}
	}
	
	// 2. Append-Only Logic:
	// We want the LATEST record that is Effective BEFORE the requested time.
	// SQL: ... AND effective_date <= ? ORDER BY effective_date DESC LIMIT 1
	whereParts = append(whereParts, "effective_date <= ?")
	args = append(args, effectiveTime)
	
	query := fmt.Sprintf(`
		SELECT sku_id, unit, price_per_unit, currency, carbon_intensity, effective_date 
		FROM pricing 
		WHERE %s 
		ORDER BY effective_date DESC 
		LIMIT 1
	`, strings.Join(whereParts, " AND "))
	
	// fmt.Printf("SQL [Simulated]: %s | Args: %v\n", query, args)

	// 3. Stub Response (Simulating DB Hit)
	// In production, s.Conn.QueryRow(...)
	
	// Simulate finding "t3.medium" if attributes match
	if val, ok := attributes["instance_type"]; ok && val == "t3.medium" {
		return &focus.PricingItem{
			SkuID:           "sku-t3-medium-us-east-1",
			Provider:        provider,
			PricePerUnit:    0.0416,
			Unit:            "Hrs",
			Currency:        "USD",
			EffectiveDate:   effectiveTime.Add(-24 * time.Hour),         // Found a record from yesterday
			CarbonIntensity: 18.5,
			// New Metadata for Explainability
			Attributes: attributes, // Return what matched
		}, nil
	}
	
	// Simulate finding "gp2"
	if val, ok := attributes["volume_type"]; ok && val == "gp2" {
		return &focus.PricingItem{
			SkuID:           "sku-ebs-gp2-us-east-1",
			Provider:        provider,
			PricePerUnit:    0.10,
			Unit:            "GB-Mo", // Different unit! Warning!
			Currency:        "USD",
			EffectiveDate:   effectiveTime.Add(-48 * time.Hour),
			CarbonIntensity: 0.2, // low for static disk
			Attributes:      attributes,
		}, nil
	}

	return nil, fmt.Errorf("no pricing record found matching criterias")
}

func isValidKey(k string) bool {
	// prevent sql injection in column names for this simple builder
	for _, c := range k {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
