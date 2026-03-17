// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

func TestIsEC2NotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "InvalidVpcID.NotFound API error",
			err: &smithy.GenericAPIError{
				Code:    "InvalidVpcID.NotFound",
				Message: "The vpc ID 'vpc-12345' does not exist",
			},
			expected: true,
		},
		{
			name: "different API error code",
			err: &smithy.GenericAPIError{
				Code:    "UnauthorizedAccess",
				Message: "You are not authorized",
			},
			expected: false,
		},
		{
			name:     "non-API error",
			err:      errors.New("connection timeout"),
			expected: false,
		},
		{
			name:     "wrapped API error",
			err:      fmt.Errorf("operation failed: %w", &smithy.GenericAPIError{Code: "InvalidVpcID.NotFound", Message: "not found"}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEC2NotFound(tt.err)
			if result != tt.expected {
				t.Errorf("isEC2NotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestEC2Prober_VPCNotFound_ByID(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	prober := NewEC2Prober(*cfg)
	result, err := prober.Probe(context.Background(), "vpc-00000000000000000")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exists {
		t.Error("expected Exists to be false for nonexistent VPC ID")
	}
}

func TestEC2Prober_VPCNotFound_ByName(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	prober := NewEC2Prober(*cfg)
	result, err := prober.Probe(context.Background(), "nonexistent-vpc-name-12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exists {
		t.Error("expected Exists to be false for nonexistent VPC name")
	}
}

func TestEC2Prober_VPCExists_ByName(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := ec2.NewFromConfig(*cfg)
	vpcName := "probe-test-vpc-by-name"

	// Create a test VPC
	createOut, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.99.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(vpcName)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test VPC: %v", err)
	}
	vpcID := aws.ToString(createOut.Vpc.VpcId)

	t.Cleanup(func() {
		_, _ = client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Test the prober by name
	prober := NewEC2Prober(*cfg)
	result, err := prober.Probe(ctx, vpcName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing VPC")
	}

	if result.Arn == "" {
		t.Error("expected ARN to be populated")
	}

	if !strings.Contains(result.Arn, vpcID) {
		t.Errorf("expected ARN to contain VPC ID %q, got %q", vpcID, result.Arn)
	}

	if result.Properties["VpcId"] != vpcID {
		t.Errorf("expected VpcId=%q, got %q", vpcID, result.Properties["VpcId"])
	}

	if result.Properties["CidrBlock"] != "10.99.0.0/16" {
		t.Errorf("expected CidrBlock=10.99.0.0/16, got %q", result.Properties["CidrBlock"])
	}
}

func TestEC2Prober_VPCExists_ByID(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := ec2.NewFromConfig(*cfg)

	// Create a test VPC
	createOut, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.98.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("probe-test-vpc-by-id")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test VPC: %v", err)
	}
	vpcID := aws.ToString(createOut.Vpc.VpcId)

	t.Cleanup(func() {
		_, _ = client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Test the prober by ID
	prober := NewEC2Prober(*cfg)
	result, err := prober.Probe(ctx, vpcID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing VPC")
	}

	if result.Properties["VpcId"] != vpcID {
		t.Errorf("expected VpcId=%q, got %q", vpcID, result.Properties["VpcId"])
	}
}

func TestEC2Prober_VPCMultipleMatches_ByName(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := ec2.NewFromConfig(*cfg)
	vpcName := "probe-test-vpc-duplicate"

	// Create two VPCs with the same Name tag
	var vpcIDs []string
	for i := 0; i < 2; i++ {
		createOut, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
			CidrBlock: aws.String("10.97.0.0/16"),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeVpc,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String(vpcName)},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to create test VPC %d: %v", i, err)
		}
		vpcIDs = append(vpcIDs, aws.ToString(createOut.Vpc.VpcId))
	}

	t.Cleanup(func() {
		for _, id := range vpcIDs {
			_, _ = client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
				VpcId: aws.String(id),
			})
		}
	})

	// Test the prober - should return error for multiple matches
	prober := NewEC2Prober(*cfg)
	_, err := prober.Probe(ctx, vpcName)

	if err == nil {
		t.Fatal("expected error for multiple VPCs with same name")
	}

	if !strings.Contains(err.Error(), "multiple VPCs found") {
		t.Errorf("expected error to contain 'multiple VPCs found', got: %v", err)
	}
}

func TestEC2Prober_VPCWithTags(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := ec2.NewFromConfig(*cfg)
	vpcName := "probe-test-vpc-tags"

	// Create a VPC with tags
	createOut, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.96.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(vpcName)},
					{Key: aws.String("Environment"), Value: aws.String("test")},
					{Key: aws.String("Owner"), Value: aws.String("probe-provider")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test VPC: %v", err)
	}
	vpcID := aws.ToString(createOut.Vpc.VpcId)

	t.Cleanup(func() {
		_, _ = client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Test the prober
	prober := NewEC2Prober(*cfg)
	result, err := prober.Probe(ctx, vpcName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true")
	}

	// Check tags (Name tag should be included)
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
