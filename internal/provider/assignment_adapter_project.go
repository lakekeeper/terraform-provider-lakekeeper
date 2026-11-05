package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// projectScopeAdapter is the scope adapter for projects. It wraps the api
// client's project assignment methods and reads the project from
// scope[projectIDAttributeName].
type projectScopeAdapter struct{ client *api.Client }

func newProjectScopeAdapter(client *api.Client) scopeAdapter {
	return projectScopeAdapter{client: client}
}

func (a projectScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetProjectAssignments(ctx, scope[projectIDAttributeName])
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("project", assignments)
}

func (a projectScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateProjectAssignments(ctx, scope[projectIDAttributeName], grantsToAPI(grants), grantsToAPI(revokes))
}
