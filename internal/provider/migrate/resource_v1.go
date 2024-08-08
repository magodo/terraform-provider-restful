package migrate

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func precheckAttributeV1(pathIsRequired bool) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"mutex": schema.StringAttribute{
					Optional: true,
				},
				"api": schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						"status_locator": schema.StringAttribute{
							Required: true,
						},
						"status": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"success": schema.StringAttribute{
									Required: true,
								},
								"pending": schema.ListAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
							},
						},
						"path": schema.StringAttribute{
							Required: pathIsRequired,
							Optional: !pathIsRequired,
						},
						"query": schema.MapAttribute{
							ElementType: types.ListType{ElemType: types.StringType},
							Optional:    true,
						},
						"header": schema.MapAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"default_delay_sec": schema.Int64Attribute{
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func pollAttributeV1() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"status_locator": schema.StringAttribute{
				Required: true,
			},
			"status": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"success": schema.StringAttribute{
						Required: true,
					},
					"pending": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			"url_locator": schema.StringAttribute{
				Optional: true,
			},
			"header": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"default_delay_sec": schema.Int64Attribute{
				Optional: true,
				Computed: true,
			},
		},
	}
}

func retryAttributeV1() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"status_locator": schema.StringAttribute{
				Required: true,
			},
			"status": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"success": schema.StringAttribute{
						Required: true,
					},
					"pending": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			"count": schema.Int64Attribute{
				Optional: true,
			},
			"wait_in_sec": schema.Int64Attribute{
				Optional: true,
			},
			"max_wait_in_sec": schema.Int64Attribute{
				Optional: true,
			},
		},
	}
}

var ResourceSchemaV1 = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true,
		},
		"path": schema.StringAttribute{
			Required: true,
		},

		"create_selector": schema.StringAttribute{
			Optional: true,
		},
		"read_selector": schema.StringAttribute{
			Optional: true,
		},

		"read_path": schema.StringAttribute{
			Optional: true,
		},
		"update_path": schema.StringAttribute{
			Optional: true,
		},
		"delete_path": schema.StringAttribute{
			Optional: true,
		},

		"body": schema.DynamicAttribute{
			Required: true,
		},

		"poll_create": pollAttributeV1(),
		"poll_update": pollAttributeV1(),
		"poll_delete": pollAttributeV1(),

		"precheck_create": precheckAttributeV1(true),
		"precheck_update": precheckAttributeV1(false),
		"precheck_delete": precheckAttributeV1(false),

		"retry_create": retryAttributeV1(),
		"retry_read":   retryAttributeV1(),
		"retry_update": retryAttributeV1(),
		"retry_delete": retryAttributeV1(),

		"create_method": schema.StringAttribute{
			Optional: true,
		},
		"update_method": schema.StringAttribute{
			Optional: true,
		},
		"delete_method": schema.StringAttribute{
			Optional: true,
		},
		"write_only_attrs": schema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
		"merge_patch_disabled": schema.BoolAttribute{
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
		"check_existance": schema.BoolAttribute{
			Optional: true,
		},
		"force_new_attrs": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
		"output_attrs": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
		"output": schema.DynamicAttribute{
			Computed: true,
		},
	},
}

type ResourceDataV1 struct {
	ID types.String `tfsdk:"id"`

	Path types.String `tfsdk:"path"`

	CreateSelector types.String `tfsdk:"create_selector"`
	ReadSelector   types.String `tfsdk:"read_selector"`

	ReadPath   types.String `tfsdk:"read_path"`
	UpdatePath types.String `tfsdk:"update_path"`
	DeletePath types.String `tfsdk:"delete_path"`

	CreateMethod types.String `tfsdk:"create_method"`
	UpdateMethod types.String `tfsdk:"update_method"`
	DeleteMethod types.String `tfsdk:"delete_method"`

	PrecheckCreate types.List `tfsdk:"precheck_create"`
	PrecheckUpdate types.List `tfsdk:"precheck_update"`
	PrecheckDelete types.List `tfsdk:"precheck_delete"`

	Body types.Dynamic `tfsdk:"body"`

	PollCreate types.Object `tfsdk:"poll_create"`
	PollUpdate types.Object `tfsdk:"poll_update"`
	PollDelete types.Object `tfsdk:"poll_delete"`

	RetryCreate types.Object `tfsdk:"retry_create"`
	RetryRead   types.Object `tfsdk:"retry_read"`
	RetryUpdate types.Object `tfsdk:"retry_update"`
	RetryDelete types.Object `tfsdk:"retry_delete"`

	WriteOnlyAttributes types.List `tfsdk:"write_only_attrs"`
	MergePatchDisabled  types.Bool `tfsdk:"merge_patch_disabled"`
	Query               types.Map  `tfsdk:"query"`
	Header              types.Map  `tfsdk:"header"`

	CheckExistance types.Bool `tfsdk:"check_existance"`
	ForceNewAttrs  types.Set  `tfsdk:"force_new_attrs"`
	OutputAttrs    types.Set  `tfsdk:"output_attrs"`

	Output types.Dynamic `tfsdk:"output"`
}
