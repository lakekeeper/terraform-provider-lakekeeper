package api

import (
	"errors"
	"fmt"

	"github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1/storage/credential"
	"github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1/storage/profile"
)

// ---------------------------------------------------------------------------
// Storage profiles

// StorageProfile is one of *S3StorageProfile, *ADLSStorageProfile or
// *GCSStorageProfile.
//
// On writes, a nil optional field means "not set by the user". The client
// then omits it or sends the SDK's default, exactly as the provider did before
// this layer existed (see the comments on each conversion below).
type StorageProfile interface {
	isStorageProfile()
}

// S3StorageProfile stores warehouse data on AWS S3 or an S3-compatible service.
type S3StorageProfile struct {
	Bucket                    string
	Region                    string
	STSEnabled                bool
	KeyPrefix                 *string
	AllowAlternativeProtocols *bool
	AssumeRoleARN             *string
	AWSKMSKeyARN              *string
	Endpoint                  *string
	Flavor                    *string // S3Flavor* constant
	PathStyleAccess           *bool
	PushS3DeleteDisabled      *bool
	RemoteSigningURLStyle     *string // RemoteSigningURLStyle* constant
	STSRoleARN                *string
	STSTokenValiditySeconds   *int64
}

// ADLSStorageProfile stores warehouse data on Azure Data Lake Storage Gen2.
type ADLSStorageProfile struct {
	AccountName               string
	Filesystem                string
	Host                      *string
	AuthorityHost             *string
	KeyPrefix                 *string
	SASTokenValiditySeconds   *int64
	AllowAlternativeProtocols *bool
}

// GCSStorageProfile stores warehouse data on Google Cloud Storage.
type GCSStorageProfile struct {
	Bucket    string
	KeyPrefix *string
}

func (*S3StorageProfile) isStorageProfile()   {}
func (*ADLSStorageProfile) isStorageProfile() {}
func (*GCSStorageProfile) isStorageProfile()  {}

func isSet(s *string) bool    { return s != nil && *s != "" }
func isTrue(b *bool) bool     { return b != nil && *b }
func isNonZero(i *int64) bool { return i != nil && *i != 0 }

func storageProfileToSDK(sp StorageProfile) (profile.StorageProfile, error) {
	switch p := sp.(type) {
	case *S3StorageProfile:
		// Unset fields keep the defaults of profile.NewS3StorageSettings
		// (flavor aws, sts token validity 3600, push-s3-delete-disabled true,
		// remote signing url style auto).
		var opts []profile.S3StorageSettingsOptions
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithS3KeyPrefix(*p.KeyPrefix))
		}
		if isTrue(p.AllowAlternativeProtocols) {
			opts = append(opts, profile.WithS3AlternativeProtocols())
		}
		if isSet(p.AssumeRoleARN) {
			opts = append(opts, profile.WithAssumeRoleARN(*p.AssumeRoleARN))
		}
		if isSet(p.AWSKMSKeyARN) {
			opts = append(opts, profile.WithAWSKMSKeyARN(*p.AWSKMSKeyARN))
		}
		if isSet(p.Endpoint) {
			opts = append(opts, profile.WithEndpoint(*p.Endpoint))
		}
		if isSet(p.Flavor) {
			opts = append(opts, profile.WithFlavor(profile.S3Flavor(*p.Flavor)))
		}
		if isTrue(p.PathStyleAccess) {
			opts = append(opts, profile.WithPathStyleAccess())
		}
		if isTrue(p.PushS3DeleteDisabled) {
			opts = append(opts, profile.WithPushS3DeleteDisabled(true))
		}
		if isSet(p.RemoteSigningURLStyle) {
			opts = append(opts, profile.WithRemoteSigningURLStyle(profile.RemoteSigningURLStyle(*p.RemoteSigningURLStyle)))
		}
		if p.STSEnabled {
			opts = append(opts, profile.WithSTSEnabled())
		}
		if isSet(p.STSRoleARN) {
			opts = append(opts, profile.WithSTSRoleARN(*p.STSRoleARN))
		}
		if isNonZero(p.STSTokenValiditySeconds) {
			opts = append(opts, profile.WithSTSTokenValiditySeconds(*p.STSTokenValiditySeconds))
		}
		return profile.NewS3StorageSettings(p.Bucket, p.Region, opts...).AsProfile(), nil

	case *ADLSStorageProfile:
		var opts []profile.ADLSStorageSettingsOptions
		if isSet(p.AuthorityHost) {
			opts = append(opts, profile.WithAuthorityHost(*p.AuthorityHost))
		}
		if isSet(p.Host) {
			opts = append(opts, profile.WithHost(*p.Host))
		}
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithADLSKeyPrefix(*p.KeyPrefix))
		}
		if isNonZero(p.SASTokenValiditySeconds) {
			opts = append(opts, profile.WithSASTokenValiditySeconds(*p.SASTokenValiditySeconds))
		}
		if isTrue(p.AllowAlternativeProtocols) {
			opts = append(opts, profile.WithADLSAlternativeProtocols())
		}
		s := profile.NewADLSStorageSettings(p.AccountName, p.Filesystem, opts...)

		// NewADLSStorageSettings injects client-side defaults for several optional
		// fields (host = "dfs.core.windows.net", authority_host =
		// "https://login.microsoftonline.com", sas_token_validity_seconds = 3600).
		// The Lakekeeper server, like the UI, expects these to be omitted unless
		// explicitly set: it stores null and applies its own defaults at runtime.
		// Sending the injected defaults instead breaks in field-specific ways:
		//   - host: a non-empty host becomes CloudLocation::Custom, which reuses the
		//     dfs URI for the blob service client and breaks the blob-only "Get User
		//     Delegation Key" SAS minting (400 InvalidUri from the Windows-Azure-HDFS
		//     endpoint) for azure_system_identity and client_credentials.
		//   - authority_host: treated as immutable, so updating a warehouse stored
		//     with null fails ("Field `authority_host` cannot be updated to prevent
		//     loss of data").
		//   - sas_token_validity_seconds: Optional+Computed stored as null, so a sent
		//     3600 triggers a "provider produced inconsistent result" error.
		// Mirror the UI and omit each value when the user has not set it explicitly,
		// letting the server apply its defaults.
		if !isSet(p.Host) {
			s.Host = nil
		}
		if !isSet(p.AuthorityHost) {
			s.AuthorityHost = nil
		}
		if p.SASTokenValiditySeconds == nil {
			s.SASTokenValiditySeconds = nil
		}
		return s.AsProfile(), nil

	case *GCSStorageProfile:
		var opts []profile.GCSStorageSettingsOptions
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithGCSKeyPrefix(*p.KeyPrefix))
		}
		return profile.NewGCSStorageSettings(p.Bucket, opts...).AsProfile(), nil
	}
	return profile.StorageProfile{}, fmt.Errorf("unsupported storage profile type %T", sp)
}

// storageProfileFromSDK converts a storage profile read from the server. It
// returns nil if the profile is empty.
func storageProfileFromSDK(sp profile.StorageProfile) (StorageProfile, error) {
	if sp.StorageSettings == nil {
		return nil, nil
	}
	switch sp.StorageSettings.GetStorageFamily() {
	case profile.StorageFamilyS3:
		cfg, ok := sp.AsS3()
		if !ok {
			return nil, errors.New("invalid storage profile")
		}
		out := &S3StorageProfile{
			Bucket:                    cfg.Bucket,
			Region:                    cfg.Region,
			STSEnabled:                cfg.STSEnabled,
			KeyPrefix:                 cfg.KeyPrefix,
			AllowAlternativeProtocols: cfg.AllowAlternativeProtocols,
			AssumeRoleARN:             cfg.AssumeRoleARN,
			AWSKMSKeyARN:              cfg.AWSKMSKeyARN,
			Endpoint:                  cfg.Endpoint,
			PathStyleAccess:           cfg.PathStyleAccess,
			PushS3DeleteDisabled:      cfg.PushS3DeleteDisabled,
			STSRoleARN:                cfg.STSRoleARN,
			STSTokenValiditySeconds:   cfg.STSTokenValiditySeconds,
		}
		if cfg.Flavor != nil {
			out.Flavor = new(string(*cfg.Flavor))
		}
		if cfg.RemoteSigningURLStyle != nil {
			out.RemoteSigningURLStyle = new(string(*cfg.RemoteSigningURLStyle))
		}
		return out, nil
	case profile.StorageFamilyADLS:
		cfg, ok := sp.AsADLS()
		if !ok {
			return nil, errors.New("invalid storage profile")
		}
		return &ADLSStorageProfile{
			AccountName:               cfg.AccountName,
			Filesystem:                cfg.Filesystem,
			Host:                      cfg.Host,
			AuthorityHost:             cfg.AuthorityHost,
			KeyPrefix:                 cfg.KeyPrefix,
			SASTokenValiditySeconds:   cfg.SASTokenValiditySeconds,
			AllowAlternativeProtocols: cfg.AllowAlternativeProtocols,
		}, nil
	case profile.StorageFamilyGCS:
		cfg, ok := sp.AsGCS()
		if !ok {
			return nil, errors.New("invalid storage profile")
		}
		return &GCSStorageProfile{
			Bucket:    cfg.Bucket,
			KeyPrefix: cfg.KeyPrefix,
		}, nil
	}
	return nil, errors.New("unsupported storage profile")
}

// ---------------------------------------------------------------------------
// Storage credentials

// StorageCredential is one of the *...Credential types below. The server
// never returns credentials, so they only flow from the provider to the server.
type StorageCredential interface {
	isStorageCredential()
}

type (
	// S3AccessKeyCredential authenticates to S3 with an access key.
	S3AccessKeyCredential struct {
		AccessKeyID     string
		SecretAccessKey string
		ExternalID      *string
	}

	// S3SystemIdentityCredential authenticates to S3 with the server's AWS identity.
	S3SystemIdentityCredential struct {
		ExternalID string
	}

	// CloudflareR2Credential authenticates to Cloudflare R2.
	CloudflareR2Credential struct {
		AccessKeyID     string
		SecretAccessKey string
		AccountID       string
		Token           string
	}

	// AzureClientCredentials authenticates to ADLS with a service principal.
	AzureClientCredentials struct {
		ClientID     string
		ClientSecret string
		TenantID     string
	}

	// AzureSharedAccessKeyCredential authenticates to ADLS with a shared key.
	AzureSharedAccessKeyCredential struct {
		Key string
	}

	// AzureManagedIdentityCredential authenticates to ADLS with the server's Azure identity.
	AzureManagedIdentityCredential struct{}

	// GCSServiceAccountKeyCredential authenticates to GCS with a service account key.
	GCSServiceAccountKeyCredential struct {
		Key GCSServiceKey
	}

	// GCSSystemIdentityCredential authenticates to GCS with the server's GCP identity.
	GCSSystemIdentityCredential struct{}
)

// GCSServiceKey is a GCP service account key. Its JSON tags match the key
// file format, so a key file can be unmarshalled into it directly.
type GCSServiceKey struct {
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	AuthURI                 string `json:"auth_uri"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
	PrivateKey              string `json:"private_key"`
	PrivateKeyID            string `json:"private_key_id"`
	ProjectID               string `json:"project_id"`
	TokenURI                string `json:"token_uri"`
	Type                    string `json:"type"`
	UniverseDomain          string `json:"universe_domain"`
}

func (*S3AccessKeyCredential) isStorageCredential()          {}
func (*S3SystemIdentityCredential) isStorageCredential()     {}
func (*CloudflareR2Credential) isStorageCredential()         {}
func (*AzureClientCredentials) isStorageCredential()         {}
func (*AzureSharedAccessKeyCredential) isStorageCredential() {}
func (*AzureManagedIdentityCredential) isStorageCredential() {}
func (*GCSServiceAccountKeyCredential) isStorageCredential() {}
func (*GCSSystemIdentityCredential) isStorageCredential()    {}

func storageCredentialToSDK(sc StorageCredential) (credential.StorageCredential, error) {
	switch c := sc.(type) {
	case *S3AccessKeyCredential:
		var opts []credential.S3CredentialAccessKeyOptions
		if isSet(c.ExternalID) {
			opts = append(opts, credential.WithExternalID(*c.ExternalID))
		}
		return credential.NewS3CredentialAccessKey(c.AccessKeyID, c.SecretAccessKey, opts...).AsCredential(), nil
	case *S3SystemIdentityCredential:
		return credential.NewS3CredentialSystemIdentity(c.ExternalID).AsCredential(), nil
	case *CloudflareR2Credential:
		return credential.NewCloudflareR2Credential(c.AccessKeyID, c.SecretAccessKey, c.AccountID, c.Token).AsCredential(), nil
	case *AzureClientCredentials:
		return credential.NewAZCredentialClientCredentials(c.ClientID, c.ClientSecret, c.TenantID).AsCredential(), nil
	case *AzureSharedAccessKeyCredential:
		return credential.NewAZCredentialSharedAccessKey(c.Key).AsCredential(), nil
	case *AzureManagedIdentityCredential:
		return credential.NewAZCredentialManagedIdentity().AsCredential(), nil
	case *GCSServiceAccountKeyCredential:
		return credential.NewGCSCredentialServiceAccountKey(credential.GCSServiceKey(c.Key)).AsCredential(), nil
	case *GCSSystemIdentityCredential:
		return credential.NewGCSCredentialSystemIdentity().AsCredential(), nil
	}
	return credential.StorageCredential{}, fmt.Errorf("unsupported storage credential type %T", sc)
}

// ---------------------------------------------------------------------------
// Delete profiles

// DeleteProfile controls what happens to tables and views when they are dropped.
type DeleteProfile struct {
	Type string // DeleteProfileTypeHard or DeleteProfileTypeSoft
	// ExpirationSeconds is how long a soft-deleted entity is kept. Only used
	// for DeleteProfileTypeSoft.
	ExpirationSeconds int32
}

func deleteProfileToSDK(d DeleteProfile) (*profile.DeleteProfile, error) {
	switch d.Type {
	case DeleteProfileTypeHard:
		return profile.NewTabularDeleteProfileHard().AsProfile(), nil
	case DeleteProfileTypeSoft:
		return profile.NewTabularDeleteProfileSoft(d.ExpirationSeconds).AsProfile(), nil
	}
	return nil, fmt.Errorf("incorrect delete profile definition, type must be [soft,hard]")
}

func deleteProfileFromSDK(d *profile.DeleteProfile) (*DeleteProfile, error) {
	if d == nil || d.DeleteProfileSettings == nil {
		return nil, nil
	}
	switch p := d.DeleteProfileSettings.(type) {
	case *profile.TabularDeleteProfileSoft:
		return &DeleteProfile{Type: DeleteProfileTypeSoft, ExpirationSeconds: p.ExpirationSeconds}, nil
	case *profile.TabularDeleteProfileHard:
		return &DeleteProfile{Type: DeleteProfileTypeHard}, nil
	default:
		return nil, fmt.Errorf("incorrect delete profile type: %T, valid: [soft,hard]", p)
	}
}
