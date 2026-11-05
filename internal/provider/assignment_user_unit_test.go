package provider

import (
	"context"
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

func TestAssignmentUserIDValidator(t *testing.T) {
	ctx := context.Background()
	attr := userIDAttribute("user_id", "User.")
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"oidc~cfb55bf6-fcbb-4a1e-bfec-30c6649b52f8", true},
		{"kubernetes~system:serviceaccount:ns/name", true},
		{"oidc~a/b", true},
		{"oidc~a~b", true},
		{"oidc~" + strings.Repeat("x", 127), true},
		// Lakekeeper counts the subject in bytes (Rust str::len): 63 two-byte runes plus one byte is 127 bytes.
		{"oidc~" + strings.Repeat("ä", 63) + "x", true},
		{"oidc~" + strings.Repeat("ä", 64), false},
		{"oidc~" + strings.Repeat("ä", 127), false},
		{"", false},
		{"no-tilde", false},
		{"~subject", false},
		{"oidc~", false},
		{"oidc~" + strings.Repeat("x", 128), false},
		{"oidc~a\nb", false},
		{"oidc~a\x00b", false},
		{"oi\tdc~a", false},
		{"oidc~a\u0085b", false}, // C1 control (NEL), also a control character in Rust
	} {
		resp := validator.StringResponse{}
		for _, v := range attr.Validators {
			v.ValidateString(ctx, validator.StringRequest{Path: path.Root(attr.Name), ConfigValue: types.StringValue(tc.value)}, &resp)
		}
		if resp.Diagnostics.HasError() == tc.ok {
			t.Errorf("user_id %q: want ok=%v, got %v", tc.value, tc.ok, resp.Diagnostics)
		}
	}
}

// slashUser is a valid user ID containing "/", which must survive ID building,
// parsing and import intact.
const slashUser = "oidc~team/alice"

func user(id, grant string) assignmentGrant {
	return assignmentGrant{AssigneeKind: userAssignee, AssigneeID: id, Type: grant}
}

func TestProjectUserAssignmentSchema(t *testing.T) {
	r := &assignmentResource{desc: projectUserAssignment}
	resp := resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "lakekeeper"}, &resp)
	if resp.TypeName != "lakekeeper_project_user_assignment" {
		t.Fatalf("type name %q", resp.TypeName)
	}
	s := testSchema(t, r)
	for _, name := range []string{"id", "project_id", "user_id", "assignments"} {
		if _, ok := s.Attributes[name]; !ok {
			t.Fatalf("missing attribute %q", name)
		}
	}
	ctx := context.Background()
	for value, ok := range map[string]bool{slashUser: true, "no-tilde": false} {
		vresp := validator.StringResponse{}
		for _, v := range s.Attributes["user_id"].(schema.StringAttribute).Validators {
			v.ValidateString(ctx, validator.StringRequest{Path: path.Root("user_id"), ConfigValue: types.StringValue(value)}, &vresp)
		}
		if vresp.Diagnostics.HasError() == ok {
			t.Errorf("user_id %q: want ok=%v, got %v", value, ok, vresp.Diagnostics)
		}
	}
}

func TestProjectUserAssignmentLifecycle(t *testing.T) {
	// A role with the same ID as the user is a different assignee and must never change.
	sameIDRole := role(slashUser, "select")
	others := append(otherGrants(), sameIDRole)
	assertOthers := func(t *testing.T, m *memoryScopeAdapter) {
		t.Helper()
		for _, g := range others {
			if !slices.Contains(m.grants, g) {
				t.Fatalf("grant of another assignee was removed: %+v", g)
			}
		}
		for _, a := range m.applies {
			for _, g := range append(slices.Clone(a.revokes), a.grants...) {
				if g.AssigneeKind != userAssignee || g.AssigneeID != slashUser {
					t.Fatalf("request touched another assignee: %+v", g)
				}
			}
		}
	}

	t.Run("create grants the user and keeps the slash in the ID", func(t *testing.T) {
		m := newMemoryScopeAdapter(others...)
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: userTestValue(t, r, "", "p", slashUser, "describe", "select")}
		resp := &resource.CreateResponse{State: testNullState(t, r)}
		r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got := m.grantsOf(userAssignee, slashUser); !reflect.DeepEqual(got, []string{"describe", "select"}) {
			t.Fatalf("server grants %v", got)
		}
		if len(m.applies) != 1 || !reflect.DeepEqual(m.applies[0].scope, assignmentScope{"project_id": "p"}) {
			t.Fatalf("applies %+v", m.applies)
		}
		assertOthers(t, m)
		assertUserState(t, resp.State, "p/"+slashUser, "p", slashUser, "describe", "select")
	})

	t.Run("create refuses when the user already holds a grant", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(slices.Clone(others), user(slashUser, "modify"))...)
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: userTestValue(t, r, "", "p", slashUser, "select")}
		resp := &resource.CreateResponse{State: testNullState(t, r)}
		r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)
		if !resp.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(resp.Diagnostics), "terraform import") {
			t.Fatalf("want an error telling the user to import, got %v", resp.Diagnostics)
		}
		if len(m.applies) != 0 || !resp.State.Raw.IsNull() {
			t.Fatalf("writes %d, state %v", len(m.applies), resp.State.Raw)
		}
	})

	t.Run("read keeps only the user's grants", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(slices.Clone(others), user(slashUser, "modify"))...)
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		state := userTestState(t, r, "p/"+slashUser, "p", slashUser, "describe")
		resp := &resource.ReadResponse{State: state}
		r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		assertUserState(t, resp.State, "p/"+slashUser, "p", slashUser, "modify")
	})

	t.Run("update reconciles against the server in one request", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(slices.Clone(others), user(slashUser, "select"), user(slashUser, "describe"))...)
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		state := userTestState(t, r, "p/"+slashUser, "p", slashUser, "select")
		plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: userTestValue(t, r, "p/"+slashUser, "p", slashUser, "select", "modify")}
		resp := &resource.UpdateResponse{State: state}
		r.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if len(m.applies) != 1 {
			t.Fatalf("want one write request, got %d", len(m.applies))
		}
		a := m.applies[0]
		if !reflect.DeepEqual(a.revokes, []assignmentGrant{user(slashUser, "describe")}) || !reflect.DeepEqual(a.grants, []assignmentGrant{user(slashUser, "modify")}) {
			t.Fatalf("revokes %v grants %v", a.revokes, a.grants)
		}
		assertOthers(t, m)
		assertUserState(t, resp.State, "p/"+slashUser, "p", slashUser, "modify", "select")
	})

	t.Run("delete revokes only the user's grants", func(t *testing.T) {
		m := newMemoryScopeAdapter(append(slices.Clone(others), user(slashUser, "select"), user(slashUser, "modify"))...)
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		state := userTestState(t, r, "p/"+slashUser, "p", slashUser, "select")
		resp := &resource.DeleteResponse{State: state}
		r.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if got := m.grantsOf(userAssignee, slashUser); len(got) != 0 {
			t.Fatalf("left grants %v", got)
		}
		if len(m.applies) != 1 || !resp.State.Raw.IsNull() {
			t.Fatalf("writes %d, state %v", len(m.applies), resp.State.Raw)
		}
		assertOthers(t, m)
	})

	t.Run("failed delete keeps state", func(t *testing.T) {
		m := newMemoryScopeAdapter(user(slashUser, "select"))
		m.applyErr = errTestWrite
		r := &assignmentResource{desc: projectUserAssignment, adapter: m}
		state := userTestState(t, r, "p/"+slashUser, "p", slashUser, "select")
		resp := &resource.DeleteResponse{State: state}
		r.Delete(context.Background(), resource.DeleteRequest{State: state}, resp)
		if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
			t.Fatalf("diags %v, state %v", resp.Diagnostics, resp.State.Raw)
		}
	})
}

func TestProjectUserAssignmentImport(t *testing.T) {
	r := &assignmentResource{desc: projectUserAssignment, adapter: newMemoryScopeAdapter()}
	resp := &resource.ImportStateResponse{State: testNullState(t, r)}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "p/" + slashUser}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	id, project, userID := userTestIdentity(t, resp.State)
	if id != "p/"+slashUser || project != "p" || userID != slashUser {
		t.Fatalf("imported id %q project_id %q user_id %q", id, project, userID)
	}

	for _, bad := range []string{"oidc~alice", "p/", "/" + slashUser, "", "p/no-tilde", "p/~alice", "p/oidc~" + strings.Repeat("ä", 64)} {
		resp := &resource.ImportStateResponse{State: testNullState(t, r)}
		r.ImportState(context.Background(), resource.ImportStateRequest{ID: bad}, resp)
		if !resp.Diagnostics.HasError() {
			t.Errorf("import %q accepted", bad)
		}
	}
}

// --- helpers for the project user schema (id, project_id, user_id, assignments) ---

// userTestValue builds a resource object. An empty id means unknown (as in a create plan).
func userTestValue(t *testing.T, r *assignmentResource, id, project, userID string, grants ...string) tftypes.Value {
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
		"user_id":     tftypes.NewValue(tftypes.String, userID),
		"assignments": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, elems),
	})
}

func userTestState(t *testing.T, r *assignmentResource, id, project, userID string, grants ...string) tfsdk.State {
	return tfsdk.State{Schema: testSchema(t, r), Raw: userTestValue(t, r, id, project, userID, grants...)}
}

func userTestIdentity(t *testing.T, state tfsdk.State) (id, project, userID string) {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{"id", "project_id", "user_id"} {
		var v types.String
		if d := state.GetAttribute(context.Background(), path.Root(name), &v); d.HasError() {
			t.Fatal(d)
		}
		out[name] = v.ValueString()
	}
	return out["id"], out["project_id"], out["user_id"]
}

func assertUserState(t *testing.T, state tfsdk.State, id, project, userID string, grants ...string) {
	t.Helper()
	gotID, gotProject, gotUser := userTestIdentity(t, state)
	if gotID != id || gotProject != project || gotUser != userID {
		t.Fatalf("state identity id %q project_id %q user_id %q", gotID, gotProject, gotUser)
	}
	var set types.Set
	if d := state.GetAttribute(context.Background(), path.Root("assignments"), &set); d.HasError() {
		t.Fatal(d)
	}
	got := []string{}
	if d := set.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatal(d)
	}
	slices.Sort(got)
	if grants == nil {
		grants = []string{}
	}
	if !reflect.DeepEqual(got, grants) {
		t.Fatalf("state grants %v, want %v", got, grants)
	}
}
