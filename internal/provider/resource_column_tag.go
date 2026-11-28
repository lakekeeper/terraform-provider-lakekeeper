package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperColumnTagResource)
}

// NewLakekeeperColumnTagResource returns a resource for attaching a governance tag to a table column.
func NewLakekeeperColumnTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "column_tag",
		TargetNoun:   "table column",
		OperationDoc: "set_table_column_tag",
		Parents: []tags.ParentAttr{
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
			{Name: "table_id", Description: "The UUID of the table."},
			{Name: "column_name", Description: "The name of the column."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetTableColumnTag(ctx, parents[0], parents[1], parents[2], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListTableColumnTags(ctx, parents[0], parents[1], parents[2], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteTableColumnTag(ctx, parents[0], parents[1], parents[2], tagName)
		},
	}, extractResourceClient)
}
