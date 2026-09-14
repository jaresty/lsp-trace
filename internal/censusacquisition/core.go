// Package censusacquisition coordinates a private, unpublished census acquisition.
// It owns no session lifecycle, filesystem traversal, public operation, or production publication.
package censusacquisition

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
)

var ErrDiscoveryIncomplete = errors.New("discovery accounting incomplete")

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

func (f Filters) Select(path string, match func(string, string) bool) bool {
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
	Name            string
	Kind            int
	Range           lsp.Range
	SymbolIdentity  string
}

func (t PreparedTarget) Position() lsp.Position { return t.SelectionRange.Start }

type Discovery struct {
	Session                  SessionIdentity
	Workspace                string
	Accounting               census.Accounting
	FileLedger, SymbolLedger captureset.Ledger
	Targets                  []PreparedTarget
	Complete                 bool
}
type Discoverer interface {
	Discover(context.Context, SessionIdentity) (Discovery, error)
}
type BatchRequest struct {
	Session                     SessionIdentity
	CensusID, BatchID           string
	Ordinal, DownDepth, UpDepth int
	Targets                     []PreparedTarget
	CanonicalSeedsV2            []byte
}

func (b BatchRequest) AcquisitionManifest(limits acquisitionops.Limits) acquisitionops.Manifest {
	ts := make([]acquisitionops.Target, len(b.Targets))
	for i, t := range b.Targets {
		line, ch := t.Position().Line, t.Position().Character
		down, up := b.DownDepth, b.UpDepth
		id := batchTargetID(i, t)
		ts[i] = acquisitionops.Target{ID: id, Locator: acquisition.Locator{URI: t.URI, Line: &line, Character: &ch}, DownDepth: &down, UpDepth: &up}
	}
	return acquisitionops.Manifest{SchemaVersion: acquisitionops.ManifestVersion, CoordinateConvention: "zero-based-session", Root: ts[0], RequiredTargets: append([]acquisitionops.Target(nil), ts[1:]...), Limits: limits}
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

type Projection struct {
	Session       SessionIdentity
	CensusID      string
	Batches       []BatchRequest
	Constituents  []Constituent
	Manifest      captureset.Manifest
	ManifestBytes []byte
	Workspace     string
}

// PlanningConfig is authority-neutral input fixed before batch identities are
// minted. A nil configuration preserves the legacy depth defaults.
type PlanningConfig struct {
	DownDepth int
	UpDepth   int
}

type Core struct {
	Discoverer Discoverer
	Acquirer   Acquirer
	Planning   *PlanningConfig
}

type batchAcquisitionFailure struct {
	ordinal int
	err     error
}

func (f *batchAcquisitionFailure) Error() string     { return f.err.Error() }
func (f *batchAcquisitionFailure) Unwrap() error     { return f.err }
func (f *batchAcquisitionFailure) BatchOrdinal() int { return f.ordinal }

func failBatch(ordinal int, err error) error {
	return &batchAcquisitionFailure{ordinal: ordinal + 1, err: err}
}

// Run performs authority-neutral discovery, batch acquisition, and exact
// validation. Its result is data, not a completion or publication capability.
func (c Core) Run(ctx context.Context, s SessionIdentity) (Projection, error) {
	if err := s.Validate(); err != nil {
		return Projection{}, err
	}
	planning, err := resolvePlanning(c.Planning)
	if err != nil {
		return Projection{}, err
	}
	if c.Discoverer == nil || c.Acquirer == nil {
		return Projection{}, errors.New("discoverer and acquirer required")
	}
	d, err := c.Discoverer.Discover(ctx, s)
	if err != nil {
		return Projection{}, fmt.Errorf("discovery: %w", err)
	}
	if d.Session != s {
		return Projection{}, errors.New("session identity drift during discovery")
	}
	if !d.Complete {
		return Projection{}, ErrDiscoveryIncomplete
	}
	if err = validateDiscovery(d); err != nil {
		return Projection{}, err
	}
	if err = d.Accounting.Validate(); err != nil {
		return Projection{}, fmt.Errorf("discovery accounting: %w", err)
	}
	targets, prepared, err := canonicalTargets(d)
	if err != nil {
		return Projection{}, err
	}
	plan, err := census.PlanTargets(targets)
	if err != nil {
		return Projection{}, err
	}
	cid := stableID("census", planningIdentity(joinTargetBytes(targets), planning))
	batches := make([]BatchRequest, len(plan.Batches))
	for i, b := range plan.Batches {
		pt := make([]PreparedTarget, len(b.Targets))
		for j, t := range b.Targets {
			pt[j] = cloneTarget(prepared[t.CensusOrdinal])
		}
		seedBytes, err := combineSeedsForPlanning(pt, d.Workspace, planning)
		if err != nil {
			return Projection{}, failBatch(i, fmt.Errorf("batch %d seeds: %w", i, err))
		}
		batchIdentity := []byte(fmt.Sprintf("%s\x00%d\x00%s", cid, i, joinTargetBytes(b.Targets)))
		bid := stableID("batch", planningIdentity(batchIdentity, planning))
		batches[i] = BatchRequest{s, cid, bid, i, planning.DownDepth, planning.UpDepth, pt, seedBytes}
		if err := ValidateBatchSeeds(batches[i], d.Workspace); err != nil {
			return Projection{}, failBatch(i, fmt.Errorf("batch %d planning: %w", i, err))
		}
	}
	cs := make([]Constituent, len(batches))
	meta := make([]captureset.Constituent, len(batches))
	for i := range batches {
		req := batches[i]
		got, e := c.Acquirer.AcquireV5(ctx, cloneBatchRequest(req))
		if e != nil {
			return Projection{}, failBatch(i, fmt.Errorf("acquire batch %d: %w", i, e))
		}
		if got.Session != s {
			return Projection{}, failBatch(i, fmt.Errorf("session identity drift during batch %d", i))
		}
		identity, e := admit(got.Raw, req)
		if e != nil {
			return Projection{}, failBatch(i, fmt.Errorf("batch %d V5 admission: %w", i, e))
		}
		meta[i] = identity
		cs[i] = Constituent{req.BatchID, i, append([]byte(nil), got.Raw...), identity}
	}
	m, err := captureset.Prepare(targets, meta, d.FileLedger, d.SymbolLedger, CensusPolicy, DuplicatePolicy)
	if err != nil {
		return Projection{}, fmt.Errorf("manifest preparation: %w", err)
	}
	m, err = captureset.AssociateBatches(m, meta)
	if err != nil {
		return Projection{}, fmt.Errorf("manifest association: %w", err)
	}
	raw, err := captureset.EncodeCanonical(m)
	if err != nil {
		return Projection{}, fmt.Errorf("manifest encoding: %w", err)
	}
	p := Projection{Session: s, CensusID: cid, Batches: batches, Constituents: cs, Manifest: m, ManifestBytes: raw, Workspace: d.Workspace}
	if err = validateProjection(p); err != nil {
		return Projection{}, fmt.Errorf("projection accounting: %w", err)
	}
	return cloneProjection(p), nil
}

func validateDiscovery(d Discovery) error {
	if d.Accounting.FileDenominator < 0 || d.Accounting.FileDenominator > captureset.MaxResources || d.Accounting.SymbolDenominator < 0 || d.Accounting.SymbolDenominator > captureset.MaxTargets || len(d.Targets) > captureset.MaxTargets {
		return errors.New("discovery bounds exceeded")
	}
	if d.FileLedger.Denominator != d.Accounting.FileDenominator || d.SymbolLedger.Denominator != d.Accounting.SymbolDenominator || len(d.FileLedger.Entries) != d.FileLedger.Denominator || len(d.SymbolLedger.Entries) != d.SymbolLedger.Denominator {
		return errors.New("ledger accounting mismatch")
	}
	acc := map[int]census.SymbolDisposition{}
	for _, e := range d.Accounting.Symbols {
		acc[e.Ordinal] = e.Disposition
	}
	led := map[int]string{}
	identities := map[string]struct{}{}
	for _, e := range d.SymbolLedger.Entries {
		if e.Ordinal < 0 || e.Ordinal >= d.SymbolLedger.Denominator {
			return errors.New("symbol ledger ordinal out of bounds")
		}
		if _, exists := led[e.Ordinal]; exists {
			return errors.New("duplicate symbol ledger ordinal")
		}
		if e.Identity == "" || strings.TrimSpace(e.Identity) != e.Identity {
			return errors.New("malformed symbol ledger identity")
		}
		if _, exists := identities[e.Identity]; exists {
			return errors.New("duplicate symbol ledger identity")
		}
		wantDisposition := string(acc[e.Ordinal])
		if acc[e.Ordinal] == census.SymbolSelected {
			wantDisposition = SymbolPrepared
		}
		if e.Disposition != wantDisposition {
			return errors.New("malformed symbol ledger disposition")
		}
		led[e.Ordinal] = e.Disposition
		identities[e.Identity] = struct{}{}
	}
	eligible := 0
	for i := 0; i < d.Accounting.SymbolDenominator; i++ {
		want := acc[i] == census.SymbolSelected
		if want != (led[i] == SymbolPrepared) {
			return errors.New("symbol disposition ledgers disagree")
		}
		if want {
			eligible++
		}
	}
	if eligible != len(d.Targets) {
		return errors.New("selected prepared symbol target mismatch")
	}
	return nil
}
func canonicalTargets(d Discovery) ([]captureset.Target, map[int]PreparedTarget, error) {
	if len(d.Targets) > captureset.MaxTargets {
		return nil, nil, errors.New("target bound exceeded")
	}
	out := make([]captureset.Target, len(d.Targets))
	by := map[int]PreparedTarget{}
	seedIdentities := make(map[string]struct{}, len(d.Targets))
	seedCoordinates := make(map[string]struct{}, len(d.Targets))
	targetIdentities := make(map[string]struct{}, len(d.Targets))
	accounting := make(map[int]census.SymbolDisposition, len(d.Accounting.Symbols))
	ledger := make(map[int]string, len(d.SymbolLedger.Entries))
	for _, e := range d.Accounting.Symbols {
		accounting[e.Ordinal] = e.Disposition
	}
	for _, e := range d.SymbolLedger.Entries {
		ledger[e.Ordinal] = e.Disposition
	}
	for i, t := range d.Targets {
		if t.CensusOrdinal < 0 || t.CensusOrdinal >= d.Accounting.SymbolDenominator || accounting[t.CensusOrdinal] != census.SymbolSelected || ledger[t.CensusOrdinal] != SymbolPrepared || len(t.CanonicalSeedV2) == 0 || t.URI == "" {
			return nil, nil, fmt.Errorf("ineligible prepared target %d", i)
		}
		if _, ok := by[t.CensusOrdinal]; ok {
			return nil, nil, errors.New("duplicate target ordinal")
		}
		if _, ok := seedIdentities[string(t.CanonicalSeedV2)]; ok {
			return nil, nil, errors.New("duplicate target identity")
		}
		relative, identityErr := canonicalWorkspaceRelativePath(d.Workspace, t.URI)
		expectedIdentity := fmt.Sprintf("%s#%d:%d:%d:%s:%d", relative, t.SelectionRange.Start.Line, t.SelectionRange.Start.Character, t.Kind, t.Name, t.CensusOrdinal)
		if identityErr != nil || t.SymbolIdentity == "" || t.SymbolIdentity != expectedIdentity || t.SymbolIdentity != ledgerIdentity(d.SymbolLedger, t.CensusOrdinal) {
			return nil, nil, errors.New("prepared target identity mismatch")
		}
		if _, ok := targetIdentities[t.SymbolIdentity]; ok {
			return nil, nil, errors.New("duplicate target identity")
		}
		if err := reconcileSeed(t, d.Workspace); err != nil {
			return nil, nil, err
		}
		coordinate := fmt.Sprintf("%s\x00%d\x00%d", t.URI, t.Position().Line, t.Position().Character)
		if _, ok := seedCoordinates[coordinate]; ok {
			return nil, nil, errors.New("duplicate exact seed coordinates")
		}
		seedIdentities[string(t.CanonicalSeedV2)] = struct{}{}
		seedCoordinates[coordinate] = struct{}{}
		targetIdentities[t.SymbolIdentity] = struct{}{}
		by[t.CensusOrdinal] = cloneTarget(t)
		out[i] = captureset.Target{CensusOrdinal: t.CensusOrdinal, CanonicalSeedV2: string(t.CanonicalSeedV2), CanonicalSeedV2SHA256: rawDigest(t.CanonicalSeedV2)}
	}
	out = captureset.OrderTargets(out)
	return out, by, nil
}
func ledgerIdentity(ledger captureset.Ledger, ordinal int) string {
	for _, entry := range ledger.Entries {
		if entry.Ordinal == ordinal {
			return entry.Identity
		}
	}
	return ""
}

func reconcileSeed(t PreparedTarget, workspace string) error {
	if t.Position().Line == math.MaxUint32 || t.Position().Character == math.MaxUint32 {
		return errors.New("canonical seed coordinate overflow")
	}
	f, err := seedformat.Decode(t.CanonicalSeedV2, workspace)
	if err != nil || len(f.Seeds) != 1 || f.Seeds[0].Type != seedformat.PositionType || f.Seeds[0].Position == nil {
		return errors.New("invalid canonical seed bytes")
	}
	canonical, err := seedformat.EncodeCanonical(f, workspace)
	if err != nil || !bytes.Equal(canonical, t.CanonicalSeedV2) {
		return errors.New("noncanonical seed bytes")
	}
	seed := f.Seeds[0].Position
	relative, pathErr := canonicalWorkspaceRelativePath(workspace, t.URI)
	if pathErr != nil || !platformPathEqual(runtime.GOOS, relative, seed.Path) || seed.Line != uint64(t.Position().Line)+1 || seed.Column != uint64(t.Position().Character)+1 || seed.Label != fmt.Sprintf("census-%06d", t.CensusOrdinal) {
		return errors.New("canonical seed target reconciliation mismatch")
	}
	return nil
}

func canonicalWorkspaceRelativePath(workspace, rawURI string) (string, error) {
	if workspace == "" || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return "", errors.New("canonical absolute workspace required")
	}
	u, err := url.Parse(rawURI)
	if err != nil || u.Scheme != "file" || u.Opaque != "" || u.User != nil || u.Host != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New("strict local file URI required")
	}
	canonicalURI := (&url.URL{Scheme: "file", Path: u.Path}).String()
	if rawURI != canonicalURI {
		return "", errors.New("noncanonical file URI")
	}
	targetPath := filepath.FromSlash(u.Path)
	if runtime.GOOS == "windows" && len(targetPath) >= 3 && targetPath[0] == filepath.Separator && targetPath[2] == ':' {
		targetPath = targetPath[1:]
	}
	if !filepath.IsAbs(targetPath) || filepath.Clean(targetPath) != targetPath {
		return "", errors.New("canonical absolute target required")
	}
	relative, err := filepath.Rel(workspace, targetPath)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("target outside workspace")
	}
	relative = filepath.ToSlash(relative)
	if relative == "." || relative == "" || path.IsAbs(relative) || path.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, "../") {
		return "", errors.New("noncanonical workspace-relative target")
	}
	return relative, nil
}

func platformPathEqual(goos, a, b string) bool {
	if goos == "windows" {
		a = strings.ReplaceAll(a, `\`, "/")
		b = strings.ReplaceAll(b, `\`, "/")
		return strings.EqualFold(a, b)
	}
	return a == b
}
func batchTargetID(index int, target PreparedTarget) string {
	if index == 0 {
		return "root"
	}
	return fmt.Sprintf("target-%06d", target.CensusOrdinal)
}
func combineSeeds(ts []PreparedTarget, workspace string) ([]byte, error) {
	return combineSeedsFile(ts, workspace, seedformat.Defaults{}, false)
}
func combineSeedsForPlanning(ts []PreparedTarget, workspace string, planning PlanningConfig) ([]byte, error) {
	down, up := planning.DownDepth, planning.UpDepth
	return combineSeedsFile(ts, workspace, seedformat.Defaults{DownDepth: &down, UpDepth: &up}, true)
}
func combineSeedsFile(ts []PreparedTarget, workspace string, defaults seedformat.Defaults, bindBatchLabels bool) ([]byte, error) {
	all := make([]seedformat.Seed, 0, len(ts))
	for i, t := range ts {
		f, err := seedformat.Decode(t.CanonicalSeedV2, workspace)
		if err != nil || len(f.Seeds) != 1 {
			return nil, errors.New("one canonical seed required per target")
		}
		canonical, err := seedformat.EncodeCanonical(f, workspace)
		if err != nil || !bytes.Equal(canonical, t.CanonicalSeedV2) {
			return nil, errors.New("canonical target seed required")
		}
		seed := f.Seeds[0]
		if bindBatchLabels {
			label := batchTargetID(i, t)
			switch seed.Type {
			case seedformat.PositionType:
				seed.Position.Label = label
			case seedformat.SymbolType:
				seed.Symbol.Label = label
			default:
				return nil, errors.New("census target seed type must be position or symbol")
			}
		}
		all = append(all, seed)
	}
	return seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: defaults, Seeds: all}, workspace)
}
func admit(raw []byte, b BatchRequest) (captureset.Constituent, error) {
	if len(raw) == 0 {
		return captureset.Constituent{}, errors.New("empty V5 bytes")
	}
	if v, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil || v != graphprovenance.VersionV5 {
		return captureset.Constituent{}, errors.New("invalid V5 exact bytes")
	}
	var e graphprovenance.EvidenceV5
	if err := json.Unmarshal(raw, &e); err != nil || e.SessionID != b.Session.SessionID || e.Generation != b.Session.Generation || e.SeedSpec == nil || !bytes.Equal(e.SeedSpec.Bytes, b.CanonicalSeedsV2) || e.SeedSpec.SHA256 != rawDigest(b.CanonicalSeedsV2) {
		return captureset.Constituent{}, errors.New("V5 request identity mismatch")
	}
	sum := sha256.Sum256(raw)
	h := hex.EncodeToString(sum[:])
	return captureset.Constituent{ImmutableSelector: captureset.ConstituentSelectorPrefix + h, SchemaID: captureset.NativeV5SchemaID, SHA256: "sha256:" + h, ByteLength: len(raw), NativeV5Identity: graphprovenance.VersionV5 + "/sha256/" + h}, nil
}
func validateProjection(p Projection) error {
	if err := p.Session.Validate(); err != nil {
		return err
	}
	if len(p.Batches) == 0 {
		return errors.New("planned batches required")
	}
	planning, err := resolvePlanning(&PlanningConfig{DownDepth: p.Batches[0].DownDepth, UpDepth: p.Batches[0].UpDepth})
	if err != nil {
		return err
	}
	var mts []captureset.Target
	for _, b := range p.Batches {
		for _, t := range b.Targets {
			mts = append(mts, captureset.Target{CensusOrdinal: t.CensusOrdinal, CanonicalSeedV2: string(t.CanonicalSeedV2), CanonicalSeedV2SHA256: rawDigest(t.CanonicalSeedV2)})
		}
	}
	mts = captureset.OrderTargets(mts)
	if p.CensusID != stableID("census", planningIdentity(joinTargetBytes(mts), planning)) {
		return errors.New("census ID mutation")
	}
	plans := captureset.PlanBatches(len(mts))
	if len(plans) != len(p.Batches) || len(p.Batches) != len(p.Constituents) {
		return errors.New("batch cardinality mutation")
	}
	for i, x := range plans {
		b := p.Batches[i]
		if b.Ordinal != i || b.Session != p.Session || b.CensusID != p.CensusID || b.DownDepth != planning.DownDepth || b.UpDepth != planning.UpDepth || len(b.Targets) != x.TargetCount {
			return errors.New("batch assignment mutation")
		}
		batchIdentity := []byte(fmt.Sprintf("%s\x00%d\x00%s", p.CensusID, i, joinTargetBytes(mts[x.TargetStart:x.TargetStart+x.TargetCount])))
		wantID := stableID("batch", planningIdentity(batchIdentity, planning))
		seeds, _ := combineSeedsForPlanning(b.Targets, p.Workspace, planning)
		if b.BatchID != wantID || !bytes.Equal(b.CanonicalSeedsV2, seeds) {
			return errors.New("batch identity or seeds mutation")
		}
		c := p.Constituents[i]
		id, err := admit(c.Raw, b)
		if err != nil || c.Ordinal != i || c.BatchID != b.BatchID || c.Identity != id {
			return errors.New("constituent association mutation")
		}
		mi := p.Manifest.Batches[i].ConstituentIndex
		if mi < 0 || mi >= len(p.Manifest.Constituents) || p.Manifest.Constituents[mi] != id {
			return errors.New("manifest batch association mutation")
		}
	}
	meta := make([]captureset.Constituent, len(p.Constituents))
	for i := range p.Constituents {
		meta[i] = p.Constituents[i].Identity
	}
	m, err := captureset.Prepare(mts, meta, p.Manifest.FileLedger, p.Manifest.SymbolLedger, CensusPolicy, DuplicatePolicy)
	if err == nil {
		m, err = captureset.AssociateBatches(m, meta)
	}
	if err != nil || !reflectManifest(m, p.Manifest) {
		return errors.New("manifest mutation")
	}
	raw, err := captureset.EncodeCanonical(m)
	if err != nil || !bytes.Equal(raw, p.ManifestBytes) {
		return errors.New("manifest bytes mutation")
	}
	return nil
}
func reflectManifest(a, b captureset.Manifest) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func cloneTarget(t PreparedTarget) PreparedTarget {
	t.CanonicalSeedV2 = append([]byte(nil), t.CanonicalSeedV2...)
	return t
}
func cloneBatchRequest(b BatchRequest) BatchRequest {
	b.Targets = append([]PreparedTarget(nil), b.Targets...)
	for i := range b.Targets {
		b.Targets[i] = cloneTarget(b.Targets[i])
	}
	b.CanonicalSeedsV2 = append([]byte(nil), b.CanonicalSeedsV2...)
	return b
}
func cloneProjection(p Projection) Projection {
	q := p
	q.ManifestBytes = append([]byte(nil), p.ManifestBytes...)
	q.Batches = append([]BatchRequest(nil), p.Batches...)
	for i := range q.Batches {
		q.Batches[i].Targets = append([]PreparedTarget(nil), p.Batches[i].Targets...)
		for j := range q.Batches[i].Targets {
			q.Batches[i].Targets[j] = cloneTarget(q.Batches[i].Targets[j])
		}
		q.Batches[i].CanonicalSeedsV2 = append([]byte(nil), p.Batches[i].CanonicalSeedsV2...)
	}
	q.Constituents = append([]Constituent(nil), p.Constituents...)
	for i := range q.Constituents {
		q.Constituents[i].Raw = append([]byte(nil), p.Constituents[i].Raw...)
	}
	raw, _ := json.Marshal(p.Manifest)
	_ = json.Unmarshal(raw, &q.Manifest)
	return q
}
func resolvePlanning(config *PlanningConfig) (PlanningConfig, error) {
	if config == nil {
		return PlanningConfig{DownDepth: census.DefaultDownDepth, UpDepth: census.DefaultUpDepth}, nil
	}
	planning := *config
	if planning.DownDepth < 0 || planning.DownDepth > 64 || planning.UpDepth < 0 || planning.UpDepth > 64 {
		return PlanningConfig{}, errors.New("planning depths must each be within [0,64]")
	}
	return planning, nil
}

func planningIdentity(base []byte, planning PlanningConfig) []byte {
	out := append([]byte(nil), base...)
	if planning.DownDepth == census.DefaultDownDepth && planning.UpDepth == census.DefaultUpDepth {
		return out
	}
	return append(out, []byte(fmt.Sprintf("\x00depths:%d:%d", planning.DownDepth, planning.UpDepth))...)
}

func joinTargetBytes(ts []captureset.Target) []byte {
	var b bytes.Buffer
	for _, t := range ts {
		fmt.Fprintf(&b, "%d:%s\x00", t.CensusOrdinal, t.CanonicalSeedV2)
	}
	return b.Bytes()
}
func stableID(k string, b []byte) string {
	h := sha256.New()
	h.Write([]byte(IDDomain))
	h.Write([]byte{0})
	h.Write([]byte(k))
	h.Write([]byte{0})
	h.Write(b)
	return k + ":" + hex.EncodeToString(h.Sum(nil))
}
func rawDigest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
