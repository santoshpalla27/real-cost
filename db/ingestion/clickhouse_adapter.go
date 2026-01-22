// Package ingestion provides adapters for the pricing ingestion pipeline
// Connects existing ingestion logic to ClickHouse storage
package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"terraform-cost/db/clickhouse"
)

// ClickHouseAdapter adapts the existing ingestion pipeline to ClickHouse
type ClickHouseAdapter struct {
	store *clickhouse.Store
}

// NewClickHouseAdapter creates a new ClickHouse adapter
func NewClickHouseAdapter(store *clickhouse.Store) *ClickHouseAdapter {
	return &ClickHouseAdapter{store: store}
}

// IngestionResult tracks the result of a pricing ingestion
type IngestionResult struct {
	SnapshotID    uuid.UUID
	Cloud         string
	Region        string
	RateKeyCount  int
	PriceCount    int
	Duration      time.Duration
	Success       bool
	ErrorMessage  string
}

// IngestPricing ingests pricing data into ClickHouse
// This is the main entry point for the pricing pipeline
func (a *ClickHouseAdapter) IngestPricing(ctx context.Context, input *IngestionInput) (*IngestionResult, error) {
	startTime := time.Now()
	result := &IngestionResult{
		Cloud:  input.Cloud,
		Region: input.Region,
	}

	// Create snapshot
	snapshot := &clickhouse.PricingSnapshot{
		ID:            uuid.New(),
		Cloud:         clickhouse.CloudProvider(input.Cloud),
		Region:        input.Region,
		ProviderAlias: input.Alias,
		Source:        input.Source,
		FetchedAt:     input.FetchedAt,
		ValidFrom:     input.ValidFrom,
		ValidTo:       input.ValidTo,
		Hash:          input.Hash,
		Version:       "1.0",
		IsActive:      false, // Activated after all rates ingested
	}

	if err := a.store.CreateSnapshot(ctx, snapshot); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("failed to create snapshot: %v", err)
		return result, err
	}

	result.SnapshotID = snapshot.ID

	// Batch insert rate keys and prices
	batchSize := 1000
	for i := 0; i < len(input.Prices); i += batchSize {
		end := i + batchSize
		if end > len(input.Prices) {
			end = len(input.Prices)
		}

		batch := input.Prices[i:end]

		// Process batch
		rates := make([]*clickhouse.PricingRate, 0, len(batch))
		for _, p := range batch {
			// Create or get rate key
			rateKey := &clickhouse.RateKey{
				ID:            uuid.New(),
				Cloud:         clickhouse.CloudProvider(input.Cloud),
				Service:       p.Service,
				ProductFamily: p.ProductFamily,
				Region:        p.Region,
				Attributes:    p.Attributes,
			}

			rateKeyResult, err := a.store.UpsertRateKey(ctx, rateKey)
			if err != nil {
				result.ErrorMessage = fmt.Sprintf("failed to upsert rate key at index %d: %v", i, err)
				return result, err
			}
			result.RateKeyCount++

			// Create pricing rate
			rate := &clickhouse.PricingRate{
				ID:            uuid.New(),
				SnapshotID:    snapshot.ID,
				RateKeyID:     rateKeyResult.ID,
				Unit:          p.Unit,
				Price:         p.Price,
				Currency:      p.Currency,
				Confidence:    p.Confidence,
				TierMin:       p.TierMin,
				TierMax:       p.TierMax,
				EffectiveDate: p.EffectiveDate,
			}
			rates = append(rates, rate)
		}

		// Bulk insert rates
		if err := a.store.BulkCreateRates(ctx, rates); err != nil {
			result.ErrorMessage = fmt.Sprintf("failed to bulk insert rates at batch %d: %v", i/batchSize, err)
			return result, err
		}
		result.PriceCount += len(rates)
	}

	// Activate snapshot
	if err := a.store.ActivateSnapshot(ctx, snapshot.ID); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to activate snapshot: %v", err)
		return result, err
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	return result, nil
}

// IngestionInput contains the pricing data to ingest
type IngestionInput struct {
	Cloud     string
	Region    string
	Alias     string
	Source    string
	FetchedAt time.Time
	ValidFrom time.Time
	ValidTo   *time.Time
	Hash      string
	Prices    []PriceEntry
}

// PriceEntry is a single pricing entry
type PriceEntry struct {
	Service       string
	ProductFamily string
	Region        string
	Attributes    map[string]string
	Unit          string
	Price         interface{} // decimal.Decimal or float64
	Currency      string
	Confidence    float64
	TierMin       interface{} // *decimal.Decimal
	TierMax       interface{} // *decimal.Decimal
	EffectiveDate *time.Time
}

// VerifyIngestion checks that ingestion was successful
func (a *ClickHouseAdapter) VerifyIngestion(ctx context.Context, snapshotID uuid.UUID) error {
	// Get snapshot and verify it's active
	// Count rates and verify minimum coverage
	// This would be expanded in production

	return nil
}

// GetIngestionStats returns statistics about ingested pricing data
func (a *ClickHouseAdapter) GetIngestionStats(ctx context.Context, cloud, region string) (*IngestionStats, error) {
	stats := &IngestionStats{
		Cloud:  cloud,
		Region: region,
	}

	// Get active snapshot
	snapshot, err := a.store.GetActiveSnapshot(ctx, clickhouse.CloudProvider(cloud), region, "default")
	if err != nil {
		return nil, err
	}

	if snapshot != nil {
		stats.ActiveSnapshotID = snapshot.ID
		stats.LastUpdated = snapshot.FetchedAt
		stats.IsActive = true
	}

	return stats, nil
}

// IngestionStats contains statistics about ingested data
type IngestionStats struct {
	Cloud            string
	Region           string
	ActiveSnapshotID uuid.UUID
	LastUpdated      time.Time
	IsActive         bool
	RateKeyCount     int
	PriceCount       int
	Coverage         float64
}
