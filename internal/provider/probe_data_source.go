// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ProbeDataSource satisfies various datasource interfaces.
var _ datasource.DataSource = &ProbeDataSource{}
var _ datasource.DataSourceWithConfigure = &ProbeDataSource{}

// ProbeDataSource defines the data source implementation.
type ProbeDataSource struct {
	cfg      aws.Config
	registry *ProberRegistry
}

// ProbeDataSourceModel describes the data source data model.
type ProbeDataSourceModel struct {
	Type       types.String  `tfsdk:"type"`
	ID         types.String  `tfsdk:"id"`
	Exists     types.Bool    `tfsdk:"exists"`
	Arn        types.String  `tfsdk:"arn"`
	Properties types.Dynamic `tfsdk:"properties"`
}

func NewProbeDataSource() datasource.DataSource {
	return &ProbeDataSource{}
}

func (d *ProbeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (d *ProbeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Checks whether an AWS resource exists without failing when it doesn't.",

		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Description: "Resource type (e.g., aws_dynamodb_table or AWS::DynamoDB::Table).",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "Resource identifier (table name, bucket name, etc.).",
				Required:    true,
			},
			"exists": schema.BoolAttribute{
				Description: "Whether the resource exists.",
				Computed:    true,
			},
			"arn": schema.StringAttribute{
				Description: "Resource ARN (null if resource does not exist).",
				Computed:    true,
			},
			"properties": schema.DynamicAttribute{
				Description: "Resource properties as a map (null if resource does not exist).",
				Computed:    true,
			},
		},
	}
}

func (d *ProbeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	cfg, ok := req.ProviderData.(aws.Config)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected aws.Config, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.cfg = cfg
	d.registry = NewProberRegistry(cfg)
}

func (d *ProbeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProbeDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := data.Type.ValueString()
	identifier := data.ID.ValueString()

	// Get the appropriate prober for this resource type
	prober, err := d.registry.GetProber(resourceType)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unsupported Resource Type",
			fmt.Sprintf("Resource type %q is not supported. Supported types: %v", resourceType, d.registry.SupportedTypes()),
		)
		return
	}

	// Probe the resource
	result, err := prober.Probe(ctx, identifier)
	if err != nil {
		resp.Diagnostics.AddError("Probe failed", err.Error())
		return
	}

	if !result.Exists {
		// Resource not found - NOT AN ERROR
		data.Exists = types.BoolValue(false)
		data.Arn = types.StringNull()
		data.Properties = types.DynamicNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Convert properties to Terraform dynamic type
	props, propsDiags := convertMapToDynamic(result.Properties)
	resp.Diagnostics.Append(propsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Exists = types.BoolValue(true)
	data.Properties = props
	if result.Arn != "" {
		data.Arn = types.StringValue(result.Arn)
	} else {
		data.Arn = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// convertMapToDynamic converts a map[string]any to a Terraform dynamic value.
func convertMapToDynamic(props map[string]any) (types.Dynamic, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(props) == 0 {
		return types.DynamicNull(), diags
	}

	val, err := convertToAttrValue(props)
	if err != nil {
		diags.AddError("Failed to convert properties", err.Error())
		return types.DynamicNull(), diags
	}

	return types.DynamicValue(val), diags
}

// convertToAttrValue recursively converts Go values to Terraform attr.Value.
func convertToAttrValue(v any) (attr.Value, error) {
	if v == nil {
		return types.StringNull(), nil
	}

	switch val := v.(type) {
	case string:
		return types.StringValue(val), nil
	case float64:
		// JSON numbers are always float64
		return types.Float64Value(val), nil
	case int:
		return types.Int64Value(int64(val)), nil
	case int64:
		return types.Int64Value(val), nil
	case bool:
		return types.BoolValue(val), nil
	case []any:
		if len(val) == 0 {
			return types.ListNull(types.StringType), nil
		}
		elements := make([]attr.Value, len(val))
		for i, elem := range val {
			converted, err := convertToAttrValue(elem)
			if err != nil {
				return nil, err
			}
			elements[i] = converted
		}
		// Determine element type from first element
		elemType := elements[0].Type(context.Background())
		return types.ListValueMust(elemType, elements), nil
	case map[string]any:
		if len(val) == 0 {
			return types.MapNull(types.StringType), nil
		}
		elements := make(map[string]attr.Value)
		for k, elem := range val {
			converted, err := convertToAttrValue(elem)
			if err != nil {
				return nil, err
			}
			elements[k] = converted
		}
		// Use object type for mixed-type maps
		attrTypes := make(map[string]attr.Type)
		for k, v := range elements {
			attrTypes[k] = v.Type(context.Background())
		}
		return types.ObjectValueMust(attrTypes, elements), nil
	case map[string]string:
		// Handle map[string]string (e.g., Tags)
		if len(val) == 0 {
			return types.MapNull(types.StringType), nil
		}
		elements := make(map[string]attr.Value)
		for k, v := range val {
			elements[k] = types.StringValue(v)
		}
		return types.MapValueMust(types.StringType, elements), nil
	default:
		// Fallback to string representation
		return types.StringValue(fmt.Sprintf("%v", val)), nil
	}
}
