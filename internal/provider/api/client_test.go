package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// wire marshals an SDK value and decodes it into a generic map, so tests can
// assert on the JSON sent to Lakekeeper independently of the SDK's types.
func wire(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStorageProfileToSDK_S3UnsetFieldsUseDefaults(t *testing.T) {
	sp, err := storageProfileToSDK(&S3StorageProfile{Bucket: "b", Region: "r"})
	if err != nil {
		t.Fatal(err)
	}
	got := wire(t, sp)
	want := map[string]any{
		"type":                       "s3",
		"bucket":                     "b",
		"region":                     "r",
		"sts-enabled":                false,
		"flavor":                     "aws",
		"sts-token-validity-seconds": float64(3600),
		"push-s3-delete-disabled":    true,
		"remote-signing-url-style":   "auto",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestStorageProfileToSDK_S3ZeroValuesAreNotSent(t *testing.T) {
	// Empty strings, false and 0 count as "not set", as they did before the
	// client layer existed.
	sp, err := storageProfileToSDK(&S3StorageProfile{
		Bucket: "b", Region: "r",
		KeyPrefix: new(""), Endpoint: new(""), PathStyleAccess: new(false), STSTokenValiditySeconds: new(int64(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := wire(t, sp)
	for _, k := range []string{"key-prefix", "endpoint", "path-style-access"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s sent: %v", k, got)
		}
	}
	if got["sts-token-validity-seconds"] != float64(3600) {
		t.Errorf("sts-token-validity-seconds = %v, want default 3600", got["sts-token-validity-seconds"])
	}
}

func TestStorageProfileToSDK_ADLSOmitsUnsetOptionalFields(t *testing.T) {
	sp, err := storageProfileToSDK(&ADLSStorageProfile{AccountName: "a", Filesystem: "f"})
	if err != nil {
		t.Fatal(err)
	}
	got := wire(t, sp)
	for _, k := range []string{"host", "authority-host", "sas-token-validity-seconds"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s must be omitted when unset, got %v", k, got)
		}
	}

	sp, err = storageProfileToSDK(&ADLSStorageProfile{
		AccountName: "a", Filesystem: "f",
		Host: new("h"), AuthorityHost: new("ah"), SASTokenValiditySeconds: new(int64(60)),
	})
	if err != nil {
		t.Fatal(err)
	}
	got = wire(t, sp)
	if got["host"] != "h" || got["authority-host"] != "ah" || got["sas-token-validity-seconds"] != float64(60) {
		t.Errorf("explicit values not sent: %v", got)
	}
}

func TestStorageProfileRoundTrip(t *testing.T) {
	for _, in := range []StorageProfile{
		&S3StorageProfile{
			Bucket: "b", Region: "r", STSEnabled: true,
			KeyPrefix: new("p"), AllowAlternativeProtocols: new(true), AssumeRoleARN: new("arn:a"),
			AWSKMSKeyARN: new("arn:k"), Endpoint: new("http://e"), Flavor: new(S3FlavorS3Compat),
			PathStyleAccess: new(true), PushS3DeleteDisabled: new(true),
			RemoteSigningURLStyle: new(RemoteSigningURLStylePath), STSRoleARN: new("arn:s"),
			STSTokenValiditySeconds: new(int64(60)),
		},
		&ADLSStorageProfile{
			AccountName: "a", Filesystem: "f", Host: new("h"), AuthorityHost: new("ah"),
			KeyPrefix: new("p"), SASTokenValiditySeconds: new(int64(60)), AllowAlternativeProtocols: new(true),
		},
		&GCSStorageProfile{Bucket: "b", KeyPrefix: new("p")},
	} {
		sp, err := storageProfileToSDK(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := storageProfileFromSDK(sp)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("round trip changed %T:\n in %+v\nout %+v", in, in, out)
		}
	}
}

func TestDeleteProfileRoundTrip(t *testing.T) {
	for _, in := range []DeleteProfile{
		{Type: DeleteProfileTypeHard},
		{Type: DeleteProfileTypeSoft, ExpirationSeconds: 60},
	} {
		dp, err := deleteProfileToSDK(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := deleteProfileFromSDK(dp)
		if err != nil {
			t.Fatal(err)
		}
		if *out != in {
			t.Errorf("got %+v, want %+v", *out, in)
		}
	}
	if _, err := deleteProfileToSDK(DeleteProfile{Type: "bogus"}); err == nil {
		t.Error("unknown delete profile type accepted")
	}
}

func TestErrorsCarryStatusCode(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"no authz backend","type":"NotFound"}}`))
	}))
	defer s.Close()
	ctx := context.Background()
	c, err := NewStaticTokenClient(ctx, s.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Typed SDK method.
	err = c.SetWarehouseManagedAccess(ctx, "w", true)
	if !IsNotFound(err) {
		t.Fatalf("SetWarehouseManagedAccess: IsNotFound(%v) = false", err)
	}
	if want := "api error, code=404 message=no authz backend type=NotFound"; err.Error() != want {
		t.Errorf("message changed: %q, want %q", err.Error(), want)
	}

	// Raw request.
	if _, err := c.GetNamespaceManagedAccess(ctx, "p", "n"); !IsNotFound(err) {
		t.Fatalf("GetNamespaceManagedAccess: IsNotFound(%v) = false", err)
	}

	if IsNotFound(nil) || StatusCode(context.Canceled) != 0 {
		t.Error("non-API errors must not report a status code")
	}
}

func TestNamespaceAssignmentsWireFormat(t *testing.T) {
	var body map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("x-project-id") != "p" {
			t.Errorf("missing project header")
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"assignments":[{"type":"select","role":"r1"},{"type":"describe","user":"u1"}]}`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()
	ctx := context.Background()
	c, err := NewStaticTokenClient(ctx, s.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.GetNamespaceAssignments(ctx, "p", "n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Assignment{
		{Assignee: RoleAssignee("r1"), Assignment: "select"},
		{Assignee: UserAssignee("u1"), Assignment: "describe"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if err := c.UpdateNamespaceAssignments(ctx, "p", "n", nil, want[:1]); err != nil {
		t.Fatal(err)
	}
	wantBody := map[string]any{"deletes": []any{map[string]any{"type": "select", "role": "r1"}}}
	if !reflect.DeepEqual(body, wantBody) {
		t.Fatalf("body %v, want %v", body, wantBody)
	}
}

// cannedServer answers every request with status and body, and returns a
// client for it.
func cannedServer(t *testing.T, status int, body string) *Client {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	c, err := NewStaticTokenClient(t.Context(), s.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGetAssignmentsRejectsMalformedResponses(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"no assignments list", `{"project-id":"p"}`},
		{"null assignments list", `{"assignments":null}`},
		{"neither role nor user", `{"assignments":[{"type":"select"}]}`},
		{"role and user", `{"assignments":[{"type":"select","role":"r1","user":"oidc~a"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cannedServer(t, http.StatusOK, tc.body)
			if got, err := c.GetProjectAssignments(t.Context(), "p"); err == nil {
				t.Errorf("GetProjectAssignments accepted it: %+v", got)
			}
			if got, err := c.GetNamespaceAssignments(t.Context(), "p", "n"); err == nil {
				t.Errorf("GetNamespaceAssignments accepted it: %+v", got)
			}
		})
	}

	c := cannedServer(t, http.StatusOK, `{"assignments":[]}`)
	if got, err := c.GetProjectAssignments(t.Context(), "p"); err != nil || got == nil || len(got) != 0 {
		t.Errorf("GetProjectAssignments with an empty list: %+v, %v", got, err)
	}
	if got, err := c.GetNamespaceAssignments(t.Context(), "p", "n"); err != nil || got == nil || len(got) != 0 {
		t.Errorf("GetNamespaceAssignments with an empty list: %+v, %v", got, err)
	}
}

func TestUserIDIsPathEscaped(t *testing.T) {
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"oidc~a/b","name":"n","user-type":"human","created-at":"2025-01-01T00:00:00Z","last-updated-with":"create-endpoint"}`))
	}))
	defer s.Close()
	ctx := t.Context()
	c, err := NewStaticTokenClient(ctx, s.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	u, err := c.GetUser(ctx, "oidc~a/b")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "oidc~a/b" {
		t.Errorf("user ID %q", u.ID)
	}
	if err := c.DeleteUser(ctx, "oidc~a/b"); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /management/v1/user/oidc~a%2Fb", "DELETE /management/v1/user/oidc~a%2Fb"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("paths %q, want %q", paths, want)
	}
}
