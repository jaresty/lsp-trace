package graphprovenance

// This is a finite-variable-scope private profile, NOT ordinary graph provenance
// V2 schema admission. EvidenceV2 is descriptor reuse only. Decoding is metadata,
// never acquisition authorization, source capability, or replay admission.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"unicode/utf8"

	"lsp-trace/internal/acquisition"
	"lsp-trace/sessionruntime"
)

const VersionSourceWithholdingPrivateV2 = "lsp-trace.source-withholding.private.v2"
const PolicySourceWithholdingPrivateV2 = "FINITE_VARIABLE_SCOPE_PRIVATE_V2"
const VersionSourcePolicyPrivateV2 = "lsp-trace.finite-variable-source-policy.private.v2"
const PolicyWithheldPrivateV2 = "POLICY_WITHHELD"

// Prospective private V2 artifact policy, not the ordinary/V1 192 MiB limit.
const MaxSourceWithholdingPrivateV2 = 4 << 20

type SourceMemberPrivateV2 struct {
	URI    string `json:"uri"`
	SHA256 string `json:"sha256"`
	Role   string `json:"role"`
}
type SourcePolicyPrivateV2 struct {
	SchemaVersion string                  `json:"schema_version"`
	WorkspaceURI  string                  `json:"workspace_uri"`
	SessionID     string                  `json:"session_id"`
	Generation    uint64                  `json:"generation"`
	Members       []SourceMemberPrivateV2 `json:"members"`
}
type BoundSourcePrivateV2 struct {
	self     *BoundSourcePrivateV2
	manager  *sessionruntime.Manager
	guard    *sessionruntime.RetainedScopeGuardPrivateV2
	policy   SourcePolicyPrivateV2
	expected map[string][]byte
}
type SourceWithholdingPrivateV2 struct {
	SchemaVersion      string                `json:"schema_version"`
	Policy             SourcePolicyPrivateV2 `json:"source_policy"`
	Evidence           EvidenceV2            `json:"evidence"`
	SourceCompleteness string                `json:"source_completeness"`
	Accepted           bool                  `json:"accepted"`
	Authority          int                   `json:"authority"`
	ReplayAdmitted     bool                  `json:"replay_admitted"`
	Partial            bool                  `json:"partial"`
	WithheldCount      int                   `json:"withheld_count"`
	FrozenQueryScope   bool                  `json:"frozen_query_scope"`
}

func (p SourcePolicyPrivateV2) validate() error {
	// Length/count preflight precedes UTF-8 parsing or member-view allocation.
	if len(p.SessionID) == 0 || len(p.SessionID) > 1024 || len(p.WorkspaceURI) > 8192 || p.Generation == 0 || len(p.Members) < 1 || len(p.Members) > 16 {
		return errors.New("private V2 source identity/count bounds")
	}
	total := len(p.WorkspaceURI) + len(p.SessionID)
	for _, m := range p.Members {
		if len(m.URI) > 8192 || len(m.SHA256) != 71 || len(m.Role) > 12 {
			return errors.New("private V2 source member metadata bounds")
		}
		total += len(m.URI) + len(m.SHA256) + len(m.Role)
		if total > 128<<10 {
			return errors.New("private V2 source aggregate metadata bound")
		}
	}
	u, err := url.Parse(p.WorkspaceURI)
	if err != nil || !utf8.ValidString(p.WorkspaceURI) || !utf8.ValidString(p.SessionID) || u.Scheme != "file" || u.Host != "" || !filepath.IsAbs(u.Path) || filepath.Clean(u.Path) != u.Path || (&url.URL{Scheme: "file", Path: u.Path}).String() != p.WorkspaceURI || p.SchemaVersion != VersionSourcePolicyPrivateV2 {
		return errors.New("private V2 source policy identity")
	}
	nom, ctx := 0, 0
	for i, m := range p.Members {
		_, s := RelativeURI(p.WorkspaceURI, m.URI)
		if !utf8.ValidString(m.URI) || s != "" || (i > 0 && p.Members[i-1].URI >= m.URI) || !validSourceHashPrivateV1(m.SHA256) {
			return errors.New("private V2 source canonical member/hash")
		}
		switch m.Role {
		case "NOMINATION":
			nom++
		case "CONTEXT_ONLY":
			ctx++
		default:
			return errors.New("private V2 source role")
		}
	}
	if nom < 1 || nom > 8 || ctx > 8 {
		return errors.New("private V2 source role counts")
	}
	return nil
}
func sourceProfileV2Private(p SourcePolicyPrivateV2) (*sourceProfilePrivate, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	v := &sourceProfilePrivate{version: VersionSourceWithholdingPrivateV2, policy: PolicySourceWithholdingPrivateV2, workspace: p.WorkspaceURI, session: p.SessionID, generation: p.Generation}
	for _, m := range p.Members {
		v.members = append(v.members, sourceMemberPrivate{m.URI, m.SHA256, m.Role})
	}
	return v, nil
}
func (p SourcePolicyPrivateV2) queryMembers() []string {
	members := make([]string, len(p.Members))
	for i, m := range p.Members {
		members[i] = m.URI
	}
	return members
}
func BindSourcePrivateV2(ctx context.Context, m *sessionruntime.Manager, g *sessionruntime.RetainedScopeGuardPrivateV2, sessionID string, gen uint64, workspace string) (*BoundSourcePrivateV2, error) {
	if ctx == nil || m == nil || g == nil || len(sessionID) == 0 || len(sessionID) > 1024 || len(workspace) > 8192 || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return nil, errors.New("private V2 source binding inputs")
	}
	if failure := m.CheckRetainedScopePrivateV2(ctx, sessionID, gen, g); failure != "" {
		return nil, fmt.Errorf("private V2 source scope: %s", failure)
	}
	current, ok := m.WorkspaceRoot(sessionID, gen)
	if !ok || current != workspace {
		return nil, errors.New("private V2 source workspace mismatch")
	}
	workspaceURI := (&url.URL{Scheme: "file", Path: workspace}).String()
	if len(workspaceURI) > 8192 {
		return nil, errors.New("private V2 source workspace URI bounds")
	}
	members := g.MembersPrivate()
	// Check every bound before allocating the source member slice or expected map.
	total, metadata := 0, len(sessionID)+len(workspaceURI)
	if len(members) < 1 || len(members) > 16 || len(sessionID) > 1024 || len(workspace) > 8192 {
		return nil, errors.New("private V2 source binding bounds")
	}
	for _, member := range members {
		total += len(member.Expected)
		metadata += len(member.URI) + 71 + len(member.Role)
		if len(member.Expected) > MaxFileBytes || total > 8<<20 || len(member.URI) > 8192 || metadata > 128<<10 {
			return nil, errors.New("private V2 source binding bytes")
		}
	}
	p := SourcePolicyPrivateV2{SchemaVersion: VersionSourcePolicyPrivateV2, WorkspaceURI: workspaceURI, SessionID: sessionID, Generation: gen, Members: make([]SourceMemberPrivateV2, 0, len(members))}
	expected := make(map[string][]byte, len(members))
	for _, member := range members {
		p.Members = append(p.Members, SourceMemberPrivateV2{member.URI, sourceHashPrivateV1(member.Expected), string(member.Role)})
		expected[member.URI] = append([]byte(nil), member.Expected...)
	}
	sort.Slice(p.Members, func(i, j int) bool { return p.Members[i].URI < p.Members[j].URI })
	if err := p.validate(); err != nil {
		return nil, err
	}
	b := &BoundSourcePrivateV2{manager: m, guard: g, policy: p, expected: expected}
	b.self = b
	return b, nil
}
func (b *BoundSourcePrivateV2) check(ctx context.Context) error {
	if ctx == nil || b == nil || b.self != b || b.manager == nil || b.guard == nil {
		return errors.New("private V2 source bound capability required")
	}
	if failure := b.manager.CheckRetainedScopePrivateV2(ctx, b.policy.SessionID, b.policy.Generation, b.guard); failure != "" {
		return fmt.Errorf("private V2 source capability expired: %s", failure)
	}
	return nil
}
func (b *BoundSourcePrivateV2) FrozenQueryDeclarationPrivateV2(ctx context.Context) (acquisition.FrozenQueryDeclarationPrivateV2, error) {
	if err := b.check(ctx); err != nil {
		return acquisition.FrozenQueryDeclarationPrivateV2{}, err
	}
	return acquisition.NewFrozenQueryDeclarationPrivateV2(b.policy.queryMembers())
}

// This prospective V2 role constraint governs source-profile seeds only. Pure
// acquisition query declarations remain restrict-only metadata, not authority.
func (p SourcePolicyPrivateV2) validateSeeds(r acquisition.Request) error {
	nominated := func(uri string) bool {
		for _, member := range p.Members {
			if uri == member.URI && member.Role == "NOMINATION" {
				return true
			}
		}
		return false
	}
	if !nominated(r.Root.Locator.URI) {
		return errors.New("private V2 source seed must be NOMINATION")
	}
	for _, target := range r.RequiredTargets {
		if !nominated(target.Locator.URI) {
			return errors.New("private V2 source seed must be NOMINATION")
		}
	}
	return nil
}

func CaptureSourceWithholdingPrivateV2(ctx context.Context, r acquisition.Result, b *BoundSourcePrivateV2) ([]byte, error) {
	return captureBoundSourceWithholdingPrivateV2(ctx, r, b, readSourcePrivateV1)
}
func captureBoundSourceWithholdingPrivateV2(ctx context.Context, r acquisition.Result, b *BoundSourcePrivateV2, read func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error)) ([]byte, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := b.check(ctx); err != nil {
		return nil, err
	}
	return captureSourceWithholdingPrivateV2(ctx, r, b.policy, b.expected, read)
}
func captureSourceWithholdingPrivateV2(ctx context.Context, r acquisition.Result, p SourcePolicyPrivateV2, expected map[string][]byte, read func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error)) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("private V2 source context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view, err := sourceProfileV2Private(p)
	if err != nil {
		return nil, err
	}
	if err := p.validateSeeds(r.Request); err != nil {
		return nil, err
	}
	members, err := r.Request.FrozenQueryMembersPrivateV2()
	if err != nil || members == nil || !reflect.DeepEqual(members, p.queryMembers()) || r.Request.FrozenQueryURIsPrivate != [6]string{} {
		return nil, errors.New("private V2 query/source policy mismatch")
	}
	out, captureErr := captureSourceProfilePrivate(ctx, r, view, expected, read, func(s EvidenceV2, n int) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e := SourceWithholdingPrivateV2{SchemaVersion: VersionSourceWithholdingPrivateV2, Policy: p, Evidence: s, SourceCompleteness: "UNKNOWN", Partial: true, WithheldCount: n, FrozenQueryScope: true}
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		if _, err = ParseSourceWithholdingPrivateV2(raw); err != nil {
			return nil, err
		}
		if len(raw) >= MaxSourceWithholdingPrivateV2 {
			return nil, errors.New("private V2 encoded artifact exceeds limit")
		}
		return append(raw, '\n'), nil
	})
	// Defined late-cancellation stop point, not an atomic cancellation guarantee.
	// The shared capture has already run its root-close defer; preserve its error.
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(captureErr, err)
	}
	return out, captureErr
}
func ParseSourceWithholdingPrivateV2(raw []byte) (SourceWithholdingPrivateV2, error) {
	var e SourceWithholdingPrivateV2
	if err := preflightV2(raw, MaxSourceWithholdingPrivateV2); err != nil {
		return e, err
	}
	if err := preflightSourceArraysPrivate(raw, 16); err != nil {
		return e, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	canonical, err := json.Marshal(e)
	if err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	a, ae := canonicalV2(raw)
	b, be := canonicalV2(canonical)
	if ae != nil || be != nil || !bytes.Equal(a, b) {
		return SourceWithholdingPrivateV2{}, errors.New("private V2 missing/noncanonical fields")
	}
	if err = e.Policy.validate(); err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	// Reconstruct restrict-only query metadata, never a bound source or execution
	// grant. The policy is the single canonical member list.
	declaration, err := acquisition.NewFrozenQueryDeclarationPrivateV2(e.Policy.queryMembers())
	if err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	e.Evidence.Acquisition.Request, err = e.Evidence.Acquisition.Request.WithFrozenQueryDeclarationPrivateV2(declaration)
	if err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	if err = validateSourceWithholdingPrivateV2(e); err != nil {
		return SourceWithholdingPrivateV2{}, err
	}
	return e, nil
}
func validateSourceWithholdingPrivateV2(e SourceWithholdingPrivateV2) error {
	if e.SchemaVersion != VersionSourceWithholdingPrivateV2 || e.SourceCompleteness != "UNKNOWN" || e.Accepted || e.Authority != 0 || e.ReplayAdmitted || !e.Partial || !e.FrozenQueryScope {
		return errors.New("private V2 source profile ceiling")
	}
	view, err := sourceProfileV2Private(e.Policy)
	if err != nil {
		return err
	}
	if err := e.Policy.validateSeeds(e.Evidence.Acquisition.Request); err != nil {
		return err
	}
	members, err := e.Evidence.Acquisition.Request.FrozenQueryMembersPrivateV2()
	if err != nil || !reflect.DeepEqual(members, e.Policy.queryMembers()) {
		return errors.New("private V2 source query metadata mismatch")
	}
	return validateSourceProfilePrivate(e.Evidence, view, e.WithheldCount)
}
