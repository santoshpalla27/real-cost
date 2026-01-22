// Package iac provides Terraform plan parsing and infrastructure graph building
// This is the entry point for the Decision Plane - all IaC inputs flow through here
package iac

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ChangeAction represents the type of change to a resource
type ChangeAction string

const (
	ActionCreate  ChangeAction = "create"
	ActionUpdate  ChangeAction = "update"
	ActionDelete  ChangeAction = "delete"
	ActionNoOp    ChangeAction = "no-op"
	ActionRead    ChangeAction = "read"
	ActionReplace ChangeAction = "replace"
)

// ParsedPlan represents a fully parsed Terraform plan
type ParsedPlan struct {
	// Metadata
	FormatVersion   string `json:"format_version"`
	TerraformVersion string `json:"terraform_version"`
	
	// Resources
	Resources    []ResourceNode    `json:"resources"`
	Dependencies map[string][]string `json:"dependencies"`
	
	// Changes
	Changes []ResourceChange `json:"changes"`
	
	// Provider configuration
	Providers map[string]ProviderConfig `json:"providers"`
	
	// Variables
	Variables map[string]interface{} `json:"variables"`
	
	// Outputs
	Outputs map[string]OutputValue `json:"outputs"`
}

// ResourceNode represents a single infrastructure resource
type ResourceNode struct {
	// Identity
	Address      string `json:"address"`       // aws_instance.web[0]
	Type         string `json:"type"`          // aws_instance
	Name         string `json:"name"`          // web
	Index        *int   `json:"index"`         // 0 (for count/for_each)
	IndexKey     string `json:"index_key"`     // key for for_each
	
	// Provider
	Provider     string `json:"provider"`      // aws
	ProviderName string `json:"provider_name"` // hashicorp/aws
	
	// Location
	Region       string `json:"region"`        // Resolved from provider or resource
	
	// Configuration
	Mode         string                 `json:"mode"`       // managed, data
	Attributes   map[string]interface{} `json:"attributes"` // All resource attributes
	Sensitive    map[string]bool        `json:"sensitive"`  // Which attributes are sensitive
	
	// Dependencies
	DependsOn    []string `json:"depends_on"`
	Dependencies []string `json:"dependencies"` // Computed from references
}

// ResourceChange represents a planned change to a resource
type ResourceChange struct {
	Address      string                 `json:"address"`
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	Provider     string                 `json:"provider"`
	Action       ChangeAction           `json:"action"`
	Actions      []string               `json:"actions"` // Raw actions from plan
	Before       map[string]interface{} `json:"before"`
	After        map[string]interface{} `json:"after"`
	AfterUnknown map[string]interface{} `json:"after_unknown"`
	
	// Computed
	ChangedAttributes []string `json:"changed_attributes"`
}

// ProviderConfig represents provider configuration
type ProviderConfig struct {
	Name       string                 `json:"name"`
	Alias      string                 `json:"alias"`
	Region     string                 `json:"region"`
	Attributes map[string]interface{} `json:"attributes"`
}

// OutputValue represents a Terraform output
type OutputValue struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive"`
}

// Parser parses Terraform plan JSON output
type Parser struct {
	// Configuration
	ResolveRegions bool // Attempt to resolve regions from provider/resource config
}

// NewParser creates a new Terraform plan parser
func NewParser() *Parser {
	return &Parser{
		ResolveRegions: true,
	}
}

// ParseFile parses a Terraform plan JSON file
func (p *Parser) ParseFile(path string) (*ParsedPlan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plan file: %w", err)
	}
	defer f.Close()
	return p.Parse(f)
}

// Parse parses Terraform plan JSON from a reader
func (p *Parser) Parse(r io.Reader) (*ParsedPlan, error) {
	var rawPlan TerraformPlanJSON
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&rawPlan); err != nil {
		return nil, fmt.Errorf("failed to decode plan JSON: %w", err)
	}
	
	return p.transform(&rawPlan)
}

// ParseBytes parses Terraform plan JSON from bytes
func (p *Parser) ParseBytes(data []byte) (*ParsedPlan, error) {
	var rawPlan TerraformPlanJSON
	if err := json.Unmarshal(data, &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to decode plan JSON: %w", err)
	}
	return p.transform(&rawPlan)
}

// transform converts raw Terraform JSON to our domain model
func (p *Parser) transform(raw *TerraformPlanJSON) (*ParsedPlan, error) {
	plan := &ParsedPlan{
		FormatVersion:    raw.FormatVersion,
		TerraformVersion: raw.TerraformVersion,
		Resources:        make([]ResourceNode, 0),
		Dependencies:     make(map[string][]string),
		Changes:          make([]ResourceChange, 0),
		Providers:        make(map[string]ProviderConfig),
		Variables:        raw.Variables,
		Outputs:          make(map[string]OutputValue),
	}
	
	// Parse provider configurations
	for name, cfg := range raw.Configuration.ProviderConfig {
		plan.Providers[name] = p.parseProviderConfig(name, cfg)
	}
	
	// Parse resource changes
	for _, rc := range raw.ResourceChanges {
		change := p.parseResourceChange(rc)
		plan.Changes = append(plan.Changes, change)
		
		// Build resource node from change
		node := p.buildResourceNode(rc, plan.Providers)
		plan.Resources = append(plan.Resources, node)
		
		// Track dependencies
		if len(node.Dependencies) > 0 {
			plan.Dependencies[node.Address] = node.Dependencies
		}
	}
	
	// Parse outputs
	for name, out := range raw.PlannedValues.Outputs {
		plan.Outputs[name] = OutputValue{
			Value:     out.Value,
			Sensitive: out.Sensitive,
		}
	}
	
	return plan, nil
}

// parseProviderConfig extracts provider configuration
func (p *Parser) parseProviderConfig(name string, cfg RawProviderConfig) ProviderConfig {
	pc := ProviderConfig{
		Name:       name,
		Alias:      cfg.Alias,
		Attributes: make(map[string]interface{}),
	}
	
	// Extract region from expressions if available
	if regionExpr, ok := cfg.Expressions["region"]; ok {
		if cv, ok := regionExpr["constant_value"]; ok {
			if region, ok := cv.(string); ok {
				pc.Region = region
			}
		}
	}
	
	return pc
}

// parseResourceChange converts raw resource change to our model
func (p *Parser) parseResourceChange(rc RawResourceChange) ResourceChange {
	change := ResourceChange{
		Address:  rc.Address,
		Type:     rc.Type,
		Name:     rc.Name,
		Provider: extractProviderFromAddress(rc.ProviderName),
		Actions:  rc.Change.Actions,
		Before:   rc.Change.Before,
		After:    rc.Change.After,
		AfterUnknown: rc.Change.AfterUnknown,
	}
	
	// Determine primary action
	change.Action = p.determineAction(rc.Change.Actions)
	
	// Compute changed attributes
	change.ChangedAttributes = p.computeChangedAttributes(change.Before, change.After)
	
	return change
}

// buildResourceNode creates a ResourceNode from change data
func (p *Parser) buildResourceNode(rc RawResourceChange, providers map[string]ProviderConfig) ResourceNode {
	node := ResourceNode{
		Address:      rc.Address,
		Type:         rc.Type,
		Name:         rc.Name,
		Mode:         rc.Mode,
		Provider:     extractProviderFromAddress(rc.ProviderName),
		ProviderName: rc.ProviderName,
		Attributes:   rc.Change.After, // Use planned state
		Sensitive:    make(map[string]bool),
		Dependencies: make([]string, 0),
	}
	
	// Handle no after state (delete)
	if node.Attributes == nil {
		node.Attributes = rc.Change.Before
	}
	
	// Extract index if present
	if rc.Index != nil {
		switch v := rc.Index.(type) {
		case float64:
			idx := int(v)
			node.Index = &idx
		case string:
			node.IndexKey = v
		}
	}
	
	// Resolve region
	if p.ResolveRegions {
		node.Region = p.resolveRegion(node, providers)
	}
	
	return node
}

// resolveRegion attempts to determine the region for a resource
func (p *Parser) resolveRegion(node ResourceNode, providers map[string]ProviderConfig) string {
	// 1. Check resource-level region attribute
	if region, ok := node.Attributes["region"].(string); ok && region != "" {
		return region
	}
	
	// 2. Check availability_zone and extract region
	if az, ok := node.Attributes["availability_zone"].(string); ok && az != "" {
		// Remove the trailing letter (e.g., us-east-1a -> us-east-1)
		if len(az) > 1 {
			return az[:len(az)-1]
		}
	}
	
	// 3. Check location (Azure)
	if location, ok := node.Attributes["location"].(string); ok && location != "" {
		return location
	}
	
	// 4. Check provider config
	if provider, ok := providers[node.Provider]; ok && provider.Region != "" {
		return provider.Region
	}
	
	// 5. Default based on provider
	switch node.Provider {
	case "aws":
		return "us-east-1" // AWS default
	case "google", "gcp":
		return "us-central1"
	case "azurerm", "azure":
		return "eastus"
	}
	
	return ""
}

// determineAction maps Terraform actions to our ChangeAction
func (p *Parser) determineAction(actions []string) ChangeAction {
	if len(actions) == 0 {
		return ActionNoOp
	}
	
	// Check for specific action combinations
	hasCreate := contains(actions, "create")
	hasDelete := contains(actions, "delete")
	hasUpdate := contains(actions, "update")
	hasRead := contains(actions, "read")
	
	if hasCreate && hasDelete {
		return ActionReplace
	}
	if hasCreate {
		return ActionCreate
	}
	if hasDelete {
		return ActionDelete
	}
	if hasUpdate {
		return ActionUpdate
	}
	if hasRead {
		return ActionRead
	}
	
	return ActionNoOp
}

// computeChangedAttributes identifies which attributes changed
func (p *Parser) computeChangedAttributes(before, after map[string]interface{}) []string {
	changed := make([]string, 0)
	
	// Check all keys in after
	for key, afterVal := range after {
		beforeVal, exists := before[key]
		if !exists {
			changed = append(changed, key)
			continue
		}
		
		// Simple equality check (deep compare would be better)
		if fmt.Sprintf("%v", beforeVal) != fmt.Sprintf("%v", afterVal) {
			changed = append(changed, key)
		}
	}
	
	// Check for deleted keys
	for key := range before {
		if _, exists := after[key]; !exists {
			changed = append(changed, key)
		}
	}
	
	return changed
}

// =============================================================================
// RAW TERRAFORM JSON STRUCTURES
// =============================================================================

// TerraformPlanJSON represents the raw terraform show -json output
type TerraformPlanJSON struct {
	FormatVersion    string                 `json:"format_version"`
	TerraformVersion string                 `json:"terraform_version"`
	Variables        map[string]interface{} `json:"variables"`
	PlannedValues    RawPlannedValues       `json:"planned_values"`
	ResourceChanges  []RawResourceChange    `json:"resource_changes"`
	Configuration    RawConfiguration       `json:"configuration"`
	PriorState       *RawState              `json:"prior_state,omitempty"`
}

type RawPlannedValues struct {
	Outputs    map[string]RawOutput `json:"outputs"`
	RootModule RawModule            `json:"root_module"`
}

type RawOutput struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive"`
}

type RawModule struct {
	Resources    []RawResource `json:"resources"`
	ChildModules []RawModule   `json:"child_modules,omitempty"`
}

type RawResource struct {
	Address       string                 `json:"address"`
	Mode          string                 `json:"mode"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Index         interface{}            `json:"index,omitempty"`
	ProviderName  string                 `json:"provider_name"`
	Values        map[string]interface{} `json:"values"`
	SensitiveValues interface{}          `json:"sensitive_values"`
}

type RawResourceChange struct {
	Address      string      `json:"address"`
	Mode         string      `json:"mode"`
	Type         string      `json:"type"`
	Name         string      `json:"name"`
	Index        interface{} `json:"index,omitempty"`
	ProviderName string      `json:"provider_name"`
	Change       RawChange   `json:"change"`
}

type RawChange struct {
	Actions      []string               `json:"actions"`
	Before       map[string]interface{} `json:"before"`
	After        map[string]interface{} `json:"after"`
	AfterUnknown map[string]interface{} `json:"after_unknown"`
}

type RawConfiguration struct {
	ProviderConfig map[string]RawProviderConfig `json:"provider_config"`
	RootModule     RawConfigModule              `json:"root_module"`
}

type RawProviderConfig struct {
	Name        string                            `json:"name"`
	Alias       string                            `json:"alias,omitempty"`
	Expressions map[string]map[string]interface{} `json:"expressions"`
}

type RawConfigModule struct {
	Resources []RawConfigResource `json:"resources"`
}

type RawConfigResource struct {
	Address           string                            `json:"address"`
	Mode              string                            `json:"mode"`
	Type              string                            `json:"type"`
	Name              string                            `json:"name"`
	ProviderConfigKey string                            `json:"provider_config_key"`
	Expressions       map[string]map[string]interface{} `json:"expressions"`
	DependsOn         []string                          `json:"depends_on,omitempty"`
}

type RawState struct {
	FormatVersion string     `json:"format_version"`
	Values        RawModule  `json:"values"`
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func extractProviderFromAddress(providerName string) string {
	// registry.terraform.io/hashicorp/aws -> aws
	parts := strings.Split(providerName, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return providerName
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
