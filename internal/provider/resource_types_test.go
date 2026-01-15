// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestResolveCloudControlType_TerraformTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"DynamoDB table", "aws_dynamodb_table", "AWS::DynamoDB::Table"},
		{"S3 bucket", "aws_s3_bucket", "AWS::S3::Bucket"},
		{"Lambda function", "aws_lambda_function", "AWS::Lambda::Function"},
		{"IAM role", "aws_iam_role", "AWS::IAM::Role"},
		{"SQS queue", "aws_sqs_queue", "AWS::SQS::Queue"},
		{"SNS topic", "aws_sns_topic", "AWS::SNS::Topic"},
		{"VPC", "aws_vpc", "AWS::EC2::VPC"},
		{"Security group", "aws_security_group", "AWS::EC2::SecurityGroup"},
		{"CloudWatch log group", "aws_cloudwatch_log_group", "AWS::Logs::LogGroup"},
		{"Secrets Manager secret", "aws_secretsmanager_secret", "AWS::SecretsManager::Secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCloudControlType(tt.input)
			if result != tt.expected {
				t.Errorf("resolveCloudControlType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveCloudControlType_CloudControlPassthrough(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"DynamoDB Table", "AWS::DynamoDB::Table"},
		{"S3 Bucket", "AWS::S3::Bucket"},
		{"Lambda Function", "AWS::Lambda::Function"},
		{"Custom resource type", "AWS::MyCompany::CustomResource"},
		{"Third-party type", "AWS::SomeVendor::SomeResource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCloudControlType(tt.input)
			if result != tt.input {
				t.Errorf("resolveCloudControlType(%q) = %q, want passthrough %q", tt.input, result, tt.input)
			}
		})
	}
}

func TestResolveCloudControlType_UnknownTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Unknown terraform type", "aws_unknown_resource"},
		{"Completely unknown", "some_random_type"},
		{"Empty string", ""},
		{"Partial Cloud Control format", "AWS::"},
		{"Invalid prefix", "aws::DynamoDB::Table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCloudControlType(tt.input)
			// Unknown types should pass through unchanged
			if result != tt.input {
				t.Errorf("resolveCloudControlType(%q) = %q, want passthrough %q", tt.input, result, tt.input)
			}
		})
	}
}
