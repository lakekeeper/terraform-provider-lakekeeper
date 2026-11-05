package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const testRolePath = "/management/v1/permissions/role/r0/assignments"

// testSelfAssignmentBody is Lakekeeper's (OpenFGA authorizer) answer to a
// request that makes a role an assignee or owner of itself.
const testSelfAssignmentBody = `{"error":{"code":400,"message":"Cannot assign r0 to itself","type":"SelfAssignment","stack":[]}}`

func TestRoleScopeAdapterListGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"role_id": "r0"}

	client, requests := testAdapterServer(t, 200, `{"assignments":[{"type":"ownership","role":"r1"},{"type":"assignee","user":"oidc~a/b"}]}`)
	grants, err := newRoleScopeAdapter(client).ListGrants(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "ownership"}, {AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "assignee"}}
	if !reflect.DeepEqual(grants, want) {
		t.Fatalf("grants %+v", grants)
	}
	if len(*requests) != 1 {
		t.Fatalf("want one request, got %d", len(*requests))
	}
	if r := (*requests)[0]; r.method != "GET" || r.path != testRolePath {
		t.Fatalf("request %s %s", r.method, r.path)
	}

	client, _ = testAdapterServer(t, 200, `{"assignments":[]}`)
	if grants, err := newRoleScopeAdapter(client).ListGrants(ctx, scope); err != nil || len(grants) != 0 {
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
		{"grant without assignee", 200, `{"assignments":[{"type":"ownership"}]}`},
		{"unknown assignee type", 200, `{"assignments":[{"type":"ownership","group":"g1"}]}`},
		{"role and user", 200, `{"assignments":[{"type":"ownership","role":"r1","user":"oidc~a"}]}`},
		{"grant without type", 200, `{"assignments":[{"role":"r1"}]}`},
		{"grant with empty type", 200, `{"assignments":[{"type":"","role":"r1"}]}`},
		{"grant with empty assignee", 200, `{"assignments":[{"type":"ownership","role":""}]}`},
		{"not JSON", 200, `oops`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testAdapterServer(t, tc.status, tc.body)
			if grants, err := newRoleScopeAdapter(client).ListGrants(ctx, scope); err == nil {
				t.Fatalf("accepted %s: %+v", tc.name, grants)
			}
		})
	}
}

func TestRoleScopeAdapterApplyGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"role_id": "r0"}
	revokes := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "ownership"}}
	grants := []assignmentGrant{{AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "assignee"}}

	client, requests := testAdapterServer(t, 204, ``)
	if err := newRoleScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 {
		t.Fatalf("revokes and grants must go out in one request, got %d", len(*requests))
	}
	r := (*requests)[0]
	if r.method != "POST" || r.path != testRolePath {
		t.Fatalf("request %s %s", r.method, r.path)
	}
	wantBody := map[string]any{
		"deletes": []any{map[string]any{"type": "ownership", "role": "r1"}},
		"writes":  []any{map[string]any{"type": "assignee", "user": "oidc~a/b"}},
	}
	if !reflect.DeepEqual(r.body, wantBody) {
		t.Fatalf("body %v", r.body)
	}

	for _, status := range []int{403, 404, 409, 500} {
		client, _ := testAdapterServer(t, status, `{"error":{"code":409,"message":"conflict","type":"Conflict"}}`)
		if err := newRoleScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err == nil {
			t.Fatalf("status %d accepted", status)
		}
	}

	t.Run("self-assignment", func(t *testing.T) {
		client, _ := testAdapterServer(t, 400, testSelfAssignmentBody)
		self := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r0", Type: "assignee"}}
		err := newRoleScopeAdapter(client).ApplyGrants(ctx, scope, nil, self)
		if err == nil {
			t.Fatal("self-assignment accepted")
		}
		if !strings.Contains(err.Error(), "Cannot assign r0 to itself") {
			t.Fatalf("error does not carry Lakekeeper's message: %v", err)
		}
	})
}
