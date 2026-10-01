package adr0011genericv5proposal

import (
	"encoding/json"
	"testing"
)

func TestB4bVerifiedSchemeOnly(t *testing.T) {
	held := B4aInput{URI: "file:///w/a.go"}
	if held.Language != nil {
		t.Fatal("held language must be absent")
	}
	for _, tc := range []struct{ name, selector, want string }{
		{"file only", `[{"scheme":"file"}]`, "SUPPORTED"},
		{"different scheme", `[{"scheme":"untitled"}]`, "UNSUPPORTED"},
		{"unheld language", `[{"scheme":"file","language":"go"}]`, "UNKNOWN"},
		{"wrong scheme type", `[{"scheme":3}]`, "MALFORMED"},
		{"absent selector", ``, "UNKNOWN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := B4bVerifiedSelector(held, json.RawMessage(tc.selector))
			if got != tc.want {
				t.Fatalf("selector %s: got %s, want %s", tc.selector, got, tc.want)
			}
		})
	}
}
