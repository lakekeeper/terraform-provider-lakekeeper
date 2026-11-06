package warehouse_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"
)

// populated builds a value of type t in which every nested object is set and
// every primitive is null, so decoding it visits every attribute of a schema.
func populated(t tftypes.Type) tftypes.Value {
	obj, ok := t.(tftypes.Object)
	if !ok {
		return tftypes.NewValue(t, nil)
	}
	attrs := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, at := range obj.AttributeTypes {
		attrs[name] = populated(at)
	}
	return tftypes.NewValue(t, attrs)
}

func TestResourceSchemaDecodesIntoResourceModel(t *testing.T) {
	ctx := context.Background()
	s := warehouse.ResourceSchema()
	if diags := s.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema: %v", diags)
	}
	state := tfsdk.State{Schema: s, Raw: populated(s.Type().TerraformType(ctx))}

	var m warehouse.ResourceModel
	if diags := state.Get(ctx, &m); diags.HasError() {
		t.Fatalf("schema and model disagree: %v", diags)
	}
	sp := m.StorageProfile
	if sp == nil || sp.S3 == nil || sp.S3.Credential == nil ||
		sp.ADLS == nil || sp.ADLS.Credential == nil ||
		sp.GCS == nil || sp.GCS.Credential == nil || m.DeleteProfile == nil {
		t.Fatalf("nested objects were not decoded: %+v", m)
	}
}

func TestDataSourceSchemaDecodesIntoDataSourceModel(t *testing.T) {
	ctx := context.Background()
	s := warehouse.DataSourceSchema()
	if diags := s.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema: %v", diags)
	}
	state := tfsdk.State{Schema: s, Raw: populated(s.Type().TerraformType(ctx))}

	var m warehouse.DataSourceModel
	if diags := state.Get(ctx, &m); diags.HasError() {
		t.Fatalf("schema and model disagree: %v", diags)
	}
	sp := m.StorageProfile
	if sp == nil || sp.S3 == nil || sp.ADLS == nil || sp.GCS == nil || m.DeleteProfile == nil {
		t.Fatalf("nested objects were not decoded: %+v", m)
	}
}
