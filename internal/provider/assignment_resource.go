package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
)

var (
	_ resource.Resource                = &assignmentResource{}
	_ resource.ResourceWithConfigure   = &assignmentResource{}
	_ resource.ResourceWithImportState = &assignmentResource{}
)

// newAssignmentResource returns a resource constructor for registerResource.
func newAssignmentResource(d assignmentDescriptor) func() resource.Resource {
	return func() resource.Resource { return &assignmentResource{d: d} }
}

// assignmentResource is the shared assignment module: one Terraform resource
// implementation serving every assignment resource, driven by its descriptor.
//
// Every create, update and delete reads the assignee's current direct grants at
// the scope and sends the difference to the desired set as one request.
type assignmentResource struct {
	d       assignmentDescriptor
	adapter scopeAdapter
}

// assignmentTarget is one assignee at one scope, read from a plan or state.
type assignmentTarget struct {
	values   []string // identity values in ID order; the assignee is last
	scope    assignmentScope
	assignee string
}

func (r *assignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.d.TypeName
}

func (r *assignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schema()
}

func (r *assignmentResource) schema() schema.Schema {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			MarkdownDescription: fmt.Sprintf("The internal ID of this resource, in the form `%s`.", r.d.idFormat()),
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"assignments": schema.SetAttribute{
			ElementType: types.StringType,
			MarkdownDescription: fmt.Sprintf("The direct grants of the %s on the %s. Allowed values: %s. An empty set revokes all of them.",
				r.d.AssigneeKind, r.d.ScopeNoun, quotedList(r.d.GrantTypes)),
			Required:   true,
			Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(r.d.GrantTypes...))},
		},
	}
	for _, a := range r.d.identity() {
		attrs[a.Name] = schema.StringAttribute{
			MarkdownDescription: a.Description,
			Required:            true,
			Validators:          a.Validators,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		}
	}
	return schema.Schema{MarkdownDescription: r.description(), Attributes: attrs}
}

// description is the one template every assignment resource's description comes from.
func (r *assignmentResource) description() string {
	kind, scope := r.d.AssigneeKind, r.d.ScopeNoun
	text := fmt.Sprintf("Manages the direct grants of one %[1]s on the %[2]s. "+
		"Grants that other users and roles hold on the %[2]s are left untouched. Use one resource per %[2]s and %[1]s.\n\n"+
		"Create fails if the %[1]s already holds any direct grant on the %[2]s. "+
		"Bring existing grants under Terraform with `terraform import` first, using the ID `%[3]s`.\n\n"+
		"Every create, update and delete compares the desired grants with the grants Lakekeeper holds, "+
		"and sends the difference as one request that Lakekeeper applies atomically. "+
		"Grants changed outside Terraform are corrected on the next apply. "+
		"Failed reads, writes and deletes report an error and keep the resource in state.",
		kind, scope, r.d.idFormat())
	if r.d.Notes != "" {
		text += "\n\n" + r.d.Notes
	}
	return text + fmt.Sprintf("\n\n**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/update_%s_assignments)", scope)
}

func (r *assignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.adapter = r.d.NewAdapter(req.ProviderData.(*LakekeeperResourceData).Client)
}

func (r *assignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	target, desired, diags := r.readModel(ctx, req.Plan.GetAttribute)
	if resp.Diagnostics.Append(diags...); resp.Diagnostics.HasError() {
		return
	}
	current, err := r.current(ctx, target)
	if err != nil {
		resp.Diagnostics.AddError(r.errorSummary("read"), err.Error())
		return
	}
	if len(current) > 0 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("The %s already holds grants on the %s", r.d.AssigneeKind, r.d.ScopeNoun),
			fmt.Sprintf("The %s already holds direct grants on the %s: %s. "+
				"To manage them with Terraform, import the assignment first: terraform import <resource address> %q",
				r.d.AssigneeKind, r.d.ScopeNoun, quotedList(current), r.d.buildID(target.values)),
		)
		return
	}
	// Store the ID before writing, so a lost response still leaves the resource tracked.
	resp.State.Raw = req.Plan.Raw.Copy()
	if resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), r.d.buildID(target.values))...); resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, target, current, desired); err != nil {
		resp.Diagnostics.AddError(r.errorSummary("create"), err.Error())
	}
}

func (r *assignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	target, _, diags := r.readModel(ctx, req.State.GetAttribute)
	if resp.Diagnostics.Append(diags...); resp.Diagnostics.HasError() {
		return
	}
	current, err := r.current(ctx, target)
	if err != nil {
		resp.Diagnostics.AddError(r.errorSummary("read"), err.Error())
		return
	}
	resp.Diagnostics.Append(r.setGrants(ctx, &resp.State, current)...)
}

func (r *assignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	target, desired, diags := r.readModel(ctx, req.Plan.GetAttribute)
	if resp.Diagnostics.Append(diags...); resp.Diagnostics.HasError() {
		return
	}
	if err := r.reconcile(ctx, target, desired); err != nil {
		resp.Diagnostics.AddError(r.errorSummary("update"), err.Error())
		return
	}
	resp.State.Raw = req.Plan.Raw.Copy()
}

func (r *assignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	target, _, diags := r.readModel(ctx, req.State.GetAttribute)
	if resp.Diagnostics.Append(diags...); resp.Diagnostics.HasError() {
		return
	}
	// On failure the state is kept, so the next apply retries the revoke.
	if err := r.reconcile(ctx, target, nil); err != nil {
		resp.Diagnostics.AddError(r.errorSummary("delete"), err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *assignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	values, err := r.d.parseID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	for i, a := range r.d.identity() {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a.Name), values[i])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// readModel reads the identity attributes and the desired grants from a plan or state.
func (r *assignmentResource) readModel(
	ctx context.Context,
	getAttribute func(context.Context, path.Path, any) diag.Diagnostics,
) (assignmentTarget, []string, diag.Diagnostics) {
	var diags diag.Diagnostics
	target := assignmentTarget{scope: assignmentScope{}}
	attrs := r.d.identity()
	for i, a := range attrs {
		var v types.String
		diags.Append(getAttribute(ctx, path.Root(a.Name), &v)...)
		target.values = append(target.values, v.ValueString())
		if i < len(attrs)-1 {
			target.scope[a.Name] = v.ValueString()
		}
	}
	target.assignee = target.values[len(target.values)-1]

	var set types.Set
	diags.Append(getAttribute(ctx, path.Root("assignments"), &set)...)
	var desired []string
	if !set.IsNull() && !set.IsUnknown() {
		diags.Append(set.ElementsAs(ctx, &desired, false)...)
	}
	return target, desired, diags
}

// current lists the direct grants at the scope and keeps only this assignee's
// grant types, sorted and without duplicates. The server can't filter by assignee.
func (r *assignmentResource) current(ctx context.Context, target assignmentTarget) ([]string, error) {
	grants, err := r.adapter.ListGrants(ctx, target.scope)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, g := range grants {
		if g.AssigneeKind == r.d.AssigneeKind && g.AssigneeID == target.assignee {
			out = append(out, g.Type)
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// reconcile brings the assignee's direct grants at the scope to the desired set.
// A failed read never leads to a write.
func (r *assignmentResource) reconcile(ctx context.Context, target assignmentTarget, desired []string) error {
	current, err := r.current(ctx, target)
	if err != nil {
		return err
	}
	return r.apply(ctx, target, current, desired)
}

// apply sends revokes and grants from current to desired in one request, or nothing if they match.
func (r *assignmentResource) apply(ctx context.Context, target assignmentTarget, current, desired []string) error {
	var revokes, grants []assignmentGrant
	for _, t := range current {
		if !slices.Contains(desired, t) {
			revokes = append(revokes, r.grant(target, t))
		}
	}
	wanted := slices.Clone(desired)
	slices.Sort(wanted)
	for _, t := range slices.Compact(wanted) {
		if !slices.Contains(current, t) {
			grants = append(grants, r.grant(target, t))
		}
	}
	if len(revokes) == 0 && len(grants) == 0 {
		return nil
	}
	return r.adapter.ApplyGrants(ctx, target.scope, revokes, grants)
}

func (r *assignmentResource) grant(target assignmentTarget, grantType string) assignmentGrant {
	return assignmentGrant{AssigneeKind: r.d.AssigneeKind, AssigneeID: target.assignee, Type: grantType}
}

func (r *assignmentResource) setGrants(ctx context.Context, state *tfsdk.State, grants []string) diag.Diagnostics {
	set, diags := types.SetValueFrom(ctx, types.StringType, grants)
	if diags.HasError() {
		return diags
	}
	return append(diags, state.SetAttribute(ctx, path.Root("assignments"), set)...)
}

func (r *assignmentResource) errorSummary(op string) string {
	return fmt.Sprintf("Unable to %s %s grants of the %s", op, r.d.ScopeNoun, r.d.AssigneeKind)
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "`" + v + "`"
	}
	return strings.Join(quoted, ", ")
}
