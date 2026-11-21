package provisionalfeaturecatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"

	"lsp-trace/internal/describerequest"
	tp "lsp-trace/internal/targetpacket"
)

var ErrNativeCatalogV3 = errors.New("native catalog v3 refused")

// Caller cost is not native evidence eligibility.
type NativeBudgetV3 = *CallerBudgetV3
type NativeSelectedRowV3 struct {
	RowPosition, SlotOrdinal   int
	Mode, RunID, SeedNamespace string
	Seed                       uint64
	Intent                     tp.PrivateSourceIntent
}
type SelectedEvidenceV3 struct {
	ID, HandoffID, CheckpointID, OriginalInputID, Interpreter, Origin, Execution string
	Selected                                                                     NativeSelectedRowV3
	Unit                                                                         tp.PrivateSourceUnit
	Request                                                                      describerequest.Record
	Record                                                                       json.RawMessage
	RawAdapterDigest                                                             string
}

// censuscontinuation already imports this package. The interface avoids a
// reverse import, but is NOT itself admission: exact concrete Go named type
// identity is checked before dispatch. DTOs, IDs and arbitrary implementations
// cannot reconstruct sourcecontinuation's unexported opaque state.
type NativeBoundDescriptionV3 interface {
	Evidence(context.Context) (SelectedEvidenceV3, error)
}

func genuineNativeBoundV3(v NativeBoundDescriptionV3) bool {
	t := reflect.TypeOf(v)
	return t != nil && t.Kind() == reflect.Struct && t.PkgPath() == "lsp-trace/internal/inventorysuccessor/sourcecontinuation" && t.Name() == "BoundSelectedDescriptionV3"
}

type NativeGroupV3 struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Reason  string   `json:"reason"`
	Members []string `json:"members"`
}
type NativeDecisionV3 struct {
	Predecessor string          `json:"predecessor"`
	Actor       string          `json:"actor"`
	Reason      string          `json:"reason"`
	Affected    []string        `json:"affected"`
	Groups      []NativeGroupV3 `json:"groups"`
}
type NativeCatalogWireV3 struct {
	Version           string               `json:"version"`
	Revision          string               `json:"revision"`
	Predecessor       string               `json:"predecessor"`
	Authority         int                  `json:"authority"`
	Accepted          bool                 `json:"accepted"`
	Status            string               `json:"status"`
	Feature           string               `json:"feature"`
	Consumer          string               `json:"consumer"`
	Completeness      string               `json:"completeness"`
	Absence           string               `json:"absence"`
	Origin            string               `json:"origin"`
	Actor             string               `json:"actor"`
	ReviewIdentity    string               `json:"review_identity"`
	Reason            string               `json:"reason"`
	Affected          []string             `json:"affected"`
	Evidence          []SelectedEvidenceV3 `json:"evidence"`
	Groups            []NativeGroupV3      `json:"groups"`
	WithMembership    int                  `json:"with_membership"`
	WithoutMembership int                  `json:"without_membership"`
	ReviewMode        ReviewModeV3         `json:"review_mode,omitempty"`
}
type NativeCatalogV3 struct{ state *nativeCatalogStateV3 }
type nativeCatalogStateV3 struct {
	wire        NativeCatalogWireV3
	raw         []byte
	budget      NativeBudgetV3
	interpreter string
	corrected   bool
	mu          sync.Mutex
	successor   bool
}

func NativeDigestV3(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

// DecodeNativeV3 rejects duplicate keys, unknown fields, null top-level values
// and trailing data. Callers must reserve input/parse costs before entering.
func DecodeNativeV3(raw []byte, v any) error {
	if len(raw) == 0 || len(raw) > 8<<20 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ErrNativeCatalogV3
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	var visit func() error
	visit = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return ErrNativeCatalogV3
				}
				seen[s] = true
				if e = visit(); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = visit(); e != nil {
					return e
				}
			}
		default:
			return ErrNativeCatalogV3
		}
		_, e = d.Token()
		return e
	}
	if e := visit(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return ErrNativeCatalogV3
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	return nil
}
func BuildNativeV3(ctx context.Context, budget NativeBudgetV3, actor string, descriptions []NativeBoundDescriptionV3, reviewModes ...ReviewModeV3) (NativeCatalogV3, error) {
	return buildNativeWithSealV3(ctx, budget, actor, descriptions, sealNativeV3, reviewModes...)
}

func buildNativeWithSealV3(ctx context.Context, budget NativeBudgetV3, actor string, descriptions []NativeBoundDescriptionV3, seal nativeSealerV3, reviewModes ...ReviewModeV3) (NativeCatalogV3, error) {
	var mode ReviewModeV3
	if len(reviewModes) == 1 {
		mode = reviewModes[0]
	}
	if len(reviewModes) > 1 || !mode.Valid() {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	if len(actor) == 0 || len(actor) > 256 || len(descriptions) > 100 || budget.Reserve(ctx, uint64(len(descriptions))*4096+4096) != nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	w := NativeCatalogWireV3{Version: "lsp-trace.native-catalog.v3", Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Origin: "SUPPLIED_UNAUTHENTICATED", Actor: actor, ReviewIdentity: "NOT_AUTHENTICATED", Evidence: []SelectedEvidenceV3{}, Groups: []NativeGroupV3{}, Affected: []string{}}
	w.ReviewMode = mode
	seen := map[string]bool{}
	for _, b := range descriptions {
		if !genuineNativeBoundV3(b) {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		e, err := b.Evidence(ctx)
		if err != nil || e.ID == "" || seen[e.ID] || e.Interpreter != actor {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		seen[e.ID] = true
		w.Evidence = append(w.Evidence, e)
	}
	sort.Slice(w.Evidence, func(i, j int) bool { return w.Evidence[i].ID < w.Evidence[j].ID })
	w.WithoutMembership = len(w.Evidence)
	return seal(ctx, budget, w, actor, false)
}
func nativeSealQuoteV3(w NativeCatalogWireV3) uint64 {
	var n uint64 = 16384
	for _, e := range w.Evidence {
		n += uint64(len(e.Unit.Body)+len(e.Request.Envelope.Prompt)+len(e.Record))*24 + 32768
	}
	for _, g := range w.Groups {
		n += uint64(len(g.ID)+len(g.Label)+len(g.Reason))*12 + uint64(len(g.Members))*1024
	}
	return n
}
func sealNativeV3(ctx context.Context, b NativeBudgetV3, w NativeCatalogWireV3, actor string, corrected bool) (NativeCatalogV3, error) {
	// Reserve escaping, identity encoding, retained canonical and return copy before
	// marshal. Each version retains original V2 bytes; no legacy packet synthesis.
	if b.Reserve(ctx, nativeSealQuoteV3(w)) != nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	w.Revision = ""
	raw, e := json.Marshal(w)
	if e != nil {
		return NativeCatalogV3{}, e
	}
	w.Revision = NativeDigestV3(raw)
	raw, e = json.Marshal(w)
	if e != nil || len(raw) > 8<<20 {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	return NativeCatalogV3{&nativeCatalogStateV3{wire: w, raw: raw, budget: b, interpreter: actor, corrected: corrected}}, nil
}
func (c NativeCatalogV3) Revision() string {
	if c.state == nil {
		return ""
	}
	return c.state.wire.Revision
}
func (c NativeCatalogV3) Bytes(ctx context.Context) ([]byte, error) {
	if c.state == nil || c.state.budget.Reserve(ctx, uint64(len(c.state.raw))+1) != nil {
		return nil, ErrNativeCatalogV3
	}
	return append([]byte(nil), c.state.raw...), nil
}
func (c NativeCatalogV3) MembershipCounts() (int, int) {
	if c.state == nil {
		return 0, 0
	}
	return c.state.wire.WithMembership, c.state.wire.WithoutMembership
}

// ApplyNativeDecisionV3 replaces a proposal's group view, not its evidence. Every
// member is an eligible opaque-bound description. Text search never calls this.
func ApplyNativeDecisionV3(ctx context.Context, c NativeCatalogV3, raw []byte, review bool) (NativeCatalogV3, error) {
	return applyNativeDecisionWithSeal(ctx, c, raw, review, sealNativeV3)
}

// The sealing seam is package-private: callers cannot choose a constructor for
// a publicly minted capability. All transitions share the guards and mutex below.
type nativeSealerV3 func(context.Context, NativeBudgetV3, NativeCatalogWireV3, string, bool) (NativeCatalogV3, error)

func applyNativeDecisionWithSeal(ctx context.Context, c NativeCatalogV3, raw []byte, review bool, seal nativeSealerV3) (NativeCatalogV3, error) {
	if c.state == nil || len(raw) > 8<<20 || c.state.budget.Reserve(ctx, uint64(len(raw))*16+16384) != nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.successor {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	var d NativeDecisionV3
	if DecodeNativeV3(raw, &d) != nil || d.Predecessor != c.Revision() || len(d.Actor) == 0 || len(d.Actor) > 256 || len(d.Reason) == 0 || len(d.Reason) > 4096 || len(d.Groups) > 100 || len(d.Affected) > 100 || d.Groups == nil || d.Affected == nil {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	if review {
		// Same-thread review is still a separate successor of a proposal. It
		// cannot relabel the base as reviewed or consume a correction twice.
		if (d.Actor == s.interpreter && !s.wire.ReviewMode.AllowsSameActor()) || s.corrected || (s.wire.ReviewMode.AllowsSameActor() && s.wire.Predecessor == "") {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
	} else if d.Actor != s.interpreter || s.corrected || s.wire.Predecessor != "" {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	eligible := map[string]bool{}
	for _, e := range s.wire.Evidence {
		eligible[e.ID] = true
	}
	ids, members := map[string]bool{}, map[string]bool{}
	for _, g := range d.Groups {
		if len(g.ID) == 0 || len(g.ID) > 256 || ids[g.ID] || len(g.Label) == 0 || len(g.Label) > 4096 || len(g.Reason) == 0 || len(g.Reason) > 4096 || len(g.Members) == 0 || len(g.Members) > 100 {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		ids[g.ID] = true
		for _, m := range g.Members {
			if !eligible[m] || members[m] {
				return NativeCatalogV3{}, ErrNativeCatalogV3
			}
			members[m] = true
		}
		sort.Strings(g.Members)
	}
	affected := map[string]bool{}
	old := map[string]NativeGroupV3{}
	for _, g := range s.wire.Groups {
		old[g.ID] = g
	}
	for _, id := range d.Affected {
		if affected[id] {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		if _, ok := old[id]; !ok {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
		affected[id] = true
	}
	// Exact changed predecessor groups must be named; immutable predecessor is
	// never edited. New merged groups may have new IDs but only eligible members.
	next := map[string]NativeGroupV3{}
	for _, g := range d.Groups {
		next[g.ID] = g
	}
	for id, g := range old {
		a, _ := json.Marshal(g)
		v, ok := next[id]
		z, _ := json.Marshal(v)
		changed := !ok || !bytes.Equal(a, z)
		if changed != affected[id] {
			return NativeCatalogV3{}, ErrNativeCatalogV3
		}
	}
	if !review && len(d.Affected) != 0 {
		return NativeCatalogV3{}, ErrNativeCatalogV3
	}
	w := s.wire
	w.Predecessor = c.Revision()
	w.Actor = d.Actor
	w.Reason = d.Reason
	w.Affected = d.Affected
	w.Groups = d.Groups
	w.WithMembership = len(members)
	w.WithoutMembership = len(eligible) - len(members)
	sort.Slice(w.Groups, func(i, j int) bool { return w.Groups[i].ID < w.Groups[j].ID })
	result, e := seal(ctx, s.budget, w, s.interpreter, review)
	if e == nil {
		s.successor = true
	}
	return result, e
}

// SearchGroups is bounded deterministic descriptive matching, not membership,
// acceptance, completeness, or a semantic similarity/identity judgment.
func (c NativeCatalogV3) SearchGroups(ctx context.Context, query string, limit int) ([]NativeGroupV3, error) {
	if c.state == nil || len(query) > 256 || limit < 1 || limit > 100 || c.state.budget.Reserve(ctx, uint64(len(c.state.raw))*2+4096) != nil {
		return nil, ErrNativeCatalogV3
	}
	out := []NativeGroupV3{}
	q := strings.ToLower(query)
	for _, g := range c.state.wire.Groups {
		if strings.Contains(strings.ToLower(g.Label), q) {
			g.Members = append([]string(nil), g.Members...)
			out = append(out, g)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
