// Package censusacquisition coordinates a private, unpublished census acquisition.
// It owns no session lifecycle, filesystem traversal, public operation, or publication.
package censusacquisition

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/lsp"
)

const (
	CensusPolicy    = "closed-file-document-symbol-preparation-v1"
	DuplicatePolicy = "reject-canonical-seed-v2-identity-and-census-ordinal-v1"
	IDDomain        = "lsp-trace:census-acquisition:v1"
)

type SessionIdentity struct {
	SessionID  string
	Generation uint64
}

func (s SessionIdentity) Validate() error {
	if strings.TrimSpace(s.SessionID) == "" || s.Generation == 0 {
		return errors.New("initialized session identity required")
	}
	return nil
}

type Filters struct{ Includes, Excludes []string }

// Select applies exclusions first. match implements the caller's documented pattern semantics.
func (f Filters) Select(path string, match func(pattern, path string) bool) bool {
	for _, p := range f.Excludes {
		if match(p, path) {
			return false
		}
	}
	if len(f.Includes) == 0 {
		return true
	}
	for _, p := range f.Includes {
		if match(p, path) {
			return true
		}
	}
	return false
}

type PreparedTarget struct {
	CensusOrdinal   int
	CanonicalSeedV2 []byte
	URI             string
	SelectionRange  lsp.Range
}

func (t PreparedTarget) Position() lsp.Position { return t.SelectionRange.Start }

type Discovery struct {
	Session      SessionIdentity
	Accounting   census.Accounting
	FileLedger   captureset.Ledger
	SymbolLedger captureset.Ledger
	Targets      []PreparedTarget
	Complete     bool
}
type Discoverer interface {
	Discover(context.Context, SessionIdentity) (Discovery, error)
}

type BatchRequest struct {
	Session   SessionIdentity
	CensusID  string
	BatchID   string
	Ordinal   int
	DownDepth int
	UpDepth   int
	Targets   []PreparedTarget
}

func (b BatchRequest) AcquisitionManifest(limits acquisitionops.Limits) acquisitionops.Manifest {
	targets := make([]acquisitionops.Target, len(b.Targets))
	for i, t := range b.Targets {
		line, character := t.SelectionRange.Start.Line, t.SelectionRange.Start.Character
		down, up := b.DownDepth, b.UpDepth
		id := fmt.Sprintf("target-%06d", t.CensusOrdinal)
		if i == 0 {
			id = "root"
		}
		targets[i] = acquisitionops.Target{ID: id, Locator: acquisition.Locator{URI: t.URI, Line: &line, Character: &character}, DownDepth: &down, UpDepth: &up}
	}
	return acquisitionops.Manifest{SchemaVersion: acquisitionops.ManifestVersion, CoordinateConvention: "zero-based-session", Root: targets[0], RequiredTargets: append([]acquisitionops.Target(nil), targets[1:]...), Limits: limits}
}

type AcquiredV5 struct {
	Session SessionIdentity
	Raw     []byte
}
type Acquirer interface {
	AcquireV5(context.Context, BatchRequest) (AcquiredV5, error)
}

type Constituent struct {
	BatchID  string
	Ordinal  int
	Raw      []byte
	Identity captureset.Constituent
}

// Publisher is the later integration commit point. Core.Run never accepts or
// invokes it; callers may supply a private publisher only after receiving a Plan.
type Publisher interface {
	Publish(context.Context, Plan) error
}

type Plan struct {
	Session       SessionIdentity
	CensusID      string
	Batches       []BatchRequest
	Constituents  []Constituent
	Manifest      captureset.Manifest
	ManifestBytes []byte
}
type Core struct {
	Discoverer Discoverer
	Acquirer   Acquirer
	Authority  captureset.ExactBytesAuthority
}

func (c Core) Run(ctx context.Context, session SessionIdentity) (Plan, error) {
	if err := session.Validate(); err != nil {
		return Plan{}, err
	}
	if c.Discoverer == nil || c.Acquirer == nil {
		return Plan{}, errors.New("discoverer and acquirer required")
	}
	d, err := c.Discoverer.Discover(ctx, session)
	if err != nil {
		return Plan{}, fmt.Errorf("discovery: %w", err)
	}
	if d.Session != session {
		return Plan{}, errors.New("session identity drift during discovery")
	}
	if !d.Complete {
		return Plan{}, errors.New("discovery accounting incomplete")
	}
	if err := d.Accounting.Validate(); err != nil {
		return Plan{}, fmt.Errorf("discovery accounting: %w", err)
	}
	if err := validateLedgers(d); err != nil {
		return Plan{}, err
	}
	if len(d.Targets) == 0 {
		return Plan{}, errors.New("no preparable symbols")
	}
	targets, prepared, err := canonicalTargets(d.Targets)
	if err != nil {
		return Plan{}, err
	}
	batchPlan, err := census.PlanTargets(targets)
	if err != nil {
		return Plan{}, fmt.Errorf("batch planning: %w", err)
	}
	censusID := stableID("census", joinTargetBytes(targets))
	batches := make([]BatchRequest, len(batchPlan.Batches))
	constituents := make([]Constituent, len(batches))
	metadata := make([]captureset.Constituent, len(batches))
	for i, b := range batchPlan.Batches {
		batchPrepared := make([]PreparedTarget, len(b.Targets))
		for j, t := range b.Targets {
			batchPrepared[j] = prepared[t.CensusOrdinal]
		}
		batchID := stableID("batch", []byte(fmt.Sprintf("%s\x00%d\x00%s", censusID, i, joinTargetBytes(b.Targets))))
		req := BatchRequest{Session: session, CensusID: censusID, BatchID: batchID, Ordinal: i, DownDepth: census.DefaultDownDepth, UpDepth: census.DefaultUpDepth, Targets: batchPrepared}
		got, acqErr := c.Acquirer.AcquireV5(ctx, req)
		if acqErr != nil {
			return Plan{}, fmt.Errorf("acquire batch %d: %w", i, acqErr)
		}
		if got.Session != session {
			return Plan{}, fmt.Errorf("session identity drift during batch %d", i)
		}
		if len(got.Raw) == 0 {
			return Plan{}, fmt.Errorf("batch %d returned empty V5 bytes", i)
		}
		identity, authErr := c.Authority.Constituent(got.Raw)
		if authErr != nil {
			return Plan{}, fmt.Errorf("batch %d V5 admission: %w", i, authErr)
		}
		batches[i] = req
		metadata[i] = identity
		constituents[i] = Constituent{BatchID: batchID, Ordinal: i, Raw: append([]byte(nil), got.Raw...), Identity: identity}
	}
	manifest, err := captureset.Prepare(targets, metadata, d.FileLedger, d.SymbolLedger, CensusPolicy, DuplicatePolicy)
	if err != nil {
		return Plan{}, fmt.Errorf("manifest preparation: %w", err)
	}
	raw, err := captureset.EncodeCanonical(manifest)
	if err != nil {
		return Plan{}, fmt.Errorf("manifest encoding: %w", err)
	}
	p := Plan{Session: session, CensusID: censusID, Batches: batches, Constituents: constituents, Manifest: manifest, ManifestBytes: raw}
	if err := p.Validate(c.Authority); err != nil {
		return Plan{}, fmt.Errorf("assembly accounting: %w", err)
	}
	return p, nil
}

func validateLedgers(d Discovery) error {
	if d.FileLedger.Denominator != d.Accounting.FileDenominator || d.SymbolLedger.Denominator != d.Accounting.SymbolDenominator {
		return errors.New("ledger denominators disagree with discovery accounting")
	}
	if len(d.FileLedger.Entries) != d.FileLedger.Denominator || len(d.SymbolLedger.Entries) != d.SymbolLedger.Denominator {
		return errors.New("capture-set ledgers are not closed")
	}
	return nil
}
func canonicalTargets(in []PreparedTarget) ([]captureset.Target, map[int]PreparedTarget, error) {
	out := make([]captureset.Target, len(in))
	byOrdinal := make(map[int]PreparedTarget, len(in))
	seen := map[string]bool{}
	for i, t := range in {
		if t.CensusOrdinal < 0 || len(t.CanonicalSeedV2) == 0 || t.URI == "" {
			return nil, nil, fmt.Errorf("invalid prepared target %d", i)
		}
		key := string(t.CanonicalSeedV2)
		if seen[key] {
			return nil, nil, fmt.Errorf("duplicate prepared target at index %d", i)
		}
		if _, ok := byOrdinal[t.CensusOrdinal]; ok {
			return nil, nil, fmt.Errorf("duplicate census ordinal %d", t.CensusOrdinal)
		}
		seen[key] = true
		byOrdinal[t.CensusOrdinal] = t
		out[i] = captureset.Target{CensusOrdinal: t.CensusOrdinal, CanonicalSeedV2: key, CanonicalSeedV2SHA256: rawDigest(t.CanonicalSeedV2)}
	}
	return captureset.OrderTargets(out), byOrdinal, nil
}
func joinTargetBytes(ts []captureset.Target) []byte {
	var b bytes.Buffer
	for _, t := range ts {
		fmt.Fprintf(&b, "%d:%s\x00", t.CensusOrdinal, t.CanonicalSeedV2)
	}
	return b.Bytes()
}
func stableID(kind string, b []byte) string {
	h := sha256.New()
	h.Write([]byte(IDDomain))
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write(b)
	return kind + ":" + hex.EncodeToString(h.Sum(nil))
}
func rawDigest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

func (p Plan) Validate(authority captureset.ExactBytesAuthority) error {
	if err := p.Session.Validate(); err != nil {
		return err
	}
	if p.CensusID == "" || len(p.Batches) == 0 || len(p.Batches) != len(p.Constituents) {
		return errors.New("invalid plan cardinality")
	}
	seenBatch := map[string]bool{}
	seenOrd := map[int]bool{}
	targetCount := 0
	for i, b := range p.Batches {
		if b.Session != p.Session || b.CensusID != p.CensusID || b.Ordinal != i || b.BatchID == "" || seenBatch[b.BatchID] || len(b.Targets) < 1 || len(b.Targets) > captureset.MaxBatchTargets || b.DownDepth != 1 || b.UpDepth != 0 {
			return fmt.Errorf("invalid batch %d", i)
		}
		seenBatch[b.BatchID] = true
		for _, t := range b.Targets {
			if seenOrd[t.CensusOrdinal] {
				return errors.New("target appears more than once")
			}
			seenOrd[t.CensusOrdinal] = true
			targetCount++
		}
	}
	for i, c := range p.Constituents {
		if c.Ordinal != i || c.BatchID != p.Batches[i].BatchID || len(c.Raw) == 0 {
			return fmt.Errorf("invalid constituent %d", i)
		}
		if err := authority.VerifyConstituent(c.Identity, c.Raw); err != nil {
			return err
		}
	}
	if targetCount != len(p.Manifest.Targets) {
		return errors.New("target accounting mismatch")
	}
	canonical, err := captureset.EncodeCanonical(p.Manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, p.ManifestBytes) {
		return errors.New("manifest bytes mismatch")
	}
	// Manifest constituents are canonicalized independently; require an exact set equality.
	want := append([]captureset.Constituent(nil), p.Manifest.Constituents...)
	got := make([]captureset.Constituent, len(p.Constituents))
	for i := range p.Constituents {
		got[i] = p.Constituents[i].Identity
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ImmutableSelector < got[j].ImmutableSelector })
	if len(want) != len(got) {
		return errors.New("constituent accounting mismatch")
	}
	for i := range want {
		if want[i] != got[i] {
			return errors.New("constituent identity mismatch")
		}
	}
	return nil
}
