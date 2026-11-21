//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"encoding/json"
	cr "lsp-trace/internal/censusresult"
	"os"
	"path/filepath"
	"testing"
)

func TestCensusCapacityExact1024(t *testing.T) {
	t.Setenv("DURABLE_SCALE_PADDING", "1024")
	p := &contractProcess{t: t, home: folderInventoryTemp(t), mode: "three-item"}
	var e error
	p.exe, e = os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	raw := p.call("start", nil, nil)
	var response struct {
		ScalePretaskRefusal bool
		Code                int
		Diagnostic          string
	}
	if json.Unmarshal(raw, &response) != nil || !response.ScalePretaskRefusal || response.Code != 1 {
		t.Fatalf("expected refusal, no task: %s", raw)
	}
	d, e := cr.DecodeCapacityDiagnostic([]byte(response.Diagnostic))
	// Owned fixture inputs retain paths, whose t.TempDir lengths are not fixed.
	// Do not tune the fixture to reproduce another execution's preparation total.
	m := d.Metrics
	if e != nil || d.Reason != "CAPACITY_BYTES" || m == nil || m.Unit != "bytes" || m.Protected == 0 || m.Limit != 67108864 || m.Spent == 0 || m.Spent > m.Limit || m.Available != m.Limit-m.Spent || m.Deficit != m.Protected-m.Available || m.Deficit == 0 {
		t.Fatalf("exact refusal: %v %s", e, response.Diagnostic)
	}
	heads, e := filepath.Glob(filepath.Join(p.home, "Library", "Application Support", "lsp-trace", "census", "durable", "inventory-run-*.json"))
	if e != nil || len(heads) != 0 {
		t.Fatal("refusal published durable head", heads, e)
	}
	t.Logf("CAPACITY_EXACT_1024: %s preparation occurred; no durable task/head published", response.Diagnostic)
}
