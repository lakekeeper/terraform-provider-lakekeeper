package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
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
	// Scope is the scope's identity attribute. Its zero value (no Name) means the server.
	Scope assignmentAttribute
	// AssigneeKind is the kind of assignee whose grants this resource manages.
	AssigneeKind assigneeKind
	// Assignee is the assignee's identity attribute. It is always the last ID part.
	Assignee assignmentAttribute
	// GrantTypes lists the allowed grant types at this scope.
	GrantTypes []string
	// Notes is optional text appended to the generated description.
	Notes string
	// NewAdapter builds the scope adapter from the configured Lakekeeper client.
	NewAdapter func(*api.Client) scopeAdapter
}

// assignmentAttribute is one identity attribute of an assignment resource.
// Identity attributes are required and force replacement.
type assignmentAttribute struct {
	Name        string
	Description string
	Validators  []validator.String
}

// assigneeKind is the kind of assignee as it crosses the scope adapter seam.
// Its values are the api assignee types.
type assigneeKind string

const (
	roleAssignee assigneeKind = api.AssigneeTypeRole
	userAssignee assigneeKind = api.AssigneeTypeUser
)

// identity returns the identity attributes in ID order: extra parts, scope, assignee.
func (d assignmentDescriptor) identity() []assignmentAttribute {
	attrs := append([]assignmentAttribute{}, d.ExtraIDParts...)
	if d.Scope.Name != "" {
		attrs = append(attrs, d.Scope)
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

// buildAssignmentID joins identity values, in ID order, into the resource ID.
func buildAssignmentID(values []string) string {
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
