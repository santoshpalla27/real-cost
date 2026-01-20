package graph

// Resource represents a single node in the infrastructure graph.
// It corresponds to a Terraform resource but with normalized attributes.
type Resource struct {
	Address      string                 `json:"address"`       // e.g., "aws_instance.web"
	Type         string                 `json:"type"`          // e.g., "aws_instance"
	Name         string                 `json:"name"`          // e.g., "web"
	Provider     string                 `json:"provider"`      // e.g., "aws"
	Region       string                 `json:"region"`        // e.g., "us-east-1"
	Attributes   map[string]interface{} `json:"attributes"`    // Raw attributes from the plan
	Dependencies []string               `json:"dependencies"`  // Addresses of dependencies
}

// InfrastructureGraph implies the Directed Acyclic Graph of resources.
type InfrastructureGraph struct {
	Resources []Resource `json:"resources"`
	Root      string     `json:"root"` // Module root
}
