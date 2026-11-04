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
	_ resource.Resource                = &lakekeeperProjectRoleAssignmentResource{}
	_ resource.ResourceWithConfigure   = &lakekeeperProjectRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &lakekeeperProjectRoleAssignmentResource{}
)

func init() {
	registerResource(NewLakekeeperProjectRoleAssignment)
}

// NewLakekeeperProjectAssignment is a helper function to simplify the provider implementation.
func NewLakekeeperProjectRoleAssignment() resource.Resource {
	return &lakekeeperProjectRoleAssignmentResource{}
}

func (r *lakekeeperProjectRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_role_assignment"
}

// lakekeeperProjectRoleAssignmentResource defines the resource implementation.
type lakekeeperProjectRoleAssignmentResource struct {
	client *api.Client
}

// lakekeeperProjectRoleAssignmentResourceModel describes the resource data model.
type lakekeeperProjectRoleAssignmentResourceModel struct {
	ID          types.String `tfsdk:"id"` // form: project_id:role_id (internal ID)
	ProjectID   types.String `tfsdk:"project_id"`
	RoleID      types.String `tfsdk:"role_id"`
	Assignments types.Set    `tfsdk:"assignments"`
}

func (r *lakekeeperProjectRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The ` + "`lakekeeper_project_role_assignment`" + ` resource allows to manage the lifecycle of a role assignement to a project.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/update_project_assignments)`),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The internal ID of this resource. In the form: {{project_id}}/{{role_id}}",
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
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the role.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile("^[^/]+$"), "must be a role UUID and NOT include the project UUID"),
				},
			},
			"assignments": schema.SetAttribute{
				ElementType: types.StringType,
				MarkdownDescription: "List of project assignments for this role. values can be " +
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
						api.ProjectAssignmentModify),
				)},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *lakekeeperProjectRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create creates a new upstream resources and adds it into the Terraform state.
func (r *lakekeeperProjectRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state, plan lakekeeperProjectRoleAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	roleID := plan.RoleID.ValueString()

	id := fmt.Sprintf("%s/%s", projectID, roleID)

	var opts []api.Assignment

	for _, v := range plan.Assignments.Elements() {
		s, ok := v.(types.String)
		if !ok {
			resp.Diagnostics.AddError("Error converting model to resource", fmt.Sprintf("Unable to read assignment %s", v.String()))
			return
		}

		opts = append(opts, api.Assignment{Assignee: api.RoleAssignee(plan.RoleID.ValueString()), Assignment: s.ValueString()})
	}

	err := r.client.UpdateProjectAssignments(ctx, projectID, opts, nil)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to write project assignment, %s", err.Error()))
		return
	}

	state.ID = types.StringValue(id)
	state.ProjectID = plan.ProjectID
	state.RoleID = plan.RoleID
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *lakekeeperProjectRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lakekeeperProjectRoleAssignmentResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, roleID := splitInternalID(state.ID)

	assignments, err := r.client.GetProjectAssignments(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read project assignments, %s", err.Error()))
		return
	}

	var elems []attr.Value
	for _, v := range assignments {
		if v.Assignee.ID == roleID && v.Assignee.Type == api.AssigneeTypeRole {
			elems = append(elems, types.StringValue(v.Assignment))
		}
	}

	newAssignments, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	state.RoleID = types.StringValue(roleID)
	state.ProjectID = types.StringValue(projectID)
	state.Assignments = newAssignments

	state.Assignments = newAssignments
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Updates updates the resource in-place.
func (r *lakekeeperProjectRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lakekeeperProjectRoleAssignmentResourceModel

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
		writes = append(writes, api.Assignment{Assignee: api.RoleAssignee(plan.RoleID.ValueString()), Assignment: v.ValueString()})
	}

	for _, v := range removed {
		deletes = append(deletes, api.Assignment{Assignee: api.RoleAssignee(plan.RoleID.ValueString()), Assignment: v.ValueString()})
	}

	if err := r.client.UpdateProjectAssignments(ctx, plan.ProjectID.ValueString(), writes, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update project assignment, %v", err.Error()))
		return
	}

	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Deletes removes the resource.
func (r *lakekeeperProjectRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lakekeeperProjectRoleAssignmentResourceModel

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
		deletes = append(deletes, api.Assignment{Assignee: api.RoleAssignee(state.RoleID.ValueString()), Assignment: v.ValueString()})
	}

	if err := r.client.UpdateProjectAssignments(ctx, state.ProjectID.ValueString(), nil, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to delete project assignment, %v", err.Error()))
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *lakekeeperProjectRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "project_id/role_id"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: project_id/role_id",
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])
	resp.State.SetAttribute(ctx, path.Root("role_id"), parts[1])

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
