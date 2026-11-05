package api

import (
	"context"

	managementv1 "github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1"
)

// ---------------------------------------------------------------------------
// Server

// ServerInfo describes the Lakekeeper server.
type ServerInfo struct {
	AuthzBackend                 string
	Bootstrapped                 bool
	DefaultProjectID             string
	AWSSystemIdentitiesEnabled   bool
	AzureSystemIdentitiesEnabled bool
	GCPSystemIdentitiesEnabled   bool
	ServerID                     string
	Version                      string
	Queues                       []string
}

// GetServerInfo returns information about the server.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	info, resp, err := c.sdk.ServerV1().Info(ctx)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return &ServerInfo{
		AuthzBackend:                 info.AuthzBackend,
		Bootstrapped:                 info.Bootstrapped,
		DefaultProjectID:             info.DefaultProjectID,
		AWSSystemIdentitiesEnabled:   info.AWSSystemIdentitiesEnabled,
		AzureSystemIdentitiesEnabled: info.AzureSystemIdentitiesEnabled,
		GCPSystemIdentitiesEnabled:   info.GCPSystemIdentitiesEnabled,
		ServerID:                     info.ServerID,
		Version:                      info.Version,
		Queues:                       info.Queues,
	}, nil
}

// ---------------------------------------------------------------------------
// Projects

// Project is a Lakekeeper project.
type Project struct {
	ID   string
	Name string
}

// CreateProject creates a project and returns its ID.
func (c *Client) CreateProject(ctx context.Context, name string) (string, error) {
	p, resp, err := c.sdk.ProjectV1().Create(ctx, &managementv1.CreateProjectOptions{Name: name})
	if err != nil {
		return "", wrapErr(resp, err)
	}
	return p.ID, nil
}

// GetProject returns a project. The nil UUID selects the default project.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	p, resp, err := c.sdk.ProjectV1().Get(ctx, id)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return &Project{ID: p.ID, Name: p.Name}, nil
}

// RenameProject renames a project.
func (c *Client) RenameProject(ctx context.Context, id, newName string) error {
	resp, err := c.sdk.ProjectV1().Rename(ctx, id, &managementv1.RenameProjectOptions{NewName: newName})
	return wrapErr(resp, err)
}

// DeleteProject deletes a project.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	resp, err := c.sdk.ProjectV1().Delete(ctx, id)
	return wrapErr(resp, err)
}

// ---------------------------------------------------------------------------
// Roles

// Role is a Lakekeeper role. Roles belong to a project.
type Role struct {
	ID          string
	ProjectID   string
	Name        string
	Description *string
	CreatedAt   string
	UpdatedAt   *string
}

// RoleRequest holds the writable fields of a role.
type RoleRequest struct {
	Name        string
	Description *string
}

func roleFromSDK(r *managementv1.Role) *Role {
	return &Role{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// CreateRole creates a role in a project.
func (c *Client) CreateRole(ctx context.Context, projectID string, req RoleRequest) (*Role, error) {
	r, resp, err := c.sdk.RoleV1(projectID).Create(ctx, &managementv1.CreateRoleOptions{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return roleFromSDK(r), nil
}

// GetRole returns a role.
func (c *Client) GetRole(ctx context.Context, projectID, roleID string) (*Role, error) {
	r, resp, err := c.sdk.RoleV1(projectID).Get(ctx, roleID)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return roleFromSDK(r), nil
}

// UpdateRole replaces the writable fields of a role.
func (c *Client) UpdateRole(ctx context.Context, projectID, roleID string, req RoleRequest) (*Role, error) {
	r, resp, err := c.sdk.RoleV1(projectID).Update(ctx, roleID, &managementv1.UpdateRoleOptions{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return roleFromSDK(r), nil
}

// DeleteRole deletes a role.
func (c *Client) DeleteRole(ctx context.Context, projectID, roleID string) error {
	resp, err := c.sdk.RoleV1(projectID).Delete(ctx, roleID)
	return wrapErr(resp, err)
}

// ---------------------------------------------------------------------------
// Users

// User is a Lakekeeper user.
type User struct {
	ID              string
	Name            string
	Email           *string
	UserType        string
	CreatedAt       string
	UpdatedAt       *string
	LastUpdatedWith string
}

// ProvisionUserRequest describes a user to create or update.
type ProvisionUserRequest struct {
	ID       *string
	Name     *string
	Email    *string
	UserType *string // UserType* constant; nil lets the server decide.
	// UpdateIfExists updates an existing user instead of failing.
	UpdateIfExists bool
}

func userFromSDK(u *managementv1.User) *User {
	return &User{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		UserType:        string(u.UserType),
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
		LastUpdatedWith: u.LastUpdatedWith,
	}
}

// ProvisionUser creates a user, or updates it if req.UpdateIfExists is set.
func (c *Client) ProvisionUser(ctx context.Context, req ProvisionUserRequest) (*User, error) {
	opts := managementv1.ProvisionUserOptions{
		ID:             req.ID,
		Name:           req.Name,
		Email:          req.Email,
		UpdateIfExists: new(req.UpdateIfExists),
	}
	if req.UserType != nil {
		opts.UserType = new(managementv1.UserType(*req.UserType))
	}
	u, resp, err := c.sdk.UserV1().Provision(ctx, &opts)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return userFromSDK(u), nil
}

// GetUser returns a user.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	u, resp, err := c.sdk.UserV1().Get(ctx, id)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return userFromSDK(u), nil
}

// Whoami returns the user the client is authenticated as.
func (c *Client) Whoami(ctx context.Context) (*User, error) {
	u, resp, err := c.sdk.UserV1().Whoami(ctx)
	if err != nil {
		return nil, wrapErr(resp, err)
	}
	return userFromSDK(u), nil
}

// DeleteUser deletes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	resp, err := c.sdk.UserV1().Delete(ctx, id)
	return wrapErr(resp, err)
}
