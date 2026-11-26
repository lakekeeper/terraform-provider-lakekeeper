package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ resource.Resource                = &protectionResource{}
	_ resource.ResourceWithConfigure   = &protectionResource{}
	_ resource.ResourceWithImportState = &protectionResource{}
)

// protectionKind declares one lakekeeper_*_protection resource. The three
// kinds differ only in their Terraform type name, the ID attribute of the
// protected object, and the API calls that read and write the flag.
type protectionKind struct {
	typeName          string // e.g. "table_protection"
	objectName        string // e.g. "table", used in descriptions and errors
	objectIDAttribute string // e.g. "table_id"
	upstreamOperation string // e.g. "set_table_protection", for the API docs link
	get               func(context.Context, *api.Client, string, string) (bool, error)
	set               func(context.Context, *api.Client, string, string, bool) (bool, error)
}

// newProtectionResource builds the resource for one protectionKind.
func newProtectionResource(kind protectionKind) resource.Resource {
	return &protectionResource{kind: kind}
}

// protectionResource manages the protection flag of a table, view or generic
// table without managing the object itself.
type protectionResource struct {
	client *api.Client
	kind   protectionKind
}

func (r *protectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind.typeName
}

func (r *protectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: fmt.Sprintf(`The `+"`lakekeeper_%s`"+` resource manages the protection flag of one %s in Lakekeeper.

Protection prevents Lakekeeper from dropping the %s. The %s itself is not managed by this resource: create it with an Iceberg or Generic Table client and reference its ID here.

Changing `+"`protected`"+` sets or clears the flag. **Destroying this resource only removes it from Terraform state; it does not change the protection flag on the server.** Unprotect the %s explicitly with `+"`protected = false`"+` before removing the resource if that is what you want.

Changing `+"`warehouse_id`"+` or `+"`%s`"+` also only removes the old resource from Terraform state: it does not change the protection flag of the %s it referred to, so a protected %s stays protected. Add a `+"`lakekeeper_%s`"+` block for that %s with `+"`protected = false`"+` to unprotect it.

A missing or soft-deleted %s cannot be protected: Lakekeeper rejects the request and the provider reports the error instead of silently skipping the change.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/warehouse/operation/%s)`,
			r.kind.typeName, r.kind.objectName, r.kind.objectName, r.kind.objectName, r.kind.objectName, r.kind.objectIDAttribute, r.kind.objectName, r.kind.objectName, r.kind.typeName, r.kind.objectName, r.kind.objectName, r.kind.upstreamOperation),

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The internal ID of this resource, in the form `{{warehouse_id}}/{{%s}}`.", r.kind.objectIDAttribute),
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"warehouse_id": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The ID of the warehouse that contains the %s.", r.kind.objectName),
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			r.kind.objectIDAttribute: schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("The ID of the %s.", r.kind.objectName),
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"protected": schema.BoolAttribute{
				MarkdownDescription: fmt.Sprintf("Whether the %s is protected from deletion.", r.kind.objectName),
				Required:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *protectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	resourceData := req.ProviderData.(*LakekeeperResourceData)
	r.client = resourceData.Client
}

// Create sets the protection flag on an existing object and adds the resource
// to the Terraform state.
func (r *protectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	warehouseID, objectID, protected := r.valuesFrom(ctx, &req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	actual, err := r.kind.set(ctx, r.client, warehouseID, objectID, protected)
	if err != nil {
		r.addSetError(&resp.Diagnostics, warehouseID, objectID, err)
		return
	}

	r.setState(ctx, &resp.State, warehouseID, objectID, actual, &resp.Diagnostics)

	tflog.Debug(ctx, "set protection on "+r.kind.objectName, map[string]any{
		"warehouse_id": warehouseID, r.kind.objectIDAttribute: objectID, "protected": actual,
	})
}

// Read refreshes the protection flag from Lakekeeper.
func (r *protectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	warehouseID, objectID, err := splitInternalID(id)
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	actual, err := r.kind.get(ctx, r.client, warehouseID, objectID)
	if err != nil {
		if api.IsNotFound(err) {
			// The object was dropped or is no longer visible. Terraform's
			// convention is to forget it; a later apply reports the missing
			// object when it tries to set protection again.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lakekeeper API error occurred", fmt.Sprintf(
			"Unable to read protection for %s %s in warehouse %s, %v", r.kind.objectName, objectID, warehouseID, err))
		return
	}

	r.setState(ctx, &resp.State, warehouseID, objectID, actual, &resp.Diagnostics)
}

// Update sets the protection flag to its planned value.
func (r *protectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	warehouseID, objectID, protected := r.valuesFrom(ctx, &req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	actual, err := r.kind.set(ctx, r.client, warehouseID, objectID, protected)
	if err != nil {
		r.addSetError(&resp.Diagnostics, warehouseID, objectID, err)
		return
	}

	r.setState(ctx, &resp.State, warehouseID, objectID, actual, &resp.Diagnostics)
}

// Delete removes the resource from the Terraform state. It deliberately does
// not change the protection flag on the server: destroying Terraform must not
// silently unprotect an object.
func (r *protectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "removing protection resource from state without changing the server's flag", map[string]any{
		"id": id.ValueString(),
	})

	resp.State.RemoveResource(ctx)
}

// ImportState imports the resource into the Terraform state.
func (r *protectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: "warehouse_id/<object_id>"
	warehouseID, objectID, err := splitInternalID(types.StringValue(req.ID))
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			err.Error(),
		)
		return
	}

	resp.State.SetAttribute(ctx, path.Root("warehouse_id"), warehouseID)
	resp.State.SetAttribute(ctx, path.Root(r.kind.objectIDAttribute), objectID)

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// valuesFrom reads the identifying attributes and the flag from a plan.
func (r *protectionResource) valuesFrom(ctx context.Context, plan *tfsdk.Plan, diags *diag.Diagnostics) (string, string, bool) {
	var warehouseID, objectID types.String
	var protected types.Bool

	diags.Append(plan.GetAttribute(ctx, path.Root("warehouse_id"), &warehouseID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root(r.kind.objectIDAttribute), &objectID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("protected"), &protected)...)

	return warehouseID.ValueString(), objectID.ValueString(), protected.ValueBool()
}

// setState writes the protection resource's attributes to state.
func (r *protectionResource) setState(ctx context.Context, state *tfsdk.State, warehouseID, objectID string, protected bool, diags *diag.Diagnostics) {
	diags.Append(state.SetAttribute(ctx, path.Root("id"), warehouseID+"/"+objectID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("warehouse_id"), warehouseID)...)
	diags.Append(state.SetAttribute(ctx, path.Root(r.kind.objectIDAttribute), objectID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("protected"), protected)...)
}

// addSetError reports a failed protection write, calling out the common cause
// explicitly instead of leaving the user with a bare 404.
func (r *protectionResource) addSetError(diags *diag.Diagnostics, warehouseID, objectID string, err error) {
	if api.IsNotFound(err) {
		diags.AddError("Lakekeeper API error occurred", fmt.Sprintf(
			"Unable to set protection for %s %s in warehouse %s: Lakekeeper reported that it does not exist. It may be missing or soft-deleted; protection is only applied to an existing %s. %v",
			r.kind.objectName, objectID, warehouseID, r.kind.objectName, err))
		return
	}
	diags.AddError("Lakekeeper API error occurred", fmt.Sprintf(
		"Unable to set protection for %s %s in warehouse %s, %v", r.kind.objectName, objectID, warehouseID, err))
}
