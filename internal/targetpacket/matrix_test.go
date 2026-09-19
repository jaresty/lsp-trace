package targetpacket

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

type errLookup struct {
	err  error
	body []byte
	id   sourceobject.Identity
}

func (l errLookup) Get(sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: l.id, Bytes: l.body}, l.err
}
func requireFailure(t *testing.T, err error, stage Stage) *Failure {
	t.Helper()
	var f *Failure
	if !errors.As(err, &f) || f.Stage != stage {
		t.Fatalf("failure stage: %T %[1]v", err)
	}
	return f
}

type censusAdmissionFingerprint struct {
	Valid          bool   `json:"valid"`
	NodeIdentities any    `json:"node_identities"`
	Occurrences    any    `json:"occurrences"`
	ClaimCeiling   string `json:"claim_ceiling"`
	SourceBinding  any    `json:"source_binding"`
}

type censusFingerprint struct {
	Exported  json.RawMessage            `json:"exported"`
	Admission censusAdmissionFingerprint `json:"admission"`
}

type pairWeightFingerprint struct {
	From, To int64
	Weight   float64
}

type outcomeFingerprint struct {
	Outcome, ProfileID, ProfileDigest, Algorithm, LogicalDigest, ClaimCeiling         string
	Resolution                                                                        float64
	Seed                                                                              uint64
	Communities                                                                       any
	NodeIdentities, NodeIDs, Occurrences, ProjectionSource, ProjectionCompositeSource any
	Source, CompositeSource                                                           any
}

type exportedCensusFingerprint struct {
	CensusID        string
	Publication     any
	Composite       any
	Admission       any
	Outcome         any
	PairWeights     []pairWeightFingerprint
	Candidates      any
	Representatives any
}

func fingerprintJSON(t *testing.T, name string, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return encoded
}

func fingerprintCensus(t *testing.T, c censusprogramc.Result) []byte {
	t.Helper()
	outcome := c.Outcome
	pairs := make([]pairWeightFingerprint, 0, len(outcome.Projection.PairWeights))
	for pair, weight := range outcome.Projection.PairWeights {
		pairs = append(pairs, pairWeightFingerprint{From: pair.From, To: pair.To, Weight: weight})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].From != pairs[j].From {
			return pairs[i].From < pairs[j].From
		}
		return pairs[i].To < pairs[j].To
	})
	publication := fingerprintJSON(t, "publication", c.Publication)
	composite := fingerprintJSON(t, "composite", c.Composite)
	admission := fingerprintJSON(t, "admission", c.Admission)
	canonicalOutcome := fingerprintJSON(t, "outcome", outcomeFingerprint{
		Outcome: outcome.Outcome, ProfileID: outcome.ProfileID, ProfileDigest: outcome.ProfileDigest, Algorithm: outcome.Algorithm, LogicalDigest: outcome.LogicalDigest, ClaimCeiling: outcome.ClaimCeiling, Resolution: outcome.Resolution, Seed: outcome.Seed, Communities: outcome.Communities,
		NodeIdentities: outcome.Projection.NodeIdentities, NodeIDs: outcome.Projection.NodeIDs, Occurrences: outcome.Projection.Occurrences, ProjectionSource: outcome.Projection.Source, ProjectionCompositeSource: outcome.Projection.CompositeSource, Source: outcome.Source, CompositeSource: outcome.CompositeSource,
	})
	candidates := fingerprintJSON(t, "candidates", c.Candidates)
	representatives := fingerprintJSON(t, "representatives", c.Representatives)
	exported := fingerprintJSON(t, "exported census", exportedCensusFingerprint{CensusID: c.CensusID, Publication: publication, Composite: composite, Admission: admission, Outcome: canonicalOutcome, PairWeights: pairs, Candidates: candidates, Representatives: representatives})
	a := c.Admission.Admission
	return fingerprintJSON(t, "census fingerprint", censusFingerprint{Exported: exported, Admission: censusAdmissionFingerprint{
		Valid: a.Valid(), NodeIdentities: a.NodeIdentities(), Occurrences: a.Occurrences(), ClaimCeiling: a.ClaimCeiling(), SourceBinding: a.SourceBinding(),
	}})
}

func TestRepresentativeBuildInvariants(t *testing.T) {
	cases := map[string]func(*censusprogramc.Representative){
		"status":            func(n *censusprogramc.Representative) { n.Status = "BAD" },
		"selection":         func(n *censusprogramc.Representative) { n.SelectionState = "BAD" },
		"authority":         func(n *censusprogramc.Representative) { n.Authority = 1 },
		"completeness":      func(n *censusprogramc.Representative) { n.SourceGraphComplete = "FULL" },
		"claim ceiling":     func(n *censusprogramc.Representative) { n.ClaimCeiling = "BAD" },
		"batch":             func(n *censusprogramc.Representative) { n.BatchID = "" },
		"community":         func(n *censusprogramc.Representative) { n.CommunityIdentity = "" },
		"bundle":            func(n *censusprogramc.Representative) { n.ExecutionBundleID = "" },
		"seed label":        func(n *censusprogramc.Representative) { n.SeedLabel = "" },
		"seed at":           func(n *censusprogramc.Representative) { n.SeedAt = "" },
		"node":              func(n *censusprogramc.Representative) { n.SelectedNode = "" },
		"members":           func(n *censusprogramc.Representative) { n.Members = nil },
		"members duplicate": func(n *censusprogramc.Representative) { n.Members = append(n.Members, n.Members[0]) },
		"members unordered": func(n *censusprogramc.Representative) { n.Members = []string{"z", "a"} },
		"scc members":       func(n *censusprogramc.Representative) { n.SCCMembers = nil },
		"scc duplicate":     func(n *censusprogramc.Representative) { n.SCCMembers = append(n.SCCMembers, n.SCCMembers[0]) },
		"scc unordered":     func(n *censusprogramc.Representative) { n.SCCMembers = []string{"z", "a"} },
		"distance":          func(n *censusprogramc.Representative) { n.Distance = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest(t)
			if name == "claim ceiling" {
				r.Census.Outcome.ClaimCeiling = r.Census.Representatives.Nominations[0].ClaimCeiling
			}
			mutate(&r.Census.Representatives.Nominations[0])
			requireFailure(t, mustBuild(r), StageSelect)
		})
	}
}

func TestSnapshotDuplicateOrdinal(t *testing.T) {
	r := validRequest(t)
	r.Snapshots = append(r.Snapshots, r.Snapshots[0])
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotIdentityMismatch(t *testing.T) {
	r := validRequest(t)
	r.Snapshots[0].ConstituentIdentity = "foreign"
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotOrdinalOutside(t *testing.T) {
	r := validRequest(t)
	r.Snapshots[0].ConstituentOrdinal = 1
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotCensusGraphDigestMismatch(t *testing.T) {
	r := validRequest(t)
	r.Census.Admission.Artifact.Constituents[0].GraphSHA256 = "sha256:bad"
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotCensusGraphLengthMismatch(t *testing.T) {
	r := validRequest(t)
	r.Census.Admission.Artifact.Constituents[0].GraphByteLength++
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotUnusedValidExtra(t *testing.T) {
	r := validRequest(t)
	x := retainedprojectiontestfixture.ValidV2Artifact(t)
	r.Snapshots = append(r.Snapshots, Snapshot{ConstituentIdentity: "extra", ConstituentOrdinal: 0, Raw: x.Raw})
	requireFailure(t, mustBuild(r), StageReconcile)
}
func TestSnapshotTwoNominationsReuseOneSuccess(t *testing.T) {
	r := validRequest(t)
	n := r.Census.Representatives.Nominations[0]
	n.CommunityIdentity = "other"
	n.SeedLabel = "other"
	r.Census.Representatives.Nominations = append(r.Census.Representatives.Nominations, n)
	out, e := Build(r)
	if e != nil || len(out.Packets) != 2 {
		t.Fatalf("reuse: %v %#v", e, out)
	}
}

func TestMappingZeroDisplayBinding(t *testing.T) {
	r := validRequest(t)
	fx := retainedprojectiontestfixture.VariantV2Artifact(t, func(a *v5sourcesnapshotv2.Artifact) { a.DisplayBindings = nil })
	r.Snapshots[0].Raw = fx.Raw
	requireFailure(t, mustBuild(r), StageAdmit)
}
func TestMappingDuplicateIdenticalDisplayRow(t *testing.T) {
	r := validRequest(t)
	fx := retainedprojectiontestfixture.VariantV2Artifact(t, func(a *v5sourcesnapshotv2.Artifact) {
		a.DisplayBindings = append(a.DisplayBindings, a.DisplayBindings[0])
	})
	r.Snapshots[0].Raw = fx.Raw
	requireFailure(t, mustBuild(r), StageAdmit)
}
func TestMappingSameNodeDistinctLogicalURIs(t *testing.T) {
	r := validRequest(t)
	fx := retainedprojectiontestfixture.VariantV2Artifact(t, func(a *v5sourcesnapshotv2.Artifact) {
		x := a.DisplayBindings[0]
		x.LogicalSourceID += "?second"
		a.DisplayBindings = append(a.DisplayBindings, x)
	})
	r.Snapshots[0].Raw = fx.Raw
	requireFailure(t, mustBuild(r), StageAdmit)
}

func TestLookupNil(t *testing.T) {
	r := validRequest(t)
	r.Lookup = nil
	requireFailure(t, mustBuild(r), StageInput)
}
func TestLookupErrorSafe(t *testing.T) {
	r := validRequest(t)
	r.Lookup = errLookup{err: errors.New("RAW_SENTINEL /private/provider")}
	f := requireFailure(t, mustBuild(r), StageResolve)
	if strings.Contains(f.Error(), "RAW_SENTINEL") || strings.Contains(f.Err.Error(), "RAW_SENTINEL") {
		t.Fatal("unsafe lookup failure")
	}
}
func TestLookupIdentityMismatch(t *testing.T) {
	r := validRequest(t)
	r.Lookup = errLookup{id: sourceobject.Identity{Digest: "sha256:wrong", ByteLength: 1}, body: []byte("RAW_SENTINEL")}
	f := requireFailure(t, mustBuild(r), StageResolve)
	if !retainedprojection.IsCode(f.Err, retainedprojection.CodeReturnedIdentityMismatch) {
		t.Fatal(f.Err)
	}
}
func TestLookupByteLengthMismatch(t *testing.T) {
	r := validRequest(t)
	r.Lookup = errLookup{body: []byte("x")}
	requireFailure(t, mustBuild(r), StageResolve)
}
func TestLookupDigestMismatch(t *testing.T) {
	r := validRequest(t)
	r.Lookup = errLookup{body: []byte("evil")}
	requireFailure(t, mustBuild(r), StageResolve)
}

func TestResolveLimitFields(t *testing.T) {
	for name, mut := range map[string]func(*retainedprojection.ResolveLimits){"objects": func(x *retainedprojection.ResolveLimits) { x.MaxDistinctObjects = 0 }, "bytes": func(x *retainedprojection.ResolveLimits) { x.MaxUniqueSourceBytes = 0 }, "selections": func(x *retainedprojection.ResolveLimits) { x.MaxLogicalSelections = 0 }} {
		t.Run(name, func(t *testing.T) {
			r := validRequest(t)
			mut(&r.ResolveLimits)
			requireFailure(t, mustBuild(r), StageResolve)
		})
	}
}
func TestPolicyLimits(t *testing.T) {
	for name, mut := range map[string]func(*sourceprojection.Policy){"bytes": func(x *sourceprojection.Policy) { x.MaxBytes = 0 }, "ranges": func(x *sourceprojection.Policy) { x.MaxRanges = 0 }, "objects": func(x *sourceprojection.Policy) { x.MaxObjects = 0 }, "work": func(x *sourceprojection.Policy) { x.MaxWork = 0 }} {
		t.Run(name, func(t *testing.T) {
			r := validRequest(t)
			mut(&r.Policy)
			requireFailure(t, mustBuild(r), StageProject)
		})
	}
}
func TestMaxResponseBytesTooSmall(t *testing.T) {
	r := validRequest(t)
	r.MaxResponseBytes = 1
	requireFailure(t, mustBuild(r), StageAssemble)
}

func TestDeterminismAndAliases(t *testing.T) {
	r := validRequest(t)
	n := r.Census.Representatives.Nominations[0]
	n.CommunityIdentity = "b"
	r.Census.Representatives.Nominations = append(r.Census.Representatives.Nominations, n)
	a, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	r.Census.Representatives.Nominations[0], r.Census.Representatives.Nominations[1] = r.Census.Representatives.Nominations[1], r.Census.Representatives.Nominations[0]
	b, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	for i := range a.Packets {
		x, _ := EncodeCanonical(a.Packets[i])
		y, _ := EncodeCanonical(b.Packets[i])
		if !bytes.Equal(x, y) {
			t.Fatal("shuffle nondeterministic")
		}
	}
}
func TestUnresolvedDeterminismUsesFullLineage(t *testing.T) {
	r := validRequest(t)
	a := r.Census.Representatives.Nominations[0]
	a.SelectionState = "UNRESOLVED"
	b := a
	a.CommunityIdentity, b.CommunityIdentity = "a", "b"
	a.SeedLabel, b.SeedLabel = "a", "b"
	r.Census.Representatives.Nominations = nil
	r.Census.Representatives.Unresolved = []censusprogramc.Representative{b, a}
	first, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	r.Census.Representatives.Unresolved[0], r.Census.Representatives.Unresolved[1] = r.Census.Representatives.Unresolved[1], r.Census.Representatives.Unresolved[0]
	second, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(first)
	y, _ := json.Marshal(second)
	if !bytes.Equal(x, y) {
		t.Fatal("unresolved shuffle nondeterministic")
	}
}

func TestStatesAndLineageClone(t *testing.T) {
	r := validRequest(t)
	u := r.Census.Representatives.Nominations[0]
	u.SelectionState = "UNRESOLVED"
	r.Census.Representatives.Unresolved = []censusprogramc.Representative{u}
	out, e := Build(r)
	if e != nil || out.State != StatePrepared || out.UnresolvedCount != 1 || !reflect.DeepEqual(out.Unresolved[0], u) {
		t.Fatal("mixed state lineage")
	}
	_ = StateEmpty
	_ = StateUnresolved
	_ = StateFailed
}
func TestCensusDeepImmutability(t *testing.T) {
	r := validRequest(t)
	before := fingerprintCensus(t, r.Census)
	if again := fingerprintCensus(t, r.Census); !bytes.Equal(before, again) {
		t.Fatal("unstable census fingerprint")
	}
	if _, err := Build(r); err != nil {
		t.Fatal(err)
	}
	if after := fingerprintCensus(t, r.Census); !bytes.Equal(before, after) {
		t.Fatal("census mutated on success")
	}
	r.Lookup = nil
	_, _ = Build(r)
	if after := fingerprintCensus(t, r.Census); !bytes.Equal(before, after) {
		t.Fatal("census mutated on failure")
	}
}

func TestValidatorTamperMatrix(t *testing.T) {
	r := validRequest(t)
	o, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	p := o.Packets[0]
	cases := map[string]func(*Packet){"schema": func(x *Packet) { x.SchemaVersion = "bad" }, "status": func(x *Packet) { x.Status = "bad" }, "authority": func(x *Packet) { x.Authority = 1 }, "accepted": func(x *Packet) { x.Accepted = true }, "completeness": func(x *Packet) { x.Completeness = "FULL" }, "claim ceiling": func(x *Packet) { x.ClaimCeiling = "bad" }, "census lineage": func(x *Packet) { x.Lineage.CensusID = "bad" }, "privacy": func(x *Packet) { x.PrivacyPolicyID = "bad" }, "custody": func(x *Packet) { x.Custody.Custody = "bad" }, "projection custody": func(x *Packet) { x.Projection.CustodyBinding.Custody = "bad" }, "node": func(x *Packet) { x.Lineage.SelectedNode = "" }, "logical": func(x *Packet) { x.Lineage.SelectedLogicalSourceID = "" }}
	for n, m := range cases {
		t.Run(n, func(t *testing.T) {
			q := p
			m(&q)
			q.PacketID, _ = packetIdentity(q)
			raw, _ := EncodeCanonical(q)
			if _, e := Validate(raw); e == nil {
				t.Fatal(n)
			}
		})
	}
}
func TestValidatorClosedPacketConsistency(t *testing.T) {
	r := validRequest(t)
	o, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	base := o.Packets[0]
	cases := map[string]func(*Packet){
		"lineage status":            func(p *Packet) { p.Lineage.Status = "BAD" },
		"lineage selection":         func(p *Packet) { p.Lineage.SelectionState = "BAD" },
		"lineage completeness":      func(p *Packet) { p.Lineage.SourceGraphComplete = "FULL" },
		"lineage required id":       func(p *Packet) { p.Lineage.BatchID = "" },
		"lineage negative distance": func(p *Packet) { p.Lineage.Distance = -1 },
		"lineage members empty":     func(p *Packet) { p.Lineage.Members = nil },
		"lineage members unordered": func(p *Packet) { p.Lineage.Members = []string{"z", "a"} },
		"lineage members duplicate": func(p *Packet) { p.Lineage.Members = []string{"member", "member"} },
		"lineage scc empty":         func(p *Packet) { p.Lineage.SCCMembers = nil },
		"lineage scc unordered":     func(p *Packet) { p.Lineage.SCCMembers = []string{"z", "a"} },
		"lineage scc duplicate":     func(p *Packet) { p.Lineage.SCCMembers = []string{"member", "member"} },
		"projection schema":         func(p *Packet) { p.Projection.SchemaVersion = "BAD" },
		"projection status":         func(p *Packet) { p.Projection.Status = "BAD" },
		"projection authority":      func(p *Packet) { p.Projection.Authority = 1 },
		"projection completeness":   func(p *Packet) { p.Projection.SourceGraphComplete = "FULL" },
		"projection custody mode":   func(p *Packet) { p.Projection.CustodyMode = "BAD" },
		"custody graph schema":      func(p *Packet) { p.Custody.GraphSchemaID = ""; p.Projection.CustodyBinding.GraphSchemaID = "" },
		"custody capture":           func(p *Packet) { p.Custody.CaptureID = ""; p.Projection.CustodyBinding.CaptureID = "" },
		"custody manifest":          func(p *Packet) { p.Custody.ManifestID = ""; p.Projection.CustodyBinding.ManifestID = "" },
		"custody resolver":          func(p *Packet) { p.Custody.ResolverKind = ""; p.Projection.CustodyBinding.ResolverKind = "" },
		"custody digest": func(p *Packet) {
			p.Custody.GraphDigest = "sha256:bad"
			p.Projection.CustodyBinding.GraphDigest = "sha256:bad"
		},
		"custody length":          func(p *Packet) { p.Custody.GraphByteLength = 0; p.Projection.CustodyBinding.GraphByteLength = 0 },
		"custody packet mismatch": func(p *Packet) { p.Custody.CaptureID = "different" },
		"privacy empty":           func(p *Packet) { p.PrivacyPolicyID = "" },
		"document target":         func(p *Packet) { p.Projection.DocumentSelection.TargetURI = "bad" },
		"document first selected": func(p *Packet) { p.Projection.DocumentSelection.SelectedURIs[0] = "bad" },
		"target unit":             func(p *Packet) { p.Projection.Units[0].Role = "ADDITIONAL" },
		"unit source digest":      func(p *Packet) { p.Projection.Units[0].SourceDigest = "sha256:bad" },
		"unit source length":      func(p *Packet) { p.Projection.Units[0].SourceByteLength = 0 },
		"unit body disposition":   func(p *Packet) { p.Projection.Units[0].BodyDisposition = "BAD" },
		"citation":                func(p *Packet) { p.Projection.Citations = nil },
		"span":                    func(p *Packet) { p.Projection.EmittedSpans = nil },
		"coordinated unit span digest": func(p *Packet) {
			p.Projection.Units[0].SourceDigest, p.Projection.EmittedSpans[0].SourceDigest = "sha256:bad", "sha256:bad"
		},
		"coordinated unit binding length": func(p *Packet) {
			p.Projection.Units[0].SourceByteLength++
			p.Projection.DocumentBindings[0].SourceByteLength++
		},
		"citation role":  func(p *Packet) { p.Projection.Citations[0].Role = "BAD" },
		"citation range": func(p *Packet) { p.Projection.Citations[0].EvidenceRange.End.Character++ },
		"span range":     func(p *Packet) { p.Projection.EmittedSpans[0].Range.End.Character++ },
		"span duplicate unit": func(p *Packet) {
			p.Projection.EmittedSpans[0].UnitIDs = append(p.Projection.EmittedSpans[0].UnitIDs, p.Projection.Units[0].UnitID)
		},
		"span body length":       func(p *Packet) { p.Projection.EmittedSpans[0].ByteLength++ },
		"packet lineage ceiling": func(p *Packet) { p.ClaimCeiling = "bad" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := base
			p.Lineage.Members = append([]string(nil), base.Lineage.Members...)
			p.Lineage.SCCMembers = append([]string(nil), base.Lineage.SCCMembers...)
			p.Projection.Units = append([]sourceprojectionv2.Unit(nil), base.Projection.Units...)
			p.Projection.Citations = append([]sourceprojectionv2.Citation(nil), base.Projection.Citations...)
			p.Projection.DocumentBindings = append([]sourceprojectionv2.DocumentBinding(nil), base.Projection.DocumentBindings...)
			p.Projection.EmittedSpans = append([]sourceprojection.Span(nil), base.Projection.EmittedSpans...)
			p.Projection.EmittedSpans[0].UnitIDs = append([]string(nil), base.Projection.EmittedSpans[0].UnitIDs...)
			mutate(&p)
			p.PacketID, _ = packetIdentity(p)
			raw, _ := EncodeCanonical(p)
			if _, err := Validate(raw); err == nil {
				t.Fatal("accepted inconsistent packet")
			}
		})
	}
}

func TestValidatorSyntaxAndReplay(t *testing.T) {
	r := validRequest(t)
	o, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := EncodeCanonical(o.Packets[0])
	if _, e = Validate(raw); e != nil {
		t.Fatal(e)
	}
	for _, x := range [][]byte{append(raw, []byte(" {}")...), append([]byte(`{"unknown":1,`), raw[1:]...), []byte(`{"packet_id":"x","packet_id":"y"}`)} {
		if _, e = Validate(x); e == nil {
			t.Fatal("syntax accepted")
		}
	}
}
func TestSuccessSchemaConcrete(t *testing.T) {
	r := validRequest(t)
	o, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	p := o.Packets[0]
	if p.Authority != 0 || p.Accepted || p.Completeness != "UNKNOWN" || p.Custody != p.Projection.CustodyBinding || len(p.Projection.Units) == 0 || len(p.Projection.Citations) == 0 || len(p.Projection.EmittedSpans) == 0 {
		t.Fatalf("schema incomplete %#v", p)
	}
}
func TestFailureErrorHygiene(t *testing.T) {
	r := validRequest(t)
	r.Lookup = errLookup{err: errors.New("BODY_SENTINEL /path provider")}
	f := requireFailure(t, mustBuild(r), StageResolve)
	if f.Error() != "targetpacket RESOLVE/RESOLVE_FAILED" || strings.Contains(f.Err.Error(), "SENTINEL") {
		t.Fatal("unsafe failure")
	}
}
func mustBuild(r Request) error { _, e := Build(r); return e }
