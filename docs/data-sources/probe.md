---
page_title: "probe Data Source - terraform-provider-probe"
subcategory: ""
description: |-
  Checks whether an AWS resource exists without failing when it doesn't.
---

# probe (Data Source)

Checks whether an AWS resource exists without failing when it doesn't.

This data source uses the AWS Cloud Control API to check for resource
existence. Unlike standard AWS data sources that fail with an error when a
resource doesn't exist, this data source returns `exists = false`.

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

### Using Cloud Control type names

```terraform
data "probe" "lambda" {
  type = "AWS::Lambda::Function"
  id   = "my-function"
}
```

## Schema

### Required

- `type` (String) Resource type. Accepts Terraform-style names
  (e.g., `aws_dynamodb_table`) or AWS Cloud Control type names
  (e.g., `AWS::DynamoDB::Table`).
- `id` (String) Resource identifier (table name, bucket name, function
  name, etc.).

### Read-Only

- `exists` (Boolean) Whether the resource exists.
- `arn` (String) Resource ARN. Null if the resource does not exist.
- `properties` (Dynamic) Resource properties as returned by the Cloud
  Control API. Null if the resource does not exist.

## Supported Resource Types

The following Terraform resource type names are mapped to Cloud Control types:

| Terraform Type         | Cloud Control Type       |
| ---------------------- | ------------------------ |
| `aws_dynamodb_table`   | `AWS::DynamoDB::Table`   |
| `aws_s3_bucket`        | `AWS::S3::Bucket`        |
| `aws_sqs_queue`        | `AWS::SQS::Queue`        |
| `aws_sns_topic`        | `AWS::SNS::Topic`        |
| `aws_lambda_function`  | `AWS::Lambda::Function`  |
| `aws_iam_role`         | `AWS::IAM::Role`         |
| `aws_iam_policy`       | `AWS::IAM::ManagedPolicy`|
| `aws_ecs_cluster`      | `AWS::ECS::Cluster`      |
| `aws_ecs_service`      | `AWS::ECS::Service`      |
| `aws_apigatewayv2_api` | `AWS::ApiGatewayV2::Api` |

You can also pass Cloud Control type names directly for any resource type
supported by the Cloud Control API.
