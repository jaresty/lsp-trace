//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestVariableCallerV2AcquisitionOrderedDuplicates(t *testing.T) {
	// Detached scalar observation transport only: this creates no original
	// capture, admission, or retention authority. Equal observations must remain
	// separate ordinal records, not be normalized into a set.
	a := variableAcquisitionObservation{Batches: []variableCaptureObservation{
		{Ordinal: 0, Disposition: "SUCCESS_OBSERVED", ArtifactSHA256: "same-observation", ArtifactBytes: 1},
		{Ordinal: 1, Disposition: "SUCCESS_OBSERVED", ArtifactSHA256: "same-observation", ArtifactBytes: 1},
	}}
	raw, err := json.Marshal(a)
	var b variableAcquisitionObservation
	if err != nil || json.Unmarshal(raw, &b) != nil || !reflect.DeepEqual(a, b) || len(b.Batches) != 2 {
		t.Fatal("ASSERT_ACQUISITION_ORDERED_DUPLICATES")
	}
}

func TestVariableCallerV2AcquisitionStatus(t *testing.T) {
	for _, kind := range []string{"", "shutdown-wrong-id", "malformed"} {
		t.Run("capture-"+kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 2, 0)
			factory, _, _, _ := variableOwnedFactory(t, kind)
			r, err := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
			raw, _ := json.Marshal(r)
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(raw, &fields)
			var a struct {
				DiscoveryComplete, Complete                           bool
				ExpectedBatches, AttemptedCaptures, CompletedCaptures int
				Batches                                               []struct {
					Ordinal                     int
					Disposition, ArtifactSHA256 string
					ArtifactBytes               uint64
				}
			}
			if len(fields["Acquisition"]) == 0 {
				t.Fatal("ASSERT_ACQUISITION_STATUS_PRESENT")
			}
			if e := json.Unmarshal(fields["Acquisition"], &a); e != nil {
				t.Fatal(e)
			}
			if kind == "malformed" {
				if err == nil || a.DiscoveryComplete || a.Complete || a.AttemptedCaptures != 0 {
					t.Fatal("ASSERT_DISCOVERY_FAILURE_NOT_COMPLETE")
				}
			} else {
				if !a.DiscoveryComplete || !a.Complete || a.ExpectedBatches != 2 || a.AttemptedCaptures != 2 || a.CompletedCaptures != 2 || len(a.Batches) != 2 {
					t.Fatal("ASSERT_ACQUISITION_COMPLETE_OBSERVED")
				}
				for i, b := range a.Batches {
					if b.Ordinal != i || b.Disposition != "SUCCESS_OBSERVED" || len(b.ArtifactSHA256) == 0 || b.ArtifactBytes == 0 {
						t.Fatal("ASSERT_ACQUISITION_ORIGINAL_ORDER")
					}
				}
			}
			if kind == "shutdown-wrong-id" {
				if err == nil || r.Phase != "FAILED_SPENT" || r.Outcome != nil {
					t.Fatal("ASSERT_COMPLETE_CAPTURE_CLEANUP_REFUSED")
				}
				for _, p := range append(c.ProducerOutputs[:], c.PostOutputs[:]...) {
					entries, e := os.ReadDir(p)
					if e != nil || len(entries) != 0 {
						t.Fatal("ASSERT_COMPLETE_CAPTURE_OUTPUTS_EMPTY")
					}
				}
			}
			saved, e := os.ReadFile(c.ReportPath)
			if e != nil || string(saved) != string(raw) {
				t.Fatal("ASSERT_ACQUISITION_FINAL_IMMUTABLE")
			}
			t.Logf("ASSERT_ACQUISITION_STATUS_PRESENT[%s]: PASS", kind)
		})
	}
}
