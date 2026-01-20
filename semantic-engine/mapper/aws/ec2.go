package aws

import (
	"fmt"
	
	"github.com/futuristic-iac/semantic-engine/mapper"
	"github.com/futuristic-iac/pkg/api"
	"github.com/futuristic-iac/pkg/graph"
)

func init() {
	mapper.Register("aws_instance", &EC2Mapper{})
}

type EC2Mapper struct{}

func (m *EC2Mapper) Map(res graph.Resource) ([]api.BillingComponent, error) {
	components := []api.BillingComponent{}

	// 1. Compute Component
	instanceType, ok := res.Attributes["instance_type"].(string)
	mappingErr := ""
	if !ok || instanceType == "" {
		// FAIL CLOSED: Do not guess "t2.micro". 
		// Return a component but flag it as erroneous so the policy engine sees it.
		instanceType = "UNKNOWN"
		mappingErr = fmt.Sprintf("Missing required attribute 'instance_type' for %s", res.Address)
	}
	
	lifecycle := "persistent" // default
	// check if spot
	if _, isSpot := res.Attributes["spot_price"]; isSpot {
		lifecycle = "ephemeral"
	}

	compute := api.BillingComponent{
		ResourceAddress: res.Address,
		ComponentType:   "compute",
		Provider:        "aws",
		UsageType:       "on_demand", // default, policy can change to reserved
		Lifecycle:       lifecycle,
		VarianceProfile: "static", // usually always on unless autoscaled
		Dependencies:    res.Dependencies,
		LookupAttributes: map[string]string{
			"instance_type": instanceType,
			"family":        "Compute Instance", // placeholder logic
			"usage_type":    "BoxUsage:" + instanceType, // simplified match key
		},
		MappingError: mappingErr,
	}
	
	components = append(components, compute)

	// 2. Storage (EBS)
	// aws_instance config blocks for ebs are tricky in JSON. 
	// They come as a list of maps in "ebs_block_device" or "root_block_device".
	
	if root, ok := res.Attributes["root_block_device"].([]interface{}); ok && len(root) > 0 {
		r := root[0].(map[string]interface{})
		volSize := 8.0 // default
		if s, ok := r["volume_size"].(float64); ok {
			volSize = s
		}
		
		storage := api.BillingComponent{
			ResourceAddress: res.Address + ".root_block_device",
			ComponentType:   "storage",
			Provider:        "aws",
			UsageType:       "on_demand",
			Lifecycle:       lifecycle,
			VarianceProfile: "static",
			LookupAttributes: map[string]string{
				"volume_type": "gp2", // default
				"size_gb":     fmt.Sprintf("%f", volSize),
			},
		}
		components = append(components, storage)
	}

	return components, nil
}
