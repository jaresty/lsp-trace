package retainedcalls

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
)

func readExportV2(t *testing.T, input []byte) EvidenceV2 {
	t.Helper()
	raw, err := ExportV2(input)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := ValidateFor(raw, Family, "v2"); err != nil || version != VersionV2 {
		t.Fatalf("ASSERT_V2_COMPOSED: %s %v", version, err)
	}
	var e EvidenceV2
	if err = json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
func cloneTablesV2(t *testing.T, x TablesV2) TablesV2 {
	t.Helper()
	raw, err := json.Marshal(x)
	if err != nil {
		t.Fatal(err)
	}
	var y TablesV2
	if err = json.Unmarshal(raw, &y); err != nil {
		t.Fatal(err)
	}
	return y
}
func TestRetainedV2RoundtripOracle(t *testing.T) {
	for _, mode := range []acquisition.Mode{acquisition.Slice, acquisition.Incoming} {
		for _, variant := range []string{"", "zero", "missing", "empty"} {
			t.Run(string(mode)+variant, func(t *testing.T) {
				input, original := fixtureV2(t, mode, variant)
				e := readExportV2(t, input)
				if _, err := Export(input); err == nil {
					t.Fatal("ASSERT_V1_REJECT_V2")
				}
				raw, _ := json.Marshal(e)
				for _, version := range []string{"", "v1", Version} {
					if _, err := ValidateFor(raw, Family, version); err == nil {
						t.Fatal("ASSERT_EXPLICIT_VERSION", version)
					}
				}
				var admitted graphprovenance.EvidenceV2
				if err := json.Unmarshal(input, &admitted); err != nil {
					t.Fatal(err)
				}
				e.InputBytes = nil
				p, err := ReconstructV2(e.Tables)
				if err != nil {
					t.Fatal(err)
				}
				// Independent test oracle: original coordinator, authoritative native bytes,
				// and every source/census row, not the production comparison helper.
				got, _ := json.Marshal(p.Result.Graph)
				want, _ := json.Marshal(original.Graph)
				if !bytes.Equal(got, want) || !bytes.Equal(got, admitted.GraphBytes) {
					t.Fatal("ASSERT_NATIVE_ORACLE")
				}
				if !bytes.Equal(canonical(original.Targets), canonical(p.Result.Targets)) || !bytes.Equal(canonical(original.Requests), canonical(p.Result.Requests)) || original.Usage != p.Result.Usage || original.AcquisitionComplete != p.Result.AcquisitionComplete || !bytes.Equal(canonical(original.Request), canonical(p.Result.Request)) {
					t.Fatal("ASSERT_COORDINATOR_ORACLE")
				}
				if !bytes.Equal(canonical(admitted.Bindings), canonical(p.Bindings)) || !bytes.Equal(canonical(admitted.Captures), canonical(p.Captures)) || !bytes.Equal(canonical(admitted.Supplies), canonical(p.Supplies)) {
					t.Fatal("ASSERT_SOURCE_CENSUS_ORACLE")
				}
				count := 0
				for _, edge := range original.Graph.Edges {
					count += len(edge.CallSites)
				}
				if count != len(e.Tables.Occurrences) || len(original.Graph.Edges) != len(e.Tables.Groups) || len(original.Targets) != len(e.Tables.Connections) {
					t.Fatal("ASSERT_MANDATORY_CARDINALITY")
				}
				if variant == "" || variant == "zero" {
					c := e.Tables.Connections[1]
					if len(c.Nodes) != 3 || len(c.GroupIDs) != 2 || original.Targets[1].Connection.Status != "FOUND" {
						t.Fatal("ASSERT_INTERMEDIATE_MAPPING")
					}
					if len(e.Tables.Connections[2].Nodes) != 1 || len(e.Tables.Connections[2].GroupIDs) != 0 {
						t.Fatal("ASSERT_ZERO_HOP_ALIAS")
					}
					if original.Targets[3].Connection.Status != "NOT_FOUND_IN_RETAINED_GRAPH" {
						t.Fatal("ASSERT_DISCONNECTED")
					}
					if e.Tables.SupportTotal != len(e.Tables.Groups) {
						t.Fatal("ASSERT_SUPPORT_ONCE")
					}
					for _, g := range e.Tables.Groups {
						if len(g.OccurrenceIDs) == 0 && g.OccurrenceIDs == nil {
							t.Fatal("ASSERT_ZERO_SITE_ARRAY")
						}
					}
				}
				again, err := ExportV2(input)
				if err != nil {
					t.Fatal(err)
				}
				base, err := ExportV2(input)
				if err != nil || !bytes.Equal(base, again) {
					t.Fatal("ASSERT_DETERMINISTIC")
				}
				t.Log("ASSERT_NATIVE_COORDINATOR_SOURCE_ROUNDTRIP: PASS")
			})
		}
	}
}
func TestRetainedV2EveryRowDeletion(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	e := readExportV2(t, input)
	probes := map[string]int{"bindings": len(e.Tables.Bindings), "captures": len(e.Tables.Captures), "groups": len(e.Tables.Groups), "occurrences": len(e.Tables.Occurrences), "fields": len(e.Tables.NativeFields), "memberships": len(e.Tables.Memberships), "targets": len(e.Tables.Acquisition.Targets), "requests": len(e.Tables.Acquisition.Requests), "observations": len(e.Tables.Acquisition.EdgeObservations), "connections": len(e.Tables.Connections), "endpoints": len(e.Tables.Endpoints)}
	for name, n := range probes {
		for i := 0; i < n; i++ {
			t.Run(name+"/"+stringID(i), func(t *testing.T) {
				bad := e
				bad.Tables = cloneTablesV2(t, e.Tables)
				x := &bad.Tables
				switch name {
				case "bindings":
					x.Bindings = append(x.Bindings[:i], x.Bindings[i+1:]...)
				case "captures":
					x.Captures = append(x.Captures[:i], x.Captures[i+1:]...)
				case "groups":
					x.Groups = append(x.Groups[:i], x.Groups[i+1:]...)
				case "occurrences":
					x.Occurrences = append(x.Occurrences[:i], x.Occurrences[i+1:]...)
				case "fields":
					x.NativeFields = append(x.NativeFields[:i], x.NativeFields[i+1:]...)
				case "memberships":
					x.Memberships = append(x.Memberships[:i], x.Memberships[i+1:]...)
				case "targets":
					x.Acquisition.Targets = append(x.Acquisition.Targets[:i], x.Acquisition.Targets[i+1:]...)
				case "requests":
					x.Acquisition.Requests = append(x.Acquisition.Requests[:i], x.Acquisition.Requests[i+1:]...)
				case "observations":
					x.Acquisition.EdgeObservations = append(x.Acquisition.EdgeObservations[:i], x.Acquisition.EdgeObservations[i+1:]...)
				case "connections":
					x.Connections = append(x.Connections[:i], x.Connections[i+1:]...)
				case "endpoints":
					x.Endpoints = append(x.Endpoints[:i], x.Endpoints[i+1:]...)
				}
				// All outer public commitments are recomputed, not relied upon as authority.
				bad.InputDigest = digest(VersionV2+":input", bad.InputBytes)
				raw, _ := json.Marshal(bad)
				if _, err := ValidateFor(raw, Family, "v2"); err == nil {
					t.Fatal("ASSERT_REHASHED_ROW_DELETION", name, i)
				}
				if _, err := ReconstructV2(bad.Tables); err == nil {
					t.Fatal("ASSERT_TABLE_JOIN_DELETION", name, i)
				}
			})
		}
	}
}
func stringID(i int) string { raw, _ := json.Marshal(i); return string(raw) }
func TestRetainedV2RehashedSubstitutions(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Incoming, "")
	e := readExportV2(t, input)
	for _, kind := range []string{"range", "status", "owner", "mapping", "witness", "uri", "support", "version", "null"} {
		t.Run(kind, func(t *testing.T) {
			bad := e
			bad.Tables = cloneTablesV2(t, e.Tables)
			x := &bad.Tables
			switch kind {
			case "range":
				x.Occurrences[0].Range.End.Character++
				old := x.Occurrences[0].ID
				x.Occurrences[0].ID = occurrenceIDV2(x.Context.ID, x.Occurrences[0])
				for i := range x.Groups {
					for j, id := range x.Groups[i].OccurrenceIDs {
						if id == old {
							x.Groups[i].OccurrenceIDs[j] = x.Occurrences[0].ID
						}
					}
				}
			case "status":
				x.Bindings[0].AnchorStatus = "INVALID_COORDINATES"
			case "owner":
				x.Acquisition.Requests[0].TargetID = "alias"
			case "mapping":
				x.Occurrences[0].NativePointer = "/edges/0/call_sites/999"
				x.Occurrences[0].ID = occurrenceIDV2(x.Context.ID, x.Occurrences[0])
			case "witness":
				x.Connections[1].Nodes[1] = x.Connections[1].Nodes[0]
			case "uri":
				x.Bindings[0].URI = "file:///substituted.go"
			case "support":
				x.SupportTotal++
			case "version":
				x.Acquisition.Request.Context.Generation++
			case "null":
				x.Groups[0].OccurrenceIDs = nil
			}
			raw, _ := json.Marshal(bad)
			if _, err := ValidateFor(raw, Family, "v2"); err == nil {
				t.Fatal("ASSERT_REHASHED_SUBSTITUTION", kind)
			}
			if _, err := ReconstructV2(bad.Tables); err == nil {
				t.Fatal("ASSERT_TABLE_SUBSTITUTION", kind)
			}
		})
	}
}
func TestRetainedV2IndependentProducerFault(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	var e graphprovenance.EvidenceV2
	if err := json.Unmarshal(input, &e); err != nil {
		t.Fatal(err)
	}
	tables, err := extractTablesV2(e, digest(VersionV2+":input", input))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ReconstructV2(tables)
	if err != nil {
		t.Fatal(err)
	}
	// A coherent reduced projection (including its own native semantic receipt)
	// must not satisfy conservation against the unreduced admitted input.
	other, _ := fixtureV2(t, acquisition.Slice, "empty")
	var reduced graphprovenance.EvidenceV2
	if err = json.Unmarshal(other, &reduced); err != nil {
		t.Fatal(err)
	}
	p.Result = reduced.Acquisition
	if err = compareProjectionV2(e, p); err == nil {
		t.Fatal("ASSERT_INDEPENDENT_PRODUCER_FAULT")
	}
	t.Log("ASSERT_INDEPENDENT_PRODUCER_FAULT: PASS")
}
func TestRetainedV2ContextAndNumbers(t *testing.T) {
	input, _ := fixtureV2(t, acquisition.Slice, "")
	e := readExportV2(t, input)
	changed := append([]byte(" \n"), input...)
	f := readExportV2(t, changed)
	if e.Tables.Context.ID == f.Tables.Context.ID || e.Tables.Groups[0].ID == f.Tables.Groups[0].ID || e.Tables.Occurrences[0].ID == f.Tables.Occurrences[0].ID {
		t.Fatal("ASSERT_EXACT_ENVELOPE_CONTEXT")
	}
	raw, _ := json.Marshal(e.Tables)
	if !bytes.Contains(raw, []byte(`9007199254740993`)) || !bytes.Contains(raw, []byte(`1e+02`)) {
		t.Fatal("ASSERT_EXACT_NUMBERS")
	}
	// Fixed literal preimages, independent of the constructor's object encoding.
	preimage := []byte(`["EXACT_ENVELOPE_NATIVE_GROUP_DISTINCT_SITE_TYPED_ACQUISITION_V2","sha256:test"]`)
	if contextIDV2("sha256:test") != "sha256:db9c7c5eaf4d4acc7fc53992d28fc908ed9f9336135b614d9689746935ce1aa7" || contextIDV2("sha256:test") != digest("lsp-trace.retained-calls.v2:context", preimage) {
		t.Fatal("ASSERT_CONTEXT_PREIMAGE")
	}
}
func TestRetainedV2LimitsAndPreflight(t *testing.T) {
	for _, n := range []int{64, 65} {
		raw := []byte(strings.Repeat("[", n) + "0" + strings.Repeat("]", n))
		err := preflightExportV2(raw, len(raw))
		if (err == nil) != (n == 64) {
			t.Fatal("ASSERT_DEPTH_BOUNDARY", n, err)
		}
	}
	for _, raw := range []string{`{"n":9007199254740993}`, `{"data":{"n":1e9999}}`, `{"content":"[[[["}`} {
		if err := preflightExportV2([]byte(raw), 1024); err != nil {
			t.Fatal("ASSERT_OPAQUE_NUMBERS", err)
		}
	}
	for _, raw := range []string{`{"n":1e1025}`, `{"n":1,"n":2}`} {
		if err := preflightExportV2([]byte(raw), 1024); err == nil {
			t.Fatal("ASSERT_PREFLIGHT_REJECT")
		}
	}
	for _, kind := range []string{"nodes", "groups", "bytes"} {
		var e graphprovenance.EvidenceV2
		switch kind {
		case "nodes":
			e.Acquisition.Graph.Nodes = make([]graph.Node, MaxNodesV2+1)
		case "groups":
			e.Acquisition.Graph.Edges = make([]graph.Edge, MaxGroupsV2+1)
		case "bytes":
			e.GraphBytes = make([]byte, MaxNativeBytesV2+1)
		}
		var limit *LimitErrorV2
		if !errors.As(checkLimitsV2(e), &limit) {
			t.Fatal("ASSERT_TYPED_LIMIT", kind)
		}
	}
	if _, err := schema.BytesFor(Family, "v2"); err != nil {
		t.Fatal(err)
	}
}
func TestRetainedV1Frozen(t *testing.T) {
	dir := "testdata"
	inPath := filepath.Join(dir, "frozen-v1-input.json")
	outPath := filepath.Join(dir, "frozen-v1-export.json")
	input, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Export(input)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("ASSERT_V1_FROZEN_BYTES", err)
	}
	if _, err = ExportV2(input); err == nil {
		t.Fatal("ASSERT_V2_REJECT_V1")
	}
	if _, err = ValidateFor(want, Family, "v1"); err != nil {
		t.Fatal(err)
	}
}
