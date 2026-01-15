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

# Check for various resource types
data "probe" "dynamodb_table" {
  type = "aws_dynamodb_table"
  id   = "test-table"
}

data "probe" "s3_bucket" {
  type = "aws_s3_bucket"
  id   = "test-bucket"
}

data "probe" "sqs_queue" {
  type = "aws_sqs_queue"
  id   = "test-queue"
}

# You can also use Cloud Control type names directly
data "probe" "lambda_function" {
  type = "AWS::Lambda::Function"
  id   = "test-function"
}

output "resources" {
  value = {
    dynamodb_exists = data.probe.dynamodb_table.exists
    s3_exists       = data.probe.s3_bucket.exists
    sqs_exists      = data.probe.sqs_queue.exists
    lambda_exists   = data.probe.lambda_function.exists
  }
}
