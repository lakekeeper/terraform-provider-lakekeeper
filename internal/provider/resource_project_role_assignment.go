package provider

import (
	permissionv1 "github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1/permission"
)

func init() {
	registerResource(newAssignmentResource(projectRoleAssignment))
}

// projectRoleAssignment manages the direct grants of one role on a project.
var projectRoleAssignment = assignmentDescriptor{
	TypeName:     "project_role_assignment",
	ScopeNoun:    "project",
	Scope:        projectIDAttribute("The ID of the project."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("role_id", "The ID of the role."),
	GrantTypes:   stringValues(permissionv1.ValidProjectAssignmentTypes),
	NewAdapter:   newProjectScopeAdapter,
}
