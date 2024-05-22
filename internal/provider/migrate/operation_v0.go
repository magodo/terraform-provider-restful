package migrate

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var OperationSchemaV0 = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true,
		},
		"path": schema.StringAttribute{
			Required: true,
		},
		"method": schema.StringAttribute{
			Required: true,
		},
		"body": schema.StringAttribute{
			Optional: true,
		},
		"query": schema.MapAttribute{
			ElementType: types.ListType{ElemType: types.StringType},
			Optional:    true,
		},
		"header": schema.MapAttribute{
			ElementType: types.StringType,
			Optional:    true,
		},

		"precheck": precheckAttributeV0(true),
		"poll":     pollAttributeV0(),
		"retry":    retryAttributeV0(),

		"delete_method": schema.StringAttribute{
			Optional: true,
		},

		"delete_path": schema.StringAttribute{
			Optional: true,
		},

		"delete_body": schema.StringAttribute{
			Optional: true,
		},

		"precheck_delete": precheckAttributeV0(false),
		"poll_delete":     pollAttributeV0(),
		"retry_delete":    retryAttributeV0(),

		"output_attrs": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},

		"output": schema.StringAttribute{
			Computed: true,
		},
	},
}

type OperationDataV0 struct {
	ID             types.String `tfsdk:"id"`
	Path           types.String `tfsdk:"path"`
	Method         types.String `tfsdk:"method"`
	Body           types.String `tfsdk:"body"`
	Query          types.Map    `tfsdk:"query"`
	Header         types.Map    `tfsdk:"header"`
	Precheck       types.List   `tfsdk:"precheck"`
	Poll           types.Object `tfsdk:"poll"`
	Retry          types.Object `tfsdk:"retry"`
	DeleteMethod   types.String `tfsdk:"delete_method"`
	DeleteBody     types.String `tfsdk:"delete_body"`
	DeletePath     types.String `tfsdk:"delete_path"`
	PrecheckDelete types.List   `tfsdk:"precheck_delete"`
	PollDelete     types.Object `tfsdk:"poll_delete"`
	RetryDelete    types.Object `tfsdk:"retry_delete"`
	OutputAttrs    types.Set    `tfsdk:"output_attrs"`
	Output         types.String `tfsdk:"output"`
}
