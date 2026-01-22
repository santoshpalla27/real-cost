// Package iac provides infrastructure graph building from parsed Terraform plans
package iac

import (
	"fmt"
	"strings"
)

// Graph represents the infrastructure dependency graph
type Graph struct {
	Nodes    map[string]*GraphNode
	Edges    map[string][]string // address -> dependent addresses
	Roots    []string            // Nodes with no dependencies
	Leaves   []string            // Nodes with no dependents
	
	// Computed properties
	ResourceCount int
	ProviderStats map[string]int // provider -> count
	RegionStats   map[string]int // region -> count
	ChangeStats   ChangeStatistics
}

// GraphNode represents a node in the infrastructure graph
type GraphNode struct {
	Resource     ResourceNode
	Change       *ResourceChange
	Dependencies []string // Addresses this node depends on
	Dependents   []string // Addresses that depend on this node
	Depth        int      // Distance from root
	Provider     string
	Region       string
}

// ChangeStatistics summarizes planned changes
type ChangeStatistics struct {
	Creates  int
	Updates  int
	Deletes  int
	Replaces int
	NoOps    int
	Total    int
}

// GraphBuilder builds infrastructure graphs from parsed plans
type GraphBuilder struct {
	includeDataSources bool
	resolveImplicit    bool
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		includeDataSources: false, // Data sources don't have cost
		resolveImplicit:    true,  // Resolve implicit dependencies
	}
}

// WithDataSources includes data sources in the graph
func (b *GraphBuilder) WithDataSources(include bool) *GraphBuilder {
	b.includeDataSources = include
	return b
}

// Build creates an infrastructure graph from a parsed plan
func (b *GraphBuilder) Build(plan *ParsedPlan) (*Graph, error) {
	g := &Graph{
		Nodes:         make(map[string]*GraphNode),
		Edges:         make(map[string][]string),
		Roots:         make([]string, 0),
		Leaves:        make([]string, 0),
		ProviderStats: make(map[string]int),
		RegionStats:   make(map[string]int),
	}
	
	// Build change lookup
	changeByAddr := make(map[string]*ResourceChange)
	for i := range plan.Changes {
		changeByAddr[plan.Changes[i].Address] = &plan.Changes[i]
	}
	
	// Create nodes from resources
	for _, resource := range plan.Resources {
		// Skip data sources unless configured
		if resource.Mode == "data" && !b.includeDataSources {
			continue
		}
		
		node := &GraphNode{
			Resource:     resource,
			Change:       changeByAddr[resource.Address],
			Dependencies: make([]string, 0),
			Dependents:   make([]string, 0),
			Provider:     resource.Provider,
			Region:       resource.Region,
		}
		
		g.Nodes[resource.Address] = node
		g.ResourceCount++
		
		// Track statistics
		g.ProviderStats[resource.Provider]++
		if resource.Region != "" {
			g.RegionStats[resource.Region]++
		}
	}
	
	// Build dependency edges
	for addr, deps := range plan.Dependencies {
		node, exists := g.Nodes[addr]
		if !exists {
			continue
		}
		
		for _, depAddr := range deps {
			depNode, depExists := g.Nodes[depAddr]
			if !depExists {
				continue // Dependency might be a data source we excluded
			}
			
			// Add forward edge
			node.Dependencies = append(node.Dependencies, depAddr)
			g.Edges[addr] = append(g.Edges[addr], depAddr)
			
			// Add reverse edge
			depNode.Dependents = append(depNode.Dependents, addr)
		}
	}
	
	// Resolve implicit dependencies (if enabled)
	if b.resolveImplicit {
		b.resolveImplicitDependencies(g)
	}
	
	// Identify roots and leaves
	for addr, node := range g.Nodes {
		if len(node.Dependencies) == 0 {
			g.Roots = append(g.Roots, addr)
		}
		if len(node.Dependents) == 0 {
			g.Leaves = append(g.Leaves, addr)
		}
	}
	
	// Calculate depths
	b.calculateDepths(g)
	
	// Calculate change statistics
	g.ChangeStats = b.calculateChangeStats(g)
	
	return g, nil
}

// resolveImplicitDependencies finds implicit dependencies based on attribute references
func (b *GraphBuilder) resolveImplicitDependencies(g *Graph) {
	// Build address lookup for reference resolution
	addressLookup := make(map[string]string) // partial addr -> full addr
	for addr := range g.Nodes {
		// aws_instance.web -> aws_instance.web
		addressLookup[addr] = addr
		
		// Also index by type.name (without module prefix)
		parts := strings.Split(addr, ".")
		if len(parts) >= 2 {
			shortAddr := parts[len(parts)-2] + "." + parts[len(parts)-1]
			addressLookup[shortAddr] = addr
		}
	}
	
	// Scan attributes for references
	for addr, node := range g.Nodes {
		refs := b.findAttributeReferences(node.Resource.Attributes, addressLookup)
		
		for _, refAddr := range refs {
			if refAddr == addr {
				continue // Skip self-references
			}
			
			refNode, exists := g.Nodes[refAddr]
			if !exists {
				continue
			}
			
			// Check if already a dependency
			if containsString(node.Dependencies, refAddr) {
				continue
			}
			
			// Add implicit dependency
			node.Dependencies = append(node.Dependencies, refAddr)
			g.Edges[addr] = append(g.Edges[addr], refAddr)
			refNode.Dependents = append(refNode.Dependents, addr)
		}
	}
}

// findAttributeReferences scans attributes for resource references
func (b *GraphBuilder) findAttributeReferences(attrs map[string]interface{}, lookup map[string]string) []string {
	refs := make([]string, 0)
	
	var scan func(v interface{})
	scan = func(v interface{}) {
		switch val := v.(type) {
		case string:
			// Look for patterns like aws_instance.web.id
			for partial, full := range lookup {
				if strings.Contains(val, partial) {
					refs = append(refs, full)
				}
			}
		case map[string]interface{}:
			for _, vv := range val {
				scan(vv)
			}
		case []interface{}:
			for _, vv := range val {
				scan(vv)
			}
		}
	}
	
	for _, v := range attrs {
		scan(v)
	}
	
	return refs
}

// calculateDepths calculates the depth of each node from roots
func (b *GraphBuilder) calculateDepths(g *Graph) {
	visited := make(map[string]bool)
	
	var visit func(addr string, depth int)
	visit = func(addr string, depth int) {
		if visited[addr] {
			return
		}
		visited[addr] = true
		
		node := g.Nodes[addr]
		if node.Depth < depth {
			node.Depth = depth
		}
		
		for _, depAddr := range node.Dependents {
			visit(depAddr, depth+1)
		}
	}
	
	// Start from roots
	for _, root := range g.Roots {
		visit(root, 0)
	}
}

// calculateChangeStats computes change statistics
func (b *GraphBuilder) calculateChangeStats(g *Graph) ChangeStatistics {
	stats := ChangeStatistics{}
	
	for _, node := range g.Nodes {
		if node.Change == nil {
			stats.NoOps++
			continue
		}
		
		switch node.Change.Action {
		case ActionCreate:
			stats.Creates++
		case ActionUpdate:
			stats.Updates++
		case ActionDelete:
			stats.Deletes++
		case ActionReplace:
			stats.Replaces++
		default:
			stats.NoOps++
		}
	}
	
	stats.Total = stats.Creates + stats.Updates + stats.Deletes + stats.Replaces + stats.NoOps
	return stats
}

// GetResourcesByProvider groups resources by provider
func (g *Graph) GetResourcesByProvider() map[string][]*GraphNode {
	result := make(map[string][]*GraphNode)
	for _, node := range g.Nodes {
		result[node.Provider] = append(result[node.Provider], node)
	}
	return result
}

// GetResourcesByRegion groups resources by region
func (g *Graph) GetResourcesByRegion() map[string][]*GraphNode {
	result := make(map[string][]*GraphNode)
	for _, node := range g.Nodes {
		region := node.Region
		if region == "" {
			region = "unknown"
		}
		result[region] = append(result[region], node)
	}
	return result
}

// GetResourcesByType groups resources by type
func (g *Graph) GetResourcesByType() map[string][]*GraphNode {
	result := make(map[string][]*GraphNode)
	for _, node := range g.Nodes {
		result[node.Resource.Type] = append(result[node.Resource.Type], node)
	}
	return result
}

// GetChangedResources returns only resources with changes
func (g *Graph) GetChangedResources() []*GraphNode {
	result := make([]*GraphNode, 0)
	for _, node := range g.Nodes {
		if node.Change != nil && node.Change.Action != ActionNoOp {
			result = append(result, node)
		}
	}
	return result
}

// GetCreatedResources returns resources being created
func (g *Graph) GetCreatedResources() []*GraphNode {
	result := make([]*GraphNode, 0)
	for _, node := range g.Nodes {
		if node.Change != nil && (node.Change.Action == ActionCreate || node.Change.Action == ActionReplace) {
			result = append(result, node)
		}
	}
	return result
}

// TopologicalSort returns nodes in dependency order
func (g *Graph) TopologicalSort() ([]*GraphNode, error) {
	result := make([]*GraphNode, 0, len(g.Nodes))
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	
	var visit func(addr string) error
	visit = func(addr string) error {
		if visited[addr] {
			return nil
		}
		if visiting[addr] {
			return fmt.Errorf("circular dependency detected at %s", addr)
		}
		
		visiting[addr] = true
		node := g.Nodes[addr]
		
		for _, depAddr := range node.Dependencies {
			if err := visit(depAddr); err != nil {
				return err
			}
		}
		
		visiting[addr] = false
		visited[addr] = true
		result = append(result, node)
		
		return nil
	}
	
	for addr := range g.Nodes {
		if err := visit(addr); err != nil {
			return nil, err
		}
	}
	
	return result, nil
}

// String returns a summary of the graph
func (g *Graph) String() string {
	return fmt.Sprintf(
		"InfrastructureGraph: %d resources (%d creates, %d updates, %d deletes) across %d providers, %d regions",
		g.ResourceCount,
		g.ChangeStats.Creates,
		g.ChangeStats.Updates,
		g.ChangeStats.Deletes,
		len(g.ProviderStats),
		len(g.RegionStats),
	)
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
