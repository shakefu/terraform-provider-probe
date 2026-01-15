# terraform-provider-probe Design Document

Design document for an open source Terraform/OpenTofu provider that checks whether AWS resources exist without failing when they don't.

**License:** MPL-2.0 (standard for Terraform providers)

**Minimum Requirements:**

- Go 1.21+
- terraform-plugin-framework 1.4+
- Terraform 1.0+ or OpenTofu 1.6+

## Problem Statement

Terraform and OpenTofu have no native way to gracefully check if a resource exists before deciding whether to create it. All existing approaches fail:

1. **Standard data sources** - Fail with an error if the resource doesn't exist
2. **`awscc` plural data sources** - Fail with "Empty result" error when zero resources exist
3. **`external` data source** - Requires bash scripts and manual endpoint configuration
4. **Import blocks** - Fail if the resource doesn't exist (no "optional import")

This is a well-documented gap:

- [terraform#30291](https://github.com/hashicorp/terraform/issues/30291) - "data source to return null if resource does not exist"
- [terraform#34224](https://github.com/hashicorp/terraform/issues/34224) - "Data sources to optionally not fail if resource not found"
- [opentofu#1289](https://github.com/opentofu/opentofu/issues/1289) - Closed as infeasible at core level

The OpenTofu team explicitly recommends implementing this at the **provider level**.

## Use Cases

1. **Create-or-adopt pattern**: Create a resource if it doesn't exist, reference it if it does
2. **E2E testing**: Run tests against LocalStack with or without pre-existing resources
3. **Multi-tool environments**: Terraform configs that coexist with CDK, CloudFormation, or manual resources
4. **Safe deployments**: Avoid destroying resources created by other tools

## Design Decisions

### 1. Use AWS Cloud Control API

The [AWS Cloud Control API](https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/what-is-cloudcontrolapi.html) provides a uniform interface for resource operations across 950+ AWS resource types.

**Key operations:**

- `GetResource(TypeName, Identifier)` - Check if a specific resource exists
- `ListResources(TypeName)` - List all resources of a type

**Benefits:**

- Single implementation handles DynamoDB, S3, SQS, Lambda, IAM, etc.
- No per-resource-type code needed
- Returns resource properties as JSON
- Supported by LocalStack (Ultimate tier)

**Resource type examples:**

| Cloud Control Type | Identifier | Notes |
|-------------------|------------|-------|
| `AWS::DynamoDB::Table` | Table name | |
| `AWS::S3::Bucket` | Bucket name | |
| `AWS::SQS::Queue` | Queue name | Cloud Control uses name, not URL |
| `AWS::Lambda::Function` | Function name | |
| `AWS::IAM::Role` | Role name | |

**Note:** Some Cloud Control resource types require composite identifiers (e.g., `parent-id|resource-name`). The provider should handle common cases and document identifier formats per resource type.

### 2. Use Terraform Plugin Framework

Use the official [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework) with the [scaffolding template](https://github.com/hashicorp/terraform-provider-scaffolding-framework).

**Why:**

- Modern, recommended approach for new providers
- Better type safety than terraform-plugin-sdk/v2
- Native support for complex types
- Active development and documentation

### 3. Map Terraform Resource Names to Cloud Control Types

Users shouldn't need to know CloudFormation type strings. The provider should accept familiar Terraform resource names and map them internally.

**User-facing syntax:**

```hcl
data "probe" "my_table" {
  type = "aws_dynamodb_table"  # Terraform-style
  id   = "my-table"
}
```

**Internal mapping:**

```go
var resourceTypeMap = map[string]string{
    "aws_dynamodb_table":  "AWS::DynamoDB::Table",
    "aws_s3_bucket":       "AWS::S3::Bucket",
    "aws_sqs_queue":       "AWS::SQS::Queue",
    "aws_lambda_function": "AWS::Lambda::Function",
    "aws_iam_role":        "AWS::IAM::Role",
    // ... extensible
}
```

**Also accept raw Cloud Control types** for resources not in the map:

```hcl
data "probe" "custom" {
  type = "AWS::Some::NewResource"  # Pass-through if not in map
  id   = "my-resource"
}
```

### 4. Auto-detect LocalStack

The provider should automatically detect LocalStack by probing `http://localhost:4566/_localstack/health` and configure endpoints accordingly.

**Detection logic:**

1. Check if `localstack` provider attribute is explicitly set
2. If not explicitly disabled, probe LocalStack health endpoint at `http://localhost:4566/_localstack/health`
3. If LocalStack responds (or `localstack = true`), configure Cloud Control endpoint to LocalStack
4. Otherwise, use standard AWS endpoints

**Provider configuration:**

```hcl
provider "probe" {
  # Optional: Explicitly enable/disable LocalStack detection
  # Set to false to force real AWS even when LocalStack is running
  # localstack = false

  # Optional: Override endpoint (implies localstack = true)
  # endpoint = "http://localhost:4566"

  # Optional: Explicit region (defaults to AWS_REGION env var, then us-east-1)
  # region = "us-west-2"
}
```

**Credentials:** Uses standard AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (EC2, ECS, Lambda)

For LocalStack, dummy credentials are automatically used if none are configured.

### 5. Data Source Schema

Single data source type named `probe`:

**Inputs:**

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Resource type (e.g., `aws_dynamodb_table` or `AWS::DynamoDB::Table`) |
| `id` | string | Yes | Resource identifier (table name, bucket name, etc.) |

**Outputs:**

| Attribute | Type | Description |
|-----------|------|-------------|
| `exists` | bool | Whether the resource exists |
| `arn` | string | Resource ARN (null if not exists) |
| `properties` | object | Decoded resource properties as a map (null if not exists) |

**ARN extraction:** The provider extracts ARN from properties automatically, checking common property names: `Arn`, `TableArn`, `FunctionArn`, `RoleArn`, `BucketArn`, `QueueArn`, etc.

**Properties:** Returned as a native Terraform object/map, not a JSON string. Users can access properties directly:

```hcl
data.probe.my_table.properties.TableStatus
data.probe.my_bucket.properties.CreationDate
```

### 6. Error Handling

**Critical design choice:** Return `exists = false` instead of failing when resource doesn't exist.

```go
func (d *ProbeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
    var config ProbeModel
    resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

    cloudControlType := resolveCloudControlType(config.Type.ValueString())

    result, err := d.client.GetResource(ctx, &cloudcontrol.GetResourceInput{
        TypeName:   aws.String(cloudControlType),
        Identifier: aws.String(config.Id.ValueString()),
    })

    if err != nil {
        var notFound *cctypes.ResourceNotFoundException
        if errors.As(err, &notFound) {
            // NOT AN ERROR - just doesn't exist
            config.Exists = types.BoolValue(false)
            config.Arn = types.StringNull()
            config.Properties = types.DynamicNull()
            resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
            return
        }
        // Actual errors (permissions, network, etc.) should fail
        resp.Diagnostics.AddError("Probe failed", err.Error())
        return
    }

    // Parse properties JSON into dynamic type
    props := parsePropertiesToDynamic(*result.ResourceDescription.Properties)

    config.Exists = types.BoolValue(true)
    config.Properties = props
    config.Arn = extractArn(props)  // Check Arn, TableArn, FunctionArn, etc.
    resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
```

## Usage Examples

### Basic existence check

```hcl
data "probe" "contacts_table" {
  type = "aws_dynamodb_table"
  id   = "myapp-contacts"
}

output "table_exists" {
  value = data.probe.contacts_table.exists
}
```

### Create-or-adopt with OpenTofu

```hcl
data "probe" "contacts_table" {
  type = "aws_dynamodb_table"
  id   = "${var.prefix}-contacts"
}

resource "aws_dynamodb_table" "contacts" {
  name         = "${var.prefix}-contacts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }

  lifecycle {
    # OpenTofu 1.11+ only
    enabled = !data.probe.contacts_table.exists
  }
}

# Works regardless of whether table was created or already existed
output "contacts_table_arn" {
  value = data.probe.contacts_table.exists
    ? data.probe.contacts_table.arn
    : aws_dynamodb_table.contacts.arn
}
```

### Create-or-adopt with Terraform (count pattern)

```hcl
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
  value = data.probe.contacts_table.exists
    ? data.aws_dynamodb_table.contacts[0].arn
    : aws_dynamodb_table.contacts[0].arn
}
```

### Access resource properties

```hcl
data "probe" "my_bucket" {
  type = "aws_s3_bucket"
  id   = "my-bucket"
}

# Properties are decoded - no jsondecode needed
output "bucket_creation_date" {
  value = data.probe.my_bucket.exists ? data.probe.my_bucket.properties.CreationDate : null
}

output "bucket_arn" {
  value = data.probe.my_bucket.arn  # Extracted automatically
}
```

## Project Structure

```
terraform-provider-probe/
├── main.go
├── go.mod
├── go.sum
├── internal/
│   └── provider/
│       ├── provider.go              # Provider config, LocalStack detection
│       ├── probe_data_source.go  # The single data source
│       ├── resource_types.go        # TF name → Cloud Control type mapping
│       └── arn_extractor.go         # ARN extraction from properties
├── examples/
│   ├── basic/
│   ├── dynamodb/
│   └── localstack/
├── docs/
│   └── data-sources/
│       └── probe.md
├── .goreleaser.yml                  # For releases
└── README.md
```

## Implementation Notes

### LocalStack Detection

```go
func detectLocalStack() (bool, string) {
    client := &http.Client{Timeout: 500 * time.Millisecond}
    resp, err := client.Get("http://localhost:4566/_localstack/health")
    if err != nil {
        return false, ""
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        return true, "http://localhost:4566"
    }
    return false, ""
}
```

### Provider Configuration for LocalStack

When LocalStack is detected, configure the Cloud Control client:

```go
cfg, _ := config.LoadDefaultConfig(ctx,
    config.WithRegion(region),
)

if isLocalStack {
    cfg.BaseEndpoint = aws.String(localStackEndpoint)
    // Use dummy credentials if none configured
    if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
        cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
    }
}

client := cloudcontrol.NewFromConfig(cfg)
```

### Resource Type Mapping

```go
var tfToCloudControl = map[string]string{
    // Compute
    "aws_lambda_function": "AWS::Lambda::Function",
    "aws_ecs_cluster":     "AWS::ECS::Cluster",
    "aws_ecs_service":     "AWS::ECS::Service",

    // Storage
    "aws_dynamodb_table":  "AWS::DynamoDB::Table",
    "aws_s3_bucket":       "AWS::S3::Bucket",

    // Messaging
    "aws_sqs_queue":       "AWS::SQS::Queue",
    "aws_sns_topic":       "AWS::SNS::Topic",

    // IAM
    "aws_iam_role":        "AWS::IAM::Role",
    "aws_iam_policy":      "AWS::IAM::ManagedPolicy",

    // API Gateway
    "aws_apigatewayv2_api": "AWS::ApiGatewayV2::Api",
}

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
```

### ARN Extraction

Cloud Control returns properties as JSON with varying ARN field names:

```go
var arnFields = []string{"Arn", "TableArn", "FunctionArn", "RoleArn", "BucketArn", "QueueArn", "TopicArn"}

func extractArn(properties map[string]interface{}) string {
    for _, field := range arnFields {
        if arn, ok := properties[field].(string); ok && arn != "" {
            return arn
        }
    }
    return ""
}
```

### Properties to Dynamic Type

```go
func parsePropertiesToDynamic(jsonStr string) types.Dynamic {
    var props map[string]interface{}
    if err := json.Unmarshal([]byte(jsonStr), &props); err != nil {
        return types.DynamicNull()
    }
    // Convert to tftypes for Dynamic
    val, err := convertToTfValue(props)
    if err != nil {
        return types.DynamicNull()
    }
    return types.DynamicValue(val)
}
```

## Testing Strategy

### Unit Tests

- Mock Cloud Control client responses
- Test resource type mapping
- Test ARN extraction for various resource types
- Test LocalStack detection logic

### Integration Tests (LocalStack)

```bash
# Start LocalStack
docker compose up -d localstack

# Run integration tests
go test -tags=integration ./...
```

### Acceptance Tests

Use Terraform acceptance test framework:

```go
func TestAccProbe_DynamoDB(t *testing.T) {
    resource.Test(t, resource.TestCase{
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: `
                    data "probe" "test" {
                        type = "aws_dynamodb_table"
                        id   = "nonexistent-table"
                    }
                `,
                Check: resource.ComposeTestCheckFunc(
                    resource.TestCheckResourceAttr("data.probe.test", "exists", "false"),
                ),
            },
        },
    })
}
```

### CI/CD

- GitHub Actions workflow
- Run unit tests on every PR
- Run integration tests against LocalStack
- Release via GoReleaser when tags are pushed

## Future Extensions

1. **Additional cloud providers** - `azureprobe`, `gcpprobe` equivalents
2. **List operations** - `data "probe_list"` for listing resources by type (return empty list, not error)
3. **Caching** - Optional caching of existence checks within a plan/apply
4. **Metrics** - Track probe latency and cache hit rates
5. **Resource type discovery** - Auto-generate mapping from CloudFormation registry

## References

- [AWS Cloud Control API](https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/what-is-cloudcontrolapi.html)
- [AWS Cloud Control Supported Resources](https://docs.aws.amazon.com/cloudcontrolapi/latest/userguide/supported-resources.html)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Provider Scaffolding Template](https://github.com/hashicorp/terraform-provider-scaffolding-framework)
- [LocalStack Cloud Control Support](https://docs.localstack.cloud/aws/services/cloudcontrol/)
- [OpenTofu `enabled` meta-argument](https://opentofu.org/docs/v1.11/language/meta-arguments/enabled/)
- [CloudFormation Resource Type Schema](https://docs.aws.amazon.com/cloudformation-cli/latest/userguide/resource-type-schema.html)
