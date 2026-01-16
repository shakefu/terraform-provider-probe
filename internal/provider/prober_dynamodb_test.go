// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// getLocalStackConfig returns an AWS config for LocalStack testing.
// Returns nil if LocalStack is not available.
func getLocalStackConfig(t *testing.T) *aws.Config {
	t.Helper()

	detected, endpoint := detectLocalStack()
	if !detected {
		return nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	cfg.BaseEndpoint = aws.String(endpoint)
	cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")

	return &cfg
}

func TestDynamoDBProber_TableNotFound(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	prober := NewDynamoDBProber(*cfg)
	result, err := prober.Probe(context.Background(), "nonexistent-table-12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exists {
		t.Error("expected Exists to be false for nonexistent table")
	}
}

func TestDynamoDBProber_TableExists(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := dynamodb.NewFromConfig(*cfg)
	tableName := "probe-test-dynamodb-exists"

	// Create a test table
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		_, _ = client.DeleteTable(ctx, &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 30_000_000_000); err != nil { // 30 seconds
		t.Fatalf("table did not become active: %v", err)
	}

	// Test the prober
	prober := NewDynamoDBProber(*cfg)
	result, err := prober.Probe(ctx, tableName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing table")
	}

	if result.Arn == "" {
		t.Error("expected ARN to be populated")
	}

	if result.Properties["TableName"] != tableName {
		t.Errorf("expected TableName=%q, got %q", tableName, result.Properties["TableName"])
	}
}

func TestDynamoDBProber_TableWithTags(t *testing.T) {
	cfg := getLocalStackConfig(t)
	if cfg == nil {
		t.Skip("LocalStack not available")
	}

	ctx := context.Background()
	client := dynamodb.NewFromConfig(*cfg)
	tableName := "probe-test-dynamodb-tags"

	// Create a test table with tags
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("test")},
			{Key: aws.String("Owner"), Value: aws.String("probe-provider")},
		},
	})
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		_, _ = client.DeleteTable(ctx, &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 30_000_000_000); err != nil { // 30 seconds
		t.Fatalf("table did not become active: %v", err)
	}

	// Test the prober
	prober := NewDynamoDBProber(*cfg)
	result, err := prober.Probe(ctx, tableName)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exists {
		t.Error("expected Exists to be true for existing table")
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
