// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// EC2Prober probes EC2/VPC resources using the native AWS SDK.
type EC2Prober struct {
	client *ec2.Client
	region string
}

// NewEC2Prober creates a new EC2 prober from an AWS config.
func NewEC2Prober(cfg aws.Config) *EC2Prober {
	return &EC2Prober{
		client: ec2.NewFromConfig(cfg),
		region: cfg.Region,
	}
}

// Probe checks whether a VPC exists and retrieves its properties.
// The identifier can be a VPC ID (vpc-xxx) or a Name tag value.
// When looking up by Name tag, returns an error if multiple VPCs match.
func (p *EC2Prober) Probe(ctx context.Context, identifier string) (*ProbeResult, error) {
	input := &ec2.DescribeVpcsInput{}

	if strings.HasPrefix(identifier, "vpc-") {
		input.VpcIds = []string{identifier}
	} else {
		input.Filters = []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{identifier},
			},
		}
	}

	output, err := p.client.DescribeVpcs(ctx, input)
	if err != nil {
		// DescribeVpcs with a specific VPC ID returns an error if not found
		if isEC2NotFound(err) {
			return &ProbeResult{Exists: false}, nil
		}
		return nil, err
	}

	if len(output.Vpcs) == 0 {
		return &ProbeResult{Exists: false}, nil
	}

	if len(output.Vpcs) > 1 {
		return nil, fmt.Errorf("multiple VPCs found with name %q (found %d)", identifier, len(output.Vpcs))
	}

	vpc := output.Vpcs[0]
	vpcID := aws.ToString(vpc.VpcId)
	ownerID := aws.ToString(vpc.OwnerId)
	arn := fmt.Sprintf("arn:aws:ec2:%s:%s:vpc/%s", p.region, ownerID, vpcID)

	result := &ProbeResult{
		Exists: true,
		Arn:    arn,
		Properties: map[string]any{
			"VpcId":     vpcID,
			"Arn":       arn,
			"CidrBlock": aws.ToString(vpc.CidrBlock),
			"State":     string(vpc.State),
			"IsDefault": vpc.IsDefault,
			"OwnerId":   aws.ToString(vpc.OwnerId),
		},
	}

	// Extract tags
	if len(vpc.Tags) > 0 {
		result.Tags = make(map[string]string, len(vpc.Tags))
		for _, tag := range vpc.Tags {
			result.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
		}
		result.Properties["Tags"] = result.Tags
	}

	return result, nil
}

// isEC2NotFound checks for EC2 "not found" error responses using typed errors.
func isEC2NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "InvalidVpcID.NotFound"
	}
	return false
}
