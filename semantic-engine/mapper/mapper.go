package mapper

import (
	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/graph"
)

// ResourceMapper defines the contract for converting a provider resource into billing components.
type ResourceMapper interface {
	// Map transforms a single resource into billable components.
	Map(resource graph.Resource) ([]api.BillingComponent, error)
}
