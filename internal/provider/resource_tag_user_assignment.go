package provider

func init() {
	registerResource(newAssignmentResource(tagUserAssignment))
}

// tagUserAssignment manages the direct grants of a user on a governance tag definition.
var tagUserAssignment = assignmentDescriptor{
	TypeName:     "tag_user_assignment",
	ScopeNoun:    "tag",
	Scope:        uuidAttribute(tagIDAttributeName, "The ID of the governance tag definition."),
	AssigneeKind: userAssignee,
	Assignee:     userIDAttribute("user_id", "The ID of the user."),
	GrantTypes:   tagGrantTypes,
	Notes:        "~> **Preview:** Governance tags are a **preview** feature in Lakekeeper and may change in future releases.",
	NewAdapter:   newTagScopeAdapter,
}
