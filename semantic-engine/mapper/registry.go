package mapper

import (
	"fmt"
	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/graph"
)

// Registry holds all registered resource mappers
type Registry struct {
	Mappers map[string]ResourceMapper
}

func NewRegistry() *Registry {
	return &Registry{
		Mappers: make(map[string]ResourceMapper),
	}
}

func (r *Registry) Register(resourceType string, mapper ResourceMapper) {
	r.Mappers[resourceType] = mapper
}

func (r *Registry) MapResource(res graph.Resource) ([]api.BillingComponent, error) {
	mapper, exists := r.Mappers[res.Type]
	if !exists {
		// FAIL CLOSED: Do not ignore unknown resources.
		// Returns a component that will trigger a Mapping Error in validation.
		return []api.BillingComponent{{
			ResourceAddress: res.Address,
			Provider:        res.Provider,
			UsageType:       "UNSUPPORTED_RESOURCE",
			MappingError:    fmt.Sprintf("No mapper registered for resource type: %s", res.Type),
		}}, nil
	}

	return mapper.Map(res)
}
