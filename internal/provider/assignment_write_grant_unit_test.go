package provider

import (
	"slices"
	"testing"
)

// Lakekeeper 0.15 added the `write` privilege (data, schema, properties and
// snapshots) next to `modify`. It exists on projects, warehouses and
// namespaces but not on servers and roles; without it in the allowed values,
// an assignment cannot declare it and an apply revokes it.
func TestWriteGrantTypeAtScopes(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc assignmentDescriptor
		want bool
	}{
		{"project role", projectRoleAssignment, true},
		{"project user", projectUserAssignment, true},
		{"warehouse role", warehouseRoleAssignment, true},
		{"warehouse user", warehouseUserAssignment, true},
		{"namespace role", namespaceRoleAssignment, true},
		{"server role", serverRoleAssignment, false},
		{"server user", serverUserAssignment, false},
		{"role role", roleRoleAssignment, false},
		{"role user", roleUserAssignment, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := slices.Contains(tc.desc.GrantTypes, "write"); got != tc.want {
				t.Errorf("%s grant types %v: write allowed = %v, want %v", tc.name, tc.desc.GrantTypes, got, tc.want)
			}
		})
	}
}
