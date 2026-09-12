// Package acquisitionauthority owns the opaque, single-use capabilities used
// by trusted acquisition orchestration.  Its exported names are unreachable to
// external modules because the package is internal; mint calls are additionally
// restricted by repository boundary tests to internal/acquisitionorchestration.
package acquisitionauthority

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sync"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

// Binding is the complete identity of one authorized acquisition. Input must be
// the exact canonical bytes passed to the engine. Opaque publication roots are
// bound by in-process identity, never by caller-controlled path text.
type Binding struct {
	Operation       operation.Name
	Route           string
	RequestID       string
	Workspace       string
	SessionID       string
	Generation      uint64
	Input           []byte
	PublicationRoot *publication.Root
	ArtifactStore   *publication.Root
}

type bindingDigest [32]byte

func digestBinding(b Binding) bindingDigest {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%p\x00%p\x00", b.Operation, b.Route, b.RequestID, b.Workspace, b.SessionID, b.Generation, b.PublicationRoot, b.ArtifactStore)
	h.Write(b.Input)
	var out bindingDigest
	copy(out[:], h.Sum(nil))
	return out
}

type useState struct {
	mu       sync.Mutex
	consumed bool
	nonce    [32]byte
}

func newUseState() (*useState, error) {
	s := &useState{}
	if _, err := io.ReadFull(rand.Reader, s.nonce[:]); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *useState) consume() error {
	if s == nil {
		return fmt.Errorf("authority is absent")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consumed {
		return fmt.Errorf("authority already consumed")
	}
	s.consumed = true
	return nil
}

// SeedAuthority is intentionally opaque. Copying it shares the same replay bit.
type SeedAuthority struct{ state *seedState }

type seedState struct {
	use           *useState
	binding       bindingDigest
	seedSpec      []byte
	provenance    seedbinding.CustodyMode
	authenticated bool
	hostReceipt   string
	retain        bool
}

// SeedGrant is returned only after successful one-shot consumption by the engine.
type SeedGrant struct {
	SeedSpec       []byte
	Provenance     seedbinding.CustodyMode
	Authenticated  bool
	HostReceiptID  string
	RetainSeedSpec bool
}

// MintSeedAuthority validates canonical Seeds V2 bytes against every exact
// manifest target and seals all route/request/policy/publication semantics.
// Production call sites must reside in internal/acquisitionorchestration.
func MintSeedAuthority(b Binding, seedSpec []byte, provenance seedbinding.CustodyMode, authenticated bool, hostReceipt string, retain bool) (SeedAuthority, error) {
	if b.Operation == "" || b.Route == "" || b.Workspace == "" || b.SessionID == "" || b.Generation == 0 || len(b.Input) == 0 {
		return SeedAuthority{}, fmt.Errorf("complete acquisition authority binding required")
	}
	if provenance != seedbinding.CallerAssertedLocal && provenance != seedbinding.VerifiedHost {
		return SeedAuthority{}, fmt.Errorf("closed seed custody provenance required")
	}
	if provenance == seedbinding.VerifiedHost && (!authenticated || hostReceipt == "") {
		return SeedAuthority{}, fmt.Errorf("verified host custody requires receipt")
	}
	if retain {
		if len(seedSpec) == 0 {
			return SeedAuthority{}, fmt.Errorf("retained seed bytes required")
		}
		if err := validateSeedSpec(seedSpec, b.Workspace, b.Input, b.Route); err != nil {
			return SeedAuthority{}, err
		}
	} else if len(seedSpec) != 0 {
		return SeedAuthority{}, fmt.Errorf("non-retained authority cannot carry seed bytes")
	}
	use, err := newUseState()
	if err != nil {
		return SeedAuthority{}, err
	}
	return SeedAuthority{state: &seedState{use: use, binding: digestBinding(b), seedSpec: bytes.Clone(seedSpec), provenance: provenance, authenticated: authenticated, hostReceipt: hostReceipt, retain: retain}}, nil
}

// ConsumeSeedAuthority validates exact binding before atomically spending the token.
func ConsumeSeedAuthority(a SeedAuthority, b Binding) (SeedGrant, error) {
	if a.state == nil || a.state.binding != digestBinding(b) {
		return SeedGrant{}, fmt.Errorf("seed authority binding mismatch")
	}
	if err := a.state.use.consume(); err != nil {
		return SeedGrant{}, err
	}
	return SeedGrant{SeedSpec: bytes.Clone(a.state.seedSpec), Provenance: a.state.provenance, Authenticated: a.state.authenticated, HostReceiptID: a.state.hostReceipt, RetainSeedSpec: a.state.retain}, nil
}

// PreparedSourceCapability is an opaque one-shot right to reuse one exact
// manager-observed document supply in the same bound operation.
type PreparedSourceCapability struct{ state *preparedState }

type preparedState struct {
	use     *useState
	binding bindingDigest
	doc     sessionruntime.DocumentResult
	content [32]byte
	params  [32]byte
}

type DocumentPreparer interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

// PrepareSource performs the authoritative preparation and seals its exact result.
// Production call sites must reside in internal/acquisitionorchestration.
func PrepareSource(ctx context.Context, runtime DocumentPreparer, req sessionruntime.DocumentRequest, b Binding) (sessionruntime.DocumentResult, PreparedSourceCapability, error) {
	if runtime == nil || req.SessionID != b.SessionID || req.Generation != b.Generation || !req.CaptureSupply {
		return sessionruntime.DocumentResult{}, PreparedSourceCapability{}, fmt.Errorf("prepared source binding mismatch")
	}
	doc := runtime.PrepareDocument(ctx, req)
	capability, err := SealPreparedSource(doc, req, b)
	return doc, capability, err
}

// SealPreparedSource binds a previously captured manager result after any exact
// symbol lookup has completed. Production call sites are restricted to trusted
// orchestration; the document value alone remains non-authoritative.
func SealPreparedSource(doc sessionruntime.DocumentResult, req sessionruntime.DocumentRequest, b Binding) (PreparedSourceCapability, error) {
	if doc.Failure != "" || doc.Supply == nil {
		return PreparedSourceCapability{}, fmt.Errorf("prepared document supply unavailable")
	}
	if err := validateDocument(req, doc); err != nil {
		return PreparedSourceCapability{}, err
	}
	use, err := newUseState()
	if err != nil {
		return PreparedSourceCapability{}, err
	}
	cloned := cloneDocument(doc)
	return PreparedSourceCapability{state: &preparedState{use: use, binding: digestBinding(b), doc: cloned, content: sha256.Sum256(cloned.Supply.Content), params: sha256.Sum256(cloned.Supply.Params)}}, nil
}

// ConsumePreparedSource validates exact operation and immutable document identity.
func ConsumePreparedSource(c PreparedSourceCapability, b Binding) (sessionruntime.DocumentResult, error) {
	if c.state == nil || c.state.binding != digestBinding(b) || c.state.doc.Supply == nil {
		return sessionruntime.DocumentResult{}, fmt.Errorf("prepared source capability mismatch")
	}
	doc := c.state.doc
	if sha256.Sum256(doc.Supply.Content) != c.state.content || sha256.Sum256(doc.Supply.Params) != c.state.params {
		return sessionruntime.DocumentResult{}, fmt.Errorf("prepared source content mismatch")
	}
	if err := c.state.use.consume(); err != nil {
		return sessionruntime.DocumentResult{}, err
	}
	return cloneDocument(doc), nil
}

func validateDocument(req sessionruntime.DocumentRequest, doc sessionruntime.DocumentResult) error {
	if doc.Supply == nil || doc.URI != req.URI || doc.Supply.URI != req.URI || doc.Supply.SessionID != req.SessionID || doc.Supply.Generation != req.Generation || doc.Supply.DocumentVersion != doc.Version || (req.LanguageID != "" && doc.LanguageID != "" && req.LanguageID != doc.LanguageID) {
		return fmt.Errorf("prepared document identity mismatch")
	}
	return nil
}

func cloneDocument(in sessionruntime.DocumentResult) sessionruntime.DocumentResult {
	out := in
	if in.Supply != nil {
		s := *in.Supply
		s.Content = bytes.Clone(in.Supply.Content)
		s.Params = bytes.Clone(in.Supply.Params)
		out.Supply = &s
	}
	return out
}

type boundInput struct {
	SessionID      string          `json:"session_id"`
	Generation     uint64          `json:"generation"`
	SeedManifest   boundManifest   `json:"seed_manifest"`
	OutputVersion  string          `json:"output_version,omitempty"`
	ProductionV5   bool            `json:"production_v5,omitempty"`
	GroupBy        string          `json:"group_by,omitempty"`
	GroupOptions   json.RawMessage `json:"group_options,omitempty"`
	OutputSelector string          `json:"output_selector,omitempty"`
}
type boundManifest struct {
	SchemaVersion        string          `json:"schema_version"`
	CoordinateConvention string          `json:"coordinate_convention"`
	Root                 boundTarget     `json:"root"`
	RequiredTargets      []boundTarget   `json:"required_targets"`
	Limits               json.RawMessage `json:"limits,omitempty"`
	Expansion            json.RawMessage `json:"expansion,omitempty"`
}
type boundTarget struct {
	ID        string       `json:"id"`
	Locator   boundLocator `json:"locator"`
	DownDepth *int         `json:"down_depth,omitempty"`
	UpDepth   *int         `json:"up_depth,omitempty"`
}
type boundLocator struct {
	URI        string  `json:"uri"`
	Line       *uint32 `json:"line,omitempty"`
	Character  *uint32 `json:"character,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	LanguageID string  `json:"language_id,omitempty"`
}
type explicitSeedFile struct {
	SchemaVersion        string            `json:"schema_version"`
	CoordinateConvention string            `json:"coordinate_convention"`
	Defaults             explicitDepths    `json:"defaults"`
	Seeds                []json.RawMessage `json:"seeds"`
}
type explicitDepths struct {
	DownDepth *int `json:"down_depth,omitempty"`
	UpDepth   *int `json:"up_depth,omitempty"`
}
type explicitSeed struct {
	Type      string          `json:"type"`
	Label     string          `json:"label,omitempty"`
	Path      string          `json:"path,omitempty"`
	Line      uint64          `json:"line,omitempty"`
	Column    uint64          `json:"column,omitempty"`
	Symbol    string          `json:"symbol,omitempty"`
	Target    json.RawMessage `json:"target,omitempty"`
	DownDepth *int            `json:"down_depth,omitempty"`
	UpDepth   *int            `json:"up_depth,omitempty"`
}

func validateSeedSpec(seedSpec []byte, workspace string, inputRaw []byte, route string) error {
	var in boundInput
	if err := decodeExact(inputRaw, &in); err != nil {
		return fmt.Errorf("canonical acquisition input: %w", err)
	}
	if in.SessionID == "" || in.Generation == 0 || (in.OutputVersion != "lsp-trace.graph-provenance.v5" && !in.ProductionV5) {
		return fmt.Errorf("exact V5 acquisition input required")
	}
	var file explicitSeedFile
	if err := decodeExact(seedSpec, &file); err != nil {
		return err
	}
	if file.SchemaVersion != "lsp-trace.seeds.v2" || file.CoordinateConvention != "one-based" || len(file.Seeds) != 1+len(in.SeedManifest.RequiredTargets) {
		return fmt.Errorf("seed schema or count mismatch")
	}
	canonical, err := json.Marshal(file)
	if err != nil || !bytes.Equal(seedSpec, canonical) {
		return fmt.Errorf("seeds must be canonical bytes")
	}
	targets := append([]boundTarget{in.SeedManifest.Root}, in.SeedManifest.RequiredTargets...)
	usedIDs := map[string]bool{}
	for i, raw := range file.Seeds {
		var seed explicitSeed
		if err := decodeExact(raw, &seed); err != nil {
			return fmt.Errorf("seed %d: %w", i, err)
		}
		base := seed
		if seed.Type == "slice" {
			if err := decodeExact(seed.Target, &base); err != nil {
				return fmt.Errorf("seed %d target: %w", i, err)
			}
		}
		t := targets[i]
		label := seed.Label
		if label == "" {
			label = base.Label
		}
		expectedID := label
		if route != "explicit-trace" {
			if i == 0 {
				expectedID = "root"
			} else {
				expectedID = collisionSafeID(label, usedIDs)
			}
		}
		if expectedID != t.ID {
			return fmt.Errorf("seed %d label/order mismatch", i)
		}
		usedIDs[expectedID] = true
		u, err := url.Parse(t.Locator.URI)
		if err != nil || u.Scheme != "file" || u.Host != "" {
			return fmt.Errorf("seed %d manifest URI mismatch", i)
		}
		resolved, err := filepath.Abs(filepath.Join(workspace, filepath.FromSlash(base.Path)))
		if err != nil || filepath.Clean(resolved) != filepath.Clean(filepath.FromSlash(u.Path)) {
			return fmt.Errorf("seed %d path mismatch", i)
		}
		switch base.Type {
		case "position":
			if t.Locator.Symbol != "" || t.Locator.Line == nil || t.Locator.Character == nil || base.Line != uint64(*t.Locator.Line)+1 || base.Column != uint64(*t.Locator.Character)+1 {
				return fmt.Errorf("seed %d position mismatch", i)
			}
		case "symbol":
			if t.Locator.Symbol == "" || base.Symbol != t.Locator.Symbol {
				return fmt.Errorf("seed %d symbol mismatch", i)
			}
		default:
			return fmt.Errorf("seed %d target type mismatch", i)
		}
		down, up := file.Defaults.DownDepth, file.Defaults.UpDepth
		if seed.DownDepth != nil {
			down = seed.DownDepth
		}
		if seed.UpDepth != nil {
			up = seed.UpDepth
		}
		depth := func(v *int) int {
			if v == nil {
				return 2
			}
			return *v
		}
		if depth(down) != depth(t.DownDepth) || depth(up) != depth(t.UpDepth) {
			return fmt.Errorf("seed %d depth mismatch", i)
		}
	}
	return nil
}

func collisionSafeID(label string, used map[string]bool) string {
	candidate := label
	if candidate == "root" {
		candidate = "seed-root"
	}
	if !used[candidate] {
		return candidate
	}
	base := candidate
	for suffix := 2; ; suffix++ {
		candidate = fmt.Sprintf("%s-%d", base, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

func decodeExact(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON object required")
	}
	return nil
}
