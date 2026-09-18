package captureset

import (
	"encoding/json"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
)

func TestVerifyPublicationEvidenceBindsCanonicalPrivateBundle(t *testing.T) {
	native, err := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: "inv", SourceRevision: "rev", ServerVersion: "v"}}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	authority := NativeV5Authority()
	constituent, err := authority.Constituent(raw)
	if err != nil {
		t.Fatal(err)
	}
	ledger := Ledger{Denominator: 1, Entries: []LedgerEntry{{Ordinal: 0, Identity: "entry", Disposition: "processed"}}}
	manifest, err := Prepare([]Target{{CensusOrdinal: 0, CanonicalSeedV2: "seed", CanonicalSeedV2SHA256: rawDigest([]byte("seed"))}}, []Constituent{constituent}, ledger, ledger, "census", "duplicates")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = AssociateBatches(manifest, []Constituent{constituent})
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := EncodeCanonical(manifest)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := encodePrivateBundle(manifest, manifestRaw, map[string][]byte{constituent.ImmutableSelector: raw})
	if err != nil {
		t.Fatal(err)
	}
	exact := []ExactConstituent{{ImmutableSelector: constituent.ImmutableSelector, Bytes: append([]byte(nil), raw...)}}
	receipt := PublicationReceipt{Selector: CaptureSetPublicationSelector(manifest), Disclosure: "PRIVATE", ArtifactSHA256: rawDigest(bundle), ByteLength: uint64(len(bundle)), Mechanism: publication.BoundFileMechanism, NamespaceAtomic: true, ConstituentCount: len(exact), VerificationStatus: "VERIFIED"}
	verified, err := VerifyPublicationEvidence(receipt, manifest, exact)
	if err != nil {
		t.Fatal(err)
	}
	verified.Constituents[0].Bytes[0] ^= 1
	if exact[0].Bytes[0] == verified.Constituents[0].Bytes[0] {
		t.Fatal("ASSERT_CLONED_EVIDENCE")
	}
	for name, mutate := range map[string]func(*PublicationReceipt, *[]ExactConstituent){
		"digest": func(r *PublicationReceipt, _ *[]ExactConstituent) { r.ArtifactSHA256 = "sha256:bad" },
		"length": func(r *PublicationReceipt, _ *[]ExactConstituent) { r.ByteLength++ },
		"count":  func(r *PublicationReceipt, _ *[]ExactConstituent) { r.ConstituentCount++ },
		"selector": func(r *PublicationReceipt, _ *[]ExactConstituent) {
			r.Selector = "capture-sets/v1/sha256/" + "0000000000000000000000000000000000000000000000000000000000000000.bundle"
		},
		"missing": func(_ *PublicationReceipt, e *[]ExactConstituent) { *e = (*e)[:0] },
		"extra": func(_ *PublicationReceipt, e *[]ExactConstituent) {
			*e = append(*e, ExactConstituent{ImmutableSelector: "extra", Bytes: []byte("extra")})
		},
		"duplicate": func(_ *PublicationReceipt, e *[]ExactConstituent) { *e = append(*e, (*e)[0]) },
		"bytes":     func(_ *PublicationReceipt, e *[]ExactConstituent) { (*e)[0].Bytes[0] ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			r := receipt
			e := make([]ExactConstituent, len(exact))
			for i := range exact {
				e[i] = ExactConstituent{ImmutableSelector: exact[i].ImmutableSelector, Bytes: append([]byte(nil), exact[i].Bytes...)}
			}
			mutate(&r, &e)
			if _, err := VerifyPublicationEvidence(r, manifest, e); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
