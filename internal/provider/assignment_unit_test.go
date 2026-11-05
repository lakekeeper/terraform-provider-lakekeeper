package provider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAssignmentIDParse(t *testing.T) {
	project := testAssignmentDescriptor()
	server := testAssignmentDescriptor()
	server.Scope = assignmentAttribute{}
	server.ScopeNoun = "server"
	namespace := testAssignmentDescriptor()
	namespace.ExtraIDParts = []assignmentAttribute{projectIDAttribute("Project.")}
	namespace.Scope = assignmentAttribute{Name: "namespace_id"}
	user := testAssignmentDescriptor()
	user.Assignee = assignmentAttribute{Name: "user_id"}

	for _, tc := range []struct {
		name string
		d    assignmentDescriptor
		id   string
		want []string // identity values in ID order; nil means malformed
	}{
		{"project role", project, "p/r", []string{"p", "r"}},
		{"assignee keeps slashes", user, "p/oidc~a/b", []string{"p", "oidc~a/b"}},
		{"server bare id", server, "r", []string{"r"}},
		{"server bare id with slash", server, "oidc~a/b", []string{"oidc~a/b"}},
		{"namespace three parts", namespace, "p/n/r", []string{"p", "n", "r"}},
		{"too few parts", project, "r", nil},
		{"empty scope", project, "/r", nil},
		{"empty assignee", project, "p/", nil},
		{"empty", server, "", nil},
		{"namespace too few", namespace, "p/n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.d.parseID(tc.id)
			if tc.want == nil {
				if err == nil {
					t.Fatalf("parseID(%q) accepted malformed ID: %v", tc.id, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseID(%q): %v", tc.id, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseID(%q) = %v, want %v", tc.id, got, tc.want)
			}
			if built := buildAssignmentID(got); built != tc.id {
				t.Fatalf("buildID(%v) = %q, want %q", got, built, tc.id)
			}
		})
	}
}

func TestAssignmentIdentityValidators(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name     string
		attr     assignmentAttribute
		accepted []string
		rejected []string
	}{
		{
			name: "project_id",
			attr: projectIDAttribute("Project."),
			// Lakekeeper uses Rust's char::is_alphanumeric (Alphabetic or Numeric). Alphabetic
			// includes Other_Alphabetic marks such as Devanagari vowel signs (U+093F, U+0902),
			// which \p{L} misses; a plain combining accent (U+0301) is neither.
			accepted: []string{"00000000-0000-0000-0000-000000000000", "my_project", "Projekt-Ä1", "项目", "हिंदी", "Ⅻ", "½"},
			rejected: []string{"", "a/b", "a b", "a.b", "a~b", "a\nb", "é"},
		},
		{
			name:     "role_id",
			attr:     uuidAttribute("role_id", "Role."),
			accepted: []string{"cb6ee351-68ff-4299-87f2-876964f6d8dd", "CB6EE351-68FF-4299-87F2-876964F6D8DD"},
			rejected: []string{"", "my-role", "p/cb6ee351-68ff-4299-87f2-876964f6d8dd", "cb6ee351-68ff-4299-87f2-876964f6d8dd/x", "cb6ee35168ff429987f2876964f6d8dd"},
		},
	} {
		for _, v := range append(append([]string{}, tc.accepted...), tc.rejected...) {
			resp := validator.StringResponse{}
			for _, val := range tc.attr.Validators {
				val.ValidateString(ctx, validator.StringRequest{Path: path.Root(tc.attr.Name), ConfigValue: types.StringValue(v)}, &resp)
			}
			if want := slices.Contains(tc.rejected, v); resp.Diagnostics.HasError() != want {
				t.Errorf("%s %q: rejected=%v, want %v (%v)", tc.name, v, resp.Diagnostics.HasError(), want, resp.Diagnostics)
			}
		}
	}
}

var (
	errTestRead  = errors.New("read failed")
	errTestWrite = errors.New("write failed")
)

func role(id, grant string) assignmentGrant {
	return assignmentGrant{AssigneeKind: roleAssignee, AssigneeID: id, Type: grant}
}

// otherGrants are held by other assignees at the scope and must never change.
func otherGrants() []assignmentGrant {
	return []assignmentGrant{role("other", "select"), {AssigneeKind: "user", AssigneeID: "target", Type: "modify"}}
}

func TestAssignmentCreate(t *testing.T) {
	t.Run("grants desired set and leaves other assignees untouched", func(t *testing.T) {
		m := newMemoryScopeAdapter(otherGrants()...)
		r := newTestAssignmentResource(m)
		resp := testCreate(t, r, "p", "target", "select", "describe")
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got := m.grantsOf(roleAssignee, "target"); !reflect.DeepEqual(got, []string{"describe", "select"}) {
			t.Fatalf("server grants %v", got)
		}
		assertOtherGrantsUntouched(t, m)
		if len(m.applies) != 1 {
			t.Fatalf("want one write request, got %d", len(m.applies))
		}
		if !reflect.DeepEqual(m.applies[0].scope, assignmentScope{"project_id": "p"}) {
			t.Fatalf("wrong scope %v", m.applies[0].scope)
		}
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target", "describe", "select")
	})

	t.Run("refuses when the assignee already holds a grant", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(otherGrants(), role("target", "modify"))...)
		r := newTestAssignmentResource(m)
		resp := testCreate(t, r, "p", "target", "select")
		if !resp.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(resp.Diagnostics), "terraform import") {
			t.Fatalf("want an error telling the user to import, got %v", resp.Diagnostics)
		}
		if len(m.applies) != 0 {
			t.Fatal("refused create must not write")
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("refused create must not store state")
		}
	})

	t.Run("stores the ID before writing, without the planned grants", func(t *testing.T) {
		m := newMemoryScopeAdapter()
		m.applyErr = errTestWrite
		r := newTestAssignmentResource(m)
		resp := testCreate(t, r, "p", "target", "select")
		if !resp.Diagnostics.HasError() {
			t.Fatal("failed write must be an error")
		}
		if resp.State.Raw.IsNull() {
			t.Fatal("failed write must still leave the resource tracked")
		}
		// The state holds the grants the server held before the write: none.
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target")
	})

	t.Run("failed read never writes", func(t *testing.T) {
		m := newMemoryScopeAdapter()
		m.listErr = errTestRead
		r := newTestAssignmentResource(m)
		resp := testCreate(t, r, "p", "target", "select")
		if !resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
			t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
		}
	})

	t.Run("empty desired set sends nothing", func(t *testing.T) {
		m := newMemoryScopeAdapter(otherGrants()...)
		r := newTestAssignmentResource(m)
		resp := testCreate(t, r, "p", "target")
		if resp.Diagnostics.HasError() || len(m.applies) != 0 {
			t.Fatalf("diags %v, writes %d", resp.Diagnostics, len(m.applies))
		}
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target")
	})
}

func TestAssignmentRead(t *testing.T) {
	t.Run("keeps only this assignee's grants", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(otherGrants(), role("target", "modify"), role("target", "select"))...)
		r := newTestAssignmentResource(m)
		resp := testRead(t, r, testState(t, r, "p/target", "p", "target", "select"))
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target", "modify", "select")
	})

	t.Run("failed read is an error", func(t *testing.T) {
		m := newMemoryScopeAdapter()
		m.listErr = errTestRead
		r := newTestAssignmentResource(m)
		resp := testRead(t, r, testState(t, r, "p/target", "p", "target", "select"))
		if !resp.Diagnostics.HasError() {
			t.Fatal("want error")
		}
	})
}

func TestAssignmentUpdate(t *testing.T) {
	t.Run("revokes and grants against the server in one request", func(t *testing.T) {
		// The server drifted: describe was added outside Terraform.
		m := newMemoryScopeAdapter(append(otherGrants(), role("target", "select"), role("target", "describe"))...)
		r := newTestAssignmentResource(m)
		resp := testUpdate(t, r, testState(t, r, "p/target", "p", "target", "select"), "p", "target", "select", "modify")
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if len(m.applies) != 1 {
			t.Fatalf("want one write request, got %d", len(m.applies))
		}
		a := m.applies[0]
		if !reflect.DeepEqual(a.revokes, []assignmentGrant{role("target", "describe")}) || !reflect.DeepEqual(a.grants, []assignmentGrant{role("target", "modify")}) {
			t.Fatalf("revokes %v grants %v", a.revokes, a.grants)
		}
		assertOtherGrantsUntouched(t, m)
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target", "modify", "select")
	})

	t.Run("no-op sends nothing", func(t *testing.T) {
		m := newMemoryScopeAdapter(role("target", "select"))
		r := newTestAssignmentResource(m)
		resp := testUpdate(t, r, testState(t, r, "p/target", "p", "target", "describe"), "p", "target", "select")
		if resp.Diagnostics.HasError() || len(m.applies) != 0 {
			t.Fatalf("diags %v, writes %d", resp.Diagnostics, len(m.applies))
		}
	})

	t.Run("empty set revokes everything", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(otherGrants(), role("target", "select"), role("target", "modify"))...)
		r := newTestAssignmentResource(m)
		resp := testUpdate(t, r, testState(t, r, "p/target", "p", "target", "select"), "p", "target")
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got := m.grantsOf(roleAssignee, "target"); len(got) != 0 {
			t.Fatalf("left grants %v", got)
		}
		assertOtherGrantsUntouched(t, m)
		assertState(t, testStateOf(t, resp.State), "p/target", "p", "target")
	})

	t.Run("failed read never writes", func(t *testing.T) {
		m := newMemoryScopeAdapter(role("target", "select"))
		m.listErr = errTestRead
		r := newTestAssignmentResource(m)
		resp := testUpdate(t, r, testState(t, r, "p/target", "p", "target", "select"), "p", "target", "modify")
		if !resp.Diagnostics.HasError() || len(m.applies) != 0 {
			t.Fatalf("diags %v, writes %d", resp.Diagnostics, len(m.applies))
		}
	})

	t.Run("failed write is an error", func(t *testing.T) {
		m := newMemoryScopeAdapter(role("target", "select"))
		m.applyErr = errTestWrite
		r := newTestAssignmentResource(m)
		resp := testUpdate(t, r, testState(t, r, "p/target", "p", "target", "select"), "p", "target", "modify")
		if !resp.Diagnostics.HasError() {
			t.Fatal("want error")
		}
	})
}

func TestAssignmentDelete(t *testing.T) {
	t.Run("revokes this assignee's grants and removes state", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(otherGrants(), role("target", "select"), role("target", "modify"))...)
		r := newTestAssignmentResource(m)
		resp := testDelete(t, r, testState(t, r, "p/target", "p", "target", "select"))
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got := m.grantsOf(roleAssignee, "target"); len(got) != 0 {
			t.Fatalf("left grants %v", got)
		}
		assertOtherGrantsUntouched(t, m)
		if len(m.applies) != 1 || !resp.State.Raw.IsNull() {
			t.Fatalf("writes %d, state %v", len(m.applies), resp.State.Raw)
		}
	})

	t.Run("nothing to revoke sends nothing", func(t *testing.T) {
		m := newMemoryScopeAdapter(otherGrants()...)
		r := newTestAssignmentResource(m)
		resp := testDelete(t, r, testState(t, r, "p/target", "p", "target", "select"))
		if resp.Diagnostics.HasError() || len(m.applies) != 0 || !resp.State.Raw.IsNull() {
			t.Fatalf("diags %v, writes %d, state %v", resp.Diagnostics, len(m.applies), resp.State.Raw)
		}
	})

	for name, setup := range map[string]func(*memoryScopeAdapter){
		"failed write keeps state": func(m *memoryScopeAdapter) { m.applyErr = errTestWrite },
		"failed read keeps state":  func(m *memoryScopeAdapter) { m.listErr = errTestRead },
	} {
		t.Run(name, func(t *testing.T) {
			m := newMemoryScopeAdapter(role("target", "select"))
			setup(m)
			r := newTestAssignmentResource(m)
			resp := testDelete(t, r, testState(t, r, "p/target", "p", "target", "select"))
			if !resp.Diagnostics.HasError() {
				t.Fatal("want error")
			}
			if resp.State.Raw.IsNull() {
				t.Fatal("failed delete must keep state")
			}
			if m.listErr != nil && len(m.applies) != 0 {
				t.Fatal("failed read must not write")
			}
		})
	}
}

func TestAssignmentImport(t *testing.T) {
	r := newTestAssignmentResource(newMemoryScopeAdapter())

	resp := testImport(t, r, "p/cb6ee351-68ff-4299-87f2-876964f6d8dd")
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	s := testStateOf(t, resp.State)
	if s.id != "p/cb6ee351-68ff-4299-87f2-876964f6d8dd" || s.identity["project_id"] != "p" || s.identity["role_id"] != "cb6ee351-68ff-4299-87f2-876964f6d8dd" {
		t.Fatalf("state %+v", s)
	}

	for _, id := range []string{"cb6ee351-68ff-4299-87f2-876964f6d8dd", "p/", "/r", ""} {
		resp := testImport(t, r, id)
		if !resp.Diagnostics.HasError() {
			t.Errorf("import %q accepted", id)
		}
	}

	// Well-formed IDs whose parts break the identity validators fail like a plan would.
	for id, attr := range map[string]string{
		"p/not-a-uuid": "role_id",
		"p.x/cb6ee351-68ff-4299-87f2-876964f6d8dd": "project_id",
		"p/cb6ee351-68ff-4299-87f2-876964f6d8dd/x": "role_id",
	} {
		resp := testImport(t, r, id)
		if !resp.Diagnostics.HasError() {
			t.Errorf("import %q accepted", id)
			continue
		}
		if got := fmt.Sprint(resp.Diagnostics); !strings.Contains(got, "Attribute "+attr+" must be") || !strings.Contains(got, "Invalid import ID") {
			t.Errorf("import %q: want an invalid import ID diagnostic naming %s, got %v", id, attr, got)
		}
		if !resp.State.Raw.IsNull() {
			t.Errorf("import %q set state %v", id, resp.State.Raw)
		}
	}
}

func TestAssignmentGrantTypeValidator(t *testing.T) {
	ctx := context.Background()
	attr := testSchema(t, newTestAssignmentResource(newMemoryScopeAdapter())).Attributes["assignments"].(schema.SetAttribute)
	for _, tc := range []struct {
		grants []string
		ok     bool
	}{{[]string{"select", "describe"}, true}, {[]string{}, true}, {[]string{"select", "ownership"}, false}} {
		set, _ := types.SetValueFrom(ctx, types.StringType, tc.grants)
		resp := validator.SetResponse{}
		for _, v := range attr.Validators {
			v.ValidateSet(ctx, validator.SetRequest{Path: path.Root("assignments"), ConfigValue: set}, &resp)
		}
		if resp.Diagnostics.HasError() == tc.ok {
			t.Errorf("grants %v: want ok=%v, got %v", tc.grants, tc.ok, resp.Diagnostics)
		}
	}
}

// --- helpers driving the module through the framework's resource interface ---

func newTestAssignmentResource(m scopeAdapter) *assignmentResource {
	return &assignmentResource{desc: testAssignmentDescriptor(), adapter: m}
}

func testSchema(t *testing.T, r *assignmentResource) schema.Schema {
	t.Helper()
	resp := resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}

// testValue builds a resource object. An empty id means unknown (as in a create plan).
func testValue(t *testing.T, r *assignmentResource, id, project, roleID string, grants ...string) tftypes.Value {
	t.Helper()
	typ := testSchema(t, r).Type().TerraformType(context.Background())
	idValue := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	if id != "" {
		idValue = tftypes.NewValue(tftypes.String, id)
	}
	elems := []tftypes.Value{}
	for _, g := range grants {
		elems = append(elems, tftypes.NewValue(tftypes.String, g))
	}
	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"id":          idValue,
		"project_id":  tftypes.NewValue(tftypes.String, project),
		"role_id":     tftypes.NewValue(tftypes.String, roleID),
		"assignments": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, elems),
	})
}

func testState(t *testing.T, r *assignmentResource, id, project, roleID string, grants ...string) tfsdk.State {
	return tfsdk.State{Schema: testSchema(t, r), Raw: testValue(t, r, id, project, roleID, grants...)}
}

func testNullState(t *testing.T, r *assignmentResource) tfsdk.State {
	s := testSchema(t, r)
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}
}

func testCreate(t *testing.T, r *assignmentResource, project, roleID string, grants ...string) *resource.CreateResponse {
	plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: testValue(t, r, "", project, roleID, grants...)}
	resp := &resource.CreateResponse{State: testNullState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)
	return resp
}

func testRead(t *testing.T, r *assignmentResource, state tfsdk.State) *resource.ReadResponse {
	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	return resp
}

func testUpdate(t *testing.T, r *assignmentResource, state tfsdk.State, project, roleID string, grants ...string) *resource.UpdateResponse {
	id := project + "/" + roleID
	plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: testValue(t, r, id, project, roleID, grants...)}
	resp := &resource.UpdateResponse{State: state}
	r.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, resp)
	return resp
}

func testDelete(t *testing.T, r *assignmentResource, state tfsdk.State) *resource.DeleteResponse {
	resp := &resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)
	return resp
}

func testImport(t *testing.T, r *assignmentResource, id string) *resource.ImportStateResponse {
	resp := &resource.ImportStateResponse{State: testNullState(t, r)}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, resp)
	return resp
}

type testStateValues struct {
	id       string
	identity map[string]string
	grants   []string
}

func testStateOf(t *testing.T, state tfsdk.State) testStateValues {
	t.Helper()
	ctx := context.Background()
	out := testStateValues{identity: map[string]string{}}
	var id types.String
	if d := state.GetAttribute(ctx, path.Root("id"), &id); d.HasError() {
		t.Fatal(d)
	}
	out.id = id.ValueString()
	for _, name := range []string{"project_id", "role_id"} {
		var v types.String
		if d := state.GetAttribute(ctx, path.Root(name), &v); d.HasError() {
			t.Fatal(d)
		}
		out.identity[name] = v.ValueString()
	}
	var grants types.Set
	if d := state.GetAttribute(ctx, path.Root("assignments"), &grants); d.HasError() {
		t.Fatal(d)
	}
	if !grants.IsNull() {
		if d := grants.ElementsAs(ctx, &out.grants, false); d.HasError() {
			t.Fatal(d)
		}
	}
	slices.Sort(out.grants)
	return out
}

func assertState(t *testing.T, s testStateValues, id, project, roleID string, grants ...string) {
	t.Helper()
	if s.id != id || s.identity["project_id"] != project || s.identity["role_id"] != roleID {
		t.Fatalf("state identity %+v", s)
	}
	if grants == nil {
		grants = []string{}
	}
	if s.grants == nil {
		s.grants = []string{}
	}
	if !reflect.DeepEqual(s.grants, grants) {
		t.Fatalf("state grants %v, want %v", s.grants, grants)
	}
}

func assertOtherGrantsUntouched(t *testing.T, m *memoryScopeAdapter) {
	t.Helper()
	for _, g := range otherGrants() {
		if !slices.Contains(m.grants, g) {
			t.Fatalf("grant of another assignee was removed: %+v", g)
		}
	}
	for _, a := range m.applies {
		for _, g := range append(slices.Clone(a.revokes), a.grants...) {
			if g.AssigneeKind != roleAssignee || g.AssigneeID != "target" {
				t.Fatalf("request touched another assignee: %+v", g)
			}
		}
	}
}

// testAssignmentDescriptor returns a project role descriptor whose adapter is set by the test.
func testAssignmentDescriptor() assignmentDescriptor {
	return assignmentDescriptor{
		TypeName:     "test_role_assignment",
		ScopeNoun:    "project",
		Scope:        projectIDAttribute("The project."),
		AssigneeKind: roleAssignee,
		Assignee:     uuidAttribute("role_id", "The role."),
		GrantTypes:   []string{"describe", "select", "modify"},
	}
}
