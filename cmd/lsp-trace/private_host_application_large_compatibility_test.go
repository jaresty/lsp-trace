//go:build darwin

package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
)

func compatibilityLargeFixture(t *testing.T, compute uint64) (hostApplicationRequest, string, string) {
	t.Helper()
	r, path, base := compatibilityOneFile(t)
	file := filepath.Join(r.Configuration.SourceRoot, "twenty.go")
	body, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	// Every range in the controlled incoming response cites one of these actual
	// distinct call sites. Neither journal padding nor historical source is used.
	body = append(body, []byte("func manyReferences() {\n"+strings.Repeat("\tOwned00()\n", 150)+"}\n")...)
	if e = os.WriteFile(file, body, 0600); e != nil {
		t.Fatal(e)
	}
	r = compatibilityEnvelope(t, r, 4)
	r.Template.ComputationUnitBytes = compute // explicit TEST opt-in, not a default
	r.Template.ApplicationAllowance.LogicalBytes = 96 << 30
	r.Template.ApplicationAllowance.ReadHashBytes = 96 << 30
	r.Template.Configuration.Exchange = r.Template.ApplicationAllowance
	r.Template.Policy.Planning.MaxBatchTargets = 20
	for n, arg := range r.Template.CensusArguments {
		if arg == "--batch-targets" {
			r.Template.CensusArguments[n+1] = "20"
		}
	}
	r.MaxUnitsPerRun = 1
	writeApplicationFixture(t, r, path)
	return r, path, base
}

func TestHostApplicationCompatibilityLargeNativeArtifact(t *testing.T) {
	r, path, base := compatibilityLargeFixture(t, 1<<20)
	tool := buildApplicationTool(t, base)
	report, runErr := controlledApplicationRun(t, tool, path, "LSP_HOST_APPLICATION_LARGE_REFERENCES=1", "TEST_COUNT_PHYSICAL=1")
	type segment struct {
		offset uint64
		raw    []byte
	}
	var segments []segment
	var nativeArtifactBytes uint64
	// The actual format is length-prefixed header + a bounded payload; physical
	// segmentation must not be mistaken for a small logical computation object.
	walkErr := filepath.WalkDir(filepath.Join(r.Configuration.Store, "units"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.Contains(p, string(filepath.Separator)+"raw-0"+string(filepath.Separator)) {
			return nil
		}
		f, e := os.Open(p)
		if e != nil {
			return e
		}
		defer f.Close()
		prefix := make([]byte, 4)
		if _, e = io.ReadFull(f, prefix); e != nil {
			return nil
		}
		n := binary.BigEndian.Uint32(prefix)
		if n > 4096 {
			return nil
		}
		header := make([]byte, n)
		if _, e = io.ReadFull(f, header); e != nil {
			return nil
		}
		var h struct {
			Role   string `json:"role"`
			Length uint64 `json:"object_byte_length"`
			Offset uint64 `json:"offset"`
		}
		if json.Unmarshal(header, &h) != nil || h.Role != "SOURCE_ARTIFACT_PRIVATE_V2" {
			return nil
		}
		if h.Length > 1<<20 {
			t.Fatal("ASSERT_CONTROLLED_OBJECT_WITHIN_OPT_IN", h.Length)
		}
		if h.Length > nativeArtifactBytes {
			nativeArtifactBytes = h.Length
		}
		payload, e := io.ReadAll(io.LimitReader(f, int64(r.Template.PayloadBytes)+1))
		if e != nil || uint64(len(payload)) > r.Template.PayloadBytes {
			t.Fatal("bounded page payload", e)
		}
		segments = append(segments, segment{h.Offset, payload})
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if nativeArtifactBytes < 827897 {
		t.Fatalf("ASSERT_GENUINE_NATIVE_ARTIFACT_HISTORICAL_SIZE bytes=%d status=%s", nativeArtifactBytes, report.Status)
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].offset < segments[j].offset })
	var assembled []byte
	for _, part := range segments {
		if part.offset != uint64(len(assembled)) {
			t.Fatal("ASSERT_ORDERED_ARTIFACT_SEGMENTS")
		}
		assembled = append(assembled, part.raw...)
	}
	if uint64(len(assembled)) != nativeArtifactBytes {
		t.Fatal("ASSERT_FULL_NATIVE_ARTIFACT_BYTES")
	}
	parsed, parseErr := graphprovenance.ParseSourceWithholdingPrivateV2(assembled)
	if parseErr != nil || len(parsed.Evidence.GraphBytes) == 0 {
		t.Fatal("ASSERT_VALID_NATIVE_ARTIFACT", parseErr)
	}
	callSites := 0
	for _, request := range parsed.Evidence.Acquisition.Requests {
		if request.Method == "callHierarchy/incomingCalls" {
			var rows []struct {
				FromRanges []json.RawMessage `json:"fromRanges"`
			}
			if json.Unmarshal(request.Response, &rows) != nil {
				t.Fatal("native incoming shape")
			}
			for _, row := range rows {
				callSites += len(row.FromRanges)
			}
		}
	}
	if callSites != 150 {
		t.Fatal("ASSERT_GENUINE_SOURCE_CALL_SITES", callSites)
	}
	if runErr != nil || report.Status != "PROVISIONAL" || report.Targets != 20 || report.Calls != 4 {
		t.Fatalf("ASSERT_LARGE_NATIVE_SEMANTIC_PREPARATION %+v: %v", report, runErr)
	}
	if evidence := os.Getenv("LSP_APPLICATION_COMPATIBILITY_EVIDENCE"); evidence != "" {
		st, e := os.Stat(evidence)
		if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || !filepath.IsAbs(evidence) || filepath.Clean(evidence) != evidence {
			t.Fatal("private owned proof root")
		}
		proof := filepath.Join(evidence, "controlled-native-artifact-"+pilot.Digest(assembled)[7:]+".json")
		f, e := os.OpenFile(proof, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if n, e := f.Write(assembled); e != nil || n != len(assembled) {
			f.Close()
			t.Fatal("proof write", e)
		}
		if e := f.Sync(); e != nil {
			f.Close()
			t.Fatal(e)
		}
		if e := f.Close(); e != nil {
			t.Fatal(e)
		}
		t.Logf("CONTROLLED_RAW_ARTIFACT_PROOF=%s", proof)
	}
	t.Logf("ASSERT_GENUINE_NATIVE_ARTIFACT_HISTORICAL_SIZE A-pass bytes=%d sha256=%s", len(assembled), pilot.Digest(assembled))
	t.Logf("CONTROLLED_LARGE_NATIVE_CALIBRATION calls=%d request=%d response=%d reserved=%+v observed=%+v child-policy-debits=%+v", report.Calls, report.RequestBytes, report.ResponseBytes, report.Reserved, report.KnownOwnedWork, report.ChildLedgerPolicyDebits)
}

func TestHostApplicationCompatibilityLargeDefaultPause(t *testing.T) {
	_, path, base := compatibilityLargeFixture(t, 256<<10)
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path, "LSP_HOST_APPLICATION_LARGE_REFERENCES=1")
	if e != nil || report.Status != "PAUSED_UNSUPPORTED_COMPUTATION_UNIT" || report.Calls != 0 || report.Reserved.Calls != 4 {
		t.Fatalf("ASSERT_DEFAULT_LARGE_OBJECT_TYPED_PAUSE %+v %v", report, e)
	}
	t.Log("ASSERT_DEFAULT_LARGE_OBJECT_TYPED_PAUSE A-pass")
}
