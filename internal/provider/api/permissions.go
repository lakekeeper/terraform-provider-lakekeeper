package api

import (
	"context"

	permissionv1 "github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1/permission"
)

// Assignee is the user or role a permission is granted to.
type Assignee struct {
	Type string // AssigneeTypeUser or AssigneeTypeRole
	ID   string
}

// UserAssignee returns the Assignee for a user ID.
func UserAssignee(id string) Assignee { return Assignee{Type: AssigneeTypeUser, ID: id} }

// RoleAssignee returns the Assignee for a role ID.
func RoleAssignee(id string) Assignee { return Assignee{Type: AssigneeTypeRole, ID: id} }

// Assignment grants a permission on an entity (server, project, warehouse,
// role or namespace) to an assignee. Assignment holds one of the
// <Entity>Assignment* constants for that entity.
type Assignment struct {
	Assignee   Assignee
	Assignment string
}

func assigneeToSDK(a Assignee) permissionv1.UserOrRole {
	return permissionv1.UserOrRole{Type: permissionv1.UserOrRoleType(a.Type), Value: a.ID}
}

func assigneeFromSDK(a permissionv1.UserOrRole) Assignee {
	return Assignee{Type: string(a.Type), ID: a.Value}
}

// principalOptions returns the (user, role) query parameters for an access check.
func principalOptions(p Assignee) (user, role *string) {
	if p.Type == AssigneeTypeRole {
		return nil, new(p.ID)
	}
	return new(p.ID), nil
}

// assignmentsToSDK converts assignments into SDK assignment values. It keeps
// nil for an empty input so the field is omitted on the wire.
func assignmentsToSDK[S any](in []Assignment, mk func(Assignment) *S) []*S {
	if len(in) == 0 {
		return nil
	}
	out := make([]*S, len(in))
	for i, a := range in {
		out[i] = mk(a)
	}
	return out
}

func assignmentsFromSDK[S any](in []*S, get func(*S) Assignment) []Assignment {
	out := make([]Assignment, len(in))
	for i, a := range in {
		out[i] = get(a)
	}
	return out
}

// ---------------------------------------------------------------------------
// Server

func serverAssignment(a Assignment) *permissionv1.ServerAssignment {
	return &permissionv1.ServerAssignment{Assignee: assigneeToSDK(a.Assignee), Assignment: permissionv1.ServerAssignmentType(a.Assignment)}
}

// GetServerAssignments lists all server assignments.
func (c *Client) GetServerAssignments(ctx context.Context) ([]Assignment, error) {
	r, resp, err := c.sdk.PermissionV1().ServerPermission().GetAssignments(ctx, nil)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return assignmentsFromSDK(r.Assignments, func(a *permissionv1.ServerAssignment) Assignment {
		return Assignment{Assignee: assigneeFromSDK(a.Assignee), Assignment: string(a.Assignment)}
	}), nil
}

// UpdateServerAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateServerAssignments(ctx context.Context, writes, deletes []Assignment) error {
	resp, err := c.sdk.PermissionV1().ServerPermission().Update(ctx, &permissionv1.UpdateServerPermissionsOptions{
		Writes:  assignmentsToSDK(writes, serverAssignment),
		Deletes: assignmentsToSDK(deletes, serverAssignment),
	})
	return wrapErr(resp, err)
}

// ---------------------------------------------------------------------------
// Project

func projectAssignment(a Assignment) *permissionv1.ProjectAssignment {
	return &permissionv1.ProjectAssignment{Assignee: assigneeToSDK(a.Assignee), Assignment: permissionv1.ProjectAssignmentType(a.Assignment)}
}

// GetProjectAssignments lists all assignments on a project.
func (c *Client) GetProjectAssignments(ctx context.Context, projectID string) ([]Assignment, error) {
	r, resp, err := c.sdk.PermissionV1().ProjectPermission().GetAssignments(ctx, projectID, nil)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return assignmentsFromSDK(r.Assignments, func(a *permissionv1.ProjectAssignment) Assignment {
		return Assignment{Assignee: assigneeFromSDK(a.Assignee), Assignment: string(a.Assignment)}
	}), nil
}

// UpdateProjectAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateProjectAssignments(ctx context.Context, projectID string, writes, deletes []Assignment) error {
	resp, err := c.sdk.PermissionV1().ProjectPermission().Update(ctx, projectID, &permissionv1.UpdateProjectPermissionsOptions{
		Writes:  assignmentsToSDK(writes, projectAssignment),
		Deletes: assignmentsToSDK(deletes, projectAssignment),
	})
	return wrapErr(resp, err)
}

// GetProjectAccess returns the actions principal may perform on a project.
func (c *Client) GetProjectAccess(ctx context.Context, projectID string, principal Assignee) ([]string, error) {
	user, role := principalOptions(principal)
	r, resp, err := c.sdk.PermissionV1().ProjectPermission().GetAccess(ctx, projectID, &permissionv1.GetProjectAccessOptions{
		PrincipalUser: user,
		PrincipalRole: role,
	})
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	actions := make([]string, len(r.AllowedActions))
	for i, a := range r.AllowedActions {
		actions[i] = string(a)
	}
	return actions, nil
}

// ---------------------------------------------------------------------------
// Warehouse

func warehouseAssignment(a Assignment) *permissionv1.WarehouseAssignment {
	return &permissionv1.WarehouseAssignment{Assignee: assigneeToSDK(a.Assignee), Assignment: permissionv1.WarehouseAssignmentType(a.Assignment)}
}

// GetWarehouseAssignments lists all assignments on a warehouse.
func (c *Client) GetWarehouseAssignments(ctx context.Context, warehouseID string) ([]Assignment, error) {
	r, resp, err := c.sdk.PermissionV1().WarehousePermission().GetAssignments(ctx, warehouseID, nil)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return assignmentsFromSDK(r.Assignments, func(a *permissionv1.WarehouseAssignment) Assignment {
		return Assignment{Assignee: assigneeFromSDK(a.Assignee), Assignment: string(a.Assignment)}
	}), nil
}

// UpdateWarehouseAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateWarehouseAssignments(ctx context.Context, warehouseID string, writes, deletes []Assignment) error {
	resp, err := c.sdk.PermissionV1().WarehousePermission().Update(ctx, warehouseID, &permissionv1.UpdateWarehousePermissionsOptions{
		Writes:  assignmentsToSDK(writes, warehouseAssignment),
		Deletes: assignmentsToSDK(deletes, warehouseAssignment),
	})
	return wrapErr(resp, err)
}

// GetWarehouseAccess returns the actions principal may perform on a warehouse.
func (c *Client) GetWarehouseAccess(ctx context.Context, warehouseID string, principal Assignee) ([]string, error) {
	user, role := principalOptions(principal)
	// GetAccess is deprecated, but its replacements do not fit yet: GetAllowedAuthorizerActions only
	// returns OpenFGA relations, and go-lakekeeper v0.0.23 cannot decode the WarehouseV1().GetAllowedActions response.
	r, resp, err := c.sdk.PermissionV1().WarehousePermission().GetAccess(ctx, warehouseID, &permissionv1.GetWarehouseAccessOptions{ //nolint:staticcheck // see above
		PrincipalUser: user,
		PrincipalRole: role,
	})
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	actions := make([]string, len(r.AllowedActions))
	for i, a := range r.AllowedActions {
		actions[i] = string(a)
	}
	return actions, nil
}

// GetWarehouseManagedAccess reports whether managed access is enabled on a
// warehouse. Servers without an authorization backend (e.g. allowall) answer
// with 404; see IsNotFound.
func (c *Client) GetWarehouseManagedAccess(ctx context.Context, warehouseID string) (bool, error) {
	r, resp, err := c.sdk.PermissionV1().WarehousePermission().GetAuthzProperties(ctx, warehouseID)
	if err != nil {
		return false, wrapErr(resp, err)
	}
	return r.ManagedAccess, nil
}

// SetWarehouseManagedAccess enables or disables managed access on a warehouse.
// Servers without an authorization backend answer with 404; see IsNotFound.
func (c *Client) SetWarehouseManagedAccess(ctx context.Context, warehouseID string, managed bool) error {
	resp, err := c.sdk.PermissionV1().WarehousePermission().SetManagedAccess(ctx, warehouseID, &permissionv1.SetWarehouseManagedAccessOptions{
		ManagedAccess: managed,
	})
	return wrapErr(resp, err)
}

// ---------------------------------------------------------------------------
// Role

func roleAssignment(a Assignment) *permissionv1.RoleAssignment {
	return &permissionv1.RoleAssignment{Assignee: assigneeToSDK(a.Assignee), Assignment: permissionv1.RoleAssignmentType(a.Assignment)}
}

// GetRoleAssignments lists all assignments on a role.
func (c *Client) GetRoleAssignments(ctx context.Context, roleID string) ([]Assignment, error) {
	r, resp, err := c.sdk.PermissionV1().RolePermission().GetAssignments(ctx, roleID, nil)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return assignmentsFromSDK(r.Assignments, func(a *permissionv1.RoleAssignment) Assignment {
		return Assignment{Assignee: assigneeFromSDK(a.Assignee), Assignment: string(a.Assignment)}
	}), nil
}

// UpdateRoleAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateRoleAssignments(ctx context.Context, roleID string, writes, deletes []Assignment) error {
	resp, err := c.sdk.PermissionV1().RolePermission().Update(ctx, roleID, &permissionv1.UpdateRolePermissionsOptions{
		Writes:  assignmentsToSDK(writes, roleAssignment),
		Deletes: assignmentsToSDK(deletes, roleAssignment),
	})
	return wrapErr(resp, err)
}
