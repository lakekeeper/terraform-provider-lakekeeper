package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// roleScopeAdapter is the scope adapter for roles. It wraps the api client's
// role assignment methods and reads the role from scope[roleIDAttributeName].
type roleScopeAdapter struct{ client *api.Client }

func newRoleScopeAdapter(client *api.Client) scopeAdapter {
	return roleScopeAdapter{client: client}
}

func (a roleScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetRoleAssignments(ctx, scope[roleIDAttributeName])
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("role", assignments)
}

func (a roleScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateRoleAssignments(ctx, scope[roleIDAttributeName], grantsToAPI(grants), grantsToAPI(revokes))
}
