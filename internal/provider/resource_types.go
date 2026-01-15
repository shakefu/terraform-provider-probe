// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import "strings"

// tfToCloudControl maps Terraform resource type names to AWS Cloud Control type names.
var tfToCloudControl = map[string]string{
	// Compute
	"aws_lambda_function": "AWS::Lambda::Function",
	"aws_ecs_cluster":     "AWS::ECS::Cluster",
	"aws_ecs_service":     "AWS::ECS::Service",
	"aws_ecs_task":        "AWS::ECS::TaskDefinition",

	// Storage
	"aws_dynamodb_table": "AWS::DynamoDB::Table",
	"aws_s3_bucket":      "AWS::S3::Bucket",

	// Messaging
	"aws_sqs_queue": "AWS::SQS::Queue",
	"aws_sns_topic": "AWS::SNS::Topic",

	// IAM
	"aws_iam_role":   "AWS::IAM::Role",
	"aws_iam_policy": "AWS::IAM::ManagedPolicy",
	"aws_iam_user":   "AWS::IAM::User",
	"aws_iam_group":  "AWS::IAM::Group",

	// API Gateway
	"aws_apigatewayv2_api":     "AWS::ApiGatewayV2::Api",
	"aws_api_gateway_rest_api": "AWS::ApiGateway::RestApi",

	// Networking
	"aws_vpc":              "AWS::EC2::VPC",
	"aws_subnet":           "AWS::EC2::Subnet",
	"aws_security_group":   "AWS::EC2::SecurityGroup",
	"aws_internet_gateway": "AWS::EC2::InternetGateway",

	// Secrets & Parameters
	"aws_secretsmanager_secret": "AWS::SecretsManager::Secret",
	"aws_ssm_parameter":         "AWS::SSM::Parameter",

	// CloudWatch
	"aws_cloudwatch_log_group":    "AWS::Logs::LogGroup",
	"aws_cloudwatch_metric_alarm": "AWS::CloudWatch::Alarm",

	// Step Functions
	"aws_sfn_state_machine": "AWS::StepFunctions::StateMachine",

	// EventBridge
	"aws_cloudwatch_event_rule": "AWS::Events::Rule",
	"aws_cloudwatch_event_bus":  "AWS::Events::EventBus",

	// Kinesis
	"aws_kinesis_stream": "AWS::Kinesis::Stream",

	// RDS
	"aws_db_instance": "AWS::RDS::DBInstance",
	"aws_db_cluster":  "AWS::RDS::DBCluster",

	// ElastiCache
	"aws_elasticache_cluster": "AWS::ElastiCache::CacheCluster",
}

// resolveCloudControlType converts a Terraform resource type to an AWS Cloud Control type.
// It accepts either Terraform-style names (aws_dynamodb_table) or Cloud Control names (AWS::DynamoDB::Table).
func resolveCloudControlType(tfType string) string {
	if mapped, ok := tfToCloudControl[tfType]; ok {
		return mapped
	}
	// Pass through if already Cloud Control format
	if strings.HasPrefix(tfType, "AWS::") {
		return tfType
	}
	// Unknown type - let Cloud Control API return the error
	return tfType
}
