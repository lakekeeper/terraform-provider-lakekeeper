package sdk

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

type (
	StorageCredsModel interface {
		ToAPI() (api.StorageCredential, error)
		IsEmpty() bool
	}

	S3AccessKeyCredsModel struct {
		AccessKeyID     types.String `tfsdk:"access_key_id"`
		SecretAccessKey types.String `tfsdk:"secret_access_key"`
		ExternalID      types.String `tfsdk:"external_id"`
	}

	AWSSystemIdentityCredsModel struct {
		ExternalID types.String `tfsdk:"external_id"`
	}

	CloudflareR2CredsModel struct {
		AccessKeyID     types.String `tfsdk:"access_key_id"`
		SecretAccessKey types.String `tfsdk:"secret_access_key"`
		AccountID       types.String `tfsdk:"account_id"`
		Token           types.String `tfsdk:"token"`
	}

	AZClientCredentialsCredsModel struct {
		ClientID     types.String `tfsdk:"client_id"`
		ClientSecret types.String `tfsdk:"client_secret"`
		TenantID     types.String `tfsdk:"tenant_id"`
	}

	AZSharedAccessKeyCredsModel struct {
		Key types.String `tfsdk:"key"`
	}

	GCSServiceAccountKeyCredsModel struct {
		Key types.String `tfsdk:"key"`
	}

	AzureSystemIdentityCredsModel struct {
		Enabled types.Bool `tfsdk:"enabled"`
	}
	GCPSystemIdentityCredsModel struct {
		Enabled types.Bool `tfsdk:"enabled"`
	}
)

var (
	_ StorageCredsModel = (*S3AccessKeyCredsModel)(nil)
	_ StorageCredsModel = (*CloudflareR2CredsModel)(nil)
	_ StorageCredsModel = (*AWSSystemIdentityCredsModel)(nil)

	_ StorageCredsModel = (*AZClientCredentialsCredsModel)(nil)
	_ StorageCredsModel = (*AZSharedAccessKeyCredsModel)(nil)
	_ StorageCredsModel = (*AzureSystemIdentityCredsModel)(nil)

	_ StorageCredsModel = (*GCPSystemIdentityCredsModel)(nil)
	_ StorageCredsModel = (*GCSServiceAccountKeyCredsModel)(nil)
)

func (m *S3AccessKeyCredsModel) ToAPI() (api.StorageCredential, error) {
	if m.AccessKeyID.IsNull() || m.AccessKeyID.IsUnknown() {
		return nil, errors.New("access_key_id is required")
	}

	if m.SecretAccessKey.IsNull() || m.SecretAccessKey.IsUnknown() {
		return nil, errors.New("secret_access_key is required")
	}

	return &api.S3AccessKeyCredential{
		AccessKeyID:     m.AccessKeyID.ValueString(),
		SecretAccessKey: m.SecretAccessKey.ValueString(),
		ExternalID:      knownString(m.ExternalID),
	}, nil
}

func (m *S3AccessKeyCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *AWSSystemIdentityCredsModel) ToAPI() (api.StorageCredential, error) {
	if m.ExternalID.IsNull() || m.ExternalID.IsUnknown() {
		return nil, errors.New("external_id is required")
	}

	return &api.S3SystemIdentityCredential{ExternalID: m.ExternalID.ValueString()}, nil
}

func (m *AWSSystemIdentityCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *CloudflareR2CredsModel) ToAPI() (api.StorageCredential, error) {
	if m.AccessKeyID.IsNull() || m.AccessKeyID.IsUnknown() {
		return nil, errors.New("access_key_id is required")
	}

	if m.SecretAccessKey.IsNull() || m.SecretAccessKey.IsUnknown() {
		return nil, errors.New("secret_access_key is required")
	}

	if m.AccountID.IsNull() || m.AccountID.IsUnknown() {
		return nil, errors.New("account_id is required")
	}

	if m.Token.IsNull() || m.Token.IsUnknown() {
		return nil, errors.New("token is required")
	}

	return &api.CloudflareR2Credential{
		AccessKeyID:     m.AccessKeyID.ValueString(),
		SecretAccessKey: m.SecretAccessKey.ValueString(),
		AccountID:       m.AccountID.ValueString(),
		Token:           m.Token.ValueString(),
	}, nil
}

func (m *CloudflareR2CredsModel) IsEmpty() bool {
	return m == nil
}

func (m *AZClientCredentialsCredsModel) ToAPI() (api.StorageCredential, error) {
	if m.ClientID.IsNull() || m.ClientID.IsUnknown() {
		return nil, errors.New("client_id is required")
	}

	if m.ClientSecret.IsNull() || m.ClientSecret.IsUnknown() {
		return nil, errors.New("client_secret is required")
	}

	if m.TenantID.IsNull() || m.TenantID.IsUnknown() {
		return nil, errors.New("tenant_id is required")
	}

	return &api.AzureClientCredentials{
		ClientID:     m.ClientID.ValueString(),
		ClientSecret: m.ClientSecret.ValueString(),
		TenantID:     m.TenantID.ValueString(),
	}, nil
}

func (m *AZClientCredentialsCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *AZSharedAccessKeyCredsModel) ToAPI() (api.StorageCredential, error) {
	if m.Key.IsNull() || m.Key.IsUnknown() {
		return nil, errors.New("key is required")
	}

	return &api.AzureSharedAccessKeyCredential{Key: m.Key.ValueString()}, nil

}

func (m *AZSharedAccessKeyCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *AzureSystemIdentityCredsModel) ToAPI() (api.StorageCredential, error) {
	return &api.AzureManagedIdentityCredential{}, nil
}

func (m *AzureSystemIdentityCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *GCSServiceAccountKeyCredsModel) ToAPI() (api.StorageCredential, error) {
	if m.Key.IsNull() || m.Key.IsUnknown() {
		return nil, errors.New("key is required")
	}

	key := api.GCSServiceKey{}

	if err := json.Unmarshal([]byte(m.Key.ValueString()), &key); err != nil {
		return nil, err
	}

	return &api.GCSServiceAccountKeyCredential{Key: key}, nil
}

func (m *GCSServiceAccountKeyCredsModel) IsEmpty() bool {
	return m == nil
}

func (m *GCPSystemIdentityCredsModel) ToAPI() (api.StorageCredential, error) {
	return &api.GCSSystemIdentityCredential{}, nil
}

func (m *GCPSystemIdentityCredsModel) IsEmpty() bool {
	return m == nil
}

// OnlyOne returns the only non-nil argument, or an error if 0 or >1 are non-nil.
func OnlyOneStorageCredential(args ...StorageCredsModel) (StorageCredsModel, error) {
	count := 0
	var selected StorageCredsModel

	for _, arg := range args {
		if !arg.IsEmpty() {
			count++
			selected = arg
		}
	}

	if count != 1 {
		return nil, fmt.Errorf("you can set one and only one storage credential, got %d", count)
	}

	return selected, nil
}
