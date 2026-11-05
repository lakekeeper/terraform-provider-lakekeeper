package provider

import (
	"context"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// namespaceScopeAdapter is the scope adapter for namespaces. It wraps the api
// client's namespace assignment methods, which address the namespace within a
// project; it reads them from scope[projectIDAttributeName] and
// scope[namespaceIDAttributeName].
type namespaceScopeAdapter struct{ client *api.Client }

func newNamespaceScopeAdapter(client *api.Client) scopeAdapter {
	return namespaceScopeAdapter{client: client}
}

func (a namespaceScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	assignments, err := a.client.GetNamespaceAssignments(ctx, scope[projectIDAttributeName], scope[namespaceIDAttributeName])
	if err != nil {
		return nil, err
	}
	return grantsFromAPI("namespace", assignments)
}

// ApplyGrants sends revokes and grants in one POST, which Lakekeeper applies
// atomically. An empty side is omitted from the request body.
func (a namespaceScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	return a.client.UpdateNamespaceAssignments(ctx, scope[projectIDAttributeName], scope[namespaceIDAttributeName], grantsToAPI(grants), grantsToAPI(revokes))
}
