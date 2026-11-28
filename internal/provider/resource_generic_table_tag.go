package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperGenericTableTagResource)
}

// NewLakekeeperGenericTableTagResource returns a resource for attaching a governance tag to a generic table.
func NewLakekeeperGenericTableTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "generic_table_tag",
		TargetNoun:   "generic table",
		OperationDoc: "set_generic_table_tag",
		Parents: []tags.ParentAttr{
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
			{Name: "generic_table_id", Description: "The UUID of the generic table."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetGenericTableTag(ctx, parents[0], parents[1], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListGenericTableTags(ctx, parents[0], parents[1], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteGenericTableTag(ctx, parents[0], parents[1], tagName)
		},
	}, extractResourceClient)
}
