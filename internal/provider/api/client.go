// Package api is the only place in the provider that talks to the Lakekeeper
// Go SDK.
//
// Resources and data sources call methods on *Client and work with the
// SDK-neutral types defined in this package (Project, Warehouse, Assignment,
// StorageProfile, ...). They never import the SDK directly; a depguard rule in
// .golangci.yml enforces this. Replacing or upgrading the SDK therefore only
// touches this package.
//
// Conventions:
//   - Every method returns errors whose message is the SDK's message, wrapped
//     so callers can test for an HTTP status with IsNotFound / StatusCode.
//   - Optional values are pointers; nil means "not set" (omitted on writes,
//     absent on reads).
//   - Enum values are plain string constants defined in this package. Their
//     values are the Lakekeeper wire values.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/apache/iceberg-go/catalog/rest"
	lakekeeper "github.com/baptistegh/go-lakekeeper/pkg/client"
	"github.com/baptistegh/go-lakekeeper/pkg/core"
)

// Client is the provider's Lakekeeper client. Create it with
// Config.NewLakekeeperClient.
type Client struct {
	sdk *lakekeeper.Client
}

// NewStaticTokenClient returns a Client that authenticates with a static
// bearer token and does not retry failed requests. It is meant for unit tests
// that run against an httptest server.
func NewStaticTokenClient(ctx context.Context, baseURL, token string) (*Client, error) {
	c, err := lakekeeper.NewClient(ctx, token, baseURL, lakekeeper.WithoutRetries())
	if err != nil {
		return nil, err
	}
	return &Client{sdk: c}, nil
}

// Catalog returns an Iceberg REST catalog client for the given warehouse.
func (c *Client) Catalog(ctx context.Context, projectID, warehouseName string) (*rest.Catalog, error) {
	return c.sdk.CatalogV1(ctx, projectID, warehouseName)
}

// Error is returned by Client methods when a Lakekeeper API call fails.
// Its message is the underlying SDK error message.
type Error struct {
	// StatusCode is the HTTP status of the failed response, or 0 if no
	// response was received.
	StatusCode int
	err        error
}

func (e *Error) Error() string { return e.err.Error() }
func (e *Error) Unwrap() error { return e.err }

// StatusCode returns the HTTP status code carried by err, or 0 if err does not
// come from a Lakekeeper API response.
func StatusCode(err error) int {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.StatusCode
	}
	return 0
}

// IsNotFound reports whether err is a Lakekeeper API error with HTTP status 404.
func IsNotFound(err error) bool {
	return StatusCode(err) == http.StatusNotFound
}

// wrapErr converts an SDK error into an *Error, taking the status code from
// the HTTP response or, failing that, from the SDK's API error.
func wrapErr(resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	status := 0
	if resp != nil {
		status = resp.StatusCode
	} else {
		if apiErr, ok := errors.AsType[*core.APIError](err); ok {
			status = apiErr.StatusCode
		}
	}
	return &Error{StatusCode: status, err: err}
}
