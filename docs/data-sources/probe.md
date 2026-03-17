---
page_title: "probe Data Source - terraform-provider-probe"
subcategory: ""
description: |-
  Checks whether an AWS resource exists without failing when it doesn't.
---

# probe (Data Source)

Checks whether an AWS resource exists without failing when it doesn't.

This data source uses native AWS SDK calls to check for resource existence
and retrieve properties. Unlike standard AWS data sources that fail with an
error when a resource doesn't exist, this data source returns `exists = false`.

## Example Usage

### Basic existence check

```terraform
data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_exists" {
  value = data.probe.my_table.exists
}
```

### Create-or-adopt pattern (Terraform)

```terraform
data "probe" "contacts_table" {
  type = "aws_dynamodb_table"
  id   = "${var.prefix}-contacts"
}

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

data "aws_dynamodb_table" "contacts" {
  count = data.probe.contacts_table.exists ? 1 : 0
  name  = "${var.prefix}-contacts"
}

output "contacts_table_arn" {
  value = (
    data.probe.contacts_table.exists
    ? data.aws_dynamodb_table.contacts[0].arn
    : aws_dynamodb_table.contacts[0].arn
  )
}
```

### Accessing Tags

```terraform
data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_tags" {
  value = data.probe.my_table.exists ? data.probe.my_table.properties.Tags : null
}
```

### S3 bucket prefix match

```terraform
data "probe" "my_bucket" {
  type = "aws_s3_bucket"
  id   = "dev-data-bucket-*"
}

output "bucket_exists" {
  value = data.probe.my_bucket.exists
}

output "bucket_name" {
  value = (
    data.probe.my_bucket.exists
    ? data.probe.my_bucket.properties.BucketName
    : null
  )
}
```

> **Note:** When `id` ends with `*`, the provider uses
> prefix matching via `ListBuckets` to find a single
> bucket whose name starts with the given prefix. If
> multiple buckets match, the provider returns an error.
> Prefix matching requires `s3:ListAllMyBuckets`
> permission, which is broader than the `s3:ListBucket`
> permission used for exact lookups.

### VPC existence check (by Name tag)

```terraform
data "probe" "my_vpc" {
  type = "aws_vpc"
  id   = "production-vpc"
}

output "vpc_exists" {
  value = data.probe.my_vpc.exists
}

output "vpc_id" {
  value = data.probe.my_vpc.exists ? data.probe.my_vpc.properties.VpcId : null
}
```

### VPC lookup by ID

```terraform
data "probe" "my_vpc" {
  type = "aws_vpc"
  id   = "vpc-0abc123def456789"
}
```

> **Note:** When using `aws_vpc`, the `id` field accepts either a VPC ID
> (starting with `vpc-`) or a Name tag value. If looking up by Name tag and
> multiple VPCs share the same name, the provider returns an error.

### OpenSearch domain existence check

```terraform
data "probe" "search" {
  type = "aws_opensearch_domain"
  id   = "my-search-domain"
}

output "domain_exists" {
  value = data.probe.search.exists
}

output "domain_endpoint" {
  value = (
    data.probe.search.exists
    ? data.probe.search.properties.Endpoint
    : null
  )
}
```

## Schema

### Required

- `type` (String) Resource type. Accepts Terraform-style names
  (e.g., `aws_dynamodb_table`) or AWS-style type names
  (e.g., `AWS::DynamoDB::Table`).
- `id` (String) Resource identifier (table name, bucket name, etc.).

### Read-Only

- `exists` (Boolean) Whether the resource exists.
- `arn` (String) Resource ARN. Null if the resource does not exist.
- `properties` (Dynamic) Resource properties including Tags when available.
  Null if the resource does not exist.

## Supported Resource Types

The provider uses native AWS SDK calls for each resource type. Currently
supported:

| Terraform Type       | AWS Type               | Identifier     |
| -------------------- | ---------------------- | -------------- |
| `aws_dynamodb_table` | `AWS::DynamoDB::Table` | Table name     |
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name or prefix`*` |
| `aws_vpc`            | `AWS::EC2::VPC`        | VPC ID or Name tag |
| `aws_opensearch_domain` | `AWS::OpenSearch::Domain` | Domain name |

Additional resource types will be added incrementally. Contributions welcome!
