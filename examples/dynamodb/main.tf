terraform {
  required_providers {
    probe = {
      source = "registry.terraform.io/shakefu/probe"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "prefix" {
  description = "Resource name prefix"
  default     = "myapp"
}

provider "probe" {}

provider "aws" {
  region = "us-east-1"
}

# Check if the table exists
data "probe" "contacts_table" {
  type = "aws_dynamodb_table"
  id   = "${var.prefix}-contacts"
}

# Create the table only if it doesn't exist (using count pattern for Terraform)
resource "aws_dynamodb_table" "contacts" {
  count = data.probe.contacts_table.exists ? 0 : 1

  name         = "${var.prefix}-contacts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }
}

# Reference existing table if it exists
data "aws_dynamodb_table" "contacts" {
  count = data.probe.contacts_table.exists ? 1 : 0
  name  = "${var.prefix}-contacts"
}

# Output works regardless of whether table was created or already existed
output "contacts_table_arn" {
  value = data.probe.contacts_table.exists ? data.aws_dynamodb_table.contacts[0].arn : aws_dynamodb_table.contacts[0].arn
}
