package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &LakekeeperWarehouseRoleAccessDataSource{}
	_ datasource.DataSourceWithConfigure = &LakekeeperWarehouseRoleAccessDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperWarehouseRoleAccessDataSource)
}

// NewLakekeeperWarehouseRoleAccessDataSource is a helper function to simplify the provider implementation.
func NewLakekeeperWarehouseRoleAccessDataSource() datasource.DataSource {
	return &LakekeeperWarehouseRoleAccessDataSource{}
}

// LakekeeperWarehouseRoleAccessDataSource is the data source implementation.
type LakekeeperWarehouseRoleAccessDataSource struct {
	client *api.Client
}

// LakekeeperWarehouseRoleAccessDataSourceModel describes the data source data model.
type lakekeeperWarehouseRoleAccessDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	WarehouseID    types.String `tfsdk:"warehouse_id"`
	RoleID         types.String `tfsdk:"role_id"`
	AllowedActions types.Set    `tfsdk:"allowed_actions"`
}

// Metadata returns the data source type name.
func (d *LakekeeperWarehouseRoleAccessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse_role_access"
}

// Schema defines the schema for the data source.
func (d *LakekeeperWarehouseRoleAccessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The ` + "`lakekeeper_warehouse_role_access`" + ` data source retrieves the accesses a role can have on a warehouse.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/permissions/operation/get_warehouse_access_by_id)`),

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The internal ID of this data source, in the form <warehouse_id>:<role_id>.",
				Computed:            true,
			},
			"warehouse_id": schema.StringAttribute{
				MarkdownDescription: "ID of the warehouse.",
				Required:            true,
			},

			"role_id": schema.StringAttribute{
				MarkdownDescription: "ID of the role.",
				Required:            true,
			},
			"allowed_actions": schema.SetAttribute{
				MarkdownDescription: `List of the role's allowed actions on the warehouse. The possible values are ` +
					fmt.Sprintf("`%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s` `%s`",
						api.WarehouseActionCreateNamespace,
						api.WarehouseActionDelete,
						api.WarehouseActionModifyStorage,
						api.WarehouseActionModifyStorageCredential,
						api.WarehouseActionGetConfig,
						api.WarehouseActionGetMetadata,
						api.WarehouseActionListNamespaces,
						api.WarehouseActionIncludeInList,
						api.WarehouseActionDeactivate,
						api.WarehouseActionActivate,
						api.WarehouseActionRename,
						api.WarehouseActionListDeletedTabulars,
						api.WarehouseActionReadAssignments,
						api.WarehouseActionGrantCreate,
						api.WarehouseActionGrantDescribe,
						api.WarehouseActionGrantModify,
						api.WarehouseActionGrantSelect,
						api.WarehouseActionGrantPassGrants,
						api.WarehouseActionGrantManageGrants,
						api.WarehouseActionChangeOwnership,
						api.WarehouseActionGetAllTasks,
						api.WarehouseActionControlAllTasks,
						api.WarehouseActionGetEndpointStatistics,
					),
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *LakekeeperWarehouseRoleAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	datasource := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = datasource.Client
}

// Read refreshes the Terraform state with the latest data.
func (d *LakekeeperWarehouseRoleAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperWarehouseRoleAccessDataSourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("%s/%s", state.WarehouseID.ValueString(), state.RoleID.ValueString()))

	actions, err := d.client.GetWarehouseAccess(ctx, state.WarehouseID.ValueString(), api.RoleAssignee(state.RoleID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read acess for role %s on warehouse %s, %v", state.RoleID.ValueString(), state.ID.ValueString(), err))
		return
	}

	allowedActions, diags := types.SetValueFrom(ctx, types.StringType, actions)
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	state.AllowedActions = allowedActions

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
