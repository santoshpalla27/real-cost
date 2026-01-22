-- ============================================================================
-- Terra-Cost ClickHouse Schema
-- Columnar storage for high-cardinality pricing data
-- Optimized for analytical queries, time-travel, and append-only semantics
-- ============================================================================

-- ============================================================================
-- PRICING SNAPSHOTS
-- Immutable point-in-time captures of pricing data
-- ============================================================================

CREATE TABLE IF NOT EXISTS pricing_snapshots (
    id              UUID,
    cloud           LowCardinality(String),  -- aws, azure, gcp
    region          LowCardinality(String),
    provider_alias  LowCardinality(String) DEFAULT 'default',
    source          LowCardinality(String),  -- aws_pricing_api, azure_retail, gcp_catalog
    fetched_at      DateTime64(3),
    valid_from      DateTime64(3),
    valid_to        Nullable(DateTime64(3)),
    hash            String,                  -- SHA256 content hash
    version         String DEFAULT '1.0',
    is_active       UInt8 DEFAULT 0,         -- Boolean: 0 or 1
    created_at      DateTime64(3) DEFAULT now64(3),
    
    -- Versioning for time-travel
    _version        UInt64 DEFAULT 1,
    _deleted        UInt8 DEFAULT 0
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(created_at)
ORDER BY (cloud, region, provider_alias, id)
SETTINGS index_granularity = 8192;

-- Materialized view for active snapshots (fast lookups)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_active_snapshots
ENGINE = ReplacingMergeTree()
ORDER BY (cloud, region, provider_alias)
AS SELECT
    id,
    cloud,
    region,
    provider_alias,
    source,
    hash,
    fetched_at,
    valid_from
FROM pricing_snapshots
WHERE is_active = 1 AND _deleted = 0;

-- ============================================================================
-- PRICING RATE KEYS
-- Normalized lookup keys with attributes
-- ============================================================================

CREATE TABLE IF NOT EXISTS pricing_rate_keys (
    id              UUID,
    cloud           LowCardinality(String),
    service         LowCardinality(String),  -- AmazonEC2, AmazonRDS
    product_family  LowCardinality(String),  -- Compute Instance, Storage
    region          LowCardinality(String),
    attributes      String,                   -- JSON: {instanceType, os, tenancy}
    attributes_hash String,                   -- Hash for fast equality
    created_at      DateTime64(3) DEFAULT now64(3),
    
    _version        UInt64 DEFAULT 1,
    _deleted        UInt8 DEFAULT 0
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY cloud
ORDER BY (cloud, service, product_family, region, attributes_hash)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- PRICING RATES
-- Actual prices tied to snapshots and rate keys
-- Supports tiered pricing (S3, data transfer, free tiers)
-- ============================================================================

CREATE TABLE IF NOT EXISTS pricing_rates (
    id              UUID,
    snapshot_id     UUID,
    rate_key_id     UUID,
    unit            LowCardinality(String),   -- hours, GB-month, requests
    price           Decimal128(10),           -- High precision pricing
    currency        LowCardinality(String) DEFAULT 'USD',
    confidence      Float64 DEFAULT 1.0,
    tier_min        Nullable(Decimal128(10)), -- NULL for non-tiered
    tier_max        Nullable(Decimal128(10)), -- NULL for unlimited tier
    effective_date  Nullable(Date),
    created_at      DateTime64(3) DEFAULT now64(3),
    
    -- Denormalized for query performance
    cloud           LowCardinality(String),
    region          LowCardinality(String),
    service         LowCardinality(String),
    product_family  LowCardinality(String),
    
    _version        UInt64 DEFAULT 1,
    _deleted        UInt8 DEFAULT 0
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY (cloud, toYYYYMM(created_at))
ORDER BY (cloud, region, service, product_family, snapshot_id, rate_key_id, unit, tier_min)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- RATE ATTRIBUTES (Flattened for analytics)
-- Enables fast filtering by specific attributes
-- ============================================================================

CREATE TABLE IF NOT EXISTS pricing_rate_attributes (
    rate_key_id     UUID,
    attribute_key   LowCardinality(String),
    attribute_value String,
    cloud           LowCardinality(String),
    created_at      DateTime64(3) DEFAULT now64(3)
) ENGINE = MergeTree()
PARTITION BY cloud
ORDER BY (cloud, attribute_key, attribute_value, rate_key_id)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- CARBON INTENSITY
-- Regional carbon data for sustainability tracking
-- ============================================================================

CREATE TABLE IF NOT EXISTS carbon_intensity (
    id              UUID,
    cloud           LowCardinality(String),
    region          LowCardinality(String),
    intensity_gco2  Float64,                  -- grams CO2 per kWh
    source          LowCardinality(String),   -- electricitymap, watttime
    measured_at     DateTime64(3),
    valid_from      DateTime64(3),
    valid_to        Nullable(DateTime64(3)),
    is_active       UInt8 DEFAULT 0,
    created_at      DateTime64(3) DEFAULT now64(3),
    
    _version        UInt64 DEFAULT 1,
    _deleted        UInt8 DEFAULT 0
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY cloud
ORDER BY (cloud, region, id)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- SERVICE CATALOG (Reference Data)
-- ============================================================================

CREATE TABLE IF NOT EXISTS service_catalog (
    cloud           LowCardinality(String),
    service         String,
    product_family  String,
    description     Nullable(String),
    is_billable     UInt8 DEFAULT 1,
    created_at      DateTime64(3) DEFAULT now64(3)
) ENGINE = ReplacingMergeTree()
ORDER BY (cloud, service, product_family)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- INGESTION STATE (Governance)
-- ============================================================================

CREATE TABLE IF NOT EXISTS ingestion_state (
    id              UUID,
    snapshot_id     UUID,
    provider        LowCardinality(String),
    status          LowCardinality(String),   -- started, in_progress, completed, failed
    record_count    UInt64 DEFAULT 0,
    dimension_count UInt64 DEFAULT 0,
    checksum        Nullable(String),
    error_message   Nullable(String),
    started_at      DateTime64(3),
    completed_at    Nullable(DateTime64(3)),
    created_at      DateTime64(3) DEFAULT now64(3)
) ENGINE = ReplacingMergeTree()
ORDER BY (snapshot_id, id)
SETTINGS index_granularity = 8192;

-- ============================================================================
-- ESTIMATION AUDIT LOG
-- Track all estimation requests for auditability
-- ============================================================================

CREATE TABLE IF NOT EXISTS estimation_audit_log (
    id              UUID,
    request_hash    String,                   -- Hash of input for deduplication
    snapshot_ids    Array(UUID),              -- All snapshots used
    resource_count  UInt32,
    monthly_cost_p50 Decimal128(4),
    monthly_cost_p90 Decimal128(4),
    carbon_kg_co2   Float64,
    confidence      Float64,
    is_incomplete   UInt8,
    policy_result   LowCardinality(String),   -- pass, deny, warn
    violations      Array(String),
    created_at      DateTime64(3) DEFAULT now64(3),
    
    -- Request metadata
    source          LowCardinality(String),   -- cli, api, ci
    environment     LowCardinality(String),   -- dev, staging, prod
    user_agent      Nullable(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (created_at, id)
TTL created_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- ============================================================================
-- SEED DATA - Common Services
-- ============================================================================

INSERT INTO service_catalog (cloud, service, product_family, description, is_billable) VALUES
-- AWS
('aws', 'AmazonEC2', 'Compute Instance', 'EC2 On-Demand Instances', 1),
('aws', 'AmazonEC2', 'Storage', 'EBS Volumes', 1),
('aws', 'AmazonEC2', 'Data Transfer', 'EC2 Data Transfer', 1),
('aws', 'AmazonRDS', 'Database Instance', 'RDS Instances', 1),
('aws', 'AmazonRDS', 'Database Storage', 'RDS Storage', 1),
('aws', 'AmazonS3', 'Storage', 'S3 Standard Storage', 1),
('aws', 'AmazonS3', 'Data Transfer', 'S3 Data Transfer', 1),
('aws', 'AWSLambda', 'Serverless', 'Lambda Functions', 1),
('aws', 'AmazonDynamoDB', 'Database', 'DynamoDB Tables', 1),
('aws', 'AmazonVPC', 'Networking', 'NAT Gateway', 1),
('aws', 'ElasticLoadBalancing', 'Networking', 'Load Balancers', 1);
