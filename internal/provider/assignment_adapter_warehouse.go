package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

const warehouseIDAttributeName = "warehouse_id"

// warehouseScopeAdapter is the scope adapter for warehouses. It wraps the api
// client's warehouse assignment methods and reads the warehouse from
// scope[warehouseIDAttributeName].
type warehouseScopeAdapter struct{ client *api.Client }

func newWarehouseScopeAdapter(client *api.Client) scopeAdapter {
	return warehouseScopeAdapter{client: client}
}

func (a warehouseScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetWarehouseAssignments(ctx, scope[warehouseIDAttributeName])
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("warehouse", assignments)
}

func (a warehouseScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateWarehouseAssignments(ctx, scope[warehouseIDAttributeName], grantsToAPI(grants), grantsToAPI(revokes))
}
