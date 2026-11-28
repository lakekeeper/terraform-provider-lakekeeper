package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTagDefinitionAndAttachmentsAPI(t *testing.T) {
	var requests []string
	mux := http.NewServeMux()
	mux.HandleFunc("/management/v1/tag-definition", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" project="+r.Header.Get("x-project-id"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "td1",
				"name":           "pii",
				"description":    "PII tag",
				"scope":          []string{"table", "column"},
				"value-kind":     "enumerated",
				"allowed-values": []string{"low", "high"},
				"created-at":     "2026-01-01T00:00:00Z",
			})
		case http.MethodGet:
			if r.URL.Query().Get("pageToken") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"tag-definitions": []map[string]any{{
						"id":         "td1",
						"name":       "pii",
						"scope":      []string{"table"},
						"value-kind": "marker",
						"created-at": "2026-01-01T00:00:00Z",
					}},
					"next-page-token": "p2",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag-definitions": []map[string]any{{
					"id":         "td2",
					"name":       "owner",
					"scope":      []string{"warehouse"},
					"value-kind": "free-text",
					"created-at": "2026-01-01T00:00:00Z",
				}},
			})
		}
	})
	mux.HandleFunc("/management/v1/tag-definition/td1", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" project="+r.Header.Get("x-project-id"))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":             "td1",
				"name":           "pii",
				"scope":          []string{"table", "column"},
				"value-kind":     "enumerated",
				"allowed-values": []string{"low", "high"},
				"created-at":     "2026-01-01T00:00:00Z",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/management/v1/tag-definition/missing", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "not found", "type": "NotFound", "code": 404}})
	})
	mux.HandleFunc("/management/v1/tag-definition/td1/attachments", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"attachments": []map[string]any{{
				"created-at": "2026-01-01T00:00:00Z",
				"source":     "manual",
				"value":      "high",
				"target": map[string]any{
					"type":         "table",
					"warehouse-id": "w1",
					"table-id":     "t1",
				},
			}},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewStaticTokenClient(context.Background(), srv.URL, "tok")
	if err != nil {
		t.Fatal(err)
	}

	td, err := c.CreateTagDefinition(context.Background(), "proj-1", CreateTagDefinitionRequest{
		Name:          "pii",
		Description:   new("PII tag"),
		Scope:         []string{TagScopeTable, TagScopeColumn},
		ValueKind:     TagValueKindEnumerated,
		AllowedValues: []string{"low", "high"},
	})
	if err != nil {
		t.Fatalf("CreateTagDefinition: %v", err)
	}
	if td.ID != "td1" || td.ValueKind != TagValueKindEnumerated || len(td.AllowedValues) != 2 {
		t.Fatalf("unexpected tag definition: %+v", td)
	}

	defs, err := c.ListTagDefinitions(context.Background(), "proj-1", nil)
	if err != nil {
		t.Fatalf("ListTagDefinitions: %v", err)
	}
	if len(defs) != 2 || defs[0].ID != "td1" || defs[1].ID != "td2" {
		t.Fatalf("unexpected paginated definitions: %+v", defs)
	}

	atts, err := c.ListTagAttachments(context.Background(), "proj-1", "td1", nil)
	if err != nil {
		t.Fatalf("ListTagAttachments: %v", err)
	}
	if len(atts) != 1 || atts[0].Target.Type != TagScopeTable || atts[0].Target.TableID != "t1" {
		t.Fatalf("unexpected attachments: %+v", atts)
	}

	exists, err := c.TagDefinitionExists(context.Background(), "missing")
	if err != nil {
		t.Fatalf("TagDefinitionExists: %v", err)
	}
	if exists {
		t.Fatal("expected missing tag definition to return exists=false")
	}

	if err := c.DeleteTagDefinition(context.Background(), "proj-1", "td1"); err != nil {
		t.Fatalf("DeleteTagDefinition: %v", err)
	}
	if len(requests) == 0 || !strings.Contains(requests[0], "project=proj-1") {
		t.Fatalf("expected x-project-id header, got %v", requests)
	}
}
