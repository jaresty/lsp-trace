//go:build darwin || linux

package censuslineagepilot

// This entire construction bridge exists only in the package's test binary.
// Production cannot mint an adapter, pending claim, or arbitrary runtime client.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/acquisitionauthority"
	"lsp-trace/internal/acquisitionmanifest"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

const ScopedRehearsalConfigLimitPrivate = 32 << 10
const scopedRehearsalArtifactPrivate = 2 << 20
const scopedRehearsalTotalPrivate = 16 << 20
const scopedRehearsalObjectsPrivate = 8

// Fixed finite proposal, private observed policy, NOT public defaults. Two staging
// copies and sixteen byte-equivalent read/hash passes are reserved before opening
// the runtime (including both existing-object pre-reads and duplicate hashing).
type ScopedRehearsalBudgetPrivate struct {
	ArtifactBytes int `json:"artifact_bytes"`
	TotalBytes    int `json:"total_bytes"`
	Objects       int `json:"objects"`
	StagingBytes  int `json:"staging_bytes"`
	ReadHashBytes int `json:"read_hash_bytes"`
}

func ScopedRehearsalBudgetFixedPrivate() ScopedRehearsalBudgetPrivate {
	return ScopedRehearsalBudgetPrivate{scopedRehearsalArtifactPrivate, scopedRehearsalTotalPrivate, scopedRehearsalObjectsPrivate, 2 * scopedRehearsalTotalPrivate, 16 * scopedRehearsalTotalPrivate}
}

type ScopedRehearsalConfigPrivate struct {
	Schema               string                                    `json:"schema"`
	InputRoots           [3]string                                 `json:"input_roots"`
	Locator              scopeRestartLocator                       `json:"restart_locator"`
	OutputRoots          [3]string                                 `json:"output_roots"`
	OutputIdentities     [3]publication.RetentionDirectoryIdentity `json:"output_identities"`
	Workspace            string                                    `json:"workspace"`
	CodeHEAD             string                                    `json:"executable_head"`
	Session              censusacquisition.SessionIdentity         `json:"session"`
	TrustDomain          string                                    `json:"trust_domain"`
	Profile              string                                    `json:"profile"`
	EnvironmentReference string                                    `json:"environment_reference"`
	Server               string                                    `json:"server"`
	Args                 []string                                  `json:"args"`
	Environment          []string                                  `json:"environment"`
	Budget               ScopedRehearsalBudgetPrivate              `json:"budget"`
}

func ParseScopedRehearsalConfigPrivate(raw []byte) (c ScopedRehearsalConfigPrivate, err error) {
	if len(raw) == 0 || len(raw) > ScopedRehearsalConfigLimitPrivate {
		return c, ErrInvalid
	}
	if err = scopeBundleStrict(raw, &c); err != nil {
		return c, err
	}
	canonical, e := json.Marshal(c)
	if e != nil || !bytes.Equal(raw, canonical) {
		return c, ErrInvalid
	}
	return c, c.validate(true)
}
func (c ScopedRehearsalConfigPrivate) executionPolicy() (scopedExecutionPolicyPrivate, error) {
	switch c.Schema {
	case "lsp-trace.scoped-rehearsal.private.v1":
		return scopedExecutionPolicyFixedPrivate(), nil
	case "lsp-trace.scoped-rehearsal.private.v2":
		return scopedExecutionPolicyV2Private(), nil
	default:
		return scopedExecutionPolicyPrivate{}, ErrInvalid
	}
}

// V2 observation only. V1 serialized reports omit this additive observation.
// These are effective fixed resources, never caller-settable configuration.
type ScopedDiscoveryPolicyObservationPrivate struct {
	Name       string `json:"name"`
	MaxSymbols int    `json:"max_symbols"`
	MaxFiles   int    `json:"max_files"`
	MaxDepth   int    `json:"max_depth"`
	MaxTargets int    `json:"max_targets"`
}

func (c ScopedRehearsalConfigPrivate) DiscoveryPolicyObservationPrivate() (*ScopedDiscoveryPolicyObservationPrivate, error) {
	p, err := c.executionPolicy()
	if err != nil {
		return nil, err
	}
	if p == scopedExecutionPolicyFixedPrivate() {
		return nil, nil
	}
	limits, err := p.discoveryLimits()
	if err != nil {
		return nil, err
	}
	return &ScopedDiscoveryPolicyObservationPrivate{p.Name, limits.MaxSymbols, limits.MaxFiles, limits.MaxDepth, p.MaxTargets}, nil
}

func (c ScopedRehearsalConfigPrivate) validate(outsideTmp bool) error {
	if _, err := c.executionPolicy(); err != nil {
		return err
	}
	if !c.Locator.valid() || !featureScopeRevision.MatchString(c.CodeHEAD) || c.Session.Validate() != nil || c.Budget != ScopedRehearsalBudgetFixedPrivate() || len(c.Args) > 16 || len(c.Environment) == 0 || len(c.Environment) > 32 {
		return ErrInvalid
	}
	for _, p := range append(append([]string{c.Workspace, c.Server}, c.InputRoots[:]...), c.OutputRoots[:]...) {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || len(p) > 4096 {
			return ErrInvalid
		}
	}
	for i, p := range c.OutputRoots {
		if c.OutputIdentities[i].Inode == 0 {
			return ErrInvalid
		}
		if outsideTmp {
			for _, temporary := range []string{"/tmp", "/private/tmp", "/var/tmp", "/private/var/tmp", "/var/folders", "/private/var/folders", filepath.Clean(os.TempDir())} {
				if p == temporary || strings.HasPrefix(p, temporary+"/") {
					return ErrInvalid
				}
			}
		}
	}
	v, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: c.TrustDomain, Workspace: c.Workspace, Profile: c.Profile, EnvironmentReference: c.EnvironmentReference})
	if e != nil || runtimeprofile.Resolve(v).Workspace().String() != c.Workspace {
		return ErrInvalid
	}
	previous := ""
	for _, entry := range c.Environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name == "" || name <= previous || strings.ContainsRune(entry, 0) || len(entry) > 4096 {
			return ErrInvalid
		}
		for i, ch := range []byte(name) {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || (i > 0 && ch >= '0' && ch <= '9')) {
				return ErrInvalid
			}
		}
		previous = name
	}
	for _, arg := range c.Args {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return ErrInvalid
		}
	}
	return nil
}

// Open is pre-runtime: read-only exact retained reconstruction, finite plan and
// topology admission all precede initialization or semantic traffic. It creates
// no roots and never opens the input bundle's mutation interfaces.
type ScopedRehearsalPrivate struct {
	config        ScopedRehearsalConfigPrivate
	restart       scopeRestartResult
	roots         [3]*publication.ContentRetentionRoot
	topology      *scopedExecutionTopologyPrivate
	store         *scopedExecutionStorePrivate
	owner         *scopedExecutionClaimOwnerPrivate
	pending       *scopedExecutionPendingPrivate
	adapter       *scopeRuntimeAdapter
	manager       *sessionruntime.Manager
	used          bool
	claimResult   scopedExecutionClaimResultPrivate
	retainedBytes int
	records       []ScopedRehearsalRecordPrivate
	retentionOps  scopeRetentionOps // nil functions select real storage; package-local fault fixtures only
}
type ScopedRehearsalRecordPrivate struct {
	Name           string                        `json:"name"`
	ID             string                        `json:"id"`
	Bytes          int                           `json:"bytes"`
	Phase          string                        `json:"phase"`
	Primary        string                        `json:"primary"`
	Backup         string                        `json:"backup"`
	PrimaryReceipt *publication.BoundFileReceipt `json:"primary_receipt,omitempty"`
	BackupReceipt  *publication.BoundFileReceipt `json:"backup_receipt,omitempty"`
	Verified       bool                          `json:"verified"`
	Error          string                        `json:"error,omitempty"`
}

func OpenScopedRehearsalPrivate(ctx context.Context, c ScopedRehearsalConfigPrivate) (h *ScopedRehearsalPrivate, err error) {
	return openScopedRehearsalPrivate(ctx, c, true)
}
func openScopedRehearsalPrivate(ctx context.Context, c ScopedRehearsalConfigPrivate, outsideTmp bool) (h *ScopedRehearsalPrivate, err error) {
	if err = c.validate(outsideTmp); err != nil {
		return nil, err
	}
	h = &ScopedRehearsalPrivate{config: c}
	defer func() {
		if err != nil {
			err = errors.Join(err, h.Close())
			h = nil
		}
	}()
	h.restart, err = readRetainedFeatureScope(ctx, c.InputRoots, c.Locator)
	if err != nil {
		return h, err
	}
	if h.restart.Scope().Selection().Workspace != c.Workspace {
		return h, ErrInvalid
	}
	for i, p := range c.OutputRoots {
		h.roots[i], err = publication.OpenContentRetentionRoot(ctx, p)
		if err != nil {
			return h, err
		}
		id, e := h.roots[i].DirectoryIdentity()
		if e != nil || id != c.OutputIdentities[i] {
			return h, errors.Join(ErrInvalid, e)
		}
	}
	h.topology, err = preflightScopedExecutionRootsPrivate(ctx, h.restart, c.InputRoots, h.roots)
	if err != nil {
		return h, err
	}
	policy, err := c.executionPolicy()
	if err != nil {
		return h, err
	}
	h.owner, err = newScopedExecutionClaimWithPolicyPrivate(ctx, h.restart, c.CodeHEAD, c.Session, c.Workspace, h.roots, policy)
	if err != nil {
		return h, err
	}
	h.store, err = newScopedExecutionStorePrivate(ctx, h.owner, h.roots, h.topology)
	return h, err
}

// Bind must run in the actual READY exact-workspace Manager lease. Initializing
// traffic is protocol traffic; six didOpen/didChange supplies are not semantic.
func (h *ScopedRehearsalPrivate) Bind(ctx context.Context, m *sessionruntime.Manager) (claim []byte, err error) {
	if h == nil || h.used || m == nil {
		return nil, ErrInvalid
	}
	if err := h.checkPolicy(); err != nil {
		return nil, err
	}
	h.used = true
	result, e := h.owner.claim(ctx, h.store)
	h.claimResult = result
	claim = result.HeadBytes()
	if e != nil {
		return claim, e
	}
	h.pending = result.pending
	if err = h.check(ctx); err != nil {
		return claim, err
	}
	policy, e := h.config.executionPolicy()
	if e != nil {
		return claim, e
	}
	h.adapter, _, err = newScopeRuntimeAdapterWithPolicyPrivate(ctx, h.restart, m, h.config.Session, policy)
	h.manager = m
	return claim, err
}

// Match the supplied configuration before readiness, retention, claim or semantic
// traffic. A schema change may not relabel a previously constructed owner.
func (h *ScopedRehearsalPrivate) MatchesConfigPrivate(c ScopedRehearsalConfigPrivate) bool {
	if h == nil || h.checkPolicy() != nil {
		return false
	}
	a, e := json.Marshal(h.config)
	b, f := json.Marshal(c)
	return e == nil && f == nil && bytes.Equal(a, b)
}

func (h *ScopedRehearsalPrivate) checkPolicy() error {
	if h == nil || h.owner == nil {
		return ErrInvalid
	}
	policy, err := h.config.executionPolicy()
	if err != nil {
		return err
	}
	head, err := parseScopedExecutionHeadPrivate(h.owner.HeadBytes())
	if err != nil || head.Binding.Policy != policy || (h.adapter != nil && h.adapter.policy != policy) {
		return ErrInvalid
	}
	return nil
}

func (h *ScopedRehearsalPrivate) check(ctx context.Context) error {
	if err := h.checkPolicy(); err != nil {
		return err
	}
	if h == nil || h.owner == nil || !h.owner.ownsPending(h.pending) {
		return ErrIndeterminateExecution
	}
	return h.store.validate(ctx, h.owner.selector)
}
func (h *ScopedRehearsalPrivate) Discover(ctx context.Context, id censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	if err := h.check(ctx); err != nil {
		return censusacquisition.Discovery{}, err
	}
	if h.adapter == nil {
		return censusacquisition.Discovery{}, ErrInvalid
	}
	return h.adapter.Discover(ctx, id)
}
func (h *ScopedRehearsalPrivate) Mint(ctx context.Context, requestID string, manifest acquisitionmanifest.Manifest, seeds []byte) (*ScopedBatchSelectionPrivate, error) {
	if err := h.check(ctx); err != nil {
		return nil, err
	}
	if h.adapter == nil {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(scopedBatchInputPrivate{SessionID: h.config.Session.SessionID, Generation: h.config.Session.Generation, Manifest: manifest, OutputVersion: graphprovenance.VersionV5})
	if err != nil {
		return nil, err
	}
	b := acquisitionauthority.Binding{Operation: acquisitionmanifest.SliceV3, Route: "private-census-batch", RequestID: requestID, Workspace: h.config.Workspace, SessionID: h.config.Session.SessionID, Generation: h.config.Session.Generation, Input: raw}
	return h.adapter.mintScopedBatchSelectionPrivate(ctx, b, seeds)
}
func (h *ScopedRehearsalPrivate) Retain(ctx context.Context, name string, raw []byte) ScopedRehearsalRecordPrivate {
	r := ScopedRehearsalRecordPrivate{Name: name, Bytes: len(raw)}
	// Reserve one object and 256 KiB for the terminal immutable observation. Raw V5
	// has a serialized cap checked AFTER producer return, never a heap-bound claim.
	ceiling := scopedRehearsalTotalPrivate - (256 << 10)
	count := scopedRehearsalObjectsPrivate - 1
	if name == "outcome" {
		ceiling = scopedRehearsalTotalPrivate
		count = scopedRehearsalObjectsPrivate
	}
	if len(raw) == 0 || len(raw) > scopedRehearsalArtifactPrivate || len(h.records) >= count || len(raw) > ceiling-h.retainedBytes {
		r.Error = "private serialized output budget exceeded"
		return r
	}
	r.ID = Digest(raw)
	h.retainedBytes += len(raw)
	ops := h.retentionOps
	if ops.publish == nil && ops.read == nil {
		ops = defaultScopeRetentionOps()
	}
	attempt, err := retainScopePairBytes(ctx, scopeRetentionObject{Digest(raw), raw}, Digest(h.owner.raw), len(h.records), h.roots[0], h.roots[1], ops)
	r.Phase = attempt.phase
	r.Primary = attempt.primary.outcome
	r.Backup = attempt.backup.outcome
	r.PrimaryReceipt = attempt.primary.receipt
	r.BackupReceipt = attempt.backup.receipt
	r.Verified = attempt.verified.state != nil
	if err != nil {
		r.Error = err.Error()
	}
	h.records = append(h.records, r)
	return r
}

// Diagnostics is observation-only: no guard minting, checks, wire work, or claim
// validation. Manager validates the existing capability and detaches the snapshot.
func (h *ScopedRehearsalPrivate) Diagnostics() ([]sessionruntime.RetainedScopeDiagnosticPrivate, bool, bool) {
	if h == nil || h.manager == nil || h.adapter == nil || h.adapter.guard == nil {
		return nil, false, false
	}
	return h.manager.RetainedScopeDiagnosticSnapshotPrivate(h.adapter.guard)
}

func (h *ScopedRehearsalPrivate) ClaimObservation() []byte {
	if !h.used {
		return nil
	}
	raw, _ := json.Marshal(struct {
		Outcome        string                                `json:"outcome"`
		Head           []byte                                `json:"head"`
		CAS            *publication.CompareAndReplaceReceipt `json:"cas,omitempty"`
		CommitReported bool                                  `json:"commit_reported"`
	}{h.claimResult.outcome, h.claimResult.HeadBytes(), h.claimResult.CASReceipt(), h.claimResult.commitReported})
	return raw
}
func (h *ScopedRehearsalPrivate) InputObservation() []byte {
	raw, _ := json.Marshal(struct {
		Head     []byte   `json:"head"`
		Scope    []byte   `json:"scope"`
		Metadata []byte   `json:"metadata"`
		Links    [][]byte `json:"links"`
	}{h.restart.HeadBytes(), h.restart.Scope().ManifestBytes(), h.restart.MetadataBytes(), h.restart.LinkedReceiptBytes()})
	return raw
}

// Finish semantic use first; stores close before proof, proof before roots.
// Outcomes are retained after this call so store/proof close failures are available.
func (h *ScopedRehearsalPrivate) CloseExecution() error {
	if h == nil {
		return nil
	}
	var err error
	if h.store != nil {
		err = errors.Join(err, h.store.Close())
		h.store = nil
	}
	if h.topology != nil {
		err = errors.Join(err, h.topology.Close())
		h.topology = nil
	}
	h.pending = nil
	return err
}
func (h *ScopedRehearsalPrivate) Close() error {
	if h == nil {
		return nil
	}
	err := h.CloseExecution()
	for i := 2; i >= 0; i-- {
		if h.roots[i] != nil {
			err = errors.Join(err, h.roots[i].Close())
			h.roots[i] = nil
		}
	}
	return err
}
func (h *ScopedRehearsalPrivate) String() string {
	return fmt.Sprintf("private scoped rehearsal (%d objects)", len(h.records))
}

func NewScopedRehearsalFixturePrivate(t *testing.T) (*ScopedRehearsalPrivate, *sessionruntime.Manager, ScopedRehearsalConfigPrivate, *ScopedBatchFixturePrivate) {
	t.Helper()
	return newScopedRehearsalFixturePrivate(t, "lsp-trace.scoped-rehearsal.private.v1")
}

func NewScopedRehearsalFixtureV2Private(t *testing.T) (*ScopedRehearsalPrivate, *sessionruntime.Manager, ScopedRehearsalConfigPrivate, *ScopedBatchFixturePrivate) {
	t.Helper()
	return newScopedRehearsalFixturePrivate(t, "lsp-trace.scoped-rehearsal.private.v2")
}

func newScopedRehearsalFixturePrivate(t *testing.T, schema string) (*ScopedRehearsalPrivate, *sessionruntime.Manager, ScopedRehearsalConfigPrivate, *ScopedBatchFixturePrivate) {
	t.Helper()
	restart, m, id, child, paths, loc := adapterFixtureWithRestartPathsAndServerPrivate(t, "/fixture/server")
	a, b, ap, bp := retentionRoots(t)
	c, _, cp, _ := retentionRoots(t)
	cfg := ScopedRehearsalConfigPrivate{Schema: schema, InputRoots: paths, Locator: loc, OutputRoots: [3]string{ap, bp, cp}, Workspace: restart.Scope().Selection().Workspace, CodeHEAD: strings.Repeat("a", 40), Session: id, TrustDomain: "test", Profile: "go", EnvironmentReference: "local", Server: "/fixture/server", Environment: []string{"PATH=/fixture/no-exec"}, Budget: ScopedRehearsalBudgetFixedPrivate()}
	for i, r := range [3]*publication.ContentRetentionRoot{a, b, c} {
		var e error
		cfg.OutputIdentities[i], e = r.DirectoryIdentity()
		if e != nil {
			t.Fatal(e)
		}
	}
	before := topologyTreePrivate(t, paths)
	h, e := openScopedRehearsalPrivate(context.Background(), cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := h.Close(); e != nil {
			t.Error(e)
		}
		if !reflect.DeepEqual(before, topologyTreePrivate(t, paths)) {
			t.Error("ASSERT_REHEARSAL_INPUT_TREE_UNCHANGED")
		}
	})
	return h, m, cfg, &ScopedBatchFixturePrivate{Manager: m, Identity: id, Workspace: cfg.Workspace, child: child}
}
