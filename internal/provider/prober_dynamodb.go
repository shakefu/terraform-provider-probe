// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBProber probes DynamoDB tables using the native AWS SDK.
type DynamoDBProber struct {
	client *dynamodb.Client
}

// NewDynamoDBProber creates a new DynamoDB prober from an AWS config.
func NewDynamoDBProber(cfg aws.Config) *DynamoDBProber {
	return &DynamoDBProber{
		client: dynamodb.NewFromConfig(cfg),
	}
}

// Probe checks whether a DynamoDB table exists and retrieves its properties.
// The identifier is the table name.
func (p *DynamoDBProber) Probe(ctx context.Context, identifier string) (*ProbeResult, error) {
	// DescribeTable returns the table description or ResourceNotFoundException
	output, err := p.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(identifier),
	})

	if err != nil {
		// Check if the table does not exist
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return &ProbeResult{Exists: false}, nil
		}
		// Other errors are unexpected
		return nil, err
	}

	table := output.Table
	result := &ProbeResult{
		Exists: true,
		Arn:    aws.ToString(table.TableArn),
		Properties: map[string]any{
			"TableName":             aws.ToString(table.TableName),
			"TableArn":              aws.ToString(table.TableArn),
			"TableStatus":           string(table.TableStatus),
			"TableId":               aws.ToString(table.TableId),
			"CreationDateTime":      table.CreationDateTime,
			"ItemCount":             table.ItemCount,
			"TableSizeBytes":        table.TableSizeBytes,
			"DeletionProtection":    aws.ToBool(table.DeletionProtectionEnabled),
			"BillingModeSummary":    table.BillingModeSummary,
			"ProvisionedThroughput": table.ProvisionedThroughput,
			"KeySchema":             table.KeySchema,
			"AttributeDefinitions":  table.AttributeDefinitions,
		},
	}

	// Get tags using the ARN
	if result.Arn != "" {
		tags, err := p.client.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
			ResourceArn: aws.String(result.Arn),
		})
		if err == nil && len(tags.Tags) > 0 {
			result.Tags = make(map[string]string, len(tags.Tags))
			for _, tag := range tags.Tags {
				result.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}
			// Also add tags to properties for visibility
			result.Properties["Tags"] = result.Tags
		}
	}

	return result, nil
}
