package provider

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Identity validators for assignment resources, taken from Lakekeeper's server rules.

// Names of identity attributes that scope adapters read from assignmentScope.
const (
	projectIDAttributeName   = "project_id"
	namespaceIDAttributeName = "namespace_id"
)

var uuidValidator = stringvalidator.RegexMatches(
	regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`),
	"must be a UUID",
)

// projectIDValidator follows Lakekeeper's ProjectId::try_new: not empty, and
// every character is '-', '_' or alphanumeric in Rust's sense
// (char::is_alphanumeric, i.e. Unicode Alphabetic or Numeric).
var projectIDValidator = identityValidator{
	description: "must be at least one character and contain only letters, digits, '-' or '_'",
	valid: func(v string) bool {
		if v == "" {
			return false
		}
		for _, c := range v {
			if c != '-' && c != '_' && !isRustAlphanumeric(c) {
				return false
			}
		}
		return true
	},
}

// isRustAlphanumeric matches Rust's char::is_alphanumeric: the Unicode
// Alphabetic property (letters, letter numbers and Other_Alphabetic marks) or a
// Numeric general category (Nd, Nl, No).
func isRustAlphanumeric(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.Is(unicode.Other_Alphabetic, c)
}

// projectIDAttribute is a project_id identity attribute.
func projectIDAttribute(description string) assignmentAttribute {
	return assignmentAttribute{Name: projectIDAttributeName, Description: description, Validators: []validator.String{projectIDValidator}}
}

// uuidAttribute is an identity attribute holding a UUID, e.g. warehouse_id, namespace_id or role_id.
func uuidAttribute(name, description string) assignmentAttribute {
	return assignmentAttribute{Name: name, Description: description, Validators: []validator.String{uuidValidator}}
}

// userIDSubjectMaxBytes is Lakekeeper's limit on the subject after '~': it must
// be shorter than 128 bytes (Rust str::len counts bytes, not characters).
const userIDSubjectMaxBytes = 127

// userIDValidator follows Lakekeeper's user ID rule: <idp>~<subject>, split at
// the first '~', with text on both sides, at most 127 bytes after it and no
// control characters. A '/' is allowed.
var userIDValidator = identityValidator{
	description: "must be in the form <idp>~<subject>, with at most 127 bytes after '~' and no control characters",
	valid: func(v string) bool {
		idp, subject, found := strings.Cut(v, "~")
		return found && idp != "" && subject != "" && len(subject) <= userIDSubjectMaxBytes &&
			!strings.ContainsFunc(v, unicode.IsControl)
	},
}

// userIDAttribute is an identity attribute holding a user ID, used as assignee.
func userIDAttribute(name, description string) assignmentAttribute {
	return assignmentAttribute{Name: name, Description: description, Validators: []validator.String{userIDValidator}}
}

// identityValidator is a string validator backed by a predicate, for rules a
// regular expression can't express. Its diagnostic matches RegexMatches.
type identityValidator struct {
	description string
	valid       func(string) bool
}

var _ validator.String = identityValidator{}

func (v identityValidator) Description(context.Context) string { return v.description }

func (v identityValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (v identityValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if value := req.ConfigValue.ValueString(); !v.valid(value) {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(req.Path, v.description, value))
	}
}
