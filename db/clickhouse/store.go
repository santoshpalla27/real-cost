// Package clickhouse provides ClickHouse implementation of PricingStore
// Optimized for columnar analytics, high-cardinality SKU data, and time-travel queries
package clickhouse

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CloudProvider represents a cloud provider
type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	Azure CloudProvider = "azure"
	GCP   CloudProvider = "gcp"
)

// PricingSnapshot represents a point-in-time pricing capture
type PricingSnapshot struct {
	ID            uuid.UUID     `ch:"id"`
	Cloud         CloudProvider `ch:"cloud"`
	Region        string        `ch:"region"`
	ProviderAlias string        `ch:"provider_alias"`
	Source        string        `ch:"source"`
	FetchedAt     time.Time     `ch:"fetched_at"`
	ValidFrom     time.Time     `ch:"valid_from"`
	ValidTo       *time.Time    `ch:"valid_to"`
	Hash          string        `ch:"hash"`
	Version       string        `ch:"version"`
	IsActive      bool          `ch:"is_active"`
	CreatedAt     time.Time     `ch:"created_at"`
}

// RateKey represents a unique pricing lookup key
type RateKey struct {
	ID            uuid.UUID         `ch:"id"`
	Cloud         CloudProvider     `ch:"cloud"`
	Service       string            `ch:"service"`
	ProductFamily string            `ch:"product_family"`
	Region        string            `ch:"region"`
	Attributes    map[string]string `ch:"attributes"`
	CreatedAt     time.Time         `ch:"created_at"`
}

// PricingRate represents a price for a rate key within a snapshot
type PricingRate struct {
	ID            uuid.UUID        `ch:"id"`
	SnapshotID    uuid.UUID        `ch:"snapshot_id"`
	RateKeyID     uuid.UUID        `ch:"rate_key_id"`
	Unit          string           `ch:"unit"`
	Price         decimal.Decimal  `ch:"price"`
	Currency      string           `ch:"currency"`
	Confidence    float64          `ch:"confidence"`
	TierMin       *decimal.Decimal `ch:"tier_min"`
	TierMax       *decimal.Decimal `ch:"tier_max"`
	EffectiveDate *time.Time       `ch:"effective_date"`
	CreatedAt     time.Time        `ch:"created_at"`
	// Denormalized fields
	Cloud         CloudProvider `ch:"cloud"`
	Region        string        `ch:"region"`
	Service       string        `ch:"service"`
	ProductFamily string        `ch:"product_family"`
}

// ResolvedRate is the result of a pricing lookup
type ResolvedRate struct {
	Price      decimal.Decimal
	Currency   string
	Confidence float64
	TierMin    *decimal.Decimal
	TierMax    *decimal.Decimal
	SnapshotID uuid.UUID
	Source     string
}

// TieredRate represents a pricing tier
type TieredRate struct {
	Min        decimal.Decimal
	Max        *decimal.Decimal
	Price      decimal.Decimal
	Confidence float64
}

// Config holds ClickHouse connection configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Debug    bool
}

// DefaultConfig returns default development configuration
func DefaultConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     9000,
		Database: "terracost",
		Username: "default",
		Password: "",
		Debug:    false,
	}
}

// Store implements PricingStore using ClickHouse
type Store struct {
	conn clickhouse.Conn
	cfg  *Config
}

// NewStore creates a new ClickHouse pricing store
func NewStore(cfg *Config) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Debug: cfg.Debug,
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	return &Store{conn: conn, cfg: cfg}, nil
}

// NewStoreFromDSN creates a store from a DSN string
// Format: clickhouse://user:password@host:port/database
func NewStoreFromDSN(dsn string) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}
	return &Store{conn: conn}, nil
}

// Ping checks database connectivity
func (s *Store) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.conn.Close()
}

// =============================================================================
// SNAPSHOT OPERATIONS
// =============================================================================

// CreateSnapshot inserts a new pricing snapshot
func (s *Store) CreateSnapshot(ctx context.Context, snapshot *PricingSnapshot) error {
	query := `
		INSERT INTO pricing_snapshots (
			id, cloud, region, provider_alias, source, fetched_at, 
			valid_from, valid_to, hash, version, is_active, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	return s.conn.Exec(ctx, query,
		snapshot.ID,
		string(snapshot.Cloud),
		snapshot.Region,
		snapshot.ProviderAlias,
		snapshot.Source,
		snapshot.FetchedAt,
		snapshot.ValidFrom,
		snapshot.ValidTo,
		snapshot.Hash,
		snapshot.Version,
		boolToUInt8(snapshot.IsActive),
		time.Now(),
	)
}

// GetSnapshot retrieves a snapshot by ID
func (s *Store) GetSnapshot(ctx context.Context, id uuid.UUID) (*PricingSnapshot, error) {
	query := `
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, is_active, created_at
		FROM pricing_snapshots FINAL
		WHERE id = ? AND _deleted = 0
	`
	row := s.conn.QueryRow(ctx, query, id)

	var snapshot PricingSnapshot
	var isActive uint8
	err := row.Scan(
		&snapshot.ID, &snapshot.Cloud, &snapshot.Region, &snapshot.ProviderAlias,
		&snapshot.Source, &snapshot.FetchedAt, &snapshot.ValidFrom, &snapshot.ValidTo,
		&snapshot.Hash, &snapshot.Version, &isActive, &snapshot.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	snapshot.IsActive = isActive == 1
	return &snapshot, nil
}

// GetActiveSnapshot retrieves the active snapshot for a cloud/region/alias
func (s *Store) GetActiveSnapshot(ctx context.Context, cloud CloudProvider, region, alias string) (*PricingSnapshot, error) {
	query := `
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, is_active, created_at
		FROM pricing_snapshots FINAL
		WHERE cloud = ? AND region = ? AND provider_alias = ? 
		  AND is_active = 1 AND _deleted = 0
		LIMIT 1
	`
	row := s.conn.QueryRow(ctx, query, string(cloud), region, alias)

	var snapshot PricingSnapshot
	var isActive uint8
	err := row.Scan(
		&snapshot.ID, &snapshot.Cloud, &snapshot.Region, &snapshot.ProviderAlias,
		&snapshot.Source, &snapshot.FetchedAt, &snapshot.ValidFrom, &snapshot.ValidTo,
		&snapshot.Hash, &snapshot.Version, &isActive, &snapshot.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active snapshot: %w", err)
	}
	snapshot.IsActive = isActive == 1
	return &snapshot, nil
}

// ActivateSnapshot activates a snapshot (marks it as active, deactivates others)
func (s *Store) ActivateSnapshot(ctx context.Context, id uuid.UUID) error {
	// Get snapshot details
	snapshot, err := s.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	// Deactivate existing active snapshots for this cloud/region/alias
	deactivateQuery := `
		INSERT INTO pricing_snapshots 
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, 0 as is_active, created_at,
			   _version + 1 as _version, _deleted
		FROM pricing_snapshots FINAL
		WHERE cloud = ? AND region = ? AND provider_alias = ? 
		  AND is_active = 1 AND _deleted = 0 AND id != ?
	`
	if err := s.conn.Exec(ctx, deactivateQuery, string(snapshot.Cloud), snapshot.Region, snapshot.ProviderAlias, id); err != nil {
		return fmt.Errorf("failed to deactivate snapshots: %w", err)
	}

	// Activate the target snapshot
	activateQuery := `
		INSERT INTO pricing_snapshots 
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, 1 as is_active, created_at,
			   _version + 1 as _version, _deleted
		FROM pricing_snapshots FINAL
		WHERE id = ?
	`
	return s.conn.Exec(ctx, activateQuery, id)
}

// ListSnapshots lists snapshots for a cloud/region
func (s *Store) ListSnapshots(ctx context.Context, cloud CloudProvider, region string) ([]*PricingSnapshot, error) {
	query := `
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, is_active, created_at
		FROM pricing_snapshots FINAL
		WHERE cloud = ? AND region = ? AND _deleted = 0
		ORDER BY created_at DESC
	`
	rows, err := s.conn.Query(ctx, query, string(cloud), region)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*PricingSnapshot
	for rows.Next() {
		var snapshot PricingSnapshot
		var isActive uint8
		if err := rows.Scan(
			&snapshot.ID, &snapshot.Cloud, &snapshot.Region, &snapshot.ProviderAlias,
			&snapshot.Source, &snapshot.FetchedAt, &snapshot.ValidFrom, &snapshot.ValidTo,
			&snapshot.Hash, &snapshot.Version, &isActive, &snapshot.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		snapshot.IsActive = isActive == 1
		snapshots = append(snapshots, &snapshot)
	}
	return snapshots, nil
}

// FindSnapshotByHash finds a snapshot by its content hash
func (s *Store) FindSnapshotByHash(ctx context.Context, cloud CloudProvider, region, alias, hash string) (*PricingSnapshot, error) {
	query := `
		SELECT id, cloud, region, provider_alias, source, fetched_at,
			   valid_from, valid_to, hash, version, is_active, created_at
		FROM pricing_snapshots FINAL
		WHERE cloud = ? AND region = ? AND provider_alias = ? AND hash = ? AND _deleted = 0
		LIMIT 1
	`
	row := s.conn.QueryRow(ctx, query, string(cloud), region, alias, hash)

	var snapshot PricingSnapshot
	var isActive uint8
	err := row.Scan(
		&snapshot.ID, &snapshot.Cloud, &snapshot.Region, &snapshot.ProviderAlias,
		&snapshot.Source, &snapshot.FetchedAt, &snapshot.ValidFrom, &snapshot.ValidTo,
		&snapshot.Hash, &snapshot.Version, &isActive, &snapshot.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot by hash: %w", err)
	}
	snapshot.IsActive = isActive == 1
	return &snapshot, nil
}

// CountRates returns the count of rates in a snapshot
func (s *Store) CountRates(ctx context.Context, snapshotID uuid.UUID) (int, error) {
	query := `SELECT count() FROM pricing_rates FINAL WHERE snapshot_id = ? AND _deleted = 0`
	row := s.conn.QueryRow(ctx, query, snapshotID)
	var count uint64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rates: %w", err)
	}
	return int(count), nil
}

// =============================================================================
// RATE KEY OPERATIONS
// =============================================================================

// UpsertRateKey inserts or returns existing rate key
func (s *Store) UpsertRateKey(ctx context.Context, key *RateKey) (*RateKey, error) {
	// Calculate attributes hash for fast lookup
	attrsHash := hashAttributes(key.Attributes)
	attrsJSON, err := json.Marshal(key.Attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Check if exists
	existingQuery := `
		SELECT id, created_at FROM pricing_rate_keys FINAL
		WHERE cloud = ? AND service = ? AND product_family = ? 
		  AND region = ? AND attributes_hash = ? AND _deleted = 0
		LIMIT 1
	`
	row := s.conn.QueryRow(ctx, existingQuery, string(key.Cloud), key.Service, key.ProductFamily, key.Region, attrsHash)
	var existingID uuid.UUID
	var createdAt time.Time
	if err := row.Scan(&existingID, &createdAt); err == nil {
		key.ID = existingID
		key.CreatedAt = createdAt
		return key, nil
	}

	// Insert new
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	key.CreatedAt = time.Now()

	insertQuery := `
		INSERT INTO pricing_rate_keys (id, cloud, service, product_family, region, attributes, attributes_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := s.conn.Exec(ctx, insertQuery, key.ID, string(key.Cloud), key.Service, key.ProductFamily, key.Region, string(attrsJSON), attrsHash, key.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to insert rate key: %w", err)
	}

	// Insert flattened attributes for analytics
	for k, v := range key.Attributes {
		attrQuery := `INSERT INTO pricing_rate_attributes (rate_key_id, attribute_key, attribute_value, cloud, created_at) VALUES (?, ?, ?, ?, ?)`
		if err := s.conn.Exec(ctx, attrQuery, key.ID, k, v, string(key.Cloud), time.Now()); err != nil {
			// Non-fatal, continue
		}
	}

	return key, nil
}

// GetRateKey retrieves a rate key
func (s *Store) GetRateKey(ctx context.Context, cloud CloudProvider, service, productFamily, region string, attrs map[string]string) (*RateKey, error) {
	attrsHash := hashAttributes(attrs)
	query := `
		SELECT id, cloud, service, product_family, region, attributes, created_at
		FROM pricing_rate_keys FINAL
		WHERE cloud = ? AND service = ? AND product_family = ? 
		  AND region = ? AND attributes_hash = ? AND _deleted = 0
		LIMIT 1
	`
	row := s.conn.QueryRow(ctx, query, string(cloud), service, productFamily, region, attrsHash)

	var key RateKey
	var attrsJSON string
	if err := row.Scan(&key.ID, &key.Cloud, &key.Service, &key.ProductFamily, &key.Region, &attrsJSON, &key.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get rate key: %w", err)
	}
	if err := json.Unmarshal([]byte(attrsJSON), &key.Attributes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}
	return &key, nil
}

// =============================================================================
// RATE OPERATIONS
// =============================================================================

// CreateRate inserts a pricing rate
func (s *Store) CreateRate(ctx context.Context, rate *PricingRate) error {
	query := `
		INSERT INTO pricing_rates (
			id, snapshot_id, rate_key_id, unit, price, currency, confidence,
			tier_min, tier_max, effective_date, created_at,
			cloud, region, service, product_family
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if rate.ID == uuid.Nil {
		rate.ID = uuid.New()
	}
	return s.conn.Exec(ctx, query,
		rate.ID, rate.SnapshotID, rate.RateKeyID, rate.Unit,
		rate.Price, rate.Currency, rate.Confidence,
		rate.TierMin, rate.TierMax, rate.EffectiveDate, time.Now(),
		string(rate.Cloud), rate.Region, rate.Service, rate.ProductFamily,
	)
}

// BulkCreateRates inserts multiple rates efficiently using batch insert
func (s *Store) BulkCreateRates(ctx context.Context, rates []*PricingRate) error {
	if len(rates) == 0 {
		return nil
	}

	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO pricing_rates (
			id, snapshot_id, rate_key_id, unit, price, currency, confidence,
			tier_min, tier_max, effective_date, created_at,
			cloud, region, service, product_family
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, rate := range rates {
		if rate.ID == uuid.Nil {
			rate.ID = uuid.New()
		}
		if err := batch.Append(
			rate.ID, rate.SnapshotID, rate.RateKeyID, rate.Unit,
			rate.Price, rate.Currency, rate.Confidence,
			rate.TierMin, rate.TierMax, rate.EffectiveDate, time.Now(),
			string(rate.Cloud), rate.Region, rate.Service, rate.ProductFamily,
		); err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	return batch.Send()
}

// ResolveRate looks up a rate from the active snapshot
func (s *Store) ResolveRate(ctx context.Context, cloud CloudProvider, service, productFamily, region string, attrs map[string]string, unit, alias string) (*ResolvedRate, error) {
	attrsHash := hashAttributes(attrs)

	query := `
		SELECT pr.price, pr.currency, pr.confidence, pr.tier_min, pr.tier_max, pr.snapshot_id, ps.source
		FROM pricing_rates pr FINAL
		JOIN pricing_snapshots ps FINAL ON pr.snapshot_id = ps.id
		JOIN pricing_rate_keys rk FINAL ON pr.rate_key_id = rk.id
		WHERE ps.cloud = ? AND ps.region = ? AND ps.provider_alias = ? AND ps.is_active = 1
		  AND rk.service = ? AND rk.product_family = ? AND rk.attributes_hash = ?
		  AND pr.unit = ?
		  AND ps._deleted = 0 AND pr._deleted = 0 AND rk._deleted = 0
		ORDER BY pr.tier_min NULLS FIRST
		LIMIT 1
	`

	row := s.conn.QueryRow(ctx, query, string(cloud), region, alias, service, productFamily, attrsHash, unit)

	var rate ResolvedRate
	if err := row.Scan(&rate.Price, &rate.Currency, &rate.Confidence, &rate.TierMin, &rate.TierMax, &rate.SnapshotID, &rate.Source); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to resolve rate: %w", err)
	}
	return &rate, nil
}

// ResolveTieredRates returns all tiers for a rate
func (s *Store) ResolveTieredRates(ctx context.Context, cloud CloudProvider, service, productFamily, region string, attrs map[string]string, unit, alias string) ([]TieredRate, error) {
	attrsHash := hashAttributes(attrs)

	query := `
		SELECT pr.price, pr.confidence, pr.tier_min, pr.tier_max
		FROM pricing_rates pr FINAL
		JOIN pricing_snapshots ps FINAL ON pr.snapshot_id = ps.id
		JOIN pricing_rate_keys rk FINAL ON pr.rate_key_id = rk.id
		WHERE ps.cloud = ? AND ps.region = ? AND ps.provider_alias = ? AND ps.is_active = 1
		  AND rk.service = ? AND rk.product_family = ? AND rk.attributes_hash = ?
		  AND pr.unit = ?
		  AND ps._deleted = 0 AND pr._deleted = 0 AND rk._deleted = 0
		ORDER BY pr.tier_min NULLS FIRST
	`

	rows, err := s.conn.Query(ctx, query, string(cloud), region, alias, service, productFamily, attrsHash, unit)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tiered rates: %w", err)
	}
	defer rows.Close()

	var tiers []TieredRate
	for rows.Next() {
		var tier TieredRate
		var tierMin, tierMax *decimal.Decimal
		if err := rows.Scan(&tier.Price, &tier.Confidence, &tierMin, &tierMax); err != nil {
			return nil, fmt.Errorf("failed to scan tier: %w", err)
		}
		if tierMin != nil {
			tier.Min = *tierMin
		}
		tier.Max = tierMax
		tiers = append(tiers, tier)
	}
	return tiers, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func hashAttributes(attrs map[string]string) string {
	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(attrs[k])
		sb.WriteString(";")
	}

	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}

func boolToUInt8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// CalculateTieredCost computes cost for tiered pricing
func CalculateTieredCost(usage decimal.Decimal, tiers []TieredRate) (decimal.Decimal, float64) {
	if len(tiers) == 0 {
		return decimal.Zero, 0
	}

	totalCost := decimal.Zero
	remaining := usage
	minConfidence := 1.0

	for _, tier := range tiers {
		if remaining.LessThanOrEqual(decimal.Zero) {
			break
		}

		var tierUsage decimal.Decimal
		if tier.Max == nil {
			tierUsage = remaining
		} else {
			tierSize := tier.Max.Sub(tier.Min)
			if remaining.GreaterThan(tierSize) {
				tierUsage = tierSize
			} else {
				tierUsage = remaining
			}
		}

		tierCost := tierUsage.Mul(tier.Price)
		totalCost = totalCost.Add(tierCost)
		remaining = remaining.Sub(tierUsage)

		if tier.Confidence < minConfidence {
			minConfidence = tier.Confidence
		}
	}

	return totalCost, minConfidence
}
