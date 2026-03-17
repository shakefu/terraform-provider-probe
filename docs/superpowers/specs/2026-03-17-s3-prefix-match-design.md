# S3 Prefix Match Design

## Goal

Support prefix matching for S3 bucket names in the probe
provider, so users can find buckets with appended hashes
(e.g., `dev-some-bucket-a3f8b2`) using a prefix like
`dev-some-bucket-*`.

## Behavior

- If the `id` ends with `*`, strip the `*` and perform a
  prefix match
- Only a single trailing `*` is supported. If `*` appears
  anywhere else in the identifier, return a validation
  error: `"wildcard (*) is only supported at the end of
  S3 bucket identifiers"`
- Bare `*` (empty prefix) is rejected with a validation
  error: `"S3 bucket prefix must not be empty"`
- Without `*`, the current exact `HeadBucket` behavior is
  unchanged (backwards-compatible)
- Prefix matching uses the `ListBuckets` API with the
  server-side `Prefix` parameter
- Result rules:
  - 0 matches: `exists: false`
  - 1 match: return full probe result (ARN, properties,
    tags -- same as exact match)
  - 2+ matches: error
    `"multiple S3 buckets found matching prefix %q
    (found %d)"`

## Permissions

Prefix matching requires `s3:ListAllMyBuckets` at the
account level, which is broader than the `s3:ListBucket`
permission needed for exact `HeadBucket` lookups. This
difference must be documented so users with tightly scoped
IAM policies are aware.

## Error Handling

- Access denied on `ListBuckets` propagates as an error
  (not treated as not-found), since it indicates a
  permission problem, not absence
- Other `ListBuckets` errors propagate as errors
- After resolving a prefix to a single bucket name, the
  existing `HeadBucket` + property/tag fetching logic is
  reused, with its existing error handling

## Implementation

All changes are in the existing `prober_s3.go`. At the top
of the `Probe()` method, check if the identifier ends with
`*`. If so, validate (no mid-string wildcards, non-empty
prefix), then call `ListBuckets` with the `Prefix`
parameter. If exactly one bucket matches, recurse into
`Probe()` with the resolved bucket name to reuse the
existing exact-match path.

No new files. No interface changes. No registry changes.

## Testing

Add to `prober_s3_test.go`:

- `TestS3Prober_PrefixNotFound` -- prefix with no matching
  buckets returns `Exists: false`
- `TestS3Prober_PrefixSingleMatch` -- prefix matching
  exactly one bucket returns full result with correct
  bucket name, ARN, and properties
- `TestS3Prober_PrefixMultipleMatches` -- prefix matching
  multiple buckets returns error containing
  "multiple S3 buckets found"
- `TestS3Prober_PrefixBareWildcard` -- bare `*` returns
  validation error
- `TestS3Prober_PrefixMidWildcard` -- `dev-*-bucket`
  returns validation error

Add acceptance test to `probe_data_source_test.go`:

- `TestAccProbeDataSource_s3PrefixMatch` -- create bucket
  with known name, probe with prefix `*`, verify exists
  and properties

## Documentation

Update `docs/data-sources/probe.md`:

- Update S3 row identifier column to:
  `Bucket name or prefix*`
- Add example showing prefix match usage
- Add note explaining the `*` suffix behavior and the
  `s3:ListAllMyBuckets` permission requirement
