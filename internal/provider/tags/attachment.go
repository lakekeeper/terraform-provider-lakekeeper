// Package tags implements the shared Terraform resource logic for attaching
// governance tags to Lakekeeper targets (warehouses, namespaces, tables,
// views, generic tables, and columns).
package tags

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// ParentAttr describes one parent identifier attribute of an attachment resource.
type ParentAttr struct {
	Name        string
	Description string
	Validators  []validator.String
}

// Spec parameterizes a tag attachment resource for a specific target kind.
type Spec struct {
	TypeName     string
	TargetNoun   string
	OperationDoc string
	Parents      []ParentAttr
	Set          func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error)
	List         func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error)
	Delete       func(ctx context.Context, client *api.Client, parents []string, tagName string) error
}

// NewResource builds a Terraform resource from spec.
func NewResource(spec Spec, extractClient func(any) (*api.Client, error)) resource.Resource {
	return &attachmentResource{spec: spec, extractClient: extractClient}
}

type attachmentResource struct {
	spec          Spec
	extractClient func(any) (*api.Client, error)
	client        *api.Client
}

var (
	_ resource.Resource                = &attachmentResource{}
	_ resource.ResourceWithConfigure   = &attachmentResource{}
	_ resource.ResourceWithImportState = &attachmentResource{}
)

func (r *attachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.TypeName
}

func (r *attachmentResource) idFormatDescription() string {
	parts := make([]string, 0, len(r.spec.Parents)+1)
	for _, p := range r.spec.Parents {
		parts = append(parts, "{{"+p.Name+"}}")
	}
	parts = append(parts, "{{tag_name}}")
	return "`" + strings.Join(parts, "/") + "`"
}

func (r *attachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: "The ID of the tag attachment, in the form " + r.idFormatDescription() + ".",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"tag_name": schema.StringAttribute{
			MarkdownDescription: "The name of the tag definition to attach. Changing this forces a new resource to be created.",
			Required:            true,
			Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"value": schema.StringAttribute{
			MarkdownDescription: "The tag value. Required for `free-text` and `enumerated` tag definitions; must be omitted for `marker` tag definitions.",
			Optional:            true,
		},
		"tag_definition_id": schema.StringAttribute{
			MarkdownDescription: "The UUID of the attached tag definition.",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"applied_at": schema.StringAttribute{
			MarkdownDescription: "Timestamp when the tag was applied or last updated.",
			Computed:            true,
		},
	}

	for _, p := range r.spec.Parents {
		vals := p.Validators
		if len(vals) == 0 {
			vals = []validator.String{stringvalidator.LengthAtLeast(1)}
		}
		attrs[p.Name] = schema.StringAttribute{
			MarkdownDescription: p.Description + " Changing this forces a new resource to be created.",
			Required:            true,
			Validators:          vals,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The `+"`lakekeeper_%s`"+` resource attaches a governance tag to a Lakekeeper %s.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag/operation/%s)`, r.spec.TypeName, r.spec.TargetNoun, r.spec.OperationDoc),
		Attributes: attrs,
	}
}

func (r *attachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, err := r.extractClient(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", err.Error())
		return
	}
	r.client = client
}

func (r *attachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.upsert(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *attachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.upsert(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *attachmentResource) upsert(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	parents, tagName, value, ok := r.readPlanAttributes(ctx, plan, diags)
	if !ok {
		return
	}

	applied, err := r.spec.Set(ctx, r.client, parents, tagName, value)
	if err != nil {
		diags.AddError("Lakekeeper API error occurred", formatSetTagError(tagName, value, err))
		return
	}

	id := strings.Join(append(append([]string{}, parents...), tagName), "/")
	r.writeAttributes(ctx, state, diags, id, parents, applied.Name, applied.Value, applied.TagDefinitionID, applied.AppliedAt)
}

func (r *attachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var idVal types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &idVal)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parents, tagName, err := parseAttachmentID(idVal.ValueString(), len(r.spec.Parents))
	if err != nil {
		resp.Diagnostics.AddError("Invalid internal ID", err.Error())
		return
	}

	tags, err := r.spec.List(ctx, r.client, parents)
	if err != nil {
		if api.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to read tags: "+err.Error())
		return
	}

	for _, t := range tags {
		if strings.EqualFold(t.Name, tagName) {
			appliedAt := t.CreatedAt
			if t.UpdatedAt != nil {
				appliedAt = *t.UpdatedAt
			}
			r.writeAttributes(ctx, &resp.State, &resp.Diagnostics, idVal.ValueString(), parents, tagName, t.Value, t.TagDefinitionID, appliedAt)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *attachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var idVal types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &idVal)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parents, tagName, err := parseAttachmentID(idVal.ValueString(), len(r.spec.Parents))
	if err != nil {
		resp.Diagnostics.AddError("Invalid internal ID", err.Error())
		return
	}

	if err := r.spec.Delete(ctx, r.client, parents, tagName); err != nil && !api.IsNotFound(err) {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to delete tag attachment: "+err.Error())
	}
}

func (r *attachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parents, tagName, err := parseAttachmentID(req.ID, len(r.spec.Parents))
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID format", err.Error())
		return
	}
	for i, p := range r.spec.Parents {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(p.Name), parents[i])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_name"), tagName)...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *attachmentResource) readPlanAttributes(ctx context.Context, plan tfsdk.Plan, diags *diag.Diagnostics) ([]string, string, *string, bool) {
	parents := make([]string, len(r.spec.Parents))
	for i, p := range r.spec.Parents {
		var v types.String
		diags.Append(plan.GetAttribute(ctx, path.Root(p.Name), &v)...)
		parents[i] = v.ValueString()
	}
	var tagName, value types.String
	diags.Append(plan.GetAttribute(ctx, path.Root("tag_name"), &tagName)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("value"), &value)...)
	if diags.HasError() {
		return nil, "", nil, false
	}
	var valPtr *string
	if !value.IsNull() && !value.IsUnknown() {
		valPtr = new(value.ValueString())
	}
	return parents, tagName.ValueString(), valPtr, true
}

func (r *attachmentResource) writeAttributes(
	ctx context.Context,
	state *tfsdk.State,
	diags *diag.Diagnostics,
	id string,
	parents []string,
	tagName string,
	value *string,
	tagDefID string,
	appliedAt string,
) {
	diags.Append(state.SetAttribute(ctx, path.Root("id"), id)...)
	for i, p := range r.spec.Parents {
		diags.Append(state.SetAttribute(ctx, path.Root(p.Name), parents[i])...)
	}
	diags.Append(state.SetAttribute(ctx, path.Root("tag_name"), tagName)...)
	diags.Append(state.SetAttribute(ctx, path.Root("value"), types.StringPointerValue(value))...)
	diags.Append(state.SetAttribute(ctx, path.Root("tag_definition_id"), tagDefID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("applied_at"), appliedAt)...)
}

func parseAttachmentID(id string, parentCount int) ([]string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != parentCount+1 {
		return nil, "", fmt.Errorf("invalid internal ID %q: expected %d '/'-separated segments", id, parentCount+1)
	}
	for _, p := range parts {
		if p == "" {
			return nil, "", fmt.Errorf("invalid internal ID %q: segments must not be empty", id)
		}
	}
	return parts[:parentCount], parts[parentCount], nil
}

func formatSetTagError(tagName string, value *string, err error) string {
	code := api.StatusCode(err)
	if code == http.StatusBadRequest || code == http.StatusUnprocessableEntity {
		if value == nil {
			return fmt.Sprintf("Unable to attach tag %q without a value (note: `value` is required for `free-text` and `enumerated` tag definitions, and must be omitted for `marker` definitions): %s", tagName, err.Error())
		}
		return fmt.Sprintf("Unable to attach tag %q with value %q (note: `value` must be omitted for `marker` tag definitions and must be one of `allowed_values` for `enumerated` definitions): %s", tagName, *value, err.Error())
	}
	return fmt.Sprintf("Unable to attach tag %q: %s", tagName, err.Error())
}
