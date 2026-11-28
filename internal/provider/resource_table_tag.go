package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperTableTagResource)
}

// NewLakekeeperTableTagResource returns a resource for attaching a governance tag to a table.
func NewLakekeeperTableTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "table_tag",
		TargetNoun:   "table",
		OperationDoc: "set_table_tag",
		Parents: []tags.ParentAttr{
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
			{Name: "table_id", Description: "The UUID of the table."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetTableTag(ctx, parents[0], parents[1], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListTableTags(ctx, parents[0], parents[1], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteTableTag(ctx, parents[0], parents[1], tagName)
		},
	}, extractResourceClient)
}
