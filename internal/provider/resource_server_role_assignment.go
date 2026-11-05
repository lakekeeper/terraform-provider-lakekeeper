package provider

import (
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(newAssignmentResource(serverRoleAssignment))
}

// serverRoleAssignment manages the direct grants of one role on the server.
// The server has no scope attribute, so the ID is the bare role ID.
var serverRoleAssignment = assignmentDescriptor{
	TypeName:     "server_role_assignment",
	ScopeNoun:    "server",
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("role_id", "The ID of the role."),
	GrantTypes:   serverGrantTypes,
	NewAdapter:   newServerScopeAdapter,
}

// serverGrantTypes lists the grant types at the server scope, shared by the
// server role and user assignment resources.
var serverGrantTypes = []string{
	api.ServerAssignmentAdmin,
	api.ServerAssignmentOperator,
}
