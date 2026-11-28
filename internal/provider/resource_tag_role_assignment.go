package provider

import (
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(newAssignmentResource(tagRoleAssignment))
}

// tagRoleAssignment manages the direct grants of a role on a governance tag definition.
var tagRoleAssignment = assignmentDescriptor{
	TypeName:     "tag_role_assignment",
	ScopeNoun:    "tag",
	Scope:        uuidAttribute(tagIDAttributeName, "The ID of the governance tag definition."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("role_id", "The ID of the role."),
	GrantTypes:   tagGrantTypes,
	Notes:        "~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.",
	NewAdapter:   newTagScopeAdapter,
}

// tagGrantTypes lists the grant types at the tag scope, shared by the tag
// role and tag user assignment resources.
var tagGrantTypes = []string{
	api.TagAssignmentOwnership,
	api.TagAssignmentApply,
}
