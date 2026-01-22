// Package billing provides the Billing Semantic Engine
// Converts infrastructure resources into atomic billing components
package billing

import (
	"fmt"
	"strings"

	"terraform-cost/decision/iac"
)

// BillingPeriod represents the billing frequency
type BillingPeriod string

const (
	PeriodHourly    BillingPeriod = "hourly"
	PeriodDaily     BillingPeriod = "daily"
	PeriodMonthly   BillingPeriod = "monthly"
	PeriodPerRequest BillingPeriod = "per_request"
	PeriodPerGB     BillingPeriod = "per_gb"
	PeriodPerUnit   BillingPeriod = "per_unit"
)

// BillingComponent represents an atomic billable unit
type BillingComponent struct {
	// Identity
	ID           string `json:"id"`
	ResourceAddr string `json:"resource_addr"` // Source Terraform resource
	
	// Billing dimensions
	Cloud         string            `json:"cloud"`          // aws, azure, gcp
	Service       string            `json:"service"`        // AmazonEC2, Virtual Machines
	ProductFamily string            `json:"product_family"` // Compute Instance, Storage
	Region        string            `json:"region"`
	UsageType     string            `json:"usage_type"`     // BoxUsage:t3.medium
	BillingPeriod BillingPeriod     `json:"billing_period"`
	Attributes    map[string]string `json:"attributes"`     // instanceType, os, etc.
	
	// Variance profile for usage prediction
	VarianceProfile VarianceProfile `json:"variance_profile"`
	
	// Metadata
	Description string   `json:"description"`
	Tags        []string `json:"tags"` // compute, storage, network, etc.
	
	// Dependencies
	DependsOn []string `json:"depends_on"` // Other component IDs
}

// VarianceProfile models usage uncertainty
type VarianceProfile struct {
	// Usage distribution
	BaselineUsage float64 `json:"baseline_usage"` // Expected usage per period
	MinUsage      float64 `json:"min_usage"`      // Minimum possible usage
	MaxUsage      float64 `json:"max_usage"`      // Maximum possible usage
	P50Usage      float64 `json:"p50_usage"`      // Median usage
	P90Usage      float64 `json:"p90_usage"`      // 90th percentile usage
	
	// Risk factors
	Confidence    float64  `json:"confidence"`    // 0-1 confidence in prediction
	VolatilityScore float64 `json:"volatility"`  // How variable is usage
	Assumptions   []string `json:"assumptions"`   // What we assumed
}

// MappingError represents a failure to map a resource
type MappingError struct {
	ResourceAddr string `json:"resource_addr"`
	ResourceType string `json:"resource_type"`
	Reason       string `json:"reason"`
	IsCritical   bool   `json:"is_critical"` // Should abort estimation?
}

func (e MappingError) Error() string {
	return fmt.Sprintf("mapping error for %s: %s", e.ResourceAddr, e.Reason)
}

// ResourceMapper maps a Terraform resource to billing components
type ResourceMapper interface {
	// ResourceType returns the Terraform resource type this mapper handles
	ResourceType() string
	
	// MapToBillingComponents converts a resource to billing components
	// Returns components and any mapping errors (may return both)
	MapToBillingComponents(node *iac.GraphNode) ([]BillingComponent, []MappingError)
	
	// SupportedAttributes returns attributes this mapper uses
	SupportedAttributes() []string
}

// Engine is the Billing Semantic Engine
type Engine struct {
	mappers  map[string]ResourceMapper
	registry *MapperRegistry
}

// NewEngine creates a new Billing Semantic Engine
func NewEngine() *Engine {
	return &Engine{
		mappers:  make(map[string]ResourceMapper),
		registry: NewMapperRegistry(),
	}
}

// RegisterMapper adds a resource mapper
func (e *Engine) RegisterMapper(m ResourceMapper) {
	e.mappers[m.ResourceType()] = m
}

// RegisterMappers adds multiple mappers
func (e *Engine) RegisterMappers(mappers ...ResourceMapper) {
	for _, m := range mappers {
		e.RegisterMapper(m)
	}
}

// DecompositionResult contains the result of decomposing a graph
type DecompositionResult struct {
	Components    []BillingComponent `json:"components"`
	MappingErrors []MappingError     `json:"mapping_errors"`
	
	// Statistics
	ResourcesProcessed int `json:"resources_processed"`
	ResourcesMapped    int `json:"resources_mapped"`
	ResourcesSkipped   int `json:"resources_skipped"`
	ComponentsCreated  int `json:"components_created"`
	
	// Coverage
	CoveredTypes   []string `json:"covered_types"`
	UncoveredTypes []string `json:"uncovered_types"`
}

// Decompose converts an infrastructure graph into billing components
func (e *Engine) Decompose(graph *iac.Graph) (*DecompositionResult, error) {
	result := &DecompositionResult{
		Components:    make([]BillingComponent, 0),
		MappingErrors: make([]MappingError, 0),
		CoveredTypes:  make([]string, 0),
		UncoveredTypes: make([]string, 0),
	}
	
	coveredTypesMap := make(map[string]bool)
	uncoveredTypesMap := make(map[string]bool)
	
	// Process each node in topological order for dependency tracking
	nodes, err := graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to sort graph: %w", err)
	}
	
	componentsByResource := make(map[string][]string) // addr -> component IDs
	
	for _, node := range nodes {
		result.ResourcesProcessed++
		
		// Skip non-billable modes
		if node.Resource.Mode == "data" {
			result.ResourcesSkipped++
			continue
		}
		
		// Find mapper for this resource type
		mapper := e.findMapper(node.Resource.Type)
		if mapper == nil {
			// No mapper - record as uncovered
			uncoveredTypesMap[node.Resource.Type] = true
			result.MappingErrors = append(result.MappingErrors, MappingError{
				ResourceAddr: node.Resource.Address,
				ResourceType: node.Resource.Type,
				Reason:       "no mapper registered for resource type",
				IsCritical:   false,
			})
			continue
		}
		
		// Map to billing components
		components, mappingErrors := mapper.MapToBillingComponents(node)
		
		// Track mapping errors
		result.MappingErrors = append(result.MappingErrors, mappingErrors...)
		
		if len(components) > 0 {
			result.ResourcesMapped++
			coveredTypesMap[node.Resource.Type] = true
			
			// Process each component
			for i := range components {
				comp := &components[i]
				
				// Generate ID if not set
				if comp.ID == "" {
					comp.ID = fmt.Sprintf("%s-%d", node.Resource.Address, i)
				}
				
				// Set resource address
				comp.ResourceAddr = node.Resource.Address
				
				// Resolve component dependencies from resource dependencies
				comp.DependsOn = e.resolveComponentDependencies(node, componentsByResource)
				
				result.Components = append(result.Components, *comp)
				result.ComponentsCreated++
				
				// Track for dependency resolution
				componentsByResource[node.Resource.Address] = append(
					componentsByResource[node.Resource.Address], comp.ID)
			}
		}
	}
	
	// Collect covered/uncovered types
	for t := range coveredTypesMap {
		result.CoveredTypes = append(result.CoveredTypes, t)
	}
	for t := range uncoveredTypesMap {
		if !coveredTypesMap[t] {
			result.UncoveredTypes = append(result.UncoveredTypes, t)
		}
	}
	
	return result, nil
}

// findMapper finds the appropriate mapper for a resource type
func (e *Engine) findMapper(resourceType string) ResourceMapper {
	// Exact match first
	if m, ok := e.mappers[resourceType]; ok {
		return m
	}
	
	// Try registry
	if e.registry != nil {
		return e.registry.GetMapper(resourceType)
	}
	
	return nil
}

// resolveComponentDependencies maps resource dependencies to component IDs
func (e *Engine) resolveComponentDependencies(node *iac.GraphNode, lookup map[string][]string) []string {
	deps := make([]string, 0)
	
	for _, depAddr := range node.Dependencies {
		if compIDs, ok := lookup[depAddr]; ok {
			deps = append(deps, compIDs...)
		}
	}
	
	return deps
}

// MapperRegistry provides centralized mapper registration
type MapperRegistry struct {
	mappers map[string]ResourceMapper
	aliases map[string]string // alias -> canonical type
}

// NewMapperRegistry creates a new mapper registry
func NewMapperRegistry() *MapperRegistry {
	return &MapperRegistry{
		mappers: make(map[string]ResourceMapper),
		aliases: make(map[string]string),
	}
}

// Register adds a mapper to the registry
func (r *MapperRegistry) Register(m ResourceMapper) {
	r.mappers[m.ResourceType()] = m
}

// RegisterAlias creates an alias for a resource type
func (r *MapperRegistry) RegisterAlias(alias, canonical string) {
	r.aliases[alias] = canonical
}

// GetMapper retrieves a mapper for a resource type
func (r *MapperRegistry) GetMapper(resourceType string) ResourceMapper {
	// Check direct registration
	if m, ok := r.mappers[resourceType]; ok {
		return m
	}
	
	// Check aliases
	if canonical, ok := r.aliases[resourceType]; ok {
		if m, ok := r.mappers[canonical]; ok {
			return m
		}
	}
	
	return nil
}

// ListMappers returns all registered mappers
func (r *MapperRegistry) ListMappers() []ResourceMapper {
	result := make([]ResourceMapper, 0, len(r.mappers))
	for _, m := range r.mappers {
		result = append(result, m)
	}
	return result
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ExtractAttribute safely extracts a string attribute
func ExtractAttribute(attrs map[string]interface{}, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ExtractAttributeInt safely extracts an integer attribute
func ExtractAttributeInt(attrs map[string]interface{}, key string, defaultVal int) int {
	if v, ok := attrs[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case int64:
			return int(val)
		}
	}
	return defaultVal
}

// ExtractAttributeFloat safely extracts a float attribute
func ExtractAttributeFloat(attrs map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := attrs[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case int64:
			return float64(val)
		}
	}
	return defaultVal
}

// ExtractAttributeBool safely extracts a boolean attribute
func ExtractAttributeBool(attrs map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := attrs[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// ExtractNestedAttribute extracts a nested attribute using dot notation
func ExtractNestedAttribute(attrs map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(attrs)
	
	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else if arr, ok := current.([]interface{}); ok {
			// Handle array index
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx < len(arr) {
				current = arr[idx]
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	
	return current
}

// NewDefaultVarianceProfile creates a default variance profile
func NewDefaultVarianceProfile(baselineHours float64) VarianceProfile {
	return VarianceProfile{
		BaselineUsage: baselineHours,
		MinUsage:      baselineHours * 0.8,
		MaxUsage:      baselineHours * 1.0,
		P50Usage:      baselineHours * 0.9,
		P90Usage:      baselineHours * 1.0,
		Confidence:    0.85,
		VolatilityScore: 0.1,
		Assumptions: []string{
			"Assumed 24/7 operation",
			"No scaling events",
		},
	}
}

// NewEnvironmentVarianceProfile creates environment-aware variance profile
func NewEnvironmentVarianceProfile(env string, fullUsage float64) VarianceProfile {
	switch strings.ToLower(env) {
	case "production", "prod":
		return VarianceProfile{
			BaselineUsage: fullUsage,
			MinUsage:      fullUsage * 0.95,
			MaxUsage:      fullUsage,
			P50Usage:      fullUsage * 0.98,
			P90Usage:      fullUsage,
			Confidence:    0.95,
			VolatilityScore: 0.05,
			Assumptions:   []string{"Production: 24/7 operation assumed"},
		}
	case "staging", "stage":
		return VarianceProfile{
			BaselineUsage: fullUsage * 0.5,
			MinUsage:      fullUsage * 0.3,
			MaxUsage:      fullUsage * 0.7,
			P50Usage:      fullUsage * 0.5,
			P90Usage:      fullUsage * 0.65,
			Confidence:    0.8,
			VolatilityScore: 0.25,
			Assumptions:   []string{"Staging: ~50% of production usage assumed"},
		}
	case "development", "dev":
		return VarianceProfile{
			BaselineUsage: fullUsage * 0.2,
			MinUsage:      fullUsage * 0.1,
			MaxUsage:      fullUsage * 0.4,
			P50Usage:      fullUsage * 0.2,
			P90Usage:      fullUsage * 0.35,
			Confidence:    0.7,
			VolatilityScore: 0.4,
			Assumptions:   []string{"Development: ~20% of production usage, business hours only"},
		}
	default:
		return NewDefaultVarianceProfile(fullUsage)
	}
}
