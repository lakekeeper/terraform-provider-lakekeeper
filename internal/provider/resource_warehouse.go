package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ resource.Resource                = &lakekeeperWarehouseResource{}
	_ resource.ResourceWithConfigure   = &lakekeeperWarehouseResource{}
	_ resource.ResourceWithImportState = &lakekeeperWarehouseResource{}
)

func init() {
	registerResource(NewLakekeeperWarehouseResource)
}

// NewLakekeeperWarehouseResource is a helper function to simplify the provider implementation.
func NewLakekeeperWarehouseResource() resource.Resource {
	return &lakekeeperWarehouseResource{}
}

func (r *lakekeeperWarehouseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warehouse"
}

// refreshErrorSummary is the diagnostic summary for a failed warehouse.Refresh.
const refreshErrorSummary = "Error refreshing warehouse state"

// lakekeeperWarehouseResource defines the resource implementation.
type lakekeeperWarehouseResource struct {
	client *api.Client
}

func (r *lakekeeperWarehouseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = warehouse.ResourceSchema()
}

// Configure adds the provider configured client to the resource.
func (r *lakekeeperWarehouseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create creates a new upstream resources and adds it into the Terraform state.
func (r *lakekeeperWarehouseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state warehouse.ResourceModel
	var plan types.Object

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(plan.As(ctx, &state, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    false,
		UnhandledUnknownAsEmpty: false,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, err := state.WarehouseRequest()
	if err != nil {
		resp.Diagnostics.AddError("Error decoding state to model", fmt.Sprintf("Incorrect Warehouse creation request, %v", err))
		return
	}

	warehouseID, err := r.client.CreateWarehouse(ctx, state.ProjectID.ValueString(), *opts)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred",
			fmt.Sprintf("Unable to create warehouse, %v", err))
		return
	}

	// The warehouse now EXISTS server-side. Persist its identity to state
	// immediately, before the post-create steps below (protection, activation,
	// managed access, read-back). If any of those fail and we returned without
	// having saved state, Terraform would consider the warehouse never-created
	// and try to create it again on the next apply — which fails with
	// CreateWarehouseStorageProfileOverlap and leaves the resource permanently
	// orphaned (created in Lakekeeper, absent from state). Recording the ID here
	// makes such a failure recoverable: the next apply updates/reads the existing
	// warehouse instead of re-creating it.
	state.ID = types.StringValue(state.ProjectID.ValueString() + "/" + warehouseID)
	state.WarehouseID = types.StringValue(warehouseID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// set protection
	if state.Protected.ValueBool() {
		err := r.client.SetWarehouseProtection(ctx, state.ProjectID.ValueString(), warehouseID, state.Protected.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred.",
				fmt.Sprintf("Unable to set protection to %t for warehouse %s, %v", state.Protected.ValueBool(), warehouseID, err),
			)
		}
	}

	// set inactive
	if !state.Active.ValueBool() {
		err := r.client.DeactivateWarehouse(ctx, state.ProjectID.ValueString(), warehouseID)
		if err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred.",
				fmt.Sprintf("Unable to deactivate warehouse %s, %v", warehouseID, err),
			)
		}
	}

	wh, err := r.client.GetWarehouse(ctx, state.ProjectID.ValueString(), warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred",
			fmt.Sprintf("Unable to read warehouse, %v", err))
		return
	}

	if !state.ManagedAccess.IsNull() {
		err := r.client.SetWarehouseManagedAccess(ctx, wh.ID, state.ManagedAccess.ValueBool())
		if err != nil {
			// The managed-access endpoint only exists when the server runs an
			// authorization backend (e.g. OpenFGA); on an `allowall` backend it
			// returns 404. Treat that as "managed access not applicable here"
			// rather than a hard error, so warehouses can still be managed on
			// allowall clusters. Any other error is real.
			if api.IsNotFound(err) {
				resp.Diagnostics.AddWarning("Managed access not supported by server",
					fmt.Sprintf("Warehouse %s created; skipping managed_access (endpoint returned 404 — the server's authorization backend does not support managed access, e.g. allowall).", wh.ID))
			} else {
				resp.Diagnostics.AddError("Lakekeeper API error occurred",
					fmt.Sprintf("Unable to set managed access, %v", err))
				return
			}
		}
	}

	// The plan supplies the storage credential and managed_access, which the
	// server does not return.
	state, err = warehouse.Refresh(state, wh)
	if err != nil {
		resp.Diagnostics.AddError(refreshErrorSummary, err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *lakekeeperWarehouseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state warehouse.ResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, warehouseID := splitInternalID(state.ID)
	wh, err := r.client.GetWarehouse(ctx, projectID, warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read warehouse %s in project %s, %s", warehouseID, projectID, err))
		return
	}

	// The prior state supplies the storage credential and managed_access.
	state, err = warehouse.Refresh(state, wh)
	if err != nil {
		resp.Diagnostics.AddError(refreshErrorSummary, err.Error())
		return
	}

	// get managed access property. Like SetManagedAccess, this endpoint only
	// exists when the server runs an authorization backend (e.g. OpenFGA); on an
	// `allowall` backend it returns 404. Tolerate that (leave managed_access as
	// refreshed from the warehouse settings) instead of erroring — otherwise Read
	// (and therefore plan/refresh/import) is impossible on an allowall cluster.
	managedAccess, err := r.client.GetWarehouseManagedAccess(ctx, wh.ID)
	if err != nil {
		if api.IsNotFound(err) {
			resp.Diagnostics.AddWarning("Managed access not supported by server",
				fmt.Sprintf("Warehouse %s: skipping managed_access read (endpoint returned 404 — the server's authorization backend does not support managed access, e.g. allowall).", warehouseID))
		} else {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read warehouse %s authorization properties in project %s, %s", warehouseID, projectID, err))
			return
		}
	} else {
		state.ManagedAccess = types.BoolValue(managedAccess)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Updates updates the resource in-place.
func (r *lakekeeperWarehouseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state warehouse.ResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, warehouseID := splitInternalID(state.ID)

	if plan.Active.ValueBool() {
		if err := r.client.ActivateWarehouse(ctx, projectID, warehouseID); err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to activate warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	} else {
		if err := r.client.DeactivateWarehouse(ctx, projectID, warehouseID); err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to deactivate warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	}

	// Rename the warehouse if the name field is different
	if plan.Name.ValueString() != state.Name.ValueString() {
		if err := r.client.RenameWarehouse(ctx, projectID, warehouseID, plan.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to rename warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	}

	// Set warehouse protection if the protected field is different
	if plan.Protected.ValueBool() != state.Protected.ValueBool() {
		if err := r.client.SetWarehouseProtection(ctx, projectID, warehouseID, plan.Protected.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to set protection for warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	}

	opts, err := plan.WarehouseRequest()
	if err != nil {
		resp.Diagnostics.AddError("Error decoding plan to model", fmt.Sprintf("Incorrect Warehouse update request, %v", err))
		return
	}

	// Update the delete profile
	if !plan.DeleteProfile.Type.Equal(state.DeleteProfile.Type) || !plan.DeleteProfile.ExpirationSeconds.Equal(state.DeleteProfile.ExpirationSeconds) {
		if err := r.client.UpdateWarehouseDeleteProfile(ctx, projectID, warehouseID, *opts.DeleteProfile); err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update delete profile for warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	}

	// Update the storage profile and its storage credential
	if err := r.client.UpdateWarehouseStorage(ctx, projectID, warehouseID, opts.StorageProfile, opts.StorageCredential); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to update storage profile for warehouse %s in project %s, %v", warehouseID, projectID, err))
		return
	}

	// Update the authorization property. As in Create, tolerate a 404 from the
	// managed-access endpoint (server without an authz backend, e.g. allowall).
	if err := r.client.SetWarehouseManagedAccess(ctx, warehouseID, plan.ManagedAccess.ValueBool()); err != nil {
		if api.IsNotFound(err) {
			resp.Diagnostics.AddWarning("Managed access not supported by server",
				fmt.Sprintf("Skipping managed_access update for warehouse %s (endpoint returned 404 — the server's authorization backend does not support managed access, e.g. allowall).", warehouseID))
		} else {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to set authorization properties for warehouse %s in project %s, %v", warehouseID, projectID, err))
			return
		}
	}

	// Refresh the state with the updated warehouse settings
	wh, err := r.client.GetWarehouse(ctx, projectID, warehouseID)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to read warehouse %s in project %s, %v", warehouseID, projectID, err))
		return
	}

	// The plan supplies the storage credential and managed_access.
	state, err = warehouse.Refresh(plan, wh)
	if err != nil {
		resp.Diagnostics.AddError(refreshErrorSummary, err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Deletes removes the resource.
func (r *lakekeeperWarehouseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state warehouse.ResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, warehouseID := splitInternalID(state.ID)

	if err := r.client.DeleteWarehouse(ctx, projectID, warehouseID, true); err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf("Unable to delete warehouse %s in project %s, %v", warehouseID, projectID, err))
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *lakekeeperWarehouseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "project_id/warehouse_id"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected format: project_id/warehouse_id",
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])
	resp.State.SetAttribute(ctx, path.Root("warehouse_id"), parts[1])

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
