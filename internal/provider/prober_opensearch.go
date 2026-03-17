// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

// OpenSearchProber probes OpenSearch domains using the native AWS SDK.
type OpenSearchProber struct {
	client *opensearch.Client
}

// NewOpenSearchProber creates a new OpenSearch prober from an AWS config.
func NewOpenSearchProber(cfg aws.Config) *OpenSearchProber {
	return &OpenSearchProber{
		client: opensearch.NewFromConfig(cfg),
	}
}

// Probe checks whether an OpenSearch domain exists and retrieves its properties.
// The identifier is the domain name.
func (p *OpenSearchProber) Probe(ctx context.Context, identifier string) (*ProbeResult, error) {
	output, err := p.client.DescribeDomain(ctx, &opensearch.DescribeDomainInput{
		DomainName: aws.String(identifier),
	})

	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return &ProbeResult{Exists: false}, nil
		}
		return nil, err
	}

	domain := output.DomainStatus
	arn := aws.ToString(domain.ARN)

	result := &ProbeResult{
		Exists: true,
		Arn:    arn,
		Properties: map[string]any{
			"DomainName":    aws.ToString(domain.DomainName),
			"DomainId":      aws.ToString(domain.DomainId),
			"Arn":           arn,
			"EngineVersion": aws.ToString(domain.EngineVersion),
			"Endpoint":      aws.ToString(domain.Endpoint),
			"Created":       aws.ToBool(domain.Created),
			"Deleted":       aws.ToBool(domain.Deleted),
			"Processing":    aws.ToBool(domain.Processing),
		},
	}

	// Add cluster config info if available
	if domain.ClusterConfig != nil {
		result.Properties["InstanceType"] = string(domain.ClusterConfig.InstanceType)
		result.Properties["InstanceCount"] = aws.ToInt32(domain.ClusterConfig.InstanceCount)
	}

	// Add encryption info
	if domain.EncryptionAtRestOptions != nil {
		result.Properties["EncryptionAtRest"] = aws.ToBool(domain.EncryptionAtRestOptions.Enabled)
	}

	// Get tags
	if arn != "" {
		tags, err := p.client.ListTags(ctx, &opensearch.ListTagsInput{
			ARN: aws.String(arn),
		})
		if err == nil && len(tags.TagList) > 0 {
			result.Tags = make(map[string]string, len(tags.TagList))
			for _, tag := range tags.TagList {
				result.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}
			result.Properties["Tags"] = result.Tags
		}
	}

	return result, nil
}
