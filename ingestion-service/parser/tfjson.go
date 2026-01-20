package parser

import "encoding/json"

// TFPlan represents the top-level structure of `terraform show -json`
type TFPlan struct {
	FormatVersion    string           `json:"format_version"`
	TerraformVersion string           `json:"terraform_version"`
	ResourceChanges  []ResourceChange `json:"resource_changes"`
	Config           *PlanConfig      `json:"configuration"`
}

func (p TFPlan) FollowsConfig() bool {
	return p.Config != nil && p.Config.RootModule != nil
}

type PlanConfig struct {
	RootModule *ConfModule `json:"root_module"`
}

type ConfModule struct {
	Resources []ConfResource `json:"resources"`
}

type ConfResource struct {
	Address   string   `json:"address"`
	Name      string   `json:"name"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type ResourceChange struct {
	Address      string `json:"address"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	ProviderName string `json:"provider_name"`
	Change       Change `json:"change"`
}

type Change struct {
	Actions []string    `json:"actions"`
	After   interface{} `json:"after"`
}
