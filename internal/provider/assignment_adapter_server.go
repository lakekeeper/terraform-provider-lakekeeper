package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// serverScopeAdapter is the scope adapter for the server. It wraps the api
// client's server assignment methods. The server has no scope attribute, so
// the scope map is always empty and ignored.
type serverScopeAdapter struct{ client *api.Client }

func newServerScopeAdapter(client *api.Client) scopeAdapter {
	return serverScopeAdapter{client: client}
}

func (a serverScopeAdapter) ListGrants(ctx context.Context, _ assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetServerAssignments(ctx)
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("server", assignments)
}

func (a serverScopeAdapter) ApplyGrants(ctx context.Context, _ assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateServerAssignments(ctx, grantsToAPI(grants), grantsToAPI(revokes))
}
