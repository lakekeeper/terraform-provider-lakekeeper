package provider

import (
	"context"
	"maps"
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

// The server role and user assignments run on the shared assignment module.
// These tests drive their real descriptors with the in-memory scope adapter;
// the HTTP shape is covered by the server adapter's own test.

const (
	testServerRole = "cb6ee351-68ff-4299-87f2-876964f6d8dd"
	testServerUser = "oidc~team/alice"
)

// testServerCase is one server assignment resource under test.
type testServerCase struct {
	desc     assignmentDescriptor
	typeName string
	attr     string
	kind     assigneeKind
	assignee string
	accepted []string
	rejected []string
}

func testServerCases() []testServerCase {
	return []testServerCase{
		{
			desc: serverRoleAssignment, typeName: "lakekeeper_server_role_assignment", attr: "role_id",
			kind: roleAssignee, assignee: testServerRole,
			accepted: []string{testServerRole},
			rejected: []string{"", "readers", testServerRole + "/x"},
		},
		{
			desc: serverUserAssignment, typeName: "lakekeeper_server_user_assignment", attr: "user_id",
			kind: userAssignee, assignee: testServerUser,
			accepted: []string{testServerUser, "kubernetes~sa", "oidc~a~b"},
			rejected: []string{"", "alice", "~alice", "oidc~", "oidc~a\nb"},
		},
	}
}

func TestServerAssignmentSchema(t *testing.T) {
	ctx := context.Background()
	for _, tc := range testServerCases() {
		t.Run(tc.typeName, func(t *testing.T) {
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())

			meta := resource.MetadataResponse{}
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "lakekeeper"}, &meta)
			if meta.TypeName != tc.typeName {
				t.Fatalf("type name %q", meta.TypeName)
			}

			s := testSchema(t, r)
			if got := s.Attributes["id"].GetMarkdownDescription(); !strings.Contains(got, "`{{"+tc.attr+"}}`") {
				t.Errorf("id description %q", got)
			}
			if !strings.Contains(s.MarkdownDescription, "update_server_assignments") {
				t.Errorf("description lacks the upstream link:\n%s", s.MarkdownDescription)
			}
			if want := []string{"assignments", "id", tc.attr}; !reflect.DeepEqual(slices.Sorted(maps.Keys(s.Attributes)), want) {
				t.Errorf("attributes %v, want %v", slices.Sorted(maps.Keys(s.Attributes)), want)
			}

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

			grantsAttr := s.Attributes["assignments"].(schema.SetAttribute)
			for _, g := range []struct {
				grants []string
				ok     bool
			}{
				{[]string{"admin", "operator"}, true},
				{[]string{}, true},
				{[]string{"project_admin"}, false},
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

func TestServerAssignmentLifecycle(t *testing.T) {
	for _, tc := range testServerCases() {
		identity := []string{tc.assignee}
		others := testOtherGrants(tc.kind, tc.assignee, "operator")
		grant := func(g string) assignmentGrant {
			return assignmentGrant{AssigneeKind: tc.kind, AssigneeID: tc.assignee, Type: g}
		}

		t.Run(tc.typeName+"/create grants on the server with an empty scope", func(t *testing.T) {
			m := newMemoryScopeAdapter(others...)
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testCreateFor(t, r, identity, "operator", "admin")
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if len(m.applies) != 1 || len(m.applies[0].scope) != 0 {
				t.Fatalf("want one write with an empty scope, got %+v", m.applies)
			}
			if got := m.grantsOf(tc.kind, tc.assignee); !reflect.DeepEqual(got, []string{"admin", "operator"}) {
				t.Fatalf("server grants %v", got)
			}
			assertOtherGrantsUntouched(t, m, others, tc.kind, tc.assignee)
			assertStateFor(t, r, resp.State, identity, "admin", "operator")
			if id := testStateValuesOf(t, r, resp.State).id; id != tc.assignee {
				t.Fatalf("id %q, want the bare assignee ID", id)
			}
		})

		t.Run(tc.typeName+"/create refuses existing grants", func(t *testing.T) {
			m := newMemoryScopeAdapter(append(others, grant("operator"))...)
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testCreateFor(t, r, identity, "admin")
			if !resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
				t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
			}
			if d := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(d, "terraform import") || !strings.Contains(d, `"`+tc.assignee+`"`) {
				t.Fatalf("detail %q", d)
			}
		})

		t.Run(tc.typeName+"/create stores the ID before writing", func(t *testing.T) {
			m := newMemoryScopeAdapter()
			m.applyErr = errTestWrite
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testCreateFor(t, r, identity, "admin")
			if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
				t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
			}
			if id := testStateValuesOf(t, r, resp.State).id; id != tc.assignee {
				t.Fatalf("id %q", id)
			}
		})

		t.Run(tc.typeName+"/update replaces grants in one request", func(t *testing.T) {
			m := newMemoryScopeAdapter(append(others, grant("operator"))...)
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testUpdateFor(t, r, testStateFor(t, r, identity, "operator"), identity, "admin")
			if resp.Diagnostics.HasError() || len(m.applies) != 1 {
				t.Fatalf("diags %v, writes %d", resp.Diagnostics, len(m.applies))
			}
			if got := m.grantsOf(tc.kind, tc.assignee); !reflect.DeepEqual(got, []string{"admin"}) {
				t.Fatalf("server grants %v", got)
			}
			assertOtherGrantsUntouched(t, m, others, tc.kind, tc.assignee)
		})

		t.Run(tc.typeName+"/read refreshes from the server", func(t *testing.T) {
			m := newMemoryScopeAdapter(append(others, grant("admin"))...)
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testRead(t, r, testStateFor(t, r, identity, "operator"))
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			assertStateFor(t, r, resp.State, identity, "admin")
		})

		for _, fail := range []bool{false, true} {
			name := "/delete removes state"
			if fail {
				name = "/failed delete keeps state"
			}
			t.Run(tc.typeName+name, func(t *testing.T) {
				m := newMemoryScopeAdapter(append(others, grant("admin"))...)
				if fail {
					m.applyErr = errTestWrite
				}
				r := newTestAssignmentResourceFor(tc.desc, m)
				resp := testDelete(t, r, testStateFor(t, r, identity, "admin"))
				if resp.Diagnostics.HasError() != fail || resp.State.Raw.IsNull() == fail {
					t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
				}
				if !fail && len(m.grantsOf(tc.kind, tc.assignee)) != 0 {
					t.Fatalf("left grants %v", m.grantsOf(tc.kind, tc.assignee))
				}
				assertOtherGrantsUntouched(t, m, others, tc.kind, tc.assignee)
			})
		}
	}
}

func TestServerAssignmentImport(t *testing.T) {
	for _, tc := range testServerCases() {
		t.Run(tc.typeName, func(t *testing.T) {
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())
			resp := testImport(t, r, tc.assignee)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			s := testStateValuesOf(t, r, resp.State)
			if want := map[string]string{tc.attr: tc.assignee}; s.id != tc.assignee || !reflect.DeepEqual(s.identity, want) {
				t.Fatalf("state %+v", s)
			}
			for _, bad := range tc.rejected {
				if resp := testImport(t, r, bad); !resp.Diagnostics.HasError() {
					t.Errorf("import %q accepted", bad)
				}
			}
		})
	}
}

func TestServerAssignmentID(t *testing.T) {
	// The ID is the bare assignee ID; a user ID with "/" stays one part.
	for _, tc := range testServerCases() {
		if got := tc.desc.idFormat(); got != "{{"+tc.attr+"}}" {
			t.Errorf("%s format %q", tc.typeName, got)
		}
		if got := buildAssignmentID([]string{tc.assignee}); got != tc.assignee {
			t.Errorf("%s build %q", tc.typeName, got)
		}
		got, err := tc.desc.parseID(tc.assignee)
		if err != nil || !reflect.DeepEqual(got, []string{tc.assignee}) {
			t.Errorf("%s parse %q: %v, %v", tc.typeName, tc.assignee, got, err)
		}
		if _, err := tc.desc.parseID(""); err == nil {
			t.Errorf("%s parsed an empty ID", tc.typeName)
		}
	}
}
