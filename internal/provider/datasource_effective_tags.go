package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ datasource.DataSource              = &lakekeeperEffectiveTagsDataSource{}
	_ datasource.DataSourceWithConfigure = &lakekeeperEffectiveTagsDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperEffectiveTagsDataSource)
}

// NewLakekeeperEffectiveTagsDataSource reads effective (direct + inherited) tags on a Lakekeeper object.
func NewLakekeeperEffectiveTagsDataSource() datasource.DataSource {
	return &lakekeeperEffectiveTagsDataSource{}
}

type lakekeeperEffectiveTagsDataSource struct {
	client *api.Client
}

type lakekeeperEffectiveTagsDataSourceModel struct {
	ProjectID   types.String                       `tfsdk:"project_id"`
	WarehouseID types.String                       `tfsdk:"warehouse_id"`
	TargetType  types.String                       `tfsdk:"target_type"`
	TargetID    types.String                       `tfsdk:"target_id"`
	ColumnName  types.String                       `tfsdk:"column_name"`
	Tags        []lakekeeperEffectiveTagsItemModel `tfsdk:"tags"`
}

type lakekeeperEffectiveTagsItemModel struct {
	TagDefinitionID          types.String `tfsdk:"tag_definition_id"`
	Name                     types.String `tfsdk:"name"`
	Value                    types.String `tfsdk:"value"`
	Source                   types.String `tfsdk:"source"`
	CreatedAt                types.String `tfsdk:"created_at"`
	UpdatedAt                types.String `tfsdk:"updated_at"`
	InheritedFromType        types.String `tfsdk:"inherited_from_type"`
	InheritedFromWarehouseID types.String `tfsdk:"inherited_from_warehouse_id"`
	InheritedFromNamespaceID types.String `tfsdk:"inherited_from_namespace_id"`
}

func (d *lakekeeperEffectiveTagsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_effective_tags"
}

func (d *lakekeeperEffectiveTagsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_effective_tags`" + ` data source retrieves the resolved effective tags on a Lakekeeper object, including tags inherited from ancestor namespaces and the parent warehouse.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag)`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the project. Used when `target_type` is `warehouse`.",
				Optional:            true,
			},
			"warehouse_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the warehouse.",
				Required:            true,
			},
			"target_type": schema.StringAttribute{
				MarkdownDescription: "Target kind: `warehouse`, `namespace`, `table`, `view`, `generic-table`, or `column`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						api.TagScopeWarehouse,
						api.TagScopeNamespace,
						api.TagScopeTable,
						api.TagScopeView,
						api.TagScopeGenericTable,
						api.TagScopeColumn,
					),
				},
			},
			"target_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the target object (`namespace_id`, `table_id`, `view_id`, or `generic_table_id`). Required when `target_type` is not `warehouse` (for `column`, set this to the `table_id`).",
				Optional:            true,
			},
			"column_name": schema.StringAttribute{
				MarkdownDescription: "The column name. Required when `target_type` is `column`.",
				Optional:            true,
			},
			"tags": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Resolved effective tags on the target.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tag_definition_id":           schema.StringAttribute{Computed: true, MarkdownDescription: "UUID of the tag definition."},
						"name":                        schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the tag definition."},
						"value":                       schema.StringAttribute{Computed: true, MarkdownDescription: "Tag value, if set."},
						"source":                      schema.StringAttribute{Computed: true, MarkdownDescription: "How the supplying attachment was produced."},
						"created_at":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the supplying attachment was created."},
						"updated_at":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the supplying attachment was last updated."},
						"inherited_from_type":         schema.StringAttribute{Computed: true, MarkdownDescription: "Ancestor kind (`warehouse` or `namespace`) if inherited; null if attached directly."},
						"inherited_from_warehouse_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Ancestor warehouse UUID if inherited."},
						"inherited_from_namespace_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Ancestor namespace UUID if inherited from a namespace."},
					},
				},
			},
		},
	}
}

func (d *lakekeeperEffectiveTagsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = data.Client
}

func (d *lakekeeperEffectiveTagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperEffectiveTagsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	warehouseID := state.WarehouseID.ValueString()
	targetType := state.TargetType.ValueString()
	targetID := state.TargetID.ValueString()
	columnName := state.ColumnName.ValueString()

	if targetType != api.TagScopeWarehouse && targetID == "" {
		resp.Diagnostics.AddError("Missing target_id", fmt.Sprintf("`target_id` is required when `target_type` is %q.", targetType))
		return
	}
	if targetType == api.TagScopeColumn && columnName == "" {
		resp.Diagnostics.AddError("Missing column_name", "`column_name` is required when `target_type` is `column`.")
		return
	}

	var tags []api.TargetTag
	var err error
	switch targetType {
	case api.TagScopeWarehouse:
		tags, err = d.client.ListWarehouseTags(ctx, projectID, warehouseID, true)
	case api.TagScopeNamespace:
		tags, err = d.client.ListNamespaceTags(ctx, warehouseID, targetID, true)
	case api.TagScopeTable:
		tags, err = d.client.ListTableTags(ctx, warehouseID, targetID, true)
	case api.TagScopeView:
		tags, err = d.client.ListViewTags(ctx, warehouseID, targetID, true)
	case api.TagScopeGenericTable:
		tags, err = d.client.ListGenericTableTags(ctx, warehouseID, targetID, true)
	case api.TagScopeColumn:
		tags, err = d.client.ListTableColumnTags(ctx, warehouseID, targetID, columnName, true)
	}
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to list effective tags: "+err.Error())
		return
	}

	state.Tags = make([]lakekeeperEffectiveTagsItemModel, 0, len(tags))
	for _, t := range tags {
		item := lakekeeperEffectiveTagsItemModel{
			TagDefinitionID: types.StringValue(t.TagDefinitionID),
			Name:            types.StringValue(t.Name),
			Value:           types.StringPointerValue(t.Value),
			Source:          types.StringValue(t.Source),
			CreatedAt:       types.StringValue(t.CreatedAt),
			UpdatedAt:       types.StringPointerValue(t.UpdatedAt),
		}
		if t.InheritedFrom != nil {
			item.InheritedFromType = types.StringValue(t.InheritedFrom.Type)
			item.InheritedFromWarehouseID = types.StringValue(t.InheritedFrom.WarehouseID)
			item.InheritedFromNamespaceID = types.StringPointerValue(t.InheritedFrom.NamespaceID)
		}
		state.Tags = append(state.Tags, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
