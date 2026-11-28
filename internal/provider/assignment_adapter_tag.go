package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// tagScopeAdapter is the scope adapter for governance tag definitions.
type tagScopeAdapter struct{ client *api.Client }

func newTagScopeAdapter(client *api.Client) scopeAdapter {
	return tagScopeAdapter{client: client}
}

func (a tagScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetTagAssignments(ctx, scope[tagIDAttributeName])
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("tag", assignments)
}

func (a tagScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateTagAssignments(ctx, scope[tagIDAttributeName], grantsToAPI(grants), grantsToAPI(revokes))
}

// ScopeExists reports whether the tag definition exists.
func (a tagScopeAdapter) ScopeExists(ctx context.Context, scope assignmentScope) (bool, error) {
	return a.client.TagDefinitionExists(ctx, scope[tagIDAttributeName])
}
