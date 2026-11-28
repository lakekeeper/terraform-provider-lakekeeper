package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ datasource.DataSource              = &lakekeeperTagAttachmentsDataSource{}
	_ datasource.DataSourceWithConfigure = &lakekeeperTagAttachmentsDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperTagAttachmentsDataSource)
}

// NewLakekeeperTagAttachmentsDataSource returns a data source for reverse-looking up tag attachments.
func NewLakekeeperTagAttachmentsDataSource() datasource.DataSource {
	return &lakekeeperTagAttachmentsDataSource{}
}

type lakekeeperTagAttachmentsDataSource struct {
	client *api.Client
}

type lakekeeperTagAttachmentsDataSourceModel struct {
	ProjectID       types.String                        `tfsdk:"project_id"`
	TagDefinitionID types.String                        `tfsdk:"tag_definition_id"`
	TargetType      types.String                        `tfsdk:"target_type"`
	WarehouseID     types.String                        `tfsdk:"warehouse_id"`
	Value           types.String                        `tfsdk:"value"`
	Attachments     []lakekeeperTagAttachmentsItemModel `tfsdk:"attachments"`
}

type lakekeeperTagAttachmentsItemModel struct {
	TargetType     types.String `tfsdk:"target_type"`
	WarehouseID    types.String `tfsdk:"warehouse_id"`
	NamespaceID    types.String `tfsdk:"namespace_id"`
	TableID        types.String `tfsdk:"table_id"`
	ViewID         types.String `tfsdk:"view_id"`
	GenericTableID types.String `tfsdk:"generic_table_id"`
	FieldID        types.Int64  `tfsdk:"field_id"`
	Value          types.String `tfsdk:"value"`
	Source         types.String `tfsdk:"source"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (d *lakekeeperTagAttachmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_attachments"
}

func (d *lakekeeperTagAttachmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_tag_attachments`" + ` data source performs a reverse lookup of all objects directly carrying a governance tag definition.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag/operation/list_tag_attachments)`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project.",
				Required:            true,
			},
			"tag_definition_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the tag definition.",
				Required:            true,
			},
			"target_type": schema.StringAttribute{
				MarkdownDescription: "Optional filter by target type (`warehouse`, `namespace`, `table`, `view`, `generic-table`, `column`).",
				Optional:            true,
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
			"warehouse_id": schema.StringAttribute{
				MarkdownDescription: "Optional filter by warehouse UUID.",
				Optional:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Optional filter by tag value.",
				Optional:            true,
			},
			"attachments": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Objects carrying this tag.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"target_type":      schema.StringAttribute{Computed: true, MarkdownDescription: "Target kind (`warehouse`, `namespace`, `table`, `view`, `generic-table`, `column`)."},
						"warehouse_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Warehouse UUID."},
						"namespace_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Namespace UUID, when `target_type` is `namespace`."},
						"table_id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Table UUID, when `target_type` is `table` or `column`."},
						"view_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "View UUID, when `target_type` is `view`."},
						"generic_table_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Generic table UUID, when `target_type` is `generic-table`."},
						"field_id":         schema.Int64Attribute{Computed: true, MarkdownDescription: "Iceberg field ID, when `target_type` is `column`."},
						"value":            schema.StringAttribute{Computed: true, MarkdownDescription: "Tag value, if set."},
						"source":           schema.StringAttribute{Computed: true, MarkdownDescription: "How the tag was applied."},
						"created_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the tag was applied."},
						"updated_at":       schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the tag was last updated."},
					},
				},
			},
		},
	}
}

func (d *lakekeeperTagAttachmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = data.Client
}

func (d *lakekeeperTagAttachmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperTagAttachmentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var filter api.ListTagAttachmentsFilter
	if !state.TargetType.IsNull() && !state.TargetType.IsUnknown() {
		filter.TargetType = new(state.TargetType.ValueString())
	}
	if !state.WarehouseID.IsNull() && !state.WarehouseID.IsUnknown() {
		filter.WarehouseID = new(state.WarehouseID.ValueString())
	}
	if !state.Value.IsNull() && !state.Value.IsUnknown() {
		filter.Value = new(state.Value.ValueString())
	}

	atts, err := d.client.ListTagAttachments(ctx, state.ProjectID.ValueString(), state.TagDefinitionID.ValueString(), &filter)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to list tag attachments: "+err.Error())
		return
	}

	state.Attachments = make([]lakekeeperTagAttachmentsItemModel, 0, len(atts))
	for _, a := range atts {
		item := lakekeeperTagAttachmentsItemModel{
			TargetType:  types.StringValue(a.Target.Type),
			WarehouseID: types.StringValue(a.Target.WarehouseID),
			Value:       types.StringPointerValue(a.Value),
			Source:      types.StringValue(a.Source),
			CreatedAt:   types.StringValue(a.CreatedAt),
			UpdatedAt:   types.StringPointerValue(a.UpdatedAt),
		}
		if a.Target.NamespaceID != "" {
			item.NamespaceID = types.StringValue(a.Target.NamespaceID)
		}
		if a.Target.TableID != "" {
			item.TableID = types.StringValue(a.Target.TableID)
		}
		if a.Target.ViewID != "" {
			item.ViewID = types.StringValue(a.Target.ViewID)
		}
		if a.Target.GenericTableID != "" {
			item.GenericTableID = types.StringValue(a.Target.GenericTableID)
		}
		if a.Target.FieldID != nil {
			item.FieldID = types.Int64Value(int64(*a.Target.FieldID))
		}
		state.Attachments = append(state.Attachments, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
