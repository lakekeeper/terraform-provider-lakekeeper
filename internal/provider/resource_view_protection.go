package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(NewLakekeeperViewProtectionResource)
}

// NewLakekeeperViewProtectionResource is a helper function to simplify the provider implementation.
func NewLakekeeperViewProtectionResource() resource.Resource {
	return newProtectionResource(protectionKind{
		typeName:          "view_protection",
		objectName:        "view",
		objectIDAttribute: "view_id",
		upstreamOperation: "set_view_protection",
		get: func(ctx context.Context, client *api.Client, warehouseID, viewID string) (bool, error) {
			return client.GetViewProtection(ctx, warehouseID, viewID)
		},
		set: func(ctx context.Context, client *api.Client, warehouseID, viewID string, protected bool) (bool, error) {
			return client.SetViewProtection(ctx, warehouseID, viewID, protected)
		},
	})
}
