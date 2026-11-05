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
	Scope:        ptr(projectIDAttribute("The ID of the project.")),
	AssigneeKind: roleAssignee,
	Assignee:     roleIDAttribute("role_id", "The ID of the role."),
	GrantTypes:   grantTypes(permissionv1.ValidProjectAssignmentTypes),
	NewAdapter:   newProjectScopeAdapter,
}
