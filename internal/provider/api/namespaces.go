package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	managementv1 "github.com/lakekeeper/go-lakekeeper/pkg/apis/management/v1"
	lakekeeper "github.com/lakekeeper/go-lakekeeper/pkg/client"
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
//
// As the Iceberg REST spec requires, each level is percent-encoded on its own
// and the levels are joined with the encoded unit separator %1F, so a level
// containing a reserved character such as "/" stays one path segment.
func (c *Client) LoadNamespace(ctx context.Context, projectID, warehouseID string, path []string) (*Namespace, error) {
	const op = "load namespace"
	encodedPath, err := EncodeNamespacePath(path)
	if err != nil {
		return nil, err
	}
	var result struct {
		Namespace  []string          `json:"namespace"`
		Properties map[string]string `json:"properties"`
	}
	if err := c.catalogGet(ctx, op, projectID, warehouseID, "/namespaces/"+encodedPath, nil, &result); err != nil {
		return nil, err
	}
	return &Namespace{Path: result.Namespace, Properties: result.Properties}, nil
}

// NamespaceListItem is a namespace as returned by the catalog list endpoint.
type NamespaceListItem struct {
	ID    string
	Parts []string
}

// ListNamespaces lists the direct child namespaces of a warehouse or namespace.
// parentParts holds the parent namespace levels (e.g. ["sales", "bronze"]);
// when empty, it lists the warehouse's top-level namespaces. It follows every
// catalog page and returns namespace UUIDs.
func (c *Client) ListNamespaces(ctx context.Context, warehouseID string, parentParts []string) ([]NamespaceListItem, error) {
	const op = "list namespaces"
	q := url.Values{}
	if len(parentParts) > 0 {
		if _, err := EncodeNamespacePath(parentParts); err != nil {
			return nil, err
		}
		// A query value: url.Values encodes the unit separator as %1F.
		q.Set("parent", strings.Join(parentParts, "\x1f"))
	}
	q.Set("returnUuids", "true")
	namespaces := []NamespaceListItem{}
	seen := map[string]bool{}
	for {
		var page struct {
			Namespaces     [][]string `json:"namespaces"`
			NamespaceUUIDs []string   `json:"namespace-uuids"`
			NextPageToken  *string    `json:"next-page-token"`
		}
		if err := c.catalogGet(ctx, op, "", warehouseID, "/namespaces", q, &page); err != nil {
			return nil, err
		}
		if len(page.NamespaceUUIDs) != len(page.Namespaces) {
			return nil, &Error{err: fmt.Errorf("%s: catalog returned %d namespace UUIDs for %d namespaces", op, len(page.NamespaceUUIDs), len(page.Namespaces))}
		}
		for i, parts := range page.Namespaces {
			namespaces = append(namespaces, NamespaceListItem{
				ID:    page.NamespaceUUIDs[i],
				Parts: parts,
			})
		}
		if page.NextPageToken == nil || *page.NextPageToken == "" {
			break
		}
		if seen[*page.NextPageToken] {
			return nil, &Error{err: fmt.Errorf("%s: repeated page token", op)}
		}
		seen[*page.NextPageToken] = true
		q.Set("pageToken", *page.NextPageToken)
	}
	return namespaces, nil
}

// catalogGet sends a GET request to the Iceberg REST catalog API and decodes
// the JSON response into out. It derives the catalog base URL from the client's
// management base URL, addresses the warehouse by UUID, and sets the x-project-id
// header when projectID is not empty.
func (c *Client) catalogGet(ctx context.Context, op, projectID, warehouseID, escapedPath string, query url.Values, out any) error {
	u := c.sdk.BaseURL()
	u.RawPath = strings.TrimSuffix(u.EscapedPath(), "/") + "/catalog/v1/" + url.PathEscape(warehouseID) + escapedPath
	unescaped, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return fmt.Errorf("%s: build path: %w", op, err)
	}
	u.Path = unescaped
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return &Error{err: fmt.Errorf("%s: %w", op, err)}
	}
	req.Header.Set("Accept", "application/json")
	if projectID != "" {
		req.Header.Set("x-project-id", projectID)
	}
	resp, err := c.sdk.GetConfig().HTTPClient.Do(req)
	if err != nil {
		return &Error{err: fmt.Errorf("%s: %w", op, err)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %w", op, err)}
	}
	if resp.StatusCode >= 300 {
		return &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: decode response: %w", op, err)}
	}
	return nil
}

// EncodeNamespacePath checks namespace levels and returns them as one path
// segment of the Iceberg REST API: each level percent-encoded on its own, and
// the levels joined with the encoded unit separator %1F. Each level must be
// non-empty and must not contain the unit separator itself.
func EncodeNamespacePath(parts []string) (string, error) {
	levels := make([]string, len(parts))
	for i, part := range parts {
		if part == "" {
			return "", fmt.Errorf("namespace levels must be non-empty")
		}
		if strings.Contains(part, "\x1f") {
			return "", fmt.Errorf("namespace levels cannot contain the unit separator \\x1f")
		}
		levels[i] = url.PathEscape(part)
	}
	return strings.Join(levels, "%1F"), nil
}

// NamespaceExists reports whether a namespace exists, even one the caller
// cannot see. A namespace load answers 404 both when the namespace is missing
// and when the caller may not see it; a batch check with error-on-not-found
// answers 404 only when it is missing. namespaceID selects the namespace when
// set, otherwise path does. A warehouse missing from the project's list is an
// error: it was deleted after the namespace load or is hidden from the caller,
// so the namespace's state is unknown.
func (c *Client) NamespaceExists(ctx context.Context, projectID, warehouseName, namespaceID string, path []string) (bool, error) {
	warehouses, err := c.sdk.Warehouses.List(ctx, nil, lakekeeper.WithProject(projectID))
	if err != nil {
		return false, wrapErr(err)
	}
	warehouseID := ""
	for _, w := range warehouses.GetWarehouses() {
		if w.GetName() == warehouseName {
			warehouseID = w.GetId()
		}
	}
	if warehouseID == "" {
		return false, &Error{err: fmt.Errorf("check namespace: warehouse %q is not in the warehouse list of project %s", warehouseName, projectID)}
	}
	action := managementv1.LakekeeperNamespaceActionGetMetadataAsLakekeeperNamespaceAction(managementv1.NewLakekeeperNamespaceActionGetMetadata("get_metadata"))
	check := managementv1.NewCatalogActionCheckOperationNamespace(*managementv1.NewCatalogActionCheckOperationNamespaceNamespace(namespaceID, warehouseID, path, action))
	req := managementv1.NewCatalogActionsBatchCheckRequest([]managementv1.CatalogActionCheckItem{
		*managementv1.NewCatalogActionCheckItem(managementv1.CatalogActionCheckOperationNamespaceAsCatalogActionCheckOperation(check)),
	})
	req.SetErrorOnNotFound(true)
	_, resp, err := c.sdk.AuthorizationAPI.BatchCheckActions(ctx).CatalogActionsBatchCheckRequest(*req).XProjectId(projectID).Execute()
	if err != nil {
		err = wrapGenErr("check namespace", resp, err)
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
