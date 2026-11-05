package provider

import (
	"context"
	"errors"
	"fmt"

	permissionv1 "github.com/baptistegh/go-lakekeeper/pkg/apis/management/v1/permission"
	lakekeeper "github.com/baptistegh/go-lakekeeper/pkg/client"
)

// projectScopeAdapter is the scope adapter for projects. It wraps go-lakekeeper's
// project permission client and reads the project from scope[projectIDAttributeName].
type projectScopeAdapter struct{ client *lakekeeper.Client }

func newProjectScopeAdapter(client *lakekeeper.Client) scopeAdapter {
	return projectScopeAdapter{client: client}
}

func (a projectScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	resp, _, err := a.client.PermissionV1().ProjectPermission().GetAssignments(ctx, scope[projectIDAttributeName], nil)
	if err != nil {
		return nil, err
	}
	if resp.Assignments == nil {
		return nil, errors.New("project assignments response has no assignments list")
	}
	grants := make([]assignmentGrant, 0, len(resp.Assignments))
	for _, g := range resp.Assignments {
		grant := assignmentGrant{AssigneeID: g.Assignee.Value, Type: string(g.Assignment)}
		switch g.Assignee.Type {
		case permissionv1.RoleType:
			grant.AssigneeKind = roleAssignee
		case permissionv1.UserType:
			grant.AssigneeKind = userAssignee
		default:
			return nil, fmt.Errorf("project assignments response has a grant with unknown assignee type %q: %+v", g.Assignee.Type, g)
		}
		if grant.Type == "" {
			return nil, fmt.Errorf("project assignments response has a grant without a type: %+v", g)
		}
		if grant.AssigneeID == "" {
			return nil, fmt.Errorf("project assignments response has a grant without an assignee ID: %+v", g)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func (a projectScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	_, err := a.client.PermissionV1().ProjectPermission().Update(ctx, scope[projectIDAttributeName], &permissionv1.UpdateProjectPermissionsOptions{
		Deletes: projectAssignments(revokes),
		Writes:  projectAssignments(grants),
	})
	return err
}

func projectAssignments(grants []assignmentGrant) []*permissionv1.ProjectAssignment {
	out := make([]*permissionv1.ProjectAssignment, 0, len(grants))
	for _, g := range grants {
		out = append(out, &permissionv1.ProjectAssignment{
			Assignee:   permissionv1.UserOrRole{Type: permissionv1.UserOrRoleType(g.AssigneeKind), Value: g.AssigneeID},
			Assignment: permissionv1.ProjectAssignmentType(g.Type),
		})
	}
	return out
}
