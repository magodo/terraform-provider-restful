package provider

import (
		"github.com/hashicorp/terraform-plugin-framework/diag"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/dynamicvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/magodo/terraform-plugin-framework-helper/ephemeral"
	"github.com/magodo/terraform-plugin-framework-helper/jsonset"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
	myvalidator "github.com/lfventura/terraform-provider-restful/internal/validator"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Resource struct {
	p *Provider
}

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithUpgradeState = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}

type resourceIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

type resourceData struct {
	ID types.String `tfsdk:"id"`

	Path    types.String `tfsdk:"path"`
	BaseURL types.String `tfsdk:"base_url"`

	CreateSelector       types.String `tfsdk:"create_selector"`
	ReadSelector         types.String `tfsdk:"read_selector"`
	ReadResponseTemplate types.String `tfsdk:"read_response_template"`

	ReadPath   types.String `tfsdk:"read_path"`
	UpdatePath types.String `tfsdk:"update_path"`
	DeletePath types.String `tfsdk:"delete_path"`

	CreateMethod types.String `tfsdk:"create_method"`
	UpdateMethod types.String `tfsdk:"update_method"`
	DeleteMethod types.String `tfsdk:"delete_method"`

	PrecheckCreate types.List `tfsdk:"precheck_create"`
	PrecheckUpdate types.List `tfsdk:"precheck_update"`
	PrecheckDelete types.List `tfsdk:"precheck_delete"`

	Body          types.Dynamic `tfsdk:"body"`
	EphemeralBody types.Dynamic `tfsdk:"ephemeral_body"`

	DeleteBody        types.Dynamic `tfsdk:"delete_body"`
	DeleteBodyRaw     types.String  `tfsdk:"delete_body_raw"`
	UpdateBodyPatches types.List    `tfsdk:"update_body_patches"`

	PollCreate types.Object `tfsdk:"poll_create"`
	PollUpdate types.Object `tfsdk:"poll_update"`
	PollDelete types.Object `tfsdk:"poll_delete"`

	PostCreateRead types.Object `tfsdk:"post_create_read"`

	WriteOnlyAttributes types.List `tfsdk:"write_only_attrs"`
	MergePatchDisabled  types.Bool `tfsdk:"merge_patch_disabled"`

	Query       types.Map `tfsdk:"query"`
	CreateQuery types.Map `tfsdk:"create_query"`
	ReadQuery   types.Map `tfsdk:"read_query"`
	UpdateQuery types.Map `tfsdk:"update_query"`
	DeleteQuery types.Map `tfsdk:"delete_query"`

	Header       types.Map `tfsdk:"header"`
	CreateHeader types.Map `tfsdk:"create_header"`
	ReadHeader   types.Map `tfsdk:"read_header"`
	UpdateHeader types.Map `tfsdk:"update_header"`
	DeleteHeader types.Map `tfsdk:"delete_header"`

	CheckExistance types.Bool `tfsdk:"check_existance"`
	ForceNewAttrs  types.Set  `tfsdk:"force_new_attrs"`
	OutputAttrs    types.Set  `tfsdk:"output_attrs"`

	UseSensitiveOutput types.Bool    `tfsdk:"use_sensitive_output"`
	Output             types.Dynamic `tfsdk:"output"`
	SensitiveOutput    types.Dynamic `tfsdk:"sensitive_output"`
	// Novo campo para permitir security no resource
	Security           types.Object  `tfsdk:"security"`
}

type bodyPatchData struct {
	Path    types.String `tfsdk:"path"`
	RawJSON types.String `tfsdk:"raw_json"`
	Removed types.Bool   `tfsdk:"removed"`
}

type pollData struct {
	StatusLocator types.String `tfsdk:"status_locator"`
	Status        types.Object `tfsdk:"status"`
	UrlLocator    types.String `tfsdk:"url_locator"`
	Header        types.Map    `tfsdk:"header"`
	DefaultDelay  types.Int64  `tfsdk:"default_delay_sec"`
}

type precheckData struct {
	Api   types.Object `tfsdk:"api"`
	Mutex types.String `tfsdk:"mutex"`
}

type precheckDataApi struct {
	StatusLocator types.String `tfsdk:"status_locator"`
	Status        types.Object `tfsdk:"status"`
	Path          types.String `tfsdk:"path"`
	Query         types.Map    `tfsdk:"query"`
	Header        types.Map    `tfsdk:"header"`
	DefaultDelay  types.Int64  `tfsdk:"default_delay_sec"`
}

type statusDataGo struct {
	Success string   `tfsdk:"success"`
	Pending []string `tfsdk:"pending"`
}

type postCreateRead struct {
	Path     types.String `tfsdk:"path"`
	Query    types.Map    `tfsdk:"query"`
	Header   types.Map    `tfsdk:"header"`
	Selector types.String `tfsdk:"selector"`
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
	resp.ResourceBehavior = resource.ResourceBehavior{
		// The identity contains the "body" field, which can be changed during update
		MutableIdentity: true,
	}
}

const paramFuncDescription = "Supported functions include: `escape` (URL path escape, by default applied), `unescape` (URL path unescape), `query_escape` (URL query escape), `query_unescape` (URL query unescape), `base` (filepath base), `url_path` (path segment of a URL), `trim_path` (trim `path`)."

const bodyParamDescription = " can be a string literal, or combined by the body param: `$(body.x.y.z)` that expands to the `x.y.z` property of the API body. It can add a chain of functions (applied from left to right), in the form of `$f1.f2(body)`. " + paramFuncDescription

const bodyOrPathParamDescription = "This can be a string literal, or combined by following params: path param: `$(path)` expanded to `path`, body param: `$(body.x.y.z)` expands to the `x.y.z` property of the API body. Especially for the body param, it can add a chain of functions (applied from left to right), in the form of `$f1.f2(body)`. " + paramFuncDescription

func operationOverridableAttrDescription(attr string, opkind string) string {
	return fmt.Sprintf("The %[1]s parameters that are applied to each %[2]s request. This overrides the `%[1]s` set in the resource block.", attr, opkind)
}

func precheckAttribute(s string, pathIsRequired bool, suffixDesc string, statusLocatorSupportParam bool) schema.ListNestedAttribute {
	pathDesc := "The path used to query readiness, relative to the `base_url` of the provider."
	if suffixDesc != "" {
		pathDesc += " " + suffixDesc
	}

	var statusLocatorSuffixDesc string
	if statusLocatorSupportParam {
		statusLocatorSuffixDesc = " The `path` can contain `$(body.x.y.z)` parameter that reference property from the `state.output`."
	}

	return schema.ListNestedAttribute{
		Description:         fmt.Sprintf("An array of prechecks that need to pass prior to the %q operation. Exactly one of `mutex` or `api` should be specified.", s),
		MarkdownDescription: fmt.Sprintf("An array of prechecks that need to pass prior to the %q operation. Exactly one of `mutex` or `api` should be specified.", s),
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"mutex": schema.StringAttribute{
					Description:         "The name of the mutex, which implies the resource will keep waiting until this mutex is held",
					MarkdownDescription: "The name of the mutex, which implies the resource will keep waiting until this mutex is held",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("api"),
						),
					},
				},
				"api": schema.SingleNestedAttribute{
					Description:         "Keeps waiting until the specified API meets the success status",
					MarkdownDescription: "Keeps waiting until the specified API meets the success status",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"status_locator": schema.StringAttribute{
							Description:         "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the gjson syntax." + statusLocatorSuffixDesc,
							MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)." + statusLocatorSuffixDesc,
							Required:            true,
							Validators: []validator.String{
								myvalidator.StringIsParsable("status_locator", func(s string) error {
									return validateLocator(s)
								}),
							},
						},
						"status": schema.SingleNestedAttribute{
							Description:         "The expected status sentinels for each polling state.",
							MarkdownDescription: "The expected status sentinels for each polling state.",
							Required:            true,
							Attributes: map[string]schema.Attribute{
								"success": schema.StringAttribute{
									Description:         "The expected status sentinel for suceess status.",
									MarkdownDescription: "The expected status sentinel for suceess status.",
									Required:            true,
								},
								"pending": schema.ListAttribute{
									Description:         "The expected status sentinels for pending status.",
									MarkdownDescription: "The expected status sentinels for pending status.",
									Optional:            true,
									ElementType:         types.StringType,
								},
							},
						},
						"path": schema.StringAttribute{
							Description:         pathDesc,
							MarkdownDescription: pathDesc,
							Required:            pathIsRequired,
							Optional:            !pathIsRequired,
						},
						"query": schema.MapAttribute{
							Description:         "The query parameters. This overrides the `query` set in the resource block.",
							MarkdownDescription: "The query parameters. This overrides the `query` set in the resource block.",
							ElementType:         types.ListType{ElemType: types.StringType},
							Optional:            true,
						},
						"header": schema.MapAttribute{
							Description:         "The header parameters. This overrides the `header` set in the resource block.",
							MarkdownDescription: "The header parameters. This overrides the `header` set in the resource block.",
							ElementType:         types.StringType,
							Optional:            true,
						},
						"default_delay_sec": schema.Int64Attribute{
							Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `10`.",
							MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `10`.",
							Optional:            true,
							Computed:            true,
							Default:             int64default.StaticInt64(10),
						},
					},
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("mutex"),
						),
					},
				},
			},
		},
	}
}

func pollAttribute(s string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description:         fmt.Sprintf("The polling option for the %q operation", s),
		MarkdownDescription: fmt.Sprintf("The polling option for the %q operation", s),
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"status_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the gjson syntax. The `path` can contain `$(body.x.y.z)` parameter that reference property from either the response body (for `Create`, after selector), or `state.output` (for `Read`/`Update`/`Delete`).",
				MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md). The `path` can contain `$(body.x.y.z)` parameter that reference property from either the response body (for `Create`, after selector), or `state.output` (for `Read`/`Update`/`Delete`).",
				Required:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("status_locator", func(s string) error {
						return validateLocator(s)
					}),
				},
			},
			"status": schema.SingleNestedAttribute{
				Description:         "The expected status sentinels for each polling state.",
				MarkdownDescription: "The expected status sentinels for each polling state.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"success": schema.StringAttribute{
						Description:         "The expected status sentinel for suceess status.",
						MarkdownDescription: "The expected status sentinel for suceess status.",
						Required:            true,
					},
					"pending": schema.ListAttribute{
						Description:         "The expected status sentinels for pending status.",
						MarkdownDescription: "The expected status sentinels for pending status.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"url_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the polling url. The format can be one of `header.path` (use the property at `path` in response header), `body.path` (use the property at `path` in response body) or `exact.value` (use the exact `value`). When absent, the current operation's URL is used for polling, execpt `Create` where it fallbacks to use the path constructed by the `read_path` as the polling URL.",
				MarkdownDescription: "Specifies how to discover the polling url. The format can be one of `header.path` (use the property at `path` in response header), `body.path` (use the property at `path` in response body) or `exact.value` (use the exact `value`). When absent, the current operation's URL is used for polling, execpt `Create` where it fallbacks to use the path constructed by the `read_path` as the polling URL.",
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("url_locator", func(s string) error {
						return validateLocator(s)
					}),
				},
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters. This overrides the `header` set in the resource block.",
				MarkdownDescription: "The header parameters. This overrides the `header` set in the resource block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"default_delay_sec": schema.Int64Attribute{
				Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `10`.",
				MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `10`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(10),
			},
		},
	}
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	       resp.Schema = schema.Schema{
		       Description:         "`restful_resource` manages a restful resource.",
		       MarkdownDescription: "`restful_resource` manages a restful resource.",
		       Version:             2,
		       Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The ID of the Resource.",
				MarkdownDescription: "The ID of the Resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"base_url": schema.StringAttribute{
				Description:         "The base URL of the API for this resource. If defined, overrides the provider's base_url.",
				MarkdownDescription: "The base URL of the API for this resource. If defined, overrides the provider's base_url.",
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("HTTP url", func(s string) error {
						_, err := url.Parse(s)
						return err
					}),
				},
			},
			"path": schema.StringAttribute{
				Description:         "The path used to create the resource, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path used to create the resource, relative to the `base_url` of the provider.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"create_selector": schema.StringAttribute{
				Description:         "A selector in gjson query syntax, that is used when create returns a collection of resources, to select exactly one member resource of from it. By default, the whole response body is used as the body.",
				MarkdownDescription: "A selector in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries) query syntax, that is used when create returns a collection of resources, to select exactly one member resource of from it. By default, the whole response body is used as the body.",
				Optional:            true,
			},
			"read_selector": schema.StringAttribute{
				Description:         "A selector expression in gjson query syntax, that is used when read returns a collection of resources, to select exactly one member resource of from it. This" + bodyParamDescription + " By default, the whole response body is used as the body.",
				MarkdownDescription: "A selector expression in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used when read returns a collection of resources, to select exactly one member resource of from it. This" + bodyParamDescription + " By default, the whole response body is used as the body.",
				Optional:            true,
			},

			"read_path": schema.StringAttribute{
				Description:         "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + bodyOrPathParamDescription,
				MarkdownDescription: "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},
			"update_path": schema.StringAttribute{
				Description:         "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + bodyOrPathParamDescription,
				MarkdownDescription: "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},
			"delete_path": schema.StringAttribute{
				Description:         "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + bodyOrPathParamDescription,
				MarkdownDescription: "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},

			"body": schema.DynamicAttribute{
				Description:         "The properties of the resource.",
				MarkdownDescription: "The properties of the resource.",
				Required:            true,
			},

			"ephemeral_body": schema.DynamicAttribute{
				Description:         "The ephemeral (write-only) properties of the resource. This will be merge-patched to the `body` to construct the actual request body.",
				MarkdownDescription: "The ephemeral (write-only) properties of the resource. This will be merge-patched to the `body` to construct the actual request body.",
				Optional:            true,
				WriteOnly:           true,
			},

			"delete_body": schema.DynamicAttribute{
				Description:         "The payload for the `Delete` call. Conflicts with `delete_body_raw`.",
				MarkdownDescription: "The payload for the `Delete` call. Conflicts with `delete_body_raw`.",
				Optional:            true,
				Validators: []validator.Dynamic{
					dynamicvalidator.ConflictsWith(
						path.MatchRoot("delete_body_raw"),
					),
				},
			},
			"delete_body_raw": schema.StringAttribute{
				Description:         "The raw payload for the `Delete` call. It can contain `$(body.x.y.z)` parameter that reference property from the `state.output`. Conflicts with `delete_body`.",
				MarkdownDescription: "The raw payload for the `Delete` call. It can contain `$(body.x.y.z)` parameter that reference property from the `state.output`. Conflicts with `delete_body`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("delete_body"),
					),
				},
			},

			"update_body_patches": schema.ListNestedAttribute{
				Description:         "The body patches for update only. Any change here won't cause a update API call by its own, only changes from `body` does. Note that this is almost only useful for APIs that require *after-create* attribute for an update (e.g. the resource ID).",
				MarkdownDescription: "The body patches for update only. Any change here won't cause a update API call by its own, only changes from `body` does. Note that this is almost only useful for APIs that require *after-create* attribute for an update (e.g. the resource ID).",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Description:         "The path (in gjson syntax) to the attribute to patch.",
							MarkdownDescription: "The path (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attribute to [patch](https://github.com/tidwall/sjson?tab=readme-ov-file#set-a-value).",
							Required:            true,
						},
						"raw_json": schema.StringAttribute{
							MarkdownDescription: "The raw json used as the patch value. It can contain `$(body.x.y.z)` parameter that reference property from the `state.output`. Exactly one of `raw_json` and `removed` shall be specified.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("raw_json"),
									path.MatchRelative().AtParent().AtName("removed"),
								),
							},
						},
						"removed": schema.BoolAttribute{
							MarkdownDescription: "Remove the value specified by `path` from the update body. Exactly one of `raw_json` and `removed` shall be specified",
							Optional:            true,
							Validators: []validator.Bool{
								boolvalidator.Equals(true),
								boolvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("raw_json"),
									path.MatchRelative().AtParent().AtName("removed"),
								),
							},
						},
					},
				},
			},

			"read_response_template": schema.StringAttribute{
				Description:         "The raw template for transforming the response of reading (after selector). It can contain `$(body.x.y.z)` parameter that reference property from the response. This is only used to transform the read response to the same struct as the `body`.",
				MarkdownDescription: "The raw template for transforming the response of reading (after selector). It can contain `$(body.x.y.z)` parameter that reference property from the response. This is only used to transform the read response to the same struct as the `body`.",
				Optional:            true,
			},

			"poll_create": pollAttribute("Create"),
			"poll_update": pollAttribute("Update"),
			"poll_delete": pollAttribute("Delete"),

			"precheck_create": precheckAttribute("Create", true, "", false),
			"precheck_update": precheckAttribute("Update", false, "By default, the `id` of this resource is used.", true),
			"precheck_delete": precheckAttribute("Delete", false, "By default, the `id` of this resource is used.", true),

			"post_create_read": schema.SingleNestedAttribute{
				Description:         "An additional read after creation (after polling, if any) for overriding the `$(body)` used for `read_path`, which was representing the response body of the initial create call. This is only meant to be used for APIs that only forms a resource id after the resource is completely created. One example is the AzureDevOps `project` API: A `project` is identified by a UUID, the user needs to create the project, polling the long running operation, then query the `project` by its (mutable) name, where it returns you the (immutable) UUID.",
				MarkdownDescription: "An additional read after creation (after polling, if any) for overriding the `$(body)` used for `read_path`, which was representing the response body of the initial create call. This is only meant to be used for APIs that only forms a resource id after the resource is completely created. One example is the AzureDevOps `project` API: A `project` is identified by a UUID, the user needs to create the project, polling the long running operation, then query the `project` by its (mutable) name, where it returns you the (immutable) UUID.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"path": schema.StringAttribute{
						Description:         "The API path used to read the resource. " + bodyOrPathParamDescription,
						MarkdownDescription: "The API path used to read the resource. " + bodyOrPathParamDescription,
						Required:            true,
						Validators: []validator.String{
							myvalidator.StringIsPathBuilder(),
						},
					},
					"query": schema.MapAttribute{
						Description:         operationOverridableAttrDescription("query", "post create read") + " The query value" + bodyParamDescription,
						MarkdownDescription: operationOverridableAttrDescription("query", "post create read") + " The query value" + bodyParamDescription,
						ElementType:         types.ListType{ElemType: types.StringType},
						Optional:            true,
					},
					"header": schema.MapAttribute{
						Description:         operationOverridableAttrDescription("header", "post create read") + " The header value" + bodyParamDescription,
						MarkdownDescription: operationOverridableAttrDescription("header", "post create read") + " The header value" + bodyParamDescription,
						ElementType:         types.StringType,
						Optional:            true,
					},
					"selector": schema.StringAttribute{
						Description:         "A selector expression in gjson query syntax, that is used when read returns a collection of resources, to select exactly one member resource of from it. This" + bodyParamDescription + " By default, the whole response body is used as the body.",
						MarkdownDescription: "A selector expression in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used when read returns a collection of resources, to select exactly one member resource of from it. This" + bodyParamDescription + " By default, the whole response body is used as the body.",
						Optional:            true,
					},
				},
			},

			"create_method": schema.StringAttribute{
				Description:         "The method used to create the resource. Possible values are `PUT`, `POST` and `PATCH`. This overrides the `create_method` set in the provider block (defaults to POST).",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT`, `POST` and `PATCH`. This overrides the `create_method` set in the provider block (defaults to POST).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "POST", "PATCH"),
				},
			},
			"update_method": schema.StringAttribute{
				Description:         "The method used to update the resource. Possible values are `PUT`, `POST` and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				MarkdownDescription: "The method used to update the resource. Possible values are `PUT`, `POST`, and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "PATCH", "POST"),
				},
			},
			"delete_method": schema.StringAttribute{
				Description:         "The method used to delete the resource. Possible values are `DELETE`, `POST`, `PUT` and `PATCH`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				MarkdownDescription: "The method used to delete the resource. Possible values are `DELETE`, `POST`, `PUT` and `PATCH`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("DELETE", "POST", "PUT", "PATCH"),
				},
			},
			"write_only_attrs": schema.ListAttribute{
				Description:         "A list of paths (in gjson syntax) to the attributes that are only settable, but won't be read in GET response. Prefer to use `ephemeral_body`.",
				MarkdownDescription: "A list of paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attributes that are only settable, but won't be read in GET response. Prefer to use `ephemeral_body`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"merge_patch_disabled": schema.BoolAttribute{
				Description:         "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				MarkdownDescription: "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				Optional:            true,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"create_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "create"),
				MarkdownDescription: operationOverridableAttrDescription("query", "create"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"update_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "update") + " The query value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("query", "update") + " The query value" + bodyParamDescription,
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"read_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "read") + " The query value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("query", "read") + " The query value" + bodyParamDescription,
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"delete_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "delete") + " The query value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("query", "delete") + " The query value" + bodyParamDescription,
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"create_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "create"),
				MarkdownDescription: operationOverridableAttrDescription("header", "create"),
				ElementType:         types.StringType,
				Optional:            true,
			},
			"update_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "update") + " The header value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("header", "update") + " The header value" + bodyParamDescription,
				ElementType:         types.StringType,
				Optional:            true,
			},
			"read_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "read") + " The header value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("header", "read") + " The header value" + bodyParamDescription,
				ElementType:         types.StringType,
				Optional:            true,
			},
			"delete_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "delete") + " The header value" + bodyParamDescription,
				MarkdownDescription: operationOverridableAttrDescription("header", "delete") + " The header value" + bodyParamDescription,
				ElementType:         types.StringType,
				Optional:            true,
			},
			"check_existance": schema.BoolAttribute{
				Description:         "Whether to check resource already existed? Defaults to `false`.",
				MarkdownDescription: "Whether to check resource already existed? Defaults to `false`.",
				Optional:            true,
			},
			"force_new_attrs": schema.SetAttribute{
				Description:         "A set of `body` attribute paths (in gjson syntax) whose value once changed, will trigger a replace of this resource. Note this only take effects when the `body` is a unknown before apply. Technically, we do a JSON merge patch and check whether the attribute path appear in the merge patch.",
				MarkdownDescription: "A set of `body` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) whose value once changed, will trigger a replace of this resource. Note this only take effects when the `body` is a unknown before apply. Technically, we do a JSON merge patch and check whether the attribute path appear in the merge patch.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"output_attrs": schema.SetAttribute{
				Description:         "A set of `output` attribute paths (in gjson syntax) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				MarkdownDescription: "A set of `output` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"use_sensitive_output": schema.BoolAttribute{
				Description:         "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				MarkdownDescription: "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				Optional:            true,
			},
			"output": schema.DynamicAttribute{
				Description:         "The response body. If `ephemeral_body` get returned by API, it will be removed from `output`. This is only populated when `use_sensitive_output` is false.",
				MarkdownDescription: "The response body. If `ephemeral_body` get returned by API, it will be removed from `output`. This is only populated when `use_sensitive_output` is false.",
				Computed:            true,
			},
			       "sensitive_output": schema.DynamicAttribute{
				       Description:         "The response body (sensitive). If `ephemeral_body` get returned by API, it will be removed from `sensitive_output`. This is only populated when `use_sensitive_output` is true.",
				       MarkdownDescription: "The response body (sensitive). If `ephemeral_body` get returned by API, it will be removed from `sensitive_output`. This is only populated when `use_sensitive_output` is true.",
				       Computed:            true,
				       Sensitive:           true,
			       },

			       // Novo campo security
			       "security": schema.SingleNestedAttribute{
				       Description:         "The OpenAPI security scheme that is used for auth. Only one of `http`, `apikey`, or `oauth2` can be specified. If defined here, it overrides the provider's security.",
				       MarkdownDescription: "The OpenAPI security scheme that is used for auth. Only one of `http`, `apikey`, or `oauth2` can be specified. If defined here, it overrides the provider's security.",
				       Optional:            true,
				       Sensitive:           true,
				       Attributes: map[string]schema.Attribute{
					       "http": schema.SingleNestedAttribute{
						       Description:         "Configuration for the HTTP authentication scheme. Exactly one of `basic` and `token` must be specified.",
						       MarkdownDescription: "Configuration for the HTTP authentication scheme. Exactly one of `basic` and `token` must be specified.",
						       Optional:            true,
						       Attributes: map[string]schema.Attribute{
							       "basic": schema.SingleNestedAttribute{
								       Description:         "Basic authentication",
								       MarkdownDescription: "Basic authentication",
								       Optional:            true,
								       Attributes: map[string]schema.Attribute{
									       "username": schema.StringAttribute{
										       Description:         "The username",
										       MarkdownDescription: "The username",
										       Required:            true,
									       },
									       "password": schema.StringAttribute{
										       Description:         "The password",
										       MarkdownDescription: "The password",
										       Required:            true,
										       Sensitive:           true,
									       },
								       },
								       Validators: []validator.Object{
									       objectvalidator.ExactlyOneOf(
										       path.MatchRoot("security").AtName("http").AtName("basic"),
										       path.MatchRoot("security").AtName("http").AtName("token"),
									       ),
								       },
							       },
							       "token": schema.SingleNestedAttribute{
								       Description:         "Auth token (e.g. Bearer).",
								       MarkdownDescription: "Auth token (e.g. Bearer).",
								       Optional:            true,
								       Attributes: map[string]schema.Attribute{
									       "token": schema.StringAttribute{
										       Description:         "The value of the token.",
										       MarkdownDescription: "The value of the token.",
										       Required:            true,
										       Sensitive:           true,
									       },
									       "scheme": schema.StringAttribute{
										       Description:         "The auth scheme. Defaults to `Bearer`.",
										       MarkdownDescription: "The auth scheme. Defaults to `Bearer`.",
										       Optional:            true,
									       },
								       },
								       Validators: []validator.Object{
									       objectvalidator.ExactlyOneOf(
										       path.MatchRoot("security").AtName("http").AtName("basic"),
										       path.MatchRoot("security").AtName("http").AtName("token"),
									       ),
								       },
							       },
						       },
					       },
					       "apikey": schema.SetNestedAttribute{
						       Description:         "Configuration for the API Key authentication scheme.",
						       MarkdownDescription: "Configuration for the API Key authentication scheme.",
						       Optional:            true,
						       NestedObject: schema.NestedAttributeObject{
							       Attributes: map[string]schema.Attribute{
								       "name": schema.StringAttribute{
									       Description:         "The API Key name",
									       MarkdownDescription: "The API Key name",
									       Required:            true,
								       },
								       "value": schema.StringAttribute{
									       Description:         "The API Key value",
									       MarkdownDescription: "The API Key value",
									       Required:            true,
								       },
								       "in": schema.StringAttribute{
									       Description:         "Specifies how the API Key is sent. Possible values are `query`, `header`, or `cookie`.",
									       MarkdownDescription: "Specifies how the API Key is sent. Possible values are `query`, `header`, or `cookie`.",
									       Required:            true,
									       Validators: []validator.String{
										       stringvalidator.OneOf("header", "query", "cookie"),
									       },
								       },
							       },
						       },
					       },
					       "oauth2": schema.SingleNestedAttribute{
						       Description:         "Configuration for the OAuth2 authentication scheme. Exactly one of `password`, `client_credentials` and `refresh_token` must be specified.",
						       MarkdownDescription: "Configuration for the OAuth2 authentication scheme. Exactly one of `password`, `client_credentials` and `refresh_token` must be specified.",
						       Optional:            true,
						       Attributes: map[string]schema.Attribute{
							       "password": schema.SingleNestedAttribute{
								       Description:         "Resource owner password credential.",
								       MarkdownDescription: "Resource owner password credential.",
								       Optional:            true,
								       Attributes: map[string]schema.Attribute{
									       "token_url": schema.StringAttribute{
										       Description:         "The token URL to be used for this flow.",
										       MarkdownDescription: "The token URL to be used for this flow.",
										       Required:            true,
									       },
									       "username": schema.StringAttribute{
										       Description:         "The username.",
										       MarkdownDescription: "The username.",
										       Required:            true,
									       },
									       "password": schema.StringAttribute{
										       Sensitive:           true,
										       Description:         "The password.",
										       MarkdownDescription: "The password.",
										       Required:            true,
									       },
									       "client_id": schema.StringAttribute{
										       Description:         "The application's ID.",
										       MarkdownDescription: "The application's ID.",
										       Optional:            true,
									       },
									       "client_secret": schema.StringAttribute{
										       Sensitive:           true,
										       Description:         "The application's secret.",
										       MarkdownDescription: "The application's secret.",
										       Optional:            true,
									       },
									       "in": schema.StringAttribute{
										       Description:         "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       MarkdownDescription: "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       Optional:            true,
										       Validators: []validator.String{stringvalidator.OneOf("params", "header")},
									       },
									       "scopes": schema.ListAttribute{
										       ElementType:         types.StringType,
										       Description:         "The optional requested permissions.",
										       MarkdownDescription: "The optional requested permissions.",
										       Optional:            true,
									       },
								       },
								       Validators: []validator.Object{
									       objectvalidator.ExactlyOneOf(
										       path.MatchRoot("security").AtName("oauth2").AtName("password"),
										       path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										       path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									       ),
								       },
							       },
							       "client_credentials": schema.SingleNestedAttribute{
								       Description:         "Client credentials.",
								       MarkdownDescription: "Client credentials.",
								       Optional:            true,
								       Attributes: map[string]schema.Attribute{
									       "token_url": schema.StringAttribute{
										       Description:         "The token URL to be used for this flow.",
										       MarkdownDescription: "The token URL to be used for this flow.",
										       Required:            true,
									       },
									       "client_id": schema.StringAttribute{
										       Description:         "The application's ID.",
										       MarkdownDescription: "The application's ID.",
										       Required:            true,
									       },
									       "client_secret": schema.StringAttribute{
										       Sensitive:           true,
										       Description:         "The application's secret.",
										       MarkdownDescription: "The application's secret.",
										       Required:            true,
									       },
									       "in": schema.StringAttribute{
										       Description:         "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       MarkdownDescription: "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       Optional:            true,
										       Validators: []validator.String{stringvalidator.OneOf("params", "header")},
									       },
									       "scopes": schema.ListAttribute{
										       ElementType:         types.StringType,
										       Description:         "The optional requested permissions.",
										       MarkdownDescription: "The optional requested permissions.",
										       Optional:            true,
									       },
									       "endpoint_params": schema.MapAttribute{
										       ElementType:         types.ListType{ElemType: types.StringType},
										       Description:         "The additional parameters for requests to the token endpoint.",
										       MarkdownDescription: "The additional parameters for requests to the token endpoint.",
										       Optional:            true,
									       },
								       },
								       Validators: []validator.Object{
									       objectvalidator.ExactlyOneOf(
										       path.MatchRoot("security").AtName("oauth2").AtName("password"),
										       path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										       path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									       ),
								       },
							       },
							       "refresh_token": schema.SingleNestedAttribute{
								       Description:         "Refresh token.",
								       MarkdownDescription: "Refresh token.",
								       Optional:            true,
								       Attributes: map[string]schema.Attribute{
									       "token_url": schema.StringAttribute{
										       Description:         "The token URL to be used for this flow.",
										       MarkdownDescription: "The token URL to be used for this flow.",
										       Required:            true,
									       },
									       "refresh_token": schema.StringAttribute{
										       Description:         "The refresh token.",
										       MarkdownDescription: "The refresh token.",
										       Sensitive:           true,
										       Required:            true,
									       },
									       "client_id": schema.StringAttribute{
										       Description:         "The application's ID.",
										       MarkdownDescription: "The application's ID.",
										       Optional:            true,
									       },
									       "client_secret": schema.StringAttribute{
										       Sensitive:           true,
										       Description:         "The application's secret.",
										       MarkdownDescription: "The application's secret.",
										       Optional:            true,
									       },
									       "scopes": schema.ListAttribute{
										       ElementType:         types.StringType,
										       Description:         "The optional requested permissions.",
										       MarkdownDescription: "The optional requested permissions.",
										       Optional:            true,
									       },
									       "in": schema.StringAttribute{
										       Description:         "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       MarkdownDescription: "Specifies how is the client ID & secret sent. Possible values are `params` e `header`. Se ausente, será auto detectado.",
										       Optional:            true,
										       Validators: []validator.String{stringvalidator.OneOf("params", "header")},
									       },
									       "token_type": schema.StringAttribute{
										       Description:         `The type of the access token. Defaults to "Bearer".`,
										       MarkdownDescription: `The type of the access token. Defaults to "Bearer".`,
										       Optional:            true,
									       },
								       },
								       Validators: []validator.Object{
									       objectvalidator.ExactlyOneOf(
										       path.MatchRoot("security").AtName("oauth2").AtName("password"),
										       path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										       path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									       ),
								       },
							       },
						       },
					       },
				       },
			       },
		},
	}
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.Body.IsUnknown() {
		b, err := dynamic.ToJSON(config.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf("marshal body: %v", err),
			)
			return
		}

		if !config.WriteOnlyAttributes.IsUnknown() && !config.WriteOnlyAttributes.IsNull() {
			for _, ie := range config.WriteOnlyAttributes.Elements() {
				ie := ie.(types.String)
				if !ie.IsUnknown() && !ie.IsNull() {
					if !gjson.Get(string(b), ie.ValueString()).Exists() {
						resp.Diagnostics.AddError(
							"Invalid configuration",
							fmt.Sprintf(`Invalid path in "write_only_attrs": %s`, ie.String()),
						)
						return
					}
				}
			}
		}

		_, diags := ephemeral.ValidateEphemeralBody(b, config.EphemeralBody)
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		if diags.HasError() {
			return
		}
	}
}

func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		// If the entire plan is null, the resource is planned for destruction.
		return
	}
	if req.State.Raw.IsNull() {
		// If the entire state is null, the resource is planned for creation.
		return
	}

	var plan resourceData
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	var state resourceData
	if diags := req.State.Get(ctx, &state); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	var config resourceData
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	defer func() {
		resp.Plan.Set(ctx, plan)
	}()

	// Set require replace if force new attributes have changed
	if !plan.ForceNewAttrs.IsUnknown() && !plan.Body.IsUnknown() {
		var forceNewAttrs []types.String
		if diags := plan.ForceNewAttrs.ElementsAs(ctx, &forceNewAttrs, false); diags != nil {
			resp.Diagnostics.Append(diags...)
			return
		}
		var knownForceNewAttrs []string
		for _, attr := range forceNewAttrs {
			if attr.IsUnknown() {
				continue
			}
			knownForceNewAttrs = append(knownForceNewAttrs, attr.ValueString())
		}

		if len(knownForceNewAttrs) != 0 {
			var state resourceData
			if diags := req.State.Get(ctx, &state); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}

			originJson, err := dynamic.ToJSON(state.Body)
			if err != nil {
				resp.Diagnostics.AddError(
					"ModifyPlan failed",
					fmt.Sprintf("marshaling state body: %v", err),
				)
			}

			modifiedJson, err := dynamic.ToJSON(plan.Body)
			if err != nil {
				resp.Diagnostics.AddError(
					"ModifyPlan failed",
					fmt.Sprintf("marshaling plan body: %v", err),
				)
			}

			patch, err := jsonpatch.CreateMergePatch(originJson, modifiedJson)
			if err != nil {
				resp.Diagnostics.AddError("failed to create merge patch", err.Error())
				return
			}
			for _, attr := range knownForceNewAttrs {
				result := gjson.Get(string(patch), attr)
				if result.Exists() {
					resp.RequiresReplace = []tfpath.Path{tfpath.Root("body")}
					break
				}
			}
		}
	}

	// Set output as unknown to trigger a plan diff, if ephemral body has changed
	diff, diags := ephemeral.Diff(ctx, req.Private, config.EphemeralBody)
	resp.Diagnostics = append(resp.Diagnostics, diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if diff {
		tflog.Info(ctx, `"ephemeral_body" has changed`)
		// Mark the appropriate output as unknown based on use_sensitive_output
		if !plan.UseSensitiveOutput.IsNull() && plan.UseSensitiveOutput.ValueBool() {
			plan.SensitiveOutput = types.DynamicUnknown()
		} else {
			plan.Output = types.DynamicUnknown()
		}
	}
}

// getOutput returns the appropriate output (sensitive or normal) based on use_sensitive_output
func getOutput(data resourceData) types.Dynamic {
	if !data.UseSensitiveOutput.IsNull() && data.UseSensitiveOutput.ValueBool() {
		return data.SensitiveOutput
	}
	return data.Output
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	r.p = providerData.provider
}

func (r Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var plan resourceData
	var diags diag.Diagnostics

       diags = req.Plan.Get(ctx, &plan)
       resp.Diagnostics.Append(diags...)
       if diags.HasError() {
	       return
       }

       // Determine baseURL: resource overrides provider
       baseURL := plan.BaseURL
       if baseURL.IsNull() {
	       baseURL = r.p.config.BaseURL
       }
       if baseURL.IsNull() {
	       resp.Diagnostics.AddError("base_url is required", "Either define base_url in the provider or in the resource.")
	       return
       }

       var c *client.Client
       var err error
       if !plan.BaseURL.IsNull() || (!plan.Security.IsNull() && !plan.Security.IsUnknown()) {
	       // Need to create a new client
	       clientOpt := &client.BuildOption{}
	       if r.p.client != nil {
		       // Copy TLS and other configs from provider client
		       clientOpt.CookieEnabled = r.p.client.Client.GetClient().Jar != nil
		       if transport, ok := r.p.client.Client.GetClient().Transport.(*http.Transport); ok {
			       clientOpt.TLSConfig = *transport.TLSClientConfig
		       }
		       // TODO: copy retry if needed
	       }
	       if !plan.Security.IsNull() && !plan.Security.IsUnknown() {
		       var security client.SecurityOption
		       security, diags = populateSecurity(ctx, plan.Security)
		       resp.Diagnostics.Append(diags...)
		       if diags.HasError() {
			       return
		       }
		       clientOpt.Security = security
	       } else if r.p.client != nil {
		       clientOpt.Security = r.p.client.Security
	       }
	       c, err = client.New(ctx, baseURL.ValueString(), clientOpt)
	       if err != nil {
		       resp.Diagnostics.AddError("Failed to create client", err.Error())
		       return
	       }
       } else {
	       c = r.p.client
       }

	var config resourceData
	diags = req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	tflog.Info(ctx, "Create a resource", map[string]interface{}{"path": plan.Path.ValueString()})

	opt, diags := r.p.apiOpt.ForResourceCreate(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if plan.CheckExistance.ValueBool() {
		opt, diags := r.p.apiOpt.ForResourceRead(ctx, plan, nil)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		response, err := c.Read(ctx, plan.Path.ValueString(), *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Existance check failed",
				err.Error(),
			)
			return
		}
		if response.StatusCode() != http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Resource already exists",
				fmt.Sprintf("A resource with the ID %q already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for %q for more information.", plan.Path.ValueString(), `restful_resource`),
			)
			return
		}
	}

	// Precheck
	if !plan.PrecheckCreate.IsNull() {
		unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, "", opt.Header, opt.Query, plan.PrecheckCreate, basetypes.NewDynamicNull())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		defer unlockFunc()
	}

	// Build the body
	b, err := dynamic.ToJSON(plan.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			`Error to marshal "body"`,
			err.Error(),
		)
		return
	}

	var eb []byte
	if !config.EphemeralBody.IsNull() {
		eb, diags = ephemeral.ValidateEphemeralBody(b, config.EphemeralBody)
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Merge patch the ephemeral body to the body
		b, err = jsonpatch.MergePatch(b, eb)
		if err != nil {
			resp.Diagnostics.AddError(
				"Merge patching `body` with `ephemeral_body`",
				err.Error(),
			)
			return
		}
	}

	// Create the resource
	response, err := c.Create(ctx, plan.Path.ValueString(), string(b), *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call create",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Create API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b = response.Body()

	if sel := plan.CreateSelector.ValueString(); sel != "" {
		bodyLocator := client.BodyLocator(sel)
		sb, ok := bodyLocator.LocateValueInResp(*response)
		if !ok {
			resp.Diagnostics.AddError(
				"`create_selector` failed to select from the response",
				string(response.Body()),
			)
			return
		}
		b = []byte(sb)
	}

	// Construct the resource id, which is used as the path to:
	// - Be the fallback read path for polling (if any) if no URL locator is specified
	// - Read the resource later on
	resourceId := plan.Path.ValueString()
	if !plan.ReadPath.IsNull() {
		resourceId, err = exparam.ExpandBodyOrPath(plan.ReadPath.ValueString(), plan.Path.ValueString(), b)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build the path for reading the resource",
				fmt.Sprintf("Can't build resource id with `read_path`: %q, `path`: %q, `body`: %q: %v", plan.ReadPath.ValueString(), plan.Path.ValueString(), string(b), err),
			)
			return
		}
	}

	output, err := dynamic.FromJSONImplied(b)
	if err != nil {
		resp.Diagnostics.AddError(
			"Evaluating `output` during Read",
			err.Error(),
		)
		return
	}

	diags = ephemeral.Set(ctx, resp.Private, eb)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// For LRO, wait for completion
	if !plan.PollCreate.IsNull() {
		var d pollData
		if diags := plan.PollCreate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d, output)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		if opt.UrlLocator == nil {
			// Update the request URL to pointing to the resource path, which is mainly for resources whose create method is POST.
			// As it will be used to poll the resource status.
			response.Request.URL = resourceId
		}
		p, err := client.NewPollableForPoll(*response, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Create: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Create: Polling failure",
				err.Error(),
			)
			return
		}
	}

	// PostCreateRead is to update the resource ID and the Output by sending a post-create only read call.
	if !plan.PostCreateRead.IsNull() {
		var pr postCreateRead
		if diags := plan.PostCreateRead.As(ctx, &pr, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForResourcePostCreateRead(ctx, plan, pr, b)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		response, err := c.Read(ctx, pr.Path.ValueString(), *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error to call post-create read",
				err.Error(),
			)
			return
		}
		if !response.IsSuccess() {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Post-create read API returns %d", response.StatusCode()),
				string(response.Body()),
			)
			return
		}

		b := response.Body()

		if sel := pr.Selector.ValueString(); sel != "" {
			sel, err = exparam.ExpandBody(sel, b)
			if err != nil {
				resp.Diagnostics.AddError(
					"Post-create read failure",
					fmt.Sprintf("Failed to expand the post-create read selector: %v", err),
				)
				return
			}
			bodyLocator := client.BodyLocator(sel)
			sb, _ := bodyLocator.LocateValueInResp(*response)
			// This means the tracked resource selected (filtered) from the response now disappears (deleted out of band).
			b = []byte(sb)
		}

		// Update the resource Id
		if !plan.ReadPath.IsNull() {
			resourceId, err = exparam.ExpandBodyOrPath(plan.ReadPath.ValueString(), plan.Path.ValueString(), b)
			if err != nil {
				resp.Diagnostics.AddError(
					"Failed to build the path for reading the resource (post-create phase)",
					fmt.Sprintf("Can't build resource id with `read_path`: %q, `path`: %q, `body`: %q: %v", plan.ReadPath.ValueString(), plan.Path.ValueString(), string(b), err),
				)
				return
			}
		}

		// Update the output
		output, err = dynamic.FromJSONImplied(b)
		if err != nil {
			resp.Diagnostics.AddError(
				"Evaluating `output` during Read",
				err.Error(),
			)
			return
		}
	}

	// Set resource ID
	plan.ID = types.StringValue(resourceId)

	// Temporarily set the output here, so that the Read at the end can expand the `$(body)` parameters.
	// Populate the appropriate output based on use_sensitive_output
	if !plan.UseSensitiveOutput.IsNull() && plan.UseSensitiveOutput.ValueBool() {
		plan.SensitiveOutput = output
		plan.Output = types.DynamicNull()
	} else {
		plan.Output = output
		plan.SensitiveOutput = types.DynamicNull()
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	rreq := resource.ReadRequest{
		State:        resp.State,
		Private:      resp.Private,
		Identity:     req.Identity,
		ProviderMeta: req.ProviderMeta,
	}
	rresp := resource.ReadResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
		Identity:    resp.Identity,
	}
	r.read(ctx, rreq, &rresp, false)

	resp.State = rresp.State
	resp.Diagnostics = rresp.Diagnostics
	resp.Identity = rresp.Identity
}

func (r Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
			       var state resourceData
			       diags := req.State.Get(ctx, &state)
			       resp.Diagnostics.Append(diags...)
			       if diags.HasError() {
				       return
			       }

			       c := r.p.client
			       c.SetLoggerContext(ctx)

			       // If security is defined in the resource, create a temporary client with the resource's security
			       if !state.Security.IsNull() && !state.Security.IsUnknown() {
				       security, diags := populateSecurity(ctx, state.Security)
				       resp.Diagnostics.Append(diags...)
				       if diags.HasError() {
					       return
				       }
				       tmpClient, err := client.NewWithOverridesFromExisting(c, c.BaseURL, security)
				       if err != nil {
					       resp.Diagnostics.AddError("Failed to create client with resource security", err.Error())
					       return
				       }
				       c = tmpClient
			       }

			       // Usa o client correto, mas chama o read padrão
			       r.read(ctx, req, resp, true)
}

func (r Resource) read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse, updateBody bool) {
	var state resourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Determine baseURL: resource overrides provider
	baseURL := state.BaseURL
	if baseURL.IsNull() {
		baseURL = r.p.config.BaseURL
	}
	if baseURL.IsNull() {
		resp.Diagnostics.AddError("base_url is required", "Either define base_url in the provider or in the resource.")
		return
	}

	var c *client.Client
	var err error
	if !state.BaseURL.IsNull() || (!state.Security.IsNull() && !state.Security.IsUnknown()) {
		// Need to create a new client
		clientOpt := &client.BuildOption{}
		if r.p.client != nil {
			// Copy TLS and other configs from provider client
			clientOpt.CookieEnabled = r.p.client.Client.GetClient().Jar != nil
			if transport, ok := r.p.client.Client.GetClient().Transport.(*http.Transport); ok {
				clientOpt.TLSConfig = *transport.TLSClientConfig
			}
			// TODO: copy retry if needed
		}
		if !state.Security.IsNull() && !state.Security.IsUnknown() {
			var security client.SecurityOption
			security, diags = populateSecurity(ctx, state.Security)
			resp.Diagnostics.Append(diags...)
			if diags.HasError() {
				return
			}
			clientOpt.Security = security
		} else if r.p.client != nil {
			clientOpt.Security = r.p.client.Security
		}
		c, err = client.New(ctx, baseURL.ValueString(), clientOpt)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create client", err.Error())
			return
		}
	} else {
		c = r.p.client
	}

	if updateBody {
		tflog.Info(ctx, "Read a resource", map[string]interface{}{"id": state.ID.ValueString()})
	}

	stateOutput, err := dynamic.ToJSON(getOutput(state))
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("marshal state output: %v", err),
		)
		return
	}

	opt, diags := r.p.apiOpt.ForResourceRead(ctx, state, stateOutput)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Read(ctx, state.ID.ValueString(), *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call read",
			err.Error(),
		)
		return
	}
	if response.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Read API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	if sel := state.ReadSelector.ValueString(); sel != "" {
		sel, err = exparam.ExpandBody(sel, stateOutput)
		if err != nil {
			resp.Diagnostics.AddError(
				"Read failure",
				fmt.Sprintf("Failed to expand the read selector: %v", err),
			)
			return
		}
		bodyLocator := client.BodyLocator(sel)
		sb, ok := bodyLocator.LocateValueInResp(*response)
		// This means the tracked resource selected (filtered) from the response now disappears (deleted out of band).
		if !ok {
			resp.State.RemoveResource(ctx)
			return
		}
		b = []byte(sb)
	}

	if tpl := state.ReadResponseTemplate.ValueString(); tpl != "" {
		sb, err := exparam.ExpandBody(tpl, b)
		if err != nil {
			resp.Diagnostics.AddError(
				"Read failure",
				fmt.Sprintf("Failed to expand the read response template: %v", err),
			)
			return
		}
		b = []byte(sb)
	}

	if updateBody {
		var writeOnlyAttributes []string
		diags = state.WriteOnlyAttributes.ElementsAs(ctx, &writeOnlyAttributes, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}

		// Update the read response by compensating the write only attributes from state
		if len(writeOnlyAttributes) != 0 {
			pb := string(b)

			stateBody, err := dynamic.ToJSON(state.Body)
			if err != nil {
				resp.Diagnostics.AddError(
					"Read failure",
					fmt.Sprintf("marshal state body: %v", err),
				)
				return
			}

			for _, path := range writeOnlyAttributes {
				if gjson.Get(string(stateBody), path).Exists() && !gjson.Get(string(b), path).Exists() {
					pb, err = sjson.Set(pb, path, gjson.Get(string(stateBody), path).Value())
					if err != nil {
						resp.Diagnostics.AddError(
							"Read failure",
							fmt.Sprintf("json set write only attr at path %q: %v", path, err),
						)
						return
					}
				}
			}
			b = []byte(pb)
		}

		var body types.Dynamic
		if state.Body.IsNull() {
			body, err = dynamic.FromJSONImplied(b)
		} else {
			body, err = dynamic.FromJSON(b, state.Body.UnderlyingValue().Type(ctx))
		}
		if err != nil {
			// An error might occur here during refresh, when the type of the state doesn't match the remote,
			// e.g. a tuple field has different number of elements.
			// In this case, we fallback to the implied types, to make the refresh proceed and return a reasonable plan diff.
			if body, err = dynamic.FromJSONImplied(b); err != nil {
				resp.Diagnostics.AddError(
					"Evaluating `body` during Read",
					err.Error(),
				)
				return
			}
		}
		state.Body = body
	}

	// Set output
	if !state.OutputAttrs.IsNull() {
		var outputAttrs []string
		diags = state.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		fb, err := FilterAttrsInJSON(string(b), outputAttrs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Filter `output` during Read",
				err.Error(),
			)
			return
		}
		b = []byte(fb)
	}

	eb, diags := ephemeral.GetNullBody(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if eb != nil {
		b, err = jsonset.Difference(b, eb)
		if err != nil {
			resp.Diagnostics.AddError(
				"Removing `ephemeral_body` from `output`",
				err.Error(),
			)
			return
		}
	}
	output, err := dynamic.FromJSONImplied(b)
	if err != nil {
		resp.Diagnostics.AddError(
			"Evaluating `output` during Read",
			err.Error(),
		)
		return
	}
	// Populate the appropriate output based on use_sensitive_output
	if state.UseSensitiveOutput.ValueBool() {
		state.SensitiveOutput = output
		state.Output = types.DynamicNull()
	} else {
		state.Output = output
		state.SensitiveOutput = types.DynamicNull()
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Set resource identity
	impspec := ImportSpec{
		Id:   state.ID.ValueString(),
		Path: state.Path.ValueString(),
	}
	if !state.Query.IsNull() {
		impspec.Query = url.Values(client.Query{}.TakeOrSelf(ctx, state.Query))
	}
	if !state.Header.IsNull() {
		impspec.Header = client.Header{}.TakeOrSelf(ctx, state.Header)
	}
	if !state.Body.IsNull() {
		body, err := dynamic.ToJSON(state.Body)
		if err != nil {
			diags.AddError(
				"Failed to construct resource identity",
				fmt.Sprintf("convert `body` to JSON: %v", err),
			)
			return
		}
		nullBody, err := jsonset.NullifyObject(body)
		if err != nil {
			diags.AddError(
				"Failed to construct resource identity",
				fmt.Sprintf("nullify `body`: %v", err),
			)
			return
		}
		if string(nullBody) != "null" {
			impspec.Body = ToPtr(json.RawMessage(nullBody))
		}
	}
	if !state.ReadSelector.IsNull() {
		impspec.ReadSelector = state.ReadSelector.ValueStringPointer()
	}
	if !state.ReadResponseTemplate.IsNull() {
		impspec.ReadResponseTemplate = state.ReadResponseTemplate.ValueStringPointer()
	}

	impspecJSON, err := json.Marshal(impspec)
	if err != nil {
		diags.AddError(
			"Failed to construct resource identity",
			fmt.Sprintf("failed to marshal the import spec: %v", err),
		)
		return
	}
	if diags := resp.Identity.Set(ctx, resourceIdentityModel{
		ID: types.StringValue(string(impspecJSON)),
	}); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
}

func (r Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

       var plan resourceData
       var state resourceData
       var diags diag.Diagnostics

       diags = req.Plan.Get(ctx, &plan)
       resp.Diagnostics.Append(diags...)
       if diags.HasError() {
	       return
       }

       // Determine baseURL: resource overrides provider
       baseURL := plan.BaseURL
       if baseURL.IsNull() {
	       baseURL = r.p.config.BaseURL
       }
       if baseURL.IsNull() {
	       resp.Diagnostics.AddError("base_url is required", "Either define base_url in the provider or in the resource.")
	       return
       }

       var c *client.Client
       var err error
       if !plan.BaseURL.IsNull() || (!plan.Security.IsNull() && !plan.Security.IsUnknown()) {
	       // Need to create a new client
	       clientOpt := &client.BuildOption{}
	       if r.p.client != nil {
		       // Copy TLS and other configs from provider client
		       clientOpt.CookieEnabled = r.p.client.Client.GetClient().Jar != nil
		       if transport, ok := r.p.client.Client.GetClient().Transport.(*http.Transport); ok {
			       clientOpt.TLSConfig = *transport.TLSClientConfig
		       }
		       // TODO: copy retry if needed
	       }
	       if !plan.Security.IsNull() && !plan.Security.IsUnknown() {
		       var security client.SecurityOption
		       security, diags = populateSecurity(ctx, plan.Security)
		       resp.Diagnostics.Append(diags...)
		       if diags.HasError() {
			       return
		       }
		       clientOpt.Security = security
	       } else if r.p.client != nil {
		       clientOpt.Security = r.p.client.Security
	       }
	       c, err = client.New(ctx, baseURL.ValueString(), clientOpt)
	       if err != nil {
		       resp.Diagnostics.AddError("Failed to create client", err.Error())
		       return
	       }
       } else {
	       c = r.p.client
       }
       c.SetLoggerContext(ctx)

       diags = req.State.Get(ctx, &state)
       resp.Diagnostics.Append(diags...)
       if diags.HasError() {
	       return
       }

       var config resourceData
       diags = req.Config.Get(ctx, &config)
       resp.Diagnostics.Append(diags...)
       if diags.HasError() {
	       return
       }

	tflog.Info(ctx, "Update a resource", map[string]interface{}{"id": state.ID.ValueString()})

	stateOutput, err := dynamic.ToJSON(getOutput(state))
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("marshal state output: %v", err),
		)
		return
	}

	// Temporarily set the output here, so that the Read at the end can
	// expand the `$(body)` parameters.
	if state.UseSensitiveOutput.ValueBool() {
		plan.SensitiveOutput = state.SensitiveOutput
		plan.Output = types.DynamicNull()
	} else {
		plan.Output = state.Output
		plan.SensitiveOutput = types.DynamicNull()
	}

	opt, diags := r.p.apiOpt.ForResourceUpdate(ctx, plan, stateOutput)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	stateBody, err := dynamic.ToJSON(state.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Update failure",
			fmt.Sprintf("Error to marshal state body: %v", err),
		)
		return
	}
	planBody, err := dynamic.ToJSON(plan.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Update failure",
			fmt.Sprintf("Error to marshal plan body: %v", err),
		)
		return
	}

	// Optionally patch the body with the update_body_patches.
	var patches []bodyPatchData
	if diags := plan.UpdateBodyPatches.ElementsAs(ctx, &patches, false); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if len(patches) != 0 {
		planBodyStr := string(planBody)
		for i, patch := range patches {
			switch {
			case !patch.Removed.IsNull():
				planBodyStr, err = sjson.Delete(planBodyStr, patch.Path.ValueString())
				if err != nil {
					resp.Diagnostics.AddError(
						fmt.Sprintf("Failed to delete json for the %d-th patch at path %q", i, patch.Path.ValueString()),
						err.Error(),
					)
					return
				}
			case !patch.RawJSON.IsNull():
				pv, err := exparam.ExpandBody(patch.RawJSON.ValueString(), stateOutput)
				if err != nil {
					resp.Diagnostics.AddError(
						fmt.Sprintf("Failed to expand the %d-th patch for expression params", i),
						err.Error(),
					)
					return
				}

				planBodyStr, err = sjson.SetRaw(planBodyStr, patch.Path.ValueString(), pv)
				if err != nil {
					resp.Diagnostics.AddError(
						fmt.Sprintf("Failed to set json for the %d-th patch with %q", i, pv),
						err.Error(),
					)
					return
				}
			}
		}
		planBody = []byte(planBodyStr)
	}

	// Optionally patch the body with emphemeral_body
	var eb []byte
	ephemeralDiff, diags := ephemeral.Diff(ctx, req.Private, config.EphemeralBody)
	resp.Diagnostics = append(resp.Diagnostics, diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if ephemeralDiff {
		eb, diags = ephemeral.ValidateEphemeralBody(planBody, config.EphemeralBody)
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Merge patch the ephemeral body to the body
		if len(eb) != 0 {
			planBody, err = jsonpatch.MergePatch(planBody, eb)
			if err != nil {
				resp.Diagnostics.AddError(
					"Merge patching `body` with `ephemeral_body`",
					err.Error(),
				)
				return
			}
		}
	}

	// Invoke API to Update the resource only when there are changes in the body (regardless of the TF type diff).
	if string(stateBody) != string(planBody) || ephemeralDiff {
		// Precheck
		if !plan.PrecheckUpdate.IsNull() {
			unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, state.ID.ValueString(), opt.Header, opt.Query, plan.PrecheckUpdate, state.Output)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			defer unlockFunc()
		}

		if opt.Method == "PATCH" && !opt.MergePatchDisabled {
			stateBodyJSON, err := dynamic.ToJSON(state.Body)
			if err != nil {
				resp.Diagnostics.AddError(
					"Update failure",
					fmt.Sprintf("Error to marshal state body: %v", err),
				)
				return
			}
			b, err := jsonpatch.CreateMergePatch(stateBodyJSON, planBody)
			if err != nil {
				resp.Diagnostics.AddError(
					"Update failure",
					fmt.Sprintf("failed to create a merge patch: %s", err.Error()),
				)
				return
			}
			planBody = b
		}

		path := plan.ID.ValueString()
		if !plan.UpdatePath.IsNull() {
			output, err := dynamic.ToJSON(state.Output)
			if err != nil {
				resp.Diagnostics.AddError(
					"Failed to marshal json for `output`",
					err.Error(),
				)
				return
			}
			path, err = exparam.ExpandBodyOrPath(plan.UpdatePath.ValueString(), plan.Path.ValueString(), output)
			if err != nil {
				resp.Diagnostics.AddError(
					"Failed to build the path for updating the resource",
					fmt.Sprintf("Can't build path with `update_path`: %q, `path`: %q, `body`: %q", plan.UpdatePath.ValueString(), plan.Path.ValueString(), output),
				)
				return
			}
		}

		response, err := c.Update(ctx, path, string(planBody), *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error to call update",
				err.Error(),
			)
			return
		}
		if !response.IsSuccess() {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Update API returns %d", response.StatusCode()),
				string(response.Body()),
			)
			return
		}

		// For LRO, wait for completion
		if !plan.PollUpdate.IsNull() {
			var d pollData
			if diags := plan.PollUpdate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}

			opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d, state.Output)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			p, err := client.NewPollableForPoll(*response, *opt)
			if err != nil {
				resp.Diagnostics.AddError(
					"Update: Failed to build poller from the response of the initiated request",
					err.Error(),
				)
				return
			}
			if err := p.PollUntilDone(ctx, c); err != nil {
				resp.Diagnostics.AddError(
					"Update: Polling failure",
					err.Error(),
				)
				return
			}
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	diags = ephemeral.Set(ctx, resp.Private, eb)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	rreq := resource.ReadRequest{
		State:        resp.State,
		Private:      resp.Private,
		ProviderMeta: req.ProviderMeta,
		Identity:     req.Identity,
	}
	rresp := resource.ReadResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
		Identity:    resp.Identity,
	}
	r.read(ctx, rreq, &rresp, false)

	resp.State = rresp.State
	resp.Diagnostics = rresp.Diagnostics
	resp.Identity = rresp.Identity
}

func (r Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

	       var state resourceData
	       var diags diag.Diagnostics

	       diags = req.State.Get(ctx, &state)
	       resp.Diagnostics.Append(diags...)
	       if diags.HasError() {
		       return
	       }

	       // Determine baseURL: resource overrides provider
	       baseURL := state.BaseURL
	       if baseURL.IsNull() {
		       baseURL = r.p.config.BaseURL
	       }
	       if baseURL.IsNull() {
		       resp.Diagnostics.AddError("base_url is required", "Either define base_url in the provider or in the resource.")
		       return
	       }

	       var c *client.Client
	       var err error
	       if !state.BaseURL.IsNull() || (!state.Security.IsNull() && !state.Security.IsUnknown()) {
		       // Need to create a new client
		       clientOpt := &client.BuildOption{}
		       if r.p.client != nil {
			       // Copy TLS and other configs from provider client
			       clientOpt.CookieEnabled = r.p.client.Client.GetClient().Jar != nil
			       if transport, ok := r.p.client.Client.GetClient().Transport.(*http.Transport); ok {
				       clientOpt.TLSConfig = *transport.TLSClientConfig
			       }
			       // TODO: copy retry if needed
		       }
		       if !state.Security.IsNull() && !state.Security.IsUnknown() {
			       var security client.SecurityOption
			       security, diags = populateSecurity(ctx, state.Security)
			       resp.Diagnostics.Append(diags...)
			       if diags.HasError() {
				       return
			       }
			       clientOpt.Security = security
		       } else if r.p.client != nil {
			       clientOpt.Security = r.p.client.Security
		       }
		       c, err = client.New(ctx, baseURL.ValueString(), clientOpt)
		       if err != nil {
			       resp.Diagnostics.AddError("Failed to create client", err.Error())
			       return
		       }
       } else {
	       c = r.p.client
       }
       c.SetLoggerContext(ctx)

	tflog.Info(ctx, "Delete a resource", map[string]interface{}{"id": state.ID.ValueString()})

	stateOutput, err := dynamic.ToJSON(getOutput(state))
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to marshal json for `output`",
			err.Error(),
		)
		return
	}

	opt, diags := r.p.apiOpt.ForResourceDelete(ctx, state, stateOutput)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Precheck
	if !state.PrecheckDelete.IsNull() {
		unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, state.ID.ValueString(), opt.Header, opt.Query, state.PrecheckDelete, getOutput(state))
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		defer unlockFunc()
	}

	path := state.ID.ValueString()
	// Overwrite the path with delete_path, if set.
	if !state.DeletePath.IsNull() {
		path, err = exparam.ExpandBodyOrPath(state.DeletePath.ValueString(), state.Path.ValueString(), stateOutput)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build the path for deleting the resource",
				fmt.Sprintf("Can't build path with `delete_path`: %q, `path`: %q, `body`: %q", state.DeletePath.ValueString(), state.Path.ValueString(), stateOutput),
			)
			return
		}
	}

	var body string
	if db := state.DeleteBody; !db.IsNull() {
		b, err := dynamic.ToJSON(db)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to marshal `delete_body`",
				err.Error(),
			)
		}
		body = string(b)
	}
	if db := state.DeleteBodyRaw; !db.IsNull() {
		b, err := exparam.ExpandBody(db.ValueString(), stateOutput)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to expand the expressions in the `delete_body_raw`",
				err.Error(),
			)
		}
		body = b
	}

	response, err := c.Delete(ctx, path, body, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call delete",
			err.Error(),
		)
		return
	}

	if strings.EqualFold(opt.Method, "DELETE") {
		if response.StatusCode() == http.StatusNotFound {
			return
		}
	}

	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Delete API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	// For LRO, wait for completion
	if !state.PollDelete.IsNull() {
		var d pollData
		if diags := state.PollDelete.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d, state.Output)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollableForPoll(*response, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Delete: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Delete: Polling failure",
				err.Error(),
			)
			return
		}
	}
}

type ImportSpec struct {
	// Id is the resource id. Required.
	Id string `json:"id"`

	// Path is the path used to create the resource. Required.
	Path string `json:"path"`

	// Query is only required when it is mandatory for reading the resource.
	Query url.Values `json:"query,omitempty"`

	// Header is only required when it is mandatory for reading the resource.
	Header map[string]string `json:"header,omitempty"`

	// Body represents the properties expected to be managed and tracked by Terraform. The value of these properties can be null as a place holder.
	// When absent, all the response payload read wil be set to `body`.
	Body *json.RawMessage `json:"body,omitempty"`

	// ReadSelector is only required when reading the ID returns a list of resources, and you'd like to read only one of them.
	// Note that in this case, the value of the `Body` is likely required if the selector reference the body.
	ReadSelector *string `json:"read_selector,omitempty"`

	// ReadResponseTemplate is only required when the response from read is structually different than the `body`.
	ReadResponseTemplate *string `json:"read_response_template,omitempty"`
}

func (r *Resource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				Description:       "The import spec described at: https://registry.terraform.io/providers/magodo/restful/latest/docs/resources/resource#import.",
				RequiredForImport: true,
			},
		},
	}
}

func (Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idPath := tfpath.Root("id")
	pathPath := tfpath.Root("path")
	queryPath := tfpath.Root("query")
	headerPath := tfpath.Root("header")
	bodyPath := tfpath.Root("body")
	readSelector := tfpath.Root("read_selector")
	readResponseTemplate := tfpath.Root("read_response_template")

	var (
		imp ImportSpec
		err error
	)
	if req.ID != "" {
		err = json.Unmarshal([]byte(req.ID), &imp)

		// Ensure the identity is set and populated to the response
		resp.Identity.SetAttribute(ctx, idPath, req.ID)
	} else {
		var identity types.String
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, idPath, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		err = json.Unmarshal([]byte(identity.ValueString()), &imp)
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			fmt.Sprintf("failed to unmarshal ID: %v", err),
		)
		return
	}

	if imp.Id == "" {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			"`id` not specified in the import spec",
		)
		return
	}

	if imp.Path == "" {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			"`path` not specified in the import spec",
		)
		return
	}

	// Set the state to passthrough to the read
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, idPath, imp.Id)...)

	if imp.Body != nil {
		body, err := dynamic.FromJSONImplied(*imp.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Resource Import Error",
				fmt.Sprintf("unmarshal `body`: %v", err),
			)
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, bodyPath, body)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathPath, imp.Path)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, queryPath, imp.Query)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, headerPath, imp.Header)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, readSelector, imp.ReadSelector)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, readResponseTemplate, imp.ReadResponseTemplate)...)
}
