package api

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/wire.golden")

// The canned responses below are what Lakekeeper sends, with every field the
// Management API spec marks as required.
const (
	roleJSON = `{"id":"r1","project-id":"p1","name":"role","description":"desc","created-at":"2025-01-02T03:04:05.123456Z","updated-at":"2025-02-03T04:05:06Z","ident":"role","provider-id":"lakekeeper","source-id":"r1"}`
	userJSON = `{"id":"oidc~a/b","name":"user","email":"u@example.com","user-type":"human","created-at":"2025-01-02T03:04:05.123456Z","updated-at":"2025-02-03T04:05:06Z","last-updated-with":"create-endpoint"}`

	warehouseJSON = `{"id":"w1","warehouse-id":"w1","project-id":"p1","name":"wh","protected":true,"status":"active","allowed-format-versions":[1,2],` +
		`"storage-profile":{"type":"s3","bucket":"b","region":"r","sts-enabled":true,"key-prefix":"p","allow-alternative-protocols":true,"assume-role-arn":"arn:a","aws-kms-key-arn":"arn:k","endpoint":"http://e","flavor":"s3-compat","path-style-access":true,"push-s3-delete-disabled":false,"remote-signing-url-style":"path","sts-role-arn":"arn:s","sts-token-validity-seconds":60},` +
		`"delete-profile":{"type":"soft","expiration-seconds":600}}`
)

// wireRoutes maps "METHOD path" to the canned response. Routes not listed
// answer 204 No Content.
var wireRoutes = map[string]string{
	"GET /management/v1/info":                     `{"authz-backend":"openfga","bootstrapped":true,"default-project-id":"00000000-0000-0000-0000-000000000000","aws-system-identities-enabled":true,"azure-system-identities-enabled":false,"gcp-system-identities-enabled":true,"server-id":"s1","version":"0.15.0","lakekeeper-version":"0.15.0","queues":["tabular_expiration"],"license-status":{"valid":true,"license-type":"Apache-2.0"},"managed-role-providers":[]}`,
	"POST /management/v1/project":                 `{"project-id":"p1"}`,
	"GET /management/v1/project":                  `{"project-id":"p1","project-name":"proj"}`,
	"POST /management/v1/role":                    roleJSON,
	"GET /management/v1/role/r1":                  roleJSON,
	"POST /management/v1/role/r1":                 roleJSON,
	"PUT /management/v1/role/r1":                  roleJSON,
	"POST /management/v1/user":                    userJSON,
	"GET /management/v1/user/oidc~a%2Fb":          userJSON,
	"GET /management/v1/whoami":                   strings.TrimSuffix(userJSON, "}") + `,"is-instance-admin":false}`,
	"POST /management/v1/warehouse":               warehouseJSON,
	"GET /management/v1/warehouse/w1":             warehouseJSON,
	"POST /management/v1/warehouse/w1/rename":     warehouseJSON,
	"POST /management/v1/warehouse/w1/protection": `{"protected":true}`,

	"GET /management/v1/permissions/server/assignments":       `{"assignments":[{"type":"operator","user":"u1"},{"type":"admin","role":"r1"}]}`,
	"GET /management/v1/permissions/project/assignments":      `{"project-id":"p1","assignments":[{"type":"project_admin","user":"u1"},{"type":"select","role":"r1"}]}`,
	"GET /management/v1/permissions/project/p1/assignments":   `{"project-id":"p1","assignments":[{"type":"project_admin","user":"u1"},{"type":"select","role":"r1"}]}`,
	"GET /management/v1/permissions/project/access":           `{"allowed-actions":["create_warehouse","list_roles"]}`,
	"GET /management/v1/permissions/project/p1/access":        `{"allowed-actions":["create_warehouse","list_roles"]}`,
	"GET /management/v1/permissions/warehouse/w1/assignments": `{"assignments":[{"type":"ownership","user":"u1"},{"type":"describe","role":"r1"}]}`,
	"GET /management/v1/permissions/warehouse/w1/access":      `{"allowed-actions":["create_namespace","get_config"]}`,
	"GET /management/v1/permissions/warehouse/w1":             `{"managed-access":true}`,
	"GET /management/v1/permissions/role/r1/assignments":      `{"assignments":[{"type":"ownership","user":"u1"},{"type":"assignee","role":"r2"}]}`,
	"GET /management/v1/permissions/namespace/n1/assignments": `{"assignments":[{"type":"select","role":"r1"},{"type":"describe","user":"u1"}]}`,
	"GET /management/v1/permissions/namespace/n1":             `{"managed-access":true,"managed-access-inherited":false}`,

	"GET /catalog/v1/w1/namespaces/sales\x1fbronze": `{"namespace":["sales","bronze"],"properties":{"owner":"me"}}`,
}

// TestWire pins what every Client method sends to Lakekeeper and what it
// returns for a known response. The SDK behind Client may change; this
// output may only change on purpose. Rewrite it with -update and review the
// diff.
func TestWire(t *testing.T) {
	var log strings.Builder
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(&log, "> %s %s", r.Method, r.URL.EscapedPath())
		if r.URL.RawQuery != "" {
			fmt.Fprintf(&log, "?%s", r.URL.RawQuery)
		}
		if p := r.Header.Get("x-project-id"); p != "" {
			fmt.Fprintf(&log, " [x-project-id: %s]", p)
		}
		log.WriteString("\n")
		if len(body) > 0 {
			fmt.Fprintf(&log, "  %s\n", canonicalJSON(t, body))
		}
		key := r.Method + " " + r.URL.EscapedPath()
		if r.URL.Path != r.URL.EscapedPath() && strings.HasPrefix(r.URL.Path, "/catalog/") {
			key = r.Method + " " + r.URL.Path
		}
		resp, ok := wireRoutes[key]
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer s.Close()

	ctx := context.Background()
	c, err := NewStaticTokenClient(ctx, s.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	step := func(name string, f func() (any, error)) {
		fmt.Fprintf(&log, "## %s\n", name)
		v, err := f()
		if err != nil {
			fmt.Fprintf(&log, "< error: %v\n\n", err)
			return
		}
		if v != nil {
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&log, "< %T %s\n", v, b)
		}
		log.WriteString("\n")
	}
	none := func(err error) (any, error) { return nil, err }
	writes := []Assignment{{Assignee: UserAssignee("u1"), Assignment: "select"}}
	deletes := []Assignment{{Assignee: RoleAssignee("r1"), Assignment: "describe"}}

	step("GetServerInfo", func() (any, error) { return c.GetServerInfo(ctx) })

	step("CreateProject", func() (any, error) { return c.CreateProject(ctx, "proj") })
	step("GetProject", func() (any, error) { return c.GetProject(ctx, "p1") })
	step("RenameProject", func() (any, error) { return none(c.RenameProject(ctx, "p1", "new")) })
	step("DeleteProject", func() (any, error) { return none(c.DeleteProject(ctx, "p1")) })

	step("CreateRole", func() (any, error) {
		return c.CreateRole(ctx, "p1", RoleRequest{Name: "role", Description: new("desc")})
	})
	step("CreateRole without description", func() (any, error) { return c.CreateRole(ctx, "p1", RoleRequest{Name: "role"}) })
	step("GetRole", func() (any, error) { return c.GetRole(ctx, "p1", "r1") })
	step("UpdateRole", func() (any, error) {
		return c.UpdateRole(ctx, "p1", "r1", RoleRequest{Name: "role", Description: new("desc")})
	})
	step("DeleteRole", func() (any, error) { return none(c.DeleteRole(ctx, "p1", "r1")) })

	step("ProvisionUser", func() (any, error) {
		return c.ProvisionUser(ctx, ProvisionUserRequest{ID: new("oidc~a/b"), Name: new("user"), Email: new("u@example.com"), UserType: new(UserTypeHuman), UpdateIfExists: true})
	})
	step("ProvisionUser minimal", func() (any, error) { return c.ProvisionUser(ctx, ProvisionUserRequest{}) })
	step("GetUser", func() (any, error) { return c.GetUser(ctx, "oidc~a/b") })
	step("Whoami", func() (any, error) { return c.Whoami(ctx) })
	step("DeleteUser", func() (any, error) { return none(c.DeleteUser(ctx, "oidc~a/b")) })

	for _, tc := range []struct {
		name string
		sp   StorageProfile
		sc   StorageCredential
	}{
		{"s3 access key", &S3StorageProfile{Bucket: "b", Region: "r"}, &S3AccessKeyCredential{AccessKeyID: "ak", SecretAccessKey: "sk"}},
		{"s3 access key external id", &S3StorageProfile{
			Bucket: "b", Region: "r", STSEnabled: true,
			KeyPrefix: new("p"), AllowAlternativeProtocols: new(true), AssumeRoleARN: new("arn:a"),
			AWSKMSKeyARN: new("arn:k"), Endpoint: new("http://e"), Flavor: new(S3FlavorS3Compat),
			PathStyleAccess: new(true), PushS3DeleteDisabled: new(true),
			RemoteSigningURLStyle: new(RemoteSigningURLStylePath), STSRoleARN: new("arn:s"),
			STSTokenValiditySeconds: new(int64(60)),
		}, &S3AccessKeyCredential{AccessKeyID: "ak", SecretAccessKey: "sk", ExternalID: new("ext")}},
		{"s3 push delete disabled false", &S3StorageProfile{Bucket: "b", Region: "r", PushS3DeleteDisabled: new(false)}, &S3SystemIdentityCredential{ExternalID: "ext"}},
		{"s3 system identity", &S3StorageProfile{Bucket: "b", Region: "r"}, &S3SystemIdentityCredential{}},
		{"cloudflare r2", &S3StorageProfile{Bucket: "b", Region: "r"}, &CloudflareR2Credential{AccessKeyID: "ak", SecretAccessKey: "sk", AccountID: "acc", Token: "tok"}},
		{"adls client credentials", &ADLSStorageProfile{AccountName: "a", Filesystem: "f"}, &AzureClientCredentials{ClientID: "cid", ClientSecret: "cs", TenantID: "tid"}},
		{"adls shared key", &ADLSStorageProfile{
			AccountName: "a", Filesystem: "f", Host: new("h"), AuthorityHost: new("ah"),
			KeyPrefix: new("p"), SASTokenValiditySeconds: new(int64(60)), AllowAlternativeProtocols: new(true),
		}, &AzureSharedAccessKeyCredential{Key: "k"}},
		{"adls managed identity", &ADLSStorageProfile{AccountName: "a", Filesystem: "f"}, &AzureManagedIdentityCredential{}},
		{"gcs service account key", &GCSStorageProfile{Bucket: "b", KeyPrefix: new("p")}, &GCSServiceAccountKeyCredential{Key: GCSServiceKey{
			AuthProviderX509CertURL: "a", AuthURI: "b", ClientEmail: "c", ClientID: "d", ClientX509CertURL: "e",
			PrivateKey: "f", PrivateKeyID: "g", ProjectID: "h", TokenURI: "i", Type: "service_account", UniverseDomain: "j",
		}}},
		{"gcs system identity", &GCSStorageProfile{Bucket: "b"}, &GCSSystemIdentityCredential{}},
	} {
		step("CreateWarehouse "+tc.name, func() (any, error) {
			return c.CreateWarehouse(ctx, "p1", CreateWarehouseRequest{Name: "wh", StorageProfile: tc.sp, StorageCredential: tc.sc})
		})
		step("UpdateWarehouseStorage "+tc.name, func() (any, error) {
			return none(c.UpdateWarehouseStorage(ctx, "p1", "w1", tc.sp, tc.sc))
		})
	}
	step("CreateWarehouse hard delete profile", func() (any, error) {
		return c.CreateWarehouse(ctx, "p1", CreateWarehouseRequest{Name: "wh", StorageProfile: &GCSStorageProfile{Bucket: "b"}, StorageCredential: &GCSSystemIdentityCredential{}, DeleteProfile: &DeleteProfile{Type: DeleteProfileTypeHard}})
	})
	step("CreateWarehouse soft delete profile", func() (any, error) {
		return c.CreateWarehouse(ctx, "p1", CreateWarehouseRequest{Name: "wh", StorageProfile: &GCSStorageProfile{Bucket: "b"}, StorageCredential: &GCSSystemIdentityCredential{}, DeleteProfile: &DeleteProfile{Type: DeleteProfileTypeSoft, ExpirationSeconds: 600}})
	})
	step("GetWarehouse", func() (any, error) { return c.GetWarehouse(ctx, "p1", "w1") })
	step("DeleteWarehouse", func() (any, error) { return none(c.DeleteWarehouse(ctx, "p1", "w1", false)) })
	step("DeleteWarehouse force", func() (any, error) { return none(c.DeleteWarehouse(ctx, "p1", "w1", true)) })
	step("ActivateWarehouse", func() (any, error) { return none(c.ActivateWarehouse(ctx, "p1", "w1")) })
	step("DeactivateWarehouse", func() (any, error) { return none(c.DeactivateWarehouse(ctx, "p1", "w1")) })
	step("RenameWarehouse", func() (any, error) { return none(c.RenameWarehouse(ctx, "p1", "w1", "new")) })
	step("SetWarehouseProtection", func() (any, error) { return none(c.SetWarehouseProtection(ctx, "p1", "w1", true)) })
	step("UpdateWarehouseDeleteProfile soft", func() (any, error) {
		return none(c.UpdateWarehouseDeleteProfile(ctx, "p1", "w1", DeleteProfile{Type: DeleteProfileTypeSoft, ExpirationSeconds: 600}))
	})
	step("UpdateWarehouseDeleteProfile hard", func() (any, error) {
		return none(c.UpdateWarehouseDeleteProfile(ctx, "p1", "w1", DeleteProfile{Type: DeleteProfileTypeHard}))
	})

	step("GetServerAssignments", func() (any, error) { return c.GetServerAssignments(ctx) })
	step("UpdateServerAssignments", func() (any, error) {
		return none(c.UpdateServerAssignments(ctx, []Assignment{{Assignee: UserAssignee("u1"), Assignment: ServerAssignmentAdmin}}, []Assignment{{Assignee: RoleAssignee("r1"), Assignment: ServerAssignmentOperator}}))
	})
	step("GetProjectAssignments", func() (any, error) { return c.GetProjectAssignments(ctx, "p1") })
	step("UpdateProjectAssignments", func() (any, error) { return none(c.UpdateProjectAssignments(ctx, "p1", writes, deletes)) })
	step("UpdateProjectAssignments writes only", func() (any, error) { return none(c.UpdateProjectAssignments(ctx, "p1", writes, nil)) })
	step("GetProjectAccess user", func() (any, error) { return c.GetProjectAccess(ctx, "p1", UserAssignee("u1")) })
	step("GetProjectAccess role", func() (any, error) { return c.GetProjectAccess(ctx, "p1", RoleAssignee("r1")) })
	step("GetWarehouseAssignments", func() (any, error) { return c.GetWarehouseAssignments(ctx, "w1") })
	step("UpdateWarehouseAssignments", func() (any, error) { return none(c.UpdateWarehouseAssignments(ctx, "w1", writes, deletes)) })
	step("GetWarehouseAccess", func() (any, error) { return c.GetWarehouseAccess(ctx, "w1", UserAssignee("u1")) })
	step("GetWarehouseManagedAccess", func() (any, error) { return c.GetWarehouseManagedAccess(ctx, "w1") })
	step("SetWarehouseManagedAccess", func() (any, error) { return none(c.SetWarehouseManagedAccess(ctx, "w1", true)) })
	step("GetRoleAssignments", func() (any, error) { return c.GetRoleAssignments(ctx, "r1") })
	step("UpdateRoleAssignments", func() (any, error) {
		return none(c.UpdateRoleAssignments(ctx, "r1", []Assignment{{Assignee: RoleAssignee("r2"), Assignment: RoleAssignmentAssignee}}, []Assignment{{Assignee: UserAssignee("u1"), Assignment: RoleAssignmentOwnership}}))
	})
	step("GetNamespaceAssignments", func() (any, error) { return c.GetNamespaceAssignments(ctx, "p1", "n1") })
	step("UpdateNamespaceAssignments", func() (any, error) {
		return none(c.UpdateNamespaceAssignments(ctx, "p1", "n1", writes, deletes))
	})
	step("GetNamespaceManagedAccess", func() (any, error) { return c.GetNamespaceManagedAccess(ctx, "p1", "n1") })
	step("SetNamespaceManagedAccess", func() (any, error) { return none(c.SetNamespaceManagedAccess(ctx, "p1", "n1", false)) })
	step("LoadNamespace", func() (any, error) { return c.LoadNamespace(ctx, "p1", "w1", []string{"sales", "bronze"}) })

	golden := filepath.Join("testdata", "wire.golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(log.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got := log.String(); got != string(want) {
		_ = os.WriteFile(golden+".got", []byte(got), 0o644)
		t.Fatalf("wire output differs from %s; see %s.got (rerun with -update to accept)", golden, golden)
	}
}

// canonicalJSON re-encodes a JSON body with sorted keys.
func canonicalJSON(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
