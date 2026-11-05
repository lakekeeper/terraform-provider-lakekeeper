package provider

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Descriptor-agnostic helpers for driving the assignment module in unit tests.
// identity is always the identity values in ID order (extra parts, scope,
// assignee), as returned by assignmentDescriptor.parseID.

// newTestAssignmentResourceFor returns the module for a real descriptor, with
// the given adapter in place of the one NewAdapter would build.
func newTestAssignmentResourceFor(d assignmentDescriptor, m scopeAdapter) *assignmentResource {
	return &assignmentResource{d: d, adapter: m}
}

// testObject builds a resource object for any descriptor. An empty id means
// unknown, as in a create plan.
func testObject(t *testing.T, r *assignmentResource, id string, identity []string, grants ...string) tftypes.Value {
	t.Helper()
	attrs := r.d.identity()
	if len(identity) != len(attrs) {
		t.Fatalf("want %d identity values, got %v", len(attrs), identity)
	}
	idValue := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	if id != "" {
		idValue = tftypes.NewValue(tftypes.String, id)
	}
	elems := []tftypes.Value{}
	for _, g := range grants {
		elems = append(elems, tftypes.NewValue(tftypes.String, g))
	}
	values := map[string]tftypes.Value{
		"id":          idValue,
		"assignments": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, elems),
	}
	for i, a := range attrs {
		values[a.Name] = tftypes.NewValue(tftypes.String, identity[i])
	}
	return tftypes.NewValue(testSchema(t, r).Type().TerraformType(context.Background()), values)
}

// testStateFor is the stored state of an existing assignment.
func testStateFor(t *testing.T, r *assignmentResource, identity []string, grants ...string) tfsdk.State {
	t.Helper()
	return tfsdk.State{Schema: testSchema(t, r), Raw: testObject(t, r, r.d.buildID(identity), identity, grants...)}
}

func testCreateFor(t *testing.T, r *assignmentResource, identity []string, grants ...string) *resource.CreateResponse {
	t.Helper()
	plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: testObject(t, r, "", identity, grants...)}
	resp := &resource.CreateResponse{State: testNullState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: plan}, resp)
	return resp
}

func testUpdateFor(t *testing.T, r *assignmentResource, state tfsdk.State, identity []string, grants ...string) *resource.UpdateResponse {
	t.Helper()
	plan := tfsdk.Plan{Schema: testSchema(t, r), Raw: testObject(t, r, r.d.buildID(identity), identity, grants...)}
	resp := &resource.UpdateResponse{State: state}
	r.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, resp)
	return resp
}

// testStateValuesOf reads the ID, every identity attribute of the descriptor,
// and the sorted grants from a state.
func testStateValuesOf(t *testing.T, r *assignmentResource, state tfsdk.State) testStateValues {
	t.Helper()
	ctx := context.Background()
	out := testStateValues{identity: map[string]string{}}
	var id types.String
	if d := state.GetAttribute(ctx, path.Root("id"), &id); d.HasError() {
		t.Fatal(d)
	}
	out.id = id.ValueString()
	for _, a := range r.d.identity() {
		var v types.String
		if d := state.GetAttribute(ctx, path.Root(a.Name), &v); d.HasError() {
			t.Fatal(d)
		}
		out.identity[a.Name] = v.ValueString()
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

// assertStateFor checks that state holds the ID built from identity, every
// identity value, and exactly the given grants.
func assertStateFor(t *testing.T, r *assignmentResource, state tfsdk.State, identity []string, grants ...string) {
	t.Helper()
	s := testStateValuesOf(t, r, state)
	want := testStateValues{id: r.d.buildID(identity), identity: map[string]string{}, grants: slices.Sorted(slices.Values(grants))}
	for i, a := range r.d.identity() {
		want.identity[a.Name] = identity[i]
	}
	if want.grants == nil {
		want.grants = []string{}
	}
	if s.grants == nil {
		s.grants = []string{}
	}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("state %+v, want %+v", s, want)
	}
}
