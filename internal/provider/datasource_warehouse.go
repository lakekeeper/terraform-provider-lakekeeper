package provider

import (
	"context"
	"fmt"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &LakekeeperWarehouseDataSource{}
	_ datasource.DataSourceWithConfigure = &LakekeeperWarehouseDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperWarehouseDataSource)
}

// NewLakekeeperWarehouseDataSource is a helper function to simplify the provider implementation.
func NewLakekeeperWarehouseDataSource() datasource.DataSource {
	return &LakekeeperWarehouseDataSource{}
}

// LakekeeperWarehouseDataSource is the data source implementation.
type LakekeeperWarehouseDataSource struct {
	client *api.Client
}

// Metadata returns the data source type name.
func (d *LakekeeperWarehouseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse"
}

// Schema defines the schema for the data source.
func (d *LakekeeperWarehouseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = warehouse.DataSourceSchema()
}

// Configure adds the provider configured client to the data source.
func (d *LakekeeperWarehouseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	datasource := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = datasource.Client
}

// Read refreshes the Terraform state with the latest data.
func (d *LakekeeperWarehouseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state warehouse.DataSourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	warehouseID := state.WarehouseID.ValueString()
	projectID := state.ProjectID.ValueString()

	wh, err := d.client.GetWarehouse(ctx, projectID, warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read warehouse %s, %v", state.Name.ValueString(), err))
		return
	}

	// Authorization Properties
	managedAccess, err := d.client.GetWarehouseManagedAccess(ctx, warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read authorization properties for warehouse %s, %v", state.Name.ValueString(), err))
		return
	}

	state, err = warehouse.DataSourceModelFromAPI(wh)
	if err != nil {
		resp.Diagnostics.AddError("Error refreshing warehouse state", err.Error())
		return
	}
	state.ManagedAccess = types.BoolValue(managedAccess)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
