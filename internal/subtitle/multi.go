package subtitle

import (
	"context"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// MultiProvider aggregates results from multiple subtitle providers.
// It queries all providers in parallel and merges the results.
type MultiProvider struct {
	providers []Provider
	names     []string
}

// NewMultiProvider creates a multi-provider from a list of named providers.
// Provider ordering is sorted for deterministic behavior.
func NewMultiProvider(providers map[string]Provider) *MultiProvider {
	mp := &MultiProvider{}
	// Sort provider names for deterministic ordering across runs.
	for name := range providers {
		mp.names = append(mp.names, name)
	}
	// Simple insertion sort (typically 2-3 providers).
	for i := 1; i < len(mp.names); i++ {
		for j := i; j > 0 && mp.names[j] < mp.names[j-1]; j-- {
			mp.names[j], mp.names[j-1] = mp.names[j-1], mp.names[j]
		}
	}
	for _, name := range mp.names {
		mp.providers = append(mp.providers, providers[name])
	}
	return mp
}

// ProviderNames returns the names of all configured providers.
func (mp *MultiProvider) ProviderNames() []string {
	if mp == nil {
		return nil
	}
	return append([]string{}, mp.names...)
}

// Search queries all providers in parallel and merges results.
// It waits for all providers to respond (or their per-provider deadline
// to elapse), then returns the combined results. The ctx deadline caps
// the total search time across all providers.
func (mp *MultiProvider) Search(ctx context.Context, query string) ([]Result, error) {
	if len(mp.providers) == 0 {
		return nil, nil
	}

	type searchResult struct {
		results []Result
		err     error
		name    string
	}

	perProviderDeadline := 10 * time.Second

	var wg sync.WaitGroup
	ch := make(chan searchResult, len(mp.providers))

	for i, p := range mp.providers {
		wg.Add(1)
		go func(name string, p Provider) {
			defer wg.Done()
			provCtx, cancel := context.WithTimeout(ctx, perProviderDeadline)
			defer cancel()
			results, err := p.Search(provCtx, query)
			ch <- searchResult{results: results, err: err, name: name}
		}(mp.names[i], p)
	}

	// Close ch in background once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	var all []Result

	for res := range ch {
		if res.err != nil {
			pterm.Debug.Printfln("subtitle provider %s: %s", res.name, res.err)
			continue
		}
		for _, r := range res.results {
			if r.Source == "" {
				r.Source = res.name
			}
			all = append(all, r)
		}
	}

	return all, nil
}

// Page queries the provider that matches the result's source.
// If the result has no source tag, it tries all providers.
func (mp *MultiProvider) Page(ctx context.Context, sdID, slug, seasonSlug string) (*PageResult, error) {
	// Try to find the provider by source tag embedded in the ID.
	// IDs may be prefixed like "subdl:sd123" or "kitsunekko:ja:Steins_Gate".
	source, cleanID := mp.splitSourcePrefix(sdID)
	if source != "" {
		if p, ok := mp.providerByName(source); ok {
			return p.Page(ctx, cleanID, slug, seasonSlug)
		}
	}

	// No source prefix — try each provider.
	var lastErr error
	for _, p := range mp.providers {
		page, err := p.Page(ctx, sdID, slug, seasonSlug)
		if err == nil && len(page.Subtitles) > 0 {
			return page, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return &PageResult{}, nil
}

// DownloadURL returns the download URL from the matching provider.
func (mp *MultiProvider) DownloadURL(sub Subtitle) string {
	source := sub.Source
	if p, ok := mp.providerByName(source); ok {
		return p.DownloadURL(sub)
	}
	// Fallback: the Link field may already contain the full URL.
	return sub.Link
}

// providerByName finds a provider by its name.
func (mp *MultiProvider) providerByName(name string) (Provider, bool) {
	for i, n := range mp.names {
		if n == name {
			return mp.providers[i], true
		}
	}
	return nil, false
}

// splitSourcePrefix splits an ID like "subdl:sd123" into ("subdl", "sd123").
// It validates the prefix against known provider names instead of using
// a length heuristic, avoiding false positives with short or long names.
func (mp *MultiProvider) splitSourcePrefix(id string) (string, string) {
	for i, c := range id {
		if c == ':' {
			prefix := id[:i]
			// Only treat as source prefix if it matches a known provider name.
			for _, name := range mp.names {
				if name == prefix {
					return prefix, id[i+1:]
				}
			}
		}
	}
	return "", id
}
