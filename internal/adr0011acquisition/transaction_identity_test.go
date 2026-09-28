package adr0011acquisition

import (
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
)

func TestReferencesTransactionIdentityGoldenAndSubstitution(t *testing.T) {
	source, revision, _, _ := targetIdentityFixture()
	key := lspwire.RequestKey{Generation: 7, ID: 31}
	occurrence := "sha256:" + strings.Repeat("1", 64)
	got, err := referencesTransactionID("s", 7, key, "inv-1", occurrence, source, revision)
	// Independent Python struct.pack('>Q') + sorted-key compact JSON + SHA256.
	if err != nil || got != "sha256:50ef77d7f5e960be79dc62d50f1a3bbf48e34b6c3629e0e64be16a55496d2390" {
		t.Fatalf("ASSERT_TRANSACTION_INDEPENDENT_GOLDEN: %s %v", got, err)
	}
	for name, test := range map[string]func() (string, error){
		"invocation": func() (string, error) {
			return referencesTransactionID("s", 7, key, "inv-2", occurrence, source, revision)
		},
		"query": func() (string, error) {
			return referencesTransactionID("s", 7, key, "inv-1", "sha256:"+strings.Repeat("2", 64), source, revision)
		},
		"request-key": func() (string, error) {
			k := key
			k.ID++
			return referencesTransactionID("s", 7, k, "inv-1", occurrence, source, revision)
		},
		"source": func() (string, error) {
			x := source
			x.Digest = "sha256:" + strings.Repeat("c", 64)
			x.Selector = sourceRecordSelector("source", x.Digest)
			return referencesTransactionID("s", 7, key, "inv-1", occurrence, x, revision)
		},
		"revision": func() (string, error) {
			x := revision
			x.Digest = "sha256:" + strings.Repeat("c", 64)
			x.Selector = sourceRecordSelector("revision", x.Digest)
			return referencesTransactionID("s", 7, key, "inv-1", occurrence, source, x)
		},
	} {
		t.Run(name, func(t *testing.T) {
			other, err := test()
			if err != nil || other == got {
				t.Fatalf("ASSERT_TRANSACTION_SUBSTITUTION: %s %v", other, err)
			}
		})
	}
	for name, test := range map[string]func() (string, error){
		"generation": func() (string, error) {
			return referencesTransactionID("s", 8, key, "inv-1", occurrence, source, revision)
		},
		"overflow": func() (string, error) {
			k := lspwire.RequestKey{Generation: maxCanonicalInteger + 1, ID: 31}
			return referencesTransactionID("s", k.Generation, k, "inv-1", occurrence, source, revision)
		},
		"wire-id-overflow": func() (string, error) {
			k := lspwire.RequestKey{Generation: 7, ID: maxCanonicalInteger + 1}
			return referencesTransactionID("s", 7, k, "inv-1", occurrence, source, revision)
		},
		"invalid-query": func() (string, error) {
			return referencesTransactionID("s", 7, key, "inv-1", "owned-1", source, revision)
		},
		"swapped-refs": func() (string, error) {
			return referencesTransactionID("s", 7, key, "inv-1", occurrence, revision, source)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := test(); err == nil {
				t.Fatal("ASSERT_TRANSACTION_INVALID_INPUT")
			}
		})
	}
}
