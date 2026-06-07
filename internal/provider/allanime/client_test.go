package allanime

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.APIURL = "https://api.example.test/api"
	cfg.SiteURL = "https://site.example.test"
	cfg.MediaURL = "https://media.example.test"
	cfg.HTTPClient = &http.Client{Transport: transport}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestSearchUsesGraphQLAndMapsShows(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.String() != "https://api.example.test/api" {
			t.Fatalf("URL = %s", req.URL)
		}
		if req.Header.Get("Origin") != "https://site.example.test" {
			t.Fatalf("Origin = %q", req.Header.Get("Origin"))
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"query":"Shirokuma Cafe"`) {
			t.Fatalf("request body does not contain search query: %s", body)
		}
		return jsonResponse(http.StatusOK, `{
			"data":{"shows":{"edges":[
				{"_id":"show-1","name":"Shirokuma Cafe","availableEpisodes":{"sub":50}},
				{"_id":"show-2","name":"Shirokuma Cafe (Dub)","availableEpisodes":{"dub":50}}
			]}}
		}`), nil
	})

	got, err := client.Search(context.Background(), "Shirokuma Cafe", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "show-1" || got[0].Title != "Shirokuma Cafe" {
		t.Fatalf("Search() = %#v", got)
	}
}

func TestEpisodesCreatesOpaqueStreamIdentifiers(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"data":{"show":{"_id":"show-1","availableEpisodesDetail":{"sub":["2","1","12.5"]}}}
		}`), nil
	})

	got, err := client.Episodes(context.Background(), "show-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(Episodes()) = %d", len(got))
	}
	if got[0].Number != "1" || got[1].Number != "2" || got[2].Number != "12.5" {
		t.Fatalf("Episodes() order = %#v", got)
	}
	if strings.Contains(got[0].ID, "show-1") || strings.Contains(got[0].ID, `"`) {
		t.Fatalf("episode ID exposes provider structure: %q", got[0].ID)
	}
}

func TestStreamsDecodesProviderPathAndMapsMedia(t *testing.T) {
	decoded, err := decodeProviderPath("--175948514e4c4f571751")
	if err != nil || decoded != "/apivtwo/i" {
		t.Fatalf("decodeProviderPath() = %q, %v", decoded, err)
	}

	requests := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("stream lookup method = %s", req.Method)
			}
			if req.URL.Query().Get("extensions") == "" {
				t.Fatal("stream lookup is missing persisted query extensions")
			}
			return jsonResponse(http.StatusOK, `{
				"data":{"episode":{"episodeString":"1","sourceUrls":[
					{"sourceName":"Default","sourceUrl":"--175948514e4c4f571751"}
				]}}
			}`), nil
		case 2:
			if req.URL.Host != "media.example.test" {
				t.Fatalf("media host = %q", req.URL.Host)
			}
			return jsonResponse(http.StatusOK, `[
				{"link":"https://cdn.example.test/1080.m3u8","resolutionStr":"1080"},
				{"link":"https://cdn.example.test/720.m3u8","resolutionStr":"720"}
			]`), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{ShowID: "show-1", TranslationType: "sub", Number: "1"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Streams()) = %d", len(got))
	}
	if got[0].Quality != "1080p" || got[0].Headers.Get("Referer") != "https://site.example.test" {
		t.Fatalf("Streams()[0] = %#v", got[0])
	}
}

func TestGraphQLErrorsDoNotLeakRequestData(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"errors":[{"message":"upstream rejected query"}]}`), nil
	})

	_, err := client.Search(context.Background(), "secret query", "anime")
	if err == nil {
		t.Fatal("Search() error = nil")
	}
	if strings.Contains(err.Error(), "secret query") {
		t.Fatalf("error leaks query: %v", err)
	}
}

func TestHTTPStatusAndCancellation(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		client := testClient(t, func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadGateway, `signed=https://secret.example/token`), nil
		})
		_, err := client.Search(context.Background(), "query", "anime")
		if err == nil || strings.Contains(err.Error(), "signed=") {
			t.Fatalf("Search() error = %v", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		client := testClient(t, func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Search(ctx, "query", "anime")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Search() error = %v", err)
		}
	})
}

func TestMediaRequestErrorsRedactURL(t *testing.T) {
	requests := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(http.StatusOK, `{
				"data":{"episode":{"episodeString":"1","sourceUrls":[
					{"sourceName":"Default","sourceUrl":"--175948514e4c4f571751"}
				]}}
			}`), nil
		}
		return nil, errors.New(`Get "https://media.example.test/path?signed=secret": timeout`)
	})
	episodeID := encodeEpisodeID(episodeRef{ShowID: "show-1", TranslationType: "sub", Number: "1"})

	_, err := client.Streams(context.Background(), episodeID)
	if err == nil {
		t.Fatal("Streams() error = nil")
	}
	if strings.Contains(err.Error(), "signed=") || strings.Contains(err.Error(), "media.example.test") {
		t.Fatalf("Streams() leaked media URL: %v", err)
	}
}

func TestDecryptResponseAcceptsNestedProtectedPayload(t *testing.T) {
	plaintext := []byte(`{"data":{"episode":{"episodeString":"1","sourceUrls":[]}}}`)
	body := protectedResponse(t, plaintext)

	got, err := decryptResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("decryptResponse() = %s", got)
	}
}

func TestGraphQLAcceptsProtectedDirectData(t *testing.T) {
	plaintext := []byte(`{"episode":{"episodeString":"1","sourceUrls":[{"sourceName":"Direct","sourceUrl":"https://cdn.example.test/video.m3u8"}]}}`)
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, string(protectedResponse(t, plaintext))), nil
	})
	episodeID := encodeEpisodeID(episodeRef{ShowID: "show-1", TranslationType: "sub", Number: "1"})

	streams, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].URL != "https://cdn.example.test/video.m3u8" {
		t.Fatalf("Streams() = %#v", streams)
	}
}

func TestStreamsAcceptsYtMP4DirectURL(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"data":{"episode":{"episodeString":"1","sourceUrls":[
				{"sourceName":"Yt-mp4","sourceUrl":"https://tools.fast4speed.rsvp/media-id"}
			]}}
		}`), nil
	})
	episodeID := encodeEpisodeID(episodeRef{ShowID: "show-1", TranslationType: "sub", Number: "1"})

	streams, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].URL != "https://tools.fast4speed.rsvp/media-id" {
		t.Fatalf("Streams() = %#v", streams)
	}
}

func protectedResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()
	key := sha256.Sum256([]byte("Xot36i3lK3:v1"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	iv := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 0, 0, 0, 2}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCTR(block, iv).XORKeyStream(ciphertext, plaintext)
	blob := append([]byte{0}, iv[:12]...)
	blob = append(blob, ciphertext...)
	blob = append(blob, make([]byte, 16)...)
	body, err := json.Marshal(map[string]any{
		"errors": []any{map[string]any{"message": "expected upstream error"}},
		"data": map[string]any{
			"tobeparsed": base64.StdEncoding.EncodeToString(blob),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
