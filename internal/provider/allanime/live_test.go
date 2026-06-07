package allanime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

func TestLiveProviderShirokumaCafe(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set ORPHION_LIVE_PROVIDER_TEST=1 to contact the live provider")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "Shirokuma Cafe", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("live search returned no results")
	}

	episodes, err := client.Episodes(ctx, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) == 0 {
		t.Fatal("live episode lookup returned no episodes")
	}

	streams, err := client.Streams(ctx, episodes[0].ID)
	if err != nil {
		var payload map[string]json.RawMessage
		ref, decodeErr := decodeEpisodeID(episodes[0].ID)
		if decodeErr == nil {
			variables := map[string]any{
				"showId":          ref.ShowID,
				"translationType": ref.TranslationType,
				"episodeString":   ref.Number,
			}
			if requestErr := client.graphQL(ctx, streamsQuery, variables, &payload); requestErr == nil {
				t.Logf("stream payload shape: %v", jsonShape(payload))
			}
		}
		t.Fatal(err)
	}
	if len(streams) == 0 {
		t.Fatal("live stream lookup returned no streams")
	}
}

func jsonShape(value map[string]json.RawMessage) map[string]string {
	shape := make(map[string]string, len(value))
	for key, raw := range value {
		var child map[string]json.RawMessage
		if json.Unmarshal(raw, &child) == nil {
			keys := make([]string, 0, len(child))
			for childKey := range child {
				keys = append(keys, childKey)
			}
			sort.Strings(keys)
			shape[key] = fmt.Sprintf("object%v", keys)
			continue
		}
		var list []map[string]json.RawMessage
		if json.Unmarshal(raw, &list) == nil {
			keys := []string{}
			if len(list) > 0 {
				for childKey := range list[0] {
					keys = append(keys, childKey)
				}
				sort.Strings(keys)
			}
			shape[key] = fmt.Sprintf("array(len=%d, keys=%v)", len(list), keys)
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			shape[key] = "string"
			continue
		}
		shape[key] = "scalar"
	}
	return shape
}
