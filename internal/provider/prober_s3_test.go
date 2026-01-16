// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestS3Prober_BucketNotFound(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	prober := NewS3Prober(*cfg)
	result, err := prober.Probe(context.Background(), "nonexistent-bucket-12345-xyz")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exists {
		t.Error("expected Exists to be false for nonexistent bucket")
	}
}

func TestS3Prober_BucketExists(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := s3.NewFromConfig(*cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	bucketName := "probe-test-s3-exists"

	// Create a test bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create test bucket: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Test the prober
	prober := NewS3Prober(*cfg)
	result, err := prober.Probe(ctx, bucketName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing bucket")
	}

	if result.Arn == "" {
		t.Error("expected ARN to be populated")
	}

	expectedArn := "arn:aws:s3:::" + bucketName
	if result.Arn != expectedArn {
		t.Errorf("expected ARN=%q, got %q", expectedArn, result.Arn)
	}

	if result.Properties["BucketName"] != bucketName {
		t.Errorf("expected BucketName=%q, got %q", bucketName, result.Properties["BucketName"])
	}
}

func TestS3Prober_BucketWithTags(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := s3.NewFromConfig(*cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	bucketName := "probe-test-s3-tags"

	// Create a test bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create test bucket: %v", err)
	}

	// Add tags
	_, err = client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(bucketName),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("Environment"), Value: aws.String("test")},
				{Key: aws.String("Owner"), Value: aws.String("probe-provider")},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to tag bucket: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Test the prober
	prober := NewS3Prober(*cfg)
	result, err := prober.Probe(ctx, bucketName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing bucket")
	}

	// Check tags
	if result.Tags == nil {
		t.Fatal("expected Tags to be populated")
	}

	if result.Tags["Environment"] != "test" {
		t.Errorf("expected Environment tag='test', got %q", result.Tags["Environment"])
	}

	if result.Tags["Owner"] != "probe-provider" {
		t.Errorf("expected Owner tag='probe-provider', got %q", result.Tags["Owner"])
	}

	// Check tags in properties
	propTags, ok := result.Properties["Tags"].(map[string]string)
	if !ok {
		t.Error("expected Tags in Properties to be map[string]string")
	} else if propTags["Environment"] != "test" {
		t.Errorf("expected Properties.Tags.Environment='test', got %q", propTags["Environment"])
	}
}
