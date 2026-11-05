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

// The namespace role assignment runs on the shared assignment module. These
// tests drive its real descriptor with the in-memory scope adapter; the HTTP
// shape is covered by the namespace adapter's own test.

const (
	testNSProject   = "p"
	testNSNamespace = "0e5ad4b6-07a1-4e3c-a5b0-6d0c5e1d5a11"
	testNSRole      = "cb6ee351-68ff-4299-87f2-876964f6d8dd"
)

var testNSIdentity = []string{testNSProject, testNSNamespace, testNSRole}

// testNSOtherGrants are held by other assignees on the namespace and must never change.
func testNSOtherGrants() []assignmentGrant {
	return testOtherGrants(roleAssignee, testNSRole, "select")
}

func newTestNamespaceRoleAssignment(m scopeAdapter) *assignmentResource {
	return newTestAssignmentResourceFor(namespaceRoleAssignment, m)
}

func TestNamespaceRoleAssignmentSchema(t *testing.T) {
	ctx := context.Background()
	r := newTestNamespaceRoleAssignment(newMemoryScopeAdapter())

	meta := resource.MetadataResponse{}
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "lakekeeper"}, &meta)
	if meta.TypeName != "lakekeeper_namespace_role_assignment" {
		t.Fatalf("type name %q", meta.TypeName)
	}

	s := testSchema(t, r)
	for _, want := range []string{"OpenFGA", "descendants", "allow-all"} {
		if !strings.Contains(s.MarkdownDescription, want) {
			t.Errorf("description lacks %q:\n%s", want, s.MarkdownDescription)
		}
	}
	if strings.Contains(s.MarkdownDescription, "partial progress") {
		t.Errorf("description still mentions partial progress:\n%s", s.MarkdownDescription)
	}
	if got := s.Attributes["id"].GetMarkdownDescription(); !strings.Contains(got, "{{project_id}}/{{namespace_id}}/{{role_id}}") {
		t.Errorf("id description %q", got)
	}

	for _, tc := range []struct {
		attr     string
		accepted []string
		rejected []string
	}{
		{"project_id", []string{testNSProject, "00000000-0000-0000-0000-000000000000"}, []string{"", "a/b", "a b"}},
		{"namespace_id", []string{testNSNamespace}, []string{"", "my.namespace", testNSProject + "/" + testNSNamespace}},
		{"role_id", []string{testNSRole}, []string{"", "readers", testNSRole + "/x"}},
	} {
		attr := s.Attributes[tc.attr].(schema.StringAttribute)
		if !attr.Required {
			t.Errorf("%s must be required", tc.attr)
		}
		for _, v := range append(slices.Clone(tc.accepted), tc.rejected...) {
			resp := validator.StringResponse{}
			for _, val := range attr.Validators {
				val.ValidateString(ctx, validator.StringRequest{Path: path.Root(tc.attr), ConfigValue: types.StringValue(v)}, &resp)
			}
			if want := slices.Contains(tc.rejected, v); resp.Diagnostics.HasError() != want {
				t.Errorf("%s %q: rejected=%v, want %v", tc.attr, v, resp.Diagnostics.HasError(), want)
			}
		}
	}

	grantsAttr := s.Attributes["assignments"].(schema.SetAttribute)
	for _, tc := range []struct {
		grants []string
		ok     bool
	}{
		{[]string{"ownership", "pass_grants", "manage_grants", "describe", "select", "create", "modify"}, true},
		{[]string{}, true},
		{[]string{"select", "project_admin"}, false},
		{[]string{"drop"}, false},
	} {
		set, _ := types.SetValueFrom(ctx, types.StringType, tc.grants)
		resp := validator.SetResponse{}
		for _, v := range grantsAttr.Validators {
			v.ValidateSet(ctx, validator.SetRequest{Path: path.Root("assignments"), ConfigValue: set}, &resp)
		}
		if resp.Diagnostics.HasError() == tc.ok {
			t.Errorf("grants %v: want ok=%v, got %v", tc.grants, tc.ok, resp.Diagnostics)
		}
	}
}

func TestNamespaceRoleAssignmentCreate(t *testing.T) {
	t.Run("grants on the namespace scope and keeps other principals", func(t *testing.T) {
		m := newMemoryScopeAdapter(testNSOtherGrants()...)
		r := newTestNamespaceRoleAssignment(m)
		resp := testCreateFor(t, r, testNSIdentity, "select", "describe")
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if len(m.applies) != 1 {
			t.Fatalf("want one write request, got %d", len(m.applies))
		}
		if want := (assignmentScope{"project_id": testNSProject, "namespace_id": testNSNamespace}); !reflect.DeepEqual(m.applies[0].scope, want) {
			t.Fatalf("scope %v, want %v", m.applies[0].scope, want)
		}
		if got := m.grantsOf(roleAssignee, testNSRole); !reflect.DeepEqual(got, []string{"describe", "select"}) {
			t.Fatalf("server grants %v", got)
		}
		assertOtherGrantsUntouched(t, m, testNSOtherGrants(), roleAssignee, testNSRole)
		assertStateFor(t, r, resp.State, testNSIdentity, "describe", "select")
	})

	t.Run("refuses existing grants", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(testNSOtherGrants(), role(testNSRole, "ownership"))...)
		r := newTestNamespaceRoleAssignment(m)
		resp := testCreateFor(t, r, testNSIdentity, "select")
		if !resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
			t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
		}
	})

	t.Run("stores the three-part ID before writing", func(t *testing.T) {
		m := newMemoryScopeAdapter()
		m.applyErr = errTestWrite
		r := newTestNamespaceRoleAssignment(m)
		resp := testCreateFor(t, r, testNSIdentity, "select")
		if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
			t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
		}
		if id := testStateValuesOf(t, r, resp.State).id; id != testNSProject+"/"+testNSNamespace+"/"+testNSRole {
			t.Fatalf("id %q", id)
		}
	})
}

func TestNamespaceRoleAssignmentReconcile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		desired   []string
		failWrite bool
	}{
		{"replace", []string{"select"}, false},
		{"revoke", nil, false},
		{"failed write", []string{"select"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMemoryScopeAdapter(append(testNSOtherGrants(), role(testNSRole, "ownership"))...)
			if tc.failWrite {
				m.applyErr = errTestWrite
			}
			r := newTestNamespaceRoleAssignment(m)
			state := testStateFor(t, r, testNSIdentity, "ownership")

			resp := testUpdateFor(t, r, state, testNSIdentity, tc.desired...)
			if resp.Diagnostics.HasError() != tc.failWrite {
				t.Fatalf("diags %v", resp.Diagnostics)
			}
			// Revokes and grants go out together, so even a failed write is all or nothing.
			if len(m.applies) != 1 {
				t.Fatalf("want one write request, got %d", len(m.applies))
			}
			want := tc.desired
			if tc.failWrite {
				want = []string{"ownership"}
			}
			if got := m.grantsOf(roleAssignee, testNSRole); !slices.Equal(got, append([]string{}, want...)) {
				t.Fatalf("server grants %v, want %v", got, want)
			}
			assertOtherGrantsUntouched(t, m, testNSOtherGrants(), roleAssignee, testNSRole)
			if tc.failWrite {
				return
			}
			assertStateFor(t, r, resp.State, testNSIdentity, tc.desired...)

			m.applies = nil
			again := testUpdateFor(t, r, resp.State, testNSIdentity, tc.desired...)
			if again.Diagnostics.HasError() || len(m.applies) != 0 {
				t.Fatalf("converged apply: diags %v, writes %d", again.Diagnostics, len(m.applies))
			}
		})
	}
}

func TestNamespaceRoleAssignmentReadFailure(t *testing.T) {
	// 403, 404 and malformed responses all surface as adapter errors; see the
	// namespace adapter test for how each is detected.
	for name, op := range map[string]func(*assignmentResource) bool{
		"read": func(r *assignmentResource) bool {
			return testRead(t, r, testStateFor(t, r, testNSIdentity, "select")).Diagnostics.HasError()
		},
		"update": func(r *assignmentResource) bool {
			return testUpdateFor(t, r, testStateFor(t, r, testNSIdentity, "select"), testNSIdentity, "modify").Diagnostics.HasError()
		},
		"delete": func(r *assignmentResource) bool {
			resp := testDelete(t, r, testStateFor(t, r, testNSIdentity, "select"))
			return resp.Diagnostics.HasError() && !resp.State.Raw.IsNull()
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := newMemoryScopeAdapter(role(testNSRole, "select"))
			m.listErr = errTestRead
			if !op(newTestNamespaceRoleAssignment(m)) {
				t.Fatal("failed read accepted")
			}
			if len(m.applies) != 0 {
				t.Fatal("wrote after failed read")
			}
		})
	}
}

func TestNamespaceRoleAssignmentDelete(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success removes state"
		if fail {
			name = "failure keeps state"
		}
		t.Run(name, func(t *testing.T) {
			m := newMemoryScopeAdapter(append(testNSOtherGrants(), role(testNSRole, "select"))...)
			if fail {
				m.applyErr = errTestWrite
			}
			r := newTestNamespaceRoleAssignment(m)
			resp := testDelete(t, r, testStateFor(t, r, testNSIdentity, "select"))
			if resp.Diagnostics.HasError() != fail {
				t.Fatal(resp.Diagnostics)
			}
			if resp.State.Raw.IsNull() == fail {
				t.Fatal("failed deletion must preserve state; successful deletion must remove it")
			}
			if !fail && len(m.grantsOf(roleAssignee, testNSRole)) != 0 {
				t.Fatalf("left grants %v", m.grantsOf(roleAssignee, testNSRole))
			}
			assertOtherGrantsUntouched(t, m, testNSOtherGrants(), roleAssignee, testNSRole)
		})
	}
}

func TestNamespaceRoleAssignmentImport(t *testing.T) {
	r := newTestNamespaceRoleAssignment(newMemoryScopeAdapter())
	id := testNSProject + "/" + testNSNamespace + "/" + testNSRole
	resp := testImport(t, r, id)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	s := testStateValuesOf(t, r, resp.State)
	want := map[string]string{"project_id": testNSProject, "namespace_id": testNSNamespace, "role_id": testNSRole}
	if s.id != id || !reflect.DeepEqual(s.identity, want) {
		t.Fatalf("state %+v", s)
	}
	for _, bad := range []string{testNSNamespace + "/" + testNSRole, testNSProject + "//" + testNSRole, "", testNSProject + "/not-a-uuid/" + testNSRole, "p.x/" + testNSNamespace + "/" + testNSRole} {
		if resp := testImport(t, r, bad); !resp.Diagnostics.HasError() {
			t.Errorf("import %q accepted", bad)
		}
	}
}
