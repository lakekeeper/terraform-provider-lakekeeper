package api

import (
	"context"
	"encoding/json"
	"fmt"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
	lakekeeper "github.com/lakekeeper/go-lakekeeper/pkg/client"
	"github.com/lakekeeper/go-lakekeeper/pkg/permissions"
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

// assignmentsToSDK converts assignments into the SDK's assignment union for
// one entity (managementv1.ServerAssignment, ...). It keeps nil for an empty
// input so the field is omitted on the wire.
func assignmentsToSDK[S any](in []Assignment) ([]S, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]S, len(in))
	for i, a := range in {
		kind := permissions.PrincipalUser
		if a.Assignee.Type == AssigneeTypeRole {
			kind = permissions.PrincipalRole
		}
		s, err := permissions.BuildAssignment[S](a.Assignment, kind, a.Assignee.ID)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// assignmentsFromSDK converts SDK assignment values. A nil input means the
// response had no assignments list, which is an error: an empty list must not
// be mistaken for "no assignments". So is an assignment without exactly one
// of role and user.
func assignmentsFromSDK[S any](entity string, in []S) ([]Assignment, error) {
	if in == nil {
		return nil, fmt.Errorf("%s assignments response has no assignments list", entity)
	}
	out := make([]Assignment, len(in))
	for i, a := range in {
		// Every assignment union has the wire shape {type, user | role}.
		b, err := json.Marshal(a)
		if err != nil {
			return nil, fmt.Errorf("%s assignments response: %w", entity, err)
		}
		var w struct {
			Type string `json:"type"`
			User string `json:"user"`
			Role string `json:"role"`
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, fmt.Errorf("%s assignments response: %w", entity, err)
		}
		switch {
		case w.Role != "" && w.User == "":
			out[i] = Assignment{Assignee: RoleAssignee(w.Role), Assignment: w.Type}
		case w.User != "" && w.Role == "":
			out[i] = Assignment{Assignee: UserAssignee(w.User), Assignment: w.Type}
		default:
			return nil, fmt.Errorf("%s assignments response has an assignment without exactly one role or user: %s", entity, b)
		}
	}
	return out, nil
}

// writesAndDeletes converts both lists of an update.
func writesAndDeletes[S any](writes, deletes []Assignment) (w, d []S, err error) {
	if w, err = assignmentsToSDK[S](writes); err != nil {
		return nil, nil, err
	}
	if d, err = assignmentsToSDK[S](deletes); err != nil {
		return nil, nil, err
	}
	return w, d, nil
}

// principalOptions returns the (user, role) query parameters for an access
// check; the empty one is not sent.
func principalOptions(p Assignee) (user, role string) {
	if p.Type == AssigneeTypeRole {
		return "", p.ID
	}
	return p.ID, ""
}

func actionsToStrings[A ~string](in []A) []string {
	out := make([]string, len(in))
	for i, a := range in {
		out[i] = string(a)
	}
	return out
}

// The Grant and Revoke façades send writes and deletes as two requests.
// Update*Assignments uses the generated client instead, so one call stays one
// request that applies both lists together.

// ---------------------------------------------------------------------------
// Server

// GetServerAssignments lists all server assignments.
func (c *Client) GetServerAssignments(ctx context.Context) ([]Assignment, error) {
	r, err := c.sdk.Permissions.Server.Assignments(ctx, nil)
	if err != nil {
		return nil, wrapErr(err)
	}
	return assignmentsFromSDK("server", r.Assignments)
}

// UpdateServerAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateServerAssignments(ctx context.Context, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.ServerAssignment](writes, deletes)
	if err != nil {
		return err
	}
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateServerAssignments(ctx).
		UpdateServerAssignmentsRequest(managementv1.UpdateServerAssignmentsRequest{Writes: w, Deletes: d}).Execute()
	return wrapGenErr("update server assignments", resp, err)
}

// ---------------------------------------------------------------------------
// Project

// GetProjectAssignments lists all assignments on a project.
func (c *Client) GetProjectAssignments(ctx context.Context, projectID string) ([]Assignment, error) {
	r, err := c.sdk.Permissions.Projects.Assignments(ctx, nil, lakekeeper.WithProject(projectID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return assignmentsFromSDK("project", r.Assignments)
}

// UpdateProjectAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateProjectAssignments(ctx context.Context, projectID string, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.ProjectAssignment](writes, deletes)
	if err != nil {
		return err
	}
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateProjectAssignments(ctx).XProjectId(projectID).
		UpdateProjectAssignmentsRequest(managementv1.UpdateProjectAssignmentsRequest{Writes: w, Deletes: d}).Execute()
	return wrapGenErr("update project assignments", resp, err)
}

// GetProjectAccess returns the actions principal may perform on a project.
func (c *Client) GetProjectAccess(ctx context.Context, projectID string, principal Assignee) ([]string, error) {
	user, role := principalOptions(principal)
	r, err := c.sdk.Permissions.Projects.Access(ctx, &lakekeeper.GetProjectAccessOptions{
		PrincipalUser: user,
		PrincipalRole: role,
	}, lakekeeper.WithProject(projectID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return actionsToStrings(r.AllowedActions), nil
}

// ---------------------------------------------------------------------------
// Warehouse

// GetWarehouseAssignments lists all assignments on a warehouse.
func (c *Client) GetWarehouseAssignments(ctx context.Context, warehouseID string) ([]Assignment, error) {
	r, err := c.sdk.Permissions.Warehouses.Assignments(ctx, warehouseID, nil)
	if err != nil {
		return nil, wrapErr(err)
	}
	return assignmentsFromSDK("warehouse", r.Assignments)
}

// UpdateWarehouseAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateWarehouseAssignments(ctx context.Context, warehouseID string, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.WarehouseAssignment](writes, deletes)
	if err != nil {
		return err
	}
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateWarehouseAssignmentsById(ctx, warehouseID).
		UpdateWarehouseAssignmentsRequest(managementv1.UpdateWarehouseAssignmentsRequest{Writes: w, Deletes: d}).Execute()
	return wrapGenErr("update warehouse assignments", resp, err)
}

// GetWarehouseAccess returns the actions principal may perform on a warehouse.
func (c *Client) GetWarehouseAccess(ctx context.Context, warehouseID string, principal Assignee) ([]string, error) {
	user, role := principalOptions(principal)
	r, err := c.sdk.Permissions.Warehouses.Access(ctx, warehouseID, &lakekeeper.GetWarehouseAccessByIDOptions{
		PrincipalUser: user,
		PrincipalRole: role,
	})
	if err != nil {
		return nil, wrapErr(err)
	}
	return actionsToStrings(r.AllowedActions), nil
}

// GetWarehouseManagedAccess reports whether managed access is enabled on a
// warehouse. Servers without an authorization backend (e.g. allowall) answer
// with 404; see IsNotFound.
func (c *Client) GetWarehouseManagedAccess(ctx context.Context, warehouseID string) (bool, error) {
	r, resp, err := c.sdk.PermissionsOpenfgaAPI.GetWarehouseById(ctx, warehouseID).Execute()
	if err != nil {
		return false, wrapGenErr("get warehouse authz properties", resp, err)
	}
	return r.ManagedAccess, nil
}

// SetWarehouseManagedAccess enables or disables managed access on a warehouse.
// Servers without an authorization backend answer with 404; see IsNotFound.
func (c *Client) SetWarehouseManagedAccess(ctx context.Context, warehouseID string, managed bool) error {
	resp, err := c.sdk.PermissionsOpenfgaAPI.SetWarehouseManagedAccess(ctx, warehouseID).
		SetManagedAccessRequest(*managementv1.NewSetManagedAccessRequest(managed)).Execute()
	return wrapGenErr("set warehouse managed access", resp, err)
}

// ---------------------------------------------------------------------------
// Role

// GetRoleAssignments lists all assignments on a role.
func (c *Client) GetRoleAssignments(ctx context.Context, roleID string) ([]Assignment, error) {
	r, err := c.sdk.Permissions.Roles.Assignments(ctx, roleID, nil)
	if err != nil {
		return nil, wrapErr(err)
	}
	return assignmentsFromSDK("role", r.Assignments)
}

// UpdateRoleAssignments adds writes and removes deletes in one request.
func (c *Client) UpdateRoleAssignments(ctx context.Context, roleID string, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.RoleAssignment](writes, deletes)
	if err != nil {
		return err
	}
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateRoleAssignmentsById(ctx, roleID).
		UpdateRoleAssignmentsRequest(managementv1.UpdateRoleAssignmentsRequest{Writes: w, Deletes: d}).Execute()
	return wrapGenErr("update role assignments", resp, err)
}
