package api

// Lakekeeper enum values used by the provider (schemas, validators, docs).
// They are plain strings so resources can use them without conversions.

// Assignee types.
const (
	AssigneeTypeUser = "user"
	AssigneeTypeRole = "role"
)

// User types.
const (
	UserTypeHuman       = "human"
	UserTypeApplication = "application"
)

// Server assignments.
const (
	ServerAssignmentOperator = "operator"
	ServerAssignmentAdmin    = "admin"
)

// Project assignments.
const (
	ProjectAssignmentAdmin         = "project_admin"
	ProjectAssignmentSecurityAdmin = "security_admin"
	ProjectAssignmentDataAdmin     = "data_admin"
	ProjectAssignmentRoleCreator   = "role_creator"
	ProjectAssignmentDescribe      = "describe"
	ProjectAssignmentSelect        = "select"
	ProjectAssignmentCreate        = "create"
	ProjectAssignmentModify        = "modify"
)

// Warehouse assignments.
const (
	WarehouseAssignmentOwnership    = "ownership"
	WarehouseAssignmentPassGrants   = "pass_grants"
	WarehouseAssignmentManageGrants = "manage_grants"
	WarehouseAssignmentDescribe     = "describe"
	WarehouseAssignmentSelect       = "select"
	WarehouseAssignmentCreate       = "create"
	WarehouseAssignmentModify       = "modify"
)

// Role assignments.
const (
	RoleAssignmentOwnership = "ownership"
	RoleAssignmentAssignee  = "assignee"
)

// Project actions, as returned by the project access endpoint.
const (
	ProjectActionCreateWarehouse       = "create_warehouse"
	ProjectActionDelete                = "delete"
	ProjectActionRename                = "rename"
	ProjectActionListWarehouses        = "list_warehouses"
	ProjectActionCreateRole            = "create_role"
	ProjectActionListRoles             = "list_roles"
	ProjectActionSearchRoles           = "search_roles"
	ProjectActionGetEndpointStatistics = "get_endpoint_statistics"
	ProjectActionReadAssignments       = "read_assignments"
	ProjectActionGrantRoleCreator      = "grant_role_creator"
	ProjectActionGrantCreate           = "grant_create"
	ProjectActionGrantDescribe         = "grant_describe"
	ProjectActionGrantModify           = "grant_modify"
	ProjectActionGrantSelect           = "grant_select"
	ProjectActionGrantProjectAdmin     = "grant_project_admin"
	ProjectActionGrantSecurityAdmin    = "grant_security_admin"
	ProjectActionGrantDataAdmin        = "grant_data_admin"
)

// Warehouse actions, as returned by the warehouse access endpoint.
const (
	WarehouseActionCreateNamespace         = "create_namespace"
	WarehouseActionDelete                  = "delete"
	WarehouseActionModifyStorage           = "modify_storage"
	WarehouseActionModifyStorageCredential = "modify_storage_credential"
	WarehouseActionGetConfig               = "get_config"
	WarehouseActionGetMetadata             = "get_metadata"
	WarehouseActionListNamespaces          = "list_namespaces"
	WarehouseActionIncludeInList           = "include_in_list"
	WarehouseActionDeactivate              = "deactivate"
	WarehouseActionActivate                = "activate"
	WarehouseActionRename                  = "rename"
	WarehouseActionListDeletedTabulars     = "list_deleted_tabulars"
	WarehouseActionReadAssignments         = "read_assignments"
	WarehouseActionGrantCreate             = "grant_create"
	WarehouseActionGrantDescribe           = "grant_describe"
	WarehouseActionGrantModify             = "grant_modify"
	WarehouseActionGrantSelect             = "grant_select"
	WarehouseActionGrantPassGrants         = "grant_pass_grants"
	WarehouseActionGrantManageGrants       = "grant_manage_grants"
	WarehouseActionChangeOwnership         = "change_ownership"
	WarehouseActionGetAllTasks             = "get_all_tasks"
	WarehouseActionControlAllTasks         = "control_all_tasks"
	WarehouseActionGetEndpointStatistics   = "get_endpoint_statistics"
)

// Warehouse statuses.
const (
	WarehouseStatusActive   = "active"
	WarehouseStatusInactive = "inactive"
)

// Delete profile types.
const (
	DeleteProfileTypeHard = "hard"
	DeleteProfileTypeSoft = "soft"
)

// S3 flavors.
const (
	S3FlavorAWS      = "aws"
	S3FlavorS3Compat = "s3-compat"
)

// S3 remote signing URL styles.
const (
	RemoteSigningURLStyleAuto        = "auto"
	RemoteSigningURLStylePath        = "path"
	RemoteSigningURLStyleVirtualHost = "virtual_host"
)
