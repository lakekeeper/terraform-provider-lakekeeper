package provider

import "context"

// assignmentGrant is one direct grant at a scope, as plain strings.
type assignmentGrant struct {
	AssigneeKind assigneeKind // "role" or "user"
	AssigneeID   string
	Type         string // grant type, e.g. "select"
}

// assignmentScope identifies a scope by the values of the resource's identity
// attributes other than the assignee, keyed by attribute name (e.g.
// {"project_id": "p"}). It is empty for the server.
type assignmentScope map[string]string

// scopeAdapter is the seam between the assignment module and Lakekeeper.
// One adapter exists per scope (server, project, warehouse, role, namespace).
type scopeAdapter interface {
	// ListGrants returns every direct grant at the scope, for all assignees.
	// A malformed response must be an error, never an empty list.
	ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error)
	// ApplyGrants sends revokes and grants in one request, which Lakekeeper
	// applies atomically. The module never calls it with both lists empty.
	ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error
}
