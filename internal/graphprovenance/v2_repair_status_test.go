package graphprovenance

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestV2RepairCoordinatePresence(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}`, true},
		{`{"start":{"line":0},"end":{"line":0,"character":0}}`, false},
		{`{"start":null,"end":{"line":0,"character":0}}`, false},
		{`{"start":{"line":"0","character":0},"end":{"line":0,"character":0}}`, false},
		{`{"start":{"line":0,"character":0},"end":{"line":0,"character":-1}}`, false},
		{`{"start":{"line":0,"character":0},"end":{"line":4294967296,"character":0}}`, false},
		{`{"start":{"line":0,"character":2},"end":{"line":0,"character":1}}`, false},
	} {
		var value any
		d := json.NewDecoder(strings.NewReader(tc.raw))
		d.UseNumber()
		if err := d.Decode(&value); err != nil {
			t.Fatal(err)
		}
		if validRangeV2(value, nil) != tc.valid {
			t.Fatal("coordinate presence/type/order", tc.raw)
		}
	}
}

func TestV2RepairStatusConsistency(t *testing.T) {
	raw, e := reviewEnvelope(t)
	for _, attr := range []string{"SOURCE", "NON_SOURCE"} {
		for _, status := range []string{"", "SOURCE_REFERENCE", "NON_SOURCE", "VALID_COORDINATES", "INVALID_COORDINATES"} {
			t.Run(attr+"/"+status, func(t *testing.T) {
				var copy EvidenceV2
				_ = json.Unmarshal(raw, &copy)
				found := false
				for i, b := range e.Bindings {
					if b.Attribution == attr && b.AnchorStatus != status {
						copy.Bindings[i].AnchorStatus = status
						found = true
						break
					}
				}
				if !found {
					if attr != "NON_SOURCE" || status != "NON_SOURCE" {
						t.Fatal("no distinct binding status fixture")
					}
					if _, err := ValidateFor(raw, Family, "v2"); err != nil {
						t.Fatal("unchanged non-source control", err)
					}
					return
				}
				bad, err := json.Marshal(copy)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = ValidateFor(bad, Family, "v2"); err == nil {
					t.Fatal("status contradiction accepted")
				}
			})
		}
	}
}

func TestV2RepairDidChangeWithoutAvailableOpen(t *testing.T) {
	for _, available := range []bool{true, false} {
		t.Run(map[bool]string{true: "normalized_language", false: "language_unavailable"}[available], func(t *testing.T) {
			_, e := reviewEnvelope(t)
			// Cached first supply and later didChange; no retained didOpen exists.
			e.Acquisition.Supplies[0].Observation = nil
			if !available {
				for i := range e.Acquisition.Supplies {
					e.Acquisition.Supplies[i].LanguageID = ""
				}
			}
			n := 0
			for i := range e.Acquisition.Requests {
				q := &e.Acquisition.Requests[i]
				if q.Method == "source/prepareDocument" {
					q.Response, _ = json.Marshal(e.Acquisition.Supplies[n])
					n++
				}
			}
			reviewAccounting(&e)
			// A fresh post-traversal capture uses the same admitted descriptor.
			workspace, err := url.Parse(e.WorkspaceURI)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := CaptureV2(context.Background(), e.Acquisition, workspace.Path)
			if err != nil {
				t.Fatal("invented unavailable language comparison", err)
			}
			if _, err = ValidateFor(raw, Family, "v2"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
