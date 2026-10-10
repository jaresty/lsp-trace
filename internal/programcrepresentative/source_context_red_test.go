package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
)

// Stage7A is deliberately specified against production-owned types. This file
// declares fixtures only: it must not provide substitutes for the typed API.

func stage7Range(sl, sc, el, ec uint32) graph.Range {
	return graph.Range{Start: graph.Position{Line: sl, Character: sc}, End: graph.Position{Line: el, Character: ec}}
}

func stage7Document(uri, body string) PreparedSourceDocument {
	sum := sha256.Sum256([]byte(body))
	return PreparedSourceDocument{
		URI: uri, SessionID: "session-7", Generation: 17, DocumentVersion: 23,
		RevisionIdentity: "revision-c2f4b427", RevisionCustody: "PROVIDER_VERIFIED",
		SourceSHA256: "sha256:" + strings.ToLower(hexString(sum[:])), ByteLength: len(body),
		PositionEncoding: "utf-16", ExactBytes: []byte(body),
	}
}

func stage7Limits() SourceContextLimits {
	return SourceContextLimits{
		MaxDocumentBytes: 1 << 16, MaxTotalBytes: 1 << 18, MaxDocuments: 8,
		MaxOccurrences: 32, MaxPoints: 32, MaxSpans: 32, MaxCandidates: 64,
		MaxStringBytes: 4096, MaxCanonicalBytes: 1 << 20, MaxWork: 1 << 20,
	}
}

func stage7Fixture(t *testing.T) SourceContextInput {
	t.Helper()
	f := stage6FixtureForMode("self-loops-repeated")
	token := RepresentativeQualificationFor(f, 8)
	representative, err := stage6Compute(f, token, stage6Verifier(token, 8))
	if err != nil {
		t.Fatal(err)
	}
	caller := stage7Document("file:///caller.go", "// caller prefix\npackage p\nfunc caller(){ target() }\n// caller suffix\n")
	target := stage7Document("file:///target.go", "// target prefix\npackage p\nfunc target() {}\n// target suffix\n")
	input := SourceContextInput{
		Representative: representative,
		Occurrences: []AdmittedSourceOccurrence{
			{Identity: "repeat-a", Ordinal: 1, Multiplicity: 2, RelationID: "relation-repeat-a", PredecessorIdentity: "pred-repeat-a", Kind: "CALLS", CallerID: "root", TargetID: "alpha", EvidenceRange: stage7Range(2, 15, 2, 23), ItemRange: stage7Range(2, 0, 2, 25), SelectionRange: stage7Range(2, 5, 2, 11)},
			{Identity: "repeat-b", Ordinal: 2, Multiplicity: 1, RelationID: "relation-repeat-b", PredecessorIdentity: "pred-repeat-b", Kind: "CALLS", CallerID: "root", TargetID: "alpha", EvidenceRange: stage7Range(2, 15, 2, 23), ItemRange: stage7Range(2, 0, 2, 25), SelectionRange: stage7Range(2, 5, 2, 11)},
			{Identity: "loop", Ordinal: 0, Multiplicity: 1, RelationID: "relation-loop", PredecessorIdentity: "pred-loop", Kind: "CALLS", CallerID: "alpha", TargetID: "alpha", EvidenceRange: stage7Range(2, 5, 2, 5), ItemRange: stage7Range(2, 0, 2, 16), SelectionRange: stage7Range(2, 5, 2, 11)},
		},
		QueryReferenceSelection: RoleSelectionInput{Candidates: []SourceCandidate{{OccurrenceIdentity: "repeat-a", DocumentURI: caller.URI, Role: SourceContextRoleQueryReference, DisplayRange: stage7Range(2, 0, 2, 25), ContextSpan: stage7Range(1, 0, 3, 0)}}},
		TargetSelection:         RoleSelectionInput{Candidates: []SourceCandidate{{OccurrenceIdentity: "loop", DocumentURI: target.URI, Role: SourceContextRoleTarget, DisplayRange: stage7Range(2, 0, 2, 16), ContextSpan: stage7Range(1, 0, 3, 0)}}},
		Documents:               []PreparedSourceDocument{caller, target},
		Policy:                  SourceContextPolicy{Identity: "stage7-policy", Digest: digestBytes([]byte("stage7-policy\n")), WithholdURIs: []string{}},
		Limits:                  stage7Limits(),
	}
	stage7SeedProductionFields(&input)
	return input
}

func stage7SeedProductionFields(input *SourceContextInput) {
	for i := range input.Occurrences {
		v := reflect.ValueOf(&input.Occurrences[i]).Elem()
		for name, text := range map[string]string{"SchemaID": "stage7-occurrence/v1", "Digest": digestBytes([]byte(fmt.Sprintf("occurrence-%d", i)))} {
			field := v.FieldByName(name)
			if field.IsValid() && field.CanSet() && field.Kind() == reflect.String && field.String() == "" {
				field.SetString(text)
			}
		}
		for _, name := range []string{"ByteLength", "Length"} {
			field := v.FieldByName(name)
			if field.IsValid() && field.CanSet() && field.CanInt() && field.Int() == 0 {
				field.SetInt(int64(i + 1))
			}
			if field.IsValid() && field.CanSet() && field.CanUint() && field.Uint() == 0 {
				field.SetUint(uint64(i + 1))
			}
		}
	}
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2], out[i*2+1] = digits[v>>4], digits[v&15]
	}
	return string(out)
}

func TestStage7ASelectedBothIndependentRolesAndExactRanges(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ContextComplete() {
		t.Fatal("ASSERT_STAGE7_CONTEXT_COMPLETE")
	}
	q, target := got.QueryReference(), got.Target()
	if q.Outcome != SourceContextSelected || target.Outcome != SourceContextSelected {
		t.Fatalf("ASSERT_STAGE7_SELECTED_BOTH q=%+v target=%+v", q, target)
	}
	if q.DocumentURI == target.DocumentURI || q.PointID == target.PointID {
		t.Fatal("ASSERT_STAGE7_QUERY_TARGET_INDEPENDENT")
	}
	wantQuery := []graph.Range{stage7Range(2, 15, 2, 23), stage7Range(2, 0, 2, 25), stage7Range(2, 5, 2, 11), stage7Range(2, 0, 2, 25), stage7Range(1, 0, 3, 0)}
	wantTarget := []graph.Range{stage7Range(2, 5, 2, 5), stage7Range(2, 0, 2, 16), stage7Range(2, 5, 2, 11), stage7Range(2, 0, 2, 16), stage7Range(1, 0, 3, 0)}
	if gotRanges := []graph.Range{q.EvidenceRange, q.ItemRange, q.SelectionRange, q.DisplayRange, q.ContextSpan}; !reflect.DeepEqual(gotRanges, wantQuery) {
		t.Fatalf("ASSERT_STAGE7_QUERY_ALL_FIVE_EXACT_RANGES got=%+v want=%+v", gotRanges, wantQuery)
	}
	if gotRanges := []graph.Range{target.EvidenceRange, target.ItemRange, target.SelectionRange, target.DisplayRange, target.ContextSpan}; !reflect.DeepEqual(gotRanges, wantTarget) {
		t.Fatalf("ASSERT_STAGE7_TARGET_ALL_FIVE_EXACT_RANGES got=%+v want=%+v", gotRanges, wantTarget)
	}
	wantQuerySpan := []byte("package p\nfunc caller(){ target() }\n")
	wantTargetSpan := []byte("package p\nfunc target() {}\n")
	if gotBytes, ok := got.SelectedContextBytes(SourceContextRoleQueryReference); !ok || !bytes.Equal(gotBytes, wantQuerySpan) {
		t.Fatalf("ASSERT_STAGE7_QUERY_EXACT_ROLE_SPAN_BYTES ok=%v got=%q want=%q", ok, gotBytes, wantQuerySpan)
	}
	if gotBytes, ok := got.SelectedContextBytes(SourceContextRoleTarget); !ok || !bytes.Equal(gotBytes, wantTargetSpan) {
		t.Fatalf("ASSERT_STAGE7_TARGET_EXACT_ROLE_SPAN_BYTES ok=%v got=%q want=%q", ok, gotBytes, wantTargetSpan)
	}
	documents := got.Documents()
	if len(documents) != 2 || !bytes.Equal(documents[0].ExactBytes, in.Documents[0].ExactBytes) || !bytes.Equal(documents[1].ExactBytes, in.Documents[1].ExactBytes) {
		t.Fatalf("ASSERT_STAGE7_FULL_DOCUMENT_BYTES_PRESERVED documents=%+v", documents)
	}
	if bytes.Equal(wantQuerySpan, in.Documents[0].ExactBytes) || bytes.Equal(wantTargetSpan, in.Documents[1].ExactBytes) {
		t.Fatal("ASSERT_STAGE7_SELECTED_SPANS_STRICT_SUBSTRINGS")
	}
	bindings := got.Bindings()
	if len(bindings) != len(in.Occurrences)*2 {
		t.Fatalf("ASSERT_STAGE7_BINDING_PER_OCCURRENCE_PER_ROLE got=%d want=%d", len(bindings), len(in.Occurrences)*2)
	}
	byOccurrenceRole := make(map[string]SourceContextBinding, len(bindings))
	for _, binding := range bindings {
		key := binding.OccurrenceIdentity + "|" + fmt.Sprint(binding.Role)
		if _, duplicate := byOccurrenceRole[key]; duplicate {
			t.Fatalf("ASSERT_STAGE7_NO_OCCURRENCE_ROLE_DEDUP key=%s", key)
		}
		byOccurrenceRole[key] = binding
	}
	for _, role := range []SourceContextRole{SourceContextRoleQueryReference, SourceContextRoleTarget} {
		a := byOccurrenceRole["repeat-a|"+fmt.Sprint(role)]
		b := byOccurrenceRole["repeat-b|"+fmt.Sprint(role)]
		if a.PointID != b.PointID || a.SpanID != b.SpanID {
			t.Fatalf("ASSERT_STAGE7_EXACT_PREIMAGE_INTERNS_REUSED role=%s a=%+v b=%+v", role, a, b)
		}
		if a.BindingID == b.BindingID {
			t.Fatalf("ASSERT_STAGE7_BINDING_IDENTITY_ORDINAL_MULTIPLICITY role=%s", role)
		}
		loop := byOccurrenceRole["loop|"+fmt.Sprint(role)]
		if loop.EvidenceRange != stage7Range(2, 5, 2, 5) || loop.EvidenceRange.Start != loop.EvidenceRange.End {
			t.Fatalf("ASSERT_STAGE7_GENUINE_ZERO_RANGE_PRESERVED role=%s got=%+v", role, loop.EvidenceRange)
		}
	}
}

func TestStage7AEqualPointDifferentSpansAndCrossRoleBinding(t *testing.T) {
	in := stage7Fixture(t)
	in.QueryReferenceSelection.Candidates[0].DocumentURI = in.TargetSelection.Candidates[0].DocumentURI
	in.QueryReferenceSelection.Candidates[0].DisplayRange = in.TargetSelection.Candidates[0].DisplayRange
	in.QueryReferenceSelection.Candidates[0].ContextSpan = stage7Range(0, 0, 1, 16)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	q, target := got.QueryReference(), got.Target()
	if q.Outcome != SourceContextUnavailable || q.PointID != "" || q.SpanID != "" {
		t.Fatalf("ASSERT_STAGE7_MALFORMED_UTF16_ATOMIC_NONSELECTED q=%+v", q)
	}
	if q.BindingID == target.BindingID {
		t.Fatal("ASSERT_STAGE7_CROSS_ROLE_BINDINGS_INDEPENDENT")
	}
}

func TestStage7AClosedRoleOutcomeMatrix(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*SourceContextInput)
		q, target SourceContextOutcome
		complete  bool
	}{
		{"selected-both", func(*SourceContextInput) {}, SourceContextSelected, SourceContextSelected, true},
		{"query-unavailable-no-supply", func(i *SourceContextInput) { i.Documents = i.Documents[1:] }, SourceContextUnavailable, SourceContextSelected, false},
		{"target-unavailable-no-supply", func(i *SourceContextInput) { i.Documents = i.Documents[:1] }, SourceContextSelected, SourceContextUnavailable, false},
		{"query-ambiguous", func(i *SourceContextInput) {
			c := i.QueryReferenceSelection.Candidates[0]
			c.DisplayRange = stage7Range(0, 0, 1, 0)
			i.QueryReferenceSelection.Candidates = append(i.QueryReferenceSelection.Candidates, c)
		}, SourceContextAmbiguous, SourceContextSelected, false},
		{"target-ambiguous", func(i *SourceContextInput) {
			c := i.TargetSelection.Candidates[0]
			c.DisplayRange = stage7Range(0, 0, 1, 0)
			i.TargetSelection.Candidates = append(i.TargetSelection.Candidates, c)
		}, SourceContextSelected, SourceContextAmbiguous, false},
		{"query-withheld", func(i *SourceContextInput) {
			i.Policy.WithholdURIs = []string{i.QueryReferenceSelection.Candidates[0].DocumentURI}
		}, SourceContextWithheld, SourceContextSelected, false},
		{"target-withheld", func(i *SourceContextInput) {
			i.Policy.WithholdURIs = []string{i.TargetSelection.Candidates[0].DocumentURI}
		}, SourceContextSelected, SourceContextWithheld, false},
		{"query-limit", func(i *SourceContextInput) {
			i.Limits.MaxCandidates = 1
			i.QueryReferenceSelection.Candidates = append(i.QueryReferenceSelection.Candidates, i.QueryReferenceSelection.Candidates[0])
		}, SourceContextLimitOmitted, SourceContextSelected, false},
		{"target-limit", func(i *SourceContextInput) { i.Limits.MaxSpans = 1 }, SourceContextSelected, SourceContextLimitOmitted, false},
		{"empty", func(i *SourceContextInput) {
			i.Occurrences = nil
			i.QueryReferenceSelection.Candidates = nil
			i.TargetSelection.Candidates = nil
		}, SourceContextUnavailable, SourceContextUnavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := stage7Fixture(t)
			tc.mutate(&in)
			got, err := ComputeSourceContext(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.QueryReference().Outcome != tc.q || got.Target().Outcome != tc.target || got.ContextComplete() != tc.complete {
				t.Fatalf("ASSERT_STAGE7_OUTCOME_MATRIX q=%s target=%s complete=%v", got.QueryReference().Outcome, got.Target().Outcome, got.ContextComplete())
			}
		})
	}
}

func TestStage7AAmbiguityNeverChoosesArbitrarily(t *testing.T) {
	in := stage7Fixture(t)
	candidate := in.QueryReferenceSelection.Candidates[0]
	candidate.DisplayRange = stage7Range(0, 0, 1, 0)
	in.QueryReferenceSelection.Candidates = append(in.QueryReferenceSelection.Candidates, candidate)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	q := got.QueryReference()
	if q.Outcome != SourceContextAmbiguous || q.PointID != "" || q.SpanID != "" || q.BindingID != "" {
		t.Fatalf("ASSERT_STAGE7_AMBIGUOUS_HAS_NO_ARBITRARY_SELECTION %+v", q)
	}
}

func TestStage7ACustodySubstitutionsRejectAtomically(t *testing.T) {
	mutations := []struct {
		name  string
		apply func(*PreparedSourceDocument)
	}{
		{"uri", func(d *PreparedSourceDocument) { d.URI += "-x" }}, {"session", func(d *PreparedSourceDocument) { d.SessionID += "-x" }},
		{"generation", func(d *PreparedSourceDocument) { d.Generation++ }},
		{"revision", func(d *PreparedSourceDocument) { d.RevisionIdentity += "-x" }}, {"revision-custody", func(d *PreparedSourceDocument) { d.RevisionCustody = "CALLER_ASSERTED" }},
		{"sha256", func(d *PreparedSourceDocument) { d.SourceSHA256 = "sha256:00" }}, {"byte-length", func(d *PreparedSourceDocument) { d.ByteLength++ }},
		{"position-encoding", func(d *PreparedSourceDocument) { d.PositionEncoding = "utf-8" }}, {"exact-bytes", func(d *PreparedSourceDocument) {
			d.ExactBytes = append([]byte(nil), d.ExactBytes...)
			d.ExactBytes[0] ^= 0xff
		}},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			in := stage7Fixture(t)
			m.apply(&in.Documents[0])
			got, err := ComputeSourceContext(in)
			if err == nil || !reflect.DeepEqual(got, SourceContextResult{}) || len(got.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_STAGE7_CUSTODY_SUBSTITUTION_ATOMIC_ZERO got=%+v err=%v", got, err)
			}
		})
	}
}

func TestStage7AUnequalDocumentVersionsRetainedExactly(t *testing.T) {
	in := stage7Fixture(t)
	in.Documents[0].DocumentVersion = 101
	in.Documents[1].DocumentVersion = 202
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	docs := got.Documents()
	if len(docs) != 2 || docs[0].DocumentVersion != 101 || docs[1].DocumentVersion != 202 {
		t.Fatalf("ASSERT_STAGE7_UNEQUAL_DOCUMENT_VERSIONS_RETAINED docs=%+v", docs)
	}
}

func TestStage7AStrictOccurrenceAdmissionAtomic(t *testing.T) {
	mutations := []struct {
		name  string
		apply func(*AdmittedSourceOccurrence)
	}{
		{"empty-id", func(o *AdmittedSourceOccurrence) { o.Identity = "" }},
		{"closed-kind", func(o *AdmittedSourceOccurrence) { o.Kind = "UNKNOWN" }},
		{"negative-ordinal", func(o *AdmittedSourceOccurrence) { o.Ordinal = -1 }},
		{"zero-multiplicity", func(o *AdmittedSourceOccurrence) { o.Multiplicity = 0 }},
		{"invalid-range", func(o *AdmittedSourceOccurrence) { o.ItemRange = stage7Range(2, 2, 1, 1) }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			in := stage7Fixture(t)
			tc.apply(&in.Occurrences[0])
			got, err := ComputeSourceContext(in)
			stage7AssertAtomicZero(t, got, err, tc.name)
		})
	}
	in := stage7Fixture(t)
	in.TargetSelection.Candidates[0].OccurrenceIdentity = "missing"
	got, err := ComputeSourceContext(in)
	stage7AssertAtomicZero(t, got, err, "candidate-occurrence-reference")
}

func TestStage7AIdentityCanonicalStrictRecomputeReplayAndCopies(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	raw := got.CanonicalBytes()
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Contains(raw, []byte("\r")) {
		t.Fatal("ASSERT_STAGE7_CANONICAL_LF_JSON")
	}
	if got.Digest == "" || got.ByteLength != len(raw) || got.RepresentativeDigest != in.Representative.Digest || got.RepresentativeLength != len(in.Representative.CanonicalBytes()) {
		t.Fatal("ASSERT_STAGE7_STAGE6_IDENTITY_BOUND")
	}
	parsed, err := ParseSourceContext(raw)
	if err != nil || !bytes.Equal(parsed.CanonicalBytes(), raw) {
		t.Fatalf("ASSERT_STAGE7_STRICT_PARSE err=%v", err)
	}
	recomputed, err := RecomputeSourceContext(in)
	if err != nil || !reflect.DeepEqual(recomputed, got) {
		t.Fatalf("ASSERT_STAGE7_RECOMPUTE err=%v", err)
	}
	replayed, err := ReplaySourceContext(in, got)
	if err != nil || !reflect.DeepEqual(replayed, got) {
		t.Fatalf("ASSERT_STAGE7_PURE_REPLAY err=%v", err)
	}
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), '{'), bytes.Replace(raw, []byte("\n"), []byte("\r\n"), 1), bytes.Replace(raw, []byte("\"Digest\""), []byte("\"Unknown\""), 1)} {
		if parsed, parseErr := ParseSourceContext(bad); parseErr == nil || !reflect.DeepEqual(parsed, SourceContextResult{}) {
			t.Fatal("ASSERT_STAGE7_STRICT_PARSE_ATOMIC_ZERO")
		}
	}
	copy1 := got.CanonicalBytes()
	copy1[0] ^= 0xff
	bindings := got.Bindings()
	bindings[0].OccurrenceIdentity = "mutated"
	if bytes.Equal(copy1, got.CanonicalBytes()) || got.Bindings()[0].OccurrenceIdentity == "mutated" {
		t.Fatal("ASSERT_STAGE7_DEFENSIVE_COPIES")
	}
}

func TestStage7AExactBalancedAccountingAndLimits(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	a := got.Accounting()
	if a.InputOccurrences != len(in.Occurrences) || a.Bindings != len(in.Occurrences)*2 || a.SelectedRoles+a.UnavailableRoles+a.AmbiguousRoles+a.WithheldRoles+a.LimitOmittedRoles != 2 || a.PointCandidates != a.InternedPoints+a.ReusedPoints || a.SpanCandidates != a.InternedSpans+a.ReusedSpans || a.Work <= 0 || a.CanonicalBytes != got.ByteLength {
		t.Fatalf("ASSERT_STAGE7_BALANCED_ACCOUNTING %+v", a)
	}
	limits := []struct {
		name  string
		apply func(*SourceContextLimits)
	}{
		{"total-bytes", func(l *SourceContextLimits) { l.MaxTotalBytes = 1 }}, {"documents", func(l *SourceContextLimits) { l.MaxDocuments = 1 }},
		{"occurrences", func(l *SourceContextLimits) { l.MaxOccurrences = 1 }}, {"points", func(l *SourceContextLimits) { l.MaxPoints = 1 }},
		{"spans", func(l *SourceContextLimits) { l.MaxSpans = 1 }}, {"candidates", func(l *SourceContextLimits) { l.MaxCandidates = 0 }},
		{"string", func(l *SourceContextLimits) { l.MaxStringBytes = 3 }}, {"canonical", func(l *SourceContextLimits) { l.MaxCanonicalBytes = 32 }},
		{"work", func(l *SourceContextLimits) { l.MaxWork = 1 }},
	}
	for _, tc := range limits {
		t.Run(tc.name, func(t *testing.T) {
			limited := stage7Fixture(t)
			tc.apply(&limited.Limits)
			first, e1 := ComputeSourceContext(limited)
			second, e2 := ComputeSourceContext(limited)
			if (e1 == nil) != (e2 == nil) || !reflect.DeepEqual(first, second) {
				t.Fatalf("ASSERT_STAGE7_LIMIT_DETERMINISTIC first=%+v/%v second=%+v/%v", first, e1, second, e2)
			}
			if e1 == nil && first.QueryReference().Outcome != SourceContextLimitOmitted && first.Target().Outcome != SourceContextLimitOmitted {
				t.Fatal("ASSERT_STAGE7_LIMIT_MUST_BE_EXPLICIT")
			}
			if e1 != nil && (!reflect.DeepEqual(first, SourceContextResult{}) || !reflect.DeepEqual(second, SourceContextResult{})) {
				t.Fatal("ASSERT_STAGE7_LIMIT_ERROR_ATOMIC_ZERO")
			}
		})
	}
}

func TestStage7AGenuineConcurrencyAndAccessorMutationIsolation(t *testing.T) {
	original := stage7Fixture(t)
	baseline, err := ComputeSourceContext(original)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, wantDigest, wantIdentity := baseline.CanonicalBytes(), baseline.Digest, baseline.Identity
	const n = 32
	inputs := make([]SourceContextInput, n)
	for i := range inputs {
		inputs[i] = stage7Fixture(t)
	}
	results := make([]SourceContextResult, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for pass := 0; pass < 64; pass++ {
				results[i], errs[i] = ComputeSourceContext(inputs[i])
				if errs[i] != nil {
					return
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range results {
		if errs[i] != nil || !bytes.Equal(results[i].CanonicalBytes(), wantBytes) || results[i].Digest != wantDigest || results[i].Identity != wantIdentity {
			t.Fatalf("ASSERT_STAGE7_CONCURRENT_INDEPENDENT_COMPUTE i=%d err=%v", i, errs[i])
		}
	}

	start = make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			for pass := 0; pass < 64; pass++ {
				replayed, replayErr := ReplaySourceContext(stage7Fixture(t), baseline)
				if replayErr != nil || replayed.Digest != wantDigest || replayed.Identity != wantIdentity || !bytes.Equal(replayed.CanonicalBytes(), wantBytes) {
					errs[0] = fmt.Errorf("replay identity drift: %v", replayErr)
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			for pass := 0; pass < 1024; pass++ {
				docs := baseline.Documents()
				bindings := baseline.Bindings()
				roles := baseline.RoleResults()
				if len(docs) > 0 && len(docs[0].ExactBytes) > 0 {
					docs[0].ExactBytes[0] ^= byte(pass)
				}
				if len(bindings) > 0 {
					bindings[0].OccurrenceIdentity = fmt.Sprint(pass)
				}
				if len(roles) > 0 {
					roles[0].PointID = fmt.Sprint(pass)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	if errs[0] != nil || baseline.Digest != wantDigest || baseline.Identity != wantIdentity || !bytes.Equal(baseline.CanonicalBytes(), wantBytes) {
		t.Fatalf("ASSERT_STAGE7_RACE_CLEAN_FIXED_IDENTITY err=%v", errs[0])
	}
}

func stage7AssertAtomicZero(t *testing.T, got SourceContextResult, err error, label string) {
	t.Helper()
	if err == nil || !reflect.DeepEqual(got, SourceContextResult{}) || len(got.CanonicalBytes()) != 0 || len(got.Bindings()) != 0 {
		t.Fatalf("ASSERT_STAGE7_ATOMIC_ZERO label=%s got=%+v err=%v", label, got, err)
	}
}

func stage7CanonicalJSON(t *testing.T, value any) []byte {
	t.Helper()
	changed, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(changed, '\n')
}

func stage7JSONSubstitutions(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	var walk func(any, string, func(any))
	walk = func(value any, path string, replace func(any)) {
		emit := func(sub any) {
			replace(sub)
			out[path] = stage7CanonicalJSON(t, root)
			replace(value)
		}
		switch v := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				old := v[key]
				child := path + "." + key
				walk(old, child, func(sub any) { v[key] = sub })
			}
		case []any:
			if len(v) > 0 {
				emit([]any{})
			}
			for i := range v {
				old := v[i]
				walk(old, fmt.Sprintf("%s[%d]", path, i), func(sub any) { v[i] = sub })
			}
		case string:
			emit(v + "-sub")
		case float64:
			emit(v + 1)
		case bool:
			emit(!v)
		case nil:
			emit("sub")
		default:
			t.Fatalf("ASSERT_STAGE7_JSON_KIND_COVERED path=%s type=%T", path, value)
		}
	}
	walk(root, "$", func(sub any) { root = sub })
	return out
}

func TestStage7AStrictCanonicalParserDecodedSemanticMutations(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	raw := got.CanonicalBytes()
	mutations := stage7JSONSubstitutions(t, raw)
	if len(mutations) < 80 {
		t.Fatalf("ASSERT_STAGE7_JSON_MUTATIONS_NONVACUOUS count=%d", len(mutations))
	}
	paths := make([]string, 0, len(mutations))
	for path, mutated := range mutations {
		paths = append(paths, strings.ToLower(path))
		t.Run(path, func(t *testing.T) {
			parsed, parseErr := ParseSourceContext(mutated)
			stage7AssertAtomicZero(t, parsed, parseErr, "semantic:"+path)
		})
	}
	joined := strings.Join(paths, "\n")
	for _, required := range []string{"document", "role", "candidate", "binding", "point", "span", "account", "identity", "digest", "length", "range"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("ASSERT_STAGE7_JSON_PATH_FAMILY_MISSING family=%s paths=%s", required, joined)
		}
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["UnknownStage7Field"] = true
	unknown := stage7CanonicalJSON(t, object)
	duplicate := append([]byte{'{'}, raw[1:len(raw)-2]...)
	duplicate = append(duplicate, []byte(",\"Digest\":\"duplicate\"}\n")...)
	nonCanonicalOrder := append([]byte("{\"zz\":0,"), raw[1:]...)
	for name, bad := range map[string][]byte{
		"unknown": unknown, "duplicate": duplicate, "trailing": append(append([]byte(nil), raw...), 'x'),
		"non-lf": bytes.TrimSuffix(raw, []byte("\n")), "crlf": bytes.ReplaceAll(raw, []byte("\n"), []byte("\r\n")), "noncanonical-order": nonCanonicalOrder,
	} {
		t.Run(name, func(t *testing.T) {
			parsed, parseErr := ParseSourceContext(bad)
			stage7AssertAtomicZero(t, parsed, parseErr, "strict-"+name)
		})
	}
}

type stage7ReflectionMutation struct {
	label string
	value reflect.Value
}

func stage7SeedValue(typ reflect.Type) reflect.Value {
	switch typ.Kind() {
	case reflect.Pointer:
		v := reflect.New(typ.Elem())
		v.Elem().Set(stage7SeedValue(typ.Elem()))
		return v
	case reflect.Interface:
		return reflect.Zero(typ)
	case reflect.Struct:
		v := reflect.New(typ).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				v.Field(i).Set(stage7SeedValue(v.Field(i).Type()))
			}
		}
		return v
	case reflect.Array:
		v := reflect.New(typ).Elem()
		for i := 0; i < v.Len(); i++ {
			v.Index(i).Set(stage7SeedValue(typ.Elem()))
		}
		return v
	case reflect.Slice:
		v := reflect.MakeSlice(typ, 1, 1)
		v.Index(0).Set(stage7SeedValue(typ.Elem()))
		return v
	case reflect.Map:
		v := reflect.MakeMap(typ)
		v.SetMapIndex(stage7SeedValue(typ.Key()), stage7SeedValue(typ.Elem()))
		return v
	case reflect.String:
		return reflect.ValueOf("seed").Convert(typ)
	case reflect.Bool:
		v := reflect.New(typ).Elem()
		v.SetBool(true)
		return v
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := reflect.New(typ).Elem()
		v.SetInt(1)
		return v
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := reflect.New(typ).Elem()
		v.SetUint(1)
		return v
	default:
		return reflect.Zero(typ)
	}
}

func stage7ReflectionSubstitutions(value reflect.Value, path string) ([]stage7ReflectionMutation, []string) {
	var out []stage7ReflectionMutation
	var skipped []string
	clone := func() reflect.Value { v := reflect.New(value.Type()).Elem(); v.Set(value); return v }
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			v := clone()
			v.Set(stage7SeedValue(value.Type()))
			return []stage7ReflectionMutation{{path + ".<nil>", v}}, nil
		}
		children, childSkipped := stage7ReflectionSubstitutions(value.Elem(), path+".*")
		for _, child := range children {
			v := clone()
			p := reflect.New(value.Type().Elem())
			p.Elem().Set(child.value)
			v.Set(p)
			out = append(out, stage7ReflectionMutation{child.label, v})
		}
		return out, childSkipped
	case reflect.Interface:
		if value.IsNil() {
			return nil, []string{path + ".<nil-interface>"}
		}
		children, childSkipped := stage7ReflectionSubstitutions(value.Elem(), path+".<interface>")
		for _, child := range children {
			v := clone()
			v.Set(child.value)
			out = append(out, stage7ReflectionMutation{child.label, v})
		}
		return out, childSkipped
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			fieldPath := path + "." + value.Type().Field(i).Name
			if value.Type().Field(i).PkgPath != "" {
				skipped = append(skipped, fieldPath)
				continue
			}
			children, childSkipped := stage7ReflectionSubstitutions(value.Field(i), fieldPath)
			skipped = append(skipped, childSkipped...)
			for _, child := range children {
				v := clone()
				v.Field(i).Set(child.value)
				out = append(out, stage7ReflectionMutation{child.label, v})
			}
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			children, childSkipped := stage7ReflectionSubstitutions(value.Index(i), fmt.Sprintf("%s[%d]", path, i))
			skipped = append(skipped, childSkipped...)
			for _, child := range children {
				v := clone()
				v.Index(i).Set(child.value)
				out = append(out, stage7ReflectionMutation{child.label, v})
			}
		}
	case reflect.Slice:
		if value.IsNil() {
			v := reflect.MakeSlice(value.Type(), 1, 1)
			v.Index(0).Set(stage7SeedValue(value.Type().Elem()))
			out = append(out, stage7ReflectionMutation{path + ".<nil-slice>", v})
		} else {
			out = append(out, stage7ReflectionMutation{path + ".<slice-state>", reflect.Zero(value.Type())})
			if value.Len() > 0 {
				out = append(out, stage7ReflectionMutation{path + ".<empty-slice>", reflect.MakeSlice(value.Type(), 0, 0)})
			}
		}
		for i := 0; i < value.Len(); i++ {
			children, childSkipped := stage7ReflectionSubstitutions(value.Index(i), fmt.Sprintf("%s[%d]", path, i))
			skipped = append(skipped, childSkipped...)
			for _, child := range children {
				v := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
				reflect.Copy(v, value)
				v.Index(i).Set(child.value)
				out = append(out, stage7ReflectionMutation{child.label, v})
			}
		}
	case reflect.Map:
		if value.IsNil() {
			out = append(out, stage7ReflectionMutation{path + ".<nil-map>", stage7SeedValue(value.Type())})
		} else {
			out = append(out, stage7ReflectionMutation{path + ".<map-state>", reflect.Zero(value.Type())})
		}
		iter := value.MapRange()
		for iter.Next() {
			key, item := iter.Key(), iter.Value()
			children, childSkipped := stage7ReflectionSubstitutions(item, fmt.Sprintf("%s[%v]", path, key.Interface()))
			skipped = append(skipped, childSkipped...)
			for _, child := range children {
				v := reflect.MakeMapWithSize(value.Type(), value.Len())
				copyIter := value.MapRange()
				for copyIter.Next() {
					v.SetMapIndex(copyIter.Key(), copyIter.Value())
				}
				v.SetMapIndex(key, child.value)
				out = append(out, stage7ReflectionMutation{child.label, v})
			}
		}
	case reflect.String:
		v := clone()
		v.SetString(value.String() + "-sub")
		out = append(out, stage7ReflectionMutation{path, v})
	case reflect.Bool:
		v := clone()
		v.SetBool(!value.Bool())
		out = append(out, stage7ReflectionMutation{path, v})
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := clone()
		v.SetInt(value.Int() + 1)
		out = append(out, stage7ReflectionMutation{path, v})
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := clone()
		v.SetUint(value.Uint() + 1)
		out = append(out, stage7ReflectionMutation{path, v})
	default:
		skipped = append(skipped, path+":"+value.Kind().String())
	}
	return out, skipped
}

func TestStage7AExhaustiveIndependentInputAndReplaySubstitutions(t *testing.T) {
	baselineInput := stage7Fixture(t)
	baseline, err := ComputeSourceContext(baselineInput)
	if err != nil {
		t.Fatal(err)
	}
	inputMutations, inputSkipped := stage7ReflectionSubstitutions(reflect.ValueOf(baselineInput), "input")
	if len(inputMutations) < 80 {
		t.Fatalf("ASSERT_STAGE7_EXHAUSTIVE_INPUT_NONVACUOUS leaves=%d", len(inputMutations))
	}
	for _, skipped := range inputSkipped {
		if !strings.Contains(skipped, "<nil-interface>") && !strings.HasSuffix(skipped, ":invalid") && !strings.HasPrefix(skipped, "input.Representative.") {
			t.Fatalf("ASSERT_STAGE7_INPUT_FIELD_ESCAPED path=%s", skipped)
		}
	}
	alternate := stage7Fixture(t)
	alternateFixture := stage6FixtureForMode("calls-only")
	alternateToken := RepresentativeQualificationFor(alternateFixture, 8)
	alternateRepresentative, alternateErr := stage6Compute(alternateFixture, alternateToken, stage6Verifier(alternateToken, 8))
	if alternateErr != nil {
		t.Fatal(alternateErr)
	}
	alternate.Representative = alternateRepresentative
	alternateGot, alternateReplayErr := ReplaySourceContext(alternate, baseline)
	stage7AssertAtomicZero(t, alternateGot, alternateReplayErr, "input:Representative-whole-object")
	for _, mutation := range inputMutations {
		clean, ok := mutation.value.Interface().(SourceContextInput)
		if !ok {
			t.Fatalf("ASSERT_STAGE7_INPUT_MUTATION_TYPE path=%s type=%s", mutation.label, mutation.value.Type())
		}
		got, replayErr := ReplaySourceContext(clean, baseline)
		if mutation.label == "input.Policy.WithholdURIs.<slice-state>" {
			if replayErr != nil || !bytes.Equal(got.CanonicalBytes(), baseline.CanonicalBytes()) {
				t.Fatalf("ASSERT_STAGE7_NIL_EMPTY_POLICY_NORMALIZATION got=%+v err=%v", got, replayErr)
			}
			continue
		}
		stage7AssertAtomicZero(t, got, replayErr, "input:"+mutation.label)
	}
	resultMutations, resultSkipped := stage7ReflectionSubstitutions(reflect.ValueOf(baseline), "result")
	if len(resultMutations) < 20 {
		t.Fatalf("ASSERT_STAGE7_EXHAUSTIVE_REPLAY_SCALARS_NONVACUOUS leaves=%d", len(resultMutations))
	}
	allowedDerived := map[string]bool{
		"result.representativeCanonical": true, "result.documents": true, "result.occurrences": true, "result.candidates": true,
		"result.bindings": true, "result.roles": true, "result.points": true,
		"result.spans": true, "result.policyWithheld": true, "result.accounting": true,
		"result.canonical": true,
	}
	for _, skipped := range resultSkipped {
		if !allowedDerived[skipped] && !strings.Contains(skipped, "<nil-interface>") {
			t.Fatalf("ASSERT_STAGE7_RESULT_FIELD_ESCAPED path=%s", skipped)
		}
	}
	for _, mutation := range resultMutations {
		cleanPrior, ok := mutation.value.Interface().(SourceContextResult)
		if !ok {
			t.Fatalf("ASSERT_STAGE7_RESULT_MUTATION_TYPE path=%s type=%s", mutation.label, mutation.value.Type())
		}
		got, replayErr := ReplaySourceContext(stage7Fixture(t), cleanPrior)
		stage7AssertAtomicZero(t, got, replayErr, "replay:"+mutation.label)
	}
}

func stage7FieldValue(t *testing.T, value any, names ...string) reflect.Value {
	t.Helper()
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	for _, name := range names {
		if field := v.FieldByName(name); field.IsValid() {
			return field
		}
	}
	t.Fatalf("ASSERT_STAGE7_REQUIRED_FIELD_MISSING type=%T names=%v", value, names)
	return reflect.Value{}
}

func stage7AssertSameNamedField(t *testing.T, binding SourceContextBinding, occurrence AdmittedSourceOccurrence, names ...string) {
	t.Helper()
	bf, of := stage7FieldValue(t, binding, names...), stage7FieldValue(t, occurrence, names...)
	if !reflect.DeepEqual(bf.Interface(), of.Interface()) {
		t.Fatalf("ASSERT_STAGE7_BINDING_FIELD field=%v got=%v want=%v", names, bf.Interface(), of.Interface())
	}
}

func TestStage7ABindingCompleteFieldwiseAndRoleMapping(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	bindings := got.Bindings()
	roles := []SourceContextRole{SourceContextRoleQueryReference, SourceContextRoleTarget}
	if len(bindings) != len(in.Occurrences)*len(roles) {
		t.Fatalf("ASSERT_STAGE7_BINDING_OCCURRENCE_ROLE_CROSS_PRODUCT bindings=%d want=%d", len(bindings), len(in.Occurrences)*len(roles))
	}
	byKey := make(map[string]SourceContextBinding, len(bindings))
	for _, binding := range bindings {
		byKey[binding.OccurrenceIdentity+"|"+fmt.Sprint(binding.Role)] = binding
	}
	for _, occurrence := range in.Occurrences {
		for _, role := range roles {
			binding, ok := byKey[occurrence.Identity+"|"+fmt.Sprint(role)]
			if !ok {
				t.Fatalf("ASSERT_STAGE7_BINDING_MISSING occurrence=%s role=%s", occurrence.Identity, role)
			}
			for _, names := range [][]string{{"Identity", "OccurrenceIdentity"}, {"SchemaID", "Schema"}, {"Digest"}, {"ByteLength", "Length"}, {"RelationID"}, {"PredecessorIdentity"}, {"Kind"}, {"CallerID"}, {"TargetID"}, {"Ordinal"}, {"Multiplicity"}} {
				stage7AssertSameNamedField(t, binding, occurrence, names...)
			}
			if binding.Role != role || binding.EvidenceRange != occurrence.EvidenceRange || binding.ItemRange != occurrence.ItemRange || binding.SelectionRange != occurrence.SelectionRange {
				t.Fatalf("ASSERT_STAGE7_BINDING_ROLE_AND_RANGES occurrence=%+v binding=%+v", occurrence, binding)
			}
			var roleResult any
			if role == SourceContextRoleQueryReference {
				roleResult = got.QueryReference()
			} else {
				roleResult = got.Target()
			}
			for _, names := range [][]string{{"Outcome"}, {"DocumentURI"}, {"PointID"}, {"SpanID"}, {"DisplayRange"}, {"ContextSpan"}} {
				bf, rf := stage7FieldValue(t, binding, names...), stage7FieldValue(t, roleResult, names...)
				if !reflect.DeepEqual(bf.Interface(), rf.Interface()) {
					t.Fatalf("ASSERT_STAGE7_BINDING_ROLE_FIELD occurrence=%s role=%s field=%v got=%v want=%v", occurrence.Identity, role, names, bf.Interface(), rf.Interface())
				}
			}
			wantBytes, ok := got.SelectedContextBytes(role)
			if !ok {
				t.Fatalf("ASSERT_STAGE7_BINDING_ROLE_BYTES_MISSING role=%s", role)
			}
			byteField := stage7FieldValue(t, binding, "ExactBytes", "ContextBytes", "SpanBytes")
			if !bytes.Equal(byteField.Bytes(), wantBytes) {
				t.Fatalf("ASSERT_STAGE7_BINDING_EXACT_BYTES occurrence=%s role=%s got=%q want=%q", occurrence.Identity, role, byteField.Bytes(), wantBytes)
			}
		}
	}
}

func TestStage7ACoordinateFixtures(t *testing.T) {
	cases := []struct {
		name, body string
		span       graph.Range
		want       []byte
		valid      bool
	}{
		{"utf16-non-bmp", "a😀b\n", stage7Range(0, 1, 0, 3), []byte("😀"), true},
		{"crlf", "a\r\nb\r\n", stage7Range(1, 0, 1, 1), []byte("b"), true},
		{"bom", "\ufeffabc\n", stage7Range(0, 0, 0, 1), []byte("\ufeff"), true},
		{"empty-document-eof", "", stage7Range(0, 0, 0, 0), []byte{}, true},
		{"eof-endpoint", "abc", stage7Range(0, 0, 0, 3), []byte("abc"), true},
		{"oob-line", "abc\n", stage7Range(2, 0, 2, 0), nil, false},
		{"oob-character", "abc\n", stage7Range(0, 0, 0, 4), nil, false},
		{"split-surrogate-interior", "😀\n", stage7Range(0, 1, 0, 2), nil, false},
		{"accented-oob-character", "é\n", stage7Range(0, 0, 0, 2), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExactUTF16Slice([]byte(tc.body), tc.span)
			if tc.valid && (err != nil || !bytes.Equal(got, tc.want)) {
				t.Fatalf("ASSERT_STAGE7_COORDINATE_VALID got=%q want=%q err=%v", got, tc.want, err)
			}
			if !tc.valid && (err == nil || got != nil) {
				t.Fatalf("ASSERT_STAGE7_COORDINATE_INVALID got=%q err=%v", got, err)
			}
		})
	}
}

func TestStage7ADefensiveCopiesIncludingNestedAndConcurrentMutation(t *testing.T) {
	in := stage7Fixture(t)
	baseline, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	want := baseline.CanonicalBytes()
	copies := [][]byte{baseline.CanonicalBytes()}
	for _, role := range []SourceContextRole{SourceContextRoleQueryReference, SourceContextRoleTarget} {
		b, ok := baseline.SelectedContextBytes(role)
		if !ok {
			t.Fatalf("missing role %s", role)
		}
		copies = append(copies, b)
	}
	documents := baseline.Documents()
	documents[0].ExactBytes[0] ^= 0xff
	candidates := baseline.Candidates()
	candidates[0].DisplayRange.Start.Line++
	bindings := baseline.Bindings()
	bindings[0].EvidenceRange.Start.Line++
	bindings[0].OccurrenceIdentity = "mutated"
	roles := baseline.RoleResults()
	roles[0].DisplayRange.Start.Line++
	points := baseline.Points()
	points[0].Position.Line++
	spans := baseline.Spans()
	spans[0].Range.Start.Line++
	accounting := baseline.Accounting()
	accounting.SelectedRoles++
	in.Documents[0].ExactBytes[0] ^= 0xff
	in.QueryReferenceSelection.Candidates[0].DisplayRange.Start.Line++
	for _, b := range copies {
		if len(b) > 0 {
			b[0] ^= 0xff
		}
	}
	const readers = 32
	start := make(chan struct{})
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1024; i++ {
			in.Documents[0].ExactBytes[0] ^= byte(i)
			in.QueryReferenceSelection.Candidates[0].DisplayRange.Start.Line++
			for _, b := range copies {
				if len(b) > 0 {
					b[0] ^= byte(i)
				}
			}
		}
	}()
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			clean := stage7Fixture(t)
			replay, e := ReplaySourceContext(clean, baseline)
			if e != nil || !bytes.Equal(replay.CanonicalBytes(), want) {
				errs <- fmt.Errorf("replay mismatch: %v", e)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	if !bytes.Equal(baseline.CanonicalBytes(), want) || bytes.Equal(baseline.Documents()[0].ExactBytes, documents[0].ExactBytes) || baseline.Candidates()[0].DisplayRange.Start.Line == candidates[0].DisplayRange.Start.Line || baseline.Bindings()[0].OccurrenceIdentity == "mutated" || baseline.RoleResults()[0].DisplayRange.Start.Line == roles[0].DisplayRange.Start.Line || baseline.Points()[0].Position.Line == points[0].Position.Line || baseline.Spans()[0].Range.Start.Line == spans[0].Range.Start.Line || baseline.Accounting().SelectedRoles == accounting.SelectedRoles {
		t.Fatal("ASSERT_STAGE7_ALL_RETURNED_AND_INPUT_COPIES_ISOLATED")
	}
}

func TestStage7AEveryLimitExactBoundaryAndNoHiddenPartialLoss(t *testing.T) {
	in := stage7Fixture(t)
	got, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	a := got.Accounting()
	boundaries := []struct {
		name string
		n    int
		set  func(*SourceContextLimits, int)
	}{
		{"document-bytes", len(in.Documents[0].ExactBytes), func(l *SourceContextLimits, n int) { l.MaxDocumentBytes = n }},
		{"total-bytes", len(in.Documents[0].ExactBytes) + len(in.Documents[1].ExactBytes), func(l *SourceContextLimits, n int) { l.MaxTotalBytes = n }},
		{"documents", len(in.Documents), func(l *SourceContextLimits, n int) { l.MaxDocuments = n }},
		{"occurrences", len(in.Occurrences), func(l *SourceContextLimits, n int) { l.MaxOccurrences = n }},
		{"points", a.InternedPoints, func(l *SourceContextLimits, n int) { l.MaxPoints = n }},
		{"spans", a.InternedSpans, func(l *SourceContextLimits, n int) { l.MaxSpans = n }},
		{"candidates", len(in.QueryReferenceSelection.Candidates) + len(in.TargetSelection.Candidates), func(l *SourceContextLimits, n int) { l.MaxCandidates = n }},
		{"string-bytes", a.MaxStringBytes, func(l *SourceContextLimits, n int) { l.MaxStringBytes = n }},
		{"canonical-bytes", got.ByteLength, func(l *SourceContextLimits, n int) { l.MaxCanonicalBytes = n }},
		{"work", a.Work, func(l *SourceContextLimits, n int) { l.MaxWork = n }},
	}
	for _, tc := range boundaries {
		t.Run(tc.name, func(t *testing.T) {
			n := tc.n
			if tc.name == "canonical-bytes" || tc.name == "work" {
				for attempts := 0; attempts < 8; attempts++ {
					probe := stage7Fixture(t)
					tc.set(&probe.Limits, n)
					value, probeErr := ComputeSourceContext(probe)
					if probeErr != nil {
						n += 16
						continue
					}
					measured := value.ByteLength
					if tc.name == "work" {
						measured = value.Accounting().Work
					}
					if measured == n {
						break
					}
					n = measured
				}
			}
			at := stage7Fixture(t)
			tc.set(&at.Limits, n)
			exact, exactErr := ComputeSourceContext(at)
			if exactErr != nil || !exact.ContextComplete() || (tc.name == "canonical-bytes" && exact.ByteLength != n) || (tc.name == "work" && exact.Accounting().Work != n) {
				t.Fatalf("ASSERT_STAGE7_LIMIT_N_ACCEPT n=%d got=%+v err=%v", n, exact, exactErr)
			}
			over := stage7Fixture(t)
			tc.set(&over.Limits, n-1)
			first, e1 := ComputeSourceContext(over)
			second, e2 := ComputeSourceContext(over)
			if (e1 == nil) != (e2 == nil) || !reflect.DeepEqual(first, second) || fmt.Sprint(e1) != fmt.Sprint(e2) {
				t.Fatalf("ASSERT_STAGE7_LIMIT_N_PLUS_ONE_DETERMINISTIC first=%+v/%v second=%+v/%v", first, e1, second, e2)
			}
			if e1 != nil {
				stage7AssertAtomicZero(t, first, e1, tc.name)
			} else if first.QueryReference().Outcome != SourceContextLimitOmitted && first.Target().Outcome != SourceContextLimitOmitted {
				t.Fatalf("ASSERT_STAGE7_LIMIT_EXPLICIT_TYPED_DISPOSITION got=%+v", first)
			}
			la := first.Accounting()
			if e1 == nil && la.InputOccurrences != len(over.Occurrences) {
				t.Fatalf("ASSERT_STAGE7_NO_HIDDEN_PARTIAL_LOSS accounting=%+v", la)
			}
		})
	}
}

func stage7WalkOwnedType(t *testing.T, typ reflect.Type, packagePath string, seen map[reflect.Type]bool, forbidden []string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if seen[typ] {
		return
	}
	seen[typ] = true
	if typ.Kind() == reflect.Func || typ.Kind() == reflect.Interface {
		t.Fatalf("ASSERT_STAGE7_FORBIDDEN_DYNAMIC_TYPE type=%s", typ)
	}
	if typ.Kind() == reflect.Map {
		stage7WalkOwnedType(t, typ.Key(), packagePath, seen, forbidden)
		stage7WalkOwnedType(t, typ.Elem(), packagePath, seen, forbidden)
		return
	}
	if typ.Kind() != reflect.Struct || (typ.PkgPath() != "" && typ.PkgPath() != packagePath) {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, typeName := strings.ToLower(field.Name), strings.ToLower(field.Type.String())
		for _, bad := range forbidden {
			if strings.Contains(name, bad) || strings.Contains(typeName, bad) {
				t.Fatalf("ASSERT_STAGE7_FORBIDDEN_TRANSITIVE_FIELD type=%s field=%s token=%s", typ, field.Name, bad)
			}
		}
		stage7WalkOwnedType(t, field.Type, packagePath, seen, forbidden)
	}
	for i := 0; i < typ.NumMethod(); i++ {
		lower := strings.ToLower(typ.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(lower, bad) {
				t.Fatalf("ASSERT_STAGE7_FORBIDDEN_TRANSITIVE_METHOD type=%s method=%s token=%s", typ, typ.Method(i).Name, bad)
			}
		}
	}
}

func TestStage7ARoleBindingClaimCeilingPermutationAndNoOrphan(t *testing.T) {
	in := stage7Fixture(t)
	baseline, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Authority != 0 || baseline.Accepted || baseline.Completeness != "UNKNOWN" || !baseline.ContextComplete() {
		t.Fatalf("ASSERT_STAGE7_CLAIM_CEILING_DISTINCT authority=%d accepted=%v completeness=%q contextComplete=%v", baseline.Authority, baseline.Accepted, baseline.Completeness, baseline.ContextComplete())
	}
	bindings := baseline.Bindings()
	for _, role := range baseline.RoleResults() {
		if role.Outcome != SourceContextSelected {
			continue
		}
		matches := 0
		for _, binding := range bindings {
			if binding.BindingID == role.BindingID && binding.Role == role.Role && binding.OccurrenceIdentity == map[SourceContextRole]string{SourceContextRoleQueryReference: "repeat-a", SourceContextRoleTarget: "loop"}[role.Role] {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("ASSERT_STAGE7_ROLE_BINDING_ID_EXACT role=%+v matches=%d", role, matches)
		}
	}
	permuted := stage7Fixture(t)
	for left, right := 0, len(permuted.Documents)-1; left < right; left, right = left+1, right-1 {
		permuted.Documents[left], permuted.Documents[right] = permuted.Documents[right], permuted.Documents[left]
	}
	for left, right := 0, len(permuted.Occurrences)-1; left < right; left, right = left+1, right-1 {
		permuted.Occurrences[left], permuted.Occurrences[right] = permuted.Occurrences[right], permuted.Occurrences[left]
	}
	permutedResult, err := ComputeSourceContext(permuted)
	if err != nil || !bytes.Equal(permutedResult.CanonicalBytes(), baseline.CanonicalBytes()) || len(permutedResult.Bindings()) != len(bindings) {
		t.Fatalf("ASSERT_STAGE7_PERMUTATION_CANONICAL_AND_NO_DEDUP err=%v", err)
	}
	limited := stage7Fixture(t)
	limited.Limits.MaxSpans = 0
	omitted, err := ComputeSourceContext(limited)
	if err != nil {
		t.Fatal(err)
	}
	if omitted.QueryReference().Outcome != SourceContextLimitOmitted || len(omitted.Points()) != 0 || len(omitted.Spans()) != 0 {
		t.Fatalf("ASSERT_STAGE7_NO_ORPHAN_POINT_ON_SPAN_OMISSION result=%+v points=%+v spans=%+v", omitted.QueryReference(), omitted.Points(), omitted.Spans())
	}
}

func TestStage7AStructuralNoFallbackSurface(t *testing.T) {
	forbidden := []string{"callback", "filesystem", "workspace", "acquire", "resolve", "reload", "fallback"}
	roots := []reflect.Type{reflect.TypeOf(SourceContextInput{}), reflect.TypeOf(PreparedSourceDocument{}), reflect.TypeOf(RoleSelectionInput{}), reflect.TypeOf(SourceContextResult{})}
	packagePath := roots[0].PkgPath()
	seen := map[reflect.Type]bool{}
	for _, root := range roots {
		stage7WalkOwnedType(t, root, packagePath, seen, forbidden)
	}

	files, err := filepath.Glob(filepath.Join(".", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	parsed := map[string]*ast.File{}
	declarationFile := map[string]string{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		parsed[path] = file
		for _, declaration := range file.Decls {
			switch d := declaration.(type) {
			case *ast.FuncDecl:
				declarationFile[d.Name.Name] = path
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						declarationFile[s.Name.Name] = path
					case *ast.ValueSpec:
						for _, name := range s.Names {
							declarationFile[name.Name] = path
						}
					}
				}
			}
		}
	}
	queue := []string{"ComputeSourceContext", "ReplaySourceContext", "RecomputeSourceContext", "ParseSourceContext", "ExactUTF16Slice", "SourceContextInput", "SourceContextResult", "PreparedSourceDocument", "RoleSelectionInput"}
	ownedFiles, visitedNames := map[string]bool{}, map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visitedNames[name] {
			continue
		}
		visitedNames[name] = true
		path, ok := declarationFile[name]
		if !ok || filepath.Base(path) != "source_context.go" {
			continue
		}
		ownedFiles[path] = true
		ast.Inspect(parsed[path], func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok {
				if _, local := declarationFile[id.Name]; local && !visitedNames[id.Name] {
					queue = append(queue, id.Name)
				}
			}
			return true
		})
	}
	if len(ownedFiles) == 0 {
		t.Fatal("ASSERT_STAGE7_PRODUCTION_OWNER_REQUIRED")
	}
	for path := range ownedFiles {
		file := parsed[path]
		for _, imp := range file.Imports {
			p := strings.Trim(imp.Path.Value, "\"")
			if p == "os" || p == "io/fs" || p == "path" || p == "path/filepath" {
				t.Fatalf("ASSERT_STAGE7_FORBIDDEN_PRODUCTION_IMPORT file=%s import=%s", path, p)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncType:
				if n.Params != nil {
					for _, field := range n.Params.List {
						if _, ok := field.Type.(*ast.FuncType); ok {
							t.Fatalf("ASSERT_STAGE7_FORBIDDEN_CALLBACK_PARAMETER file=%s", path)
						}
						if id, ok := field.Type.(*ast.InterfaceType); ok && id != nil {
							t.Fatalf("ASSERT_STAGE7_FORBIDDEN_INTERFACE_PARAMETER file=%s", path)
						}
					}
				}
			case *ast.Ident:
				lower := strings.ToLower(n.Name)
				for _, bad := range forbidden {
					if strings.Contains(lower, bad) {
						t.Fatalf("ASSERT_STAGE7_FORBIDDEN_TRANSITIVE_AST file=%s identifier=%s token=%s", path, n.Name, bad)
					}
				}
			}
			return true
		})
	}
}

func stage7ResealWire(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	mutate(root)
	root["Identity"], root["Digest"], root["ByteLength"] = "", "", float64(0)
	accounting, ok := root["AccountingValue"].(map[string]any)
	if !ok {
		t.Fatal("ASSERT_STAGE7_WIRE_ACCOUNTING_OBJECT")
	}
	accounting["CanonicalBytes"] = float64(0)
	core, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	digest := digestBytes(core)
	root["Identity"], root["Digest"] = digest, digest
	for i := 0; i < 8; i++ {
		sealed, marshalErr := json.Marshal(root)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		n := len(sealed) + 1
		if int(root["ByteLength"].(float64)) == n && int(accounting["CanonicalBytes"].(float64)) == n {
			return append(sealed, '\n')
		}
		root["ByteLength"], accounting["CanonicalBytes"] = float64(n), float64(n)
	}
	t.Fatal("ASSERT_STAGE7_ADVERSARIAL_RESEAL_CONVERGES")
	return nil
}

func TestStage7AResultStoragePrivateAndZeroSafe(t *testing.T) {
	typ := reflect.TypeOf(SourceContextResult{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			switch field.Type.Kind() {
			case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface:
				t.Fatalf("ASSERT_STAGE7_NO_EXPORTED_IDENTITY_STORAGE field=%s type=%s", field.Name, field.Type)
			}
			lower := strings.ToLower(field.Name)
			for _, token := range []string{"document", "candidate", "binding", "role", "point", "span", "account", "canonical"} {
				if strings.Contains(lower, token) {
					t.Fatalf("ASSERT_STAGE7_NO_EXPORTED_IDENTITY_FIELD field=%s", field.Name)
				}
			}
		}
	}
	var zero SourceContextResult
	if zero.CanonicalBytes() != nil || zero.Documents() != nil || zero.Candidates() != nil || zero.Bindings() != nil || zero.RoleResults() != nil || zero.Points() != nil || zero.Spans() != nil || zero.ContextComplete() {
		t.Fatal("ASSERT_STAGE7_ZERO_VALUE_SAFE")
	}
}

func TestStage7AAdversarialSelfConsistentResealsRejectEveryNestedClass(t *testing.T) {
	baseline, err := ComputeSourceContext(stage7Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(map[string]any){
		"representative-claim": func(root map[string]any) { root["RepresentativeDigest"] = "sha256:resealed" },
		"document": func(root map[string]any) {
			root["DocumentValues"].([]any)[0].(map[string]any)["URI"] = "file:///resealed.go"
		},
		"occurrence": func(root map[string]any) {
			root["OccurrenceValues"].([]any)[0].(map[string]any)["RelationID"] = "resealed-relation"
		},
		"candidate": func(root map[string]any) {
			root["CandidateValues"].([]any)[0].(map[string]any)["OccurrenceIdentity"] = "missing"
		},
		"binding": func(root map[string]any) {
			root["BindingValues"].([]any)[0].(map[string]any)["BindingID"] = "sha256:resealed"
		},
		"role": func(root map[string]any) {
			root["RoleValues"].([]any)[0].(map[string]any)["PointID"] = "sha256:resealed"
		},
		"point": func(root map[string]any) {
			root["PointValues"].([]any)[0].(map[string]any)["PointID"] = "sha256:resealed"
		},
		"span": func(root map[string]any) {
			root["SpanValues"].([]any)[0].(map[string]any)["ExactBytes"] = "cmVzZWFsZWQ="
		},
		"accounting": func(root map[string]any) { root["AccountingValue"].(map[string]any)["Work"] = float64(1) },
		"limits":     func(root map[string]any) { root["Limits"].(map[string]any)["MaxWork"] = float64(1) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			adversarial := stage7ResealWire(t, baseline.CanonicalBytes(), mutate)
			parsed, parseErr := ParseSourceContext(adversarial)
			stage7AssertAtomicZero(t, parsed, parseErr, "self-consistent-reseal:"+name)
		})
	}
}

func TestStage7AWorkExactBoundaryAndParserRecompute(t *testing.T) {
	in := stage7Fixture(t)
	baseline, err := ComputeSourceContext(in)
	if err != nil {
		t.Fatal(err)
	}
	n := baseline.Accounting().Work
	var accepted SourceContextResult
	for attempts := 0; attempts < 8; attempts++ {
		at := stage7Fixture(t)
		at.Limits.MaxWork = n
		accepted, err = ComputeSourceContext(at)
		if err != nil {
			t.Fatalf("ASSERT_STAGE7_WORK_FIXED_POINT_CONSTRUCT n=%d err=%v", n, err)
		}
		if accepted.Accounting().Work == n {
			break
		}
		n = accepted.Accounting().Work
	}
	if accepted.Accounting().Work != n {
		t.Fatalf("ASSERT_STAGE7_WORK_N_ACCEPT n=%d accounting=%+v", n, accepted.Accounting())
	}
	parsed, err := ParseSourceContext(accepted.CanonicalBytes())
	if err != nil || parsed.Accounting().Work != n || !reflect.DeepEqual(parsed.Accounting(), accepted.Accounting()) {
		t.Fatalf("ASSERT_STAGE7_PARSE_RECOMPUTES_WORK parsed=%+v accepted=%+v err=%v", parsed.Accounting(), accepted.Accounting(), err)
	}
	below := stage7Fixture(t)
	below.Limits.MaxWork = n - 1
	first, firstErr := ComputeSourceContext(below)
	second, secondErr := ComputeSourceContext(below)
	if firstErr == nil || secondErr == nil || fmt.Sprint(firstErr) != fmt.Sprint(secondErr) {
		t.Fatalf("ASSERT_STAGE7_WORK_N_MINUS_ONE_DETERMINISTIC first=%+v/%v second=%+v/%v", first, firstErr, second, secondErr)
	}
	stage7AssertAtomicZero(t, first, firstErr, "work-n-minus-one")
	stage7AssertAtomicZero(t, second, secondErr, "work-n-minus-one-repeat")
}

func TestStage7ARepositoryDiffScopeAllowList(t *testing.T) {
	cmd := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
	cmd.Dir = filepath.Join(".", "..", "..")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("ASSERT_STAGE7_DIFF_SCOPE_GIT_STATUS err=%v", err)
	}
	allowed := map[string]bool{
		"internal/programcrepresentative/source_context.go":          true,
		"internal/programcrepresentative/source_context_red_test.go": true,
		"internal/programcrepresentative/stage7b_custody.go":         true,
		"internal/programcrepresentative/stage7b_causal_red_test.go": true,
		"internal/programc/stage7c_artifacts_red_test.go":            true,
		"internal/programc/stage7c_coupling.go":                      true,
		"internal/programc/stage7c_instability.go":                   true,
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		if len(line) < 4 {
			t.Fatalf("ASSERT_STAGE7_DIFF_SCOPE_MALFORMED_STATUS line=%q", line)
		}
		fields := strings.Fields(line)
		path := fields[len(fields)-1]
		if !allowed[path] {
			t.Fatalf("ASSERT_STAGE7_DIFF_SCOPE_FORBIDS_STAGE7B_STAGE7C_PUBLIC_OR_DOC path=%s status=%s", path, line[:2])
		}
	}
	if _, err := os.Stat("source_context_red_test.go"); err != nil {
		t.Fatal(err)
	}
}
