package provider

import (
	"fmt"
	"strings"

	lakekeeper "github.com/baptistegh/go-lakekeeper/pkg/client"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// assignmentDescriptor declares one assignment resource. The shared assignment
// module (assignment_resource.go) derives the schema, validators, ID and import
// handling and the whole lifecycle from it.
type assignmentDescriptor struct {
	// TypeName is the resource type name without the provider prefix, e.g. "project_role_assignment".
	TypeName string
	// ScopeNoun names the scope in generated descriptions, e.g. "project" or "server".
	ScopeNoun string
	// ExtraIDParts are identity attributes that precede the scope in the ID, e.g. project_id for a namespace.
	ExtraIDParts []assignmentAttribute
	// Scope is the scope's identity attribute, or nil for the server.
	Scope *assignmentAttribute
	// AssigneeKind is the kind of assignee whose grants this resource manages.
	AssigneeKind assigneeKind
	// Assignee is the assignee's identity attribute. It is always the last ID part.
	Assignee assignmentAttribute
	// GrantTypes lists the allowed grant types at this scope.
	GrantTypes []string
	// Notes is optional text appended to the generated description.
	Notes string
	// NewAdapter builds the scope adapter from the configured Lakekeeper client.
	NewAdapter func(*lakekeeper.Client) scopeAdapter
}

// assignmentAttribute is one identity attribute of an assignment resource.
// Identity attributes are required and force replacement.
type assignmentAttribute struct {
	Name        string
	Description string
	Validators  []validator.String
}

// assigneeKind is the kind of assignee as it crosses the scope adapter seam.
type assigneeKind string

const (
	roleAssignee assigneeKind = "role"
)

// grantTypes converts go-lakekeeper's typed grant type constants to plain strings.
func grantTypes[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

func ptr[T any](v T) *T { return &v }

// identity returns the identity attributes in ID order: extra parts, scope, assignee.
func (d assignmentDescriptor) identity() []assignmentAttribute {
	attrs := append([]assignmentAttribute{}, d.ExtraIDParts...)
	if d.Scope != nil {
		attrs = append(attrs, *d.Scope)
	}
	return append(attrs, d.Assignee)
}

// idFormat documents the ID and import format, e.g. "{{project_id}}/{{role_id}}".
func (d assignmentDescriptor) idFormat() string {
	parts := []string{}
	for _, a := range d.identity() {
		parts = append(parts, "{{"+a.Name+"}}")
	}
	return strings.Join(parts, "/")
}

// buildID joins identity values, in ID order, into the resource ID.
func (d assignmentDescriptor) buildID(values []string) string {
	return strings.Join(values, "/")
}

// parseID splits an ID into identity values in ID order. It splits only up to
// the expected number of parts, so the assignee (always last) may contain "/".
func (d assignmentDescriptor) parseID(id string) ([]string, error) {
	attrs := d.identity()
	parts := strings.SplitN(id, "/", len(attrs))
	if len(parts) != len(attrs) {
		return nil, fmt.Errorf("expected an ID in the form %s, got %q", d.idFormat(), id)
	}
	for i, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("expected an ID in the form %s, got %q with an empty %s", d.idFormat(), id, attrs[i].Name)
		}
	}
	return parts, nil
}
