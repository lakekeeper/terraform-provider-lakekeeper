package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(NewLakekeeperTableProtectionResource)
}

// NewLakekeeperTableProtectionResource is a helper function to simplify the provider implementation.
func NewLakekeeperTableProtectionResource() resource.Resource {
	return newProtectionResource(protectionKind{
		typeName:          "table_protection",
		objectName:        "table",
		objectIDAttribute: "table_id",
		upstreamOperation: "set_table_protection",
		get: func(ctx context.Context, client *api.Client, warehouseID, tableID string) (bool, error) {
			return client.GetTableProtection(ctx, warehouseID, tableID)
		},
		set: func(ctx context.Context, client *api.Client, warehouseID, tableID string, protected bool) (bool, error) {
			return client.SetTableProtection(ctx, warehouseID, tableID, protected)
		},
	})
}
