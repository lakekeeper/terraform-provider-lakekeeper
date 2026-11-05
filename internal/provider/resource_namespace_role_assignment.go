package provider

func init() {
	registerResource(newAssignmentResource(namespaceRoleAssignment))
}

// namespaceRoleAssignment manages the direct grants of one role on a namespace.
var namespaceRoleAssignment = assignmentDescriptor{
	TypeName:     "namespace_role_assignment",
	ScopeNoun:    "namespace",
	ExtraIDParts: []assignmentAttribute{projectIDAttribute("The ID of the project containing both the namespace and the role.")},
	Scope:        uuidAttribute(namespaceIDAttributeName, "The ID of the namespace, available as `lakekeeper_namespace.<name>.namespace_id`."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("role_id", "The ID of the role."),
	// The api package has no namespace grant type constants.
	GrantTypes: []string{"ownership", "pass_grants", "manage_grants", "describe", "select", "create", "modify"},
	Notes: "Requires Lakekeeper's OpenFGA authorization backend. " +
		"Namespace grants apply to descendants according to Lakekeeper's authorization model. " +
		"A missing permissions endpoint is an error, including with allow-all.",
	NewAdapter: newNamespaceScopeAdapter,
}
