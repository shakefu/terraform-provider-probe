// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// normalizedTypes maps both Terraform and Cloud Control type names to a canonical key.
// The canonical key is the Terraform-style name (e.g., "aws_dynamodb_table").
var normalizedTypes = map[string]string{
	// DynamoDB
	"aws_dynamodb_table":         "aws_dynamodb_table",
	"AWS::DynamoDB::Table":       "aws_dynamodb_table",
	"dynamodb_table":             "aws_dynamodb_table", // short form
	"AWS::DynamoDB::GlobalTable": "aws_dynamodb_table", // maps to same prober

	// S3 (placeholder for future implementation)
	"aws_s3_bucket":   "aws_s3_bucket",
	"AWS::S3::Bucket": "aws_s3_bucket",
	"s3_bucket":       "aws_s3_bucket",
}

// ProberFactory is a function that creates a ResourceProber from an AWS config.
type ProberFactory func(cfg aws.Config) ResourceProber

// proberFactories maps canonical type names to prober factory functions.
var proberFactories = map[string]ProberFactory{
	"aws_dynamodb_table": func(cfg aws.Config) ResourceProber {
		return NewDynamoDBProber(cfg)
	},
	"aws_s3_bucket": func(cfg aws.Config) ResourceProber {
		return NewS3Prober(cfg)
	},
}

// ProberRegistry manages ResourceProber instances for different resource types.
type ProberRegistry struct {
	cfg     aws.Config
	probers map[string]ResourceProber
}

// NewProberRegistry creates a new ProberRegistry with the given AWS config.
func NewProberRegistry(cfg aws.Config) *ProberRegistry {
	return &ProberRegistry{
		cfg:     cfg,
		probers: make(map[string]ResourceProber),
	}
}

// GetProber returns a ResourceProber for the given resource type.
// The type can be either Terraform-style (aws_dynamodb_table) or
// Cloud Control-style (AWS::DynamoDB::Table).
// Returns an error if the type is not supported.
func (r *ProberRegistry) GetProber(resourceType string) (ResourceProber, error) {
	// Normalize the type name
	canonicalType := normalizeTypeName(resourceType)

	// Check if we already have an instance
	if prober, ok := r.probers[canonicalType]; ok {
		return prober, nil
	}

	// Get the factory for this type
	factory, ok := proberFactories[canonicalType]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	// Create and cache the prober
	prober := factory(r.cfg)
	r.probers[canonicalType] = prober

	return prober, nil
}

// SupportedTypes returns a list of all supported resource types.
func (r *ProberRegistry) SupportedTypes() []string {
	seen := make(map[string]bool)
	var types []string

	for _, canonical := range normalizedTypes {
		if !seen[canonical] {
			seen[canonical] = true
			if _, ok := proberFactories[canonical]; ok {
				types = append(types, canonical)
			}
		}
	}

	return types
}

// normalizeTypeName converts any recognized type format to the canonical Terraform-style name.
func normalizeTypeName(typeName string) string {
	// Check direct mapping
	if canonical, ok := normalizedTypes[typeName]; ok {
		return canonical
	}

	// If it's already in aws_* format, return as-is (might be unsupported)
	if strings.HasPrefix(typeName, "aws_") {
		return typeName
	}

	// If it's in AWS::Service::Resource format, try to convert
	if strings.HasPrefix(typeName, "AWS::") {
		parts := strings.Split(typeName, "::")
		if len(parts) == 3 {
			// Convert AWS::Service::Resource to aws_service_resource (lowercase)
			service := strings.ToLower(parts[1])
			resource := strings.ToLower(parts[2])
			return "aws_" + service + "_" + resource
		}
	}

	// Return as-is for unknown types
	return typeName
}
