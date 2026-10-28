//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// Small fake-wire fixture qualification only, not a 25-target reconstruction of
// either spent run. Successful admissions can themselves be observed events.
func TestScopedTerminalDiagnosticsRetainedPrivate(t *testing.T) {
	for _, fault := range []string{"rehearsal-empty-directions", "rehearsal-external", "malformed", "poison", "preconstruction"} {
		t.Run(fault, func(t *testing.T) {
			h, m, cfg, f := pilot.NewScopedRehearsalFixtureV2Private(t)
			f.Fault(fault)
			if fault == "preconstruction" {
				// Only this test's owned disposable workspace, not retained inputs.
				changed := 0
				e := filepath.WalkDir(cfg.Workspace, func(path string, entry fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if !entry.IsDir() && filepath.Ext(path) == ".go" {
						if e := os.WriteFile(path, []byte("package drift\n"), 0600); e != nil {
							return e
						}
						changed++
					}
					return nil
				})
				if e != nil || changed == 0 {
					t.Fatal("owned supply drift fixture", e)
				}
			}
			report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
			if report.Schema != "lsp-trace.scoped-rehearsal-observation.private.v3" || report.Diagnostics == nil || report.Diagnostics.Available != (fault != "preconstruction") || report.Diagnostics.Overflow {
				t.Fatalf("snapshot: %+v", report)
			}
			if fault == "rehearsal-empty-directions" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || report.Failure == "" || report.Outcome != "INDETERMINATE" {
				t.Fatalf("public failure lost: %+v %v", report, err)
			}
			if fault == "malformed" {
				// Method-result validation need not generate a guard refusal. An
				// available snapshot without a refusal does not exonerate a result.
				for _, event := range report.Diagnostics.Events {
					if event.Reason != "ACCEPTED" {
						t.Fatal("unexpected guard refusal", event)
					}
				}
			}
			if fault == "rehearsal-external" {
				found := false
				for _, e := range report.Diagnostics.Events {
					if e.Reason == "URI_OUTSIDE_FROZEN" && e.URIClass == "FROZEN_EXTERNAL" {
						found = true
					}
				}
				if !found || !strings.Contains(report.Failure, "acquire batch 0: DOCUMENT_SUPPLY_UNAVAILABLE") {
					t.Fatalf("external witness=%v public failure=%v", found, err)
				}
			}
			raw, e := json.Marshal(report.Diagnostics)
			if e != nil || len(raw) > 8192 || len(report.Diagnostics.Events) > 64 || bytes.Contains(raw, []byte("file:")) {
				t.Fatalf("bounded closed snapshot: %s %v", raw, e)
			}
			// Observation identity may outlive the lease until generation reset;
			// mutation of a later accessor result must not alias the report.
			later, _, _ := h.Diagnostics()
			if len(later) > 0 {
				later[0].Reason = "tampered"
			}
			if report.OutcomeReceipt == nil || !report.OutcomeReceipt.Verified || len(report.Records)+1 > 8 {
				t.Fatal("outcome pair/object budget", report)
			}
			total := report.OutcomeReceipt.Bytes
			for _, record := range report.Records {
				total += record.Bytes
			}
			if total > cfg.Budget.TotalBytes || cfg.Budget.Objects != 8 {
				t.Fatal("effective output budget changed")
			}
			var primary []byte
			for _, root := range cfg.OutputRoots[:2] {
				raw, e := os.ReadFile(filepath.Join(root, report.OutcomeReceipt.ID))
				if e != nil || pilot.Digest(raw) != report.OutcomeReceipt.ID {
					t.Fatal("outcome hash", e)
				}
				if primary != nil && !bytes.Equal(primary, raw) {
					t.Fatal("pair mismatch")
				}
				primary = raw
				var retained scopedLiveReportPrivate
				if json.Unmarshal(raw, &retained) != nil || retained.Diagnostics == nil || retained.Diagnostics.Available != report.Diagnostics.Available || retained.Schema != report.Schema || retained.Failure != report.Failure {
					t.Fatal("terminal snapshot missing", string(raw))
				}
			}
			// The report still owns its snapshot after capability reset and store close.
			again, _ := json.Marshal(report.Diagnostics)
			if !bytes.Equal(again, raw) {
				t.Fatal("detached snapshot changed")
			}
		})
	}
}

func TestScopedTerminalDiagnosticsUnavailablePrivate(t *testing.T) {
	h, m, cfg, _ := pilot.NewScopedRehearsalFixtureV2Private(t)
	if events, overflow, available := h.Diagnostics(); available || overflow || len(events) != 0 {
		t.Fatal("preconstruction available")
	}
	cfg.CodeHEAD = strings.Repeat("b", 40)
	report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
	if err == nil || report.Diagnostics == nil || report.Diagnostics.Available || len(report.Diagnostics.Events) != 0 || report.Diagnostics.Overflow || len(report.Records) != 0 {
		t.Fatal("configuration failure availability", report, err)
	}
}

func TestScopedTerminalV1EncodingPrivate(t *testing.T) {
	h, m, cfg, f := pilot.NewScopedRehearsalFixturePrivate(t)
	f.Fault("rehearsal-empty-directions")
	report, err := executeScopedRehearsalPrivate(context.Background(), h, m, cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report)
	if err != nil || report.Schema != "lsp-trace.scoped-rehearsal-observation.private.v1" || report.Diagnostics != nil || bytes.Contains(raw, []byte(`"diagnostics"`)) || bytes.Contains(raw, []byte(`"discovery_policy"`)) {
		t.Fatal("v1 encoding changed", string(raw), err)
	}
	// Compare byte-for-byte with the pre-diagnostics fieldset, preserving field
	// order, tags, values, and nil-vs-empty distinctions.
	v := reflect.ValueOf(report)
	var legacyFields []reflect.StructField
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if field.Name != "Diagnostics" {
			legacyFields = append(legacyFields, field)
		}
	}
	legacy := reflect.New(reflect.StructOf(legacyFields)).Elem()
	for i := 0; i < legacy.NumField(); i++ {
		legacy.Field(i).Set(v.FieldByName(legacy.Type().Field(i).Name))
	}
	previous, e := json.Marshal(legacy.Interface())
	if e != nil || !bytes.Equal(previous, raw) {
		t.Fatal("v1 bytes changed", e)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("report")
	}
	if _, ok := fields["diagnostics"]; ok {
		t.Fatal("v1 additive field")
	}
}
