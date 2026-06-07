package allanime

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/distiled/orphion/internal/provider"
)

const (
	searchQuery   = `query($search: SearchInput, $limit: Int, $page: Int, $translationType: VaildTranslationTypeEnumType, $countryOrigin: VaildCountryOriginEnumType) { shows(search: $search, limit: $limit, page: $page, translationType: $translationType, countryOrigin: $countryOrigin) { edges { _id name availableEpisodes } } }`
	episodesQuery = `query($showId: String!) { show(_id: $showId) { _id availableEpisodesDetail } }`
	streamsQuery  = `query($showId: String!, $translationType: VaildTranslationTypeEnumType!, $episodeString: String!) { episode(showId: $showId, translationType: $translationType, episodeString: $episodeString) { episodeString sourceUrls } }`
)

// Client fetches and normalizes AllAnime data.
type Client struct {
	httpClient       *http.Client
	apiURL           *url.URL
	siteURL          *url.URL
	mediaURL         *url.URL
	userAgent        string
	episodeQueryHash string
}

type graphQLRequest struct {
	Variables any    `json:"variables"`
	Query     string `json:"query"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLEnvelope[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type episodeRef struct {
	ShowID          string `json:"s"`
	TranslationType string `json:"t"`
	Number          string `json:"e"`
}

type redactedRequestError struct {
	err error
}

func (e redactedRequestError) Error() string {
	return "request failed"
}

func (e redactedRequestError) Unwrap() error {
	return e.err
}

// NewClient validates configuration and creates an AllAnime client.
func NewClient(cfg Config) (*Client, error) {
	apiURL, err := parseEndpoint("API", cfg.APIURL)
	if err != nil {
		return nil, err
	}
	siteURL, err := parseEndpoint("site", cfg.SiteURL)
	if err != nil {
		return nil, err
	}
	mediaURL, err := parseEndpoint("media", cfg.MediaURL)
	if err != nil {
		return nil, err
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.EpisodeQueryHash == "" {
		return nil, errors.New("episode query hash is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{
		httpClient:       cfg.HTTPClient,
		apiURL:           apiURL,
		siteURL:          siteURL,
		mediaURL:         mediaURL,
		userAgent:        cfg.UserAgent,
		episodeQueryHash: cfg.EpisodeQueryHash,
	}, nil
}

// Search queries AllAnime for matching shows.
func (c *Client) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	var response struct {
		Shows struct {
			Edges []struct {
				ID   string `json:"_id"`
				Name string `json:"name"`
			} `json:"edges"`
		} `json:"shows"`
	}
	variables := map[string]any{
		"search": map[string]any{
			"allowAdult":   false,
			"allowUnknown": false,
			"query":        query,
		},
		"limit":           40,
		"page":            1,
		"translationType": "sub",
		"countryOrigin":   countryOrigin(kind),
	}
	if err := c.graphQL(ctx, searchQuery, variables, &response); err != nil {
		return nil, fmt.Errorf("allanime search: %w", err)
	}
	results := make([]provider.Anime, 0, len(response.Shows.Edges))
	for _, edge := range response.Shows.Edges {
		if edge.ID == "" || edge.Name == "" {
			continue
		}
		results = append(results, provider.Anime{ID: edge.ID, Title: edge.Name})
	}
	return results, nil
}

// Episodes returns provider-ordered episodes for a show.
func (c *Client) Episodes(ctx context.Context, showID string) ([]provider.Episode, error) {
	var response struct {
		Show struct {
			Available map[string][]string `json:"availableEpisodesDetail"`
		} `json:"show"`
	}
	if err := c.graphQL(ctx, episodesQuery, map[string]any{"showId": showID}, &response); err != nil {
		return nil, fmt.Errorf("allanime episodes: %w", err)
	}
	translation := "sub"
	numbers := response.Show.Available[translation]
	if len(numbers) == 0 {
		translation = "dub"
		numbers = response.Show.Available[translation]
	}
	episodes := make([]provider.Episode, 0, len(numbers))
	for _, raw := range numbers {
		number := raw
		sortKey, err := strconv.ParseFloat(number, 64)
		if err != nil {
			continue
		}
		episodes = append(episodes, provider.Episode{
			ID: encodeEpisodeID(episodeRef{
				ShowID:          showID,
				TranslationType: translation,
				Number:          number,
			}),
			Number:  number,
			SortKey: sortKey,
			Title:   "Episode " + number,
		})
	}
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].SortKey < episodes[j].SortKey
	})
	return episodes, nil
}

// Streams resolves AllAnime source entries into downloadable media streams.
func (c *Client) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	ref, err := decodeEpisodeID(episodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid episode ID")
	}
	var response struct {
		Episode struct {
			SourceURLs []struct {
				Name string `json:"sourceName"`
				URL  string `json:"sourceUrl"`
			} `json:"sourceUrls"`
		} `json:"episode"`
	}
	variables := map[string]any{
		"showId":          ref.ShowID,
		"translationType": ref.TranslationType,
		"episodeString":   ref.Number,
	}
	persistedErr := c.persistedEpisode(ctx, variables, &response)
	if persistedErr != nil || len(response.Episode.SourceURLs) == 0 {
		if err := c.graphQL(ctx, streamsQuery, variables, &response); err != nil {
			if persistedErr != nil {
				return nil, fmt.Errorf("allanime streams: persisted query: %v; fallback: %w", persistedErr, err)
			}
			return nil, fmt.Errorf("allanime streams: %w", err)
		}
	}
	var sourceErrors []string
	for _, source := range response.Episode.SourceURLs {
		found, err := c.resolveSource(ctx, source.Name, source.URL)
		if err != nil {
			sourceErrors = append(sourceErrors, fmt.Sprintf("%s: %v", source.Name, err))
			continue
		}
		if len(found) > 0 {
			return found, nil
		}
	}
	if len(sourceErrors) > 0 {
		return nil, fmt.Errorf("allanime streams: no supported sources (%s)", strings.Join(sourceErrors, "; "))
	}
	return nil, fmt.Errorf("allanime streams: no supported sources")
}

func (c *Client) graphQL(ctx context.Context, query string, variables any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Variables: variables, Query: query})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", c.siteURL.String())
	req.Header.Set("Referer", c.siteURL.String()+"/")
	req.Header.Set("User-Agent", c.userAgent)
	return c.doGraphQL(req, out)
}

func (c *Client) persistedEpisode(ctx context.Context, variables any, out any) error {
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("encode variables: %w", err)
	}
	extensionsJSON, err := json.Marshal(map[string]any{
		"persistedQuery": map[string]any{
			"version":    1,
			"sha256Hash": c.episodeQueryHash,
		},
	})
	if err != nil {
		return fmt.Errorf("encode extensions: %w", err)
	}
	requestURL := *c.apiURL
	query := requestURL.Query()
	query.Set("variables", string(variablesJSON))
	query.Set("extensions", string(extensionsJSON))
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", c.siteURL.String())
	req.Header.Set("Referer", c.siteURL.String()+"/")
	req.Header.Set("User-Agent", c.userAgent)
	return c.doGraphQL(req, out)
}

func (c *Client) doGraphQL(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", redactedRequestError{err: err})
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	body, err = decryptResponse(body)
	if err != nil {
		return fmt.Errorf("decode protected response: %w", err)
	}
	var envelope graphQLEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			msgs = append(msgs, fmt.Sprintf("%q", e.Message))
		}
		return fmt.Errorf("upstream GraphQL returned errors: %s", strings.Join(msgs, ", "))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode direct data: %w", err)
		}
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

func (c *Client) resolveSource(ctx context.Context, sourceName, raw string) ([]provider.Stream, error) {
	if strings.HasPrefix(raw, "--") {
		decoded, err := decodeProviderPath(raw)
		if err != nil {
			return nil, err
		}
		ref, err := url.Parse(decoded)
		if err != nil {
			return nil, fmt.Errorf("parse media path: %w", err)
		}
		return c.fetchMedia(ctx, c.mediaURL.ResolveReference(ref))
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("unsupported source URL")
	}
	if strings.Contains(strings.ToLower(u.Path), ".m3u8") || sourceName == "Yt-mp4" {
		return []provider.Stream{c.stream(u.String(), "")}, nil
	}
	return nil, fmt.Errorf("unsupported source host")
}

func (c *Client) fetchMedia(ctx context.Context, mediaURL *url.URL) ([]provider.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create media request: %w", err)
	}
	req.Header.Set("Referer", c.siteURL.String()+"/")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("media request: %w", redactedRequestError{err: err})
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("media upstream status %d", resp.StatusCode)
	}
	var value any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&value); err != nil {
		return nil, fmt.Errorf("decode media response: %w", err)
	}
	var streams []provider.Stream
	collectStreams(value, func(rawURL, quality string) {
		u, err := url.Parse(rawURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return
		}
		streams = append(streams, c.stream(u.String(), quality))
	})
	if len(streams) == 0 {
		return nil, fmt.Errorf("media response contains no streams")
	}
	return streams, nil
}

func (c *Client) stream(rawURL, quality string) provider.Stream {
	quality = strings.TrimSpace(quality)
	if quality != "" && !strings.HasSuffix(strings.ToLower(quality), "p") {
		quality += "p"
	}
	headers := make(http.Header)
	headers.Set("Referer", c.siteURL.String())
	headers.Set("User-Agent", c.userAgent)
	return provider.Stream{URL: rawURL, Quality: quality, Headers: headers}
}

func collectStreams(value any, add func(string, string)) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectStreams(item, add)
		}
	case map[string]any:
		rawURL, _ := typed["link"].(string)
		if rawURL == "" {
			rawURL, _ = typed["url"].(string)
		}
		quality, _ := typed["resolutionStr"].(string)
		if quality == "" {
			switch height := typed["height"].(type) {
			case float64:
				quality = strconv.Itoa(int(height))
			case string:
				quality = height
			}
		}
		if rawURL != "" {
			add(rawURL, quality)
		}
		for _, child := range typed {
			if _, ok := child.(map[string]any); ok {
				collectStreams(child, add)
			}
			if _, ok := child.([]any); ok {
				collectStreams(child, add)
			}
		}
	}
}

func encodeEpisodeID(ref episodeRef) string {
	data, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeEpisodeID(raw string) (episodeRef, error) {
	var ref episodeRef
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return ref, err
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		return ref, err
	}
	if ref.ShowID == "" || ref.TranslationType == "" || ref.Number == "" {
		return ref, errors.New("incomplete episode ID")
	}
	return ref, nil
}

func decodeProviderPath(raw string) (string, error) {
	if !strings.HasPrefix(raw, "--") {
		return raw, nil
	}
	encoded := strings.TrimPrefix(raw, "--")
	if len(encoded)%2 != 0 {
		return "", errors.New("invalid encoded source path")
	}
	var decoded strings.Builder
	for i := 0; i < len(encoded); i += 2 {
		value, err := strconv.ParseUint(encoded[i:i+2], 16, 8)
		if err != nil {
			return "", errors.New("invalid encoded source path")
		}
		decoded.WriteByte(byte(value) ^ 0x38)
	}
	result := decoded.String()
	if !strings.HasPrefix(result, "/") || strings.HasPrefix(result, "//") {
		return "", errors.New("decoded source is not a relative path")
	}
	return strings.Replace(result, "/clock", "/clock.json", 1), nil
}

func decryptResponse(body []byte) ([]byte, error) {
	var wrapper struct {
		ToBeParsed string `json:"tobeparsed"`
		Data       struct {
			ToBeParsed string `json:"tobeparsed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("decode protected wrapper: %w", err)
	}
	protected := wrapper.ToBeParsed
	if protected == "" {
		protected = wrapper.Data.ToBeParsed
	}
	if protected == "" {
		return body, nil
	}
	blob, err := base64.StdEncoding.DecodeString(protected)
	if err != nil {
		return nil, err
	}
	if len(blob) <= 29 {
		return nil, errors.New("protected response is too short")
	}
	key := sha256.Sum256([]byte("Xot36i3lK3:v1"))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	iv := make([]byte, aes.BlockSize)
	copy(iv, blob[1:13])
	iv[15] = 2
	ciphertext := blob[13 : len(blob)-16]
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

// countryOrigin maps the content kind to an AllAnime country origin filter.
func countryOrigin(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "anime":
		return "JP"
	case "drama":
		return "JP"
	default:
		return "ALL"
	}
}
