package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

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

// ScopeExists reports whether the warehouse exists. Warehouse IDs are global,
// and GetWarehouse answers a missing warehouse with 404 under every
// authorization backend.
func (a warehouseScopeAdapter) ScopeExists(ctx context.Context, scope assignmentScope) (bool, error) {
	_, err := a.client.GetWarehouse(ctx, "", scope[warehouseIDAttributeName])
	if api.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}
