package graphprovenance

// This private format is deliberately NOT registered as graph-provenance v2–v5.
// It is inspection evidence only, not a replay/admission or analyzed-source claim.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"lsp-trace/internal/acquisition"
	"lsp-trace/sessionruntime"
)

const VersionSourceWithholdingPrivateV1 = "lsp-trace.source-withholding.private.v1"
const PolicySourceWithholdingPrivateV1 = "FROZEN_SIX_POST_CAPTURE_PRIVATE_V1"
const VersionSourcePolicyPrivateV1 = "lsp-trace.frozen-six-source-policy.private.v1"
const PolicyWithheldPrivateV1 = "POLICY_WITHHELD"
const MaxSourceWithholdingPrivateV1 = MaxEnvelopeBytesV2

var errSourcePrivateUnsupportedV1 = errors.New("private no-follow source reads unsupported on this platform")

// Hashes bind the allowed expectations, not the server's analyzed bytes.
type SourceMemberPrivateV1 struct {
	URI    string `json:"uri"`
	SHA256 string `json:"sha256"`
	Role   string `json:"role"`
}
type SourcePolicyPrivateV1 struct {
	SchemaVersion string                  `json:"schema_version"`
	WorkspaceURI  string                  `json:"workspace_uri"`
	SessionID     string                  `json:"session_id"`
	Generation    uint64                  `json:"generation"`
	Members       []SourceMemberPrivateV1 `json:"members"`
}

// BoundSourcePrivateV1 cannot be minted from caller URI lists or decoded policy.
// Only the exact constructor-minted pointer is valid; struct copies are refused.
// A fresh Manager check is required before capture, under the caller's lease.
type BoundSourcePrivateV1 struct {
	self     *BoundSourcePrivateV1
	manager  *sessionruntime.Manager
	guard    *sessionruntime.RetainedScopeGuardPrivate
	policy   SourcePolicyPrivateV1
	expected map[string][]byte
}
type SourceWithholdingPrivateV1 struct {
	SchemaVersion string                `json:"schema_version"`
	Policy        SourcePolicyPrivateV1 `json:"source_policy"`
	// Reuses the Go descriptor, not the old V2 wire schema. Its version/policy are
	// private, and only the explicit private parser admits this closed variant.
	Evidence           EvidenceV2 `json:"evidence"`
	SourceCompleteness string     `json:"source_completeness"`
	Accepted           bool       `json:"accepted"`
	Authority          int        `json:"authority"`
	ReplayAdmitted     bool       `json:"replay_admitted"`
	Partial            bool       `json:"partial"`
	WithheldCount      int        `json:"withheld_count"`
}

func sourceHashPrivateV1(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func BindSourcePrivateV1(ctx context.Context, manager *sessionruntime.Manager, guard *sessionruntime.RetainedScopeGuardPrivate, sessionID string, generation uint64, workspace string) (*BoundSourcePrivateV1, error) {
	if ctx == nil || manager == nil || guard == nil || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return nil, errors.New("private source binding inputs")
	}
	if failure := manager.CheckRetainedScopePrivate(ctx, sessionID, generation, guard); failure != "" {
		return nil, fmt.Errorf("private source scope: %s", failure)
	}
	boundWorkspace, ok := manager.WorkspaceRoot(sessionID, generation)
	if !ok || boundWorkspace != workspace {
		return nil, errors.New("private source workspace differs from bound session")
	}
	p := SourcePolicyPrivateV1{SchemaVersion: VersionSourcePolicyPrivateV1, WorkspaceURI: (&url.URL{Scheme: "file", Path: workspace}).String(), SessionID: sessionID, Generation: generation, Members: []SourceMemberPrivateV1{}}
	expected := map[string][]byte{}
	for _, m := range guard.MembersPrivate() {
		if len(m.Expected) > MaxFileBytes {
			return nil, errors.New("private source member bytes")
		}
		p.Members = append(p.Members, SourceMemberPrivateV1{m.URI, sourceHashPrivateV1(m.Expected), string(m.Role)})
		expected[m.URI] = append([]byte(nil), m.Expected...)
	}
	sort.Slice(p.Members, func(i, j int) bool { return p.Members[i].URI < p.Members[j].URI })
	if err := p.validate(); err != nil {
		return nil, err
	}
	bound := &BoundSourcePrivateV1{manager: manager, guard: guard, policy: p, expected: expected}
	bound.self = bound
	return bound, nil
}
func (p SourcePolicyPrivateV1) validate() error {
	u, err := url.Parse(p.WorkspaceURI)
	if err != nil || u.Scheme != "file" || u.Host != "" || !filepath.IsAbs(u.Path) || filepath.Clean(u.Path) != u.Path || (&url.URL{Scheme: "file", Path: u.Path}).String() != p.WorkspaceURI || p.SchemaVersion != VersionSourcePolicyPrivateV1 || p.SessionID == "" || len(p.SessionID) > 1024 || len(p.WorkspaceURI) > 8192 || p.Generation == 0 || len(p.Members) != 6 {
		return errors.New("private six-source policy identity")
	}
	nominations := 0
	for i, m := range p.Members {
		_, disposition := RelativeURI(p.WorkspaceURI, m.URI)
		if disposition != "" || len(m.URI) > 8192 || (i > 0 && p.Members[i-1].URI >= m.URI) || !validSourceHashPrivateV1(m.SHA256) {
			return errors.New("private source policy canonical member/hash")
		}
		switch m.Role {
		case "NOMINATION":
			nominations++
		case "CONTEXT_ONLY":
		default:
			return errors.New("private source policy role")
		}
	}
	if nominations != 4 {
		return errors.New("private source policy nomination count")
	}
	return nil
}
func validSourceHashPrivateV1(s string) bool {
	if len(s) != 71 || s[:7] != "sha256:" {
		return false
	}
	for _, c := range s[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (p SourcePolicyPrivateV1) member(uri string) *SourceMemberPrivateV1 {
	for i := range p.Members {
		if p.Members[i].URI == uri {
			return &p.Members[i]
		}
	}
	return nil
}
func (p SourcePolicyPrivateV1) withholds(workspace, uri string) bool {
	_, s := RelativeURI(workspace, uri)
	return s == "" && p.member(uri) == nil
}

// No source canonical receipt is fabricated: this ID hashes metadata only.
func withheldPrivateV1(workspace, uri string) Receipt {
	name, _ := RelativeURI(workspace, uri)
	r := Receipt{URI: uri, Path: name, Classification: PostTraversal, AnalyzedVersion: Unverified, Status: PolicyWithheldPrivateV1}
	raw, _ := json.Marshal(r)
	r.ID = digest(VersionSourceWithholdingPrivateV1+":withholding", raw)
	return r
}
func validWithheldPrivateV1(workspace string, r Receipt) bool {
	return reflect.DeepEqual(r, withheldPrivateV1(workspace, r.URI))
}

// ParseSourceWithholdingPrivateV1 is the sole wire dispatch for this private
// profile. Preflight precedes recursive typed allocation. Decoded data never
// grants a source capability. Canonical re-encoding enforces all required keys.
func ParseSourceWithholdingPrivateV1(raw []byte) (SourceWithholdingPrivateV1, error) {
	var e SourceWithholdingPrivateV1
	if err := preflightV2(raw, MaxSourceWithholdingPrivateV1); err != nil {
		return e, err
	}
	if err := preflightSourceArraysPrivateV1(raw); err != nil {
		return e, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return SourceWithholdingPrivateV1{}, err
	}
	canonical, err := json.Marshal(e)
	if err != nil {
		return SourceWithholdingPrivateV1{}, err
	}
	left, le := canonicalV2(raw)
	right, re := canonicalV2(canonical)
	if le != nil || re != nil || !bytes.Equal(left, right) {
		return SourceWithholdingPrivateV1{}, errors.New("private source missing/noncanonical fields")
	}
	if err = validateSourceWithholdingPrivateV1(e); err != nil {
		return SourceWithholdingPrivateV1{}, err
	}
	return e, nil
}

// Bound arrays before typed slice allocation. Opaque LSP data remains raw JSON,
// just as in the historical preflight; its shape is not a policy/source carrier.
func preflightSourceArraysPrivateV1(raw []byte) error {
	type frame struct {
		object, key bool
		name        string
		count       int
	}
	stack := []frame{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	for {
		token, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok && (delim == '}' || delim == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		name := ""
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.object && top.key {
				top.name = token.(string)
				top.key = false
				if top.name == "data" {
					var opaque json.RawMessage
					if err := d.Decode(&opaque); err != nil {
						return err
					}
					top.key = true
				}
				continue
			}
			if top.object {
				name = top.name
				top.key = true
			} else {
				top.count++
				bound := MaxRecordsV2
				if top.name == "members" {
					bound = 6
				}
				if top.count > bound {
					return errors.New("private source array allocation bound")
				}
			}
		}
		if delim, ok := token.(json.Delim); ok && (delim == '{' || delim == '[') {
			stack = append(stack, frame{object: delim == '{', key: delim == '{', name: name})
		}
	}
}

func validateSourceWithholdingPrivateV1(e SourceWithholdingPrivateV1) error {
	if e.SchemaVersion != VersionSourceWithholdingPrivateV1 || e.SourceCompleteness != "UNKNOWN" || e.Accepted || e.Authority != 0 || e.ReplayAdmitted || !e.Partial {
		return errors.New("private source profile ceiling")
	}
	if err := e.Policy.validate(); err != nil {
		return err
	}
	s := e.Evidence
	c := s.Acquisition.Request.Context
	if s.WorkspaceURI != e.Policy.WorkspaceURI || c.SessionID != e.Policy.SessionID || c.Generation != e.Policy.Generation {
		return errors.New("private source policy context")
	}
	if err := validateV2Profile(s, &e.Policy); err != nil {
		return err
	}
	withheld := 0
	for _, r := range s.Captures {
		if e.Policy.withholds(s.WorkspaceURI, r.URI) {
			withheld++
			continue
		}
		if m := e.Policy.member(r.URI); m != nil && r.Status == "READABLE" && sourceHashPrivateV1(r.Content) != m.SHA256 {
			return errors.New("private source captured content drift")
		}
	}
	// Supplied bytes outside the read policy would contradict this profile even
	// though the old V2 supply validator can represent them.
	for _, r := range s.Supplies {
		if r.Receipt != nil {
			m := e.Policy.member(r.Receipt.URI)
			if m == nil || sourceHashPrivateV1(r.Receipt.Content) != m.SHA256 {
				return errors.New("private source supply outside policy/drift")
			}
		}
	}
	if withheld != e.WithheldCount {
		return errors.New("private source withholding count")
	}
	return nil
}

func CaptureSourceWithholdingPrivateV1(ctx context.Context, r acquisition.Result, b *BoundSourcePrivateV1) ([]byte, error) {
	return captureBoundSourceWithholdingPrivateV1(ctx, r, b, readSourcePrivateV1)
}

// Package-local read injection observes refusal-before-read in tests; callers
// cannot substitute a reader or mint a capability through the public API.
func captureBoundSourceWithholdingPrivateV1(ctx context.Context, r acquisition.Result, b *BoundSourcePrivateV1, read func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error)) ([]byte, error) {
	if ctx == nil || b == nil || b.self != b || b.manager == nil || b.guard == nil {
		return nil, errors.New("private source bound capability required")
	}
	if failure := b.manager.CheckRetainedScopePrivate(ctx, b.policy.SessionID, b.policy.Generation, b.guard); failure != "" {
		return nil, fmt.Errorf("private source capability expired: %s", failure)
	}
	return captureSourceWithholdingPrivateV1(ctx, r, b.policy, b.expected, read)
}

// Read injection is package-local testing only; public callers cannot substitute
// a policy predicate, a byte supplier, or arbitrary six-file expectations.
func captureSourceWithholdingPrivateV1(ctx context.Context, r acquisition.Result, p SourcePolicyPrivateV1, expected map[string][]byte, read func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error)) (out []byte, resultErr error) {
	if ctx == nil {
		return nil, errors.New("private source context required")
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	if len(expected) != 6 {
		return nil, errors.New("private expected source count")
	}
	for _, m := range p.Members {
		b, ok := expected[m.URI]
		if !ok || len(b) > MaxFileBytes || sourceHashPrivateV1(b) != m.SHA256 {
			return nil, errors.New("private expected source binding")
		}
	}
	raw, err := json.Marshal(r.Graph)
	if err != nil {
		return nil, err
	}
	s := EvidenceV2{SchemaVersion: VersionSourceWithholdingPrivateV1, Policy: PolicySourceWithholdingPrivateV1, GraphBytes: raw, GraphDigest: digest(VersionV2+":graph", raw), WorkspaceURI: p.WorkspaceURI, AnalyzedVersion: Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", Acquisition: r, Captures: []Receipt{}, Supplies: []SupplyReceiptV2{}, Bindings: []BindingV2{}}
	if err = checkResultV2(s); err != nil {
		return nil, err
	}
	if r.Request.Context.SessionID != p.SessionID || r.Request.Context.Generation != p.Generation {
		return nil, errors.New("private source acquisition context differs from policy")
	}
	if s.Bindings, err = CensusV2(r); err != nil {
		return nil, err
	}
	if s.Supplies, err = suppliesV2(r, s.WorkspaceURI); err != nil {
		return nil, err
	}
	rootURI, _ := url.Parse(p.WorkspaceURI)
	root, rootErr := openSourceRootPrivateV1(ctx, rootURI.Path)
	if errors.Is(rootErr, errSourcePrivateUnsupportedV1) {
		return nil, rootErr
	}
	s.CaptureBudget.RootStatus = "UNAVAILABLE"
	if rootErr == nil {
		s.CaptureBudget.RootStatus = "OPENED"
		defer func() {
			resultErr = errors.Join(resultErr, root.Close())
			if resultErr != nil {
				out = nil
			}
		}()
	}
	used, attempts, withheld := 0, 0, 0
	for _, uri := range sourceURIsV2(s.Bindings) {
		name, status := RelativeURI(s.WorkspaceURI, uri)
		if p.withholds(s.WorkspaceURI, uri) {
			s.Captures = append(s.Captures, withheldPrivateV1(s.WorkspaceURI, uri))
			withheld++
			continue
		}
		var content []byte
		if status == "" {
			switch {
			case ctx.Err() != nil:
				status = "CANCELLED"
			case attempts >= MaxFiles || used >= MaxTotalBytes:
				status = "BUDGET_EXCEEDED"
			case rootErr != nil:
				status = "UNREADABLE"
			default:
				attempts++
				limit := min(MaxFileBytes, MaxTotalBytes-used)
				content, err = read(ctx, root, name, int64(limit))
				if err != nil {
					used += limit
					status = "UNREADABLE"
					switch {
					case errors.Is(err, os.ErrNotExist):
						status = "MISSING"
					case err.Error() == "input must be a regular file":
						status = "NONREGULAR"
					case err.Error() == "input exceeds byte limit":
						status = "BYTE_LIMIT_EXCEEDED"
					}
				} else {
					used += len(content)
					status = "READABLE"
					if !bytes.Equal(content, expected[uri]) {
						return nil, errors.New("private source expected bytes changed")
					}
				}
			}
		}
		s.Captures = append(s.Captures, receiptV2(uri, name, PostTraversal, status, content, nil))
	}
	s.CaptureBudget.Attempts, s.CaptureBudget.ChargedBytes = attempts, used
	if err = bindV2(&s); err != nil {
		return nil, err
	}
	e := SourceWithholdingPrivateV1{SchemaVersion: VersionSourceWithholdingPrivateV1, Policy: p, Evidence: s, SourceCompleteness: "UNKNOWN", Partial: true, WithheldCount: withheld}
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if _, err = ParseSourceWithholdingPrivateV1(encoded); err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
