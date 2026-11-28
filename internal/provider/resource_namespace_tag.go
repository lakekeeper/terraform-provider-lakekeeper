package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperNamespaceTagResource)
}

// NewLakekeeperNamespaceTagResource returns a resource for attaching a governance tag to a namespace.
func NewLakekeeperNamespaceTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "namespace_tag",
		TargetNoun:   "namespace",
		OperationDoc: "set_namespace_tag",
		Parents: []tags.ParentAttr{
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
			{Name: "namespace_id", Description: "The UUID of the namespace."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetNamespaceTag(ctx, parents[0], parents[1], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListNamespaceTags(ctx, parents[0], parents[1], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteNamespaceTag(ctx, parents[0], parents[1], tagName)
		},
	}, extractResourceClient)
}
