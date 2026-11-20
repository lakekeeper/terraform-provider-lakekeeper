package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lakekeeper/terraform-provider-lakekeeper/internal/provider/api"
)

func TestRoleNameLookup(t *testing.T) {
	roleJSON := func(id, projectID, name string) string {
		return fmt.Sprintf(`{"id":%q,"project-id":%q,"name":%q,"created-at":"2026-10-30T10:00:00Z","ident":"lakekeeper~%s","provider-id":"lakekeeper","source-id":%q}`, id, projectID, name, id, id)
	}
	for _, tc := range []struct {
		name      string
		pages     []string
		wantError string
	}{
		{"exact", []string{`{"roles":[` + roleJSON("r", "p", "target") + `]}`}, ""},
		{"different case", []string{`{"roles":[` + roleJSON("r", "p", "Target") + `]}`}, ""},
		{"missing", []string{`{"roles":[]}`}, "no role"},
		{"prefix is not exact", []string{`{"roles":[` + roleJSON("r", "p", "target-extra") + `]}`}, "no role"},
		{"other project", []string{`{"roles":[` + roleJSON("r", "other", "target") + `]}`}, "another project"},
		{"second page", []string{`{"roles":[],"next-page-token":"next"}`, `{"roles":[` + roleJSON("r", "p", "target") + `]}`}, ""},
		{"ambiguous across pages", []string{`{"roles":[` + roleJSON("r", "p", "target") + `],"next-page-token":"next"}`, `{"roles":[` + roleJSON("s", "p", "target") + `]}`}, "multiple roles"},
		{"repeated token", []string{`{"roles":[],"next-page-token":"next"}`, `{"roles":[],"next-page-token":"next"}`}, "repeated"},
		{"later page fails", []string{`{"roles":[` + roleJSON("r", "p", "target") + `],"next-page-token":"next"}`, "error"}, "list roles"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/management/v1/role" || r.Header.Get("x-project-id") != "p" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
				}
				if calls > 0 && r.URL.Query().Get("pageToken") != "next" {
					t.Error("missing pagination token")
				}
				if calls >= len(tc.pages) {
					t.Error("unexpected extra page")
					w.WriteHeader(500)
					return
				}
				page := tc.pages[calls]
				calls++
				w.Header().Set("Content-Type", "application/json")
				if page == "error" {
					w.WriteHeader(403)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "denied", "type": "Forbidden", "code": 403}})
					return
				}
				_, _ = w.Write([]byte(page))
			}))
			defer server.Close()
			client, err := api.NewStaticTokenClient(context.Background(), server.URL, "test")
			if err != nil {
				t.Fatal(err)
			}
			role, err := findRoleByName(context.Background(), client, "p", "target")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("wanted %q, got %v", tc.wantError, err)
				}
			} else if err != nil || role.ID != "r" {
				t.Fatalf("role=%v err=%v", role, err)
			}
		})
	}
}

func TestRoleReadByIDAndName(t *testing.T) {
	for _, byName := range []bool{false, true} {
		t.Run(fmt.Sprint(byName), func(t *testing.T) {
			roleJSON := `{"id":"r","project-id":"p","name":"target","created-at":"2026-10-30T10:00:00Z","ident":"lakekeeper~r","provider-id":"lakekeeper","source-id":"r"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if byName {
					if r.URL.Path != "/management/v1/role" {
						t.Errorf("unexpected name lookup %s", r.URL.Path)
					}
					_, _ = w.Write([]byte(`{"roles":[` + roleJSON + `]}`))
				} else {
					if r.URL.Path != "/management/v1/role/r" {
						t.Errorf("unexpected ID lookup %s", r.URL.Path)
					}
					_, _ = w.Write([]byte(roleJSON))
				}
			}))
			defer server.Close()
			client, err := api.NewStaticTokenClient(context.Background(), server.URL, "test")
			if err != nil {
				t.Fatal(err)
			}
			d := &LakekeeperRoleDataSource{client: client}
			var schemaResp datasource.SchemaResponse
			d.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)
			state := tfsdk.State{Schema: schemaResp.Schema}
			input := LakekeeperRoleDataSourceModel{ProjectID: types.StringValue("p")}
			wantName := "target"
			if byName {
				// A configured name keeps its spelling even when Lakekeeper stores another case.
				wantName = "Target"
				input.Name = types.StringValue(wantName)
			} else {
				input.RoleID = types.StringValue("r")
			}
			if diags := state.Set(context.Background(), input); diags.HasError() {
				t.Fatal(diags)
			}
			resp := datasource.ReadResponse{State: state}
			d.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var result LakekeeperRoleDataSourceModel
			if diags := resp.State.Get(context.Background(), &result); diags.HasError() {
				t.Fatal(diags)
			}
			if result.ID.ValueString() != "p/r" || result.RoleID.ValueString() != "r" || result.Name.ValueString() != wantName {
				t.Fatalf("incorrect role state: %+v", result)
			}
		})
	}
}
