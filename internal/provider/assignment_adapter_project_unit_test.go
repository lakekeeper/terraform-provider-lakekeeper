package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	lakekeeper "github.com/baptistegh/go-lakekeeper/pkg/client"
)

// testHTTPRequest is one request recorded by testAdapterServer.
type testHTTPRequest struct {
	method, path string
	header       http.Header
	body         map[string]any
}

// testAdapterServer serves one canned response to every request, records the
// requests, and returns a go-lakekeeper client for it with retries disabled.
func testAdapterServer(t *testing.T, status int, body string) (*lakekeeper.Client, *[]testHTTPRequest) {
	t.Helper()
	requests := &[]testHTTPRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := testHTTPRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone()}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			if err := json.Unmarshal(raw, &req.body); err != nil {
				t.Errorf("request body is not JSON: %s", raw)
			}
		}
		*requests = append(*requests, req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client, err := lakekeeper.NewClient(context.Background(), "test", server.URL, lakekeeper.WithoutRetries())
	if err != nil {
		t.Fatal(err)
	}
	return client, requests
}

func TestProjectScopeAdapterListGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"project_id": "p1"}

	client, requests := testAdapterServer(t, 200, `{"project-id":"p1","assignments":[{"type":"select","role":"r1"},{"type":"project_admin","user":"oidc~a/b"}]}`)
	grants, err := newProjectScopeAdapter(client).ListGrants(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "select"}, {AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "project_admin"}}
	if !reflect.DeepEqual(grants, want) {
		t.Fatalf("grants %+v", grants)
	}
	if r := (*requests)[0]; r.method != "GET" || r.path != "/management/v1/permissions/project/p1/assignments" {
		t.Fatalf("request %s %s", r.method, r.path)
	}

	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden", 403, `{"error":{"code":403,"message":"forbidden","type":"Forbidden"}}`},
		{"not found", 404, `{"error":{"code":404,"message":"not found","type":"NotFound"}}`},
		{"server error", 500, `{"error":{"code":500,"message":"boom","type":"Internal"}}`},
		{"no assignments list", 200, `{"project-id":"p1"}`},
		{"grant without assignee", 200, `{"assignments":[{"type":"select"}]}`},
		{"not JSON", 200, `oops`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testAdapterServer(t, tc.status, tc.body)
			if grants, err := newProjectScopeAdapter(client).ListGrants(ctx, scope); err == nil {
				t.Fatalf("accepted %s: %+v", tc.name, grants)
			}
		})
	}
}

func TestProjectScopeAdapterApplyGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"project_id": "p1"}
	revokes := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "select"}}
	grants := []assignmentGrant{{AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "modify"}}

	client, requests := testAdapterServer(t, 204, ``)
	if err := newProjectScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 {
		t.Fatalf("want one request, got %d", len(*requests))
	}
	r := (*requests)[0]
	if r.method != "POST" || r.path != "/management/v1/permissions/project/p1/assignments" {
		t.Fatalf("request %s %s", r.method, r.path)
	}
	wantBody := map[string]any{
		"deletes": []any{map[string]any{"type": "select", "role": "r1"}},
		"writes":  []any{map[string]any{"type": "modify", "user": "oidc~a/b"}},
	}
	if !reflect.DeepEqual(r.body, wantBody) {
		t.Fatalf("body %v", r.body)
	}

	for _, status := range []int{404, 409, 500} {
		client, _ := testAdapterServer(t, status, `{"error":{"code":409,"message":"conflict","type":"Conflict"}}`)
		if err := newProjectScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err == nil {
			t.Fatalf("status %d accepted", status)
		}
	}
}
