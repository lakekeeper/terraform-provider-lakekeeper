package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// The role role and role user assignments run on the shared assignment module.
// These tests drive their real descriptors with the in-memory scope adapter;
// the HTTP shape is covered by the role adapter's own test.

const (
	testRoleScope    = "cb6ee351-68ff-4299-87f2-876964f6d8dd"
	testRoleAssignee = "4a8f0a43-6c9e-4b8e-9f0b-5d0b8c1e2f33"
	testRoleUser     = "oidc~team/alice"
)

func TestRoleAssignmentSchemas(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		desc     assignmentDescriptor
		typeName string
		idFormat string
		attrs    map[string][2][]string // attribute -> accepted, rejected
	}{
		{roleRoleAssignment, "lakekeeper_role_role_assignment", "{{role_id}}/{{assignee_id}}", map[string][2][]string{
			"role_id":     {{testRoleScope}, {"", "readers", "p/" + testRoleScope}},
			"assignee_id": {{testRoleAssignee}, {"", "readers", "p/" + testRoleAssignee}},
		}},
		{roleUserAssignment, "lakekeeper_role_user_assignment", "{{role_id}}/{{user_id}}", map[string][2][]string{
			"role_id": {{testRoleScope}, {"", "readers", "p/" + testRoleScope}},
			"user_id": {{testRoleUser, "kubernetes~sa"}, {"", "alice", "oidc~", "~alice"}},
		}},
	} {
		t.Run(tc.typeName, func(t *testing.T) {
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())
			meta := resource.MetadataResponse{}
			r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "lakekeeper"}, &meta)
			if meta.TypeName != tc.typeName {
				t.Fatalf("type name %q", meta.TypeName)
			}

			s := testSchema(t, r)
			if got := s.Attributes["id"].GetMarkdownDescription(); !strings.Contains(got, tc.idFormat) {
				t.Errorf("id description %q lacks %q", got, tc.idFormat)
			}
			for name, values := range tc.attrs {
				attr := s.Attributes[name].(schema.StringAttribute)
				if !attr.Required {
					t.Errorf("%s must be required", name)
				}
				accepted, rejected := values[0], values[1]
				for _, v := range append(slices.Clone(accepted), rejected...) {
					resp := validator.StringResponse{}
					for _, val := range attr.Validators {
						val.ValidateString(ctx, validator.StringRequest{Path: path.Root(name), ConfigValue: types.StringValue(v)}, &resp)
					}
					if want := slices.Contains(rejected, v); resp.Diagnostics.HasError() != want {
						t.Errorf("%s %q: rejected=%v, want %v", name, v, resp.Diagnostics.HasError(), want)
					}
				}
			}

			grantsAttr := s.Attributes["assignments"].(schema.SetAttribute)
			for _, g := range []struct {
				grants []string
				ok     bool
			}{
				{[]string{"ownership", "assignee"}, true},
				{[]string{}, true},
				{[]string{"select"}, false},
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

func TestRoleAssignmentLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name     string
		desc     assignmentDescriptor
		kind     assigneeKind
		identity []string
	}{
		{"role", roleRoleAssignment, roleAssignee, []string{testRoleScope, testRoleAssignee}},
		{"user", roleUserAssignment, userAssignee, []string{testRoleScope, testRoleUser}},
	} {
		assignee := tc.identity[1]
		// Grants of other assignees on the role, including one of the other
		// kind with the same ID, must never change.
		others := []assignmentGrant{
			role("11111111-2222-3333-4444-555555555555", "assignee"),
			{AssigneeKind: "user", AssigneeID: "oidc~bob", Type: "ownership"},
		}
		if tc.kind == roleAssignee {
			others = append(others, assignmentGrant{AssigneeKind: userAssignee, AssigneeID: assignee, Type: "assignee"})
		} else {
			others = append(others, assignmentGrant{AssigneeKind: roleAssignee, AssigneeID: assignee, Type: "assignee"})
		}
		assertOthers := func(t *testing.T, m *memoryScopeAdapter) {
			t.Helper()
			for _, g := range others {
				if !slices.Contains(m.grants, g) {
					t.Fatalf("grant of another assignee was removed: %+v", g)
				}
			}
		}

		t.Run(tc.name+"/create, update, delete", func(t *testing.T) {
			m := newMemoryScopeAdapter(others...)
			r := newTestAssignmentResourceFor(tc.desc, m)

			created := testCreateFor(t, r, tc.identity, "ownership")
			if created.Diagnostics.HasError() {
				t.Fatal(created.Diagnostics)
			}
			if want := (assignmentScope{"role_id": testRoleScope}); !reflect.DeepEqual(m.applies[0].scope, want) {
				t.Fatalf("scope %v, want %v", m.applies[0].scope, want)
			}
			assertStateFor(t, r, created.State, tc.identity, "ownership")

			updated := testUpdateFor(t, r, created.State, tc.identity, "assignee")
			if updated.Diagnostics.HasError() {
				t.Fatal(updated.Diagnostics)
			}
			if len(m.applies) != 2 {
				t.Fatalf("revokes and grants must go out in one request, got %d requests", len(m.applies))
			}
			if got := m.grantsOf(tc.kind, assignee); !reflect.DeepEqual(got, []string{"assignee"}) {
				t.Fatalf("server grants %v", got)
			}

			deleted := testDelete(t, r, updated.State)
			if deleted.Diagnostics.HasError() || !deleted.State.Raw.IsNull() {
				t.Fatalf("diags %v, state %v", deleted.Diagnostics, deleted.State.Raw)
			}
			if got := m.grantsOf(tc.kind, assignee); len(got) != 0 {
				t.Fatalf("left grants %v", got)
			}
			assertOthers(t, m)
		})

		t.Run(tc.name+"/create refuses existing grants", func(t *testing.T) {
			m := newMemoryScopeAdapter(append(slices.Clone(others), assignmentGrant{AssigneeKind: tc.kind, AssigneeID: assignee, Type: "assignee"})...)
			resp := testCreateFor(t, newTestAssignmentResourceFor(tc.desc, m), tc.identity, "ownership")
			if !resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
				t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
			}
		})

		t.Run(tc.name+"/failed delete keeps state", func(t *testing.T) {
			m := newMemoryScopeAdapter(append(slices.Clone(others), assignmentGrant{AssigneeKind: tc.kind, AssigneeID: assignee, Type: "ownership"})...)
			m.applyErr = errTestWrite
			r := newTestAssignmentResourceFor(tc.desc, m)
			resp := testDelete(t, r, testStateFor(t, r, tc.identity, "ownership"))
			if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
				t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
			}
			assertOthers(t, m)
		})
	}
}

func TestRoleAssignmentImport(t *testing.T) {
	for _, tc := range []struct {
		desc     assignmentDescriptor
		assignee string
		id       string
		want     map[string]string
		bad      []string
	}{
		{
			roleRoleAssignment, "assignee_id",
			testRoleScope + "/" + testRoleAssignee,
			map[string]string{"role_id": testRoleScope, "assignee_id": testRoleAssignee},
			[]string{testRoleScope, testRoleScope + "/", "/" + testRoleAssignee, "", "readers/" + testRoleAssignee, testRoleScope + "/x/" + testRoleAssignee},
		},
		{
			// The user ID is the last part and may contain "/".
			roleUserAssignment, "user_id",
			testRoleScope + "/" + testRoleUser,
			map[string]string{"role_id": testRoleScope, "user_id": testRoleUser},
			[]string{testRoleScope, testRoleScope + "/", "/" + testRoleUser, "", "readers/" + testRoleUser, testRoleScope + "/alice"},
		},
	} {
		t.Run(tc.desc.TypeName, func(t *testing.T) {
			if got := buildAssignmentID([]string{tc.want["role_id"], tc.want[tc.assignee]}); got != tc.id {
				t.Fatalf("built ID %q, want %q", got, tc.id)
			}
			r := newTestAssignmentResourceFor(tc.desc, newMemoryScopeAdapter())
			resp := testImport(t, r, tc.id)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if s := testStateValuesOf(t, r, resp.State); s.id != tc.id || !reflect.DeepEqual(s.identity, tc.want) {
				t.Fatalf("state %+v", s)
			}
			for _, bad := range tc.bad {
				if resp := testImport(t, r, bad); !resp.Diagnostics.HasError() {
					t.Errorf("import %q accepted", bad)
				}
			}
		})
	}
}

// Lakekeeper rejects making a role an assignee or owner of itself with a 400.
// Through the real role adapter, that rejection reaches the user as a create
// diagnostic that names the operation and carries Lakekeeper's message.
func TestRoleRoleAssignmentSelfAssignment(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"assignments":[]}`))
			return
		}
		writes++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Cannot assign ` + testRoleScope + ` to itself","type":"SelfAssignment","stack":[]}}`))
	}))
	t.Cleanup(server.Close)
	client, err := api.NewStaticTokenClient(context.Background(), server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	r := newTestAssignmentResourceFor(roleRoleAssignment, newRoleScopeAdapter(client))
	resp := testCreateFor(t, r, []string{testRoleScope, testRoleScope}, "assignee")
	if writes != 1 {
		t.Fatalf("want one write request, got %d", writes)
	}
	if resp.Diagnostics.ErrorsCount() != 1 {
		t.Fatalf("want one error, got %v", resp.Diagnostics)
	}
	d := resp.Diagnostics.Errors()[0]
	if d.Summary() != "Unable to create role grants of the role" {
		t.Errorf("summary %q", d.Summary())
	}
	if !strings.Contains(d.Detail(), "Cannot assign "+testRoleScope+" to itself") {
		t.Errorf("detail lacks Lakekeeper's message: %q", d.Detail())
	}
}
