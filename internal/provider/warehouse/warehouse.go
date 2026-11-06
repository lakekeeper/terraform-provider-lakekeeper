// Package warehouse owns the Terraform side of a Lakekeeper Warehouse: the
// resource and data-source schemas, their models, and the conversions between
// those models and the provider's api types. The lakekeeper_warehouse resource
// and data source keep only lifecycle orchestration.
//
// The module's surface is:
//
//   - ResourceSchema and DataSourceSchema;
//   - ResourceModel and DataSourceModel;
//   - ResourceModel.WarehouseRequest, Refresh and DataSourceModelFromAPI.
//
// Conversions return Go errors, never framework diagnostics.
package warehouse

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// ResourceModel is the lakekeeper_warehouse resource's data model.
type ResourceModel struct {
	ID             types.String         `tfsdk:"id"` // form: project_id/warehouse_id (internal ID)
	WarehouseID    types.String         `tfsdk:"warehouse_id"`
	Name           types.String         `tfsdk:"name"`
	ProjectID      types.String         `tfsdk:"project_id"`
	Protected      types.Bool           `tfsdk:"protected"`
	Active         types.Bool           `tfsdk:"active"`
	ManagedAccess  types.Bool           `tfsdk:"managed_access"`
	DeleteProfile  *DeleteProfileModel  `tfsdk:"delete_profile"`
	StorageProfile *StorageProfileModel `tfsdk:"storage_profile"`
}

// DataSourceModel is the lakekeeper_warehouse data source's data model.
type DataSourceModel struct {
	ID             types.String                   `tfsdk:"id"` // form: project_id/warehouse_id (internal ID)
	WarehouseID    types.String                   `tfsdk:"warehouse_id"`
	Name           types.String                   `tfsdk:"name"`
	ProjectID      types.String                   `tfsdk:"project_id"`
	Protected      types.Bool                     `tfsdk:"protected"`
	Active         types.Bool                     `tfsdk:"active"`
	ManagedAccess  types.Bool                     `tfsdk:"managed_access"`
	StorageProfile *StorageProfileDataSourceModel `tfsdk:"storage_profile"`
	DeleteProfile  *DeleteProfileModel            `tfsdk:"delete_profile"`
}

// WarehouseRequest converts the resource model (the plan) into a warehouse
// request. Create sends all of it; Update sends its storage profile, storage
// credential and delete profile. The delete profile is always resolved: an
// absent one becomes hard, the schema's default.
func (m ResourceModel) WarehouseRequest() (*api.CreateWarehouseRequest, error) {
	req := api.CreateWarehouseRequest{
		Name: m.Name.ValueString(),
	}

	if m.StorageProfile == nil {
		return nil, errors.New("storage profile is required")
	}

	s, err := m.StorageProfile.selected()
	if err != nil {
		return nil, err
	}

	storage, err := s.toAPI()
	if err != nil {
		return nil, err
	}
	req.StorageProfile = storage

	storageCredential, err := s.credentialToAPI()
	if err != nil {
		return nil, err
	}
	req.StorageCredential = storageCredential

	deleteProfile, err := m.DeleteProfile.toAPI()
	if err != nil {
		return nil, err
	}
	req.DeleteProfile = deleteProfile

	return &req, nil
}

// Refresh builds the resource model from the warehouse returned by the
// server. Lakekeeper never returns the storage credential, and managed access
// is read separately, so both are taken from prior: pass the plan in Create
// and Update and the state in Read. A prior without a storage profile (for
// example right after import), or with a storage profile of another type than
// the server's, leaves the storage credential null.
func Refresh(prior ResourceModel, w *api.Warehouse) (ResourceModel, error) {
	s, err := readServerWarehouse(w)
	if err != nil {
		return ResourceModel{}, err
	}

	if prior.StorageProfile != nil {
		priorProfile, err := prior.StorageProfile.selected()
		if err != nil {
			return ResourceModel{}, err
		}

		// A storage profile whose type changed on the server (for example S3
		// to GCS) cannot hold the prior storage credential. Drop it so the
		// credential reads as null and the plan shows the drift.
		if priorProfile.kind() == s.storageProfile.kind() {
			storageCredential, err := priorProfile.credential()
			if err != nil {
				return ResourceModel{}, err
			}

			if err := s.storageProfile.addCredential(storageCredential); err != nil {
				return ResourceModel{}, err
			}
		}
	}

	return ResourceModel{
		ID:             s.id,
		WarehouseID:    s.warehouseID,
		Name:           s.name,
		ProjectID:      s.projectID,
		Protected:      s.protected,
		Active:         s.active,
		ManagedAccess:  prior.ManagedAccess,
		DeleteProfile:  s.deleteProfile,
		StorageProfile: s.storageProfile.resourceModel(),
	}, nil
}

// DataSourceModelFromAPI builds the data-source model from the warehouse
// returned by the server. It never holds a storage credential, and leaves
// ManagedAccess for the caller to set.
func DataSourceModelFromAPI(w *api.Warehouse) (DataSourceModel, error) {
	s, err := readServerWarehouse(w)
	if err != nil {
		return DataSourceModel{}, err
	}

	return DataSourceModel{
		ID:             s.id,
		WarehouseID:    s.warehouseID,
		Name:           s.name,
		ProjectID:      s.projectID,
		Protected:      s.protected,
		Active:         s.active,
		StorageProfile: s.storageProfile.dataSourceModel(),
		DeleteProfile:  s.deleteProfile,
	}, nil
}

// serverWarehouse holds what Refresh and DataSourceModelFromAPI both read
// from the server's warehouse.
type serverWarehouse struct {
	id, warehouseID, name, projectID types.String
	protected, active                types.Bool
	storageProfile                   storageProfile
	deleteProfile                    *DeleteProfileModel
}

func readServerWarehouse(w *api.Warehouse) (serverWarehouse, error) {
	if w.StorageProfile == nil {
		return serverWarehouse{}, errors.New("storage profile must be defined")
	}

	sp, err := storageProfileFromAPI(w.StorageProfile)
	if err != nil {
		return serverWarehouse{}, err
	}

	deleteProfile, err := deleteProfileFromAPI(w.DeleteProfile)
	if err != nil {
		return serverWarehouse{}, err
	}

	return serverWarehouse{
		// The internal ID; see splitInternalID in the provider package, which
		// this package cannot import.
		id:             types.StringValue(w.ProjectID + "/" + w.ID),
		warehouseID:    types.StringValue(w.ID),
		name:           types.StringValue(w.Name),
		projectID:      types.StringValue(w.ProjectID),
		protected:      types.BoolValue(w.Protected),
		active:         types.BoolValue(w.IsActive()),
		storageProfile: sp,
		deleteProfile:  deleteProfile,
	}, nil
}

// toAPI converts a delete profile into its api form and never returns nil
// without an error. An absent profile, or a null or unknown type, means hard.
func (d *DeleteProfileModel) toAPI() (*api.DeleteProfile, error) {
	if d == nil || d.Type.IsNull() || d.Type.IsUnknown() {
		return &api.DeleteProfile{Type: api.DeleteProfileTypeHard}, nil
	}

	switch d.Type.ValueString() {
	case api.DeleteProfileTypeSoft:
		return &api.DeleteProfile{Type: api.DeleteProfileTypeSoft, ExpirationSeconds: d.ExpirationSeconds.ValueInt32()}, nil
	case api.DeleteProfileTypeHard:
		return &api.DeleteProfile{Type: api.DeleteProfileTypeHard}, nil
	default:
		return nil, fmt.Errorf("incorrect delete profile definition, type must be [soft,hard]")
	}
}

// deleteProfileFromAPI converts the server's delete profile; nil stays nil.
func deleteProfileFromAPI(d *api.DeleteProfile) (*DeleteProfileModel, error) {
	if d == nil {
		return nil, nil
	}

	switch d.Type {
	case api.DeleteProfileTypeSoft:
		return &DeleteProfileModel{
			Type:              types.StringValue(api.DeleteProfileTypeSoft),
			ExpirationSeconds: types.Int32Value(d.ExpirationSeconds),
		}, nil
	case api.DeleteProfileTypeHard:
		return &DeleteProfileModel{
			Type: types.StringValue(api.DeleteProfileTypeHard),
		}, nil
	default:
		return nil, fmt.Errorf("incorrect delete profile type: %s, valid: [soft,hard]", d.Type)
	}
}
