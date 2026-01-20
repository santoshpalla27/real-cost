package storage

import (
	"context"
	"time"

	"github.com/futuristic-iac/pkg/focus"
)

type PricingStore interface {
	// Ingest bulk inserts pricing items
	Ingest(ctx context.Context, items []focus.PricingItem) error
	
	// GetPrice finds the best matching price for a SKU/Attributes at a specific time.
	GetPrice(ctx context.Context, provider string, attributes map[string]string, effectiveTime time.Time) (*focus.PricingItem, error)
}
