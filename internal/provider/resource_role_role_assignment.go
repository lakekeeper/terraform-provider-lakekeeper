package provider

import (
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(newAssignmentResource(roleRoleAssignment))
}

// roleRoleAssignment manages the direct grants of one role on another role.
var roleRoleAssignment = assignmentDescriptor{
	TypeName:     "role_role_assignment",
	ScopeNoun:    "role",
	Scope:        uuidAttribute(roleIDAttributeName, "The ID of the scope role, which the grants are on."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("assignee_id", "The ID of the assignee role, which holds the grants."),
	GrantTypes:   roleGrantTypes,
	Notes:        "Lakekeeper rejects assigning a role to itself.",
	NewAdapter:   newRoleScopeAdapter,
}

// roleGrantTypes lists the grant types at the role scope, shared by the role
// role and role user assignment resources.
var roleGrantTypes = []string{
	api.RoleAssignmentOwnership,
	api.RoleAssignmentAssignee,
}
