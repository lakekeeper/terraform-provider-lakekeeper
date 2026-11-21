package api

import (
	"context"
	"fmt"
	"net/http"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
)

// Tabular protection endpoints are addressed by warehouse ID and the table,
// view or generic-table ID. They do not take a project.

// GetTableProtection reports whether a table is protected from deletion.
func (c *Client) GetTableProtection(ctx context.Context, warehouseID, tableID string) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.GetTableProtection(ctx, warehouseID, tableID).Execute()
	return protectionFromSDK("get table protection", r, resp, err)
}

// SetTableProtection sets a table's protection flag and returns the value the
// server now holds.
func (c *Client) SetTableProtection(ctx context.Context, warehouseID, tableID string, protected bool) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.SetTableProtection(ctx, warehouseID, tableID).
		SetProtectionRequest(*managementv1.NewSetProtectionRequest(protected)).Execute()
	return protectionFromSDK("set table protection", r, resp, err)
}

// GetViewProtection reports whether a view is protected from deletion.
func (c *Client) GetViewProtection(ctx context.Context, warehouseID, viewID string) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.GetViewProtection(ctx, warehouseID, viewID).Execute()
	return protectionFromSDK("get view protection", r, resp, err)
}

// SetViewProtection sets a view's protection flag and returns the value the
// server now holds.
func (c *Client) SetViewProtection(ctx context.Context, warehouseID, viewID string, protected bool) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.SetViewProtection(ctx, warehouseID, viewID).
		SetProtectionRequest(*managementv1.NewSetProtectionRequest(protected)).Execute()
	return protectionFromSDK("set view protection", r, resp, err)
}

// GetGenericTableProtection reports whether a generic table is protected from
// deletion.
func (c *Client) GetGenericTableProtection(ctx context.Context, warehouseID, genericTableID string) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.GetGenericTableProtection(ctx, warehouseID, genericTableID).Execute()
	return protectionFromSDK("get generic table protection", r, resp, err)
}

// SetGenericTableProtection sets a generic table's protection flag and returns
// the value the server now holds.
func (c *Client) SetGenericTableProtection(ctx context.Context, warehouseID, genericTableID string, protected bool) (bool, error) {
	r, resp, err := c.sdk.WarehouseAPI.SetGenericTableProtection(ctx, warehouseID, genericTableID).
		SetProtectionRequest(*managementv1.NewSetProtectionRequest(protected)).Execute()
	return protectionFromSDK("set generic table protection", r, resp, err)
}

func protectionFromSDK(op string, r *managementv1.ProtectionResponse, resp *http.Response, err error) (bool, error) {
	if err != nil {
		return false, wrapGenErr(op, resp, err)
	}
	if r == nil {
		return false, &Error{err: fmt.Errorf("%s: response has no protection value", op)}
	}
	return r.Protected, nil
}
