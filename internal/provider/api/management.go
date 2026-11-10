package api

import (
	"context"
	"time"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
	lakekeeper "github.com/lakekeeper/go-lakekeeper/pkg/client"
)

// formatTime renders a server timestamp as RFC 3339, the format Lakekeeper
// sends.
func formatTime(t time.Time) string { return t.Format(time.RFC3339Nano) }

func formatNullableTime(t managementv1.NullableTime) *string {
	if v := t.Get(); v != nil {
		return new(formatTime(*v))
	}
	return nil
}

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
	info, err := c.sdk.Server.Info(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}
	return &ServerInfo{
		AuthzBackend:                 info.AuthzBackend,
		Bootstrapped:                 info.Bootstrapped,
		DefaultProjectID:             info.GetDefaultProjectId(),
		AWSSystemIdentitiesEnabled:   info.AwsSystemIdentitiesEnabled,
		AzureSystemIdentitiesEnabled: info.AzureSystemIdentitiesEnabled,
		GCPSystemIdentitiesEnabled:   info.GcpSystemIdentitiesEnabled,
		ServerID:                     info.ServerId,
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
	p, err := c.sdk.Projects.Create(ctx, managementv1.NewCreateProjectRequest(name))
	if err != nil {
		return "", wrapErr(err)
	}
	return p.ProjectId, nil
}

// GetProject returns a project. The nil UUID selects the default project.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	p, err := c.sdk.Projects.Get(ctx, lakekeeper.WithProject(id))
	if err != nil {
		return nil, wrapErr(err)
	}
	return &Project{ID: p.ProjectId, Name: p.ProjectName}, nil
}

// RenameProject renames a project.
func (c *Client) RenameProject(ctx context.Context, id, newName string) error {
	return wrapErr(c.sdk.Projects.Rename(ctx, managementv1.NewRenameProjectRequest(newName), lakekeeper.WithProject(id)))
}

// DeleteProject deletes a project.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	return wrapErr(c.sdk.Projects.Delete(ctx, lakekeeper.WithProject(id)))
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
		ID:          r.Id,
		ProjectID:   r.ProjectId,
		Name:        r.Name,
		Description: r.Description.Get(),
		CreatedAt:   formatTime(r.CreatedAt),
		UpdatedAt:   formatNullableTime(r.UpdatedAt),
	}
}

// CreateRole creates a role in a project.
func (c *Client) CreateRole(ctx context.Context, projectID string, req RoleRequest) (*Role, error) {
	body := managementv1.NewCreateRoleRequest(req.Name)
	// An unset description is sent as null, as before.
	body.Description = *managementv1.NewNullableString(req.Description)
	r, err := c.sdk.Roles.Create(ctx, body, lakekeeper.WithProject(projectID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return roleFromSDK(r), nil
}

// GetRole returns a role.
func (c *Client) GetRole(ctx context.Context, projectID, roleID string) (*Role, error) {
	r, err := c.sdk.Roles.Get(ctx, roleID, lakekeeper.WithProject(projectID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return roleFromSDK(r), nil
}

// UpdateRole replaces the writable fields of a role.
func (c *Client) UpdateRole(ctx context.Context, projectID, roleID string, req RoleRequest) (*Role, error) {
	body := managementv1.NewUpdateRoleRequest(req.Name)
	body.Description = *managementv1.NewNullableString(req.Description)
	r, err := c.sdk.Roles.Update(ctx, roleID, body, lakekeeper.WithProject(projectID))
	if err != nil {
		return nil, wrapErr(err)
	}
	return roleFromSDK(r), nil
}

// DeleteRole deletes a role.
func (c *Client) DeleteRole(ctx context.Context, projectID, roleID string) error {
	return wrapErr(c.sdk.Roles.Delete(ctx, roleID, lakekeeper.WithProject(projectID)))
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
		ID:              u.Id,
		Name:            u.Name,
		Email:           u.Email.Get(),
		UserType:        string(u.UserType),
		CreatedAt:       formatTime(u.CreatedAt),
		UpdatedAt:       formatNullableTime(u.UpdatedAt),
		LastUpdatedWith: string(u.LastUpdatedWith),
	}
}

// ProvisionUser creates a user, or updates it if req.UpdateIfExists is set.
func (c *Client) ProvisionUser(ctx context.Context, req ProvisionUserRequest) (*User, error) {
	body := managementv1.NewCreateUserRequest()
	if req.ID != nil {
		body.SetId(*req.ID)
	}
	if req.Name != nil {
		body.SetName(*req.Name)
	}
	if req.Email != nil {
		body.SetEmail(*req.Email)
	}
	if req.UserType != nil {
		body.SetUserType(managementv1.UserType(*req.UserType))
	}
	body.SetUpdateIfExists(req.UpdateIfExists)
	u, err := c.sdk.Users.Create(ctx, body)
	if err != nil {
		return nil, wrapErr(err)
	}
	return userFromSDK(u), nil
}

// GetUser returns a user. The ID may contain "/".
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	u, err := c.sdk.Users.Get(ctx, id) // the SDK path-escapes the ID
	if err != nil {
		return nil, wrapErr(err)
	}
	return userFromSDK(u), nil
}

// Whoami returns the user the client is authenticated as.
func (c *Client) Whoami(ctx context.Context) (*User, error) {
	u, err := c.sdk.Users.Whoami(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}
	return &User{
		ID:              u.Id,
		Name:            u.Name,
		Email:           u.Email.Get(),
		UserType:        string(u.UserType),
		CreatedAt:       formatTime(u.CreatedAt),
		UpdatedAt:       formatNullableTime(u.UpdatedAt),
		LastUpdatedWith: string(u.LastUpdatedWith),
	}, nil
}

// DeleteUser deletes a user. The ID may contain "/".
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return wrapErr(c.sdk.Users.Delete(ctx, id))
}
