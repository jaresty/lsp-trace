package graphprovenance

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/schema"
	"strings"
	"testing"
)

func TestRepairExternalCoordinateProperties(t *testing.T) {
	count := 0
	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			for c := 0; c < 8; c++ {
				for d := 0; d < 8; d++ {
					raw := fmt.Sprintf(`{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}}`, a, b, c, d)
					dec := json.NewDecoder(strings.NewReader(raw))
					dec.UseNumber()
					var value any
					if err := dec.Decode(&value); err != nil {
						t.Fatal(err)
					}
					want := a < c || a == c && b <= d
					if validRangeV2(value, nil) != want {
						t.Fatalf("range %s", raw)
					}
					count++
				}
			}
		}
	}
	for _, bad := range []string{`null`, `{}`, `{"line":0}`, `{"line":0,"character":null}`, `{"line":0,"character":true}`, `{"line":-1,"character":0}`, `{"line":4294967296,"character":0}`, `{"line":1e0,"character":0}`, `{"line":0,"character":1.5}`} {
		dec := json.NewDecoder(strings.NewReader(bad))
		dec.UseNumber()
		var value any
		_ = dec.Decode(&value)
		if _, ok := positionV2(value); ok {
			t.Fatal("invalid position", bad)
		}
	}
	t.Logf("%d independently ordered range controls and 9 malformed positions", count)
}

func TestRepairExternalEveryStatus(t *testing.T) {
	raw, e := reviewEnvelope(t)
	n := 0
	for i, b := range e.Bindings {
		for _, status := range []string{"", "SOURCE_REFERENCE", "VALID_COORDINATES", "INVALID_COORDINATES", "NON_SOURCE"} {
			if status == b.AnchorStatus {
				continue
			}
			e.Bindings[i].AnchorStatus = status
			if err := ValidateV2(e); err == nil {
				t.Fatalf("substitution admitted %s %s", b.Pointer, status)
			}
			e.Bindings[i].AnchorStatus = b.AnchorStatus
			n++
		}
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	for _, row := range doc["bindings"].([]any) {
		delete(row.(map[string]any), "anchor_status")
	}
	missing, _ := json.Marshal(doc)
	if _, err := schema.ValidateStructure(missing, Family, "v2"); err == nil {
		t.Fatal("schema admitted omitted statuses")
	}
	if _, err := ValidateFor(missing, Family, "v2"); err == nil {
		t.Fatal("semantics admitted omitted statuses")
	}
	t.Logf("%d individual substitutions rejected; coherent all-status omission rejected by schema and semantics", n)
}

func TestRepairExternalSelectorContexts(t *testing.T) {
	for _, enc := range []string{"utf-8", "utf-16", "utf-32"} {
		for _, gen := range []uint64{1, 2, 999} {
			t.Run(fmt.Sprintf("%s/%d", enc, gen), func(t *testing.T) {
				r, root := coordinatorV2Fixture(t, func(r *acquisition.Request) {
					r.Context.PositionEncoding = enc
					r.Context.Generation = gen
					r.Limits.MaxRequests = 0
					x := ^uint32(0)
					r.Root.Locator.Line = &x
					r.Root.Locator.Character = &x
				})
				raw, err := CaptureV2(context.Background(), r, root)
				if err != nil {
					t.Fatal(err)
				}
				var e EvidenceV2
				_ = json.Unmarshal(raw, &e)
				by := map[string]BindingV2{}
				for _, b := range e.Bindings {
					by[b.Pointer] = b
				}
				for _, p := range []string{"/acquisition/request/root/locator", "/acquisition/request/required_targets/0/locator", "/acquisition/targets/0/requested/locator", "/acquisition/targets/1/requested/locator"} {
					if by[p].AnchorStatus != "VALID_COORDINATES" {
						t.Fatal("typed uint32 selector missing", p, by[p])
					}
				}
				if _, err = ValidateFor(raw, Family, "v2"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestRepairExternalLanguageVariants(t *testing.T) {
	for _, language := range []string{"typescript", "GO", " Go ", ""} {
		t.Run(fmt.Sprintf("mismatch=%q", language), func(t *testing.T) {
			_, e := reviewEnvelope(t)
			for i := range e.Acquisition.Supplies {
				e.Acquisition.Supplies[i].LanguageID = language
				e.Supplies[i].Observation.LanguageID = language
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
			if err := bindV2(&e); err != nil {
				t.Fatal(err)
			}
			if err := acquisition.ValidateResult(e.Acquisition); err != nil {
				t.Fatal("attack setup", err)
			}
			bad, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ValidateFor(bad, Family, "v2"); err == nil || !strings.Contains(err.Error(), "didOpen language") {
				t.Fatal("coherent didOpen mismatch", err)
			}
		})
	}
}
