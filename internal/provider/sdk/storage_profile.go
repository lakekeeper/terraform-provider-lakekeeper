package sdk

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

type (
	StorageProfileModel interface {
		CredentialToAPI() (api.StorageCredential, error)
		AddCreds(StorageCredsModel) error
		GetCredentials() (StorageCredsModel, error)
		ToAPI() (api.StorageProfile, error)
		IsEmpty() bool
	}

	StorageProfileDataSourceModel interface {
		FromResourceModel(StorageProfileModel) error
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
	_ StorageProfileModel = (*S3StorageProfileModel)(nil)
	_ StorageProfileModel = (*ADLSStorageProfileModel)(nil)
	_ StorageProfileModel = (*GCSStorageProfileModel)(nil)

	_ StorageProfileDataSourceModel = (*S3StorageProfileDataSourceModel)(nil)
	_ StorageProfileDataSourceModel = (*ADLSStorageProfileDataSourceModel)(nil)
	_ StorageProfileDataSourceModel = (*GCSStorageProfileDataSourceModel)(nil)
)

func (m *S3StorageProfileModel) ToAPI() (api.StorageProfile, error) {
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

func (m *S3StorageProfileModel) CredentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	storage, err := OnlyOneStorageCredential(m.Credential.AccessKey, m.Credential.CloudflareR2, m.Credential.AWSSystemIdentity)
	if err != nil {
		return nil, err
	}

	return storage.ToAPI()
}

func (m *S3StorageProfileModel) IsEmpty() bool {
	return m == nil
}

func (m *S3StorageProfileModel) AddCreds(c StorageCredsModel) error {
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

func (m *S3StorageProfileModel) GetCredentials() (StorageCredsModel, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return OnlyOneStorageCredential(m.Credential.AccessKey, m.Credential.AWSSystemIdentity, m.Credential.CloudflareR2)
}

func (m *ADLSStorageProfileModel) ToAPI() (api.StorageProfile, error) {
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

func (m *ADLSStorageProfileModel) CredentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	storage, err := OnlyOneStorageCredential(m.Credential.SharedAccessKey, m.Credential.ClientCredentials, m.Credential.SystemIdentity)
	if err != nil {
		return nil, err
	}

	return storage.ToAPI()
}

func (m *GCSStorageProfileModel) IsEmpty() bool {
	return m == nil
}

func (m *GCSStorageProfileModel) AddCreds(c StorageCredsModel) error {
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

func (m *GCSStorageProfileModel) GetCredentials() (StorageCredsModel, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return OnlyOneStorageCredential(m.Credential.ServiceAccountKey, m.Credential.SystemIdentity)
}

func (m *GCSStorageProfileModel) ToAPI() (api.StorageProfile, error) {
	if m.Bucket.IsNull() || m.Bucket.IsUnknown() {
		return nil, errors.New("bucket is required")
	}

	return &api.GCSStorageProfile{
		Bucket:    m.Bucket.ValueString(),
		KeyPrefix: knownString(m.KeyPrefix),
	}, nil
}

func (m *GCSStorageProfileModel) CredentialToAPI() (api.StorageCredential, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	creds, err := OnlyOneStorageCredential(m.Credential.ServiceAccountKey, m.Credential.SystemIdentity)
	if err != nil {
		return nil, err
	}

	return creds.ToAPI()
}

func (m *ADLSStorageProfileModel) IsEmpty() bool {
	return m == nil
}

func (m *ADLSStorageProfileModel) AddCreds(c StorageCredsModel) error {
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

func (m *ADLSStorageProfileModel) GetCredentials() (StorageCredsModel, error) {
	if m.Credential == nil {
		return nil, errors.New("credential is required")
	}

	return OnlyOneStorageCredential(m.Credential.ClientCredentials, m.Credential.SharedAccessKey, m.Credential.SystemIdentity)
}

func (m *S3StorageProfileDataSourceModel) FromResourceModel(s StorageProfileModel) error {
	switch v := s.(type) {
	case *S3StorageProfileModel:
		m.Bucket = v.Bucket
		m.Region = v.Region
		m.KeyPrefix = v.KeyPrefix
		m.AllowAlternativeProtocols = v.AllowAlternativeProtocols
		m.AssumeRoleARN = v.AssumeRoleARN
		m.AWSKMSKeyARN = v.AWSKMSKeyARN
		m.Endpoint = v.Endpoint
		m.Flavor = v.Flavor
		m.PathStyleAccess = v.PathStyleAccess
		m.PushS3DeleteDisabled = v.PushS3DeleteDisabled
		m.RemoteSigningURLStyle = v.RemoteSigningURLStyle
		m.STSEnabled = v.STSEnabled
		m.STSRoleARN = v.STSRoleARN
		m.STSTokenValiditySeconds = v.STSTokenValiditySeconds
	default:
		return fmt.Errorf("incorrect storage profile type %T", v)
	}

	return nil
}

func (m *ADLSStorageProfileDataSourceModel) FromResourceModel(s StorageProfileModel) error {
	switch v := s.(type) {
	case *ADLSStorageProfileModel:
		m.AccountName = v.AccountName
		m.Filesystem = v.Filesystem
		m.Host = v.Host
		m.AuthorityHost = v.AuthorityHost
		m.KeyPrefix = v.KeyPrefix
		m.SASTokenValiditySeconds = v.SASTokenValiditySeconds
		m.AllowAlternativeProtocols = v.AllowAlternativeProtocols
	default:
		return fmt.Errorf("incorrect storage profile type %T", v)
	}

	return nil
}

func (m *GCSStorageProfileDataSourceModel) FromResourceModel(s StorageProfileModel) error {
	switch v := s.(type) {
	case *GCSStorageProfileModel:
		m.Bucket = v.Bucket
		m.KeyPrefix = v.KeyPrefix
	default:
		return fmt.Errorf("incorrect storage profile type %T", v)
	}

	return nil
}

func StorageProfileModelFromAPI(sp api.StorageProfile) (StorageProfileModel, error) {
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

// OnlyOne returns the only non-nil argument, or an error if 0 or >1 are non-nil.
func OnlyOneStorageProfile(args ...StorageProfileModel) (StorageProfileModel, error) {
	count := 0
	var selected StorageProfileModel

	for _, arg := range args {
		if !arg.IsEmpty() {
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
