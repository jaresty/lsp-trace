package adr0011acquisition

import (
	"bytes"
	"testing"

	"lsp-trace/internal/publication"
)

func TestRawScannerGrammarAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name, raw, form string
		count           int
	}{
		{"null", "null", "NULL", 0}, {"empty", "[]", "ARRAY", 0},
		{"equal-locations", `[{"uri":"x"},{"uri":"x"}]`, "ARRAY", 2},
		{"managed-fixture", `[{"uri":"file:///ref.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}},{"uri":"file:///ref.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}]`, "ARRAY", 2},
		{"nested", `[{"a":[1,2,{"b":"a,\\\"b"}]},[3,4]]`, "ARRAY", 2},
		{"trailing", `[] true`, "MALFORMED", 0}, {"invalid", `[1,]`, "MALFORMED", 0},
		{"duplicate-keys", `[{"a":1,"\u0061":2}]`, "MALFORMED", 0},
		{"scalar", `true`, "MALFORMED", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scanRawTopLevel([]byte(tc.raw))
			if got.form != tc.form || got.knownE != tc.count {
				t.Fatalf("ASSERT_SCANNER_%s: %+v", tc.name, got)
			}
		})
	}
	at := append([]byte("["), bytes.Repeat([]byte(" "), rawResultPayloadLimit-2)...)
	at = append(at, ']')
	if got := scanRawTopLevel(at); got.form != "ARRAY" || got.workUnits != rawResultPayloadLimit {
		t.Fatalf("ASSERT_SCANNER_EDGE: %+v", got)
	}
	if got := scanRawTopLevel(append(at, ' ')); got.form != "MALFORMED" || got.workUnits > rawResultPayloadLimit {
		t.Fatalf("ASSERT_SCANNER_OVER: %+v", got)
	}
}

func TestRawScannerRetainedBinding(t *testing.T) {
	root := privatePublicationRoot(t)
	f, p, b := ownerReadFixture(t, "textDocument/references")
	ref, err := publishOwnerRead(root, f, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := publishRawResultPayload(root, ref, p, b, "owned", 7, f.invocation, reviewedSuccessorSchemaDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, e := observeRawScanner(root, raw, nil); e != nil || got.form != "NULL" || got.workUnits != 4 || got.implementationDigest != "" {
		t.Fatalf("ASSERT_SCANNER_BOUND: %+v %v", got, e)
	}
	if _, e := observeRawScanner(root, privateBodyPublication{selector: raw.selector, digest: privateDigest([]byte("[]")), byteCount: raw.byteCount, stage: "VERIFIED"}, nil); e == nil {
		t.Fatal("ASSERT_SCANNER_DIGEST")
	}
	if _, e := observeRawScanner(root, privateBodyPublication{selector: raw.selector, digest: raw.digest, byteCount: raw.byteCount, stage: "ABSENT"}, nil); e == nil {
		t.Fatal("ASSERT_SCANNER_ABSENT")
	}
	if _, e := observeRawScanner(root, raw, func(*publication.Root, string, int64) ([]byte, error) { return []byte("[]"), nil }); e == nil {
		t.Fatal("ASSERT_SCANNER_SUBSTITUTED")
	}
}
