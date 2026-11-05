package provider

import (
	"context"
	"reflect"
	"testing"
)

const testNamespacePath = "/management/v1/permissions/namespace/n1/assignments"

func TestNamespaceScopeAdapterListGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"project_id": "p1", "namespace_id": "n1"}

	client, requests := testAdapterServer(t, 200, `{"assignments":[{"type":"select","role":"r1"},{"type":"ownership","user":"oidc~a/b"}]}`)
	grants, err := newNamespaceScopeAdapter(client).ListGrants(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "select"}, {AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "ownership"}}
	if !reflect.DeepEqual(grants, want) {
		t.Fatalf("grants %+v", grants)
	}
	if len(*requests) != 1 {
		t.Fatalf("want one request, got %d", len(*requests))
	}
	r := (*requests)[0]
	if r.method != "GET" || r.path != testNamespacePath {
		t.Fatalf("request %s %s", r.method, r.path)
	}
	if got := r.header.Get("x-project-id"); got != "p1" {
		t.Fatalf("project header %q", got)
	}

	client, _ = testAdapterServer(t, 200, `{"assignments":[]}`)
	if grants, err := newNamespaceScopeAdapter(client).ListGrants(ctx, scope); err != nil || len(grants) != 0 {
		t.Fatalf("empty list: grants %+v, err %v", grants, err)
	}

	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden", 403, `{"error":{"code":403,"message":"forbidden","type":"Forbidden"}}`},
		{"not found", 404, `{"error":{"code":404,"message":"not found","type":"NotFound"}}`},
		{"server error", 500, `{"error":{"code":500,"message":"boom","type":"Internal"}}`},
		{"no assignments list", 200, `{}`},
		{"null assignments list", 200, `{"assignments":null}`},
		{"grant without assignee", 200, `{"assignments":[{"type":"select"}]}`},
		{"grant with role and user", 200, `{"assignments":[{"type":"select","role":"r1","user":"oidc~a"}]}`},
		{"grant without type", 200, `{"assignments":[{"role":"r1"}]}`},
		{"grant with empty type", 200, `{"assignments":[{"type":"","role":"r1"}]}`},
		{"grant with empty assignee", 200, `{"assignments":[{"type":"select","role":""}]}`},
		{"not JSON", 200, `oops`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testAdapterServer(t, tc.status, tc.body)
			if grants, err := newNamespaceScopeAdapter(client).ListGrants(ctx, scope); err == nil {
				t.Fatalf("accepted %s: %+v", tc.name, grants)
			}
		})
	}
}

func TestNamespaceScopeAdapterApplyGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"project_id": "p1", "namespace_id": "n1"}
	revokes := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "select"}, {AssigneeKind: "role", AssigneeID: "r1", Type: "describe"}}
	grants := []assignmentGrant{{AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "modify"}}

	client, requests := testAdapterServer(t, 204, ``)
	if err := newNamespaceScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 {
		t.Fatalf("revokes and grants must go out in one request, got %d", len(*requests))
	}
	r := (*requests)[0]
	if r.method != "POST" || r.path != testNamespacePath {
		t.Fatalf("request %s %s", r.method, r.path)
	}
	if got := r.header.Get("x-project-id"); got != "p1" {
		t.Fatalf("project header %q", got)
	}
	wantBody := map[string]any{
		"deletes": []any{map[string]any{"type": "select", "role": "r1"}, map[string]any{"type": "describe", "role": "r1"}},
		"writes":  []any{map[string]any{"type": "modify", "user": "oidc~a/b"}},
	}
	if !reflect.DeepEqual(r.body, wantBody) {
		t.Fatalf("body %v", r.body)
	}

	t.Run("one side empty", func(t *testing.T) {
		// The api layer omits an empty side; Lakekeeper treats it as empty.
		client, requests := testAdapterServer(t, 204, ``)
		if err := newNamespaceScopeAdapter(client).ApplyGrants(ctx, scope, nil, grants); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"writes": []any{map[string]any{"type": "modify", "user": "oidc~a/b"}}}
		if body := (*requests)[0].body; !reflect.DeepEqual(body, want) {
			t.Fatalf("body %v", body)
		}
	})

	for _, status := range []int{403, 404, 409, 500} {
		client, _ := testAdapterServer(t, status, `{"error":{"code":409,"message":"conflict","type":"Conflict"}}`)
		if err := newNamespaceScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err == nil {
			t.Fatalf("status %d accepted", status)
		}
	}
}
