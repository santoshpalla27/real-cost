// Package aws provides AWS resource mappers registration
package aws

import "terraform-cost/decision/billing"

// RegisterAllMappers registers all AWS resource mappers with the engine
func RegisterAllMappers(engine *billing.Engine) {
	// Compute
	engine.RegisterMapper(NewEC2InstanceMapper())
	engine.RegisterMapper(NewEBSVolumeMapper())
	engine.RegisterMapper(NewLambdaFunctionMapper())
	
	// Database
	engine.RegisterMapper(NewRDSInstanceMapper())
	engine.RegisterMapper(NewDynamoDBTableMapper())
	
	// Storage
	engine.RegisterMapper(NewS3BucketMapper())
	
	// Networking
	engine.RegisterMapper(NewNATGatewayMapper())
	engine.RegisterMapper(NewLBMapper())
	engine.RegisterMapper(NewEIPMapper())
	
	// TODO: Add more mappers as needed
}

// SupportedResourceTypes returns all AWS resource types with mappers
func SupportedResourceTypes() []string {
	return []string{
		"aws_instance",
		"aws_ebs_volume",
		"aws_lambda_function",
		"aws_db_instance",
		"aws_dynamodb_table",
		"aws_s3_bucket",
		"aws_nat_gateway",
		"aws_lb",
		"aws_alb",
		"aws_elb",
		"aws_eip",
	}
}
