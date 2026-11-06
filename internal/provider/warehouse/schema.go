package warehouse

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// ResourceSchema returns the lakekeeper_warehouse resource schema.
func ResourceSchema() rschema.Schema {
	return rschema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_warehouse`" + ` resource allows to manage the lifecycle of a lakekeeper warehouse.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/warehouse)`,
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				MarkdownDescription: "The internal ID of this resource. In the form: `{{project_id}}/{{warehouse_id}}`",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"warehouse_id": rschema.StringAttribute{
				MarkdownDescription: "The ID of the warehouse.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				MarkdownDescription: `Name of the warehouse to create. Must be unique within a project and may not contain "/"`,
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"project_id": rschema.StringAttribute{
				MarkdownDescription: "The project ID to which the warehouse belongs.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"protected": rschema.BoolAttribute{
				MarkdownDescription: "Whether the warehouse is protected from being deleted. Default is `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"active": rschema.BoolAttribute{
				MarkdownDescription: "Whether the warehouse is active. Default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"managed_access": rschema.BoolAttribute{
				MarkdownDescription: "Whether the managed access is configured on this warehouse. Default is `false`.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"delete_profile":  deleteProfileResourceSchema(),
			"storage_profile": storageProfileResourceSchema(),
		},
	}
}

// DataSourceSchema returns the lakekeeper_warehouse data source schema.
func DataSourceSchema() dschema.Schema {
	return dschema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_warehouse`" + ` data source retrieves information about a warehouse.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/warehouse/operation/get_warehouse)`,

		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				MarkdownDescription: "The internal ID the warehouse. In the form: {{project_id}}/{{warehouse_id}}",
				Computed:            true,
			},
			"warehouse_id": dschema.StringAttribute{
				MarkdownDescription: "The ID the warehouse.",
				Required:            true,
			},
			"project_id": dschema.StringAttribute{
				MarkdownDescription: "The project ID to which the warehouse belongs. If not provided, the default project will be used.",
				Required:            true,
			},
			"name": dschema.StringAttribute{
				MarkdownDescription: "Name of the warehouse.",
				Computed:            true,
			},
			"protected": dschema.BoolAttribute{
				MarkdownDescription: "Whether the warehouse is protected from being deleted.",
				Computed:            true,
			},
			"active": dschema.BoolAttribute{
				MarkdownDescription: "Whether the warehouse is active.",
				Computed:            true,
			},
			"managed_access": dschema.BoolAttribute{
				MarkdownDescription: "Whether managed access is active for this warehouse.",
				Computed:            true,
			},
			"storage_profile": storageProfileDataSourceSchema(),
			"delete_profile":  deleteProfileDataSourceSchema(),
		},
	}
}
