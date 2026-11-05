package provider

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The warehouse role and user assignments run on the shared assignment module.
// These tests drive their real descriptors with the in-memory scope adapter;
// the HTTP shape is covered by the warehouse adapter's own test.

const (
	testWHWarehouse = "6f1d2c3b-4a59-4e6f-8a7b-9c0d1e2f3a4b"
	testWHRole      = "cb6ee351-68ff-4299-87f2-876964f6d8dd"
	testWHUser      = "oidc~team/alice"
)

// testWarehouseAssignment is one warehouse assignment resource under test.
type testWarehouseAssignment struct {
	desc     assignmentDescriptor
	typeName string
	assignee assignmentGrant // kind and ID of the managed assignee; Type unused
	// assigneeAttr, accepted and rejected values for the assignee attribute.
	assigneeAttr       string
	accepted, rejected []string
}

func testWarehouseAssignments() []testWarehouseAssignment {
	return []testWarehouseAssignment{
		{
			desc: warehouseRoleAssignment, typeName: "lakekeeper_warehouse_role_assignment",
			assignee: role(testWHRole, ""), assigneeAttr: "role_id",
			accepted: []string{testWHRole},
			rejected: []string{"", "readers", "p/" + testWHRole},
		},
		{
			desc: warehouseUserAssignment, typeName: "lakekeeper_warehouse_user_assignment",
			assignee: user(testWHUser, ""), assigneeAttr: "user_id",
			accepted: []string{testWHUser, "oidc~alice", "kubernetes~a~b"},
			rejected: []string{"", "alice", "~alice", "oidc~"},
		},
	}
}

// testWHOtherGrants are held by other assignees on the warehouse, including a
// role and a user whose ID equals the managed assignee's of the other kind.
func testWHOtherGrants() []assignmentGrant {
	return []assignmentGrant{
		role("4a8f0a43-6c9e-4b8e-9f0b-5d0b8c1e2f33", "select"),
		user("oidc~bob", "select"),
		role(testWHUser, "describe"),
		user(testWHRole, "describe"),
	}
}

func (tc testWarehouseAssignment) identity() []string {
	return []string{testWHWarehouse, tc.assignee.AssigneeID}
}

func (tc testWarehouseAssignment) grant(t string) assignmentGrant {
	g := tc.assignee
	g.Type = t
	return g
}

func assertWHOtherGrantsUntouched(t *testing.T, tc testWarehouseAssignment, m *memoryScopeAdapter) {
	t.Helper()
	for _, g := range testWHOtherGrants() {
		if !slices.Contains(m.grants, g) {
			t.Fatalf("grant of another assignee was removed: %+v", g)
		}
	}
	for _, a := range m.applies {
		for _, g := range append(slices.Clone(a.revokes), a.grants...) {
			if g.AssigneeKind != tc.assignee.AssigneeKind || g.AssigneeID != tc.assignee.AssigneeID {
				t.Fatalf("request changed another assignee: %+v", g)
			}
		}
	}
}

func TestWarehouseAssignmentSchema(t *testing.T) {
	ctx := context.Background()
	for _, tc := range testWarehouseAssignments() {
		t.Run(tc.typeName, func(t *testing.T) {
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())

			meta := resource.MetadataResponse{}
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "lakekeeper"}, &meta)
			if meta.TypeName != tc.typeName {
				t.Fatalf("type name %q", meta.TypeName)
			}

			s := testSchema(t, r)
			if got := s.Attributes["id"].GetMarkdownDescription(); !strings.Contains(got, "{{warehouse_id}}/{{"+tc.assigneeAttr+"}}") {
				t.Errorf("id description %q", got)
			}
			if !strings.Contains(s.MarkdownDescription, "update_warehouse_assignments") {
				t.Errorf("description lacks the upstream API link:\n%s", s.MarkdownDescription)
			}

			for _, attrCase := range []struct {
				attr               string
				accepted, rejected []string
			}{
				{"warehouse_id", []string{testWHWarehouse}, []string{"", "my-warehouse", "p/" + testWHWarehouse}},
				{tc.assigneeAttr, tc.accepted, tc.rejected},
			} {
				attr := s.Attributes[attrCase.attr].(schema.StringAttribute)
				if !attr.Required {
					t.Errorf("%s must be required", attrCase.attr)
				}
				for _, v := range append(slices.Clone(attrCase.accepted), attrCase.rejected...) {
					resp := validator.StringResponse{}
					for _, val := range attr.Validators {
						val.ValidateString(ctx, validator.StringRequest{Path: path.Root(attrCase.attr), ConfigValue: types.StringValue(v)}, &resp)
					}
					if want := slices.Contains(attrCase.rejected, v); resp.Diagnostics.HasError() != want {
						t.Errorf("%s %q: rejected=%v, want %v", attrCase.attr, v, resp.Diagnostics.HasError(), want)
					}
				}
			}

			grantsAttr := s.Attributes["assignments"].(schema.SetAttribute)
			for _, g := range []struct {
				grants []string
				ok     bool
			}{
				{[]string{"ownership", "pass_grants", "manage_grants", "describe", "select", "create", "modify"}, true},
				{[]string{}, true},
				{[]string{"select", "project_admin"}, false},
			} {
				set, _ := types.SetValueFrom(ctx, types.StringType, g.grants)
				resp := validator.SetResponse{}
				for _, v := range grantsAttr.Validators {
					v.ValidateSet(ctx, validator.SetRequest{Path: path.Root("assignments"), ConfigValue: set}, &resp)
				}
				if resp.Diagnostics.HasError() == g.ok {
					t.Errorf("grants %v: want ok=%v, got %v", g.grants, g.ok, resp.Diagnostics)
				}
			}
		})
	}
}

func TestWarehouseAssignmentLifecycle(t *testing.T) {
	for _, tc := range testWarehouseAssignments() {
		t.Run(tc.typeName, func(t *testing.T) {
			t.Run("create grants on the warehouse scope", func(t *testing.T) {
				m := newMemoryScopeAdapter(testWHOtherGrants()...)
				r := newTestAssignmentResourceFor(tc.desc, m)
				resp := testCreateFor(t, r, tc.identity(), "ownership", "select")
				if resp.Diagnostics.HasError() {
					t.Fatal(resp.Diagnostics)
				}
				if len(m.applies) != 1 {
					t.Fatalf("want one write request, got %d", len(m.applies))
				}
				if want := (assignmentScope{"warehouse_id": testWHWarehouse}); !reflect.DeepEqual(m.applies[0].scope, want) {
					t.Fatalf("scope %v, want %v", m.applies[0].scope, want)
				}
				if got := m.grantsOf(tc.assignee.AssigneeKind, tc.assignee.AssigneeID); !reflect.DeepEqual(got, []string{"ownership", "select"}) {
					t.Fatalf("server grants %v", got)
				}
				assertWHOtherGrantsUntouched(t, tc, m)
				assertStateFor(t, r, resp.State, tc.identity(), "ownership", "select")
			})

			t.Run("create refuses existing grants", func(t *testing.T) {
				m := newMemoryScopeAdapter(append(testWHOtherGrants(), tc.grant("describe"))...)
				r := newTestAssignmentResourceFor(tc.desc, m)
				resp := testCreateFor(t, r, tc.identity(), "select")
				if !resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
					t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
				}
			})

			t.Run("update reconciles in one request", func(t *testing.T) {
				m := newMemoryScopeAdapter(append(testWHOtherGrants(), tc.grant("ownership"))...)
				r := newTestAssignmentResourceFor(tc.desc, m)
				resp := testUpdateFor(t, r, testStateFor(t, r, tc.identity(), "ownership"), tc.identity(), "select", "modify")
				if resp.Diagnostics.HasError() || len(m.applies) != 1 {
					t.Fatalf("diags %v, writes %d", resp.Diagnostics, len(m.applies))
				}
				if got := m.grantsOf(tc.assignee.AssigneeKind, tc.assignee.AssigneeID); !reflect.DeepEqual(got, []string{"modify", "select"}) {
					t.Fatalf("server grants %v", got)
				}
				assertWHOtherGrantsUntouched(t, tc, m)
				assertStateFor(t, r, resp.State, tc.identity(), "modify", "select")
			})

			for _, fail := range []bool{false, true} {
				name := "delete removes state"
				if fail {
					name = "failed delete keeps state"
				}
				t.Run(name, func(t *testing.T) {
					m := newMemoryScopeAdapter(append(testWHOtherGrants(), tc.grant("select"))...)
					if fail {
						m.applyErr = errTestWrite
					}
					r := newTestAssignmentResourceFor(tc.desc, m)
					resp := testDelete(t, r, testStateFor(t, r, tc.identity(), "select"))
					if resp.Diagnostics.HasError() != fail || resp.State.Raw.IsNull() == fail {
						t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
					}
					if !fail && len(m.grantsOf(tc.assignee.AssigneeKind, tc.assignee.AssigneeID)) != 0 {
						t.Fatal("grants left after delete")
					}
					assertWHOtherGrantsUntouched(t, tc, m)
				})
			}
		})
	}
}

func TestWarehouseAssignmentImport(t *testing.T) {
	for _, tc := range testWarehouseAssignments() {
		t.Run(tc.typeName, func(t *testing.T) {
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())
			id := testWHWarehouse + "/" + tc.assignee.AssigneeID
			resp := testImport(t, r, id)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			s := testStateValuesOf(t, r, resp.State)
			want := map[string]string{"warehouse_id": testWHWarehouse, tc.assigneeAttr: tc.assignee.AssigneeID}
			if s.id != id || !reflect.DeepEqual(s.identity, want) {
				t.Fatalf("state %+v", s)
			}
			for _, bad := range []string{"", testWHWarehouse, testWHWarehouse + "/", "/" + tc.assignee.AssigneeID} {
				if resp := testImport(t, r, bad); !resp.Diagnostics.HasError() {
					t.Errorf("import %q accepted", bad)
				}
			}
		})
	}
}
