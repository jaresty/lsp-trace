package censusprogramc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
)

type discover func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error)

func (f discover) Discover(c context.Context, s censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return f(c, s)
}

type acquire func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error)

func (f acquire) AcquireV5(c context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	return f(c, b)
}

func testProjection(t *testing.T) censusacquisition.Projection {
	t.Helper()
	session := censusacquisition.SessionIdentity{SessionID: "session", Generation: 7}
	d := censusacquisition.Discovery{Session: session, Workspace: "/w", Complete: true, Accounting: census.Accounting{FileDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}}, FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "file", Disposition: "processed"}}}}
	for i := 0; i < 64; i++ {
		name := fmt.Sprintf("T%d", i)
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: fmt.Sprintf("f%03d.go", i), Line: uint64(i + 1), Column: 1}}}}, "/w")
		if err != nil {
			t.Fatal(err)
		}
		target := censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: fmt.Sprintf("file:///w/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i), Character: 0}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("f%03d.go#%d:%d:%d:%s:%d", i, i, 0, 12, name, i)}
		d.Targets = append(d.Targets, target)
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: target.SymbolIdentity, Disposition: "prepared"})
	}
	d.Accounting.SymbolDenominator = 64
	d.SymbolLedger.Denominator = 64
	p, err := (censusacquisition.Core{Discoverer: discover(func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
		return d, nil
	}), Acquirer: acquire(func(_ context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		n := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Provenance: graph.InvocationProvenance{InvocationID: fmt.Sprintf("inv-%d", b.Ordinal), SourceRevision: "rev", ServerVersion: "v"}}, Nodes: []graph.Node{graph.NewNode(graph.Item{Name: "node", Kind: 12, URI: "file:///w/a.go", Range: graph.Range{End: graph.Position{Character: 4}}, SelectionRange: graph.Range{End: graph.Position{Character: 4}}})}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
		native, _ := json.Marshal(n)
		raw, e := graphprovenance.CaptureV5WithSeedSpec(native, b.Session.SessionID, b.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, b.CanonicalSeedsV2)
		return censusacquisition.AcquiredV5{Session: b.Session, Raw: raw}, e
	})}).Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func evidence(t *testing.T, p censusacquisition.Projection) VerifiedPublication {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	exact := make([][]byte, len(p.Constituents))
	for i, c := range p.Constituents {
		exact[i] = append([]byte(nil), c.Raw...)
	}
	published := captureset.NewPublisher(root).PublishCaptureSet(p.Manifest, exact, captureset.NativeV5Authority())
	if published.Err != nil || published.Receipt == nil || published.Receipt.VerificationStatus != "VERIFIED" {
		t.Fatalf("publication=%+v", published)
	}
	resolved := make([]ResolvedConstituent, len(p.Manifest.Constituents))
	for i, c := range p.Manifest.Constituents {
		raw, resolveErr := captureset.NewPublisher(root).ResolveConstituent(published.Receipt.Selector, c.ImmutableSelector, captureset.NativeV5Authority())
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		resolved[i] = ResolvedConstituent{ImmutableSelector: c.ImmutableSelector, Bytes: raw}
	}
	return VerifiedPublication{Receipt: *published.Receipt, Manifest: p.Manifest, Resolved: resolved}
}
func metadata() programccompose.ExactMetadata {
	return programccompose.ExactMetadata{WorkspaceIdentity: "workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp", PrivacyPolicy: "private"}
}

func TestComposeVerifiedProjectionDeterministicAndEmpty(t *testing.T) {
	p := testProjection(t)
	r := Request{Projection: p, Publication: evidence(t, p), Metadata: metadata(), Seed: 9}
	one, err := Compose(r)
	if err != nil {
		t.Fatal(err)
	}
	two, err := Compose(r)
	if err != nil {
		t.Fatal(err)
	}
	if one.Outcome.Outcome != "EMPTY" || one.Outcome.LogicalDigest != two.Outcome.LogicalDigest || len(one.Candidates) != 1 {
		t.Fatalf("outcome=%+v", one.Outcome)
	}
	if one.CensusID != p.CensusID || one.Publication.Selector != r.Publication.Receipt.Selector || len(one.Composite.Bytes) == 0 || len(one.Admission.Bytes) == 0 {
		t.Fatal("missing retained bindings")
	}
}
func TestPublicationAndReconciliationFailures(t *testing.T) {
	p := testProjection(t)
	cases := map[string]func(*Request){"aggregate": func(x *Request) { x.Publication.Receipt.ArtifactSHA256 = "" }, "missing": func(x *Request) { x.Publication.Resolved = x.Publication.Resolved[:1] }, "reordered": func(x *Request) {
		x.Publication.Resolved[0], x.Publication.Resolved[1] = x.Publication.Resolved[1], x.Publication.Resolved[0]
	}, "raw": func(x *Request) { x.Publication.Resolved[0].Bytes[0] ^= 1 }, "metadata": func(x *Request) { x.Metadata.PrivacyPolicy = "" }}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			x := Request{Projection: p, Publication: evidence(t, p), Metadata: metadata()}
			mut(&x)
			_, err := Compose(x)
			if err == nil {
				t.Fatal("expected failure")
			}
			var f *Failure
			if !asFailure(err, &f) {
				t.Fatal(err)
			}
			if (name == "aggregate" || name == "missing" || name == "raw") && f.Stage != StagePublication {
				t.Fatal(f)
			}
			if name == "reordered" && f.Stage != StageReconciliation {
				t.Fatal(f)
			}
			if name == "metadata" && f.Stage != StageComposition {
				t.Fatal(f)
			}
		})
	}
}
func TestCandidateStatusAndAliasing(t *testing.T) {
	p := testProjection(t)
	r := Request{Projection: p, Publication: evidence(t, p), Metadata: metadata()}
	r.Publication.Resolved[0].Bytes[0] ^= 1
	if _, err := Compose(r); err == nil {
		t.Fatal("mutated input must reject")
	}
	r.Publication = evidence(t, p)
	got, err := Compose(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome.ClaimCeiling == "" {
		t.Fatal("claim ceiling")
	}
	got.Composite.Bytes[0] ^= 1
	got.Admission.Bytes[0] ^= 1
	if len(got.Candidates) != 1 || got.Candidates[0].Status != CandidateStatus || got.Candidates[0].ClaimCeiling != got.Outcome.ClaimCeiling {
		t.Fatalf("candidate=%+v outcome=%+v", got.Candidates, got.Outcome)
	}
}
func asFailure(err error, target **Failure) bool {
	for err != nil {
		if f, ok := err.(*Failure); ok {
			*target = f
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
