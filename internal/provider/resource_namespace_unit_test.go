package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

// A namespace load answers 404 both for a deleted namespace and for one the
// caller may not see. Read removes the namespace from state only when the
// existence check also answers 404.
func TestNamespaceReadNotFound(t *testing.T) {
	const project = "11111111-1111-4111-8111-111111111111"
	const warehouse = "22222222-2222-4222-8222-222222222222"
	const namespace = "33333333-3333-4333-8333-333333333333"
	for _, tc := range []struct {
		name        string
		checkStatus int
		wantRemoved bool
		wantError   string
	}{
		{"deleted", http.StatusNotFound, true, ""},
		{"hidden", http.StatusOK, false, "exists but cannot be read"},
		{"check fails", http.StatusServiceUnavailable, false, "Cannot confirm that the namespace was deleted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /catalog/v1/config":
					_, _ = w.Write([]byte(`{"defaults":{},"overrides":{"prefix":"` + warehouse + `"}}`))
				case "GET /catalog/v1/" + warehouse + "/namespaces/sales":
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"error":{"code":404,"type":"NoSuchNamespaceException","message":"Namespace not found or access denied"}}`))
				case "GET /management/v1/warehouse":
					_, _ = w.Write([]byte(`{"warehouses":[{"id":"` + warehouse + `","warehouse-id":"` + warehouse + `","project-id":"` + project + `","name":"wh","status":"active","protected":false,` +
						`"allowed-format-versions":[1,2],"delete-profile":{"type":"hard"},"storage-profile":{"type":"gcs","bucket":"b"}}]}`))
				case "POST /management/v1/action/batch-check":
					checks++
					body, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(body), `"namespace-id":"`+namespace+`"`) || !strings.Contains(string(body), `"error-on-not-found":true`) {
						t.Errorf("unexpected check request: %s", body)
					}
					w.WriteHeader(tc.checkStatus)
					if tc.checkStatus == http.StatusOK {
						_, _ = w.Write([]byte(`{"results":[{"allowed":false}]}`))
						return
					}
					_, _ = fmt.Fprintf(w, `{"error":{"code":%d,"type":"Test","message":"check"}}`, tc.checkStatus)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			ctx := context.Background()
			client, err := api.NewStaticTokenClient(ctx, server.URL, "test")
			if err != nil {
				t.Fatal(err)
			}
			r := &lakekeeperNamespaceResource{client: client}
			var schema resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			state := tfsdk.State{Schema: schema.Schema}
			if diags := state.Set(ctx, lakekeeperNamespaceResourceModel{
				ID:            types.StringValue(project + "/wh/sales"),
				NamespaceID:   types.StringValue(namespace),
				ManagedAccess: types.BoolNull(),
				ProjectID:     types.StringValue(project),
				WarehouseName: types.StringValue("wh"),
				Name:          types.StringValue("sales"),
			}); diags.HasError() {
				t.Fatal(diags)
			}
			resp := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &resp)

			if checks != 1 {
				t.Errorf("existence checks = %d, want 1", checks)
			}
			if removed := resp.State.Raw.IsNull(); removed != tc.wantRemoved {
				t.Errorf("removed from state = %t, want %t", removed, tc.wantRemoved)
			}
			if tc.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected error: %v", resp.Diagnostics)
				}
				return
			}
			if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics[0].Summary(), tc.wantError) {
				t.Errorf("diagnostics = %v, want an error %q", resp.Diagnostics, tc.wantError)
			}
		})
	}
}
