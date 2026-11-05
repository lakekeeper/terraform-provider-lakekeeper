package provider

import (
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
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
	GrantTypes:   projectGrantTypes,
	NewAdapter:   newProjectScopeAdapter,
}

// projectGrantTypes lists the grant types at the project scope, shared by the
// project role and user assignment resources.
var projectGrantTypes = []string{
	api.ProjectAssignmentAdmin,
	api.ProjectAssignmentSecurityAdmin,
	api.ProjectAssignmentDataAdmin,
	api.ProjectAssignmentRoleCreator,
	api.ProjectAssignmentDescribe,
	api.ProjectAssignmentSelect,
	api.ProjectAssignmentCreate,
	api.ProjectAssignmentModify,
}
