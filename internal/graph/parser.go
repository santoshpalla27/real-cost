// Package graph provides Terraform plan parsing and infrastructure graph building.
package graph

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santoshpalla27/fiac-platform/pkg/api"
)

// TerraformPlan represents the structure of terraform show -json output.
type TerraformPlan struct {
	FormatVersion    string                 `json:"format_version"`
	TerraformVersion string                 `json:"terraform_version"`
	PlannedValues    PlannedValues          `json:"planned_values"`
	ResourceChanges  []ResourceChange       `json:"resource_changes"`
	Configuration    map[string]interface{} `json:"configuration"`
	PriorState       *PriorState            `json:"prior_state,omitempty"`
}

// PlannedValues contains the planned infrastructure state.
type PlannedValues struct {
	RootModule Module `json:"root_module"`
}

// Module represents a Terraform module.
type Module struct {
	Resources    []PlannedResource `json:"resources"`
	ChildModules []Module          `json:"child_modules,omitempty"`
}

// PlannedResource represents a planned resource.
type PlannedResource struct {
	Address       string                 `json:"address"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	ProviderName  string                 `json:"provider_name"`
	SchemaVersion int                    `json:"schema_version"`
	Values        map[string]interface{} `json:"values"`
}

// ResourceChange represents a change to a resource.
type ResourceChange struct {
	Address      string   `json:"address"`
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	ProviderName string   `json:"provider_name"`
	Change       Change   `json:"change"`
}

// Change represents the before/after of a resource change.
type Change struct {
	Actions []string               `json:"actions"`
	Before  map[string]interface{} `json:"before"`
	After   map[string]interface{} `json:"after"`
}

// PriorState represents existing infrastructure state.
type PriorState struct {
	Values PlannedValues `json:"values"`
}

// ParseTerraformPlan parses JSON plan data into an infrastructure graph.
func ParseTerraformPlan(planData []byte) (*api.InfrastructureGraph, error) {
	var plan TerraformPlan
	if err := json.Unmarshal(planData, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse terraform plan JSON: %w", err)
	}

	graph := &api.InfrastructureGraph{
		Nodes: []api.ResourceNode{},
		Edges: []api.Dependency{},
	}

	// Extract provider context from first resource
	if len(plan.ResourceChanges) > 0 {
		graph.ProviderContext = extractProviderContext(plan.ResourceChanges[0])
	}

	// Process resource changes
	for _, rc := range plan.ResourceChanges {
		node := buildResourceNode(rc)
		graph.Nodes = append(graph.Nodes, node)
	}

	// Extract dependencies from configuration references
	graph.Edges = extractDependencies(plan, graph.Nodes)

	return graph, nil
}

func extractProviderContext(rc ResourceChange) api.ProviderContext {
	provider := "aws" // default
	region := "us-east-1" // default

	// Extract provider from provider_name (e.g., "registry.terraform.io/hashicorp/aws")
	parts := strings.Split(rc.ProviderName, "/")
	if len(parts) > 0 {
		provider = parts[len(parts)-1]
	}

	// Try to extract region from resource attributes
	if rc.Change.After != nil {
		if r, ok := rc.Change.After["region"].(string); ok && r != "" {
			region = r
		}
		// For AWS resources, check availability_zone
		if az, ok := rc.Change.After["availability_zone"].(string); ok && az != "" {
			// Extract region from AZ (e.g., "us-west-2a" -> "us-west-2")
			if len(az) > 1 {
				region = az[:len(az)-1]
			}
		}
	}

	return api.ProviderContext{
		Provider: provider,
		Region:   region,
	}
}

func buildResourceNode(rc ResourceChange) api.ResourceNode {
	// Determine change action
	action := api.ChangeActionNoOp
	if len(rc.Change.Actions) > 0 {
		switch rc.Change.Actions[0] {
		case "create":
			action = api.ChangeActionCreate
		case "update":
			action = api.ChangeActionUpdate
		case "delete":
			action = api.ChangeActionDelete
		case "replace":
			action = api.ChangeActionReplace
		}
		// Handle create-then-destroy and similar combinations
		if len(rc.Change.Actions) > 1 {
			action = api.ChangeActionReplace
		}
	}

	// Use "after" values for creates/updates, "before" for deletes
	attributes := rc.Change.After
	if action == api.ChangeActionDelete {
		attributes = rc.Change.Before
	}
	if attributes == nil {
		attributes = make(map[string]interface{})
	}

	return api.ResourceNode{
		ID:           rc.Address,
		Type:         rc.Type,
		Name:         rc.Name,
		Provider:     extractProvider(rc.ProviderName),
		Attributes:   attributes,
		ChangeAction: action,
	}
}

func extractProvider(providerName string) string {
	parts := strings.Split(providerName, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return providerName
}

func extractDependencies(plan TerraformPlan, nodes []api.ResourceNode) []api.Dependency {
	deps := []api.Dependency{}
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}

	// Extract dependencies from attribute references (recursive)
	for _, node := range nodes {
		foundDeps := findReferencesInValue(node.Attributes, node.ID, nodeIDs)
		deps = append(deps, foundDeps...)
	}

	// Deduplicate dependencies
	seen := make(map[string]bool)
	unique := []api.Dependency{}
	for _, d := range deps {
		key := d.FromID + "->" + d.ToID
		if !seen[key] {
			seen[key] = true
			unique = append(unique, d)
		}
	}

	return unique
}

// findReferencesInValue recursively searches for resource references in attribute values
func findReferencesInValue(value interface{}, currentNodeID string, nodeIDs map[string]bool) []api.Dependency {
	deps := []api.Dependency{}

	switch v := value.(type) {
	case string:
		// Check for direct references like "aws_instance.web" or "${aws_instance.web.id}"
		for otherID := range nodeIDs {
			if otherID != currentNodeID && strings.Contains(v, otherID) {
				deps = append(deps, api.Dependency{
					FromID: currentNodeID,
					ToID:   otherID,
					Type:   api.DependencyTypeReference,
				})
			}
		}
	case map[string]interface{}:
		for _, subVal := range v {
			deps = append(deps, findReferencesInValue(subVal, currentNodeID, nodeIDs)...)
		}
	case []interface{}:
		for _, item := range v {
			deps = append(deps, findReferencesInValue(item, currentNodeID, nodeIDs)...)
		}
	}

	return deps
}
