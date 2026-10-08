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
		got, err := Derive(DeriveInput{RawAttemptBytes: raw, AdmittedBindingBytes: bindingForRaw(t, raw), PayloadFreezeBytes: payload, ToolingManifestBytes: tooling, PredecessorManifestBytes: predecessor})
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
		if err := ValidateBundle(mutate(bun)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
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
