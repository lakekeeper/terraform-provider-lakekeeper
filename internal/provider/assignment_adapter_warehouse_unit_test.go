package provider

import (
	"context"
	"reflect"
	"testing"
)

const testWarehouseAdapterID = "6f1d2c3b-4a59-4e6f-8a7b-9c0d1e2f3a4b"

func TestWarehouseScopeAdapterListGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"warehouse_id": testWarehouseAdapterID}
	wantPath := "/management/v1/permissions/warehouse/" + testWarehouseAdapterID + "/assignments"

	client, requests := testAdapterServer(t, 200, `{"assignments":[{"type":"ownership","role":"r1"},{"type":"select","user":"oidc~a/b"}]}`)
	grants, err := newWarehouseScopeAdapter(client).ListGrants(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "ownership"}, {AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "select"}}
	if !reflect.DeepEqual(grants, want) {
		t.Fatalf("grants %+v", grants)
	}
	if r := (*requests)[0]; r.method != "GET" || r.path != wantPath {
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
		{"no assignments list", 200, `{}`},
		{"grant without assignee", 200, `{"assignments":[{"type":"select"}]}`},
		{"role and user", 200, `{"assignments":[{"type":"select","role":"r1","user":"oidc~a"}]}`},
		{"grant with empty type", 200, `{"assignments":[{"type":"","role":"r1"}]}`},
		{"grant with empty assignee", 200, `{"assignments":[{"type":"select","role":""}]}`},
		{"not JSON", 200, `oops`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := testAdapterServer(t, tc.status, tc.body)
			if grants, err := newWarehouseScopeAdapter(client).ListGrants(ctx, scope); err == nil {
				t.Fatalf("accepted %s: %+v", tc.name, grants)
			}
		})
	}
}

func TestWarehouseScopeAdapterApplyGrants(t *testing.T) {
	ctx := context.Background()
	scope := assignmentScope{"warehouse_id": testWarehouseAdapterID}
	revokes := []assignmentGrant{{AssigneeKind: "role", AssigneeID: "r1", Type: "ownership"}}
	grants := []assignmentGrant{{AssigneeKind: "user", AssigneeID: "oidc~a/b", Type: "modify"}}

	client, requests := testAdapterServer(t, 204, ``)
	if err := newWarehouseScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 {
		t.Fatalf("want one request, got %d", len(*requests))
	}
	r := (*requests)[0]
	if r.method != "POST" || r.path != "/management/v1/permissions/warehouse/"+testWarehouseAdapterID+"/assignments" {
		t.Fatalf("request %s %s", r.method, r.path)
	}
	wantBody := map[string]any{
		"deletes": []any{map[string]any{"type": "ownership", "role": "r1"}},
		"writes":  []any{map[string]any{"type": "modify", "user": "oidc~a/b"}},
	}
	if !reflect.DeepEqual(r.body, wantBody) {
		t.Fatalf("body %v", r.body)
	}

	for _, status := range []int{403, 404, 409, 500} {
		client, _ := testAdapterServer(t, status, `{"error":{"code":409,"message":"conflict","type":"Conflict"}}`)
		if err := newWarehouseScopeAdapter(client).ApplyGrants(ctx, scope, revokes, grants); err == nil {
			t.Fatalf("status %d accepted", status)
		}
	}
}
