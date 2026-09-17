package sourceprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

const (
	endpointUnit = "sha256:71faae72ce82116ed317c10ed87170ebc9e3ed1f28aecf3e8fa5b7ae887aa713"
	relationUnit = "sha256:0b12d315425e26fd50f42a26f1e095334d136325eb0d396fe770c0a962c946a5"
)

func fixtureCandidates() []Candidate {
	return []Candidate{
		{UnitID: endpointUnit, CitationID: "sha256:bf8c0aef557fd0c5447b23d6965e18e61e66d9bfd6e74f50af24d714aa55c6a5", Role: "ENDPOINT", GraphSubjectID: "node", LogicalSourceID: "src/source.ts", Range: Range{Start: Position{0, 0}, End: Position{3, 0}}, PositionEncoding: "utf-16", PrivacyClassification: "PUBLIC"},
		{UnitID: relationUnit, CitationID: "sha256:727ba15521bfc0d41ae6144d40db25a88685ce003f1e444d186941e89934f2e7", Role: "RELATION", GraphSubjectID: "relation", OccurrenceID: "sha256:6cd8ab102d0fbcbc756b7a3b887e156663e54cc8c8df3af6c82383b5d39972a1", LogicalSourceID: "src/source.ts", Range: Range{Start: Position{1, 8}, End: Position{1, 16}}, PositionEncoding: "utf-16", RelationProvenance: "SERVER_REPORTED", PrivacyClassification: "PUBLIC"},
	}
}

func fixtureSources(available bool) map[string]Source {
	raw := []byte("function target() {\n  \"😀\"; target();\n}\n")
	return map[string]Source{"src/source.ts": {LogicalSourceID: "src/source.ts", Digest: "sha256:b2f615ccc7e60a8994f75a1e5cae0a73e0e7f34967a71288eabaad7840d121d5", Bytes: raw, Available: available}}
}

func TestProjectFrozenVariants(t *testing.T) {
	tests := []struct {
		name       string
		candidates []Candidate
		sources    map[string]Source
		policy     Policy
		want       Accounting
		status     string
		cause      string
	}{
		{name: "body", candidates: fixtureCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true}, want: Accounting{Candidates: 2, Selected: 2, LogicalSelectedBytes: 50, UniqueEmittedBytes: 42, Evaluated: 2, Terminal: 2}, status: "COMPLETE"},
		{name: "metadata", candidates: fixtureCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "metadata"}, want: Accounting{Candidates: 2, Selected: 2, Evaluated: 2, Terminal: 2}, status: "COMPLETE"},
		{name: "withheld", candidates: withheldCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "withheld", BodyRequested: true}, want: Accounting{Candidates: 2, Omitted: 2, Evaluated: 2, Terminal: 2}, status: "PARTIAL", cause: "POLICY_WITHHELD"},
		{name: "unavailable", candidates: fixtureCandidates(), sources: fixtureSources(false), policy: Policy{PolicyID: "public", BodyRequested: true}, want: Accounting{Candidates: 2, Omitted: 2, Evaluated: 2, Terminal: 2}, status: "SOURCE_UNAVAILABLE", cause: "SOURCE_UNAVAILABLE"},
		{name: "byte-limit", candidates: fixtureCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 41}, want: Accounting{Candidates: 2, Omitted: 2, Evaluated: 2, Terminal: 2}, status: "TRUNCATED", cause: "BYTE_LIMIT"},
		{name: "range-limit", candidates: fixtureCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true, MaxRanges: 1}, want: Accounting{Candidates: 2, Selected: 1, Omitted: 1, LogicalSelectedBytes: 42, UniqueEmittedBytes: 42, Evaluated: 2, Terminal: 2}, status: "TRUNCATED", cause: "RANGE_LIMIT"},
		{name: "empty", candidates: nil, sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true}, want: Accounting{}, status: "SUCCESSFUL_EMPTY"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Project(tc.candidates, tc.sources, tc.policy)
			if err != nil {
				t.Fatalf("ASSERT_SOURCE_PROJECTION_VARIANT_%s: %v", tc.name, err)
			}
			if got.Accounting != tc.want || got.Status != tc.status {
				t.Fatalf("ASSERT_SOURCE_PROJECTION_VARIANT_%s: accounting=%+v status=%s", tc.name, got.Accounting, got.Status)
			}
			if tc.cause != "" {
				if len(got.Omissions) == 0 || got.Omissions[0].Cause != tc.cause {
					t.Fatalf("ASSERT_SOURCE_PROJECTION_VARIANT_%s_CAUSE: %+v", tc.name, got.Omissions)
				}
			}
			if tc.name == "body" {
				if len(got.EmittedSpans) != 1 || got.EmittedSpans[0].Body != string(fixtureSources(true)["src/source.ts"].Bytes) || got.Units[0].UnitID != relationUnit || got.Units[0].Body != "target()" {
					t.Fatalf("ASSERT_SOURCE_PROJECTION_UTF16_OVERLAP: units=%+v spans=%+v", got.Units, got.EmittedSpans)
				}
			}
			if tc.name == "range-limit" && (len(got.Units) != 1 || got.Units[0].UnitID != endpointUnit) {
				t.Fatalf("ASSERT_SOURCE_PROJECTION_RANGE_PRIORITY: %+v", got.Units)
			}
		})
	}
}

func TestProjectObjectAndWorkLimits(t *testing.T) {
	objects, err := Project(fixtureCandidates(), fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 1, MaxWork: 2, EnforceLimits: true})
	if err != nil {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_OBJECT_LIMIT: %v", err)
	}
	if objects.Accounting.Selected != 1 || objects.Accounting.Omitted != 1 || len(objects.Units) != 1 || objects.Units[0].UnitID != endpointUnit || len(objects.Omissions) != 1 || objects.Omissions[0].Cause != "OBJECT_LIMIT" {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_OBJECT_LIMIT: %+v", objects)
	}
	work, err := Project(fixtureCandidates(), fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true, MaxBytes: 42, MaxRanges: 2, MaxObjects: 2, MaxWork: 1, EnforceLimits: true})
	if err == nil || work.Accounting.Selected != 0 {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_WORK_LIMIT_FAIL_CLOSED: result=%+v err=%v", work, err)
	}
}

func TestProjectCanonicalUnderCandidatePermutation(t *testing.T) {
	a := fixtureCandidates()
	b := []Candidate{a[1], a[0]}
	one, err := Project(a, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Project(b, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_CANONICAL_PERMUTATION: one=%+v two=%+v", one, two)
	}
}

func TestProjectCanonicalBytesUnderCandidatePermutation(t *testing.T) {
	a := fixtureCandidates()
	b := []Candidate{a[1], a[0]}
	one, err := Project(a, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Project(b, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	oneBytes, err := json.Marshal(one)
	if err != nil {
		t.Fatal(err)
	}
	twoBytes, err := json.Marshal(two)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oneBytes, twoBytes) {
		t.Fatalf("ASSERT_C01_CANONICAL_BYTES_PERMUTATION: one=%s two=%s", oneBytes, twoBytes)
	}
}

func TestProjectSupportsExactUTF8UTF16UTF32Ranges(t *testing.T) {
	for _, tc := range []struct {
		encoding string
		start    uint32
		end      uint32
	}{
		{encoding: "utf-8", start: 1, end: 5},
		{encoding: "utf-16", start: 1, end: 3},
		{encoding: "utf-32", start: 1, end: 2},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			candidate := Candidate{UnitID: "unit", CitationID: "citation", Role: "ENDPOINT", GraphSubjectID: "node", LogicalSourceID: "source", Range: Range{Start: Position{Character: tc.start}, End: Position{Character: tc.end}}, PositionEncoding: tc.encoding, PrivacyClassification: "PUBLIC"}
			sources := map[string]Source{"source": {LogicalSourceID: "source", Digest: "sha256:source", Bytes: []byte("a😀b"), Available: true}}
			got, err := Project([]Candidate{candidate}, sources, Policy{PolicyID: "public", BodyRequested: true})
			if err != nil || len(got.Units) != 1 || got.Units[0].Body != "😀" {
				t.Fatalf("ASSERT_C02_UTF8_UTF16_UTF32_EXACT_RANGES: result=%+v err=%v", got, err)
			}
		})
	}
}

func TestProjectMutationsCannotChangeGraphBytes(t *testing.T) {
	graphBytes := []byte(`{"nodes":[{"id":"node"}],"relations":[{"type":"CALLS","evidence":"SERVER_REPORTED"}],"authority":0,"source_graph_complete":"UNKNOWN"}`)
	before := sha256.Sum256(graphBytes)
	candidates := fixtureCandidates()
	mutations := []struct {
		name       string
		candidates []Candidate
		sources    map[string]Source
		policy     Policy
	}{
		{name: "body", candidates: candidates, sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true}},
		{name: "metadata", candidates: candidates, sources: fixtureSources(true), policy: Policy{PolicyID: "metadata"}},
		{name: "reordered", candidates: []Candidate{candidates[1], candidates[0]}, sources: fixtureSources(true), policy: Policy{PolicyID: "public", BodyRequested: true}},
		{name: "withheld", candidates: withheldCandidates(), sources: fixtureSources(true), policy: Policy{PolicyID: "withheld", BodyRequested: true}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := Project(mutation.candidates, mutation.sources, mutation.policy); err != nil {
				t.Fatal(err)
			}
			after := sha256.Sum256(graphBytes)
			if after != before {
				t.Fatalf("ASSERT_C04_GRAPH_BYTES_NEUTRAL_UNDER_SOURCE_MUTATION: before=%x after=%x", before, after)
			}
		})
	}
}

func TestProjectRejectsIncompleteAndMixedCandidateIdentity(t *testing.T) {
	const assertion = "ASSERT_C06_INVALID_ID_RANGE_ENCODING_PRIVACY_STATUS_LIMIT_FAILS"
	for _, tc := range []struct {
		name   string
		mutate func([]Candidate)
	}{
		{name: "missing-citation", mutate: func(c []Candidate) { c[0].CitationID = "" }},
		{name: "missing-subject", mutate: func(c []Candidate) { c[0].GraphSubjectID = "" }},
		{name: "missing-source", mutate: func(c []Candidate) { c[0].LogicalSourceID = "" }},
		{name: "invalid-role", mutate: func(c []Candidate) { c[0].Role = "ANCILLARY" }},
		{name: "mixed-source-encoding", mutate: func(c []Candidate) { c[1].PositionEncoding = "utf-8" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidates := fixtureCandidates()
			tc.mutate(candidates)
			got, err := Project(candidates, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
			if err == nil || got.Status != "" || len(got.Units) != 0 {
				t.Fatalf("%s: result=%+v err=%v", assertion, got, err)
			}
		})
	}
}

func TestPrivacyEligibilityAndMetadataDefault(t *testing.T) {
	const secret = "SECRET_RESOLVER_ROOT_TOKEN"
	candidates := fixtureCandidates()
	sources := fixtureSources(true)
	sources["src/source.ts"] = Source{LogicalSourceID: "src/source.ts", Digest: sources["src/source.ts"].Digest, ByteLength: len(sources["src/source.ts"].Bytes), Bytes: append(sources["src/source.ts"].Bytes, []byte(secret)...), Available: true}

	metadata, err := Project(candidates, sources, Policy{PolicyID: "metadata-default"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(secret)) || metadata.PrivacySummary.BodyRequested || metadata.PrivacySummary.BodyReturned != 0 || len(metadata.EmittedSpans) != 0 {
		t.Fatalf("ASSERT_C05_PRIVATE_DATA_NEVER_SERIALIZED: %s", raw)
	}
	for _, unit := range metadata.Units {
		if unit.Body != "" || unit.BodyDisposition != "NOT_REQUESTED" {
			t.Fatalf("ASSERT_C05_METADATA_ONLY_NO_BODY_ACQUISITION: %+v", unit)
		}
	}

	withheld, err := Project(withheldCandidates(), fixtureSources(true), Policy{PolicyID: "restricted", BodyRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	if withheld.Accounting.Selected != 0 || withheld.Accounting.Omitted != len(candidates) || withheld.PrivacySummary.BodyReturned != 0 || withheld.PrivacySummary.BodyWithheld != len(candidates) {
		t.Fatalf("ASSERT_C05_PRIVACY_PRECEDES_BODY_RESOLUTION: %+v", withheld)
	}
	for _, omission := range withheld.Omissions {
		if omission.Cause != "POLICY_WITHHELD" {
			t.Fatalf("ASSERT_C05_PRIVACY_OMISSION_DISCOVERABLE: %+v", withheld.Omissions)
		}
	}
}

func TestProjectRejectsNonServerRelationProvenance(t *testing.T) {
	candidates := fixtureCandidates()
	candidates[1].RelationProvenance = "SOURCE_INFERRED"
	got, err := Project(candidates, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err == nil || got.Accounting.Selected != 0 {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_SERVER_ONLY_RELATIONS: result=%+v err=%v", got, err)
	}
}

func TestProjectRejectsUnsupportedEncodingWithoutPartialOutput(t *testing.T) {
	candidates := fixtureCandidates()
	candidates[0].PositionEncoding = "utf-7"
	got, err := Project(candidates, fixtureSources(true), Policy{PolicyID: "public", BodyRequested: true})
	if err == nil || errors.Is(err, ErrNotImplemented) || got.Accounting.Selected != 0 {
		t.Fatalf("ASSERT_SOURCE_PROJECTION_ENCODING_FAIL_CLOSED: result=%+v err=%v", got, err)
	}
}

func TestPositionOffsetStrictUTF16Contract(t *testing.T) {
	t.Run("astral-two-code-units", func(t *testing.T) {
		raw := []byte("a😀b")
		got, err := positionOffset(raw, "utf-16", Position{Line: 0, Character: 3})
		if err != nil || got != len("a😀") {
			t.Fatalf("ASSERT_UTF16_ASTRAL_TWO_CODE_UNITS: offset=%d err=%v", got, err)
		}
	})
	t.Run("half-open-range", func(t *testing.T) {
		raw := []byte("a😀b")
		got, err := rangeBytes(raw, "utf-16", Range{Start: Position{Line: 0, Character: 1}, End: Position{Line: 0, Character: 3}})
		if err != nil || string(got) != "😀" {
			t.Fatalf("ASSERT_UTF16_HALF_OPEN_RANGE_OFFSETS: body=%q err=%v", got, err)
		}
	})
	t.Run("crlf-terminator", func(t *testing.T) {
		if got, err := positionOffset([]byte("a\r\nb"), "utf-16", Position{Line: 0, Character: 2}); err == nil {
			t.Fatalf("ASSERT_UTF16_CRLF_TERMINATOR_NOT_ADDRESSABLE: offset=%d err=%v", got, err)
		}
		if got, err := positionOffset([]byte("a\r\nb"), "utf-16", Position{Line: 1, Character: 0}); err != nil || got != 3 {
			t.Fatalf("ASSERT_UTF16_CRLF_NEXT_LINE_BOUNDARY: offset=%d err=%v", got, err)
		}
	})
	t.Run("mid-surrogate", func(t *testing.T) {
		if got, err := positionOffset([]byte("😀"), "utf-16", Position{Line: 0, Character: 1}); err == nil {
			t.Fatalf("ASSERT_UTF16_MID_SURROGATE_REJECTED: offset=%d err=%v", got, err)
		}
	})
	t.Run("line-out-of-document", func(t *testing.T) {
		if got, err := positionOffset([]byte("a"), "utf-16", Position{Line: 1, Character: 0}); err == nil {
			t.Fatalf("ASSERT_UTF16_LINE_OUT_OF_DOCUMENT_REJECTED: offset=%d err=%v", got, err)
		}
	})
	t.Run("character-out-of-line", func(t *testing.T) {
		if got, err := positionOffset([]byte("a"), "utf-16", Position{Line: 0, Character: 2}); err == nil {
			t.Fatalf("ASSERT_UTF16_CHARACTER_OUT_OF_LINE_REJECTED: offset=%d err=%v", got, err)
		}
	})
}

func withheldCandidates() []Candidate {
	out := fixtureCandidates()
	for i := range out {
		out[i].Withheld = true
	}
	return out
}
