package api

import (
	"context"

	managementv1 "github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1"
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
	opts := managementv1.CreateWarehouseOptions{
		Name:              req.Name,
		StorageProfile:    sp,
		StorageCredential: sc,
	}
	if req.DeleteProfile != nil {
		if opts.DeleteProfile, err = deleteProfileToSDK(*req.DeleteProfile); err != nil {
			return "", err
		}
	}
	w, resp, err := c.sdk.WarehouseV1(projectID).Create(ctx, &opts)
	if err != nil {
		return "", wrapErr(resp, err)
	}
	return w.ID, nil
}

// GetWarehouse returns a warehouse.
func (c *Client) GetWarehouse(ctx context.Context, projectID, warehouseID string) (*Warehouse, error) {
	w, resp, err := c.sdk.WarehouseV1(projectID).Get(ctx, warehouseID)
	if err != nil {
		return nil, wrapErr(resp, err)
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
		ID:             w.ID,
		ProjectID:      w.ProjectID,
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
	resp, err := c.sdk.WarehouseV1(projectID).Delete(ctx, warehouseID, &managementv1.DeleteWarehouseOptions{Force: new(force)})
	return wrapErr(resp, err)
}

// ActivateWarehouse activates a warehouse.
func (c *Client) ActivateWarehouse(ctx context.Context, projectID, warehouseID string) error {
	resp, err := c.sdk.WarehouseV1(projectID).Activate(ctx, warehouseID)
	return wrapErr(resp, err)
}

// DeactivateWarehouse deactivates a warehouse.
func (c *Client) DeactivateWarehouse(ctx context.Context, projectID, warehouseID string) error {
	resp, err := c.sdk.WarehouseV1(projectID).Deactivate(ctx, warehouseID)
	return wrapErr(resp, err)
}

// RenameWarehouse renames a warehouse.
func (c *Client) RenameWarehouse(ctx context.Context, projectID, warehouseID, newName string) error {
	resp, err := c.sdk.WarehouseV1(projectID).Rename(ctx, warehouseID, &managementv1.RenameWarehouseOptions{NewName: newName})
	return wrapErr(resp, err)
}

// SetWarehouseProtection sets the protection flag of a warehouse.
func (c *Client) SetWarehouseProtection(ctx context.Context, projectID, warehouseID string, protected bool) error {
	_, resp, err := c.sdk.WarehouseV1(projectID).SetWarehouseProtection(ctx, warehouseID, &managementv1.SetProtectionOptions{Protected: protected})
	return wrapErr(resp, err)
}

// UpdateWarehouseDeleteProfile replaces the delete profile of a warehouse.
func (c *Client) UpdateWarehouseDeleteProfile(ctx context.Context, projectID, warehouseID string, dp DeleteProfile) error {
	p, err := deleteProfileToSDK(dp)
	if err != nil {
		return err
	}
	resp, err := c.sdk.WarehouseV1(projectID).UpdateDeleteProfile(ctx, warehouseID, &managementv1.UpdateDeleteProfileOptions{DeleteProfile: *p})
	return wrapErr(resp, err)
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
	resp, err := c.sdk.WarehouseV1(projectID).UpdateStorageProfile(ctx, warehouseID, &managementv1.UpdateStorageProfileOptions{
		StorageProfile:    p,
		StorageCredential: &cred,
	})
	return wrapErr(resp, err)
}
