package warehouse

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

type (
	// StorageProfileModel is the resource's storage_profile block. Exactly one
	// of S3, ADLS or GCS is set.
	StorageProfileModel struct {
		S3   *S3StorageProfileModel   `tfsdk:"s3"`
		ADLS *ADLSStorageProfileModel `tfsdk:"adls"`
		GCS  *GCSStorageProfileModel  `tfsdk:"gcs"`
	}

	// StorageProfileDataSourceModel is the data source's storage_profile
	// block. It never holds a storage credential.
	StorageProfileDataSourceModel struct {
		S3   *S3StorageProfileDataSourceModel   `tfsdk:"s3"`
		ADLS *ADLSStorageProfileDataSourceModel `tfsdk:"adls"`
		GCS  *GCSStorageProfileDataSourceModel  `tfsdk:"gcs"`
	}

	// storageProfile is one of the resource's storage profile models.
	storageProfile interface {
		credentialToAPI() (api.StorageCredential, error)
		addCredential(storageCredential) error
		credential() (storageCredential, error)
		toAPI() (api.StorageProfile, error)
		isEmpty() bool
		// resourceModel wraps the profile into the resource's storage_profile
		// block.
		resourceModel() *StorageProfileModel
		// dataSourceModel copies the profile into the data source's
		// storage_profile block, leaving out its storage credential.
		dataSourceModel() *StorageProfileDataSourceModel
	}

	S3StorageProfileModel struct {
		Bucket                    types.String         `tfsdk:"bucket"`
		Region                    types.String         `tfsdk:"region"`
		KeyPrefix                 types.String         `tfsdk:"key_prefix"`
		AllowAlternativeProtocols types.Bool           `tfsdk:"allow_alternative_protocols"`
		AssumeRoleARN             types.String         `tfsdk:"assume_role_arn"`
		AWSKMSKeyARN              types.String         `tfsdk:"aws_kms_key_arn"`
		Endpoint                  types.String         `tfsdk:"endpoint"`
		Flavor                    types.String         `tfsdk:"flavor"`
		PathStyleAccess           types.Bool           `tfsdk:"path_style_access"`
		PushS3DeleteDisabled      types.Bool           `tfsdk:"push_s3_delete_disabled"`
		RemoteSigningURLStyle     types.String         `tfsdk:"remote_signing_url_style"`
		STSEnabled                types.Bool           `tfsdk:"sts_enabled"`
		STSRoleARN                types.String         `tfsdk:"sts_role_arn"`
		STSTokenValiditySeconds   types.Int64          `tfsdk:"sts_token_validity_seconds"`
		Credential                *S3CredentialWrapper `tfsdk:"credential"`
	}

	ADLSStorageProfileModel struct {
		AccountName               types.String           `tfsdk:"account_name"`
		Filesystem                types.String           `tfsdk:"filesystem"`
		Host                      types.String           `tfsdk:"host"`
		AuthorityHost             types.String           `tfsdk:"authority_host"`
		KeyPrefix                 types.String           `tfsdk:"key_prefix"`
		SASTokenValiditySeconds   types.Int64            `tfsdk:"sas_token_validity_seconds"`
		AllowAlternativeProtocols types.Bool             `tfsdk:"allow_alternative_protocols"`
		Credential                *ADLSCredentialWrapper `tfsdk:"credential"`
	}

	GCSStorageProfileModel struct {
		Bucket     types.String          `tfsdk:"bucket"`
		KeyPrefix  types.String          `tfsdk:"key_prefix"`
		Credential *GCSCredentialWrapper `tfsdk:"credential"`
	}

	S3CredentialWrapper struct {
		AccessKey         *S3AccessKeyCredsModel       `tfsdk:"access_key"`
		CloudflareR2      *CloudflareR2CredsModel      `tfsdk:"cloudflare_r2"`
		AWSSystemIdentity *AWSSystemIdentityCredsModel `tfsdk:"aws_system_identity"`
	}

	GCSCredentialWrapper struct {
		ServiceAccountKey *GCSServiceAccountKeyCredsModel `tfsdk:"service_account_key"`
		SystemIdentity    *GCPSystemIdentityCredsModel    `tfsdk:"gcp_system_identity"`
	}

	ADLSCredentialWrapper struct {
		SharedAccessKey   *AZSharedAccessKeyCredsModel   `tfsdk:"shared_access_key"`
		ClientCredentials *AZClientCredentialsCredsModel `tfsdk:"client_credentials"`
		SystemIdentity    *AzureSystemIdentityCredsModel `tfsdk:"azure_system_identity"`
	}

	S3StorageProfileDataSourceModel struct {
		Bucket                    types.String `tfsdk:"bucket"`
		Region                    types.String `tfsdk:"region"`
		KeyPrefix                 types.String `tfsdk:"key_prefix"`
		AllowAlternativeProtocols types.Bool   `tfsdk:"allow_alternative_protocols"`
		AssumeRoleARN             types.String `tfsdk:"assume_role_arn"`
		AWSKMSKeyARN              types.String `tfsdk:"aws_kms_key_arn"`
		Endpoint                  types.String `tfsdk:"endpoint"`
		Flavor                    types.String `tfsdk:"flavor"`
		PathStyleAccess           types.Bool   `tfsdk:"path_style_access"`
		PushS3DeleteDisabled      types.Bool   `tfsdk:"push_s3_delete_disabled"`
		RemoteSigningURLStyle     types.String `tfsdk:"remote_signing_url_style"`
		STSEnabled                types.Bool   `tfsdk:"sts_enabled"`
		STSRoleARN                types.String `tfsdk:"sts_role_arn"`
		STSTokenValiditySeconds   types.Int64  `tfsdk:"sts_token_validity_seconds"`
	}

	ADLSStorageProfileDataSourceModel struct {
		AccountName               types.String `tfsdk:"account_name"`
		Filesystem                types.String `tfsdk:"filesystem"`
		Host                      types.String `tfsdk:"host"`
		AuthorityHost             types.String `tfsdk:"authority_host"`
		KeyPrefix                 types.String `tfsdk:"key_prefix"`
		SASTokenValiditySeconds   types.Int64  `tfsdk:"sas_token_validity_seconds"`
		AllowAlternativeProtocols types.Bool   `tfsdk:"allow_alternative_protocols"`
	}

	GCSStorageProfileDataSourceModel struct {
		Bucket    types.String `tfsdk:"bucket"`
		KeyPrefix types.String `tfsdk:"key_prefix"`
	}
)

var (
	_ storageProfile = (*S3StorageProfileModel)(nil)
	_ storageProfile = (*ADLSStorageProfileModel)(nil)
	_ storageProfile = (*GCSStorageProfileModel)(nil)
)

func (m *S3StorageProfileModel) toAPI() (api.StorageProfile, error) {
	if m.Bucket.IsNull() || m.Bucket.IsUnknown() {
		return nil, errors.New("bucket is required")
	}

	if m.Region.IsNull() || m.Region.IsUnknown() {
		return nil, errors.New("region is required")
	}

	return &api.S3StorageProfile{
		Bucket:                    m.Bucket.ValueString(),
		Region:                    m.Region.ValueString(),
		STSEnabled:                m.STSEnabled.ValueBool(),
		KeyPrefix:                 knownString(m.KeyPrefix),
		AllowAlternativeProtocols: knownBool(m.AllowAlternativeProtocols),
		AssumeRoleARN:             knownString(m.AssumeRoleARN),
		AWSKMSKeyARN:              knownString(m.AWSKMSKeyARN),
		Endpoint:                  knownString(m.Endpoint),
		Flavor:                    knownString(m.Flavor),
		PathStyleAccess:           knownBool(m.PathStyleAccess),
		PushS3DeleteDisabled:      knownBool(m.PushS3DeleteDisabled),
		RemoteSigningURLStyle:     knownString(m.RemoteSigningURLStyle),
		STSRoleARN:                knownString(m.STSRoleARN),
		STSTokenValiditySeconds:   knownInt64(m.STSTokenValiditySeconds),
	}, nil
}

func (m *S3StorageProfileModel) credentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	storage, err := onlyOneStorageCredential(m.Credential.AccessKey, m.Credential.CloudflareR2, m.Credential.AWSSystemIdentity)
	if err != nil {
		return nil, err
	}

	return storage.toAPI()
}

func (m *S3StorageProfileModel) isEmpty() bool {
	return m == nil
}

func (m *S3StorageProfileModel) addCredential(c storageCredential) error {
	m.Credential = &S3CredentialWrapper{}

	switch v := c.(type) {
	case *S3AccessKeyCredsModel:
		m.Credential.AccessKey = v
	case *AWSSystemIdentityCredsModel:
		m.Credential.AWSSystemIdentity = v
	case *CloudflareR2CredsModel:
		m.Credential.CloudflareR2 = v
	default:
		return fmt.Errorf("incorrect storage credential type %T", v)
	}
	return nil
}

func (m *S3StorageProfileModel) credential() (storageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return onlyOneStorageCredential(m.Credential.AccessKey, m.Credential.AWSSystemIdentity, m.Credential.CloudflareR2)
}

func (m *ADLSStorageProfileModel) toAPI() (api.StorageProfile, error) {
	if m.AccountName.IsNull() || m.AccountName.IsUnknown() {
		return nil, errors.New("account_name is required")
	}

	if m.Filesystem.IsNull() || m.Filesystem.IsUnknown() {
		return nil, errors.New("filesystem is required")
	}

	// Unset optional fields stay nil so the client omits them and the server
	// applies its own defaults; see api.ADLSStorageProfile.
	return &api.ADLSStorageProfile{
		AccountName:               m.AccountName.ValueString(),
		Filesystem:                m.Filesystem.ValueString(),
		Host:                      knownString(m.Host),
		AuthorityHost:             knownString(m.AuthorityHost),
		KeyPrefix:                 knownString(m.KeyPrefix),
		SASTokenValiditySeconds:   knownInt64(m.SASTokenValiditySeconds),
		AllowAlternativeProtocols: knownBool(m.AllowAlternativeProtocols),
	}, nil
}

func (m *ADLSStorageProfileModel) credentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	storage, err := onlyOneStorageCredential(m.Credential.SharedAccessKey, m.Credential.ClientCredentials, m.Credential.SystemIdentity)
	if err != nil {
		return nil, err
	}

	return storage.toAPI()
}

func (m *GCSStorageProfileModel) isEmpty() bool {
	return m == nil
}

func (m *GCSStorageProfileModel) addCredential(c storageCredential) error {
	m.Credential = &GCSCredentialWrapper{}

	switch v := c.(type) {
	case *GCSServiceAccountKeyCredsModel:
		m.Credential.ServiceAccountKey = v
	case *GCPSystemIdentityCredsModel:
		m.Credential.SystemIdentity = v
	default:
		return fmt.Errorf("incorrect storage credential type %T", v)
	}
	return nil
}

func (m *GCSStorageProfileModel) credential() (storageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return onlyOneStorageCredential(m.Credential.ServiceAccountKey, m.Credential.SystemIdentity)
}

func (m *GCSStorageProfileModel) toAPI() (api.StorageProfile, error) {
	if m.Bucket.IsNull() || m.Bucket.IsUnknown() {
		return nil, errors.New("bucket is required")
	}

	return &api.GCSStorageProfile{
		Bucket:    m.Bucket.ValueString(),
		KeyPrefix: knownString(m.KeyPrefix),
	}, nil
}

func (m *GCSStorageProfileModel) credentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	storageCredential, err := onlyOneStorageCredential(m.Credential.ServiceAccountKey, m.Credential.SystemIdentity)
	if err != nil {
		return nil, err
	}

	return storageCredential.toAPI()
}

func (m *ADLSStorageProfileModel) isEmpty() bool {
	return m == nil
}

func (m *ADLSStorageProfileModel) addCredential(c storageCredential) error {
	m.Credential = &ADLSCredentialWrapper{}

	switch v := c.(type) {
	case *AZClientCredentialsCredsModel:
		m.Credential.ClientCredentials = v
	case *AZSharedAccessKeyCredsModel:
		m.Credential.SharedAccessKey = v
	case *AzureSystemIdentityCredsModel:
		m.Credential.SystemIdentity = v
	default:
		return fmt.Errorf("incorrect storage credential type %T", v)
	}

	return nil
}

func (m *ADLSStorageProfileModel) credential() (storageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return onlyOneStorageCredential(m.Credential.ClientCredentials, m.Credential.SharedAccessKey, m.Credential.SystemIdentity)
}

func (m *S3StorageProfileModel) resourceModel() *StorageProfileModel {
	return &StorageProfileModel{S3: m}
}

func (m *ADLSStorageProfileModel) resourceModel() *StorageProfileModel {
	return &StorageProfileModel{ADLS: m}
}

func (m *GCSStorageProfileModel) resourceModel() *StorageProfileModel {
	return &StorageProfileModel{GCS: m}
}

func (m *S3StorageProfileModel) dataSourceModel() *StorageProfileDataSourceModel {
	return &StorageProfileDataSourceModel{S3: &S3StorageProfileDataSourceModel{
		Bucket:                    m.Bucket,
		Region:                    m.Region,
		KeyPrefix:                 m.KeyPrefix,
		AllowAlternativeProtocols: m.AllowAlternativeProtocols,
		AssumeRoleARN:             m.AssumeRoleARN,
		AWSKMSKeyARN:              m.AWSKMSKeyARN,
		Endpoint:                  m.Endpoint,
		Flavor:                    m.Flavor,
		PathStyleAccess:           m.PathStyleAccess,
		PushS3DeleteDisabled:      m.PushS3DeleteDisabled,
		RemoteSigningURLStyle:     m.RemoteSigningURLStyle,
		STSEnabled:                m.STSEnabled,
		STSRoleARN:                m.STSRoleARN,
		STSTokenValiditySeconds:   m.STSTokenValiditySeconds,
	}}
}

func (m *ADLSStorageProfileModel) dataSourceModel() *StorageProfileDataSourceModel {
	return &StorageProfileDataSourceModel{ADLS: &ADLSStorageProfileDataSourceModel{
		AccountName:               m.AccountName,
		Filesystem:                m.Filesystem,
		Host:                      m.Host,
		AuthorityHost:             m.AuthorityHost,
		KeyPrefix:                 m.KeyPrefix,
		SASTokenValiditySeconds:   m.SASTokenValiditySeconds,
		AllowAlternativeProtocols: m.AllowAlternativeProtocols,
	}}
}

func (m *GCSStorageProfileModel) dataSourceModel() *StorageProfileDataSourceModel {
	return &StorageProfileDataSourceModel{GCS: &GCSStorageProfileDataSourceModel{
		Bucket:    m.Bucket,
		KeyPrefix: m.KeyPrefix,
	}}
}

// selected returns the block's only storage profile, or an error if it holds
// none or more than one.
func (m *StorageProfileModel) selected() (storageProfile, error) {
	return onlyOneStorageProfile(m.S3, m.ADLS, m.GCS)
}

func storageProfileFromAPI(sp api.StorageProfile) (storageProfile, error) {
	switch cfg := sp.(type) {
	case nil:
		return nil, errors.New("storage profile is empty")
	case *api.S3StorageProfile:
		return &S3StorageProfileModel{
			Bucket:                    types.StringValue(cfg.Bucket),
			Region:                    types.StringValue(cfg.Region),
			STSEnabled:                types.BoolValue(cfg.STSEnabled),
			STSRoleARN:                types.StringPointerValue(cfg.STSRoleARN),
			STSTokenValiditySeconds:   types.Int64PointerValue(cfg.STSTokenValiditySeconds),
			PushS3DeleteDisabled:      types.BoolPointerValue(cfg.PushS3DeleteDisabled),
			KeyPrefix:                 types.StringPointerValue(cfg.KeyPrefix),
			AllowAlternativeProtocols: types.BoolPointerValue(cfg.AllowAlternativeProtocols),
			AssumeRoleARN:             types.StringPointerValue(cfg.AssumeRoleARN),
			AWSKMSKeyARN:              types.StringPointerValue(cfg.AWSKMSKeyARN),
			Endpoint:                  types.StringPointerValue(cfg.Endpoint),
			PathStyleAccess:           types.BoolPointerValue(cfg.PathStyleAccess),
			Flavor:                    types.StringPointerValue(cfg.Flavor),
			RemoteSigningURLStyle:     types.StringPointerValue(cfg.RemoteSigningURLStyle),
		}, nil
	case *api.ADLSStorageProfile:
		return &ADLSStorageProfileModel{
			AccountName:               types.StringValue(cfg.AccountName),
			Filesystem:                types.StringValue(cfg.Filesystem),
			Host:                      types.StringPointerValue(cfg.Host),
			AuthorityHost:             types.StringPointerValue(cfg.AuthorityHost),
			KeyPrefix:                 types.StringPointerValue(cfg.KeyPrefix),
			SASTokenValiditySeconds:   types.Int64PointerValue(cfg.SASTokenValiditySeconds),
			AllowAlternativeProtocols: types.BoolPointerValue(cfg.AllowAlternativeProtocols),
		}, nil
	case *api.GCSStorageProfile:
		return &GCSStorageProfileModel{
			Bucket:    types.StringValue(cfg.Bucket),
			KeyPrefix: types.StringPointerValue(cfg.KeyPrefix),
		}, nil
	}

	return nil, errors.New("unsupported storage profile")
}

// onlyOneStorageProfile returns the only non-nil argument, or an error if 0
// or >1 are non-nil.
func onlyOneStorageProfile(args ...storageProfile) (storageProfile, error) {
	count := 0
	var selected storageProfile

	for _, arg := range args {
		if !arg.isEmpty() {
			count++
			selected = arg
		}
	}

	if count != 1 {
		return nil, fmt.Errorf("you can set one and only one storage profile, got %d", count)
	}

	return selected, nil
}

// knownString returns nil for a null or unknown value.
func knownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return new(v.ValueString())
}

// knownBool returns nil for a null or unknown value.
func knownBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return new(v.ValueBool())
}

// knownInt64 returns nil for a null or unknown value.
func knownInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return new(v.ValueInt64())
}
