package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
)

// Namespace permissions are addressed by namespace ID alone, so projectID is
// not sent; it stays in the signatures for the callers.

// GetNamespaceAssignments lists all direct assignments on a namespace.
func (c *Client) GetNamespaceAssignments(ctx context.Context, projectID, namespaceID string) ([]Assignment, error) {
	r, resp, err := c.sdk.PermissionsOpenfgaAPI.GetNamespaceAssignmentsById(ctx, namespaceID).Execute()
	if err != nil {
		return nil, wrapGenErr("get namespace assignments", resp, err)
	}
	return assignmentsFromSDK("namespace", r.Assignments)
}

// UpdateNamespaceAssignments adds writes and removes deletes in one request.
// Empty lists are omitted from the request.
func (c *Client) UpdateNamespaceAssignments(ctx context.Context, projectID, namespaceID string, writes, deletes []Assignment) error {
	w, d, err := writesAndDeletes[managementv1.NamespaceAssignment](writes, deletes)
	if err != nil {
		return err
	}
	resp, err := c.sdk.PermissionsOpenfgaAPI.UpdateNamespaceAssignmentsById(ctx, namespaceID).
		UpdateNamespaceAssignmentsRequest(managementv1.UpdateNamespaceAssignmentsRequest{Writes: w, Deletes: d}).Execute()
	return wrapGenErr("update namespace assignments", resp, err)
}

// GetNamespaceManagedAccess returns the namespace's managed-access flag. It
// never returns nil without an error: a response without the flag fails to
// decode.
func (c *Client) GetNamespaceManagedAccess(ctx context.Context, projectID, namespaceID string) (*bool, error) {
	r, resp, err := c.sdk.PermissionsOpenfgaAPI.GetNamespaceById(ctx, namespaceID).Execute()
	if err != nil {
		return nil, wrapGenErr("get namespace authz properties", resp, err)
	}
	return new(r.ManagedAccess), nil
}

// SetNamespaceManagedAccess enables or disables managed access on a namespace.
func (c *Client) SetNamespaceManagedAccess(ctx context.Context, projectID, namespaceID string, managed bool) error {
	resp, err := c.sdk.PermissionsOpenfgaAPI.SetNamespaceManagedAccess(ctx, namespaceID).
		SetManagedAccessRequest(*managementv1.NewSetManagedAccessRequest(managed)).Execute()
	return wrapGenErr("set namespace managed access", resp, err)
}

// Namespace is a catalog namespace as returned by the Iceberg REST API.
type Namespace struct {
	Path       []string
	Properties map[string]string
}

// LoadNamespace loads a namespace from the Iceberg REST catalog, addressing
// the warehouse by its UUID rather than its name. path holds the namespace
// levels, e.g. ["sales", "bronze"].
//
// The SDK's catalog client addresses warehouses by name, so this is a plain
// request sent through the SDK's authenticated HTTP client.
func (c *Client) LoadNamespace(ctx context.Context, projectID, warehouseID string, path []string) (*Namespace, error) {
	const op = "load namespace"
	u := c.sdk.BaseURL()
	u.Path = strings.TrimSuffix(u.Path, "/") + "/catalog/v1/" + warehouseID + "/namespaces/" + strings.Join(path, "\x1f")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-project-id", projectID)
	resp, err := c.sdk.GetConfig().HTTPClient.Do(req)
	if err != nil {
		return nil, &Error{err: fmt.Errorf("%s: %w", op, err)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %w", op, err)}
	}
	if resp.StatusCode >= 300 {
		return nil, &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))}
	}
	var result struct {
		Namespace  []string          `json:"namespace"`
		Properties map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", op, err)
	}
	return &Namespace{Path: result.Namespace, Properties: result.Properties}, nil
}
