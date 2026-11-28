package provider

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ datasource.DataSource              = &lakekeeperTagDefinitionsDataSource{}
	_ datasource.DataSourceWithConfigure = &lakekeeperTagDefinitionsDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperTagDefinitionsDataSource)
}

// NewLakekeeperTagDefinitionsDataSource lists all governance tag definitions in a project.
func NewLakekeeperTagDefinitionsDataSource() datasource.DataSource {
	return &lakekeeperTagDefinitionsDataSource{}
}

type lakekeeperTagDefinitionsDataSource struct {
	client *api.Client
}

type lakekeeperTagDefinitionsDataSourceModel struct {
	ProjectID   types.String                        `tfsdk:"project_id"`
	Definitions []lakekeeperTagDefinitionsItemModel `tfsdk:"definitions"`
}

type lakekeeperTagDefinitionsItemModel struct {
	ID              types.String `tfsdk:"id"`
	TagDefinitionID types.String `tfsdk:"tag_definition_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Scope           types.Set    `tfsdk:"scope"`
	ValueKind       types.String `tfsdk:"value_kind"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

func (d *lakekeeperTagDefinitionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_definitions"
}

func (d *lakekeeperTagDefinitionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_tag_definitions`" + ` data source lists all governance tag definitions in a project, sorted by name.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag/operation/list_tag_definitions)`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project to list tag definitions for.",
				Required:            true,
			},
			"definitions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Tag definitions in the project, sorted by name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Internal ID in the form `{{project_id}}/{{tag_definition_id}}`."},
						"tag_definition_id": schema.StringAttribute{Computed: true, MarkdownDescription: "The UUID of the tag definition."},
						"name":              schema.StringAttribute{Computed: true, MarkdownDescription: "The name of the tag definition."},
						"description":       schema.StringAttribute{Computed: true, MarkdownDescription: "The description of the tag definition."},
						"scope":             schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Target types this tag definition may be applied to."},
						"value_kind":        schema.StringAttribute{Computed: true, MarkdownDescription: "How values are constrained (`marker`, `free-text`, `enumerated`)."},
						"created_at":        schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the tag definition was created."},
						"updated_at":        schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp when the tag definition was last updated."},
					},
				},
			},
		},
	}
}

func (d *lakekeeperTagDefinitionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = data.Client
}

func (d *lakekeeperTagDefinitionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperTagDefinitionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	defs, err := d.client.ListTagDefinitions(ctx, projectID, nil)
	if err != nil {
		resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to list tag definitions: "+err.Error())
		return
	}

	slices.SortFunc(defs, func(a, b api.TagDefinition) int {
		return cmp.Compare(a.Name, b.Name)
	})

	state.Definitions = make([]lakekeeperTagDefinitionsItemModel, 0, len(defs))
	for _, td := range defs {
		state.Definitions = append(state.Definitions, lakekeeperTagDefinitionsItemModel{
			ID:              types.StringValue(fmt.Sprintf("%s/%s", projectID, td.ID)),
			TagDefinitionID: types.StringValue(td.ID),
			Name:            types.StringValue(td.Name),
			Description:     types.StringPointerValue(td.Description),
			Scope:           stringSetValue(td.Scope),
			ValueKind:       types.StringValue(td.ValueKind),
			CreatedAt:       types.StringValue(td.CreatedAt),
			UpdatedAt:       types.StringPointerValue(td.UpdatedAt),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
