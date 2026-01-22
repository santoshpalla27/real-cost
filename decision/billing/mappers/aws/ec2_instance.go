// Package aws provides AWS resource mappers for the Billing Semantic Engine
package aws

import (
	"fmt"
	"strings"

	"terraform-cost/decision/billing"
	"terraform-cost/decision/iac"
)

// EC2InstanceMapper maps aws_instance to billing components
type EC2InstanceMapper struct{}

// NewEC2InstanceMapper creates a new EC2 instance mapper
func NewEC2InstanceMapper() *EC2InstanceMapper {
	return &EC2InstanceMapper{}
}

// ResourceType returns the Terraform resource type
func (m *EC2InstanceMapper) ResourceType() string {
	return "aws_instance"
}

// SupportedAttributes returns attributes this mapper uses
func (m *EC2InstanceMapper) SupportedAttributes() []string {
	return []string{
		"instance_type",
		"ami",
		"availability_zone",
		"tenancy",
		"ebs_optimized",
		"monitoring",
		"root_block_device",
		"ebs_block_device",
		"credit_specification",
	}
}

// MapToBillingComponents converts an EC2 instance to billing components
func (m *EC2InstanceMapper) MapToBillingComponents(node *iac.GraphNode) ([]billing.BillingComponent, []billing.MappingError) {
	components := make([]billing.BillingComponent, 0)
	errors := make([]billing.MappingError, 0)
	
	attrs := node.Resource.Attributes
	
	// Extract key attributes
	instanceType := billing.ExtractAttribute(attrs, "instance_type")
	if instanceType == "" {
		errors = append(errors, billing.MappingError{
			ResourceAddr: node.Resource.Address,
			ResourceType: "aws_instance",
			Reason:       "instance_type attribute is required",
			IsCritical:   true,
		})
		return components, errors
	}
	
	// Determine OS from AMI (simplified - would need AMI lookup in production)
	operatingSystem := m.inferOperatingSystem(attrs)
	
	// Tenancy
	tenancy := billing.ExtractAttribute(attrs, "tenancy")
	if tenancy == "" {
		tenancy = "Shared"
	}
	
	// Pre-installed software (simplified)
	preInstalledSw := "NA"
	
	// Capacity status
	capacityStatus := "Used"
	
	// ==========================================================================
	// Component 1: EC2 Compute Hours
	// ==========================================================================
	computeComponent := billing.BillingComponent{
		ID:            fmt.Sprintf("%s-compute", node.Resource.Address),
		Cloud:         "aws",
		Service:       "AmazonEC2",
		ProductFamily: "Compute Instance",
		Region:        node.Region,
		UsageType:     fmt.Sprintf("BoxUsage:%s", instanceType),
		BillingPeriod: billing.PeriodHourly,
		Attributes: map[string]string{
			"instanceType":       instanceType,
			"operatingSystem":    operatingSystem,
			"tenancy":            normalizeTenancy(tenancy),
			"preInstalledSw":     preInstalledSw,
			"capacityStatus":     capacityStatus,
			"licenseModel":       "No License required",
		},
		Description: fmt.Sprintf("EC2 %s (%s) compute hours", instanceType, operatingSystem),
		Tags:        []string{"compute", "ec2"},
		VarianceProfile: billing.NewDefaultVarianceProfile(730), // 730 hours/month
	}
	components = append(components, computeComponent)
	
	// ==========================================================================
	// Component 2: Root Block Device (EBS)
	// ==========================================================================
	if rootDevice := m.extractRootBlockDevice(attrs); rootDevice != nil {
		ebsComponent := m.createEBSComponent(node, rootDevice, "root", 0)
		components = append(components, ebsComponent)
	}
	
	// ==========================================================================
	// Component 3: Additional EBS Volumes
	// ==========================================================================
	ebsDevices := m.extractEBSBlockDevices(attrs)
	for i, device := range ebsDevices {
		ebsComponent := m.createEBSComponent(node, device, "ebs", i)
		components = append(components, ebsComponent)
	}
	
	// ==========================================================================
	// Component 4: EBS-Optimized (if enabled)
	// ==========================================================================
	if billing.ExtractAttributeBool(attrs, "ebs_optimized", false) {
		ebsOptComponent := billing.BillingComponent{
			ID:            fmt.Sprintf("%s-ebs-optimized", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonEC2",
			ProductFamily: "Compute Instance",
			Region:        node.Region,
			UsageType:     fmt.Sprintf("EBSOptimized:%s", instanceType),
			BillingPeriod: billing.PeriodHourly,
			Attributes: map[string]string{
				"instanceType": instanceType,
			},
			Description:     fmt.Sprintf("EBS-optimized usage for %s", instanceType),
			Tags:            []string{"compute", "ebs-optimized"},
			VarianceProfile: billing.NewDefaultVarianceProfile(730),
		}
		components = append(components, ebsOptComponent)
	}
	
	// ==========================================================================
	// Component 5: Detailed Monitoring (if enabled)
	// ==========================================================================
	if billing.ExtractAttributeBool(attrs, "monitoring", false) {
		monitoringComponent := billing.BillingComponent{
			ID:            fmt.Sprintf("%s-monitoring", node.Resource.Address),
			Cloud:         "aws",
			Service:       "AmazonCloudWatch",
			ProductFamily: "Metric",
			Region:        node.Region,
			UsageType:     "MetricMonitorUsage",
			BillingPeriod: billing.PeriodMonthly,
			Attributes:    map[string]string{},
			Description:   "EC2 detailed monitoring (7 metrics)",
			Tags:          []string{"monitoring", "cloudwatch"},
			VarianceProfile: billing.VarianceProfile{
				BaselineUsage: 7, // 7 detailed metrics per instance
				P50Usage:      7,
				P90Usage:      7,
				Confidence:    0.95,
			},
		}
		components = append(components, monitoringComponent)
	}
	
	return components, errors
}

// extractRootBlockDevice extracts root block device configuration
func (m *EC2InstanceMapper) extractRootBlockDevice(attrs map[string]interface{}) map[string]interface{} {
	if rootBlock, ok := attrs["root_block_device"]; ok {
		if arr, ok := rootBlock.([]interface{}); ok && len(arr) > 0 {
			if device, ok := arr[0].(map[string]interface{}); ok {
				return device
			}
		}
	}
	
	// Default root volume if not specified
	return map[string]interface{}{
		"volume_type": "gp3",
		"volume_size": float64(8), // Default 8 GB
	}
}

// extractEBSBlockDevices extracts additional EBS volumes
func (m *EC2InstanceMapper) extractEBSBlockDevices(attrs map[string]interface{}) []map[string]interface{} {
	devices := make([]map[string]interface{}, 0)
	
	if ebsBlock, ok := attrs["ebs_block_device"]; ok {
		if arr, ok := ebsBlock.([]interface{}); ok {
			for _, item := range arr {
				if device, ok := item.(map[string]interface{}); ok {
					devices = append(devices, device)
				}
			}
		}
	}
	
	return devices
}

// createEBSComponent creates an EBS billing component
func (m *EC2InstanceMapper) createEBSComponent(node *iac.GraphNode, device map[string]interface{}, prefix string, index int) billing.BillingComponent {
	volumeType := "gp3"
	if vt, ok := device["volume_type"].(string); ok && vt != "" {
		volumeType = vt
	}
	
	volumeSize := 8.0
	if vs, ok := device["volume_size"].(float64); ok {
		volumeSize = vs
	}
	
	iops := 0
	if i, ok := device["iops"].(float64); ok {
		iops = int(i)
	}
	
	throughput := 0
	if t, ok := device["throughput"].(float64); ok {
		throughput = int(t)
	}
	
	id := fmt.Sprintf("%s-%s-volume", node.Resource.Address, prefix)
	if index > 0 {
		id = fmt.Sprintf("%s-%s-volume-%d", node.Resource.Address, prefix, index)
	}
	
	component := billing.BillingComponent{
		ID:            id,
		Cloud:         "aws",
		Service:       "AmazonEC2",
		ProductFamily: "Storage",
		Region:        node.Region,
		UsageType:     fmt.Sprintf("EBS:VolumeUsage.%s", volumeType),
		BillingPeriod: billing.PeriodMonthly,
		Attributes: map[string]string{
			"volumeType": normalizeVolumeType(volumeType),
		},
		Description: fmt.Sprintf("EBS %s volume (%.0f GB)", volumeType, volumeSize),
		Tags:        []string{"storage", "ebs"},
		VarianceProfile: billing.VarianceProfile{
			BaselineUsage: volumeSize,
			MinUsage:      volumeSize,
			MaxUsage:      volumeSize,
			P50Usage:      volumeSize,
			P90Usage:      volumeSize,
			Confidence:    0.99, // EBS size is deterministic
			Assumptions:   []string{"Volume size is fixed as provisioned"},
		},
	}
	
	// Add IOPS component for provisioned IOPS volumes
	if iops > 0 && (volumeType == "io1" || volumeType == "io2" || volumeType == "gp3") {
		// IOPS would be a separate component in production
		component.Attributes["iops"] = fmt.Sprintf("%d", iops)
	}
	
	// Add throughput for gp3
	if throughput > 0 && volumeType == "gp3" {
		component.Attributes["throughput"] = fmt.Sprintf("%d", throughput)
	}
	
	return component
}

// inferOperatingSystem attempts to determine OS from AMI or other attributes
func (m *EC2InstanceMapper) inferOperatingSystem(attrs map[string]interface{}) string {
	// In production, this would:
	// 1. Look up AMI in a database
	// 2. Use tags/naming conventions
	// 3. Check platform attribute
	
	// Check platform attribute (Windows instances)
	if platform, ok := attrs["platform"].(string); ok {
		if strings.EqualFold(platform, "windows") {
			return "Windows"
		}
	}
	
	// Check AMI name/description if available (would need API lookup)
	ami := billing.ExtractAttribute(attrs, "ami")
	amiLower := strings.ToLower(ami)
	
	// Simple heuristics based on common AMI patterns
	switch {
	case strings.Contains(amiLower, "windows"):
		return "Windows"
	case strings.Contains(amiLower, "rhel"):
		return "RHEL"
	case strings.Contains(amiLower, "suse"):
		return "SUSE"
	default:
		return "Linux" // Default to Linux
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func normalizeTenancy(tenancy string) string {
	switch strings.ToLower(tenancy) {
	case "dedicated":
		return "Dedicated"
	case "host":
		return "Host"
	default:
		return "Shared"
	}
}

func normalizeVolumeType(volumeType string) string {
	switch strings.ToLower(volumeType) {
	case "gp2":
		return "General Purpose"
	case "gp3":
		return "General Purpose"
	case "io1":
		return "Provisioned IOPS"
	case "io2":
		return "Provisioned IOPS"
	case "st1":
		return "Throughput Optimized HDD"
	case "sc1":
		return "Cold HDD"
	case "standard":
		return "Magnetic"
	default:
		return volumeType
	}
}
