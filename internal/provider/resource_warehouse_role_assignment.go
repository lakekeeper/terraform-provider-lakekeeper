package provider

import (
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func init() {
	registerResource(newAssignmentResource(warehouseRoleAssignment))
}

// warehouseRoleAssignment manages the direct grants of one role on a warehouse.
var warehouseRoleAssignment = assignmentDescriptor{
	TypeName:     "warehouse_role_assignment",
	ScopeNoun:    "warehouse",
	Scope:        uuidAttribute(warehouseIDAttributeName, "The ID of the warehouse."),
	AssigneeKind: roleAssignee,
	Assignee:     uuidAttribute("role_id", "The ID of the role."),
	GrantTypes:   warehouseGrantTypes,
	NewAdapter:   newWarehouseScopeAdapter,
}

// warehouseGrantTypes lists the grant types at the warehouse scope, shared by
// the warehouse role and user assignment resources.
var warehouseGrantTypes = []string{
	api.WarehouseAssignmentOwnership,
	api.WarehouseAssignmentPassGrants,
	api.WarehouseAssignmentManageGrants,
	api.WarehouseAssignmentDescribe,
	api.WarehouseAssignmentSelect,
	api.WarehouseAssignmentCreate,
	api.WarehouseAssignmentModify,
}
