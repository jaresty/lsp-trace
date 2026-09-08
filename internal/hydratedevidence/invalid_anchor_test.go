package hydratedevidence

import "testing"

func TestInvalidNativeAnchorNeverUpgraded(t *testing.T) {
	a := admitted{sourceMap: map[string]Source{"s": {ID: "s", State: "RETAINED_BYTES", SourceEncoding: "utf-8"}}, bodies: map[string][]byte{"s": []byte("abc")}, recordMap: map[string]Record{"r": {ID: "r", SourceIDs: []string{"s"}, Encoding: "utf-8", Range: &Range{}, AnchorStatus: "INVALID_COORDINATES"}}}
	p := DefaultPolicy()
	p.IncludeBodies = true
	r := Request{Policy: p, Selections: []Selection{{ID: "one", SourceID: "s", RecordID: "r", Mode: "RETAINED_RANGE"}}}
	got, _ := selected(a, r)
	if got[0].Status != "INVALID_COORDINATES" {
		t.Fatalf("invalid retained anchor upgraded: %s", got[0].Status)
	}
}
