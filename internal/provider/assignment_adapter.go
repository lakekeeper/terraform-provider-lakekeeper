package provider

import (
	"context"
	"fmt"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// assignmentGrant is one direct grant at a scope, as plain strings.
type assignmentGrant struct {
	AssigneeKind assigneeKind // roleAssignee or userAssignee
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

// grantsFromAPI converts the assignments an api.Client method listed at a
// scope into grants. The api layer rejects responses it cannot decode (no
// assignments list, a grant with neither or both of role and user); this
// rejects decoded assignments the module cannot reconcile, so that a
// malformed response is an error rather than a wrong plan.
func grantsFromAPI(scopeNoun string, assignments []api.Assignment) ([]assignmentGrant, error) {
	grants := make([]assignmentGrant, 0, len(assignments))
	for _, a := range assignments {
		grant := assignmentGrant{AssigneeID: a.Assignee.ID, Type: a.Assignment}
		switch a.Assignee.Type {
		case api.AssigneeTypeRole:
			grant.AssigneeKind = roleAssignee
		case api.AssigneeTypeUser:
			grant.AssigneeKind = userAssignee
		default:
			return nil, fmt.Errorf("%s assignments response has a grant with unknown assignee type %q: %+v", scopeNoun, a.Assignee.Type, a)
		}
		if grant.Type == "" {
			return nil, fmt.Errorf("%s assignments response has a grant without a type: %+v", scopeNoun, a)
		}
		if grant.AssigneeID == "" {
			return nil, fmt.Errorf("%s assignments response has a grant without an assignee ID: %+v", scopeNoun, a)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// grantsToAPI converts grants into api assignments for an Update*Assignments call.
func grantsToAPI(grants []assignmentGrant) []api.Assignment {
	out := make([]api.Assignment, len(grants))
	for i, g := range grants {
		out[i] = api.Assignment{Assignee: api.Assignee{Type: string(g.AssigneeKind), ID: g.AssigneeID}, Assignment: g.Type}
	}
	return out
}
