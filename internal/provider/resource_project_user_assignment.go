package provider

func init() {
	registerResource(newAssignmentResource(projectUserAssignment))
}

// projectUserAssignment manages the direct grants of one user on a project.
var projectUserAssignment = assignmentDescriptor{
	TypeName:     "project_user_assignment",
	ScopeNoun:    "project",
	Scope:        projectIDAttribute("The ID of the project."),
	AssigneeKind: userAssignee,
	Assignee:     userIDAttribute("user_id", "The ID of the user, in the form `<idp>~<subject>`."),
	GrantTypes:   projectGrantTypes,
	NewAdapter:   newProjectScopeAdapter,
}
