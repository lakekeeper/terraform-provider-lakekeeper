package provider

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ datasource.DataSource              = &lakekeeperNamespacesDataSource{}
	_ datasource.DataSourceWithConfigure = &lakekeeperNamespacesDataSource{}
)

func init() { registerDataSource(NewLakekeeperNamespacesDataSource) }

// NewLakekeeperNamespacesDataSource lists the direct children of a warehouse or namespace.
func NewLakekeeperNamespacesDataSource() datasource.DataSource {
	return &lakekeeperNamespacesDataSource{}
}

type lakekeeperNamespacesDataSource struct {
	client *api.Client
}

type lakekeeperNamespacesDataSourceModel struct {
	WarehouseID types.String                    `tfsdk:"warehouse_id"`
	Parent      []string                        `tfsdk:"parent"`
	Namespaces  []lakekeeperNamespacesItemModel `tfsdk:"namespaces"`
}

type lakekeeperNamespacesItemModel struct {
	ID    types.String `tfsdk:"id"`
	Parts []string     `tfsdk:"parts"`
}

func (d *lakekeeperNamespacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespaces"
}

func (d *lakekeeperNamespacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_namespaces`" + ` data source lists the direct child namespaces of a warehouse, or of a namespace in it, that the provider's identity can see, sorted by path. It follows every page of the Iceberg REST catalog list. With an authorizer such as OpenFGA, Lakekeeper filters the list by the caller's permissions.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/catalog/#tag/Catalog-API/operation/listNamespaces)`,
		Attributes: map[string]schema.Attribute{
			"warehouse_id": schema.StringAttribute{Required: true, MarkdownDescription: "Warehouse UUID."},
			"parent":       schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Namespace levels of the parent whose children to list. Lists the warehouse's top-level namespaces when unset. Each level must be non-empty."},
			"namespaces": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Visible direct child namespaces, sorted by path.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Namespace UUID."},
					"parts": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Namespace path levels."},
				}},
			},
		},
	}
}

func (d *lakekeeperNamespacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	datasource := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = datasource.Client
}

func (d *lakekeeperNamespacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperNamespacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	warehouse := state.WarehouseID.ValueString()
	if _, err := uuid.Parse(warehouse); err != nil {
		resp.Diagnostics.AddError("Invalid warehouse_id", "warehouse_id must be a UUID")
		return
	}
	list, err := d.client.ListNamespaces(ctx, warehouse, state.Parent)
	if err != nil {
		resp.Diagnostics.AddError("Cannot list namespaces", err.Error())
		return
	}
	state.Namespaces = make([]lakekeeperNamespacesItemModel, 0, len(list))
	for _, ns := range list {
		if len(ns.Parts) != len(state.Parent)+1 || !slices.Equal(ns.Parts[:len(state.Parent)], state.Parent) {
			resp.Diagnostics.AddError("Unexpected namespace", "Catalog returned namespace "+strings.Join(ns.Parts, ".")+" outside the requested parent")
			return
		}
		if _, err := uuid.Parse(ns.ID); err != nil {
			resp.Diagnostics.AddError("Invalid namespace UUID", "Catalog returned invalid namespace UUID "+ns.ID)
			return
		}
		state.Namespaces = append(state.Namespaces, lakekeeperNamespacesItemModel{
			ID:    types.StringValue(ns.ID),
			Parts: ns.Parts,
		})
	}
	slices.SortFunc(state.Namespaces, func(a, b lakekeeperNamespacesItemModel) int {
		return slices.Compare(a.Parts, b.Parts)
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
