package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

var (
	_ datasource.DataSource              = &lakekeeperTagDefinitionDataSource{}
	_ datasource.DataSourceWithConfigure = &lakekeeperTagDefinitionDataSource{}
)

func init() {
	registerDataSource(NewLakekeeperTagDefinitionDataSource)
}

// NewLakekeeperTagDefinitionDataSource returns a data source that reads a single governance tag definition.
func NewLakekeeperTagDefinitionDataSource() datasource.DataSource {
	return &lakekeeperTagDefinitionDataSource{}
}

type lakekeeperTagDefinitionDataSource struct {
	client *api.Client
}

type lakekeeperTagDefinitionDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	TagDefinitionID types.String `tfsdk:"tag_definition_id"`
	ProjectID       types.String `tfsdk:"project_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Scope           types.Set    `tfsdk:"scope"`
	ValueKind       types.String `tfsdk:"value_kind"`
	AllowedValues   types.Set    `tfsdk:"allowed_values"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

func (d *lakekeeperTagDefinitionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_definition"
}

func (d *lakekeeperTagDefinitionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`lakekeeper_tag_definition`" + ` data source retrieves a governance tag definition by ID or name.

~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.

**Upstream API**: [Lakekeeper REST API docs](https://docs.lakekeeper.io/docs/nightly/api/management/#tag/tag/operation/get_tag_definition)`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the tag definition, in the form `{{project_id}}/{{tag_definition_id}}`.",
				Computed:            true,
			},
			"tag_definition_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the tag definition. Configure exactly one of `tag_definition_id` or `name`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ExactlyOneOf(path.MatchRoot("name")),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project the tag definition belongs to.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the tag definition. Configure exactly one of `tag_definition_id` or `name`.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the tag definition.",
				Computed:            true,
			},
			"scope": schema.SetAttribute{
				MarkdownDescription: "Target types this tag definition may be applied to.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"value_kind": schema.StringAttribute{
				MarkdownDescription: "How values of this tag definition are constrained.",
				Computed:            true,
			},
			"allowed_values": schema.SetAttribute{
				MarkdownDescription: "Permitted values for an `enumerated` tag definition.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the tag definition was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the tag definition was last updated.",
				Computed:            true,
			},
		},
	}
}

func (d *lakekeeperTagDefinitionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data := req.ProviderData.(*LakekeeperDatasourceData)
	d.client = data.Client
}

func (d *lakekeeperTagDefinitionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lakekeeperTagDefinitionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	var td *api.TagDefinition
	var err error

	if !state.TagDefinitionID.IsNull() && !state.TagDefinitionID.IsUnknown() {
		td, err = d.client.GetTagDefinition(ctx, projectID, state.TagDefinitionID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to read tag definition: "+err.Error())
			return
		}
	} else {
		name := state.Name.ValueString()
		defs, listErr := d.client.ListTagDefinitions(ctx, projectID, &name)
		if listErr != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to search tag definitions: "+listErr.Error())
			return
		}
		if len(defs) == 0 {
			resp.Diagnostics.AddError("Tag definition not found", fmt.Sprintf("No tag definition named %q was found in project %q.", name, projectID))
			return
		}
		// Fetch the single definition so allowed_values is populated.
		td, err = d.client.GetTagDefinition(ctx, projectID, defs[0].ID)
		if err != nil {
			resp.Diagnostics.AddError("Lakekeeper API error occurred", "Unable to read tag definition: "+err.Error())
			return
		}
	}

	state.ID = types.StringValue(fmt.Sprintf("%s/%s", projectID, td.ID))
	state.TagDefinitionID = types.StringValue(td.ID)
	state.Name = types.StringValue(td.Name)
	state.Description = types.StringPointerValue(td.Description)
	state.Scope = stringSetValue(td.Scope)
	state.ValueKind = types.StringValue(td.ValueKind)
	if len(td.AllowedValues) > 0 {
		state.AllowedValues = stringSetValue(td.AllowedValues)
	} else {
		state.AllowedValues = types.SetNull(types.StringType)
	}
	state.CreatedAt = types.StringValue(td.CreatedAt)
	state.UpdatedAt = types.StringPointerValue(td.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
