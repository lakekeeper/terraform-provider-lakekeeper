package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(NewLakekeeperGenericTableProtectionResource)
}

// NewLakekeeperGenericTableProtectionResource is a helper function to simplify the provider implementation.
func NewLakekeeperGenericTableProtectionResource() resource.Resource {
	return newProtectionResource(protectionKind{
		typeName:          "generic_table_protection",
		objectName:        "generic table",
		objectIDAttribute: "generic_table_id",
		upstreamOperation: "set_generic_table_protection",
		get: func(ctx context.Context, client *api.Client, warehouseID, genericTableID string) (bool, error) {
			return client.GetGenericTableProtection(ctx, warehouseID, genericTableID)
		},
		set: func(ctx context.Context, client *api.Client, warehouseID, genericTableID string, protected bool) (bool, error) {
			return client.SetGenericTableProtection(ctx, warehouseID, genericTableID, protected)
		},
	})
}
