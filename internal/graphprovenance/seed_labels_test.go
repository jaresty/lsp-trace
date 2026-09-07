package graphprovenance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Label/cardinality errors already belong to graph-v3 semantic admission. Ensure
// both envelope entry points preserve that rejection rather than masking it.
func TestGraphSeedLabelJoinsRemainClosed(t *testing.T) {
	root, raw, seed := fixture(t)
	base := captured(t, root, raw, seed, nil)
	for _, label := range []string{"", "wrong"} {
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		doc["seeds"].([]any)[0].(map[string]any)["label"] = label
		bad, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Capture(context.Background(), bad, root, seed, "s", 1, nil); err == nil || !strings.Contains(err.Error(), "seed join mismatch") {
			t.Fatalf("Capture seed label join: %v", err)
		}
		e := base
		e.GraphBytes = bad
		e.GraphDigest = digest(Version+":graph", bad)
		encoded, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateFor(encoded, Family, "v1"); err == nil || !strings.Contains(err.Error(), "seed join mismatch") {
			t.Fatalf("offline seed label join: %v", err)
		}
	}
}
