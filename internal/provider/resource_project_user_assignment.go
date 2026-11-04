package provider

import (
	"context"
	"fmt"
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
	_ resource.Resource                = &lakekeeperProjectUserAssignmentResource{}
	_ resource.ResourceWithConfigure   = &lakekeeperProjectUserAssignmentResource{}
	_ resource.ResourceWithImportState = &lakekeeperProjectUserAssignmentResource{}
)

func init() {
	registerResource(NewLakekeeperProjectUserAssignment)
}

// NewLakekeeperProjectAssignment is a helper function to simplify the provider implementation.
func NewLakekeeperProjectUserAssignment() resource.Resource {
	return &lakekeeperProjectUserAssignmentResource{}
}

func (r *lakekeeperProjectUserAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_user_assignment"
}

// lakekeeperProjectUserAssignmentResource defines the resource implementation.
type lakekeeperProjectUserAssignmentResource struct {
	client *api.Client
}

// lakekeeperProjectUserAssignmentResourceModel describes the resource data model.
type lakekeeperProjectUserAssignmentResourceModel struct {
	ID          types.String `tfsdk:"id"` // form: project_id:user_id (internal ID)
	ProjectID   types.String `tfsdk:"project_id"`
	UserID      types.String `tfsdk:"user_id"`
	Assignments types.Set    `tfsdk:"assignments"`
}

func (r *lakekeeperProjectUserAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The ` + "`lakekeeper_project_user_assignment`" + ` resource allows to manage the lifecycle of a user assignement to a project.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/update_project_assignments)`),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The internal ID of this resource. In the form: `{{project_id}}/{{user_id}}`",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the user.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"assignments": schema.SetAttribute{
				ElementType: types.StringType,
				MarkdownDescription: "List of project assignments for this user. values can be " +
					fmt.Sprintf("`%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s`",
						api.ProjectAssignmentAdmin,
						api.ProjectAssignmentSecurityAdmin,
						api.ProjectAssignmentDataAdmin,
						api.ProjectAssignmentRoleCreator,
						api.ProjectAssignmentDescribe,
						api.ProjectAssignmentSelect,
						api.ProjectAssignmentCreate,
						api.ProjectAssignmentModify,
					),
				Required: true,
				Validators: []validator.Set{setvalidator.ValueStringsAre(
					stringvalidator.OneOf(
						api.ProjectAssignmentAdmin,
						api.ProjectAssignmentSecurityAdmin,
						api.ProjectAssignmentDataAdmin,
						api.ProjectAssignmentRoleCreator,
						api.ProjectAssignmentDescribe,
						api.ProjectAssignmentSelect,
						api.ProjectAssignmentCreate,
						api.ProjectAssignmentModify,
					),
				)},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *lakekeeperProjectUserAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create creates a new upstream resources and adds it into the Terraform state.
func (r *lakekeeperProjectUserAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state, plan lakekeeperProjectUserAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	userID := plan.UserID.ValueString()

	id := fmt.Sprintf("%s/%s", projectID, userID)

	var opts []api.Assignment

	for _, v := range plan.Assignments.Elements() {
		s, ok := v.(types.String)
		if !ok {
			resp.Diagnostics.AddError("Error converting model to resource", fmt.Sprintf("Unable to read assignment %s", v.String()))
			return
		}

		opts = append(opts, api.Assignment{Assignee: api.UserAssignee(plan.UserID.ValueString()), Assignment: s.ValueString()})
	}

	err := r.client.UpdateProjectAssignments(ctx, projectID, opts, nil)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to write project assignment, %s", err.Error()))
		return
	}

	state.ID = types.StringValue(id)
	state.ProjectID = plan.ProjectID
	state.UserID = plan.UserID
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *lakekeeperProjectUserAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lakekeeperProjectUserAssignmentResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, userID := splitInternalID(state.ID)

	assignments, err := r.client.GetProjectAssignments(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read project assignments, %s", err.Error()))
		return
	}

	var elems []attr.Value
	for _, v := range assignments {
		if v.Assignee.ID == userID && v.Assignee.Type == api.AssigneeTypeUser {
			elems = append(elems, types.StringValue(v.Assignment))
		}
	}

	newAssignments, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	state.UserID = types.StringValue(userID)
	state.ProjectID = types.StringValue(projectID)
	state.Assignments = newAssignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Updates updates the resource in-place.
func (r *lakekeeperProjectUserAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lakekeeperProjectUserAssignmentResourceModel

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
		writes = append(writes, api.Assignment{Assignee: api.UserAssignee(plan.UserID.ValueString()), Assignment: v.ValueString()})
	}

	for _, v := range removed {
		deletes = append(deletes, api.Assignment{Assignee: api.UserAssignee(plan.UserID.ValueString()), Assignment: v.ValueString()})
	}

	if err := r.client.UpdateProjectAssignments(ctx, plan.ProjectID.ValueString(), writes, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update project assignment, %v", err.Error()))
		return
	}

	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Deletes removes the resource.
func (r *lakekeeperProjectUserAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lakekeeperProjectUserAssignmentResourceModel

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
		deletes = append(deletes, api.Assignment{Assignee: api.UserAssignee(state.UserID.ValueString()), Assignment: v.ValueString()})
	}

	if err := r.client.UpdateProjectAssignments(ctx, state.ProjectID.ValueString(), nil, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to delete project assignment, %v", err.Error()))
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *lakekeeperProjectUserAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "project_id/user_id"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: project_id/user_id",
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])
	resp.State.SetAttribute(ctx, path.Root("user_id"), parts[1])

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
