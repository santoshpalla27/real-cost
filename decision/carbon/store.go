// Package carbon provides carbon intensity data sources
// Integrates with Electricity Maps, WattTime, and static fallback data
package carbon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CarbonStore provides carbon intensity data for regions
type CarbonStore interface {
	GetIntensity(ctx context.Context, cloud, region string) (float64, error)
}

// =============================================================================
// ELECTRICITY MAPS CLIENT
// =============================================================================

// ElectricityMapsClient fetches real-time carbon intensity from Electricity Maps API
type ElectricityMapsClient struct {
	apiKey     string
	httpClient *http.Client
	cache      map[string]cachedIntensity
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
}

type cachedIntensity struct {
	value     float64
	expiresAt time.Time
}

// NewElectricityMapsClient creates a new Electricity Maps client
func NewElectricityMapsClient(apiKey string) *ElectricityMapsClient {
	return &ElectricityMapsClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache:    make(map[string]cachedIntensity),
		cacheTTL: 15 * time.Minute,
	}
}

// GetIntensity fetches carbon intensity for a cloud region
func (c *ElectricityMapsClient) GetIntensity(ctx context.Context, cloud, region string) (float64, error) {
	zone := cloudRegionToZone(cloud, region)
	if zone == "" {
		return 0, fmt.Errorf("unknown region mapping: %s/%s", cloud, region)
	}

	// Check cache
	c.cacheMu.RLock()
	if cached, ok := c.cache[zone]; ok && time.Now().Before(cached.expiresAt) {
		c.cacheMu.RUnlock()
		return cached.value, nil
	}
	c.cacheMu.RUnlock()

	// Fetch from API
	intensity, err := c.fetchIntensity(ctx, zone)
	if err != nil {
		// Fall back to static data on error
		if fallback, ok := staticIntensityData[zone]; ok {
			return fallback, nil
		}
		return 0, err
	}

	// Update cache
	c.cacheMu.Lock()
	c.cache[zone] = cachedIntensity{
		value:     intensity,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.cacheMu.Unlock()

	return intensity, nil
}

func (c *ElectricityMapsClient) fetchIntensity(ctx context.Context, zone string) (float64, error) {
	url := fmt.Sprintf("https://api.electricitymap.org/v3/carbon-intensity/latest?zone=%s", zone)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("auth-token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("electricity maps API returned status %d", resp.StatusCode)
	}

	var result struct {
		CarbonIntensity float64 `json:"carbonIntensity"`
		DateTime        string  `json:"datetime"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.CarbonIntensity, nil
}

// =============================================================================
// STATIC CARBON STORE (FALLBACK)
// =============================================================================

// StaticCarbonStore provides carbon intensity from static data
type StaticCarbonStore struct{}

// NewStaticCarbonStore creates a static carbon store
func NewStaticCarbonStore() *StaticCarbonStore {
	return &StaticCarbonStore{}
}

// GetIntensity returns static carbon intensity for a region
func (s *StaticCarbonStore) GetIntensity(_ context.Context, cloud, region string) (float64, error) {
	zone := cloudRegionToZone(cloud, region)
	if zone == "" {
		// Return global average if unknown
		return 475, nil
	}

	if intensity, ok := staticIntensityData[zone]; ok {
		return intensity, nil
	}

	// Return global average
	return 475, nil
}

// =============================================================================
// REGION MAPPINGS
// =============================================================================

// cloudRegionToZone maps cloud provider regions to Electricity Maps zones
func cloudRegionToZone(cloud, region string) string {
	key := cloud + ":" + region
	if zone, ok := regionToZoneMap[key]; ok {
		return zone
	}
	return ""
}

// AWS, Azure, GCP region to Electricity Maps zone mapping
var regionToZoneMap = map[string]string{
	// AWS US
	"aws:us-east-1":      "US-MIDA-PJM",
	"aws:us-east-2":      "US-MIDA-PJM",
	"aws:us-west-1":      "US-CAL-CISO",
	"aws:us-west-2":      "US-NW-PACW",

	// AWS Europe
	"aws:eu-west-1":      "IE",
	"aws:eu-west-2":      "GB",
	"aws:eu-west-3":      "FR",
	"aws:eu-central-1":   "DE",
	"aws:eu-north-1":     "SE",
	"aws:eu-south-1":     "IT-NO",

	// AWS Asia Pacific
	"aws:ap-northeast-1": "JP-TK",
	"aws:ap-northeast-2": "KR",
	"aws:ap-northeast-3": "JP-KN",
	"aws:ap-southeast-1": "SG",
	"aws:ap-southeast-2": "AU-NSW",
	"aws:ap-south-1":     "IN-WE",

	// AWS Other
	"aws:ca-central-1":   "CA-ON",
	"aws:sa-east-1":      "BR-CS",

	// Azure US
	"azure:eastus":       "US-MIDA-PJM",
	"azure:eastus2":      "US-MIDA-PJM",
	"azure:westus":       "US-CAL-CISO",
	"azure:westus2":      "US-NW-PACW",
	"azure:centralus":    "US-MIDW-MISO",

	// Azure Europe
	"azure:westeurope":   "NL",
	"azure:northeurope":  "IE",
	"azure:uksouth":      "GB",
	"azure:ukwest":       "GB",
	"azure:francecentral": "FR",
	"azure:germanywestcentral": "DE",
	"azure:swedencentral": "SE",
	"azure:norwayeast":   "NO",

	// GCP US
	"gcp:us-east1":       "US-SE-SOCO",
	"gcp:us-east4":       "US-MIDA-PJM",
	"gcp:us-central1":    "US-MIDW-MISO",
	"gcp:us-west1":       "US-NW-PACW",
	"gcp:us-west2":       "US-CAL-CISO",
	"gcp:us-west3":       "US-SW-PNM",
	"gcp:us-west4":       "US-SW-NEVP",

	// GCP Europe
	"gcp:europe-west1":   "BE",
	"gcp:europe-west2":   "GB",
	"gcp:europe-west3":   "DE",
	"gcp:europe-west4":   "NL",
	"gcp:europe-west6":   "CH",
	"gcp:europe-north1":  "FI",

	// GCP Asia
	"gcp:asia-east1":     "TW",
	"gcp:asia-east2":     "HK",
	"gcp:asia-northeast1": "JP-TK",
	"gcp:asia-northeast2": "JP-KN",
	"gcp:asia-southeast1": "SG",
}

// Static carbon intensity data (gCO2/kWh) - 2024 averages
var staticIntensityData = map[string]float64{
	// North America
	"US-MIDA-PJM":   386,
	"US-CAL-CISO":   210,
	"US-NW-PACW":    180,
	"US-MIDW-MISO":  450,
	"US-SE-SOCO":    420,
	"US-SW-PNM":     380,
	"US-SW-NEVP":    350,
	"CA-ON":         35,
	"CA-QC":         5,
	"CA-BC":         12,

	// Europe
	"IE":            320,
	"GB":            225,
	"FR":            55,
	"DE":            380,
	"SE":            25,
	"NO":            20,
	"FI":            90,
	"NL":            325,
	"BE":            165,
	"CH":            30,
	"IT-NO":         310,

	// Asia Pacific
	"JP-TK":         470,
	"JP-KN":         450,
	"KR":            420,
	"SG":            395,
	"AU-NSW":        640,
	"AU-VIC":        620,
	"IN-WE":         680,
	"TW":            530,
	"HK":            600,

	// South America
	"BR-CS":         90,
	"BR-S":          120,
}

// =============================================================================
// COMPOSED CARBON STORE
// =============================================================================

// ComposedCarbonStore tries multiple sources in order
type ComposedCarbonStore struct {
	stores []CarbonStore
}

// NewComposedCarbonStore creates a composed store with fallback
func NewComposedCarbonStore(stores ...CarbonStore) *ComposedCarbonStore {
	return &ComposedCarbonStore{stores: stores}
}

// GetIntensity tries each store in order until one succeeds
func (c *ComposedCarbonStore) GetIntensity(ctx context.Context, cloud, region string) (float64, error) {
	var lastErr error
	for _, store := range c.stores {
		intensity, err := store.GetIntensity(ctx, cloud, region)
		if err == nil {
			return intensity, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

// =============================================================================
// FACTORY
// =============================================================================

// NewCarbonStore creates the appropriate carbon store based on configuration
func NewCarbonStore(electricityMapsAPIKey string) CarbonStore {
	stores := make([]CarbonStore, 0)

	// Add Electricity Maps if API key provided
	if electricityMapsAPIKey != "" {
		stores = append(stores, NewElectricityMapsClient(electricityMapsAPIKey))
	}

	// Always add static fallback
	stores = append(stores, NewStaticCarbonStore())

	if len(stores) == 1 {
		return stores[0]
	}

	return NewComposedCarbonStore(stores...)
}

// GetLowCarbonRegions returns regions with carbon intensity below threshold
func GetLowCarbonRegions(cloud string, thresholdGCO2 float64) []string {
	result := make([]string, 0)

	prefix := cloud + ":"
	for key := range regionToZoneMap {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			region := key[len(prefix):]
			zone := regionToZoneMap[key]
			if intensity, ok := staticIntensityData[zone]; ok && intensity < thresholdGCO2 {
				result = append(result, region)
			}
		}
	}

	return result
}
