package provider

import (
	"reflect"
	"testing"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

const (
	testTagDefID  = "cb6ee351-68ff-4299-87f2-876964f6d8dd"
	testTagRoleID = "4a8f0a43-6c9e-4b8e-9f0b-5d0b8c1e2f33"
	testTagUserID = "oidc~alice"
)

func TestTagAssignmentSchemasAndLifecycle(t *testing.T) {
	cases := []struct {
		name     string
		desc     assignmentDescriptor
		kind     assigneeKind
		identity []string
	}{
		{"tag role", tagRoleAssignment, roleAssignee, []string{testTagDefID, testTagRoleID}},
		{"tag user", tagUserAssignment, userAssignee, []string{testTagDefID, testTagUserID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assignee := tc.identity[1]
			m := newMemoryScopeAdapter()
			r := newTestAssignmentResourceFor(tc.desc, m)

			created := testCreateFor(t, r, tc.identity, api.TagAssignmentApply)
			if created.Diagnostics.HasError() {
				t.Fatal(created.Diagnostics)
			}
			if want := (assignmentScope{tagIDAttributeName: testTagDefID}); !reflect.DeepEqual(m.applies[0].scope, want) {
				t.Fatalf("scope %v, want %v", m.applies[0].scope, want)
			}
			assertStateFor(t, r, created.State, tc.identity, api.TagAssignmentApply)

			updated := testUpdateFor(t, r, created.State, tc.identity, api.TagAssignmentOwnership)
			if updated.Diagnostics.HasError() {
				t.Fatal(updated.Diagnostics)
			}
			if got := m.grantsOf(tc.kind, assignee); !reflect.DeepEqual(got, []string{api.TagAssignmentOwnership}) {
				t.Fatalf("server grants %v", got)
			}

			deleted := testDelete(t, r, updated.State)
			if deleted.Diagnostics.HasError() || !deleted.State.Raw.IsNull() {
				t.Fatalf("diags %v, state %v", deleted.Diagnostics, deleted.State.Raw)
			}
		})
	}
}
