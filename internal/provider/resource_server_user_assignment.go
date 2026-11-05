package provider

func init() {
	registerResource(newAssignmentResource(serverUserAssignment))
}

// serverUserAssignment manages the direct grants of one user on the server.
// The server has no scope attribute, so the ID is the bare user ID.
var serverUserAssignment = assignmentDescriptor{
	TypeName:     "server_user_assignment",
	ScopeNoun:    "server",
	AssigneeKind: userAssignee,
	Assignee:     userIDAttribute("user_id", "The ID of the user, in the form `<idp>~<subject>`."),
	GrantTypes:   serverGrantTypes,
	NewAdapter:   newServerScopeAdapter,
}
