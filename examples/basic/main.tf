terraform {
  required_providers {
    probe = {
      source = "registry.terraform.io/shakefu/probe"
    }
  }
}

provider "probe" {}

data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_exists" {
  value = data.probe.my_table.exists
}

output "table_arn" {
  value = data.probe.my_table.arn
}
