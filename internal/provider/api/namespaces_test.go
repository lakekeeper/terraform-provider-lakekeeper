package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEncodeNamespacePath(t *testing.T) {
	for _, tc := range []struct {
		name, want, wantError string
		parts                 []string
	}{
		{"single level", "a", "", []string{"a"}},
		{"multiple levels", "a\x1fb\x1fc", "", []string{"a", "b", "c"}},
		{"level with dot", "a.b", "", []string{"a.b"}},
		{"empty level", "", "non-empty", []string{"a", ""}},
		{"level with slash", "", "cannot contain", []string{"a/b"}},
		{"level with separator", "", "cannot contain", []string{"a\x1fb"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeNamespacePath(tc.parts)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("wanted error %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("EncodeNamespacePath(%v)=%q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}

func TestCatalogURLBuilding(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		name, basePath, wantPath string
	}{
		{"base path without trailing slash", "/management/v1", "/catalog/v1/22222222-2222-4222-8222-222222222222/namespaces"},
		{"base path with trailing slash", "/management/v1/", "/catalog/v1/22222222-2222-4222-8222-222222222222/namespaces"},
		{"empty base path", "", "/catalog/v1/22222222-2222-4222-8222-222222222222/namespaces"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.wantPath {
					t.Errorf("path=%q, want %q", r.URL.Path, tc.wantPath)
				}
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			client, err := NewStaticTokenClient(context.Background(), server.URL+tc.basePath, "test")
			if err != nil {
				t.Fatal(err)
			}
			var result struct{}
			_ = client.catalogGet(context.Background(), "test", "proj", warehouse, "/namespaces", nil, &result)
		})
	}
}

func TestCatalogURLWithProjectHeader(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	project := "11111111-1111-4111-8111-111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-project-id"); got != project {
			t.Errorf("x-project-id header=%q, want %q", got, project)
		}
		if !strings.HasSuffix(r.URL.Path, "/namespaces/a") {
			t.Errorf("path=%q does not end with /namespaces/a", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client, err := NewStaticTokenClient(context.Background(), server.URL+"/management/v1", "test")
	if err != nil {
		t.Fatal(err)
	}
	var result struct{}
	_ = client.catalogGet(context.Background(), "test", project, warehouse, "/namespaces/a", nil, &result)
}

func TestListNamespacesPagination(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	calls := 0
	pages := []string{
		`{"namespaces":[["instrument"]],"namespace-uuids":["22222222-2222-4222-8222-222222222222"],"next-page-token":"next"}`,
		`{"namespaces":[["analyzer"]],"namespace-uuids":["11111111-1111-4111-8111-111111111111"],"next-page-token":null}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/catalog/v1/"+warehouse+"/namespaces" || q.Get("returnUuids") != "true" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if (calls > 0) != (q.Get("pageToken") == "next") {
			t.Errorf("call %d sent pageToken %q", calls, q.Get("pageToken"))
		}
		if calls >= len(pages) {
			t.Error("unexpected extra page")
			w.WriteHeader(500)
			return
		}
		_, _ = w.Write([]byte(pages[calls]))
		calls++
	}))
	defer server.Close()
	client, err := NewStaticTokenClient(context.Background(), server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ListNamespaces(context.Background(), warehouse, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0].Parts[0] != "instrument" || result[1].Parts[0] != "analyzer" {
		t.Errorf("got %+v", result)
	}
}

func TestListNamespacesUUIDCountMismatch(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"namespaces":[["a"]],"namespace-uuids":[]}`))
	}))
	defer server.Close()
	client, err := NewStaticTokenClient(context.Background(), server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListNamespaces(context.Background(), warehouse, nil)
	if err == nil || !strings.Contains(err.Error(), "1 namespaces") {
		t.Fatalf("wanted UUID count mismatch error, got %v", err)
	}
}

func TestListNamespacesRepeatedPageToken(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"namespaces":[],"next-page-token":"next"}`))
	}))
	defer server.Close()
	client, err := NewStaticTokenClient(context.Background(), server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListNamespaces(context.Background(), warehouse, nil)
	if err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("wanted repeated token error, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestListNamespacesWithParent(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	parent := []string{"reference", "instruments"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if want := "reference\x1finstruments"; q.Get("parent") != want {
			t.Errorf("parent=%q, want %q", q.Get("parent"), want)
		}
		_, _ = w.Write([]byte(`{"namespaces":[["reference","instruments","xrd"]],"namespace-uuids":["33333333-3333-4333-8333-333333333333"]}`))
	}))
	defer server.Close()
	client, err := NewStaticTokenClient(context.Background(), server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ListNamespaces(context.Background(), warehouse, parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || strings.Join(result[0].Parts, ".") != "reference.instruments.xrd" {
		t.Errorf("got %+v", result)
	}
}

func TestListNamespacesEmptyParentLevel(t *testing.T) {
	warehouse := "22222222-2222-4222-8222-222222222222"
	client, err := NewStaticTokenClient(context.Background(), "http://localhost", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListNamespaces(context.Background(), warehouse, []string{"a", ""})
	if err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("wanted non-empty error, got %v", err)
	}
}
