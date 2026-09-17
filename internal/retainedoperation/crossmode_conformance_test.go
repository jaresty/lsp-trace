package retainedoperation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedinspection"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojectionv2"
)

const op41FixtureDir = "../sourceprojection/testdata/crossmode-v2"

type op41FixtureLookup struct {
	calls int
	id    sourceobject.Identity
	raw   []byte
	err   error
}

func (l *op41FixtureLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	l.calls++
	if l.err != nil {
		return sourceobject.Object{}, l.err
	}
	return sourceobject.Object{Identity: l.id, Bytes: append([]byte(nil), l.raw...)}, nil
}

func op41Fixture(t *testing.T) (retainedinspection.Request, []byte, []byte, sourceobject.Identity) {
	t.Helper()
	read := func(name string) []byte {
		raw, err := os.ReadFile(filepath.Join(op41FixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var request retainedinspection.Request
	if err := json.Unmarshal(read("retained-request.json"), &request); err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Identity            sourceobject.Identity `json:"identity"`
		PhysicalLookupCount int                   `json:"physical_lookup_count"`
	}
	if err := json.Unmarshal(read("source-object.json"), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.PhysicalLookupCount != 1 {
		t.Fatalf("ASSERT_OP41_FIXTURE_EXACT_ONCE: %d", metadata.PhysicalLookupCount)
	}
	return request, read("source-object.bin"), read("retained-expected.json"), metadata.Identity
}

func TestOperation41CrossModeRetainedOracle(t *testing.T) {
	request, source, expected, id := op41Fixture(t)
	lookup := &op41FixtureLookup{id: id, raw: source}
	result, failure := execute(t, request, lookup, operation.Request{})
	if failure != nil {
		t.Fatal(failure)
	}
	if !bytes.Equal(result.Artifact, expected) {
		t.Fatalf("ASSERT_OP41_RETAINED_EXACT_ARTIFACT_BYTES")
	}
	if result.ArtifactSchemaID != retainedinspection.SourceProjectionSchemaID || lookup.calls != 1 || !bytes.HasSuffix(result.Artifact, []byte("\n")) || bytes.HasSuffix(result.Artifact, []byte("\n\n")) {
		t.Fatalf("ASSERT_OP41_SCHEMA_NEWLINE_LOOKUP: schema=%q calls=%d", result.ArtifactSchemaID, lookup.calls)
	}
	wire := result.Value.(sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding])
	if wire.Authority != 0 || wire.SourceGraphComplete != "UNKNOWN" || wire.GraphFactsAdded != 0 || len(wire.DocumentBindings) != 2 || wire.DocumentSelection.SelectedURIs[0] != wire.DocumentSelection.TargetURI || wire.DocumentSelection.Ordering != "TARGET_FIRST_THEN_URI_LEXICOGRAPHIC" {
		t.Fatalf("ASSERT_OP41_NEUTRAL_TARGET_FIRST: %+v", wire)
	}
	if len(wire.Units) != 2 || wire.Units[0].EvidenceRange == wire.Units[0].DisplayRange || wire.Units[0].ItemRange == nil || wire.Units[0].SelectionRange == nil || wire.Units[0].PositionEncoding != "utf-16" || !strings.Contains(wire.Units[0].Body, "😀") {
		t.Fatalf("ASSERT_OP41_INDEPENDENT_UTF16_FULL_DEFINITION: %+v", wire.Units)
	}
	if wire.PrivacySummary.BodyReturned != 2 || wire.PrivacySummary.BodyWithheld != 0 || len(wire.Omissions) != 0 || wire.Accounting.Selected != 2 || wire.Accounting.Terminal != 2 {
		t.Fatalf("ASSERT_OP41_PRIVACY_OMISSIONS_ACCOUNTING: %+v", wire)
	}
}

func TestOperation41CrossModeAdversarialAtomicFailures(t *testing.T) {
	base, source, _, id := op41Fixture(t)
	cases := []struct {
		name, phase, state string
		mutate             func(*retainedinspection.Request)
		lookup             func() retainedprojection.Lookup
	}{
		{"missing-source", "RESOLVE", "MISSING", func(*retainedinspection.Request) {}, func() retainedprojection.Lookup {
			return &op41FixtureLookup{err: &sourceobject.Error{Code: sourceobject.CodeMissing}}
		}},
		{"corrupt-source", "RESOLVE", "RESOLVED_DIGEST_MISMATCH", func(*retainedinspection.Request) {}, func() retainedprojection.Lookup {
			bad := append([]byte(nil), source...)
			bad[0] ^= 1
			return &op41FixtureLookup{id: id, raw: bad}
		}},
		{"source-budget", "RESOLVE", "RESOLVE_UNIQUE_SOURCE_BYTES_LIMIT", func(r *retainedinspection.Request) { r.ResolveLimits.MaxUniqueSourceBytes = 1 }, func() retainedprojection.Lookup { return &op41FixtureLookup{id: id, raw: source} }},
		{"work-budget", "PROJECT", "PROJECTION_FAILED", func(r *retainedinspection.Request) { r.Projection.Limits.MaxWork = 0 }, func() retainedprojection.Lookup { return &op41FixtureLookup{id: id, raw: source} }},
		{"response-budget", "ASSEMBLE", "ASSEMBLY_FAILED", func(r *retainedinspection.Request) { r.Projection.Limits.MaxResponseBytes = 1 }, func() retainedprojection.Lookup { return &op41FixtureLookup{id: id, raw: source} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.mutate(&r)
			result, failure := execute(t, r, tc.lookup(), operation.Request{})
			assertTypedFailure(t, result, failure, tc.phase, tc.state)
		})
	}
	t.Run("object-budget-decode", func(t *testing.T) {
		r := base
		r.ResolveLimits.MaxDistinctObjects = 0
		raw, _ := json.Marshal(r)
		result, failure := NewInspectHydratedHandler(&op41FixtureLookup{id: id, raw: source})(t.Context(), operation.Request{Input: raw})
		assertTypedFailure(t, result, failure, "DECODE", "INVALID_REQUEST")
	})
	t.Run("range-budget-truncates", func(t *testing.T) {
		r := base
		r.Projection.Limits.MaxRanges = 0
		result, failure := execute(t, r, &op41FixtureLookup{id: id, raw: source}, operation.Request{})
		if failure != nil {
			t.Fatal(failure)
		}
		wire := result.Value.(sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding])
		if wire.Status != "TRUNCATED" || len(wire.Units) != 0 || len(wire.Omissions) != 2 || wire.Omissions[0].Cause != "RANGE_LIMIT" {
			t.Fatalf("ASSERT_OP41_RANGE_BUDGET_TERMINAL: %+v", wire)
		}
	})

	t.Run("invalid-utf16-boundary", func(t *testing.T) {
		r := base
		r.RetainedSourceEvidence.InlineSnapshotV2 = strings.Replace(r.RetainedSourceEvidence.InlineSnapshotV2, `"character":21`, `"character":17`, 1)
		result, failure := execute(t, r, &op41FixtureLookup{id: id, raw: source}, operation.Request{})
		assertTypedFailure(t, result, failure, "PROJECT", "PROJECTION_FAILED")
	})
	t.Run("missing-display-evidence", func(t *testing.T) {
		r := base
		r.Selection.Target.GraphSubjectID = "missing"
		r.Selection.Selections[0] = r.Selection.Target
		result, failure := execute(t, r, &op41FixtureLookup{id: id, raw: source}, operation.Request{})
		assertTypedFailure(t, result, failure, "SELECT", "MISSING_SUBJECT_BINDING")
	})
	t.Run("mixed-carrier", func(t *testing.T) {
		r := base
		r.RetainedSourceEvidence.ContentAddressedSnapshotV2 = &retainedinspection.ContentAddressedSnapshot{ID: id.Digest, ArtifactByteLength: 1, ArtifactSchemaID: retainedinspection.SourceSnapshotSchemaID, Generation: "g-" + strings.Repeat("0", 64)}
		raw, _ := json.Marshal(r)
		result, failure := NewInspectHydratedHandler(&op41FixtureLookup{id: id, raw: source})(t.Context(), operation.Request{Input: raw})
		assertTypedFailure(t, result, failure, "DECODE", "INVALID_REQUEST")
	})
	t.Run("malformed-carrier", func(t *testing.T) {
		result, failure := NewInspectHydratedHandler(&op41FixtureLookup{id: id, raw: source})(t.Context(), operation.Request{Input: []byte(`{"mode":"RETAINED_SOURCE_PROJECTION"}`)})
		assertTypedFailure(t, result, failure, "DECODE", "INVALID_REQUEST")
	})
}
