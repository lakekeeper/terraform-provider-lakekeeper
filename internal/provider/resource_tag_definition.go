package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ resource.Resource                   = &lakekeeperTagDefinitionResource{}
	_ resource.ResourceWithConfigure      = &lakekeeperTagDefinitionResource{}
	_ resource.ResourceWithImportState    = &lakekeeperTagDefinitionResource{}
	_ resource.ResourceWithValidateConfig = &lakekeeperTagDefinitionResource{}
)

func init() {
	registerResource(NewLakekeeperTagDefinitionResource)
}

// NewLakekeeperTagDefinitionResource returns a resource for managing governance tag definitions.
func NewLakekeeperTagDefinitionResource() resource.Resource {
	return &lakekeeperTagDefinitionResource{}
}

type lakekeeperTagDefinitionResource struct {
	client *api.Client
}

// LakekeeperTagDefinitionResourceModel describes the Terraform state for lakekeeper_tag_definition.
type LakekeeperTagDefinitionResourceModel struct {
	ID              types.String `tfsdk:"id"`
	TagDefinitionID types.String `tfsdk:"tag_definition_id"`
	ProjectID       types.String `tfsdk:"project_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Scope           types.Set    `tfsdk:"scope"`
	ValueKind       types.String `tfsdk:"value_kind"`
	AllowedValues   types.Set    `tfsdk:"allowed_values"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

func (r *lakekeeperTagDefinitionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_definition"
}

func (r *lakekeeperTagDefinitionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_tag_definition`" + ` resource manages a governance tag definition in a Lakekeeper project.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag)`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the tag definition, in the form `{{project_id}}/{{tag_definition_id}}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tag_definition_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the tag definition.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project the tag definition belongs to. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the tag definition. `.` is the hierarchy delimiter (for example `pii.classification`). Unique within the project, case-insensitively.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the tag definition.",
				Optional:            true,
			},
			"scope": schema.SetAttribute{
				MarkdownDescription: "Target types this tag definition may be applied to. Valid values: `warehouse`, `namespace`, `table`, `view`, `generic-table`, `column`. Existing scopes cannot be removed on update.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(
						api.TagScopeWarehouse,
						api.TagScopeNamespace,
						api.TagScopeTable,
						api.TagScopeView,
						api.TagScopeGenericTable,
						api.TagScopeColumn,
					)),
				},
				PlanModifiers: []planmodifier.Set{
					widenOnlySetModifier("scope", "Lakekeeper only allows widening the scope of an existing tag definition; existing scopes cannot be removed."),
				},
			},
			"value_kind": schema.StringAttribute{
				MarkdownDescription: "How values of this tag definition are constrained: `marker`, `free-text`, or `enumerated`. Changing this forces a new resource to be created.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(api.TagValueKindMarker, api.TagValueKindFreeText, api.TagValueKindEnumerated),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allowed_values": schema.SetAttribute{
				MarkdownDescription: "Permitted values for an `enumerated` tag definition. Required and non-empty when `value_kind` is `enumerated`; must be omitted otherwise. Existing values cannot be removed on update.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
				PlanModifiers: []planmodifier.Set{
					widenOnlySetModifier("allowed_values", "Lakekeeper only allows adding allowed_values to an existing enumerated tag definition; existing values cannot be removed."),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the tag definition was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the tag definition was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *lakekeeperTagDefinitionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*LakekeeperResourceData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *LakekeeperResourceData, got: %T.", req.ProviderData))
		return
	}
	r.client = data.Client
}

func (r *lakekeeperTagDefinitionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg LakekeeperTagDefinitionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ValueKind.IsUnknown() || cfg.ValueKind.IsNull() {
		return
	}
	kind := cfg.ValueKind.ValueString()
	if kind == api.TagValueKindEnumerated {
		if cfg.AllowedValues.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("allowed_values"),
				"Missing allowed_values for enumerated tag definition",
				"`allowed_values` is required and must be non-empty when `value_kind` is `enumerated`.",
			)
			return
		}
		if !cfg.AllowedValues.IsUnknown() && len(cfg.AllowedValues.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("allowed_values"),
				"Empty allowed_values for enumerated tag definition",
				"`allowed_values` must contain at least one value when `value_kind` is `enumerated`.",
			)
		}
		return
	}

	if !cfg.AllowedValues.IsNull() && !cfg.AllowedValues.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("allowed_values"),
			"Unexpected allowed_values",
			fmt.Sprintf("`allowed_values` is only valid when `value_kind` is `enumerated` (got %q).", kind),
		)
	}
}

func (r *lakekeeperTagDefinitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LakekeeperTagDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope, diags := stringSetElements(ctx, plan.Scope)
	resp.Diagnostics.Append(diags...)
	allowedValues, diags := stringSetElements(ctx, plan.AllowedValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var desc *string
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		desc = new(plan.Description.ValueString())
	}

	td, err := r.client.CreateTagDefinition(ctx, plan.ProjectID.ValueString(), api.CreateTagDefinitionRequest{
		Name:          plan.Name.ValueString(),
		Description:   desc,
		Scope:         scope,
		ValueKind:     plan.ValueKind.ValueString(),
		AllowedValues: allowedValues,
	})
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to create tag definition: "+err.Error())
		return
	}

	applyTagDefinitionToModel(&plan, plan.ProjectID.ValueString(), td)
	tflog.Trace(ctx, "created tag definition", map[string]any{"id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lakekeeperTagDefinitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LakekeeperTagDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, tagDefID, err := splitInternalID(state.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid internal ID", err.Error())
		return
	}

	td, err := r.client.GetTagDefinition(ctx, projectID, tagDefID)
	if err != nil {
		if api.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to read tag definition: "+err.Error())
		return
	}

	applyTagDefinitionToModel(&state, projectID, td)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lakekeeperTagDefinitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state LakekeeperTagDefinitionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, tagDefID, err := splitInternalID(state.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid internal ID", err.Error())
		return
	}

	scope, diags := stringSetElements(ctx, plan.Scope)
	resp.Diagnostics.Append(diags...)
	planAllowed, diags := stringSetElements(ctx, plan.AllowedValues)
	resp.Diagnostics.Append(diags...)
	stateAllowed, diags := stringSetElements(ctx, state.AllowedValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing := make(map[string]struct{}, len(stateAllowed))
	for _, v := range stateAllowed {
		existing[v] = struct{}{}
	}
	var addAllowed []string
	for _, v := range planAllowed {
		if _, ok := existing[v]; !ok {
			addAllowed = append(addAllowed, v)
		}
	}

	var desc *string
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		desc = new(plan.Description.ValueString())
	}

	td, err := r.client.UpdateTagDefinition(ctx, projectID, tagDefID, api.UpdateTagDefinitionRequest{
		Name:             plan.Name.ValueString(),
		Description:      desc,
		Scope:            scope,
		AddAllowedValues: addAllowed,
	})
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to update tag definition: "+err.Error())
		return
	}

	applyTagDefinitionToModel(&plan, projectID, td)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *lakekeeperTagDefinitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LakekeeperTagDefinitionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, tagDefID, err := splitInternalID(state.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid internal ID", err.Error())
		return
	}

	if err := r.client.DeleteTagDefinition(ctx, projectID, tagDefID); err != nil && !api.IsNotFound(err) {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to delete tag definition: "+err.Error())
	}
}

func (r *lakekeeperTagDefinitionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, tagDefID, err := splitInternalID(types.StringValue(req.ID))
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID format", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_definition_id"), tagDefID)...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyTagDefinitionToModel(m *LakekeeperTagDefinitionResourceModel, projectID string, td *api.TagDefinition) {
	m.ID = types.StringValue(fmt.Sprintf("%s/%s", projectID, td.ID))
	m.TagDefinitionID = types.StringValue(td.ID)
	m.ProjectID = types.StringValue(projectID)
	m.Name = types.StringValue(td.Name)
	m.Description = types.StringPointerValue(td.Description)
	m.Scope = stringSetValue(td.Scope)
	m.ValueKind = types.StringValue(td.ValueKind)
	if len(td.AllowedValues) > 0 {
		m.AllowedValues = stringSetValue(td.AllowedValues)
	} else {
		m.AllowedValues = types.SetNull(types.StringType)
	}
	m.CreatedAt = types.StringValue(td.CreatedAt)
	m.UpdatedAt = types.StringPointerValue(td.UpdatedAt)
}

func stringSetValue(vals []string) types.Set {
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, elems)
}

func stringSetElements(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var out []string
	diags := s.ElementsAs(ctx, &out, false)
	return out, diags
}

type widenOnlySetPlanModifier struct {
	attrName string
	message  string
}

func widenOnlySetModifier(attrName, message string) planmodifier.Set {
	return widenOnlySetPlanModifier{attrName: attrName, message: message}
}

func (m widenOnlySetPlanModifier) Description(_ context.Context) string { return m.message }
func (m widenOnlySetPlanModifier) MarkdownDescription(_ context.Context) string {
	return m.message
}

func (m widenOnlySetPlanModifier) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	if req.State.Raw.IsNull() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if req.PlanValue.IsUnknown() {
		return
	}
	stateElems, diags := stringSetElements(ctx, req.StateValue)
	resp.Diagnostics.Append(diags...)
	planElems, diags := stringSetElements(ctx, req.PlanValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	planned := make(map[string]struct{}, len(planElems))
	for _, v := range planElems {
		planned[v] = struct{}{}
	}
	for _, v := range stateElems {
		if _, ok := planned[v]; !ok {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				fmt.Sprintf("Cannot remove value %q from %s", v, m.attrName),
				m.message,
			)
			return
		}
	}
}
