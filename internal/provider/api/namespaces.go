package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/baptistegh/go-lakekeeper/pkg/core"
)

// The SDK has no typed methods for namespace permissions or for loading a
// namespace by warehouse ID, so these calls are raw HTTP requests made with
// the SDK's request helpers.

// do sends a management API request scoped to a project.
func (c *Client) do(ctx context.Context, project, method, path string, body, result any) error {
	req, err := c.sdk.NewRequest(ctx, method, path, body, []core.RequestOptionFunc{core.WithHeader("x-project-id", project)})
	if err != nil {
		return err
	}
	resp, apiErr := c.sdk.Do(req, result)
	if apiErr != nil { // Do returns a typed *core.APIError; compare before converting to error.
		return wrapErr(resp, apiErr)
	}
	return nil
}

// namespaceAssignment is the wire format of a namespace assignment.
type namespaceAssignment struct {
	Type string `json:"type"`
	Role string `json:"role,omitempty"`
	User string `json:"user,omitempty"`
}

func namespaceAssignmentsToWire(in []Assignment) []namespaceAssignment {
	if len(in) == 0 {
		return nil
	}
	out := make([]namespaceAssignment, len(in))
	for i, a := range in {
		out[i] = namespaceAssignment{Type: a.Assignment}
		if a.Assignee.Type == AssigneeTypeUser {
			out[i].User = a.Assignee.ID
		} else {
			out[i].Role = a.Assignee.ID
		}
	}
	return out
}

// GetNamespaceAssignments lists all direct assignments on a namespace.
func (c *Client) GetNamespaceAssignments(ctx context.Context, projectID, namespaceID string) ([]Assignment, error) {
	var result struct {
		Assignments *[]namespaceAssignment `json:"assignments"`
	}
	if err := c.do(ctx, projectID, http.MethodGet, "/permissions/namespace/"+namespaceID+"/assignments", nil, &result); err != nil {
		return nil, err
	}
	if result.Assignments == nil {
		return nil, errors.New("namespace assignments response has no assignments list")
	}
	out := make([]Assignment, 0, len(*result.Assignments))
	for _, a := range *result.Assignments {
		var assignee Assignee
		switch {
		case a.Role != "" && a.User == "":
			assignee = RoleAssignee(a.Role)
		case a.User != "" && a.Role == "":
			assignee = UserAssignee(a.User)
		default:
			return nil, fmt.Errorf("namespace assignments response has an assignment without exactly one role or user: %+v", a)
		}
		out = append(out, Assignment{Assignee: assignee, Assignment: a.Type})
	}
	return out, nil
}

// UpdateNamespaceAssignments adds writes and removes deletes in one request.
// Empty lists are omitted from the request.
func (c *Client) UpdateNamespaceAssignments(ctx context.Context, projectID, namespaceID string, writes, deletes []Assignment) error {
	body := struct {
		Writes  []namespaceAssignment `json:"writes,omitempty"`
		Deletes []namespaceAssignment `json:"deletes,omitempty"`
	}{
		Writes:  namespaceAssignmentsToWire(writes),
		Deletes: namespaceAssignmentsToWire(deletes),
	}
	return c.do(ctx, projectID, http.MethodPost, "/permissions/namespace/"+namespaceID+"/assignments", body, nil)
}

// GetNamespaceManagedAccess returns the namespace's managed-access flag, or
// nil if the response does not contain it.
func (c *Client) GetNamespaceManagedAccess(ctx context.Context, projectID, namespaceID string) (*bool, error) {
	var props struct {
		Managed *bool `json:"managed-access"`
	}
	if err := c.do(ctx, projectID, http.MethodGet, "/permissions/namespace/"+namespaceID, nil, &props); err != nil {
		return nil, err
	}
	return props.Managed, nil
}

// SetNamespaceManagedAccess enables or disables managed access on a namespace.
func (c *Client) SetNamespaceManagedAccess(ctx context.Context, projectID, namespaceID string, managed bool) error {
	return c.do(ctx, projectID, http.MethodPost, "/permissions/namespace/"+namespaceID+"/managed-access", map[string]bool{"managed-access": managed}, nil)
}

// Namespace is a catalog namespace as returned by the Iceberg REST API.
type Namespace struct {
	Path       []string
	Properties map[string]string
}

// LoadNamespace loads a namespace from the Iceberg REST catalog, addressing
// the warehouse by its UUID rather than its name. path holds the namespace
// levels, e.g. ["sales", "bronze"].
func (c *Client) LoadNamespace(ctx context.Context, projectID, warehouseID string, path []string) (*Namespace, error) {
	req, err := c.sdk.NewRequest(ctx, http.MethodGet, "/", nil, []core.RequestOptionFunc{core.WithHeader("x-project-id", projectID)})
	if err != nil {
		return nil, err
	}
	req.URL.Path = strings.TrimSuffix(c.sdk.BaseURL().Path, "/management/v1") + "/catalog/v1/" + warehouseID + "/namespaces/" + strings.Join(path, "\x1f")
	req.URL.RawPath = ""
	var result struct {
		Namespace  []string          `json:"namespace"`
		Properties map[string]string `json:"properties"`
	}
	if resp, apiErr := c.sdk.Do(req, &result); apiErr != nil {
		return nil, wrapErr(resp, apiErr)
	}
	return &Namespace{Path: result.Namespace, Properties: result.Properties}, nil
}
