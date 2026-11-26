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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/iceberg-go/catalog/rest"
	lakekeeper "github.com/lakekeeper/go-lakekeeper/pkg/client"
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
	c, err := lakekeeper.New(ctx, baseURL, token, lakekeeper.WithoutRetries())
	if err != nil {
		return nil, err
	}
	return &Client{sdk: c}, nil
}

// Catalog returns an Iceberg REST catalog client for the given warehouse.
//
// Creating the client fetches the catalog config of the warehouse. On failure
// the error is an *Error, so IsNotFound reports a warehouse that does not
// exist (Lakekeeper answers 404 NoSuchWarehouseException).
func (c *Client) Catalog(ctx context.Context, projectID, warehouseName string) (*rest.Catalog, error) {
	cat, err := c.sdk.CatalogV1(ctx, projectID, warehouseName)
	if err != nil {
		return nil, &Error{StatusCode: c.catalogConfigStatus(ctx, projectID, warehouseName), err: err}
	}
	return cat, nil
}

// catalogConfigStatus returns the HTTP status of the catalog config request
// for a warehouse, or 0 if no response was received. iceberg-go keeps the
// status of a failed request in an unexported type, so Catalog sends the
// request again as a plain request to learn it.
func (c *Client) catalogConfigStatus(ctx context.Context, projectID, warehouseName string) int {
	u := c.sdk.BaseURL()
	u.Path = strings.TrimSuffix(u.Path, "/") + "/catalog/v1/config"
	u.RawQuery = url.Values{"warehouse": {projectID + "/" + warehouseName}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.sdk.GetConfig().HTTPClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// Error is returned by Client methods when a Lakekeeper API call fails.
// Its message is the underlying SDK error message.
type Error struct {
	// StatusCode is the HTTP status of the failed response, or 0 if no
	// response was received.
	StatusCode int
	err        error
}

// NewError returns an *Error carrying statusCode, for code outside this package
// that has to produce one, such as tests of how callers react to a status.
func NewError(statusCode int, err error) error {
	return &Error{StatusCode: statusCode, err: err}
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

// wrapErr converts an error returned by an SDK façade into an *Error, taking
// the status code from the SDK's API error.
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	status := 0
	if apiErr, ok := errors.AsType[*lakekeeper.APIError](err); ok {
		status = apiErr.StatusCode
	}
	return &Error{StatusCode: status, err: err}
}

// wrapGenErr is wrapErr for calls made with the generated client, which
// return the raw response and error. op names the operation in the message.
func wrapGenErr(op string, resp *http.Response, err error) error {
	return wrapErr(lakekeeper.WrapAPIError(op, err, resp))
}

// rawCall describes a request to an endpoint the SDK does not wrap. path is
// appended to the configured base URL and may contain the multi-level
// namespace separator (\x1f). out, when non-nil, receives the decoded JSON
// response.
type rawCall struct {
	op        string
	method    string
	path      string
	query     url.Values
	projectID string
	body      any
	out       any
}

// do sends a rawCall through the SDK's authenticated HTTP client and wraps
// failures like the generated client's, so callers can still use IsNotFound
// and StatusCode.
func (c *Client) do(ctx context.Context, call rawCall) error {
	var body io.Reader
	if call.body != nil {
		b, err := json.Marshal(call.body)
		if err != nil {
			return &Error{err: fmt.Errorf("%s: encode request: %w", call.op, err)}
		}
		body = bytes.NewReader(b)
	}

	u := c.sdk.BaseURL()
	u.Path = strings.TrimSuffix(u.Path, "/") + call.path
	if call.query != nil {
		u.RawQuery = call.query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, call.method, u.String(), body)
	if err != nil {
		return &Error{err: fmt.Errorf("%s: %w", call.op, err)}
	}
	req.Header.Set("Accept", "application/json")
	if call.projectID != "" {
		req.Header.Set("x-project-id", call.projectID)
	}
	if call.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.sdk.GetConfig().HTTPClient.Do(req)
	if err != nil {
		return &Error{err: fmt.Errorf("%s: %w", call.op, err)}
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %w", call.op, err)}
	}
	if resp.StatusCode >= 300 {
		return &Error{StatusCode: resp.StatusCode, err: fmt.Errorf("%s: %s: %s", call.op, resp.Status, strings.TrimSpace(string(b)))}
	}
	if call.out == nil {
		return nil
	}
	if err := json.Unmarshal(b, call.out); err != nil {
		return &Error{err: fmt.Errorf("%s: decode response: %w", call.op, err)}
	}
	return nil
}
