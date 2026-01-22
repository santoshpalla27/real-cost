// Package aws provides stub mappers for AWS resources
// These will be fully implemented as the platform develops
package aws

import (
	"fmt"

	"terraform-cost/decision/billing"
	"terraform-cost/decision/iac"
)

// =============================================================================
// EBS Volume Mapper
// =============================================================================

type EBSVolumeMapper struct{}

func NewEBSVolumeMapper() *EBSVolumeMapper { return &EBSVolumeMapper{} }

func (m *EBSVolumeMapper) ResourceType() string { return "aws_ebs_volume" }

func (m *EBSVolumeMapper) SupportedAttributes() []string {
	return []string{"size", "type", "iops", "throughput"}
}

func (m *EBSVolumeMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	
	volumeType := billing.ExtractAttribute(attrs, "type")
	if volumeType == "" {
		volumeType = "gp3"
	}
	
	volumeSize := billing.ExtractAttributeFloat(attrs, "size", 8)
	
	return []billing.BillingComponent{{
		ID:            fmt.Sprintf("%s-storage", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonEC2",
		ProductFamily: "Storage",
		Region:        node.Region,
		UsageType:     fmt.Sprintf("EBS:VolumeUsage.%s", volumeType),
		BillingPeriod: billing.PeriodMonthly,
		Attributes: map[string]string{
			"volumeType": volumeType,
		},
		Description:     fmt.Sprintf("EBS %s volume (%.0f GB)", volumeType, volumeSize),
		Tags:            []string{"storage", "ebs"},
		VarianceProfile: billing.VarianceProfile{BaselineUsage: volumeSize, P50Usage: volumeSize, Confidence: 0.99},
	}}, nil
}

// =============================================================================
// Lambda Function Mapper
// =============================================================================

type LambdaFunctionMapper struct{}

func NewLambdaFunctionMapper() *LambdaFunctionMapper { return &LambdaFunctionMapper{} }

func (m *LambdaFunctionMapper) ResourceType() string { return "aws_lambda_function" }

func (m *LambdaFunctionMapper) SupportedAttributes() []string {
	return []string{"memory_size", "timeout", "architectures"}
}

func (m *LambdaFunctionMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	
	memorySize := billing.ExtractAttributeInt(attrs, "memory_size", 128)
	
	return []billing.BillingComponent{{
		ID:            fmt.Sprintf("%s-invocations", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AWSLambda",
		ProductFamily: "Serverless",
		Region:        node.Region,
		UsageType:     "Lambda-GB-Second",
		BillingPeriod: billing.PeriodPerRequest,
		Attributes: map[string]string{
			"memorySize": fmt.Sprintf("%d", memorySize),
		},
		Description: fmt.Sprintf("Lambda function (%d MB)", memorySize),
		Tags:        []string{"serverless", "lambda"},
		VarianceProfile: billing.VarianceProfile{
			BaselineUsage: 1000000, // 1M requests/month estimate
			P50Usage:      500000,
			P90Usage:      2000000,
			Confidence:    0.5, // High uncertainty for serverless
			Assumptions:   []string{"Usage highly variable, estimate based on environment"},
		},
	}}, nil
}

// =============================================================================
// RDS Instance Mapper
// =============================================================================

type RDSInstanceMapper struct{}

func NewRDSInstanceMapper() *RDSInstanceMapper { return &RDSInstanceMapper{} }

func (m *RDSInstanceMapper) ResourceType() string { return "aws_db_instance" }

func (m *RDSInstanceMapper) SupportedAttributes() []string {
	return []string{"instance_class", "engine", "allocated_storage", "multi_az"}
}

func (m *RDSInstanceMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	components := make([]billing.BillingComponent, 0)
	
	instanceClass := billing.ExtractAttribute(attrs, "instance_class")
	engine := billing.ExtractAttribute(attrs, "engine")
	storage := billing.ExtractAttributeFloat(attrs, "allocated_storage", 20)
	multiAZ := billing.ExtractAttributeBool(attrs, "multi_az", false)
	
	deploymentOption := "Single-AZ"
	if multiAZ {
		deploymentOption = "Multi-AZ"
	}
	
	// Compute component
	components = append(components, billing.BillingComponent{
		ID:            fmt.Sprintf("%s-compute", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonRDS",
		ProductFamily: "Database Instance",
		Region:        node.Region,
		UsageType:     fmt.Sprintf("RDS:%s", instanceClass),
		BillingPeriod: billing.PeriodHourly,
		Attributes: map[string]string{
			"instanceType":     instanceClass,
			"databaseEngine":   engine,
			"deploymentOption": deploymentOption,
		},
		Description:     fmt.Sprintf("RDS %s (%s, %s)", instanceClass, engine, deploymentOption),
		Tags:            []string{"database", "rds"},
		VarianceProfile: billing.NewDefaultVarianceProfile(730),
	})
	
	// Storage component
	components = append(components, billing.BillingComponent{
		ID:            fmt.Sprintf("%s-storage", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonRDS",
		ProductFamily: "Database Storage",
		Region:        node.Region,
		UsageType:     "RDS:GP3-Storage",
		BillingPeriod: billing.PeriodMonthly,
		Attributes: map[string]string{
			"deploymentOption": deploymentOption,
		},
		Description:     fmt.Sprintf("RDS storage (%.0f GB)", storage),
		Tags:            []string{"database", "storage"},
		VarianceProfile: billing.VarianceProfile{BaselineUsage: storage, P50Usage: storage, Confidence: 0.95},
	})
	
	return components, nil
}

// =============================================================================
// DynamoDB Table Mapper
// =============================================================================

type DynamoDBTableMapper struct{}

func NewDynamoDBTableMapper() *DynamoDBTableMapper { return &DynamoDBTableMapper{} }

func (m *DynamoDBTableMapper) ResourceType() string { return "aws_dynamodb_table" }

func (m *DynamoDBTableMapper) SupportedAttributes() []string {
	return []string{"billing_mode", "read_capacity", "write_capacity"}
}

func (m *DynamoDBTableMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	
	billingMode := billing.ExtractAttribute(attrs, "billing_mode")
	if billingMode == "" {
		billingMode = "PROVISIONED"
	}
	
	if billingMode == "PAY_PER_REQUEST" {
		return []billing.BillingComponent{{
			ID:            fmt.Sprintf("%s-ondemand", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonDynamoDB",
			ProductFamily: "Database",
			Region:        node.Region,
			UsageType:     "PayPerRequest",
			BillingPeriod: billing.PeriodPerRequest,
			Attributes:    map[string]string{"billingMode": "on-demand"},
			Description:   "DynamoDB on-demand capacity",
			Tags:          []string{"database", "dynamodb"},
			VarianceProfile: billing.VarianceProfile{
				BaselineUsage: 1000000,
				Confidence:    0.5,
				Assumptions:   []string{"On-demand usage highly variable"},
			},
		}}, nil
	}
	
	rcu := billing.ExtractAttributeFloat(attrs, "read_capacity", 5)
	wcu := billing.ExtractAttributeFloat(attrs, "write_capacity", 5)
	
	return []billing.BillingComponent{
		{
			ID:            fmt.Sprintf("%s-rcu", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonDynamoDB",
			ProductFamily: "Database",
			Region:        node.Region,
			UsageType:     "ReadCapacityUnit-Hrs",
			BillingPeriod: billing.PeriodHourly,
			Attributes:    map[string]string{},
			Description:   fmt.Sprintf("DynamoDB %.0f RCU", rcu),
			Tags:          []string{"database", "dynamodb"},
			VarianceProfile: billing.VarianceProfile{BaselineUsage: rcu * 730, P50Usage: rcu * 730, Confidence: 0.9},
		},
		{
			ID:            fmt.Sprintf("%s-wcu", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonDynamoDB",
			ProductFamily: "Database",
			Region:        node.Region,
			UsageType:     "WriteCapacityUnit-Hrs",
			BillingPeriod: billing.PeriodHourly,
			Attributes:    map[string]string{},
			Description:   fmt.Sprintf("DynamoDB %.0f WCU", wcu),
			Tags:          []string{"database", "dynamodb"},
			VarianceProfile: billing.VarianceProfile{BaselineUsage: wcu * 730, P50Usage: wcu * 730, Confidence: 0.9},
		},
	}, nil
}

// =============================================================================
// S3 Bucket Mapper
// =============================================================================

type S3BucketMapper struct{}

func NewS3BucketMapper() *S3BucketMapper { return &S3BucketMapper{} }

func (m *S3BucketMapper) ResourceType() string { return "aws_s3_bucket" }

func (m *S3BucketMapper) SupportedAttributes() []string {
	return []string{}
}

func (m *S3BucketMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	return []billing.BillingComponent{{
		ID:            fmt.Sprintf("%s-storage", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonS3",
		ProductFamily: "Storage",
		Region:        node.Region,
		UsageType:     "TimedStorage-ByteHrs",
		BillingPeriod: billing.PeriodMonthly,
		Attributes: map[string]string{
			"storageClass": "STANDARD",
		},
		Description: "S3 Standard storage",
		Tags:        []string{"storage", "s3"},
		VarianceProfile: billing.VarianceProfile{
			BaselineUsage: 100, // 100 GB estimate
			P50Usage:      50,
			P90Usage:      500,
			Confidence:    0.3,
			Assumptions:   []string{"S3 usage highly variable, using environment-based estimate"},
		},
	}}, nil
}

// =============================================================================
// NAT Gateway Mapper
// =============================================================================

type NATGatewayMapper struct{}

func NewNATGatewayMapper() *NATGatewayMapper { return &NATGatewayMapper{} }

func (m *NATGatewayMapper) ResourceType() string { return "aws_nat_gateway" }

func (m *NATGatewayMapper) SupportedAttributes() []string {
	return []string{}
}

func (m *NATGatewayMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	return []billing.BillingComponent{
		{
			ID:            fmt.Sprintf("%s-hours", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonVPC",
			ProductFamily: "NAT Gateway",
			Region:        node.Region,
			UsageType:     "NatGateway-Hours",
			BillingPeriod: billing.PeriodHourly,
			Attributes:    map[string]string{},
			Description:   "NAT Gateway hours",
			Tags:          []string{"networking", "nat"},
			VarianceProfile: billing.NewDefaultVarianceProfile(730),
		},
		{
			ID:            fmt.Sprintf("%s-data", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonVPC",
			ProductFamily: "NAT Gateway",
			Region:        node.Region,
			UsageType:     "NatGateway-Bytes",
			BillingPeriod: billing.PeriodPerGB,
			Attributes:    map[string]string{},
			Description:   "NAT Gateway data processing",
			Tags:          []string{"networking", "data-transfer"},
			VarianceProfile: billing.VarianceProfile{
				BaselineUsage: 100, // 100 GB/month estimate
				P50Usage:      50,
				P90Usage:      500,
				Confidence:    0.5,
			},
		},
	}, nil
}

// =============================================================================
// Load Balancer Mapper
// =============================================================================

type LBMapper struct{}

func NewLBMapper() *LBMapper { return &LBMapper{} }

func (m *LBMapper) ResourceType() string { return "aws_lb" }

func (m *LBMapper) SupportedAttributes() []string {
	return []string{"load_balancer_type"}
}

func (m *LBMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	lbType := billing.ExtractAttribute(attrs, "load_balancer_type")
	if lbType == "" {
		lbType = "application"
	}
	
	var productFamily, usageType, service string
	switch lbType {
	case "network":
		service = "ElasticLoadBalancing"
		productFamily = "Load Balancer-Network"
		usageType = "LoadBalancerUsage"
	case "gateway":
		service = "ElasticLoadBalancing"
		productFamily = "Load Balancer-Gateway"
		usageType = "LoadBalancerUsage"
	default: // application
		service = "ElasticLoadBalancing"
		productFamily = "Load Balancer-Application"
		usageType = "LoadBalancerUsage"
	}
	
	return []billing.BillingComponent{
		{
			ID:            fmt.Sprintf("%s-hours", node.Resource.Address),
			Cloud:         "aws",
			Service:       service,
			ProductFamily: productFamily,
			Region:        node.Region,
			UsageType:     usageType,
			BillingPeriod: billing.PeriodHourly,
			Attributes: map[string]string{
				"loadBalancerType": lbType,
			},
			Description:     fmt.Sprintf("%s Load Balancer hours", lbType),
			Tags:            []string{"networking", "loadbalancer"},
			VarianceProfile: billing.NewDefaultVarianceProfile(730),
		},
	}, nil
}

// =============================================================================
// Elastic IP Mapper
// =============================================================================

type EIPMapper struct{}

func NewEIPMapper() *EIPMapper { return &EIPMapper{} }

func (m *EIPMapper) ResourceType() string { return "aws_eip" }

func (m *EIPMapper) SupportedAttributes() []string {
	return []string{"instance", "network_interface"}
}

func (m *EIPMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	attrs := node.Resource.Attributes
	
	// EIP is free when attached, charged when unattached
	isAttached := billing.ExtractAttribute(attrs, "instance") != "" ||
		billing.ExtractAttribute(attrs, "network_interface") != ""
	
	if isAttached {
		// No charge for attached EIP
		return []billing.BillingComponent{}, nil
	}
	
	return []billing.BillingComponent{{
		ID:            fmt.Sprintf("%s-idle", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonEC2",
		ProductFamily: "IP Address",
		Region:        node.Region,
		UsageType:     "ElasticIP:IdleAddress",
		BillingPeriod: billing.PeriodHourly,
		Attributes:    map[string]string{},
		Description:   "Idle Elastic IP address",
		Tags:          []string{"networking", "eip"},
		VarianceProfile: billing.NewDefaultVarianceProfile(730),
	}}, nil
}
