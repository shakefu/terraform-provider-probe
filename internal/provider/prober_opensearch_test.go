// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

func TestOpenSearchProber_DomainNotFound(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	prober := NewOpenSearchProber(*cfg)
	result, err := prober.Probe(context.Background(), "nonexistent-domain-12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exists {
		t.Error("expected Exists to be false for nonexistent domain")
	}
}

func TestOpenSearchProber_DomainExists(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := opensearch.NewFromConfig(*cfg)
	domainName := "probe-test-os-exists"

	// Create a test domain
	_, err := client.CreateDomain(ctx, &opensearch.CreateDomainInput{
		DomainName:    aws.String(domainName),
		EngineVersion: aws.String("OpenSearch_2.5"),
		ClusterConfig: &ostypes.ClusterConfig{
			InstanceType:  ostypes.OpenSearchPartitionInstanceTypeT3SmallSearch,
			InstanceCount: aws.Int32(1),
		},
		EBSOptions: &ostypes.EBSOptions{
			EBSEnabled: aws.Bool(true),
			VolumeSize: aws.Int32(10),
			VolumeType: ostypes.VolumeTypeGp2,
		},
	})
	if err != nil {
		t.Fatalf("failed to create test domain: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDomain(ctx, &opensearch.DeleteDomainInput{
			DomainName: aws.String(domainName),
		})
	})

	// Test the prober
	prober := NewOpenSearchProber(*cfg)
	result, err := prober.Probe(ctx, domainName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing domain")
	}

	if result.Arn == "" {
		t.Error("expected ARN to be populated")
	}

	if result.Properties["DomainName"] != domainName {
		t.Errorf("expected DomainName=%q, got %q", domainName, result.Properties["DomainName"])
	}
}

func TestOpenSearchProber_DomainWithTags(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := opensearch.NewFromConfig(*cfg)
	domainName := "probe-test-os-tags"

	// Create a test domain
	createOut, err := client.CreateDomain(ctx, &opensearch.CreateDomainInput{
		DomainName:    aws.String(domainName),
		EngineVersion: aws.String("OpenSearch_2.5"),
		ClusterConfig: &ostypes.ClusterConfig{
			InstanceType:  ostypes.OpenSearchPartitionInstanceTypeT3SmallSearch,
			InstanceCount: aws.Int32(1),
		},
		EBSOptions: &ostypes.EBSOptions{
			EBSEnabled: aws.Bool(true),
			VolumeSize: aws.Int32(10),
			VolumeType: ostypes.VolumeTypeGp2,
		},
	})
	if err != nil {
		t.Fatalf("failed to create test domain: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDomain(ctx, &opensearch.DeleteDomainInput{
			DomainName: aws.String(domainName),
		})
	})

	// Add tags
	arn := aws.ToString(createOut.DomainStatus.ARN)
	_, err = client.AddTags(ctx, &opensearch.AddTagsInput{
		ARN: aws.String(arn),
		TagList: []ostypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("test")},
			{Key: aws.String("Owner"), Value: aws.String("probe-provider")},
		},
	})
	if err != nil {
		t.Fatalf("failed to tag domain: %v", err)
	}

	// Test the prober
	prober := NewOpenSearchProber(*cfg)
	result, err := prober.Probe(ctx, domainName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true")
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
