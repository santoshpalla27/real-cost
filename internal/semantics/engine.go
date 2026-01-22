// Package semantics provides billing semantic extraction from infrastructure.
package semantics

import (
	"fmt"
	"strings"

	"github.com/santoshpalla27/fiac-platform/pkg/api"
	fiacerrors "github.com/santoshpalla27/fiac-platform/pkg/errors"
)

// Engine converts infrastructure resources to billing components.
type Engine struct {
	mappers map[string]ResourceMapper
}

// ResourceMapper defines the interface for resource-specific mapping.
type ResourceMapper interface {
	Map(node api.ResourceNode) ([]api.BillingComponent, error)
	Supports(resourceType string) bool
}

// NewEngine creates a new semantic engine with registered mappers.
func NewEngine() *Engine {
	e := &Engine{
		mappers: make(map[string]ResourceMapper),
	}
	
	// Register AWS mappers
	e.RegisterMapper("aws_instance", &EC2Mapper{})
	e.RegisterMapper("aws_ebs_volume", &EBSMapper{})
	e.RegisterMapper("aws_db_instance", &RDSMapper{})
	e.RegisterMapper("aws_s3_bucket", &S3Mapper{})
	e.RegisterMapper("aws_lambda_function", &LambdaMapper{})
	
	return e
}

// RegisterMapper adds a resource type mapper.
func (e *Engine) RegisterMapper(resourceType string, mapper ResourceMapper) {
	e.mappers[resourceType] = mapper
}

// Process converts an infrastructure graph to billing components.
func (e *Engine) Process(graph *api.InfrastructureGraph) (*api.SemanticResult, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph cannot be nil")
	}

	result := &api.SemanticResult{
		Components:    []api.BillingComponent{},
		MappingErrors: []api.MappingError{},
	}

	for _, node := range graph.Nodes {
		// Skip resources being deleted
		if node.ChangeAction == api.ChangeActionDelete {
			continue
		}

		mapper, exists := e.mappers[node.Type]
		if !exists {
			// Fail closed: unknown resources generate errors
			err := fiacerrors.NewUnknownResourceError(node.Type, node.ID)
			result.MappingErrors = append(result.MappingErrors, api.MappingError{
				Code:        err.Code,
				Message:     err.Message,
				Recoverable: false,
			})
			continue
		}

		components, err := mapper.Map(node)
		if err != nil {
			result.MappingErrors = append(result.MappingErrors, api.MappingError{
				Code:        fiacerrors.ErrCodeMissingAttribute,
				Message:     err.Error(),
				Recoverable: false,
			})
			continue
		}

		result.Components = append(result.Components, components...)
	}

	return result, nil
}

// EC2Mapper handles aws_instance resources.
type EC2Mapper struct{}

func (m *EC2Mapper) Supports(resourceType string) bool {
	return resourceType == "aws_instance"
}

func (m *EC2Mapper) Map(node api.ResourceNode) ([]api.BillingComponent, error) {
	components := []api.BillingComponent{}

	// Extract instance type
	instanceType, _ := node.Attributes["instance_type"].(string)
	if instanceType == "" {
		instanceType = "t3.micro" // conservative default
	}

	// Compute component (EC2 instance hours)
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:compute", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeCompute,
		UsageType:  api.UsageTypeOnDemand,
		Lifecycle:  api.LifecycleHourly,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "stable",
			Seasonality: "none",
			Confidence: 0.9,
		},
		Attributes: map[string]any{
			"instance_type": instanceType,
			"tenancy":       getStringAttr(node.Attributes, "tenancy", "default"),
		},
	})

	// Root EBS volume (if specified)
	if rootBlock, ok := node.Attributes["root_block_device"].([]interface{}); ok && len(rootBlock) > 0 {
		if rootDev, ok := rootBlock[0].(map[string]interface{}); ok {
			volumeSize := getFloat64Attr(rootDev, "volume_size", 8)
			volumeType := getStringAttr(rootDev, "volume_type", "gp3")

			components = append(components, api.BillingComponent{
				ID:         fmt.Sprintf("%s:root_storage", node.ID),
				ResourceID: node.ID,
				Type:       api.ComponentTypeStorage,
				UsageType:  api.UsageTypeProvisioned,
				Lifecycle:  api.LifecycleMonthly,
				VarianceProfile: api.VarianceProfile{
					Pattern:    "stable",
					Seasonality: "none",
					Confidence: 0.95,
				},
				Attributes: map[string]any{
					"volume_type": volumeType,
					"volume_size": volumeSize,
					"iops":        getFloat64Attr(rootDev, "iops", 3000),
					"throughput":  getFloat64Attr(rootDev, "throughput", 125),
				},
				Dependencies: []string{fmt.Sprintf("%s:compute", node.ID)},
			})
		}
	}

	// Network egress (estimated)
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:network", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeNetwork,
		UsageType:  api.UsageTypeOnDemand,
		Lifecycle:  api.LifecyclePerUnit,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "bursty",
			Seasonality: "daily",
			Confidence: 0.6, // Network is hard to predict
		},
		Attributes: map[string]any{
			"direction": "egress",
		},
		Dependencies: []string{fmt.Sprintf("%s:compute", node.ID)},
	})

	return components, nil
}

// EBSMapper handles aws_ebs_volume resources.
type EBSMapper struct{}

func (m *EBSMapper) Supports(resourceType string) bool {
	return resourceType == "aws_ebs_volume"
}

func (m *EBSMapper) Map(node api.ResourceNode) ([]api.BillingComponent, error) {
	volumeType := getStringAttr(node.Attributes, "type", "gp3")
	volumeSize := getFloat64Attr(node.Attributes, "size", 100)

	component := api.BillingComponent{
		ID:         fmt.Sprintf("%s:storage", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeStorage,
		UsageType:  api.UsageTypeProvisioned,
		Lifecycle:  api.LifecycleMonthly,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "stable",
			Seasonality: "none",
			Confidence: 0.95,
		},
		Attributes: map[string]any{
			"volume_type": volumeType,
			"volume_size": volumeSize,
			"iops":        getFloat64Attr(node.Attributes, "iops", 3000),
			"throughput":  getFloat64Attr(node.Attributes, "throughput", 125),
		},
	}

	return []api.BillingComponent{component}, nil
}

// RDSMapper handles aws_db_instance resources.
type RDSMapper struct{}

func (m *RDSMapper) Supports(resourceType string) bool {
	return resourceType == "aws_db_instance"
}

func (m *RDSMapper) Map(node api.ResourceNode) ([]api.BillingComponent, error) {
	components := []api.BillingComponent{}

	instanceClass := getStringAttr(node.Attributes, "instance_class", "db.t3.micro")
	engine := getStringAttr(node.Attributes, "engine", "mysql")
	storageSize := getFloat64Attr(node.Attributes, "allocated_storage", 20)
	multiAZ := getBoolAttr(node.Attributes, "multi_az", false)

	// Compute component
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:compute", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeCompute,
		UsageType:  api.UsageTypeOnDemand,
		Lifecycle:  api.LifecycleHourly,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "stable",
			Seasonality: "none",
			Confidence: 0.9,
		},
		Attributes: map[string]any{
			"instance_class": instanceClass,
			"engine":         engine,
			"multi_az":       multiAZ,
		},
	})

	// Storage component
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:storage", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeStorage,
		UsageType:  api.UsageTypeProvisioned,
		Lifecycle:  api.LifecycleMonthly,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "stable",
			Seasonality: "none",
			Confidence: 0.9,
		},
		Attributes: map[string]any{
			"storage_type": getStringAttr(node.Attributes, "storage_type", "gp2"),
			"storage_size": storageSize,
			"multi_az":     multiAZ,
		},
		Dependencies: []string{fmt.Sprintf("%s:compute", node.ID)},
	})

	return components, nil
}

// S3Mapper handles aws_s3_bucket resources.
type S3Mapper struct{}

func (m *S3Mapper) Supports(resourceType string) bool {
	return resourceType == "aws_s3_bucket"
}

func (m *S3Mapper) Map(node api.ResourceNode) ([]api.BillingComponent, error) {
	components := []api.BillingComponent{}

	// Storage component (S3 is usage-based, hard to predict)
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:storage", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeStorage,
		UsageType:  api.UsageTypeOnDemand,
		Lifecycle:  api.LifecyclePerUnit,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "predictable",
			Seasonality: "monthly",
			Confidence: 0.5, // S3 usage is hard to predict from plan
		},
		Attributes: map[string]any{
			"storage_class": "STANDARD",
		},
	})

	// Request component
	components = append(components, api.BillingComponent{
		ID:         fmt.Sprintf("%s:requests", node.ID),
		ResourceID: node.ID,
		Type:       api.ComponentTypeData,
		UsageType:  api.UsageTypeOnDemand,
		Lifecycle:  api.LifecyclePerUnit,
		VarianceProfile: api.VarianceProfile{
			Pattern:    "bursty",
			Seasonality: "daily",
			Confidence: 0.4,
		},
		Attributes: map[string]any{},
		Dependencies: []string{fmt.Sprintf("%s:storage", node.ID)},
	})

	return components, nil
}

// LambdaMapper handles aws_lambda_function resources.
type LambdaMapper struct{}

func (m *LambdaMapper) Supports(resourceType string) bool {
	return resourceType == "aws_lambda_function"
}

func (m *LambdaMapper) Map(node api.ResourceNode) ([]api.BillingComponent, error) {
	memorySize := getFloat64Attr(node.Attributes, "memory_size", 128)
	
	return []api.BillingComponent{
		{
			ID:         fmt.Sprintf("%s:invocations", node.ID),
			ResourceID: node.ID,
			Type:       api.ComponentTypeCompute,
			UsageType:  api.UsageTypeOnDemand,
			Lifecycle:  api.LifecyclePerUnit,
			VarianceProfile: api.VarianceProfile{
				Pattern:    "bursty",
				Seasonality: "daily",
				Confidence: 0.4, // Lambda usage very hard to predict
			},
			Attributes: map[string]any{
				"memory_size": memorySize,
				"runtime":     getStringAttr(node.Attributes, "runtime", "nodejs18.x"),
			},
		},
	}, nil
}

// Helper functions
func getStringAttr(attrs map[string]any, key, defaultVal string) string {
	if v, ok := attrs[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}

func getFloat64Attr(attrs map[string]any, key string, defaultVal float64) float64 {
	if v, ok := attrs[key].(float64); ok {
		return v
	}
	if v, ok := attrs[key].(int); ok {
		return float64(v)
	}
	return defaultVal
}

func getBoolAttr(attrs map[string]any, key string, defaultVal bool) bool {
	if v, ok := attrs[key].(bool); ok {
		return v
	}
	return defaultVal
}

// isSupportedType checks if a resource type prefix is supported
func isSupportedType(resourceType string) bool {
	supportedPrefixes := []string{"aws_", "azurerm_", "google_"}
	for _, prefix := range supportedPrefixes {
		if strings.HasPrefix(resourceType, prefix) {
			return true
		}
	}
	return false
}
