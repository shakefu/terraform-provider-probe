---
page_title: "probe_iam_policy_simulation Data Source - terraform-provider-probe"
subcategory: ""
description: |-
  Probes IAM permissions using the AWS Policy Simulator API without failing when permissions are denied.
---

# probe_iam_policy_simulation (Data Source)

Probes IAM permissions using the AWS Policy Simulator API without failing when
permissions are denied. This follows the probe provider philosophy of returning
results instead of errors.

Use this data source to check whether an IAM principal has specific permissions
before conditionally creating resources or policy attachments.

## Example Usage

### Simple permission check

```terraform
data "probe_iam_policy_simulation" "s3_access" {
  policy_source_arn = aws_iam_role.my_role.arn
  actions           = ["s3:GetObject", "s3:PutObject"]
  resource_arns     = ["arn:aws:s3:::my-bucket/*"]
}

output "can_access_s3" {
  value = data.probe_iam_policy_simulation.s3_access.allowed
}
```

### Conditional resource creation

```terraform
data "probe_iam_policy_simulation" "lambda_permissions" {
  policy_source_arn = aws_iam_role.lambda_role.arn
  actions           = ["dynamodb:GetItem", "dynamodb:PutItem"]
  resource_arns     = [aws_dynamodb_table.my_table.arn]
}

# Only create additional permissions if the role doesn't already have them
resource "aws_iam_role_policy" "additional_permissions" {
  count = data.probe_iam_policy_simulation.lambda_permissions.allowed ? 0 : 1

  name = "additional-dynamodb-access"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:GetItem", "dynamodb:PutItem"]
      Resource = aws_dynamodb_table.my_table.arn
    }]
  })
}
```

### Detailed analysis with context

```terraform
data "probe_iam_policy_simulation" "ip_restricted" {
  policy_source_arn = aws_iam_role.my_role.arn
  actions           = ["s3:GetObject"]
  resource_arns     = ["arn:aws:s3:::restricted-bucket/*"]

  context {
    key    = "aws:SourceIp"
    type   = "ip"
    values = ["203.0.113.0/24"]
  }
}

output "results" {
  value = data.probe_iam_policy_simulation.ip_restricted.results
}
```

## Schema

### Required

- `policy_source_arn` (String) ARN of the IAM principal (user, role, group)
  whose policies will be tested.
- `actions` (List of String) List of IAM actions to simulate
  (e.g., `s3:GetObject`, `iam:CreateUser`).

### Optional

- `resource_arns` (List of String) List of resource ARNs to simulate against.
  Defaults to `["*"]` if not specified.
- `caller_arn` (String) ARN of user to simulate as caller. Defaults to
  `policy_source_arn` if it's a user.
- `additional_policies` (List of String) Additional IAM policies (as JSON
  strings) to include in simulation.
- `permissions_boundary_policies` (List of String) Permissions boundary policies
  (as JSON strings) to apply.
- `resource_policy` (String) Resource-based policy (as JSON string) for
  simulation.
- `context` (Block Set) Context entries for condition evaluation. Each block
  accepts:
  - `key` (String, Required) Context key name (e.g., `aws:CurrentTime`).
  - `type` (String, Required) Value type: `string`, `stringList`, `numeric`,
    `numericList`, `boolean`, `booleanList`, `date`, `dateList`, `ip`, `ipList`,
    `binary`, `binaryList`.
  - `values` (List of String, Required) Values for the context key.

### Read-Only

- `allowed` (Boolean) `true` if ALL actions are allowed for ALL resources.
  This is the primary output for simple permission checks.
- `error` (String) Error message if simulation could not be performed (e.g.,
  principal doesn't exist). Null if successful.
- `results` (List of Object) Detailed results for each action/resource
  combination. Each object contains:
  - `action` (String) The action that was evaluated.
  - `resource_arn` (String) The resource ARN evaluated.
  - `allowed` (Boolean) Whether this specific action/resource is allowed.
  - `decision` (String) `allowed`, `explicitDeny`, or `implicitDeny`.
  - `matched_statements` (List of Object) Statements that contributed to the
    decision:
    - `source_policy_id` (String) The identifier of the policy.
    - `source_policy_type` (String) The type of policy (e.g., user, group,
      role, aws-managed, user-managed).
  - `missing_context_keys` (List of String) Context keys required but not
    provided.

## Limitations

- **LocalStack not supported**: The IAM Policy Simulator API
  (`SimulatePrincipalPolicy`) is not available in LocalStack. This data source
  requires real AWS credentials.
- **Required IAM permissions**: The caller must have
  `iam:SimulatePrincipalPolicy` permission to use this data source.
