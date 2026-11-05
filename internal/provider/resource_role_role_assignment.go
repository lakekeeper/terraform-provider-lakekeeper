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
	Scope:        uuidAttribute(roleIDAttributeName, "The ID of the role the grants are on."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("assignee_id", "The ID of the role that receives the grants."),
	GrantTypes:   roleGrantTypes,
	Notes: "Here the grants are on the role in `role_id`, and the role in `assignee_id` holds them. " +
		"Lakekeeper rejects assigning a role to itself.",
	NewAdapter: newRoleScopeAdapter,
}

// roleGrantTypes lists the grant types at the role scope, shared by the role
// role and role user assignment resources.
var roleGrantTypes = []string{
	api.RoleAssignmentOwnership,
	api.RoleAssignmentAssignee,
}
