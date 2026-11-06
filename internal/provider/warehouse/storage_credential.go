package warehouse

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

type (
	storageCredential interface {
		toAPI() (api.StorageCredential, error)
		isEmpty() bool
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
	_ storageCredential = (*S3AccessKeyCredsModel)(nil)
	_ storageCredential = (*CloudflareR2CredsModel)(nil)
	_ storageCredential = (*AWSSystemIdentityCredsModel)(nil)

	_ storageCredential = (*AZClientCredentialsCredsModel)(nil)
	_ storageCredential = (*AZSharedAccessKeyCredsModel)(nil)
	_ storageCredential = (*AzureSystemIdentityCredsModel)(nil)

	_ storageCredential = (*GCPSystemIdentityCredsModel)(nil)
	_ storageCredential = (*GCSServiceAccountKeyCredsModel)(nil)
)

func (m *S3AccessKeyCredsModel) toAPI() (api.StorageCredential, error) {
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

func (m *S3AccessKeyCredsModel) isEmpty() bool {
	return m == nil
}

func (m *AWSSystemIdentityCredsModel) toAPI() (api.StorageCredential, error) {
	if m.ExternalID.IsNull() || m.ExternalID.IsUnknown() {
		return nil, errors.New("external_id is required")
	}

	return &api.S3SystemIdentityCredential{ExternalID: m.ExternalID.ValueString()}, nil
}

func (m *AWSSystemIdentityCredsModel) isEmpty() bool {
	return m == nil
}

func (m *CloudflareR2CredsModel) toAPI() (api.StorageCredential, error) {
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

func (m *CloudflareR2CredsModel) isEmpty() bool {
	return m == nil
}

func (m *AZClientCredentialsCredsModel) toAPI() (api.StorageCredential, error) {
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

func (m *AZClientCredentialsCredsModel) isEmpty() bool {
	return m == nil
}

func (m *AZSharedAccessKeyCredsModel) toAPI() (api.StorageCredential, error) {
	if m.Key.IsNull() || m.Key.IsUnknown() {
		return nil, errors.New("key is required")
	}

	return &api.AzureSharedAccessKeyCredential{Key: m.Key.ValueString()}, nil

}

func (m *AZSharedAccessKeyCredsModel) isEmpty() bool {
	return m == nil
}

func (m *AzureSystemIdentityCredsModel) toAPI() (api.StorageCredential, error) {
	return &api.AzureManagedIdentityCredential{}, nil
}

func (m *AzureSystemIdentityCredsModel) isEmpty() bool {
	return m == nil
}

func (m *GCSServiceAccountKeyCredsModel) toAPI() (api.StorageCredential, error) {
	if m.Key.IsNull() || m.Key.IsUnknown() {
		return nil, errors.New("key is required")
	}

	key := api.GCSServiceKey{}

	if err := json.Unmarshal([]byte(m.Key.ValueString()), &key); err != nil {
		return nil, err
	}

	return &api.GCSServiceAccountKeyCredential{Key: key}, nil
}

func (m *GCSServiceAccountKeyCredsModel) isEmpty() bool {
	return m == nil
}

func (m *GCPSystemIdentityCredsModel) toAPI() (api.StorageCredential, error) {
	return &api.GCSSystemIdentityCredential{}, nil
}

func (m *GCPSystemIdentityCredsModel) isEmpty() bool {
	return m == nil
}

// onlyOneStorageCredential returns the only non-nil argument, or an error if 0 or >1 are non-nil.
func onlyOneStorageCredential(args ...storageCredential) (storageCredential, error) {
	count := 0
	var selected storageCredential

	for _, arg := range args {
		if !arg.isEmpty() {
			count++
			selected = arg
		}
	}

	if count != 1 {
		return nil, fmt.Errorf("you can set one and only one storage credential, got %d", count)
	}

	return selected, nil
}
