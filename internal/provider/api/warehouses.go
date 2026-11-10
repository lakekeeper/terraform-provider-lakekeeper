package api

import (
	"context"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
	lakekeeper "github.com/lakekeeper/go-lakekeeper/pkg/client"
)

// Warehouse is a Lakekeeper warehouse.
type Warehouse struct {
	ID        string
	ProjectID string
	Name      string
	Protected bool
	Status    string // WarehouseStatus* constant
	// StorageProfile is nil if the server returned no storage profile.
	StorageProfile StorageProfile
	// DeleteProfile is nil if the server returned no delete profile.
	DeleteProfile *DeleteProfile
}

// IsActive reports whether the warehouse is active.
func (w *Warehouse) IsActive() bool { return w.Status == WarehouseStatusActive }

// CreateWarehouseRequest describes a warehouse to create.
type CreateWarehouseRequest struct {
	Name              string
	StorageProfile    StorageProfile
	StorageCredential StorageCredential
	DeleteProfile     *DeleteProfile // nil lets the server decide
}

// CreateWarehouse creates a warehouse in a project and returns its ID.
func (c *Client) CreateWarehouse(ctx context.Context, projectID string, req CreateWarehouseRequest) (string, error) {
	sp, err := storageProfileToSDK(req.StorageProfile)
	if err != nil {
		return "", err
	}
	sc, err := storageCredentialToSDK(req.StorageCredential)
	if err != nil {
		return "", err
	}
	body := managementv1.NewCreateWarehouseRequest(sp, req.Name)
	body.SetStorageCredential(sc)
	if req.DeleteProfile != nil {
		dp, err := deleteProfileToSDK(*req.DeleteProfile)
		if err != nil {
			return "", err
		}
		body.SetDeleteProfile(dp)
	}
	w, err := c.sdk.Warehouses.Create(ctx, body, lakekeeper.WithProject(projectID))
	if err != nil {
		return "", wrapErr(err)
	}
	return w.Id, nil
}

// GetWarehouse returns a warehouse.
//
// The warehouse ID selects the project, so projectID is not sent.
func (c *Client) GetWarehouse(ctx context.Context, projectID, warehouseID string) (*Warehouse, error) {
	w, err := c.sdk.Warehouses.Get(ctx, warehouseID)
	if err != nil {
		return nil, wrapErr(err)
	}
	sp, err := storageProfileFromSDK(w.StorageProfile)
	if err != nil {
		return nil, err
	}
	dp, err := deleteProfileFromSDK(w.DeleteProfile)
	if err != nil {
		return nil, err
	}
	return &Warehouse{
		ID:             w.Id,
		ProjectID:      w.ProjectId,
		Name:           w.Name,
		Protected:      w.Protected,
		Status:         string(w.Status),
		StorageProfile: sp,
		DeleteProfile:  dp,
	}, nil
}

// DeleteWarehouse deletes a warehouse. With force, the warehouse is deleted
// even if it is protected or not empty.
func (c *Client) DeleteWarehouse(ctx context.Context, projectID, warehouseID string, force bool) error {
	// The Warehouses façade has no force flag.
	resp, err := c.sdk.WarehouseAPI.DeleteWarehouse(ctx, warehouseID).Force(force).Execute()
	return wrapGenErr("delete warehouse", resp, err)
}

// ActivateWarehouse activates a warehouse.
func (c *Client) ActivateWarehouse(ctx context.Context, projectID, warehouseID string) error {
	return wrapErr(c.sdk.Warehouses.Activate(ctx, warehouseID))
}

// DeactivateWarehouse deactivates a warehouse.
func (c *Client) DeactivateWarehouse(ctx context.Context, projectID, warehouseID string) error {
	return wrapErr(c.sdk.Warehouses.Deactivate(ctx, warehouseID))
}

// RenameWarehouse renames a warehouse.
func (c *Client) RenameWarehouse(ctx context.Context, projectID, warehouseID, newName string) error {
	_, err := c.sdk.Warehouses.Rename(ctx, warehouseID, managementv1.NewRenameWarehouseRequest(newName))
	return wrapErr(err)
}

// SetWarehouseProtection sets the protection flag of a warehouse.
func (c *Client) SetWarehouseProtection(ctx context.Context, projectID, warehouseID string, protected bool) error {
	_, err := c.sdk.Warehouses.SetProtection(ctx, warehouseID, managementv1.NewSetProtectionRequest(protected))
	return wrapErr(err)
}

// UpdateWarehouseDeleteProfile replaces the delete profile of a warehouse.
func (c *Client) UpdateWarehouseDeleteProfile(ctx context.Context, projectID, warehouseID string, dp DeleteProfile) error {
	p, err := deleteProfileToSDK(dp)
	if err != nil {
		return err
	}
	_, resp, err := c.sdk.WarehouseAPI.UpdateWarehouseDeleteProfile(ctx, warehouseID).
		UpdateWarehouseDeleteProfileRequest(*managementv1.NewUpdateWarehouseDeleteProfileRequest(p)).Execute()
	return wrapGenErr("update warehouse delete profile", resp, err)
}

// UpdateWarehouseStorage replaces the storage profile and storage credential
// of a warehouse.
func (c *Client) UpdateWarehouseStorage(ctx context.Context, projectID, warehouseID string, sp StorageProfile, sc StorageCredential) error {
	p, err := storageProfileToSDK(sp)
	if err != nil {
		return err
	}
	cred, err := storageCredentialToSDK(sc)
	if err != nil {
		return err
	}
	body := managementv1.NewUpdateWarehouseStorageRequest(p)
	body.SetStorageCredential(cred)
	_, resp, err := c.sdk.WarehouseAPI.UpdateStorageProfile(ctx, warehouseID).UpdateWarehouseStorageRequest(*body).Execute()
	return wrapGenErr("update warehouse storage", resp, err)
}
