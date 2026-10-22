package censusenumerationprojection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func fixture(t *testing.T) Projection {
	t.Helper()
	start := Position{Line: 3, Character: 4}
	digest, _, err := SymbolIdentityDigest("a.go", start, 12, "secretName", 0)
	if err != nil {
		t.Fatal(err)
	}
	fo := 0
	so := 0
	omitted := uint64(1)
	p, err := Build(Input{SessionID: "session", Generation: 2, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///workspace", Files: []File{{0, "a.go", "incomplete"}, {1, "failed.go", "unreadable"}}, Symbols: []Symbol{{0, 0, start, 12, "incomplete", digest}}, Observations: []Observation{{Dimension: "PROVIDER_COMPLETENESS", State: "KNOWN", Omitted: &omitted}}, ProviderFailures: []ProviderFailure{{Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &so}, {Stage: "DOCUMENT_SUPPLY", Code: "REQUEST_FAILED", FileOrdinal: func() *int { v := fo + 1; return &v }()}}, IncompleteCauses: []IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", FileOrdinal: &fo}, {Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &so}}, RequestMaxNodes: 631, RequestTimeoutMS: 30, TimeoutMS: 60})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCanonicalStrictIdentityPrivacyAndMutationBindings(t *testing.T) {
	p := fixture(t)
	raw, err := EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeCanonical(p)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("not deterministic")
	}
	if bytes.Contains(raw, []byte("secretName")) || bytes.Contains(raw, []byte("file:///workspace")) || bytes.Contains(raw, []byte("/workspace/")) {
		t.Fatalf("private source identity leaked: %s", raw)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func([]byte) []byte{"unknown": func(b []byte) []byte {
		return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"extra":1,"schema_version"`), 1)
	}, "duplicate": func(b []byte) []byte {
		return bytes.Replace(b, []byte(`{"schema_version"`), []byte(`{"schema_version":"x","schema_version"`), 1)
	}, "trailing": func(b []byte) []byte { return append(b, []byte("{}")...) }, "identity": func(b []byte) []byte {
		return bytes.Replace(b, []byte(p.Symbols[0].IdentityDigest), []byte("sha256:"+strings.Repeat("b", 64)), 1)
	}} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(mut(append([]byte(nil), raw...))); err == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
}
func TestProviderFailureReconciliationIsBidirectionalAndExact(t *testing.T) {
	for name, mutate := range map[string]func(*Projection){
		"missing": func(p *Projection) { p.ProviderFailures = p.ProviderFailures[1:] },
		"extra": func(p *Projection) {
			v := 0
			p.Files[0].Disposition = "selected"
			p.FileDispositionCounts = countsFiles(p.Files)
			p.ProviderFailures = append(p.ProviderFailures, ProviderFailure{Stage: "DOCUMENT_SYMBOL_FLATTEN", Code: "NODE_LIMIT", FileOrdinal: &v})
		},
		"mismatch": func(p *Projection) { p.ProviderFailures[0].Code = "UNSUPPORTED" },
		"duplicate": func(p *Projection) {
			v := *p.ProviderFailures[0].FileOrdinal
			p.ProviderFailures = append(p.ProviderFailures, ProviderFailure{Stage: "DOCUMENT_SUPPLY", Code: "CANCELED", FileOrdinal: &v})
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := fixture(t)
			mutate(&p)
			sort.Slice(p.ProviderFailures, func(i, j int) bool { return failureKey(p.ProviderFailures[i]) < failureKey(p.ProviderFailures[j]) })
			p.Work = 0
			raw, err := encodeWithWork(p)
			if err == nil {
				_, err = Decode(raw)
			}
			if err == nil {
				t.Fatal("invalid reconciliation accepted")
			}
		})
	}
}

func TestProjectionPathRejectsNonLocalAndNoncanonicalForms(t *testing.T) {
	for _, candidate := range []string{"../secret.go", "a/../secret.go", "file:/x.go", "file://x.go", "C:", "C:/x.go", "//host/x.go", `a\\b.go`, "./a.go", "a/./b.go", "a//b.go", "a\x00b.go", string([]byte{0xff})} {
		t.Run(strings.ReplaceAll(candidate, "/", "_"), func(t *testing.T) {
			p := fixture(t)
			p.Files[0].Path = candidate
			p.Work = 0
			raw, err := encodeWithWork(p)
			if err == nil {
				_, err = Decode(raw)
			}
			if err == nil {
				t.Fatalf("accepted %q", candidate)
			}
		})
	}
}

func TestBoundsWorkAndReconciliation(t *testing.T) {
	p := fixture(t)
	p.Work++
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("work mutation accepted")
	}
	p = fixture(t)
	p.ProviderFailures[0].SymbolOrdinal = new(int)
	*p.ProviderFailures[0].SymbolOrdinal = 9
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("dangling failure accepted")
	}
	p = fixture(t)
	p.Files[0].Ordinal = 1
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("ordinal gap accepted")
	}
}
func TestExactFileAndSymbolCardinalityCapsAndPlusOne(t *testing.T) {
	files := make([]File, MaxFiles)
	for i := range files {
		files[i] = File{Ordinal: i, Path: fmt.Sprintf("f/%05d.go", i), Disposition: "selected"}
	}
	symbols := make([]Symbol, MaxSymbols)
	for i := range symbols {
		fileOrdinal := i % len(files)
		start := Position{Line: uint32(i / len(files)), Character: uint32(i % len(files))}
		d, _, err := SymbolIdentityDigest(files[fileOrdinal].Path, start, 5, "", i)
		if err != nil {
			t.Fatal(err)
		}
		symbols[i] = Symbol{Ordinal: i, FileOrdinal: fileOrdinal, SelectionStart: start, Kind: 5, Disposition: "non-callable", IdentityDigest: d}
	}
	input := Input{SessionID: "s", Generation: 1, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///workspace", Files: files, Symbols: symbols, RequestMaxNodes: MaxSymbols, RequestTimeoutMS: 1, TimeoutMS: 1}
	if _, err := Build(input); err != nil {
		t.Fatalf("exact caps rejected: %v", err)
	}
	input.Files = append(input.Files, File{Ordinal: MaxFiles, Path: "overflow.go", Disposition: "selected"})
	if _, err := Build(input); err == nil {
		t.Fatal("MaxFiles+1 accepted")
	}
	input.Files = files
	input.Symbols = append(input.Symbols, Symbol{Ordinal: MaxSymbols, FileOrdinal: 0, Disposition: "non-callable", IdentityDigest: "sha256:" + strings.Repeat("0", 64)})
	if _, err := Build(input); err == nil {
		t.Fatal("MaxSymbols+1 accepted")
	}
	if _, ok := add(math.MaxUint64, 1); ok {
		t.Fatal("work overflow accepted")
	}
}

func TestLarge631FixtureUnderLimits(t *testing.T) {
	files := []File{{0, "many.go", "incomplete"}}
	symbols := make([]Symbol, 631)
	for i := range symbols {
		start := Position{Line: uint32(i)}
		d, _, e := SymbolIdentityDigest("many.go", start, 12, "n", i)
		if e != nil {
			t.Fatal(e)
		}
		symbols[i] = Symbol{i, 0, start, 12, "incomplete", d}
	}
	causes := make([]IncompleteCause, 0, len(symbols)+1)
	fo := 0
	causes = append(causes, IncompleteCause{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", FileOrdinal: &fo})
	for i := range symbols {
		ordinal := i
		causes = append(causes, IncompleteCause{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "SYMBOL_LIMIT", SymbolOrdinal: &ordinal})
	}
	p, e := Build(Input{SessionID: "s", Generation: 1, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///workspace", Files: files, Symbols: symbols, IncompleteCauses: causes, RequestMaxNodes: 631, RequestTimeoutMS: 1, TimeoutMS: 1})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := EncodeCanonical(p)
	if e != nil || len(raw) >= MaxCanonicalBytes {
		t.Fatalf("raw=%d err=%v", len(raw), e)
	}
}
func TestPublishDistinctNamespaceCustodyAndCollision(t *testing.T) {
	d := t.TempDir()
	if err := os.Chmod(d, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(d)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p := fixture(t)
	r, err := Publish(root, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.Selector, SelectorPrefix) || r.VerificationStatus != "VERIFIED" || !r.NamespaceAtomic {
		t.Fatalf("%+v", r)
	}
	raw, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(r.Selector)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(root, p); err == nil {
		t.Fatal("collision accepted")
	}
}
func TestInjectedExactEncodeDecodeAndIdentityByteLimits(t *testing.T) {
	p := fixture(t)
	raw, err := EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	oldEncode, oldDecode, oldIdentity := encodeCanonicalLimit, decodeCanonicalLimit, identityInputLimit
	defer func() {
		encodeCanonicalLimit, decodeCanonicalLimit, identityInputLimit = oldEncode, oldDecode, oldIdentity
	}()
	encodeCanonicalLimit = len(raw)
	if _, err := EncodeCanonical(p); err != nil {
		t.Fatalf("exact encode rejected: %v", err)
	}
	encodeCanonicalLimit = len(raw) - 1
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("encode +1 accepted")
	}
	encodeCanonicalLimit = oldEncode
	decodeCanonicalLimit = len(raw)
	if _, err := Decode(raw); err != nil {
		t.Fatalf("exact decode rejected: %v", err)
	}
	decodeCanonicalLimit = len(raw) - 1
	if _, err := Decode(raw); err == nil {
		t.Fatal("decode +1 accepted")
	}
	decodeCanonicalLimit = oldDecode
	identity := identityInputSize(p.Symbols[0], p.Files[0].Path)
	identityInputLimit = identity
	if _, err := EncodeCanonical(p); err != nil {
		t.Fatalf("exact identity rejected: %v", err)
	}
	identityInputLimit = identity - 1
	if _, err := EncodeCanonical(p); err == nil {
		t.Fatal("identity +1 accepted")
	}
}

func TestDocumentSupplyLifecycleCausesBuildCanonicalRoundTripAndOrder(t *testing.T) {
	codes := []string{"SESSION_POISONED", "LIFECYCLE_CONFLICT", "SESSION_UNAVAILABLE"}
	files := make([]File, 5)
	causes := make([]IncompleteCause, 0, 5)
	for i := range files {
		files[i] = File{Ordinal: i, Path: fmt.Sprintf("private/%d.go", i), Disposition: "incomplete"}
	}
	for i, code := range codes {
		ordinal := i
		causes = append(causes, IncompleteCause{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: code, FileOrdinal: &ordinal})
	}
	discoveryOrdinal := 3
	causes = append(causes, IncompleteCause{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", FileOrdinal: &discoveryOrdinal})
	flattenOrdinal := 4
	causes = append(causes, IncompleteCause{Scope: "FILE", Stage: "DOCUMENT_SYMBOL_FLATTEN", Code: "NODE_LIMIT", FileOrdinal: &flattenOrdinal})
	reverseCauses := append([]IncompleteCause(nil), causes...)
	for i := 0; i < len(reverseCauses)/2; i++ {
		reverseCauses[i], reverseCauses[len(reverseCauses)-1-i] = reverseCauses[len(reverseCauses)-1-i], reverseCauses[i]
	}
	build := func(inputCauses []IncompleteCause) Projection {
		t.Helper()
		got, err := Build(Input{SessionID: "session", Generation: 2, RequestFingerprint: "sha256:" + strings.Repeat("a", 64), WorkspaceURI: "file:///private/workspace", Files: files, IncompleteCauses: inputCauses, RequestMaxNodes: 631, RequestTimeoutMS: 30, TimeoutMS: 60})
		if err != nil {
			t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_BUILD: %v", err)
		}
		return got
	}
	first := build(reverseCauses)
	second := build(causes)
	firstRaw, err := EncodeCanonical(first)
	if err != nil {
		t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_ENCODE: %v", err)
	}
	secondRaw, err := EncodeCanonical(second)
	if err != nil || !bytes.Equal(firstRaw, secondRaw) {
		t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_DETERMINISTIC: err=%v first=%q second=%q", err, firstRaw, secondRaw)
	}
	decoded, err := Decode(firstRaw)
	if err != nil {
		t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_STRICT_DECODE: %v", err)
	}
	want := []IncompleteCause{
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_POISONED", FileOrdinal: func() *int { v := 0; return &v }()},
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "LIFECYCLE_CONFLICT", FileOrdinal: func() *int { v := 1; return &v }()},
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: func() *int { v := 2; return &v }()},
		{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", FileOrdinal: func() *int { v := 3; return &v }()},
		{Scope: "FILE", Stage: "DOCUMENT_SYMBOL_FLATTEN", Code: "NODE_LIMIT", FileOrdinal: func() *int { v := 4; return &v }()},
	}
	if !reflect.DeepEqual(decoded.IncompleteCauses, want) {
		t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_EXACT_ORDER_AND_SCALARS: got=%+v want=%+v", decoded.IncompleteCauses, want)
	}
	if bytes.Contains(firstRaw, []byte("file:///private/workspace")) {
		t.Fatalf("ASSERT_DOCUMENT_SUPPLY_CAUSE_PRIVACY: workspace URI in %s", firstRaw)
	}
}

func TestCanonicalPermutationDeepCloneAndFixedPointWork(t *testing.T) {
	p := fixture(t)
	input := Input{SessionID: p.SessionID, Generation: p.Generation, RequestFingerprint: p.RequestFingerprint, WorkspaceURI: "file:///workspace", Files: append([]File(nil), p.Files...), Symbols: append([]Symbol(nil), p.Symbols...), Observations: append([]Observation(nil), p.DiscoveryObservations...), ProviderFailures: append([]ProviderFailure(nil), p.ProviderFailures...), IncompleteCauses: append([]IncompleteCause(nil), p.IncompleteCauses...), RequestMaxNodes: p.Bounds.RequestMaxNodes, RequestTimeoutMS: p.Bounds.RequestTimeoutMS, TimeoutMS: p.Bounds.TimeoutMS}
	reverse := func(n int, swap func(int, int)) {
		for i := 0; i < n/2; i++ {
			swap(i, n-1-i)
		}
	}
	reverse(len(input.Observations), func(i, j int) {
		input.Observations[i], input.Observations[j] = input.Observations[j], input.Observations[i]
	})
	reverse(len(input.ProviderFailures), func(i, j int) {
		input.ProviderFailures[i], input.ProviderFailures[j] = input.ProviderFailures[j], input.ProviderFailures[i]
	})
	reverse(len(input.IncompleteCauses), func(i, j int) {
		input.IncompleteCauses[i], input.IncompleteCauses[j] = input.IncompleteCauses[j], input.IncompleteCauses[i]
	})
	got, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	rawA, _ := EncodeCanonical(p)
	rawB, _ := EncodeCanonical(got)
	if !bytes.Equal(rawA, rawB) {
		t.Fatal("set permutation changed canonical JSON")
	}
	if input.IncompleteCauses[0].FileOrdinal != nil {
		*input.IncompleteCauses[0].FileOrdinal = 99
	} else {
		*input.IncompleteCauses[0].SymbolOrdinal = 99
	}
	if bytes.Equal(rawB, mustEncode(t, got)) == false {
		t.Fatal("input pointer mutation escaped clone")
	}
	identity := uint64(identityInputSize(got.Symbols[0], got.Files[0].Path))
	want := uint64(len(got.Files)+len(got.Symbols)) + identity + uint64(len(rawB))
	if got.Work != want || got.Work >= MaxWork {
		t.Fatalf("work=%d want=%d ceiling=%d", got.Work, want, MaxWork)
	}
}
func mustEncode(t *testing.T, p Projection) []byte {
	t.Helper()
	b, e := EncodeCanonical(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestInjectedPublicationFailureReturnsNoReceiptSelector(t *testing.T) {
	old := publishBoundFile
	defer func() { publishBoundFile = old }()
	publishBoundFile = func(*publication.Root, string, []byte, func([]byte) error) (*publication.BoundFileReceipt, error) {
		return nil, errors.New("injected")
	}
	r, err := Publish(nil, fixture(t))
	if err == nil || r.Selector != "" || r.Digest != "" || r.Status != "" {
		t.Fatalf("receipt=%+v err=%v", r, err)
	}
}

func TestPrivateEncodingFailureCategoryClosedScalarAndPublicBytesStable(t *testing.T) {
	baseline, err := MarshalReceipt(Unavailable("ENCODE_FAILED"))
	if err != nil {
		t.Fatal(err)
	}
	field, ok := reflect.TypeOf(Receipt{}).FieldByName("EncodingFailureCategory")
	if !ok || field.Tag.Get("json") != "-" {
		t.Fatalf("ASSERT_PRIVATE_ENCODING_CATEGORY_JSON_EXCLUDED: field=%+v ok=%v", field, ok)
	}
	valid := fixture(t)
	oldLimit := encodeCanonicalLimit
	encodeCanonicalLimit = 1
	_, boundErr := EncodeCanonical(valid)
	encodeCanonicalLimit = oldLimit
	invalid := valid
	invalid.Symbols[0].FileOrdinal = len(invalid.Files)
	_, invalidErr := EncodeCanonical(invalid)
	for _, tc := range []struct {
		want string
		err  error
	}{
		{want: "BOUND_EXHAUSTION", err: boundErr},
		{want: "INVALID_INPUT", err: invalidErr},
		{want: "OTHER_ENCODING_FAILURE", err: errors.New("provider leak sentinel")},
	} {
		want := tc.want
		if got := string(EncodingFailureCategoryOf(tc.err)); got != want {
			t.Fatalf("ASSERT_ENCODING_CLASSIFIER_%s: got=%q", want, got)
		}
		r, categoryErr := UnavailableEncodingFailure(EncodingFailureCategory(want))
		if categoryErr != nil {
			t.Fatalf("ASSERT_ENCODING_FAILURE_CATEGORY_%s: %v", want, categoryErr)
		}
		value := reflect.ValueOf(r).FieldByName("EncodingFailureCategory")
		if !value.IsValid() || value.String() != want {
			t.Fatalf("ASSERT_ENCODING_FAILURE_CATEGORY_%s: got=%q", want, value.String())
		}
		raw, marshalErr := MarshalReceipt(r)
		if marshalErr != nil || !bytes.Equal(raw, baseline) || bytes.Contains(raw, []byte(want)) || bytes.Contains(raw, []byte("provider leak sentinel")) {
			t.Fatalf("ASSERT_PUBLIC_RECEIPT_BYTES_UNCHANGED_%s: raw=%q baseline=%q err=%v", want, raw, baseline, marshalErr)
		}
	}
	if got, err := UnavailableEncodingFailure(EncodingFailureCategory("provider supplied category")); err == nil || got != (Receipt{}) {
		t.Fatalf("ASSERT_INVALID_EXPLICIT_ENCODING_CATEGORY_FAILS_CLOSED: receipt=%+v err=%v", got, err)
	}
}

func TestReceiptClosedUnavailableAndStrictJSONRoundTrip(t *testing.T) {
	for _, reason := range []string{"ENCODE_FAILED", "PUBLICATION_FAILED", "CANCELED_PRE_RECONCILIATION", "CANCELED_PRE_PUBLICATION"} {
		raw, err := MarshalReceipt(Unavailable(reason))
		if err != nil || !json.Valid(raw) {
			t.Fatalf("%s %q %v", reason, raw, err)
		}
	}
	if _, err := MarshalReceipt(Unavailable("raw server prose")); err == nil {
		t.Fatal("open reason accepted")
	}
}
