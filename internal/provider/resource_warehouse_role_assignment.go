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
	_ resource.Resource                = &lakekeeperWarehouseRoleAssignmentResource{}
	_ resource.ResourceWithConfigure   = &lakekeeperWarehouseRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &lakekeeperWarehouseRoleAssignmentResource{}
)

func init() {
	registerResource(NewLakekeeperWarehouseRoleAssignment)
}

// NewLakekeeperWarehouseAssignment is a helper function to simplify the provider implementation.
func NewLakekeeperWarehouseRoleAssignment() resource.Resource {
	return &lakekeeperWarehouseRoleAssignmentResource{}
}

func (r *lakekeeperWarehouseRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_role_assignment"
}

// lakekeeperWarehouseRoleAssignmentResource defines the resource implementation.
type lakekeeperWarehouseRoleAssignmentResource struct {
	client *api.Client
}

// lakekeeperWarehouseRoleAssignmentResourceModel describes the resource data model.
type lakekeeperWarehouseRoleAssignmentResourceModel struct {
	ID          types.String `tfsdk:"id"` // form: role_id:warehouse_id (internal ID)
	WarehouseID types.String `tfsdk:"warehouse_id"`
	RoleID      types.String `tfsdk:"role_id"`
	Assignments types.Set    `tfsdk:"assignments"`
}

func (r *lakekeeperWarehouseRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The ` + "`lakekeeper_warehouse_role_assignment`" + ` resource allows to manage the lifecycle of a role assignement to a warehouse.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/update_warehouse_assignments)`),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The internal ID of this resource. In the form: `{{warehouse_id}}/{{role_id}}`",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"warehouse_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the warehouse.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile("^[^/]+$"), "must be a warehouse UUID and NOT include the project UUID"),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the role to assign to the warehouse.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile("^[^/]+$"), "must be a role UUID and NOT include the project UUID"),
				},
			},
			"assignments": schema.SetAttribute{
				ElementType: types.StringType,
				MarkdownDescription: "List of assignments the role has on this warehouse. values can be " +
					fmt.Sprintf("`%s` `%s` `%s` `%s` `%s` `%s` `%s`",
						api.WarehouseAssignmentOwnership,
						api.WarehouseAssignmentPassGrants,
						api.WarehouseAssignmentManageGrants,
						api.WarehouseAssignmentDescribe,
						api.WarehouseAssignmentSelect,
						api.WarehouseAssignmentCreate,
						api.WarehouseAssignmentModify,
					),
				Required: true,
				Validators: []validator.Set{setvalidator.ValueStringsAre(
					stringvalidator.OneOf(
						api.WarehouseAssignmentOwnership,
						api.WarehouseAssignmentPassGrants,
						api.WarehouseAssignmentManageGrants,
						api.WarehouseAssignmentDescribe,
						api.WarehouseAssignmentSelect,
						api.WarehouseAssignmentCreate,
						api.WarehouseAssignmentModify,
					)),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *lakekeeperWarehouseRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create creates a new upstream resources and adds it into the Terraform state.
func (r *lakekeeperWarehouseRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state, plan lakekeeperWarehouseRoleAssignmentResourceModel

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

		opts = append(opts, api.Assignment{Assignee: api.RoleAssignee(plan.RoleID.ValueString()), Assignment: s.ValueString()})
	}

	err := r.client.UpdateWarehouseAssignments(ctx, plan.WarehouseID.ValueString(), opts, nil)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to write warehouse assignment, %s", err.Error()))
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.WarehouseID.ValueString(), plan.RoleID.ValueString()))

	state.WarehouseID = plan.WarehouseID
	state.RoleID = plan.RoleID
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *lakekeeperWarehouseRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lakekeeperWarehouseRoleAssignmentResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	warehouseID, roleID := splitInternalID(state.ID)

	assignments, err := r.client.GetWarehouseAssignments(ctx, warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read role assignments, %s", err.Error()))
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

	state.WarehouseID = types.StringValue(warehouseID)
	state.RoleID = types.StringValue(roleID)

	state.Assignments = newAssignments
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Updates updates the resource in-place.
func (r *lakekeeperWarehouseRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lakekeeperWarehouseRoleAssignmentResourceModel

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

	warehouseID, roleID := splitInternalID(state.ID)

	if err := r.client.UpdateWarehouseAssignments(ctx, warehouseID, writes, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update role assignment, %v", err.Error()))
		return
	}

	state.RoleID = types.StringValue(roleID)
	state.Assignments = plan.Assignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Deletes removes the resource.
func (r *lakekeeperWarehouseRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lakekeeperWarehouseRoleAssignmentResourceModel

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

	if err := r.client.UpdateWarehouseAssignments(ctx, state.WarehouseID.ValueString(), nil, deletes); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to delete role assignment, %v", err.Error()))
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *lakekeeperWarehouseRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "warehouse_id/role_id"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: warehouse_id/role_id",
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("warehouse_id"), parts[0])
	resp.State.SetAttribute(ctx, path.Root("role_id"), parts[1])

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
