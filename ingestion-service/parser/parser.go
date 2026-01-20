package parser

import (
	"encoding/json"
	"fmt"

	"github.com/futuristic-iac/pkg/graph"
)

// Parse converts a Terraform Plan JSON into a normalized InfrastructureGraph.
func Parse(planJSON []byte) (*graph.InfrastructureGraph, error) {
	var plan TFPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	g := &graph.InfrastructureGraph{
		Resources: []graph.Resource{},
	}

	// Index Configuration Dependencies
	// Map[ResourceAddress] -> []DependsOnAddresses
	dependencies := make(map[string][]string)
	if plan.FollowsConfig() {
		for _, res := range plan.Config.RootModule.Resources {
			// address usually matches, but config address might differ slightly from plan address (module prefixes)
			// For MVP, assuming match.
			if len(res.DependsOn) > 0 {
				dependencies[res.Address] = res.DependsOn
			}
		}
	}

	// Iterate over resource changes to build the graph nodes
	for _, rc := range plan.ResourceChanges {
		if rc.Change.Actions[0] == "delete" {
			continue // Start with ignoring deletes for MVP
		}

		// Handle "attributes" being nil in some cases (e.g. unknown)
		attrs, ok := rc.Change.After.(map[string]interface{})
		if !ok {
			attrs = make(map[string]interface{})
		}

		res := graph.Resource{
			Address:      rc.Address,
			Type:         rc.Type,
			Name:         rc.Name,
			Provider:     rc.ProviderName,
			Attributes:   attrs,
			Dependencies: []string{},
		}
		
		// Attach dependencies if found in config
		if deps, found := dependencies[rc.Address]; found {
			res.Dependencies = deps
		}
		
		g.Resources = append(g.Resources, res)
	}

	return g, nil
}
