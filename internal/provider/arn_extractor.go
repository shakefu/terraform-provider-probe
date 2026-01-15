// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

// arnFields lists the property names that may contain ARN values.
// Cloud Control API returns properties with varying ARN field names depending on the resource type.
var arnFields = []string{
	"Arn",
	"TableArn",
	"FunctionArn",
	"RoleArn",
	"BucketArn",
	"QueueArn",
	"TopicArn",
	"ClusterArn",
	"ServiceArn",
	"TaskDefinitionArn",
	"SecretArn",
	"StateMachineArn",
	"StreamArn",
	"LogGroupArn",
	"DBInstanceArn",
	"DBClusterArn",
	"CacheClusterArn",
	"VpcId", // Not an ARN but useful identifier
	"ApiId", // API Gateway
}

// extractArn attempts to extract an ARN from the resource properties.
// It checks common ARN field names and returns the first non-empty value found.
func extractArn(properties map[string]interface{}) string {
	if properties == nil {
		return ""
	}
	for _, field := range arnFields {
		if arn, ok := properties[field].(string); ok && arn != "" {
			return arn
		}
	}
	return ""
}
