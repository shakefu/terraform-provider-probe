// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package provider implements the Terraform provider for probing AWS resources.
package provider

import (
	"context"
)

// ProbeResult contains the results of probing an AWS resource.
type ProbeResult struct {
	// Exists indicates whether the resource was found.
	Exists bool

	// Arn is the Amazon Resource Name, if available.
	Arn string

	// Properties contains the resource properties as a map.
	// The structure varies by resource type.
	Properties map[string]any

	// Tags contains the resource tags, if available.
	Tags map[string]string
}

// ResourceProber defines the interface for probing AWS resources.
// Each supported resource type implements this interface.
type ResourceProber interface {
	// Probe checks whether a resource exists and retrieves its properties.
	// The identifier format depends on the resource type (e.g., table name for DynamoDB).
	// Returns a ProbeResult with Exists=false if the resource is not found.
	// Returns an error only for unexpected failures (not for "resource not found").
	Probe(ctx context.Context, identifier string) (*ProbeResult, error)
}
