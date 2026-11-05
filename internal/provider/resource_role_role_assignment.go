package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ resource.Resource                = &lakekeeperRoleRoleAssignmentResource{}
	_ resource.ResourceWithConfigure   = &lakekeeperRoleRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &lakekeeperRoleRoleAssignmentResource{}
)

func init() {
	registerResource(NewLakekeeperRoleRoleAssignment)
}

// NewLakekeeperRoleAssignment is a helper function to simplify the provider implementation.
func NewLakekeeperRoleRoleAssignment() resource.Resource {
	return &lakekeeperRoleRoleAssignmentResource{}
}

func (r *lakekeeperRoleRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_role_assignment"
}

// lakekeeperRoleRoleAssignmentResource defines the resource implementation.
type lakekeeperRoleRoleAssignmentResource struct {
	client *api.Client
}

// lakekeeperRoleRoleAssignmentResourceModel describes the resource data model.
type lakekeeperRoleRoleAssignmentResourceModel struct {
	ID          types.String `tfsdk:"id"` // form: role_id:assignee_id (internal ID)
	RoleID      types.String `tfsdk:"role_id"`
	AssigneeID  types.String `tfsdk:"assignee_id"`
	Assignments types.Set    `tfsdk:"assignments"`
}

func (r *lakekeeperRoleRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The ` + "`lakekeeper_role_role_assignment`" + ` resource allows to manage the lifecycle of a role assignement to a role.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/update_role_assignments)`),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The internal ID of this resource. In the form: <role_id>:<assignee_id>",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the role.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile("^[^/]+$"), "must be a role UUID and NOT include the project UUID"),
				},
			},
			"assignee_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the role to assign to the role.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile("^[^/]+$"), "must be a role UUID and NOT include the project UUID"),
				},
			},
			"assignments": schema.SetAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: fmt.Sprintf("List of role assignments for this role. values can be `%s` or `%s`", api.RoleAssignmentOwnership, api.RoleAssignmentAssignee),
				Required:            true,
				Validators: []validator.Set{setvalidator.ValueStringsAre(
					stringvalidator.OneOf(api.RoleAssignmentOwnership, api.RoleAssignmentAssignee),
				)},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *lakekeeperRoleRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create creates a new upstream resources and adds it into the Terraform state.
func (r *lakekeeperRoleRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state, plan lakekeeperRoleRoleAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var opts []api.Assignment

	for _, v := range plan.Assignments.Elements() {
		s, ok := v.(types.String)
		if !ok {
			resp.Diagnostics.AddError("Error converting model to resource", fmt.Sprintf("Unable to read assignment %s", v.String()))
			return
		}

		opts = append(opts, api.Assignment{Assignee: api.RoleAssignee(plan.AssigneeID.ValueString()), Assignment: s.ValueString()})
	}

	err := r.client.UpdateRoleAssignments(ctx, plan.RoleID.ValueString(), opts, nil)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to write role assignment, %s", err.Error()))
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.RoleID.ValueString(), plan.AssigneeID.ValueString()))

	state.RoleID = plan.RoleID
	state.AssigneeID = plan.AssigneeID
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *lakekeeperRoleRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lakekeeperRoleRoleAssignmentResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roleID, assigneeID := splitInternalID(state.ID)

	assignments, err := r.client.GetRoleAssignments(ctx, roleID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read role assignments, %s", err.Error()))
		return
	}

	var elems []attr.Value
	for _, v := range assignments {
		if v.Assignee.ID == assigneeID && v.Assignee.Type == api.AssigneeTypeRole {
			elems = append(elems, types.StringValue(v.Assignment))
		}
	}

	newAssignments, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	state.RoleID = types.StringValue(roleID)
	state.AssigneeID = types.StringValue(assigneeID)

	state.Assignments = newAssignments
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Updates updates the resource in-place.
func (r *lakekeeperRoleRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lakekeeperRoleRoleAssignmentResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var planAssignments []types.String
	resp.Diagnostics.Append(plan.Assignments.ElementsAs(ctx, &planAssignments, false)...)

	var stateAssignments []types.String
	resp.Diagnostics.Append(state.Assignments.ElementsAs(ctx, &stateAssignments, false)...)

	if resp.Diagnostics.HasError() {
		return
	}

	added, removed := DiffTypedStrings(stateAssignments, planAssignments)

	var writes []api.Assignment
	var deletes []api.Assignment

	for _, v := range added {
		writes = append(writes, api.Assignment{Assignee: api.RoleAssignee(plan.AssigneeID.ValueString()), Assignment: v.ValueString()})
	}

	for _, v := range removed {
		deletes = append(deletes, api.Assignment{Assignee: api.RoleAssignee(plan.AssigneeID.ValueString()), Assignment: v.ValueString()})
	}

	roleID, assigneeID := splitInternalID(state.ID)

	if err := r.client.UpdateRoleAssignments(ctx, roleID, writes, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update role assignment, %v", err.Error()))
		return
	}

	state.AssigneeID = types.StringValue(assigneeID)
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Deletes removes the resource.
func (r *lakekeeperRoleRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lakekeeperRoleRoleAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var assignments []types.String
	var deletes []api.Assignment

	resp.Diagnostics.Append(state.Assignments.ElementsAs(ctx, &assignments, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, v := range assignments {
		deletes = append(deletes, api.Assignment{Assignee: api.RoleAssignee(state.AssigneeID.ValueString()), Assignment: v.ValueString()})
	}

	if err := r.client.UpdateRoleAssignments(ctx, state.RoleID.ValueString(), nil, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to delete role assignment, %v", err.Error()))
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *lakekeeperRoleRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "role_id/assingee_id"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: role_id/assignee_id",
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("role_id"), parts[0])
	resp.State.SetAttribute(ctx, path.Root("assignee_id"), parts[1])

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
