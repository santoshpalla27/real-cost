// Package api defines the infrastructure graph model.
package api

// InfrastructureGraph represents parsed Terraform infrastructure.
type InfrastructureGraph struct {
	Nodes           []ResourceNode  `json:"nodes"`
	Edges           []Dependency    `json:"edges"`
	ProviderContext ProviderContext `json:"provider_context"`
}

// ResourceNode represents a single infrastructure resource.
type ResourceNode struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Provider     string         `json:"provider"`
	Region       string         `json:"region"`
	Attributes   map[string]any `json:"attributes"`
	ChangeAction ChangeAction   `json:"change_action"`
}

// Dependency represents a relationship between resources.
type Dependency struct {
	FromID string         `json:"from_id"`
	ToID   string         `json:"to_id"`
	Type   DependencyType `json:"type"`
}

// ProviderContext contains provider-level configuration.
type ProviderContext struct {
	Provider string `json:"provider"`
	Region   string `json:"region"`
	Account  string `json:"account,omitempty"`
}

// ChangeAction represents Terraform change types.
type ChangeAction string

const (
	ChangeActionCreate  ChangeAction = "create"
	ChangeActionUpdate  ChangeAction = "update"
	ChangeActionDelete  ChangeAction = "delete"
	ChangeActionNoOp    ChangeAction = "no-op"
	ChangeActionReplace ChangeAction = "replace"
)

// DependencyType represents relationship types.
type DependencyType string

const (
	DependencyTypeReference DependencyType = "reference"
	DependencyTypeImplicit  DependencyType = "implicit"
)
