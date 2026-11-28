package tags

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func TestAttachmentSpecSchemaAndHelpers(t *testing.T) {
	spec := Spec{
		TypeName:     "column_tag",
		TargetNoun:   "column",
		OperationDoc: "set_table_column_tag",
		Parents: []ParentAttr{
			{Name: "warehouse_id", Description: "Warehouse UUID."},
			{Name: "table_id", Description: "Table UUID."},
			{Name: "column_name", Description: "Column name."},
		},
	}

	r := NewResource(spec, func(any) (*api.Client, error) { return nil, nil })
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	for _, name := range []string{"id", "warehouse_id", "table_id", "column_name", "tag_name", "value", "tag_definition_id", "applied_at"} {
		if _, ok := schemaResp.Schema.Attributes[name]; !ok {
			t.Fatalf("missing attribute %q in schema", name)
		}
	}

	parts, tagName, err := parseAttachmentID("w1/t1/email/pii", 3)
	if err != nil {
		t.Fatalf("parseAttachmentID failed: %v", err)
	}
	if len(parts) != 3 || parts[0] != "w1" || parts[1] != "t1" || parts[2] != "email" || tagName != "pii" {
		t.Fatalf("unexpected parse result: %v, %q", parts, tagName)
	}

	if _, _, err := parseAttachmentID("w1//email/pii", 3); err == nil {
		t.Fatal("expected error for empty segment")
	}
	if _, _, err := parseAttachmentID("w1/t1/pii", 3); err == nil {
		t.Fatal("expected error for wrong segment count")
	}

	errMsg := formatSetTagError("pii", new("secret"), api.NewError(http.StatusBadRequest, context.DeadlineExceeded))
	if !strings.Contains(errMsg, "marker") || !strings.Contains(errMsg, "enumerated") {
		t.Fatalf("expected actionable value_kind hint in error, got: %s", errMsg)
	}
}
