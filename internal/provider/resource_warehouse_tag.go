package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/tags"
)

func init() {
	registerResource(NewLakekeeperWarehouseTagResource)
}

func extractResourceClient(providerData any) (*api.Client, error) {
	data, ok := providerData.(*LakekeeperResourceData)
	if !ok {
		return nil, fmt.Errorf("expected *LakekeeperResourceData, got: %T", providerData)
	}
	return data.Client, nil
}

// NewLakekeeperWarehouseTagResource returns a resource for attaching a governance tag to a warehouse.
func NewLakekeeperWarehouseTagResource() resource.Resource {
	return tags.NewResource(tags.Spec{
		TypeName:     "warehouse_tag",
		TargetNoun:   "warehouse",
		OperationDoc: "set_warehouse_tag",
		Parents: []tags.ParentAttr{
			{Name: "project_id", Description: "The UUID of the project."},
			{Name: "warehouse_id", Description: "The UUID of the warehouse."},
		},
		Set: func(ctx context.Context, client *api.Client, parents []string, tagName string, value *string) (*api.AppliedTag, error) {
			return client.SetWarehouseTag(ctx, parents[0], parents[1], tagName, value)
		},
		List: func(ctx context.Context, client *api.Client, parents []string) ([]api.TargetTag, error) {
			return client.ListWarehouseTags(ctx, parents[0], parents[1], false)
		},
		Delete: func(ctx context.Context, client *api.Client, parents []string, tagName string) error {
			return client.DeleteWarehouseTag(ctx, parents[0], parents[1], tagName)
		},
	}, extractResourceClient)
}
