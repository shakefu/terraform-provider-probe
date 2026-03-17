// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Prober probes S3 buckets using the native AWS SDK.
type S3Prober struct {
	client *s3.Client
	region string
}

// NewS3Prober creates a new S3 prober from an AWS config.
func NewS3Prober(cfg aws.Config) *S3Prober {
	// Check if we're using a custom endpoint (e.g., LocalStack)
	usePathStyle := cfg.BaseEndpoint != nil && *cfg.BaseEndpoint != ""

	return &S3Prober{
		client: s3.NewFromConfig(cfg, func(o *s3.Options) {
			if usePathStyle {
				o.UsePathStyle = true
			}
		}),
		region: cfg.Region,
	}
}

// Probe checks whether an S3 bucket exists and retrieves its properties.
// The identifier is the bucket name, or a prefix pattern ending with *.
func (p *S3Prober) Probe(ctx context.Context, identifier string) (*ProbeResult, error) {
	// Handle prefix matching when identifier ends with *
	if strings.HasSuffix(identifier, "*") {
		return p.probeByPrefix(ctx, identifier)
	}

	// Reject wildcards in non-suffix positions
	if strings.Contains(identifier, "*") {
		return nil, fmt.Errorf("wildcard (*) is only supported at the end of S3 bucket identifiers")
	}

	// HeadBucket returns success or NotFound/Forbidden
	_, err := p.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(identifier),
	})

	if err != nil {
		// Check if the bucket does not exist
		var notFoundErr *types.NotFound
		var noSuchBucket *types.NoSuchBucket
		if errors.As(err, &notFoundErr) || errors.As(err, &noSuchBucket) {
			return &ProbeResult{Exists: false}, nil
		}
		// Other errors (like 403 Forbidden for buckets you don't own) should
		// also be treated as "not found" since you can't access them
		// Check for S3-style "Not Found" responses
		if isS3NotFound(err) {
			return &ProbeResult{Exists: false}, nil
		}
		// Other errors are unexpected
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
	location, err := p.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(identifier),
	})
	if err == nil {
		region := string(location.LocationConstraint)
		if region == "" {
			region = "us-east-1" // Default region has empty LocationConstraint
		}
		result.Properties["Region"] = region
	}

	// Get tags
	tags, err := p.client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(identifier),
	})
	if err == nil && len(tags.TagSet) > 0 {
		result.Tags = make(map[string]string, len(tags.TagSet))
		for _, tag := range tags.TagSet {
			result.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}
		result.Properties["Tags"] = result.Tags
	}

	return result, nil
}

// probeByPrefix resolves a prefix pattern to a single bucket, then probes it.
func (p *S3Prober) probeByPrefix(ctx context.Context, identifier string) (*ProbeResult, error) {
	prefix := strings.TrimSuffix(identifier, "*")

	if prefix == "" {
		return nil, fmt.Errorf("S3 bucket prefix must not be empty")
	}

	// Validate no wildcard in the prefix portion
	if strings.Contains(prefix, "*") {
		return nil, fmt.Errorf("wildcard (*) is only supported at the end of S3 bucket identifiers")
	}

	output, err := p.client.ListBuckets(ctx, &s3.ListBucketsInput{
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	if len(output.Buckets) == 0 {
		return &ProbeResult{Exists: false}, nil
	}

	if len(output.Buckets) > 1 {
		return nil, fmt.Errorf("multiple S3 buckets found matching prefix %q (found %d)", prefix, len(output.Buckets))
	}

	// Exactly one match - probe it by exact name
	bucketName := aws.ToString(output.Buckets[0].Name)
	return p.Probe(ctx, bucketName)
}

// isS3NotFound checks for various S3 "not found" error responses.
func isS3NotFound(err error) bool {
	// S3 returns 404 as a generic HTTP error in some cases
	errStr := err.Error()
	return contains(errStr, "404") || contains(errStr, "NotFound") || contains(errStr, "NoSuchBucket")
}

// contains is a simple string contains helper.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
