package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestTagDefinitionValidateConfig(t *testing.T) {
	r := &lakekeeperTagDefinitionResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	makeConfig := func(valueKind string, allowedValues []string) tfsdk.Config {
		av := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil)
		if allowedValues != nil {
			elems := make([]tftypes.Value, len(allowedValues))
			for i, v := range allowedValues {
				elems[i] = tftypes.NewValue(tftypes.String, v)
			}
			av = tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, elems)
		}
		raw := tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                tftypes.String,
				"tag_definition_id": tftypes.String,
				"project_id":        tftypes.String,
				"name":              tftypes.String,
				"description":       tftypes.String,
				"scope":             tftypes.Set{ElementType: tftypes.String},
				"value_kind":        tftypes.String,
				"allowed_values":    tftypes.Set{ElementType: tftypes.String},
				"created_at":        tftypes.String,
				"updated_at":        tftypes.String,
			},
		}, map[string]tftypes.Value{
			"id":                tftypes.NewValue(tftypes.String, nil),
			"tag_definition_id": tftypes.NewValue(tftypes.String, nil),
			"project_id":        tftypes.NewValue(tftypes.String, "p1"),
			"name":              tftypes.NewValue(tftypes.String, "pii"),
			"description":       tftypes.NewValue(tftypes.String, nil),
			"scope":             tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "table")}),
			"value_kind":        tftypes.NewValue(tftypes.String, valueKind),
			"allowed_values":    av,
			"created_at":        tftypes.NewValue(tftypes.String, nil),
			"updated_at":        tftypes.NewValue(tftypes.String, nil),
		})
		return tfsdk.Config{Schema: schemaResp.Schema, Raw: raw}
	}

	cases := []struct {
		name      string
		valueKind string
		allowed   []string
		wantErr   bool
	}{
		{"enumerated with values", "enumerated", []string{"low", "high"}, false},
		{"enumerated without values", "enumerated", nil, true},
		{"enumerated with empty values", "enumerated", []string{}, true},
		{"marker without values", "marker", nil, false},
		{"marker with values", "marker", []string{"a"}, true},
		{"free-text without values", "free-text", nil, false},
		{"free-text with values", "free-text", []string{"a"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: makeConfig(tc.valueKind, tc.allowed),
			}, &resp)
			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("wantErr=%v, got diags: %v", tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestWidenOnlySetModifier(t *testing.T) {
	mod := widenOnlySetModifier("scope", "cannot narrow")

	setOf := func(vals ...string) types.Set {
		elems := make([]attr.Value, len(vals))
		for i, v := range vals {
			elems[i] = types.StringValue(v)
		}
		return types.SetValueMust(types.StringType, elems)
	}

	nonNullState := tfsdk.State{Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})}

	// Widening succeeds.
	var resp planmodifier.SetResponse
	mod.PlanModifySet(context.Background(), planmodifier.SetRequest{
		Path:       path.Root("scope"),
		State:      nonNullState,
		StateValue: setOf("table"),
		PlanValue:  setOf("table", "column"),
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("widening should succeed, got: %v", resp.Diagnostics)
	}

	// Narrowing fails.
	resp = planmodifier.SetResponse{}
	mod.PlanModifySet(context.Background(), planmodifier.SetRequest{
		Path:       path.Root("scope"),
		State:      nonNullState,
		StateValue: setOf("table", "column"),
		PlanValue:  setOf("table"),
	}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("narrowing should fail")
	}
}
