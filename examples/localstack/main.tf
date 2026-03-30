terraform {
  required_providers {
    probe = {
      source = "registry.terraform.io/shakefu/probe"
    }
  }
}

# Provider auto-detects LocalStack at localhost:4566
# Or explicitly enable it:
provider "probe" {
  # localstack = true
  # endpoint = "http://localhost:4566"
}

# Check for supported resource types
data "probe" "dynamodb_table" {
  type = "aws_dynamodb_table"
  id   = "test-table"
}

data "probe" "s3_bucket" {
  type = "aws_s3_bucket"
  id   = "test-bucket"
}

data "probe" "vpc" {
  type = "aws_vpc"
  id   = "test-vpc"
}

data "probe" "opensearch" {
  type = "aws_opensearch_domain"
  id   = "test-domain"
}

# You can also use AWS CloudFormation type names
data "probe" "dynamodb_cfn_style" {
  type = "AWS::DynamoDB::Table"
  id   = "another-table"
}

output "resources" {
  value = {
    dynamodb_exists     = data.probe.dynamodb_table.exists
    s3_exists           = data.probe.s3_bucket.exists
    vpc_exists          = data.probe.vpc.exists
    opensearch_exists   = data.probe.opensearch.exists
    dynamodb_cfn_exists = data.probe.dynamodb_cfn_style.exists
  }
}
