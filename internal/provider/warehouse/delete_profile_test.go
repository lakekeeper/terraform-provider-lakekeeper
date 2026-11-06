package warehouse_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/warehouse"
)

func TestDeleteProfileModel_Equal(t *testing.T) {
	soft60 := &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(60)}
	hard := &warehouse.DeleteProfileModel{Type: types.StringValue("hard"), ExpirationSeconds: types.Int32Null()}

	tests := []struct {
		name string
		a, b *warehouse.DeleteProfileModel
		want bool
	}{
		{"same soft", soft60, &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(60)}, true},
		{"same hard", hard, &warehouse.DeleteProfileModel{Type: types.StringValue("hard"), ExpirationSeconds: types.Int32Null()}, true},
		{"other type", soft60, hard, false},
		{"other expiration", soft60, &warehouse.DeleteProfileModel{Type: types.StringValue("soft"), ExpirationSeconds: types.Int32Value(120)}, false},
		{"nil and set", nil, hard, false},
		{"set and nil", hard, nil, false},
		{"both nil", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("Equal = %v, want %v", got, tt.want)
			}
		})
	}
}
