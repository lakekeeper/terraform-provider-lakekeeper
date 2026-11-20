package provider

import (
	"slices"
	"testing"
)

// Lakekeeper's tag model adds `manage_tags` on warehouses and namespaces and
// `tag_creator` on projects. Assignment resources own every direct grant of
// their assignee at the scope, so a grant type missing from the allowed values
// cannot be declared and is revoked by the next apply.
func TestTagGrantTypesAtScopes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		desc      assignmentDescriptor
		grantType string
		want      bool
	}{
		{"project role", projectRoleAssignment, "tag_creator", true},
		{"project user", projectUserAssignment, "tag_creator", true},
		{"warehouse role", warehouseRoleAssignment, "manage_tags", true},
		{"warehouse user", warehouseUserAssignment, "manage_tags", true},
		{"namespace role", namespaceRoleAssignment, "manage_tags", true},
		{"project role has no manage_tags", projectRoleAssignment, "manage_tags", false},
		{"warehouse role has no tag_creator", warehouseRoleAssignment, "tag_creator", false},
		{"server role", serverRoleAssignment, "manage_tags", false},
		{"role role", roleRoleAssignment, "manage_tags", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := slices.Contains(tc.desc.GrantTypes, tc.grantType); got != tc.want {
				t.Errorf("%s grant types %v: %s allowed = %v, want %v", tc.name, tc.desc.GrantTypes, tc.grantType, got, tc.want)
			}
		})
	}
}
