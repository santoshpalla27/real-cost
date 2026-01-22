// Package carbon provides carbon footprint modeling.
package carbon

// RegionIntensity returns carbon intensity for a cloud region.
// Values are in gCO2e per kWh.
func RegionIntensity(provider, region string) float64 {
	// AWS region carbon intensities (approximate, 2024 data)
	awsIntensities := map[string]float64{
		"us-east-1":      383.0,  // Virginia
		"us-east-2":      425.0,  // Ohio
		"us-west-1":      233.0,  // N. California
		"us-west-2":      78.0,   // Oregon (hydro)
		"eu-west-1":      316.0,  // Ireland
		"eu-west-2":      228.0,  // London
		"eu-west-3":      51.0,   // Paris (nuclear)
		"eu-central-1":   338.0,  // Frankfurt
		"eu-north-1":     8.0,    // Stockholm (hydro)
		"ap-northeast-1": 471.0,  // Tokyo
		"ap-southeast-1": 408.0,  // Singapore
		"ap-southeast-2": 656.0,  // Sydney
		"ap-south-1":     708.0,  // Mumbai
		"sa-east-1":      74.0,   // São Paulo
		"ca-central-1":   120.0,  // Montreal
	}

	if provider == "aws" || provider == "" {
		if intensity, ok := awsIntensities[region]; ok {
			return intensity
		}
	}

	// Default to US average
	return 400.0
}

// ComputeCarbon calculates carbon footprint for compute resources.
type ComputeCarbon struct {
	Region      string
	InstanceType string
	Hours       float64
}

// EstimateKgCO2e returns estimated carbon in kg CO2e.
func (c *ComputeCarbon) EstimateKgCO2e() float64 {
	// Power consumption estimates by instance size (Watts)
	powerWatts := instancePowerConsumption(c.InstanceType)
	
	// Convert to kWh
	kWh := (powerWatts * c.Hours) / 1000.0
	
	// Apply regional carbon intensity
	intensity := RegionIntensity("aws", c.Region)
	gCO2e := kWh * intensity
	
	// Convert to kg
	return gCO2e / 1000.0
}

func instancePowerConsumption(instanceType string) float64 {
	// Approximate power consumption by instance family
	powerByFamily := map[string]float64{
		"t3":  10.0,   // Burstable
		"t2":  8.0,
		"m5":  35.0,   // General purpose
		"m6i": 40.0,
		"c5":  45.0,   // Compute optimized
		"c6i": 50.0,
		"r5":  55.0,   // Memory optimized
		"r6i": 60.0,
		"i3":  80.0,   // Storage optimized
		"p3":  300.0,  // GPU
		"p4":  400.0,
	}

	// Extract family from instance type (e.g., "t3.medium" -> "t3")
	for family, power := range powerByFamily {
		if len(instanceType) >= len(family) && instanceType[:len(family)] == family {
			// Adjust by size
			return power * sizeMultiplier(instanceType)
		}
	}

	return 20.0 // Default
}

func sizeMultiplier(instanceType string) float64 {
	sizes := map[string]float64{
		"nano":    0.25,
		"micro":   0.5,
		"small":   0.75,
		"medium":  1.0,
		"large":   2.0,
		"xlarge":  4.0,
		"2xlarge": 8.0,
		"4xlarge": 16.0,
		"8xlarge": 32.0,
	}

	for size, mult := range sizes {
		if len(instanceType) > len(size) && instanceType[len(instanceType)-len(size):] == size {
			return mult
		}
	}

	return 1.0
}

// StorageCarbon calculates carbon for storage.
type StorageCarbon struct {
	Region     string
	GBMonths   float64
	VolumeType string
}

// EstimateKgCO2e returns estimated carbon for storage.
func (s *StorageCarbon) EstimateKgCO2e() float64 {
	// Storage has relatively low direct carbon
	// Approximately 0.0001 kWh per GB-month
	kWh := s.GBMonths * 0.0001
	intensity := RegionIntensity("aws", s.Region)
	gCO2e := kWh * intensity
	return gCO2e / 1000.0
}
