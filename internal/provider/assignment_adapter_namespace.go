package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	lakekeeper "github.com/baptistegh/go-lakekeeper/pkg/client"
)

// namespaceScopeAdapter is the scope adapter for namespaces. go-lakekeeper has
// no namespace permission client, so it sends raw requests through the
// configured client. Namespace permission endpoints need the project in the
// x-project-id header; the adapter reads it from scope["project_id"] and the
// namespace from scope["namespace_id"].
type namespaceScopeAdapter struct{ client *lakekeeper.Client }

func newNamespaceScopeAdapter(client *lakekeeper.Client) scopeAdapter {
	return namespaceScopeAdapter{client: client}
}

// namespaceGrantJSON is one grant as Lakekeeper's namespace permission API encodes it.
type namespaceGrantJSON struct {
	Type string `json:"type"`
	Role string `json:"role,omitempty"`
	User string `json:"user,omitempty"`
}

func (a namespaceScopeAdapter) ListGrants(ctx context.Context, scope assignmentScope) ([]assignmentGrant, error) {
	var resp struct {
		Assignments *[]namespaceGrantJSON `json:"assignments"`
	}
	if err := a.do(ctx, http.MethodGet, scope, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Assignments == nil {
		return nil, errors.New("namespace assignments response has no assignments list")
	}
	grants := make([]assignmentGrant, 0, len(*resp.Assignments))
	for _, g := range *resp.Assignments {
		grant := assignmentGrant{Type: g.Type}
		switch {
		case g.Type == "":
			return nil, fmt.Errorf("namespace assignments response has a grant without a type: %+v", g)
		case g.Role != "" && g.User == "":
			grant.AssigneeKind, grant.AssigneeID = roleAssignee, g.Role
		case g.User != "" && g.Role == "":
			grant.AssigneeKind, grant.AssigneeID = userAssignee, g.User
		default:
			return nil, fmt.Errorf("namespace assignments response has a grant without exactly one role or user: %+v", g)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// ApplyGrants sends revokes and grants in one POST, which Lakekeeper applies atomically.
func (a namespaceScopeAdapter) ApplyGrants(ctx context.Context, scope assignmentScope, revokes, grants []assignmentGrant) error {
	body := map[string][]namespaceGrantJSON{
		"deletes": namespaceGrants(revokes),
		"writes":  namespaceGrants(grants),
	}
	return a.do(ctx, http.MethodPost, scope, body, nil)
}

func (a namespaceScopeAdapter) do(ctx context.Context, method string, scope assignmentScope, body, result any) error {
	return namespaceRequest(ctx, a.client, scope["project_id"], url.PathEscape(scope["namespace_id"]), "/assignments", method, body, result)
}

func namespaceGrants(grants []assignmentGrant) []namespaceGrantJSON {
	out := make([]namespaceGrantJSON, 0, len(grants))
	for _, g := range grants {
		j := namespaceGrantJSON{Type: g.Type}
		if g.AssigneeKind == roleAssignee {
			j.Role = g.AssigneeID
		} else {
			j.User = g.AssigneeID
		}
		out = append(out, j)
	}
	return out
}
