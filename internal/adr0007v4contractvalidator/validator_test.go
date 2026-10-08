package adr0007v4contractvalidator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawCorpusMatrix50(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4")
	var matrix []struct {
		ID              string `json:"id"`
		ExpectedOutcome string `json:"expected_outcome"`
		ExpectedCode    string `json:"expected_code"`
	}
	mb, err := os.ReadFile(filepath.Join(root, "cases", "CASE_MATRIX.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mb, &matrix); err != nil {
		t.Fatal(err)
	}
	if len(matrix) != 50 {
		t.Fatalf("matrix count = %d, want 50", len(matrix))
	}
	payload := mustReadString(t, filepath.Join(root, "PAYLOAD_MANIFEST.json"))
	tooling := mustReadString(t, filepath.Join(root, "TOOLING_CENSUS.json"))
	predecessor := mustReadString(t, filepath.Join(root, "FREEZE_DESIGN.md"))
	hashes := map[string]string{}
	for _, tc := range matrix {
		raw := mustReadString(t, filepath.Join(root, "cases", tc.ID, "attempt.json"))
		got, err := Derive(DeriveInput{RawAttemptBytes: raw, AdmittedSourceBytes: sourceMapForRaw(t, raw), AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: payload, ToolingManifestBytes: tooling, PredecessorManifestBytes: predecessor})
		if err != nil {
			t.Fatalf("%s derive: %v", tc.ID, err)
		}
		var term Terminal
		if err := json.Unmarshal([]byte(got), &term); err != nil {
			t.Fatalf("%s terminal json: %v", tc.ID, err)
		}
		if term.Terminal != tc.ExpectedOutcome {
			t.Fatalf("%s outcome = %s want %s", tc.ID, term.Terminal, tc.ExpectedOutcome)
		}
		code := ""
		if term.Failure != nil {
			code = term.Failure.Code
		}
		if code != tc.ExpectedCode {
			t.Fatalf("%s code = %s want %s", tc.ID, code, tc.ExpectedCode)
		}
		h := sha256.Sum256([]byte(got))
		hashes[tc.ID] = "sha256:" + hex.EncodeToString(h[:])
	}
	if len(hashes) != 50 {
		t.Fatalf("hash count = %d, want 50", len(hashes))
	}
	for _, tc := range matrix {
		t.Logf("%s %s", tc.ID, hashes[tc.ID])
	}
}

func TestConformanceFixtures(t *testing.T) {
	pos, _ := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "*.json"))
	if len(pos) != 50 {
		t.Fatalf("positive fixture count = %d, want 50", len(pos))
	}
	for _, p := range pos {
		if err := ValidateBundleFile(p); err != nil {
			t.Fatalf("positive %s rejected: %v", p, err)
		}
	}
	neg, _ := filepath.Glob(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "negative", "*.json"))
	if len(neg) == 0 {
		t.Fatalf("negative fixture count = 0")
	}
	for _, p := range neg {
		bun, err := loadBundleForTest(p)
		if err != nil {
			t.Fatal(err)
		}
		err = ValidateBundle(bun)
		if err == nil {
			t.Fatalf("negative %s accepted", p)
		}
		var ve *VError
		if !errors.As(err, &ve) {
			t.Fatalf("negative %s returned non-VError %v", p, err)
		}
		_ = ve
	}
}

func TestDeriveIsCanonicalTerminalAuthority(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	derived, err := Derive(deriveInputFromBundle(bun))
	if err != nil {
		t.Fatal(err)
	}
	if derived != bun.TerminalBytes {
		t.Fatalf("derived terminal mismatch")
	}
	var term Terminal
	if err := json.Unmarshal([]byte(bun.TerminalBytes), &term); err != nil {
		t.Fatal(err)
	}
	term.Request.Query = "other"
	mut, err := CanonicalJSON(term)
	if err != nil {
		t.Fatal(err)
	}
	bun.TerminalBytes = mut
	if err := ValidateBundle(bun); err == nil {
		t.Fatal("terminal not derived from raw attempt accepted")
	} else {
		var ve *VError
		if !errors.As(err, &ve) || ve.Code != "ASSOCIATION_FAILED" || ve.Path != "/matches/literal" {
			t.Fatalf("got %v", err)
		}
	}
}

func TestSchemaWalkerDetectsMutation(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	bun.SchemaBytes = []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"mutated","required":[],"properties":{}}`)
	err = ValidateBundle(bun)
	if err == nil {
		t.Fatal("mutated schema accepted")
	}
	var ve *VError
	if !errors.As(err, &ve) || ve.Path != "/schema_bytes/$id" {
		t.Fatalf("schema mutation got %v", err)
	}
}

func TestRawArtifactAndTerminalMutationsReject(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	mutators := map[string]func(Bundle) Bundle{
		"source-bytes-mismatch": func(b Bundle) Bundle {
			for k := range b.AdmittedSourceBytes {
				b.AdmittedSourceBytes[k] = b.AdmittedSourceBytes[k] + "x"
				break
			}
			return b
		},
		"source-bytes-missing": func(b Bundle) Bundle {
			for k := range b.AdmittedSourceBytes {
				delete(b.AdmittedSourceBytes, k)
				break
			}
			return b
		},
		"source-bytes-extra": func(b Bundle) Bundle { b.AdmittedSourceBytes["extra"] = "x"; return b },
		"binding-arbitrary":  func(b Bundle) Bundle { b.AdmittedBindingBytes = "admission\n"; return b },
		"payload-tooling-swap": func(b Bundle) Bundle {
			b.PayloadFreezeBytes, b.ToolingManifestBytes = b.ToolingManifestBytes, b.PayloadFreezeBytes
			return b
		},
		"terminal-query": func(b Bundle) Bundle {
			var term Terminal
			_ = json.Unmarshal([]byte(b.TerminalBytes), &term)
			term.Request.Query = "other"
			b.TerminalBytes, _ = CanonicalJSON(term)
			return b
		},
		"candidate-digest": func(b Bundle) Bundle {
			var term Terminal
			_ = json.Unmarshal([]byte(b.TerminalBytes), &term)
			term.RangeUnionCandidate.CandidateDigest = zeroSHA
			b.TerminalBytes, _ = CanonicalJSON(term)
			return b
		},
		"location-pin": func(b Bundle) Bundle {
			var term Terminal
			_ = json.Unmarshal([]byte(b.TerminalBytes), &term)
			term.RangeUnionCandidate.QualifiedLocationPins[0].StartByte++
			b.TerminalBytes, _ = CanonicalJSON(term)
			return b
		},
		"accounting-counter": func(b Bundle) Bundle {
			var term Terminal
			_ = json.Unmarshal([]byte(b.TerminalBytes), &term)
			term.Accounting.TScannedTuples++
			b.TerminalBytes, _ = CanonicalJSON(term)
			return b
		},
	}
	for name, mutate := range mutators {
		if err := ValidateBundle(mutate(cloneBundleForMutation(bun))); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func cloneBundleForMutation(b Bundle) Bundle {
	clone := b
	clone.SchemaBytes = append([]byte(nil), b.SchemaBytes...)
	clone.AdmittedSourceBytes = map[string]string{}
	for k, v := range b.AdmittedSourceBytes {
		clone.AdmittedSourceBytes[k] = v
	}
	return clone
}

func TestNoCaseNamesInDerivationSource(t *testing.T) {
	body, err := os.ReadFile("validator.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "case-") || strings.Contains(string(body), "raw_malformed_json") || strings.Contains(string(body), "max_files_plus_one") {
		t.Fatal("derivation source contains corpus case names")
	}
}

func TestSampleComparisonCountsAndAdmissionDigests(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4")
	for _, id := range []string{"case-13-utf8_nfc_path", "case-25-overlap_literal", "case-36-max_files_plus_one"} {
		raw := mustReadString(t, filepath.Join(root, "cases", id, "attempt.json"))
		got, err := Derive(DeriveInput{RawAttemptBytes: raw, AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: mustReadString(t, filepath.Join(root, "PAYLOAD_MANIFEST.json")), ToolingManifestBytes: mustReadString(t, filepath.Join(root, "TOOLING_CENSUS.json")), PredecessorManifestBytes: mustReadString(t, filepath.Join(root, "FREEZE_DESIGN.md"))})
		if err != nil {
			t.Fatal(err)
		}
		var term Terminal
		if err := json.Unmarshal([]byte(got), &term); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s comparisons=%d admission=%s", id, term.Accounting.TScannedTuples, term.Admission.AdmissionDigest)
	}
}

func TestControlAndZeroComparisonEdges(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4")
	payload := mustReadString(t, filepath.Join(root, "PAYLOAD_MANIFEST.json"))
	tooling := mustReadString(t, filepath.Join(root, "TOOLING_CENSUS.json"))
	pred := mustReadString(t, filepath.Join(root, "FREEZE_DESIGN.md"))
	base := mustReadString(t, filepath.Join(root, "cases", "case-13-utf8_nfc_path", "attempt.json"))
	var a rawAttempt
	if err := json.Unmarshal([]byte(base), &a); err != nil {
		t.Fatal(err)
	}
	a.ExecutionControl.Observations = []rawObservation{{PollIndex: 0, Cancelled: true}}
	raw, _ := CanonicalJSON(a)
	got, err := Derive(DeriveInput{RawAttemptBytes: raw, AdmittedSourceBytes: sourceMapForRaw(t, raw), AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: payload, ToolingManifestBytes: tooling, PredecessorManifestBytes: pred})
	if err != nil {
		t.Fatal(err)
	}
	var term Terminal
	_ = json.Unmarshal([]byte(got), &term)
	if term.Failure == nil || term.Failure.Code != "CANCELLED" || term.Accounting.TScannedTuples != 0 {
		t.Fatalf("cancel before compare got %#v T=%d", term.Failure, term.Accounting.TScannedTuples)
	}
	a.ExecutionControl.Observations = nil
	a.Request.Query = "needle-that-is-longer-than-source"
	raw, _ = CanonicalJSON(a)
	got, err = Derive(DeriveInput{RawAttemptBytes: raw, AdmittedSourceBytes: sourceMapForRaw(t, raw), AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: payload, ToolingManifestBytes: tooling, PredecessorManifestBytes: pred})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(got), &term)
	if term.Accounting.TScannedTuples != 0 {
		t.Fatalf("long query comparisons=%d", term.Accounting.TScannedTuples)
	}
	a.ExecutionControl.Observations = []rawObservation{{PollIndex: 999, Cancelled: true}}
	raw, _ = CanonicalJSON(a)
	got, err = Derive(DeriveInput{RawAttemptBytes: raw, AdmittedSourceBytes: sourceMapForRaw(t, raw), AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: payload, ToolingManifestBytes: tooling, PredecessorManifestBytes: pred})
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(got), &term)
	if term.Failure == nil || term.Failure.Code != "INVALID_INPUT" {
		t.Fatalf("unconsumed observation got %#v", term.Failure)
	}
}

func TestReflectionTerminalLeafMutationsReject(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	var term any
	if err := json.Unmarshal([]byte(bun.TerminalBytes), &term); err != nil {
		t.Fatal(err)
	}
	paths := leafPaths(term, nil)
	for _, pth := range paths {
		mutated := cloneJSON(term)
		mutateLeaf(mutated, pth)
		b, _ := CanonicalJSON(mutated)
		mb := bun
		mb.TerminalBytes = b
		if err := ValidateBundle(mb); err == nil {
			t.Fatalf("leaf mutation accepted at %v", pth)
		}
	}
	t.Logf("terminal leaf mutations=%d", len(paths))
}

func TestExactSourceMapIsOperativeAndCopied(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string]string{}
	for k, v := range bun.AdmittedSourceBytes {
		bad[k] = v + "x"
	}
	badTerminal, err := Derive(DeriveInput{RawAttemptBytes: bun.RawAttemptBytes, AdmittedSourceBytes: bad, AdmittedBindingBytes: bun.AdmittedBindingBytes, PayloadFreezeBytes: bun.PayloadFreezeBytes, ToolingManifestBytes: bun.ToolingManifestBytes, PredecessorManifestBytes: bun.PredecessorManifestBytes})
	if err != nil {
		t.Fatal(err)
	}
	var badTerm Terminal
	if err := json.Unmarshal([]byte(badTerminal), &badTerm); err != nil || badTerm.Terminal != "FAILED" || badTerm.Failure == nil || badTerm.Failure.Code != "ASSOCIATION_FAILED" {
		t.Fatalf("mutated admitted source map accepted: err=%v terminal=%s failure=%+v", err, badTerm.Terminal, badTerm.Failure)
	}
	good := map[string]string{}
	for k, v := range bun.AdmittedSourceBytes {
		good[k] = v
	}
	got, err := Derive(DeriveInput{RawAttemptBytes: bun.RawAttemptBytes, AdmittedSourceBytes: good, AdmittedBindingBytes: bun.AdmittedBindingBytes, PayloadFreezeBytes: bun.PayloadFreezeBytes, ToolingManifestBytes: bun.ToolingManifestBytes, PredecessorManifestBytes: bun.PredecessorManifestBytes})
	if err != nil {
		t.Fatal(err)
	}
	for k := range good {
		good[k] = "changed after derive"
	}
	var term Terminal
	if err := json.Unmarshal([]byte(got), &term); err != nil || term.Terminal != "COMPLETE" {
		t.Fatalf("derive did not use copied source map data: %v %s", err, term.Terminal)
	}
}

func TestDeriveRejectsNamedArtifactRoleSwaps(t *testing.T) {
	bun, err := loadBundleForTest(filepath.Join("..", "..", "docs", "pilot", "adr0007", "source-text-search-v4", "contracts", "fixtures", "positive", "case-13-utf8_nfc_path.json"))
	if err != nil {
		t.Fatal(err)
	}
	in := deriveInputFromBundle(bun)
	in.PayloadFreezeBytes, in.ToolingManifestBytes = in.ToolingManifestBytes, in.PayloadFreezeBytes
	if _, err := Derive(in); err == nil {
		t.Fatal("swapped named artifacts accepted")
	}
}

func TestArithmeticProbeEnums(t *testing.T) {
	for _, c := range []ProbeCounter{ProbeCounterBOutputBytes, ProbeCounterWWork, ProbeCounterTScannedTuples} {
		raw := []byte(`{"schema_version":"lsp-trace.adr0007.source-text-search.test-control.private.v4","counter":"` + string(c) + `","initial":1,"increment":2}`)
		var p ArithmeticProbeControl
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("%s rejected: %v", c, err)
		}
	}
	var p ArithmeticProbeControl
	if err := json.Unmarshal([]byte(`{"schema_version":"lsp-trace.adr0007.source-text-search.test-control.private.v4","counter":"bad","initial":1,"increment":2}`), &p); err == nil {
		t.Fatal("invalid counter accepted")
	}
}

func TestDeriveDoesNotReferenceCandidateTerminalBytes(t *testing.T) {
	body, err := os.ReadFile("validator.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "func Derive(")
	if start < 0 {
		t.Fatal("Derive function not found")
	}
	rest := text[start:]
	next := strings.Index(rest[len("func Derive("):], "\nfunc ")
	if next >= 0 {
		rest = rest[:len("func Derive(")+next]
	}
	for _, forbidden := range []string{"TerminalBytes", "terminal_bytes", "json.NewDecoder(strings.NewReader(b.TerminalBytes))"} {
		if strings.Contains(rest, forbidden) {
			t.Fatalf("Derive references forbidden terminal seed %q", forbidden)
		}
	}
}

func sourceMapForRaw(t *testing.T, raw string) map[string]string {
	t.Helper()
	a, err := parseRawAttempt(raw)
	if err != nil || a.Request.Query == "" || len(a.Request.Sources) == 0 || len(a.SourceInputs) == 0 || badControl(a.ExecutionControl.Observations) {
		return map[string]string{}
	}
	selected, err := associateSources(a, nil)
	if err != nil {
		return map[string]string{}
	}
	admitted, _, _, ok, _ := admitSources(selected, a.Request.Limits)
	if !ok {
		return map[string]string{}
	}
	m := map[string]string{}
	for i, s := range admitted {
		m[deriveSourceID(s.ref.Path, s.ref.Revision, uint64(i))] = string(s.data)
	}
	return m
}

func bindingForRaw(t *testing.T, raw string) string {
	t.Helper()
	a, err := parseRawAttempt(raw)
	if err != nil || a.Request.Query == "" || len(a.Request.Sources) == 0 || len(a.SourceInputs) == 0 || badControl(a.ExecutionControl.Observations) {
		return ""
	}
	selected, err := associateSources(a, nil)
	if err != nil {
		return ""
	}
	_, binding, _, ok, _ := admitSources(selected, a.Request.Limits)
	if !ok {
		return ""
	}
	return binding
}

func mustReadString(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func leafPaths(v any, prefix []any) [][]any {
	switch x := v.(type) {
	case map[string]any:
		var out [][]any
		for k, vv := range x {
			out = append(out, leafPaths(vv, append(append([]any{}, prefix...), k))...)
		}
		return out
	case []any:
		var out [][]any
		for i, vv := range x {
			out = append(out, leafPaths(vv, append(append([]any{}, prefix...), i))...)
		}
		return out
	default:
		return [][]any{prefix}
	}
}
func cloneJSON(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
func mutateLeaf(v any, p []any) {
	cur := v
	for _, e := range p[:len(p)-1] {
		switch k := e.(type) {
		case string:
			cur = cur.(map[string]any)[k]
		case int:
			cur = cur.([]any)[k]
		}
	}
	last := p[len(p)-1]
	set := func(old any) any {
		switch old.(type) {
		case string:
			return "__mutated__"
		case bool:
			return !old.(bool)
		case nil:
			return "not-null"
		default:
			return float64(999999)
		}
	}
	switch k := last.(type) {
	case string:
		m := cur.(map[string]any)
		m[k] = set(m[k])
	case int:
		a := cur.([]any)
		a[k] = set(a[k])
	}
}

func loadBundleForTest(p string) (Bundle, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return Bundle{}, err
	}
	var bun Bundle
	if err := json.Unmarshal(b, &bun); err != nil {
		return Bundle{}, err
	}
	return bun, nil
}
