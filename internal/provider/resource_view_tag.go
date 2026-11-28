package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperViewTagResource)
}

// NewLakekeeperViewTagResource returns a resource for attaching a governance tag to a view.
func NewLakekeeperViewTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "view_tag",
		TargetNoun:   "view",
		OperationDoc: "set_view_tag",
		Parents: []tags.ParentAttr{
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
			{Name: "view_id", Description: "The UUID of the view."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetViewTag(ctx, parents[0], parents[1], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListViewTags(ctx, parents[0], parents[1], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteViewTag(ctx, parents[0], parents[1], tagName)
		},
	}, extractResourceClient)
}
