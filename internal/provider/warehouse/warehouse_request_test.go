package warehouse_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"
)

const gcsKeyJSON = `{"type":"service_account","project_id":"p","client_email":"e@p.iam.gserviceaccount.com"}`

func s3Profile(cred *warehouse.S3CredentialWrapper) *warehouse.StorageProfileModel {
	return &warehouse.StorageProfileModel{S3: &warehouse.S3StorageProfileModel{
		Bucket:                    types.StringValue("bucket"),
		Region:                    types.StringValue("eu-central-1"),
		STSEnabled:                types.BoolValue(true),
		KeyPrefix:                 types.StringValue("prefix"),
		AllowAlternativeProtocols: types.BoolUnknown(),
		Endpoint:                  types.StringNull(),
		Flavor:                    types.StringValue(api.S3FlavorS3Compat),
		PathStyleAccess:           types.BoolValue(true),
		STSTokenValiditySeconds:   types.Int64Value(60),
		Credential:                cred,
	}}
}

func adlsProfile(cred *warehouse.ADLSCredentialWrapper) *warehouse.StorageProfileModel {
	return &warehouse.StorageProfileModel{ADLS: &warehouse.ADLSStorageProfileModel{
		AccountName: types.StringValue("account"),
		Filesystem:  types.StringValue("fs"),
		Host:        types.StringUnknown(),
		KeyPrefix:   types.StringValue("prefix"),
		Credential:  cred,
	}}
}

func gcsProfile(cred *warehouse.GCSCredentialWrapper) *warehouse.StorageProfileModel {
	return &warehouse.StorageProfileModel{GCS: &warehouse.GCSStorageProfileModel{
		Bucket:     types.StringValue("bucket"),
		KeyPrefix:  types.StringNull(),
		Credential: cred,
	}}
}

var (
	wantS3Profile = &api.S3StorageProfile{
		Bucket:                  "bucket",
		Region:                  "eu-central-1",
		STSEnabled:              true,
		KeyPrefix:               new("prefix"),
		Flavor:                  new(api.S3FlavorS3Compat),
		PathStyleAccess:         new(true),
		STSTokenValiditySeconds: new(int64(60)),
	}
	wantADLSProfile = &api.ADLSStorageProfile{
		AccountName: "account",
		Filesystem:  "fs",
		KeyPrefix:   new("prefix"),
	}
	wantGCSProfile = &api.GCSStorageProfile{Bucket: "bucket"}
)

func TestWarehouseRequest_StorageProfileAndCredential(t *testing.T) {
	tests := []struct {
		name        string
		profile     *warehouse.StorageProfileModel
		wantProfile api.StorageProfile
		wantCred    api.StorageCredential
	}{
		{
			name: "s3 access key",
			profile: s3Profile(&warehouse.S3CredentialWrapper{AccessKey: &warehouse.S3AccessKeyCredsModel{
				AccessKeyID:     types.StringValue("id"),
				SecretAccessKey: types.StringValue("secret"),
				ExternalID:      types.StringValue("ext"),
			}}),
			wantProfile: wantS3Profile,
			wantCred:    &api.S3AccessKeyCredential{AccessKeyID: "id", SecretAccessKey: "secret", ExternalID: new("ext")},
		},
		{
			name: "s3 cloudflare r2",
			profile: s3Profile(&warehouse.S3CredentialWrapper{CloudflareR2: &warehouse.CloudflareR2CredsModel{
				AccessKeyID:     types.StringValue("id"),
				SecretAccessKey: types.StringValue("secret"),
				AccountID:       types.StringValue("account"),
				Token:           types.StringValue("token"),
			}}),
			wantProfile: wantS3Profile,
			wantCred:    &api.CloudflareR2Credential{AccessKeyID: "id", SecretAccessKey: "secret", AccountID: "account", Token: "token"},
		},
		{
			name: "s3 aws system identity",
			profile: s3Profile(&warehouse.S3CredentialWrapper{AWSSystemIdentity: &warehouse.AWSSystemIdentityCredsModel{
				ExternalID: types.StringValue("ext"),
			}}),
			wantProfile: wantS3Profile,
			wantCred:    &api.S3SystemIdentityCredential{ExternalID: "ext"},
		},
		{
			name: "adls shared access key",
			profile: adlsProfile(&warehouse.ADLSCredentialWrapper{SharedAccessKey: &warehouse.AZSharedAccessKeyCredsModel{
				Key: types.StringValue("key"),
			}}),
			wantProfile: wantADLSProfile,
			wantCred:    &api.AzureSharedAccessKeyCredential{Key: "key"},
		},
		{
			name: "adls client credentials",
			profile: adlsProfile(&warehouse.ADLSCredentialWrapper{ClientCredentials: &warehouse.AZClientCredentialsCredsModel{
				ClientID:     types.StringValue("client"),
				ClientSecret: types.StringValue("secret"),
				TenantID:     types.StringValue("tenant"),
			}}),
			wantProfile: wantADLSProfile,
			wantCred:    &api.AzureClientCredentials{ClientID: "client", ClientSecret: "secret", TenantID: "tenant"},
		},
		{
			name: "adls azure system identity",
			profile: adlsProfile(&warehouse.ADLSCredentialWrapper{SystemIdentity: &warehouse.AzureSystemIdentityCredsModel{
				Enabled: types.BoolValue(true),
			}}),
			wantProfile: wantADLSProfile,
			wantCred:    &api.AzureManagedIdentityCredential{},
		},
		{
			name: "gcs service account key",
			profile: gcsProfile(&warehouse.GCSCredentialWrapper{ServiceAccountKey: &warehouse.GCSServiceAccountKeyCredsModel{
				Key: types.StringValue(gcsKeyJSON),
			}}),
			wantProfile: wantGCSProfile,
			wantCred: &api.GCSServiceAccountKeyCredential{Key: api.GCSServiceKey{
				Type: "service_account", ProjectID: "p", ClientEmail: "e@p.iam.gserviceaccount.com",
			}},
		},
		{
			name: "gcs gcp system identity",
			profile: gcsProfile(&warehouse.GCSCredentialWrapper{SystemIdentity: &warehouse.GCPSystemIdentityCredsModel{
				Enabled: types.BoolValue(true),
			}}),
			wantProfile: wantGCSProfile,
			wantCred:    &api.GCSSystemIdentityCredential{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := warehouse.ResourceModel{Name: types.StringValue("wh"), StorageProfile: tt.profile}

			got, err := m.WarehouseRequest()
			if err != nil {
				t.Fatalf("WarehouseRequest: %v", err)
			}

			want := &api.CreateWarehouseRequest{Name: "wh", StorageProfile: tt.wantProfile, StorageCredential: tt.wantCred}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got  %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestWarehouseRequest_DeleteProfile(t *testing.T) {
	tests := []struct {
		name  string
		model *warehouse.DeleteProfileModel
		want  *api.DeleteProfile
	}{
		{
			name:  "soft",
			model: &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(600)},
			want:  &api.DeleteProfile{Type: api.DeleteProfileTypeSoft, ExpirationSeconds: 600},
		},
		{
			name:  "hard",
			model: &warehouse.DeleteProfileModel{Type: types.StringValue("hard"), ExpirationSeconds: types.Int32Null()},
			want:  &api.DeleteProfile{Type: api.DeleteProfileTypeHard},
		},
		{
			name:  "unknown type means hard",
			model: &warehouse.DeleteProfileModel{Type: types.StringUnknown(), ExpirationSeconds: types.Int32Unknown()},
			want:  &api.DeleteProfile{Type: api.DeleteProfileTypeHard},
		},
		{
			name:  "absent is left to the server",
			model: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := warehouse.ResourceModel{
				Name:           types.StringValue("wh"),
				StorageProfile: gcsProfile(&warehouse.GCSCredentialWrapper{SystemIdentity: &warehouse.GCPSystemIdentityCredsModel{}}),
				DeleteProfile:  tt.model,
			}

			got, err := m.WarehouseRequest()
			if err != nil {
				t.Fatalf("WarehouseRequest: %v", err)
			}
			if !reflect.DeepEqual(got.DeleteProfile, tt.want) {
				t.Errorf("delete profile: got %#v, want %#v", got.DeleteProfile, tt.want)
			}
		})
	}
}

func TestWarehouseRequest_Errors(t *testing.T) {
	accessKey := &warehouse.S3CredentialWrapper{AccessKey: &warehouse.S3AccessKeyCredsModel{
		AccessKeyID: types.StringValue("id"), SecretAccessKey: types.StringValue("secret"),
	}}
	systemIdentity := &warehouse.GCSCredentialWrapper{SystemIdentity: &warehouse.GCPSystemIdentityCredsModel{}}

	tests := []struct {
		name    string
		model   warehouse.ResourceModel
		wantErr string
	}{
		{
			name:    "no storage profile",
			model:   warehouse.ResourceModel{},
			wantErr: "storage profile is required",
		},
		{
			name:    "empty storage profile",
			model:   warehouse.ResourceModel{StorageProfile: &warehouse.StorageProfileModel{}},
			wantErr: "you can set one and only one storage profile, got 0",
		},
		{
			name: "two storage profiles",
			model: warehouse.ResourceModel{StorageProfile: &warehouse.StorageProfileModel{
				S3:  s3Profile(accessKey).S3,
				GCS: gcsProfile(systemIdentity).GCS,
			}},
			wantErr: "you can set one and only one storage profile, got 2",
		},
		{
			name: "s3 without bucket",
			model: func() warehouse.ResourceModel {
				sp := s3Profile(accessKey)
				sp.S3.Bucket = types.StringNull()
				return warehouse.ResourceModel{StorageProfile: sp}
			}(),
			wantErr: "bucket is required",
		},
		{
			name: "s3 without region",
			model: func() warehouse.ResourceModel {
				sp := s3Profile(accessKey)
				sp.S3.Region = types.StringUnknown()
				return warehouse.ResourceModel{StorageProfile: sp}
			}(),
			wantErr: "region is required",
		},
		{
			name: "adls without filesystem",
			model: func() warehouse.ResourceModel {
				sp := adlsProfile(&warehouse.ADLSCredentialWrapper{SystemIdentity: &warehouse.AzureSystemIdentityCredsModel{}})
				sp.ADLS.Filesystem = types.StringNull()
				return warehouse.ResourceModel{StorageProfile: sp}
			}(),
			wantErr: "filesystem is required",
		},
		{
			name:    "no storage credential",
			model:   warehouse.ResourceModel{StorageProfile: gcsProfile(nil)},
			wantErr: "credential is required",
		},
		{
			name:    "empty storage credential",
			model:   warehouse.ResourceModel{StorageProfile: s3Profile(&warehouse.S3CredentialWrapper{})},
			wantErr: "you can set one and only one storage credential, got 0",
		},
		{
			name: "s3 access key without secret",
			model: warehouse.ResourceModel{StorageProfile: s3Profile(&warehouse.S3CredentialWrapper{AccessKey: &warehouse.S3AccessKeyCredsModel{
				AccessKeyID: types.StringValue("id"), SecretAccessKey: types.StringNull(),
			}})},
			wantErr: "secret_access_key is required",
		},
		{
			name: "cloudflare r2 without token",
			model: warehouse.ResourceModel{StorageProfile: s3Profile(&warehouse.S3CredentialWrapper{CloudflareR2: &warehouse.CloudflareR2CredsModel{
				AccessKeyID: types.StringValue("id"), SecretAccessKey: types.StringValue("s"), AccountID: types.StringValue("a"), Token: types.StringNull(),
			}})},
			wantErr: "token is required",
		},
		{
			name: "adls client credentials without tenant",
			model: warehouse.ResourceModel{StorageProfile: adlsProfile(&warehouse.ADLSCredentialWrapper{ClientCredentials: &warehouse.AZClientCredentialsCredsModel{
				ClientID: types.StringValue("c"), ClientSecret: types.StringValue("s"), TenantID: types.StringNull(),
			}})},
			wantErr: "tenant_id is required",
		},
		{
			name: "gcs service account key that is not JSON",
			model: warehouse.ResourceModel{StorageProfile: gcsProfile(&warehouse.GCSCredentialWrapper{ServiceAccountKey: &warehouse.GCSServiceAccountKeyCredsModel{
				Key: types.StringValue("not json"),
			}})},
			wantErr: "invalid character",
		},
		{
			name: "invalid delete profile type",
			model: warehouse.ResourceModel{
				StorageProfile: gcsProfile(systemIdentity),
				DeleteProfile:  &warehouse.DeleteProfileModel{Type: types.StringValue("later")},
			},
			wantErr: "incorrect delete profile definition, type must be [soft,hard]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.model.WarehouseRequest()
			if err == nil {
				t.Fatalf("expected error containing %q, got request %#v", tt.wantErr, got)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
