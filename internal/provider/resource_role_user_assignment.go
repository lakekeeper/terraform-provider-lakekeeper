package provider

func init() {
	registerResource(newAssignmentResource(roleUserAssignment))
}

// roleUserAssignment manages the direct grants of one user on a role.
var roleUserAssignment = assignmentDescriptor{
	TypeName:     "role_user_assignment",
	ScopeNoun:    "role",
	Scope:        uuidAttribute(roleIDAttributeName, "The ID of the role the grants are on."),
	AssigneeKind: userAssignee,
	Assignee:     userIDAttribute("user_id", "The ID of the user, in the form `<idp>~<subject>`."),
	GrantTypes:   roleGrantTypes,
	NewAdapter:   newRoleScopeAdapter,
}
