# S3 Prefix Match Implementation Plan

> **For agentic workers:** REQUIRED: Use
> superpowers:subagent-driven-development (if subagents
> available) or superpowers:executing-plans to implement
> this plan. Steps use checkbox (`- [ ]`) syntax for
> tracking.

**Goal:** Add prefix matching to the S3 prober so
`dev-bucket-*` finds `dev-bucket-a3f8b2`.

**Architecture:** Modify `Probe()` in `prober_s3.go` to
detect trailing `*`, validate the identifier, call
`ListBuckets` with server-side `Prefix`, then recurse
with the resolved bucket name for the existing exact-match
path.

**Tech Stack:** Existing `s3.Client`, `ListBuckets` API
with `Prefix` parameter

---

## Tasks

### Task 1: Write prefix validation and matching tests

**Files:**

- Modify: `internal/provider/prober_s3_test.go`

- [ ] **Step 1: Add prefix unit tests**

Append to `internal/provider/prober_s3_test.go`:

```go
func TestS3Prober_PrefixBareWildcard(t *testing.T) {
 cfg := getLocalStackConfig(t)
 if cfg == nil {
  t.Skip("LocalStack not available")
 }

 prober := NewS3Prober(*cfg)
 _, err := prober.Probe(context.Background(), "*")

 if err == nil {
  t.Fatal("expected error for bare wildcard")
 }

 if !strings.Contains(err.Error(),
  "S3 bucket prefix must not be empty") {
  t.Errorf("unexpected error: %v", err)
 }
}

func TestS3Prober_PrefixMidWildcard(t *testing.T) {
 cfg := getLocalStackConfig(t)
 if cfg == nil {
  t.Skip("LocalStack not available")
 }

 prober := NewS3Prober(*cfg)
 _, err := prober.Probe(context.Background(),
  "dev-*-bucket")

 if err == nil {
  t.Fatal("expected error for mid-string wildcard")
 }

 if !strings.Contains(err.Error(),
  "wildcard (*) is only supported at the end") {
  t.Errorf("unexpected error: %v", err)
 }
}

func TestS3Prober_PrefixNotFound(t *testing.T) {
 cfg := getLocalStackConfig(t)
 if cfg == nil {
  t.Skip("LocalStack not available")
 }

 prober := NewS3Prober(*cfg)
 result, err := prober.Probe(context.Background(),
  "nonexistent-prefix-xyz-*")

 if err != nil {
  t.Fatalf("unexpected error: %v", err)
 }

 if result.Exists {
  t.Error("expected Exists to be false")
 }
}

func TestS3Prober_PrefixSingleMatch(t *testing.T) {
 cfg := getLocalStackConfig(t)
 if cfg == nil {
  t.Skip("LocalStack not available")
 }

 ctx := context.Background()
 client := s3.NewFromConfig(*cfg, func(o *s3.Options) {
  o.UsePathStyle = true
 })
 bucketName := "probe-prefix-test-abc123"

 _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
  Bucket: aws.String(bucketName),
 })
 if err != nil {
  t.Fatalf("failed to create test bucket: %v", err)
 }

 t.Cleanup(func() {
  _, _ = client.DeleteBucket(ctx,
   &s3.DeleteBucketInput{
    Bucket: aws.String(bucketName),
   })
 })

 prober := NewS3Prober(*cfg)
 result, err := prober.Probe(ctx,
  "probe-prefix-test-*")

 if err != nil {
  t.Fatalf("unexpected error: %v", err)
 }

 if !result.Exists {
  t.Error("expected Exists to be true")
 }

 expectedArn := "arn:aws:s3:::" + bucketName
 if result.Arn != expectedArn {
  t.Errorf("expected ARN=%q, got %q",
   expectedArn, result.Arn)
 }

 if result.Properties["BucketName"] != bucketName {
  t.Errorf("expected BucketName=%q, got %q",
   bucketName,
   result.Properties["BucketName"])
 }
}

func TestS3Prober_PrefixMultipleMatches(t *testing.T) {
 cfg := getLocalStackConfig(t)
 if cfg == nil {
  t.Skip("LocalStack not available")
 }

 ctx := context.Background()
 client := s3.NewFromConfig(*cfg, func(o *s3.Options) {
  o.UsePathStyle = true
 })

 buckets := []string{
  "probe-multi-test-aaa",
  "probe-multi-test-bbb",
 }
 for _, name := range buckets {
  _, err := client.CreateBucket(ctx,
   &s3.CreateBucketInput{
    Bucket: aws.String(name),
   })
  if err != nil {
   t.Fatalf("failed to create bucket %s: %v",
    name, err)
  }
 }

 t.Cleanup(func() {
  for _, name := range buckets {
   _, _ = client.DeleteBucket(ctx,
    &s3.DeleteBucketInput{
     Bucket: aws.String(name),
    })
  }
 })

 prober := NewS3Prober(*cfg)
 _, err := prober.Probe(ctx, "probe-multi-test-*")

 if err == nil {
  t.Fatal("expected error for multiple matches")
 }

 if !strings.Contains(err.Error(),
  "multiple S3 buckets found") {
  t.Errorf("unexpected error: %v", err)
 }
}
```

- [ ] **Step 2: Add `"strings"` to imports if not present**

The test file needs `"strings"` in its import block. Add
it after `"context"` if not already there.

- [ ] **Step 3: Verify tests fail to compile**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go build ./... 2>&1 | head -5
```

Expected: Tests compile (no new symbols yet, tests just
call existing `Probe` with new inputs). Some tests will
pass trivially (validation errors won't fire yet), prefix
tests will fail at runtime.

- [ ] **Step 4: Commit tests**

```bash
git add internal/provider/prober_s3_test.go
git commit -m "test: add S3 prefix matching unit tests"
```

---

### Task 2: Implement prefix matching in S3 prober

**Files:**

- Modify: `internal/provider/prober_s3.go`

- [ ] **Step 1: Add `"strings"` to imports**

In `prober_s3.go`, add `"strings"` to the import block.

- [ ] **Step 2: Add prefix logic at the top of `Probe()`**

Replace the existing `Probe` method with:

```go
// Probe checks whether an S3 bucket exists and retrieves
// its properties. The identifier is the bucket name, or a
// prefix ending with * to match a single bucket by prefix.
func (p *S3Prober) Probe(
 ctx context.Context, identifier string,
) (*ProbeResult, error) {
 // Handle prefix matching
 if strings.HasSuffix(identifier, "*") {
  return p.probeByPrefix(ctx, identifier)
 }

 // Validate no wildcard in non-trailing position
 if strings.Contains(identifier, "*") {
  return nil, fmt.Errorf(
   "wildcard (*) is only supported at the " +
    "end of S3 bucket identifiers")
 }

 // HeadBucket returns success or NotFound/Forbidden
 _, err := p.client.HeadBucket(ctx,
  &s3.HeadBucketInput{
   Bucket: aws.String(identifier),
  })

 if err != nil {
  var notFoundErr *types.NotFound
  var noSuchBucket *types.NoSuchBucket
  if errors.As(err, &notFoundErr) ||
   errors.As(err, &noSuchBucket) {
   return &ProbeResult{Exists: false}, nil
  }
  if isS3NotFound(err) {
   return &ProbeResult{Exists: false}, nil
  }
  return nil, err
 }

 // Bucket exists - construct ARN
 arn := fmt.Sprintf("arn:aws:s3:::%s", identifier)

 result := &ProbeResult{
  Exists: true,
  Arn:    arn,
  Properties: map[string]any{
   "BucketName": identifier,
   "Arn":        arn,
  },
 }

 // Get bucket location (region)
 location, err := p.client.GetBucketLocation(ctx,
  &s3.GetBucketLocationInput{
   Bucket: aws.String(identifier),
  })
 if err == nil {
  region := string(location.LocationConstraint)
  if region == "" {
   region = "us-east-1"
  }
  result.Properties["Region"] = region
 }

 // Get tags
 tags, err := p.client.GetBucketTagging(ctx,
  &s3.GetBucketTaggingInput{
   Bucket: aws.String(identifier),
  })
 if err == nil && len(tags.TagSet) > 0 {
  result.Tags = make(map[string]string,
   len(tags.TagSet))
  for _, tag := range tags.TagSet {
   result.Tags[aws.ToString(tag.Key)] =
    aws.ToString(tag.Value)
  }
  result.Properties["Tags"] = result.Tags
 }

 return result, nil
}

// probeByPrefix resolves a prefix pattern to a single
// bucket, then probes it.
func (p *S3Prober) probeByPrefix(
 ctx context.Context, identifier string,
) (*ProbeResult, error) {
 prefix := strings.TrimSuffix(identifier, "*")

 if prefix == "" {
  return nil, fmt.Errorf(
   "S3 bucket prefix must not be empty")
 }

 // Validate no wildcard in the prefix portion
 if strings.Contains(prefix, "*") {
  return nil, fmt.Errorf(
   "wildcard (*) is only supported at the " +
    "end of S3 bucket identifiers")
 }

 output, err := p.client.ListBuckets(ctx,
  &s3.ListBucketsInput{
   Prefix: aws.String(prefix),
  })
 if err != nil {
  return nil, err
 }

 if len(output.Buckets) == 0 {
  return &ProbeResult{Exists: false}, nil
 }

 if len(output.Buckets) > 1 {
  return nil, fmt.Errorf(
   "multiple S3 buckets found matching "+
    "prefix %q (found %d)",
   prefix, len(output.Buckets))
 }

 // Exactly one match - probe it by exact name
 bucketName := aws.ToString(
  output.Buckets[0].Name)
 return p.Probe(ctx, bucketName)
}
```

- [ ] **Step 3: Verify build compiles**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go build ./...
```

- [ ] **Step 4: Run prefix tests**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go test -v -run 'TestS3Prober_Prefix' \
  ./internal/provider/
```

Expected: All pass (or skip if no LocalStack).

- [ ] **Step 5: Run all S3 tests to verify no regression**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go test -v -run 'TestS3Prober|TestContains|TestIsS3' \
  ./internal/provider/
```

Expected: All existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/prober_s3.go
git commit -m "feat: add S3 bucket prefix matching"
```

---

### Task 3: Add acceptance test

**Files:**

- Modify: `internal/provider/probe_data_source_test.go`

- [ ] **Step 1: Add S3 prefix acceptance test**

Append to `internal/provider/probe_data_source_test.go`:

```go
func TestAccProbeDataSource_s3PrefixMatch(t *testing.T) {
 if !useLocalStack() {
  t.Skip(
   "S3 prefix test only runs against LocalStack")
 }

 resource.Test(t, resource.TestCase{
  PreCheck: func() { testAccPreCheck(t) },
  ProtoV6ProviderFactories:
   testAccProtoV6ProviderFactories,
  ExternalProviders: map[string]resource.ExternalProvider{
   "aws": {
    Source:            "hashicorp/aws",
    VersionConstraint: "~> 5.0",
   },
  },
  Steps: []resource.TestStep{
   {
    Config:
     testAccProbeDataSourceConfig_s3Prefix_localstack,
    Check: resource.ComposeAggregateTestCheckFunc(
     resource.TestCheckResourceAttr(
      "data.probe.test",
      "exists", "true"),
     resource.TestCheckResourceAttrSet(
      "data.probe.test", "arn"),
     resource.TestCheckResourceAttrSet(
      "data.probe.test",
      "properties.%"),
     resource.TestCheckResourceAttrSet(
      "data.probe.test",
      "properties.BucketName"),
    ),
   },
  },
 })
}

const testAccProbeDataSourceConfig_s3Prefix_localstack = `
provider "probe" {
  localstack = true
}

provider "aws" {
  region                      = "us-east-1"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  access_key                  = "test"
  secret_key                  = "test"

  endpoints {
    s3 = "http://localhost:4566"
  }
}

resource "aws_s3_bucket" "test" {
  bucket = "probe-acc-prefix-test-hash789"
}

data "probe" "test" {
  type = "aws_s3_bucket"
  id   = "probe-acc-prefix-test-*"

  depends_on = [aws_s3_bucket.test]
}
`
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/provider/probe_data_source_test.go
git commit -m \
  "test: add S3 prefix match acceptance test"
```

---

### Task 4: Update documentation

**Files:**

- Modify: `docs/data-sources/probe.md`

- [ ] **Step 1: Update S3 row in supported types table**

Change the S3 row from:

```markdown
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name    |
```

To:

```markdown
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name or prefix`*` |
```

- [ ] **Step 2: Add S3 prefix example**

Add after the "Accessing Tags" example (before the VPC
section):

````markdown
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
````

- [ ] **Step 3: Commit**

```bash
git add docs/data-sources/probe.md
git commit -m \
  "docs: add S3 prefix match to supported types and examples"
```

---

### Task 5: Full verification

- [ ] **Step 1: Format code**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go fmt ./...
```

- [ ] **Step 2: Run go vet**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go vet ./...
```

- [ ] **Step 3: Run full test suite**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && go test ./...
```

Expected: All tests pass.

- [ ] **Step 4: Run prek**

```bash
cd /Users/shakefu/git/shakefu/terraform-provider-probe \
  && prek run --all-files
```

Expected: All hooks pass.
