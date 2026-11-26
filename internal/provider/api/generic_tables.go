package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Generic tables live in Lakekeeper's Generic Table API
// (/lakekeeper/v1/{warehouse-id-or-name}/...), which the SDK does not wrap.
// These methods send plain requests through the SDK's authenticated HTTP
// client.

// GenericTable is a generic (non-Iceberg) table as returned by the Generic
// Table API. ID is only present in listings and after CreateGenericTable
// resolves it, because load and create responses carry no ID.
type GenericTable struct {
	ID           string
	Name         string
	Namespace    []string
	Format       string
	BaseLocation string
	Doc          *string
	Properties   map[string]string
	Schema       json.RawMessage
	Protected    bool
}

// GenericTableRequest is the create body for a generic table.
type GenericTableRequest struct {
	Name         string
	Format       string
	BaseLocation *string
	Doc          *string
	Properties   map[string]string
	Schema       json.RawMessage
}

type genericTableData struct {
	Name         string            `json:"name"`
	Format       string            `json:"format"`
	BaseLocation string            `json:"base-location"`
	Doc          *string           `json:"doc,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
	Schema       json.RawMessage   `json:"schema,omitempty"`
	Protected    bool              `json:"protected,omitempty"`
}

type createGenericTableBody struct {
	Name         string             `json:"name"`
	Format       string             `json:"format"`
	BaseLocation *string            `json:"base-location,omitempty"`
	Doc          *string            `json:"doc,omitempty"`
	Properties   *map[string]string `json:"properties,omitempty"`
	Schema       json.RawMessage    `json:"schema,omitempty"`
}

type renameGenericTableTarget struct {
	Namespace []string `json:"namespace"`
	Name      string   `json:"name"`
}

// CreateGenericTable creates a generic table in namespace and returns it with
// its ID. The create response carries the table but no ID, so the ID is looked
// up in the namespace's listing afterwards.
func (c *Client) CreateGenericTable(ctx context.Context, projectID, warehouseID string, namespace []string, req GenericTableRequest) (*GenericTable, error) {
	if err := validateGenericTableName(req.Name); err != nil {
		return nil, err
	}
	body := createGenericTableBody{
		Name:         req.Name,
		Format:       req.Format,
		BaseLocation: req.BaseLocation,
		Doc:          req.Doc,
		Schema:       req.Schema,
	}
	if req.Properties != nil {
		body.Properties = &req.Properties
	}

	var result struct {
		Table genericTableData `json:"table"`
	}
	err := c.do(ctx, rawCall{
		op:        "create generic table",
		method:    http.MethodPost,
		path:      genericTablesPath(warehouseID, namespace),
		projectID: projectID,
		body:      body,
		out:       &result,
	})
	if err != nil {
		return nil, err
	}

	table := genericTableFromData(result.Table)
	table.Namespace = slices.Clone(namespace)
	// The table exists now. A failed ID lookup must not fail the create, or
	// Terraform records nothing and the table is orphaned; Read resolves the
	// ID on the next refresh.
	table.ID, _ = c.ResolveGenericTableID(ctx, projectID, warehouseID, namespace, req.Name)
	return table, nil
}

// LoadGenericTable loads one generic table. The response has no ID; use
// ListGenericTables to resolve it.
func (c *Client) LoadGenericTable(ctx context.Context, projectID, warehouseID string, namespace []string, name string) (*GenericTable, error) {
	if err := validateGenericTableName(name); err != nil {
		return nil, err
	}
	var result struct {
		Table genericTableData `json:"table"`
	}
	err := c.do(ctx, rawCall{
		op:        "load generic table",
		method:    http.MethodGet,
		path:      genericTablesPath(warehouseID, namespace) + "/" + name,
		projectID: projectID,
		out:       &result,
	})
	if err != nil {
		return nil, err
	}

	table := genericTableFromData(result.Table)
	table.Namespace = slices.Clone(namespace)
	return table, nil
}

// ListGenericTables lists all generic tables in a namespace, following the
// listing's page tokens.
func (c *Client) ListGenericTables(ctx context.Context, projectID, warehouseID string, namespace []string) ([]GenericTable, error) {
	path := genericTablesPath(warehouseID, namespace)
	var tables []GenericTable
	pageToken := ""

	for {
		query := url.Values{}
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}

		var page struct {
			Identifiers []struct {
				Namespace []string `json:"namespace"`
				Name      string   `json:"name"`
				ID        string   `json:"id"`
				Format    string   `json:"format"`
			} `json:"identifiers"`
			NextPageToken string `json:"next-page-token"`
		}
		err := c.do(ctx, rawCall{
			op:        "list generic tables",
			method:    http.MethodGet,
			path:      path,
			query:     query,
			projectID: projectID,
			out:       &page,
		})
		if err != nil {
			return nil, err
		}

		for _, id := range page.Identifiers {
			tables = append(tables, GenericTable{
				ID:        id.ID,
				Name:      id.Name,
				Namespace: id.Namespace,
				Format:    id.Format,
			})
		}

		if page.NextPageToken == "" {
			return tables, nil
		}
		pageToken = page.NextPageToken
	}
}

// RenameGenericTable renames a generic table in place. The rename endpoint
// takes full source and destination identifiers, not a namespace in the path.
func (c *Client) RenameGenericTable(ctx context.Context, projectID, warehouseID string, sourceNamespace []string, sourceName string, destinationNamespace []string, destinationName string) error {
	if err := validateGenericTableName(sourceName); err != nil {
		return err
	}
	if err := validateGenericTableName(destinationName); err != nil {
		return err
	}
	body := struct {
		Source      renameGenericTableTarget `json:"source"`
		Destination renameGenericTableTarget `json:"destination"`
	}{
		Source:      renameGenericTableTarget{Namespace: sourceNamespace, Name: sourceName},
		Destination: renameGenericTableTarget{Namespace: destinationNamespace, Name: destinationName},
	}

	return c.do(ctx, rawCall{
		op:        "rename generic table",
		method:    http.MethodPost,
		path:      "/lakekeeper/v1/" + warehouseID + "/generic-tables/rename",
		projectID: projectID,
		body:      body,
	})
}

// DeleteGenericTable drops a generic table. With purge, its files are deleted
// too; with force, it is dropped even while protected.
func (c *Client) DeleteGenericTable(ctx context.Context, projectID, warehouseID string, namespace []string, name string, purge, force bool) error {
	if err := validateGenericTableName(name); err != nil {
		return err
	}
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

// ResolveGenericTableID returns a generic table's UUID from its namespace
// listing, because load and create responses do not carry it.
func (c *Client) ResolveGenericTableID(ctx context.Context, projectID, warehouseID string, namespace []string, name string) (string, error) {
	if err := validateGenericTableName(name); err != nil {
		return "", err
	}
	tables, err := c.ListGenericTables(ctx, projectID, warehouseID, namespace)
	if err != nil {
		return "", err
	}
	for _, table := range tables {
		if table.Name == name && slices.Equal(table.Namespace, namespace) {
			if table.ID == "" {
				return "", &Error{err: fmt.Errorf("generic table %q has no ID in the namespace listing", name)}
			}
			return table.ID, nil
		}
	}
	return "", &Error{err: fmt.Errorf("generic table %q is missing from the namespace listing", name)}
}

func genericTableFromData(data genericTableData) *GenericTable {
	return &GenericTable{
		Name:         data.Name,
		Format:       data.Format,
		BaseLocation: data.BaseLocation,
		Doc:          data.Doc,
		Properties:   data.Properties,
		Schema:       rawOrNil(data.Schema),
		Protected:    data.Protected,
	}
}

// rawOrNil turns a JSON null into nil, so callers can distinguish "absent"
// from a JSON document.
func rawOrNil(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

// validateGenericTableName rejects names that would change the request path:
// Go's URL encoding keeps '/' as a path separator, and '.'/'..' are resolved
// as path segments.
func validateGenericTableName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x1f") {
		return fmt.Errorf("invalid generic table name %q: it must not be empty, '.', '..' or contain '/'", name)
	}
	return nil
}

func genericTablesPath(warehouseID string, namespace []string) string {
	return "/lakekeeper/v1/" + warehouseID + "/namespaces/" + strings.Join(namespace, "\x1f") + "/generic-tables"
}
