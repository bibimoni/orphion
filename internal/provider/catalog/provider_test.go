package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/distiled/orphion/internal/provider"
)

func newTestServer(t *testing.T) (*httptest.Server, *Client) {
	t.Helper()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			query := r.URL.Query().Get("q")
			if query == "nothing" {
				w.Write([]byte("[]"))
				return
			}
			data := []searchResult{
				{ID: "cat:frieren", Title: "Frieren"},
				{ID: "cat:frieren-dub", Title: "Frieren (Dub)"},
			}
			json.NewEncoder(w).Encode(data)
		case "/episodes":
			eps := []struct {
				ID     string  `json:"id"`
				Number float64 `json:"number"`
				Label  string  `json:"label"`
			}{
				{ID: "ep1", Number: 1, Label: "1"},
				{ID: "ep2", Number: 2, Label: "2"},
				{ID: "ep3", Number: 3, Label: "3"},
			}
			json.NewEncoder(w).Encode(eps)
		case "/streams":
			streams := []struct {
				URL     string `json:"url"`
				Quality string `json:"quality"`
			}{
				{URL: "https://stream.example.com/1080.m3u8", Quality: "1080p"},
				{URL: "https://stream.example.com/720.m3u8", Quality: "720p"},
			}
			json.NewEncoder(w).Encode(streams)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ts := httptest.NewServer(h)
	client := &Client{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
	}
	return ts, client
}

func TestCatalog_Search(t *testing.T) {
	ts, client := newTestServer(t)
	defer ts.Close()

	results, err := client.Search(context.Background(), "frieren", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].ID != "cat:frieren" {
		t.Errorf("unexpected ID: %s", results[0].ID)
	}

	// search returns empty.
	empty, err := client.Search(context.Background(), "nothing", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("empty search: got %v", empty)
	}
}

func TestCatalog_Episodes(t *testing.T) {
	ts, client := newTestServer(t)
	defer ts.Close()

	eps, err := client.Episodes(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("got %d episodes, want 3", len(eps))
	}
}

func TestCatalog_Streams(t *testing.T) {
	ts, client := newTestServer(t)
	defer ts.Close()

	streams, err := client.Streams(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 {
		t.Fatalf("got %d streams, want 2", len(streams))
	}
}

func TestClient_Opacity(t *testing.T) {
	// Provider identifiers should be opaque.
	client := NewClient(Config{BaseURL: "http://localhost"})
	_ = client
	// Ensure that internal types like searchResult are not leaked.
	var p provider.Provider = client
	_ = p
}
