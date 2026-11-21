package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Generic tables live in Lakekeeper's Generic Table API
// (/lakekeeper/v1/{warehouse_id}/...), which the SDK does not wrap. These
// methods send plain requests through the SDK's authenticated HTTP client.

// CreateGenericTable creates a generic table and returns its ID. The create
// response carries no ID, so the table is looked up in the namespace's list
// afterwards.
func (c *Client) CreateGenericTable(ctx context.Context, projectID, warehouseID string, namespace []string, name, format string) (string, error) {
	path := genericTablesPath(warehouseID, namespace)
	body := struct {
		Name   string `json:"name"`
		Format string `json:"format"`
	}{Name: name, Format: format}

	if err := c.do(ctx, rawCall{
		op:        "create generic table",
		method:    http.MethodPost,
		path:      path,
		projectID: projectID,
		body:      body,
	}); err != nil {
		return "", err
	}

	var list struct {
		Identifiers []struct {
			Namespace []string `json:"namespace"`
			Name      string   `json:"name"`
			ID        string   `json:"id"`
		} `json:"identifiers"`
	}
	if err := c.do(ctx, rawCall{
		op:        "list generic tables",
		method:    http.MethodGet,
		path:      path,
		projectID: projectID,
		out:       &list,
	}); err != nil {
		return "", err
	}

	for _, id := range list.Identifiers {
		if id.Name != name || !slices.Equal(id.Namespace, namespace) {
			continue
		}
		if id.ID == "" {
			return "", fmt.Errorf("create generic table: catalog returned no ID for %q", name)
		}
		return id.ID, nil
	}
	return "", fmt.Errorf("create generic table: %q is missing from the namespace listing after creation", name)
}

// DeleteGenericTable drops a generic table. With purge, its files are deleted
// too; with force, it is dropped even while protected.
func (c *Client) DeleteGenericTable(ctx context.Context, projectID, warehouseID string, namespace []string, name string, purge, force bool) error {
	query := url.Values{}
	query.Set("purgeRequested", strconv.FormatBool(purge))
	query.Set("force", strconv.FormatBool(force))
	return c.do(ctx, rawCall{
		op:        "drop generic table",
		method:    http.MethodDelete,
		path:      genericTablesPath(warehouseID, namespace) + "/" + name,
		query:     query,
		projectID: projectID,
	})
}

func genericTablesPath(warehouseID string, namespace []string) string {
	return "/lakekeeper/v1/" + warehouseID + "/namespaces/" + strings.Join(namespace, "\x1f") + "/generic-tables"
}
