package api

import (
	"errors"
	"fmt"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
	"github.com/lakekeeper/go-lakekeeper/pkg/storage/credential"
	"github.com/lakekeeper/go-lakekeeper/pkg/storage/profile"
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

// S3 defaults sent when the user leaves a field unset. The previous SDK
// injected them on the client side; keeping them preserves what the server
// stores for existing configurations.
const (
	defaultS3Flavor                  = S3FlavorAWS
	defaultS3STSTokenValiditySeconds = 3600
	defaultS3RemoteSigningURLStyle   = RemoteSigningURLStyleAuto
)

// defaultADLSSASTokenValiditySeconds replaces an explicit 0, as the previous
// SDK did. An unset value is not sent at all.
const defaultADLSSASTokenValiditySeconds = 3600

func storageProfileToSDK(sp StorageProfile) (managementv1.StorageProfile, error) {
	switch p := sp.(type) {
	case *S3StorageProfile:
		opts := []profile.S3Option{
			profile.WithS3Flavor(profile.S3Flavor(defaultS3Flavor)),
			profile.WithS3RemoteSigningURLStyle(profile.S3UrlStyleDetectionMode(defaultS3RemoteSigningURLStyle)),
			s3STSTokenValiditySeconds(defaultS3STSTokenValiditySeconds),
			// push-s3-delete-disabled is always sent as true: it defaulted to
			// true and an explicit false was never sent.
			profile.WithS3PushS3DeleteDisabled(true),
		}
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithS3KeyPrefix(*p.KeyPrefix))
		}
		if isTrue(p.AllowAlternativeProtocols) {
			opts = append(opts, profile.WithS3AlternativeProtocols())
		}
		if isSet(p.AssumeRoleARN) {
			opts = append(opts, profile.WithS3AssumeRoleARN(*p.AssumeRoleARN))
		}
		if isSet(p.AWSKMSKeyARN) {
			opts = append(opts, profile.WithS3AWSKMSKeyARN(*p.AWSKMSKeyARN))
		}
		if isSet(p.Endpoint) {
			opts = append(opts, profile.WithS3Endpoint(*p.Endpoint))
		}
		if isSet(p.Flavor) {
			opts = append(opts, profile.WithS3Flavor(profile.S3Flavor(*p.Flavor)))
		}
		if isTrue(p.PathStyleAccess) {
			opts = append(opts, profile.WithS3PathStyleAccess())
		}
		if isSet(p.RemoteSigningURLStyle) {
			opts = append(opts, profile.WithS3RemoteSigningURLStyle(profile.S3UrlStyleDetectionMode(*p.RemoteSigningURLStyle)))
		}
		if p.STSEnabled {
			opts = append(opts, profile.WithS3STSEnabled())
		}
		if isSet(p.STSRoleARN) {
			opts = append(opts, profile.WithS3STSRoleARN(*p.STSRoleARN))
		}
		if isNonZero(p.STSTokenValiditySeconds) {
			opts = append(opts, s3STSTokenValiditySeconds(*p.STSTokenValiditySeconds))
		}
		return profile.NewS3Profile(p.Bucket, p.Region, opts...), nil

	case *ADLSStorageProfile:
		// Optional fields are omitted unless set explicitly. The Lakekeeper
		// server, like the UI, stores null for them and applies its own
		// defaults at runtime. Sending defaults instead breaks in
		// field-specific ways:
		//   - host: a non-empty host becomes CloudLocation::Custom, which reuses the
		//     dfs URI for the blob service client and breaks the blob-only "Get User
		//     Delegation Key" SAS minting (400 InvalidUri from the Windows-Azure-HDFS
		//     endpoint) for azure_system_identity and client_credentials.
		//   - authority_host: treated as immutable, so updating a warehouse stored
		//     with null fails ("Field `authority_host` cannot be updated to prevent
		//     loss of data").
		//   - sas_token_validity_seconds: Optional+Computed stored as null, so a sent
		//     3600 triggers a "provider produced inconsistent result" error.
		var opts []profile.ADLSOption
		if isSet(p.AuthorityHost) {
			opts = append(opts, profile.WithADLSAuthorityHost(*p.AuthorityHost))
		}
		if isSet(p.Host) {
			opts = append(opts, profile.WithADLSHost(*p.Host))
		}
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithADLSKeyPrefix(*p.KeyPrefix))
		}
		if p.SASTokenValiditySeconds != nil {
			v := *p.SASTokenValiditySeconds
			if v == 0 {
				v = defaultADLSSASTokenValiditySeconds
			}
			opts = append(opts, adlsSASTokenValiditySeconds(v))
		}
		if isTrue(p.AllowAlternativeProtocols) {
			opts = append(opts, profile.WithADLSAlternativeProtocols())
		}
		return profile.NewADLSProfile(p.AccountName, p.Filesystem, opts...), nil

	case *GCSStorageProfile:
		var opts []profile.GCSOption
		if isSet(p.KeyPrefix) {
			opts = append(opts, profile.WithGCSKeyPrefix(*p.KeyPrefix))
		}
		return profile.NewGCSProfile(p.Bucket, opts...), nil
	}
	return managementv1.StorageProfile{}, fmt.Errorf("unsupported storage profile type %T", sp)
}

// s3STSTokenValiditySeconds sets the STS token validity in whole seconds.
func s3STSTokenValiditySeconds(seconds int64) profile.S3Option {
	return func(s *managementv1.StorageProfileS3) { s.StsTokenValiditySeconds = &seconds }
}

// adlsSASTokenValiditySeconds sets the SAS token validity in whole seconds.
func adlsSASTokenValiditySeconds(seconds int64) profile.ADLSOption {
	return func(s *managementv1.StorageProfileAdls) { s.SetSasTokenValiditySeconds(seconds) }
}

// storageProfileFromSDK converts a storage profile read from the server. It
// returns nil if the profile is empty.
func storageProfileFromSDK(sp managementv1.StorageProfile) (StorageProfile, error) {
	if cfg, ok := profile.AsS3(sp); ok {
		out := &S3StorageProfile{
			Bucket:                    cfg.Bucket,
			Region:                    cfg.Region,
			STSEnabled:                cfg.StsEnabled,
			KeyPrefix:                 cfg.KeyPrefix.Get(),
			AllowAlternativeProtocols: cfg.AllowAlternativeProtocols.Get(),
			AssumeRoleARN:             cfg.AssumeRoleArn.Get(),
			AWSKMSKeyARN:              cfg.AwsKmsKeyArn.Get(),
			Endpoint:                  cfg.Endpoint.Get(),
			PathStyleAccess:           cfg.PathStyleAccess.Get(),
			PushS3DeleteDisabled:      cfg.PushS3DeleteDisabled,
			STSRoleARN:                cfg.StsRoleArn.Get(),
			STSTokenValiditySeconds:   cfg.StsTokenValiditySeconds,
		}
		if cfg.Flavor != nil {
			out.Flavor = new(string(*cfg.Flavor))
		}
		if cfg.RemoteSigningUrlStyle != nil {
			out.RemoteSigningURLStyle = new(string(*cfg.RemoteSigningUrlStyle))
		}
		return out, nil
	}
	if cfg, ok := profile.AsADLS(sp); ok {
		return &ADLSStorageProfile{
			AccountName:               cfg.AccountName,
			Filesystem:                cfg.Filesystem,
			Host:                      cfg.Host.Get(),
			AuthorityHost:             cfg.AuthorityHost.Get(),
			KeyPrefix:                 cfg.KeyPrefix.Get(),
			SASTokenValiditySeconds:   cfg.SasTokenValiditySeconds.Get(),
			AllowAlternativeProtocols: cfg.AllowAlternativeProtocols,
		}, nil
	}
	if cfg, ok := profile.AsGCS(sp); ok {
		return &GCSStorageProfile{
			Bucket:    cfg.Bucket,
			KeyPrefix: cfg.KeyPrefix.Get(),
		}, nil
	}
	if sp == (managementv1.StorageProfile{}) {
		return nil, nil
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

func storageCredentialToSDK(sc StorageCredential) (managementv1.StorageCredential, error) {
	switch c := sc.(type) {
	case *S3AccessKeyCredential:
		if isSet(c.ExternalID) {
			return credential.NewS3AccessKeyWithExternalID(c.AccessKeyID, c.SecretAccessKey, *c.ExternalID), nil
		}
		return credential.NewS3AccessKey(c.AccessKeyID, c.SecretAccessKey), nil
	case *S3SystemIdentityCredential:
		if c.ExternalID != "" {
			return credential.NewS3AwsSystemIdentityWithExternalID(c.ExternalID), nil
		}
		return credential.NewS3AwsSystemIdentity(), nil
	case *CloudflareR2Credential:
		return credential.NewS3CloudflareR2(c.AccessKeyID, c.SecretAccessKey, c.AccountID, c.Token), nil
	case *AzureClientCredentials:
		return credential.NewAZClientCredentials(c.TenantID, c.ClientID, c.ClientSecret), nil
	case *AzureSharedAccessKeyCredential:
		return credential.NewAZSharedAccessKey(c.Key), nil
	case *AzureManagedIdentityCredential:
		return credential.NewAZManagedIdentity(), nil
	case *GCSServiceAccountKeyCredential:
		k := c.Key
		return credential.NewGCSServiceAccountKey(managementv1.GcsServiceKey{
			AuthProviderX509CertUrl: k.AuthProviderX509CertURL,
			AuthUri:                 k.AuthURI,
			ClientEmail:             k.ClientEmail,
			ClientId:                k.ClientID,
			ClientX509CertUrl:       k.ClientX509CertURL,
			PrivateKey:              k.PrivateKey,
			PrivateKeyId:            k.PrivateKeyID,
			ProjectId:               k.ProjectID,
			TokenUri:                k.TokenURI,
			Type:                    k.Type,
			UniverseDomain:          k.UniverseDomain,
		}), nil
	case *GCSSystemIdentityCredential:
		return credential.NewGCSSystemIdentity(), nil
	}
	return managementv1.StorageCredential{}, fmt.Errorf("unsupported storage credential type %T", sc)
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

func deleteProfileToSDK(d DeleteProfile) (managementv1.TabularDeleteProfile, error) {
	switch d.Type {
	case DeleteProfileTypeHard:
		return managementv1.TabularDeleteProfileHardAsTabularDeleteProfile(managementv1.NewTabularDeleteProfileHard(DeleteProfileTypeHard)), nil
	case DeleteProfileTypeSoft:
		return managementv1.TabularDeleteProfileSoftAsTabularDeleteProfile(managementv1.NewTabularDeleteProfileSoft(int64(d.ExpirationSeconds), DeleteProfileTypeSoft)), nil
	}
	return managementv1.TabularDeleteProfile{}, fmt.Errorf("incorrect delete profile definition, type must be [soft,hard]")
}

// deleteProfileFromSDK converts a delete profile read from the server. It
// returns nil if the profile is empty.
func deleteProfileFromSDK(d managementv1.TabularDeleteProfile) (*DeleteProfile, error) {
	switch {
	case d.TabularDeleteProfileSoft != nil:
		return &DeleteProfile{Type: DeleteProfileTypeSoft, ExpirationSeconds: int32(d.TabularDeleteProfileSoft.ExpirationSeconds)}, nil
	case d.TabularDeleteProfileHard != nil:
		return &DeleteProfile{Type: DeleteProfileTypeHard}, nil
	}
	return nil, nil
}
