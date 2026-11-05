package provider

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Identity validators for assignment resources, taken from Lakekeeper's server rules.

var (
	uuidValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`),
		"must be a UUID",
	)
	projectIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[\p{L}\p{N}_-]+$`),
		"must be at least one character and contain only letters, digits, '-' or '_'",
	)
)

// projectIDAttribute is a project_id identity attribute.
func projectIDAttribute(description string) assignmentAttribute {
	return assignmentAttribute{Name: "project_id", Description: description, Validators: []validator.String{projectIDValidator}}
}

// uuidAttribute is an identity attribute holding a UUID, e.g. warehouse_id or namespace_id.
func uuidAttribute(name, description string) assignmentAttribute {
	return assignmentAttribute{Name: name, Description: description, Validators: []validator.String{uuidValidator}}
}

// roleIDAttribute is an identity attribute holding a role UUID, used as scope or assignee.
func roleIDAttribute(name, description string) assignmentAttribute {
	return uuidAttribute(name, description)
}
