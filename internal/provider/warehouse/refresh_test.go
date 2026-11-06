package warehouse_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"
)

func serverWarehouse(sp api.StorageProfile, dp *api.DeleteProfile) *api.Warehouse {
	return &api.Warehouse{
		ID:             "wh-id",
		ProjectID:      "project-id",
		Name:           "wh",
		Protected:      true,
		Status:         api.WarehouseStatusActive,
		StorageProfile: sp,
		DeleteProfile:  dp,
	}
}

func serverS3() *api.S3StorageProfile {
	return &api.S3StorageProfile{
		Bucket:                    "bucket",
		Region:                    "eu-central-1",
		STSEnabled:                true,
		KeyPrefix:                 new("prefix"),
		AllowAlternativeProtocols: new(false),
		Endpoint:                  new("http://minio:9000/"),
		Flavor:                    new(api.S3FlavorS3Compat),
		PathStyleAccess:           new(true),
		PushS3DeleteDisabled:      new(true),
		RemoteSigningURLStyle:     new(api.RemoteSigningURLStyleAuto),
		STSTokenValiditySeconds:   new(int64(3600)),
	}
}

// refreshedS3 is serverS3 as a resource storage profile, without credential.
func refreshedS3() *warehouse.S3StorageProfileModel {
	return &warehouse.S3StorageProfileModel{
		Bucket:                    types.StringValue("bucket"),
		Region:                    types.StringValue("eu-central-1"),
		STSEnabled:                types.BoolValue(true),
		KeyPrefix:                 types.StringValue("prefix"),
		AllowAlternativeProtocols: types.BoolValue(false),
		AssumeRoleARN:             types.StringNull(),
		AWSKMSKeyARN:              types.StringNull(),
		Endpoint:                  types.StringValue("http://minio:9000/"),
		Flavor:                    types.StringValue(api.S3FlavorS3Compat),
		PathStyleAccess:           types.BoolValue(true),
		PushS3DeleteDisabled:      types.BoolValue(true),
		RemoteSigningURLStyle:     types.StringValue(api.RemoteSigningURLStyleAuto),
		STSRoleARN:                types.StringNull(),
		STSTokenValiditySeconds:   types.Int64Value(3600),
	}
}

// Refresh's prior is the plan in Create and Update and the state in Read. The
// tests below build prior directly, so each case covers every caller: a prior
// with a storage credential stands for a plan or a state, and a prior holding
// only IDs stands for the state right after import.

func TestRefresh_TopLevelAttributes(t *testing.T) {
	w := serverWarehouse(&api.GCSStorageProfile{Bucket: "bucket"}, nil)
	w.Status = api.WarehouseStatusInactive

	got, err := warehouse.Refresh(warehouse.ResourceModel{}, w)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	want := warehouse.ResourceModel{
		ID:          types.StringValue("project-id/wh-id"),
		WarehouseID: types.StringValue("wh-id"),
		Name:        types.StringValue("wh"),
		ProjectID:   types.StringValue("project-id"),
		Protected:   types.BoolValue(true),
		Active:      types.BoolValue(false),
		StorageProfile: &warehouse.StorageProfileModel{GCS: &warehouse.GCSStorageProfileModel{
			Bucket:    types.StringValue("bucket"),
			KeyPrefix: types.StringNull(),
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestRefresh_KeepsPriorStorageCredential(t *testing.T) {
	accessKey := &warehouse.S3AccessKeyCredsModel{
		AccessKeyID:     types.StringValue("id"),
		SecretAccessKey: types.StringValue("secret"),
		ExternalID:      types.StringNull(),
	}
	prior := warehouse.ResourceModel{StorageProfile: s3Profile(&warehouse.S3CredentialWrapper{AccessKey: accessKey})}

	got, err := warehouse.Refresh(prior, serverWarehouse(serverS3(), nil))
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	want := refreshedS3()
	want.Credential = &warehouse.S3CredentialWrapper{AccessKey: accessKey}
	if !reflect.DeepEqual(got.StorageProfile, &warehouse.StorageProfileModel{S3: want}) {
		t.Errorf("storage profile:\ngot  %+v\nwant %+v", got.StorageProfile.S3, want)
	}
}

func TestRefresh_KeepsPriorStorageCredentialPerBackend(t *testing.T) {
	tests := []struct {
		name   string
		prior  *warehouse.StorageProfileModel
		server api.StorageProfile
		check  func(t *testing.T, sp *warehouse.StorageProfileModel)
	}{
		{
			name:   "adls client credentials",
			prior:  adlsProfile(&warehouse.ADLSCredentialWrapper{ClientCredentials: &warehouse.AZClientCredentialsCredsModel{ClientID: types.StringValue("c")}}),
			server: &api.ADLSStorageProfile{AccountName: "account", Filesystem: "fs"},
			check: func(t *testing.T, sp *warehouse.StorageProfileModel) {
				if sp.ADLS == nil || sp.ADLS.Credential == nil || sp.ADLS.Credential.ClientCredentials == nil ||
					sp.ADLS.Credential.ClientCredentials.ClientID.ValueString() != "c" {
					t.Errorf("client credentials not kept: %+v", sp.ADLS)
				}
			},
		},
		{
			name:   "gcs service account key",
			prior:  gcsProfile(&warehouse.GCSCredentialWrapper{ServiceAccountKey: &warehouse.GCSServiceAccountKeyCredsModel{Key: types.StringValue(gcsKeyJSON)}}),
			server: &api.GCSStorageProfile{Bucket: "bucket"},
			check: func(t *testing.T, sp *warehouse.StorageProfileModel) {
				if sp.GCS == nil || sp.GCS.Credential == nil || sp.GCS.Credential.ServiceAccountKey == nil ||
					sp.GCS.Credential.ServiceAccountKey.Key.ValueString() != gcsKeyJSON {
					t.Errorf("service account key not kept: %+v", sp.GCS)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := warehouse.Refresh(warehouse.ResourceModel{StorageProfile: tt.prior}, serverWarehouse(tt.server, nil))
			if err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			tt.check(t, got.StorageProfile)
		})
	}
}

func TestRefresh_WithoutPriorStorageProfileLeavesCredentialNull(t *testing.T) {
	// After import the state holds only the IDs.
	prior := warehouse.ResourceModel{
		ID:          types.StringValue("project-id/wh-id"),
		ProjectID:   types.StringValue("project-id"),
		WarehouseID: types.StringValue("wh-id"),
	}

	got, err := warehouse.Refresh(prior, serverWarehouse(serverS3(), nil))
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if !reflect.DeepEqual(got.StorageProfile, &warehouse.StorageProfileModel{S3: refreshedS3()}) {
		t.Errorf("storage profile:\ngot  %+v\nwant %+v", got.StorageProfile.S3, refreshedS3())
	}
}

func TestRefresh_CarriesManagedAccessFromPrior(t *testing.T) {
	for _, prior := range []types.Bool{types.BoolValue(true), types.BoolValue(false), types.BoolNull()} {
		got, err := warehouse.Refresh(warehouse.ResourceModel{ManagedAccess: prior}, serverWarehouse(&api.GCSStorageProfile{Bucket: "b"}, nil))
		if err != nil {
			t.Fatalf("Refresh: %v", err)
		}
		if !got.ManagedAccess.Equal(prior) {
			t.Errorf("managed_access: got %v, want %v", got.ManagedAccess, prior)
		}
	}
}

func TestRefresh_DeleteProfile(t *testing.T) {
	tests := []struct {
		name   string
		server *api.DeleteProfile
		want   *warehouse.DeleteProfileModel
	}{
		{
			name:   "soft",
			server: &api.DeleteProfile{Type: api.DeleteProfileTypeSoft, ExpirationSeconds: 600},
			want:   &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(600)},
		},
		{
			name:   "hard",
			server: &api.DeleteProfile{Type: api.DeleteProfileTypeHard},
			want:   &warehouse.DeleteProfileModel{Type: types.StringValue("hard")},
		},
		{
			name:   "absent",
			server: nil,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := warehouse.Refresh(warehouse.ResourceModel{}, serverWarehouse(&api.GCSStorageProfile{Bucket: "b"}, tt.server))
			if err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			if !reflect.DeepEqual(got.DeleteProfile, tt.want) {
				t.Errorf("delete profile: got %+v, want %+v", got.DeleteProfile, tt.want)
			}
		})
	}
}

func TestRefresh_StorageProfileTypeChangedOnServerDropsCredential(t *testing.T) {
	s3AccessKey := s3Profile(&warehouse.S3CredentialWrapper{AccessKey: &warehouse.S3AccessKeyCredsModel{
		AccessKeyID:     types.StringValue("id"),
		SecretAccessKey: types.StringValue("secret"),
	}})
	adlsClientCredentials := adlsProfile(&warehouse.ADLSCredentialWrapper{ClientCredentials: &warehouse.AZClientCredentialsCredsModel{ClientID: types.StringValue("c")}})
	gcsSystemIdentity := gcsProfile(&warehouse.GCSCredentialWrapper{SystemIdentity: &warehouse.GCPSystemIdentityCredsModel{}})

	tests := []struct {
		name   string
		prior  *warehouse.StorageProfileModel
		server api.StorageProfile
		want   *warehouse.StorageProfileModel
	}{
		{
			name:   "s3 to gcs",
			prior:  s3AccessKey,
			server: &api.GCSStorageProfile{Bucket: "bucket"},
			want: &warehouse.StorageProfileModel{GCS: &warehouse.GCSStorageProfileModel{
				Bucket:    types.StringValue("bucket"),
				KeyPrefix: types.StringNull(),
			}},
		},
		{
			name:   "adls to s3",
			prior:  adlsClientCredentials,
			server: serverS3(),
			want:   &warehouse.StorageProfileModel{S3: refreshedS3()},
		},
		{
			name:   "gcs to s3",
			prior:  gcsSystemIdentity,
			server: serverS3(),
			want:   &warehouse.StorageProfileModel{S3: refreshedS3()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := warehouse.Refresh(warehouse.ResourceModel{StorageProfile: tt.prior}, serverWarehouse(tt.server, nil))
			if err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			if !reflect.DeepEqual(got.StorageProfile, tt.want) {
				t.Errorf("storage profile:\ngot  %+v\nwant %+v", got.StorageProfile, tt.want)
			}
		})
	}
}

func TestRefresh_Errors(t *testing.T) {
	tests := []struct {
		name    string
		prior   warehouse.ResourceModel
		w       *api.Warehouse
		wantErr string
	}{
		{
			name:    "server returned no storage profile",
			w:       serverWarehouse(nil, nil),
			wantErr: "storage profile must be defined",
		},
		{
			name:    "unknown delete profile type",
			w:       serverWarehouse(&api.GCSStorageProfile{Bucket: "b"}, &api.DeleteProfile{Type: "later"}),
			wantErr: "incorrect delete profile type: later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := warehouse.Refresh(tt.prior, tt.w)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDataSourceModelFromAPI(t *testing.T) {
	tests := []struct {
		name   string
		server api.StorageProfile
		want   *warehouse.StorageProfileDataSourceModel
	}{
		{
			name:   "s3",
			server: serverS3(),
			want: &warehouse.StorageProfileDataSourceModel{S3: &warehouse.S3StorageProfileDataSourceModel{
				Bucket:                    types.StringValue("bucket"),
				Region:                    types.StringValue("eu-central-1"),
				STSEnabled:                types.BoolValue(true),
				KeyPrefix:                 types.StringValue("prefix"),
				AllowAlternativeProtocols: types.BoolValue(false),
				AssumeRoleARN:             types.StringNull(),
				AWSKMSKeyARN:              types.StringNull(),
				Endpoint:                  types.StringValue("http://minio:9000/"),
				Flavor:                    types.StringValue(api.S3FlavorS3Compat),
				PathStyleAccess:           types.BoolValue(true),
				PushS3DeleteDisabled:      types.BoolValue(true),
				RemoteSigningURLStyle:     types.StringValue(api.RemoteSigningURLStyleAuto),
				STSRoleARN:                types.StringNull(),
				STSTokenValiditySeconds:   types.Int64Value(3600),
			}},
		},
		{
			name: "adls",
			server: &api.ADLSStorageProfile{
				AccountName:               "account",
				Filesystem:                "fs",
				Host:                      new("dfs.core.windows.net"),
				AuthorityHost:             new("https://login.microsoftonline.com"),
				SASTokenValiditySeconds:   new(int64(3600)),
				AllowAlternativeProtocols: new(false),
			},
			want: &warehouse.StorageProfileDataSourceModel{ADLS: &warehouse.ADLSStorageProfileDataSourceModel{
				AccountName:               types.StringValue("account"),
				Filesystem:                types.StringValue("fs"),
				Host:                      types.StringValue("dfs.core.windows.net"),
				AuthorityHost:             types.StringValue("https://login.microsoftonline.com"),
				KeyPrefix:                 types.StringNull(),
				SASTokenValiditySeconds:   types.Int64Value(3600),
				AllowAlternativeProtocols: types.BoolValue(false),
			}},
		},
		{
			name:   "gcs",
			server: &api.GCSStorageProfile{Bucket: "bucket", KeyPrefix: new("prefix")},
			want: &warehouse.StorageProfileDataSourceModel{GCS: &warehouse.GCSStorageProfileDataSourceModel{
				Bucket:    types.StringValue("bucket"),
				KeyPrefix: types.StringValue("prefix"),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := warehouse.DataSourceModelFromAPI(serverWarehouse(tt.server, &api.DeleteProfile{Type: api.DeleteProfileTypeSoft, ExpirationSeconds: 60}))
			if err != nil {
				t.Fatalf("DataSourceModelFromAPI: %v", err)
			}

			want := warehouse.DataSourceModel{
				ID:             types.StringValue("project-id/wh-id"),
				WarehouseID:    types.StringValue("wh-id"),
				Name:           types.StringValue("wh"),
				ProjectID:      types.StringValue("project-id"),
				Protected:      types.BoolValue(true),
				Active:         types.BoolValue(true),
				StorageProfile: tt.want,
				DeleteProfile:  &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(60)},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got  %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestDataSourceModelFromAPI_HardDeleteProfile(t *testing.T) {
	got, err := warehouse.DataSourceModelFromAPI(serverWarehouse(&api.GCSStorageProfile{Bucket: "b"}, &api.DeleteProfile{Type: api.DeleteProfileTypeHard}))
	if err != nil {
		t.Fatalf("DataSourceModelFromAPI: %v", err)
	}

	want := &warehouse.DeleteProfileModel{Type: types.StringValue("hard")}
	if !reflect.DeepEqual(got.DeleteProfile, want) {
		t.Errorf("delete profile: got %+v, want %+v", got.DeleteProfile, want)
	}
}

func TestDataSourceModelFromAPI_Errors(t *testing.T) {
	if _, err := warehouse.DataSourceModelFromAPI(serverWarehouse(nil, nil)); err == nil ||
		!strings.Contains(err.Error(), "storage profile must be defined") {
		t.Errorf("no storage profile: got error %v", err)
	}

	got, err := warehouse.DataSourceModelFromAPI(serverWarehouse(&api.GCSStorageProfile{Bucket: "b"}, nil))
	if err != nil {
		t.Fatalf("absent delete profile: %v", err)
	}
	if got.DeleteProfile != nil {
		t.Errorf("absent delete profile: got %+v, want nil", got.DeleteProfile)
	}
}
