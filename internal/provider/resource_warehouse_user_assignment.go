package provider

func init() {
	registerResource(newAssignmentResource(warehouseUserAssignment))
}

// warehouseUserAssignment manages the direct grants of one user on a warehouse.
var warehouseUserAssignment = assignmentDescriptor{
	TypeName:     "warehouse_user_assignment",
	ScopeNoun:    "warehouse",
	Scope:        uuidAttribute(warehouseIDAttributeName, "The ID of the warehouse."),
	AssigneeKind: userAssignee,
	Assignee:     userIDAttribute("user_id", "The ID of the user, in the form `<idp>~<subject>`."),
	GrantTypes:   warehouseGrantTypes,
	NewAdapter:   newWarehouseScopeAdapter,
}
