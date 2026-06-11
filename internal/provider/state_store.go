package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/statestore"
	ssschema "github.com/hashicorp/terraform-plugin-framework/statestore/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tidwall/gjson"
)

// StateStore is a generic restful state store. It allows users to configure
// REST API endpoints used for storing Terraform state, as well as managing
// state locks.
//
// Placeholders in the form `$(...)` can be used inside the configured `path`
// and `body` template strings. They are replaced at request time with the
// actual values supplied by Terraform / the framework. The supported
// placeholders are:
//
//	$(state_id)             - the workspace / state ID (e.g. "default")
//	$(lock_id)              - the lock ID (lock/unlock operations)
//	$(lock_info)            - the JSON-encoded LockInfo (lock operations)
//	$(lock_info.id)         - the lock UUID
//	$(lock_info.who)        - the actor that acquired the lock
//	$(lock_info.operation)  - the Terraform operation (plan / apply / ...)
//	$(lock_info.created)    - RFC3339-encoded creation time
type StateStore struct {
	p          *Provider
	stateCfg   stateStoreEndpointData
	lockCfg    *stateStoreLockData
	listConfig *stateStoreListData

	// in-memory tracker of locks issued during this provider session, so
	// Unlock can correlate the LockID with the LockInfo body that was
	// originally written to the lock endpoint.
	lockMu    sync.Mutex
	lockInfos map[string]statestore.LockInfo // keyed by stateID
}

var _ statestore.StateStore = &StateStore{}
var _ statestore.StateStoreWithConfigure = &StateStore{}

// --- Config types ---

type stateStoreData struct {
	State types.Object `tfsdk:"state"`
	Lock  types.Object `tfsdk:"lock"`
	List  types.Object `tfsdk:"list"`
}

type stateStoreEndpointData struct {
	Path         types.String `tfsdk:"path"`
	ReadMethod   types.String `tfsdk:"read_method"`
	WriteMethod  types.String `tfsdk:"write_method"`
	DeleteMethod types.String `tfsdk:"delete_method"`
	WriteBody    types.String `tfsdk:"write_body"`
	DeleteBody   types.String `tfsdk:"delete_body"`
	ReadSelector types.String `tfsdk:"read_selector"`

	Header types.Map `tfsdk:"header"`
	Query  types.Map `tfsdk:"query"`

	ReadHeader   types.Map `tfsdk:"read_header"`
	ReadQuery    types.Map `tfsdk:"read_query"`
	WriteHeader  types.Map `tfsdk:"write_header"`
	WriteQuery   types.Map `tfsdk:"write_query"`
	DeleteHeader types.Map `tfsdk:"delete_header"`
	DeleteQuery  types.Map `tfsdk:"delete_query"`
}

type stateStoreLockData struct {
	LockPath          types.String `tfsdk:"lock_path"`
	UnlockPath        types.String `tfsdk:"unlock_path"`
	LockMethod        types.String `tfsdk:"lock_method"`
	UnlockMethod      types.String `tfsdk:"unlock_method"`
	LockBody          types.String `tfsdk:"lock_body"`
	UnlockBody        types.String `tfsdk:"unlock_body"`
	LockConflictCodes types.List   `tfsdk:"lock_conflict_status_codes"`

	Header types.Map `tfsdk:"header"`
	Query  types.Map `tfsdk:"query"`

	LockHeader   types.Map `tfsdk:"lock_header"`
	LockQuery    types.Map `tfsdk:"lock_query"`
	UnlockHeader types.Map `tfsdk:"unlock_header"`
	UnlockQuery  types.Map `tfsdk:"unlock_query"`
}

type stateStoreListData struct {
	Path     types.String `tfsdk:"path"`
	Method   types.String `tfsdk:"method"`
	Selector types.String `tfsdk:"selector"`
	Header   types.Map    `tfsdk:"header"`
	Query    types.Map    `tfsdk:"query"`
}

// --- Metadata / Schema ---

func (s *StateStore) Metadata(_ context.Context, req statestore.MetadataRequest, resp *statestore.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint"
}

func (s *StateStore) Schema(_ context.Context, _ statestore.SchemaRequest, resp *statestore.SchemaResponse) {
	resp.Schema = ssschema.Schema{
		Description: "A generic restful state store. Persists Terraform state to a user defined HTTP endpoint, and optionally manages state locks via another endpoint. " +
			"`path` and `*_body` fields support `$(state_id)`, `$(lock_id)`, `$(lock_info)`, `$(lock_info.id)`, `$(lock_info.who)`, `$(lock_info.operation)` and `$(lock_info.created)` placeholders.",
		MarkdownDescription: "A generic restful state store. Persists Terraform state to a user defined HTTP endpoint, and optionally manages state locks via another endpoint.\n\n" +
			"`path` and `*_body` fields support the following placeholders, which are interpolated at request time:\n\n" +
			"- `$(state_id)` - The workspace / state ID. Defaults to `default`.\n" +
			"- `$(lock_id)` - The lock ID (lock/unlock).\n" +
			"- `$(lock_info)` - The full JSON encoded `LockInfo`.\n" +
			"- `$(lock_info.id)` / `$(lock_info.who)` / `$(lock_info.operation)` / `$(lock_info.created)` - Individual fields of the `LockInfo`.",
		Attributes: map[string]ssschema.Attribute{
			"state": ssschema.SingleNestedAttribute{
				Description:         "Endpoint configuration used to read/write/delete the Terraform state.",
				MarkdownDescription: "Endpoint configuration used to read/write/delete the Terraform state.",
				Required:            true,
				Attributes: map[string]ssschema.Attribute{
					"path": ssschema.StringAttribute{
						Description:         "Path template (relative to provider `base_url`) used for reading, writing and deleting the state. Supports `$(state_id)`.",
						MarkdownDescription: "Path template (relative to provider `base_url`) used for reading, writing and deleting the state. Supports `$(state_id)`.",
						Required:            true,
					},
					"read_method": ssschema.StringAttribute{
						Description:         "HTTP method used to read the state. Defaults to `GET`.",
						MarkdownDescription: "HTTP method used to read the state. Defaults to `GET`.",
						Optional:            true,
					},
					"write_method": ssschema.StringAttribute{
						Description:         "HTTP method used to write the state. Defaults to `PUT`.",
						MarkdownDescription: "HTTP method used to write the state. Defaults to `PUT`.",
						Optional:            true,
					},
					"delete_method": ssschema.StringAttribute{
						Description:         "HTTP method used to delete the state. Defaults to `DELETE`.",
						MarkdownDescription: "HTTP method used to delete the state. Defaults to `DELETE`.",
						Optional:            true,
					},
					"write_body": ssschema.StringAttribute{
						Description:         "Template for the request body sent when writing the state. The string `$(state_bytes)` is replaced with the (UTF-8) state bytes. If unset, the raw state bytes are sent as the body.",
						MarkdownDescription: "Template for the request body sent when writing the state. The string `$(state_bytes)` is replaced with the (UTF-8) state bytes. If unset, the raw state bytes are sent as the body.",
						Optional:            true,
					},
					"delete_body": ssschema.StringAttribute{
						Description:         "Optional template for the request body sent when deleting the state. Supports `$(state_id)`. If unset, no body is sent.",
						MarkdownDescription: "Optional template for the request body sent when deleting the state. Supports `$(state_id)`. If unset, no body is sent.",
						Optional:            true,
					},
					"read_selector": ssschema.StringAttribute{
						Description:         "Optional gjson selector applied to the response body of a Read. If set, the string located at this path is treated as the state content. If unset, the full response body is used.",
						MarkdownDescription: "Optional [gjson](https://github.com/tidwall/gjson) selector applied to the response body of a Read. If set, the string located at this path is treated as the state content. If unset, the full response body is used.",
						Optional:            true,
					},
					"header": ssschema.MapAttribute{
						Description: "Extra headers to send for all state requests. Per-operation `read_header` / `write_header` / `delete_header` replace it entirely when set.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for all state requests. Per-operation `read_query` / `write_query` / `delete_query` replace it entirely when set.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
					"read_header": ssschema.MapAttribute{
						Description: "Extra headers to send for state Read requests. If set, fully replaces `header` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"read_query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for state Read requests. If set, fully replaces `query` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
					"write_header": ssschema.MapAttribute{
						Description: "Extra headers to send for state Write requests. If set, fully replaces `header` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"write_query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for state Write requests. If set, fully replaces `query` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
					"delete_header": ssschema.MapAttribute{
						Description: "Extra headers to send for state Delete requests. If set, fully replaces `header` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"delete_query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for state Delete requests. If set, fully replaces `query` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
				},
			},
			"lock": ssschema.SingleNestedAttribute{
				Description:         "Endpoint configuration used to acquire and release state locks. If omitted, the state store reports that it does not support locking.",
				MarkdownDescription: "Endpoint configuration used to acquire and release state locks. If omitted, the state store reports that it does not support locking.",
				Optional:            true,
				Attributes: map[string]ssschema.Attribute{
					"lock_path": ssschema.StringAttribute{
						Description: "Path template used to acquire a lock. Supports `$(state_id)`, `$(lock_id)` and `$(lock_info.*)` placeholders.",
						Required:    true,
					},
					"unlock_path": ssschema.StringAttribute{
						Description: "Path template used to release a lock. Supports `$(state_id)`, `$(lock_id)` and `$(lock_info.*)` placeholders.",
						Required:    true,
					},
					"lock_method": ssschema.StringAttribute{
						Description: "HTTP method used to acquire a lock. Defaults to `POST`.",
						Optional:    true,
					},
					"unlock_method": ssschema.StringAttribute{
						Description: "HTTP method used to release a lock. Defaults to `DELETE`.",
						Optional:    true,
					},
					"lock_body": ssschema.StringAttribute{
						Description: "Optional body template for the lock request. Supports `$(lock_info)` and `$(lock_info.*)` placeholders. If unset, the JSON encoded LockInfo is sent.",
						Optional:    true,
					},
					"unlock_body": ssschema.StringAttribute{
						Description: "Optional body template for the unlock request. Supports `$(state_id)`, `$(lock_id)` and `$(lock_info.*)` placeholders. If unset, no body is sent.",
						Optional:    true,
					},
					"lock_conflict_status_codes": ssschema.ListAttribute{
						Description: "HTTP status codes that should be interpreted as 'lock already held'. Defaults to `[409, 423]`.",
						ElementType: types.Int64Type,
						Optional:    true,
					},
					"header": ssschema.MapAttribute{
						Description: "Extra headers to send for lock/unlock requests. Per-operation `lock_header` / `unlock_header` replace it entirely when set.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for lock/unlock requests. Per-operation `lock_query` / `unlock_query` replace it entirely when set.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
					"lock_header": ssschema.MapAttribute{
						Description: "Extra headers to send for Lock requests. If set, fully replaces `header` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"lock_query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for Lock requests. If set, fully replaces `query` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
					"unlock_header": ssschema.MapAttribute{
						Description: "Extra headers to send for Unlock requests. If set, fully replaces `header` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"unlock_query": ssschema.MapAttribute{
						Description: "Extra query parameters to send for Unlock requests. If set, fully replaces `query` for this operation (same precedence as `restful_operation` resource overrides). Supports `$(body.x.y)` exparam references against the request body.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
				},
			},
			"list": ssschema.SingleNestedAttribute{
				Description:         "Endpoint configuration used to list all known state IDs (workspaces). If omitted, the state store will only report the `default` workspace.",
				MarkdownDescription: "Endpoint configuration used to list all known state IDs (workspaces). If omitted, the state store will only report the `default` workspace.",
				Optional:            true,
				Attributes: map[string]ssschema.Attribute{
					"path": ssschema.StringAttribute{
						Description: "Path used to list known state IDs.",
						Required:    true,
					},
					"method": ssschema.StringAttribute{
						Description: "HTTP method used. Defaults to `GET`.",
						Optional:    true,
					},
					"selector": ssschema.StringAttribute{
						Description: "Optional gjson selector that returns the list of state IDs from the response. The selector must return a JSON array of strings. If unset, the response body is parsed as a JSON array of strings.",
						Optional:    true,
					},
					"header": ssschema.MapAttribute{
						Description: "Extra headers to send.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"query": ssschema.MapAttribute{
						Description: "Extra query parameters to send.",
						ElementType: types.ListType{ElemType: types.StringType},
						Optional:    true,
					},
				},
			},
		},
	}
}

// --- Initialize / Configure ---

func (s *StateStore) Initialize(ctx context.Context, req statestore.InitializeRequest, resp *statestore.InitializeResponse) {
	pd, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected state store provider data",
			fmt.Sprintf("Expected providerData, got: %T. Please report this issue.", req.ProviderData),
		)
		return
	}

	// Ensure the provider's HTTP client has been initialized.
	if diags := pd.provider.Init(ctx, pd.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	s.p = pd.provider

	var data stateStoreData
	if diags := req.Config.Get(ctx, &data); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if diags := data.State.As(ctx, &s.stateCfg, basetypes.ObjectAsOptions{}); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !data.Lock.IsNull() && !data.Lock.IsUnknown() {
		var l stateStoreLockData
		if diags := data.Lock.As(ctx, &l, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		s.lockCfg = &l
	}

	if !data.List.IsNull() && !data.List.IsUnknown() {
		var l stateStoreListData
		if diags := data.List.As(ctx, &l, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		s.listConfig = &l
	}

	s.lockInfos = map[string]statestore.LockInfo{}

	// Pass ourself along so Configure receives the populated struct.
	resp.StateStoreData = s
}

func (s *StateStore) Configure(_ context.Context, req statestore.ConfigureRequest, _ *statestore.ConfigureResponse) {
	if req.StateStoreData == nil {
		return
	}
	if other, ok := req.StateStoreData.(*StateStore); ok && other != s {
		// Re-use the populated state from Initialize.
		s.p = other.p
		s.stateCfg = other.stateCfg
		s.lockCfg = other.lockCfg
		s.listConfig = other.listConfig
		s.lockInfos = other.lockInfos
	}
}

// --- Placeholder expansion ---

// placeholderRe matches `$(key)` or `$(key.sub.sub)` style placeholders.
var placeholderRe = regexp.MustCompile(`\$\(([\w.]+)\)`)

func expandPlaceholders(s string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := placeholderRe.FindStringSubmatch(match)
		if v, ok := vars[sub[1]]; ok {
			return v
		}
		return match
	})
}

func placeholderVars(stateID string, lockID string, info *statestore.LockInfo, stateBytes []byte) map[string]string {
	m := map[string]string{
		"state_id": stateID,
	}
	if lockID != "" {
		m["lock_id"] = lockID
	}
	if info != nil {
		b, _ := json.Marshal(info)
		m["lock_info"] = string(b)
		m["lock_info.id"] = info.ID
		m["lock_info.who"] = info.Who
		m["lock_info.operation"] = info.Operation
		m["lock_info.created"] = info.Created.Format("2006-01-02T15:04:05Z07:00")
	}
	if stateBytes != nil {
		m["state_bytes"] = string(stateBytes)
	}
	return m
}

// --- HTTP plumbing ---

// callOp issues a single HTTP request using the provider's apiOption pipeline
// (apiOption.ForOperation + Client.Operation), so that the request composition
// follows exactly the same precedence and exparam interpolation rules as the
// `restful_operation` resource:
//
//  1. Start from the provider-wide `query`/`header`.
//  2. Replace them with `defHeader`/`defQuery` (the state-store level
//     `state.header`/`state.query` or `lock.header`/`lock.query`) if those
//     are non-empty.
//  3. Replace them again with the operation-specific `ovHeader`/`ovQuery`
//     (e.g. `read_header`/`write_query`/`lock_header`/...) if those are
//     non-empty.
//
// This matches client.Query.TakeOrSelf semantics used elsewhere in the
// provider.
func (s *StateStore) callOp(
	ctx context.Context,
	method, path string,
	defHeader, defQuery, ovHeader, ovQuery basetypes.MapValue,
	body []byte,
) (*resty.Response, error) {
	opt, diags := s.p.apiOpt.ForOperation(
		ctx,
		basetypes.NewStringValue(method),
		defQuery, defHeader,
		ovQuery, ovHeader,
		body,
	)
	if diags.HasError() {
		return nil, fmt.Errorf("composing request options: %s", diags)
	}
	return s.p.client.Operation(ctx, path, body, *opt)
}

func defaultStr(v types.String, def string) string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return def
	}
	return v.ValueString()
}

// --- Read / Write / Delete state ---

func (s *StateStore) Read(ctx context.Context, req statestore.ReadRequest, resp *statestore.ReadResponse) {
	s.p.client.SetLoggerContext(ctx)

	vars := placeholderVars(req.StateID, "", nil, nil)
	path := expandPlaceholders(s.stateCfg.Path.ValueString(), vars)
	method := defaultStr(s.stateCfg.ReadMethod, "GET")

	tflog.Debug(ctx, "state store: reading state", map[string]any{"path": path, "method": method, "state_id": req.StateID})
	httpResp, err := s.callOp(ctx, method, path,
		s.stateCfg.Header, s.stateCfg.Query,
		s.stateCfg.ReadHeader, s.stateCfg.ReadQuery,
		nil)
	if err != nil {
		resp.Diagnostics.AddError("State Store Read failed", err.Error())
		return
	}

	switch httpResp.StatusCode() {
	case http.StatusNotFound, http.StatusNoContent:
		// No state yet.
		return
	}
	if httpResp.IsError() {
		resp.Diagnostics.AddError("State Store Read failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}

	body := httpResp.Body()
	if sel := s.stateCfg.ReadSelector.ValueString(); sel != "" {
		r := gjson.GetBytes(body, sel)
		if !r.Exists() {
			resp.Diagnostics.AddError("State Store Read failed",
				fmt.Sprintf("Response did not contain a value at selector %q. Body: %s", sel, string(body)))
			return
		}
		body = []byte(r.String())
	}
	resp.StateBytes = body
}

func (s *StateStore) Write(ctx context.Context, req statestore.WriteRequest, resp *statestore.WriteResponse) {
	s.p.client.SetLoggerContext(ctx)

	vars := placeholderVars(req.StateID, "", nil, req.StateBytes)
	path := expandPlaceholders(s.stateCfg.Path.ValueString(), vars)
	method := defaultStr(s.stateCfg.WriteMethod, "PUT")

	var body []byte
	if tpl := s.stateCfg.WriteBody.ValueString(); tpl != "" {
		body = []byte(expandPlaceholders(tpl, vars))
	} else {
		body = req.StateBytes
	}

	tflog.Debug(ctx, "state store: writing state", map[string]any{"path": path, "method": method, "state_id": req.StateID, "bytes": len(req.StateBytes)})
	httpResp, err := s.callOp(ctx, method, path,
		s.stateCfg.Header, s.stateCfg.Query,
		s.stateCfg.WriteHeader, s.stateCfg.WriteQuery,
		body)
	if err != nil {
		resp.Diagnostics.AddError("State Store Write failed", err.Error())
		return
	}
	if httpResp.IsError() {
		resp.Diagnostics.AddError("State Store Write failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}
}

func (s *StateStore) DeleteState(ctx context.Context, req statestore.DeleteStateRequest, resp *statestore.DeleteStateResponse) {
	s.p.client.SetLoggerContext(ctx)

	vars := placeholderVars(req.StateID, "", nil, nil)
	path := expandPlaceholders(s.stateCfg.Path.ValueString(), vars)
	method := defaultStr(s.stateCfg.DeleteMethod, "DELETE")

	var body []byte
	if tpl := s.stateCfg.DeleteBody.ValueString(); tpl != "" {
		body = []byte(expandPlaceholders(tpl, vars))
	}

	httpResp, err := s.callOp(ctx, method, path,
		s.stateCfg.Header, s.stateCfg.Query,
		s.stateCfg.DeleteHeader, s.stateCfg.DeleteQuery,
		body)
	if err != nil {
		resp.Diagnostics.AddError("State Store DeleteState failed", err.Error())
		return
	}
	if httpResp.IsError() && httpResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError("State Store DeleteState failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}
}

// --- Lock / Unlock ---

func (s *StateStore) Lock(ctx context.Context, req statestore.LockRequest, resp *statestore.LockResponse) {
	if s.lockCfg == nil {
		// Locking not supported in this configuration.
		return
	}
	s.p.client.SetLoggerContext(ctx)

	info := statestore.NewLockInfo(req)
	vars := placeholderVars(req.StateID, info.ID, &info, nil)

	path := expandPlaceholders(s.lockCfg.LockPath.ValueString(), vars)
	method := defaultStr(s.lockCfg.LockMethod, "POST")

	var body []byte
	if tpl := s.lockCfg.LockBody.ValueString(); tpl != "" {
		body = []byte(expandPlaceholders(tpl, vars))
	} else {
		body, _ = json.Marshal(info)
	}

	tflog.Debug(ctx, "state store: acquiring lock", map[string]any{"path": path, "method": method, "state_id": req.StateID, "lock_id": info.ID})
	httpResp, err := s.callOp(ctx, method, path,
		s.lockCfg.Header, s.lockCfg.Query,
		s.lockCfg.LockHeader, s.lockCfg.LockQuery,
		body)
	if err != nil {
		resp.Diagnostics.AddError("State Store Lock failed", err.Error())
		return
	}

	conflict := []int64{http.StatusConflict, http.StatusLocked}
	if !s.lockCfg.LockConflictCodes.IsNull() && !s.lockCfg.LockConflictCodes.IsUnknown() {
		conflict = conflict[:0]
		for _, e := range s.lockCfg.LockConflictCodes.Elements() {
			if iv, ok := e.(types.Int64); ok && !iv.IsNull() {
				conflict = append(conflict, iv.ValueInt64())
			}
		}
	}

	for _, c := range conflict {
		if int64(httpResp.StatusCode()) == c {
			existing := statestore.LockInfo{}
			_ = json.Unmarshal(httpResp.Body(), &existing)
			resp.Diagnostics.Append(statestore.WorkspaceAlreadyLockedDiagnostic(req, existing))
			return
		}
	}

	if httpResp.IsError() {
		resp.Diagnostics.AddError("State Store Lock failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}

	s.lockMu.Lock()
	s.lockInfos[req.StateID] = info
	s.lockMu.Unlock()

	resp.LockID = info.ID
}

func (s *StateStore) Unlock(ctx context.Context, req statestore.UnlockRequest, resp *statestore.UnlockResponse) {
	if s.lockCfg == nil {
		return
	}
	s.p.client.SetLoggerContext(ctx)

	s.lockMu.Lock()
	info, hasInfo := s.lockInfos[req.StateID]
	s.lockMu.Unlock()

	var infoPtr *statestore.LockInfo
	if hasInfo {
		infoPtr = &info
	}

	vars := placeholderVars(req.StateID, req.LockID, infoPtr, nil)
	path := expandPlaceholders(s.lockCfg.UnlockPath.ValueString(), vars)
	method := defaultStr(s.lockCfg.UnlockMethod, "DELETE")

	var body []byte
	if tpl := s.lockCfg.UnlockBody.ValueString(); tpl != "" {
		body = []byte(expandPlaceholders(tpl, vars))
	}

	httpResp, err := s.callOp(ctx, method, path,
		s.lockCfg.Header, s.lockCfg.Query,
		s.lockCfg.UnlockHeader, s.lockCfg.UnlockQuery,
		body)
	if err != nil {
		resp.Diagnostics.AddError("State Store Unlock failed", err.Error())
		return
	}
	if httpResp.IsError() && httpResp.StatusCode() != http.StatusNotFound {
		resp.Diagnostics.AddError("State Store Unlock failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}

	s.lockMu.Lock()
	delete(s.lockInfos, req.StateID)
	s.lockMu.Unlock()
}

// --- GetStates ---

func (s *StateStore) GetStates(ctx context.Context, _ statestore.GetStatesRequest, resp *statestore.GetStatesResponse) {
	if s.listConfig == nil {
		resp.StateIDs = []string{"default"}
		return
	}
	s.p.client.SetLoggerContext(ctx)

	method := defaultStr(s.listConfig.Method, "GET")
	httpResp, err := s.callOp(ctx, method, s.listConfig.Path.ValueString(),
		s.listConfig.Header, s.listConfig.Query,
		basetypes.NewMapNull(types.StringType),
		basetypes.NewMapNull(types.ListType{ElemType: types.StringType}),
		nil)
	if err != nil {
		resp.Diagnostics.AddError("State Store GetStates failed", err.Error())
		return
	}
	if httpResp.IsError() {
		resp.Diagnostics.AddError("State Store GetStates failed",
			fmt.Sprintf("Unexpected status %d: %s", httpResp.StatusCode(), httpResp.String()))
		return
	}

	body := httpResp.Body()
	var result gjson.Result
	if sel := s.listConfig.Selector.ValueString(); sel != "" {
		result = gjson.GetBytes(body, sel)
	} else {
		result = gjson.ParseBytes(body)
	}
	if !result.IsArray() {
		resp.Diagnostics.AddError("State Store GetStates failed",
			fmt.Sprintf("Expected a JSON array, got: %s", result.Raw))
		return
	}

	var ids []string
	result.ForEach(func(_, v gjson.Result) bool {
		ids = append(ids, v.String())
		return true
	})
	if len(ids) == 0 {
		ids = []string{"default"}
	}
	resp.StateIDs = ids
}
