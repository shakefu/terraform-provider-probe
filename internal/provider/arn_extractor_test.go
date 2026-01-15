// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestExtractArn_CommonFields(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
		expected   string
	}{
		{
			name:       "Arn field",
			properties: map[string]interface{}{"Arn": "arn:aws:iam::123456789012:role/test"},
			expected:   "arn:aws:iam::123456789012:role/test",
		},
		{
			name:       "TableArn field",
			properties: map[string]interface{}{"TableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/test"},
			expected:   "arn:aws:dynamodb:us-east-1:123456789012:table/test",
		},
		{
			name:       "FunctionArn field",
			properties: map[string]interface{}{"FunctionArn": "arn:aws:lambda:us-east-1:123456789012:function:test"},
			expected:   "arn:aws:lambda:us-east-1:123456789012:function:test",
		},
		{
			name:       "RoleArn field",
			properties: map[string]interface{}{"RoleArn": "arn:aws:iam::123456789012:role/test-role"},
			expected:   "arn:aws:iam::123456789012:role/test-role",
		},
		{
			name:       "BucketArn field",
			properties: map[string]interface{}{"BucketArn": "arn:aws:s3:::my-bucket"},
			expected:   "arn:aws:s3:::my-bucket",
		},
		{
			name:       "QueueArn field",
			properties: map[string]interface{}{"QueueArn": "arn:aws:sqs:us-east-1:123456789012:test-queue"},
			expected:   "arn:aws:sqs:us-east-1:123456789012:test-queue",
		},
		{
			name:       "TopicArn field",
			properties: map[string]interface{}{"TopicArn": "arn:aws:sns:us-east-1:123456789012:test-topic"},
			expected:   "arn:aws:sns:us-east-1:123456789012:test-topic",
		},
		{
			name:       "VpcId field (non-ARN identifier)",
			properties: map[string]interface{}{"VpcId": "vpc-12345678"},
			expected:   "vpc-12345678",
		},
		{
			name:       "ApiId field",
			properties: map[string]interface{}{"ApiId": "abc123def4"},
			expected:   "abc123def4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractArn(tt.properties)
			if result != tt.expected {
				t.Errorf("extractArn() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractArn_NilAndEmpty(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
	}{
		{
			name:       "Nil properties",
			properties: nil,
		},
		{
			name:       "Empty properties",
			properties: map[string]interface{}{},
		},
		{
			name:       "Empty string values",
			properties: map[string]interface{}{"Arn": "", "TableArn": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractArn(tt.properties)
			if result != "" {
				t.Errorf("extractArn() = %q, want empty string", result)
			}
		})
	}
}

func TestExtractArn_NoArnPresent(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]interface{}
	}{
		{
			name:       "Unrelated fields only",
			properties: map[string]interface{}{"Name": "test", "Description": "a resource"},
		},
		{
			name:       "Similar but wrong field names",
			properties: map[string]interface{}{"arn": "lowercase-arn", "ARN": "uppercase-arn"},
		},
		{
			name:       "Non-string ARN values",
			properties: map[string]interface{}{"Arn": 12345, "TableArn": true},
		},
		{
			name:       "Nested ARN (not supported)",
			properties: map[string]interface{}{"nested": map[string]interface{}{"Arn": "arn:aws:s3:::bucket"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractArn(tt.properties)
			if result != "" {
				t.Errorf("extractArn() = %q, want empty string", result)
			}
		})
	}
}

func TestExtractArn_Priority(t *testing.T) {
	// When multiple ARN fields are present, the first one in arnFields order wins
	properties := map[string]interface{}{
		"TableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/test",
		"Arn":      "arn:aws:generic::123456789012:resource",
	}

	result := extractArn(properties)
	// "Arn" comes first in arnFields, so it should be returned
	expected := "arn:aws:generic::123456789012:resource"
	if result != expected {
		t.Errorf("extractArn() = %q, want %q (Arn should have priority)", result, expected)
	}
}
