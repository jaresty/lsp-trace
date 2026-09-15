package transientstructuraldelta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"sort"

	"lsp-trace/internal/strictjson"
	tsr "lsp-trace/internal/transientstructuralresult"
)

const SchemaVersion = "lsp-trace.transient-structural-delta.v1"

type Code string

const (
	CodeInvalidInput             Code = "INVALID_INPUT"
	CodeAmbiguousSymbolKey       Code = "AMBIGUOUS_SYMBOL_KEY"
	CodeTargetMismatch           Code = "TARGET_MISMATCH"
	CodePositionEncodingMismatch Code = "POSITION_ENCODING_MISMATCH"
)

type DomainError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *DomainError) Error() string { return string(e.Code) + ": " + e.Message }

type Input struct {
	Before tsr.LocatorResultV2 `json:"before"`
	After  tsr.LocatorResultV2 `json:"after"`
}
type SymbolKey struct {
	Path string `json:"path"`
	Kind int    `json:"kind"`
	Name string `json:"name"`
}
type CallKey struct {
	Caller SymbolKey `json:"caller"`
	Callee SymbolKey `json:"callee"`
	Path   string    `json:"path"`
}
type CountChange struct {
	Call   CallKey `json:"call"`
	Before int     `json:"before"`
	After  int     `json:"after"`
}
type CallDelta struct {
	Added        []CallCount   `json:"added"`
	Removed      []CallCount   `json:"removed"`
	CountChanged []CountChange `json:"count_changed"`
}
type CallCount struct {
	Call  CallKey `json:"call"`
	Count int     `json:"count"`
}
type IntTransition struct {
	Before int `json:"before"`
	After  int `json:"after"`
}
type FloatTransition struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}
type BoolTransition struct {
	Before bool `json:"before"`
	After  bool `json:"after"`
}
type AnalyticsDelta struct {
	Symbol        SymbolKey       `json:"symbol"`
	Ca            IntTransition   `json:"ca"`
	Ce            IntTransition   `json:"ce"`
	Instability   FloatTransition `json:"instability"`
	Cyclic        BoolTransition  `json:"cyclic"`
	Articulation  BoolTransition  `json:"articulation"`
	PageRank      FloatTransition `json:"pagerank"`
	HITSHub       FloatTransition `json:"hits_hub"`
	HITSAuthority FloatTransition `json:"hits_authority"`
}
type BridgeKey struct {
	A SymbolKey `json:"a"`
	B SymbolKey `json:"b"`
}
type Qualification struct {
	Authority                  int      `json:"authority"`
	SourceGraphComplete        string   `json:"source_graph_complete"`
	ComparisonScope            string   `json:"comparison_scope"`
	AcquisitionScopeComparable string   `json:"acquisition_scope_comparable"`
	NodeUniverseEqual          bool     `json:"node_universe_equal"`
	AnalyticsPoliciesEqual     bool     `json:"analytics_policies_equal"`
	ScopeSensitive             []string `json:"scope_sensitive"`
}
type Result struct {
	SchemaVersion      string           `json:"schema_version"`
	Qualification      Qualification    `json:"qualification"`
	AddedSymbols       []SymbolKey      `json:"added_symbols"`
	RemovedSymbols     []SymbolKey      `json:"removed_symbols"`
	MatchedSymbols     []SymbolKey      `json:"matched_symbols"`
	Calls              CallDelta        `json:"calls"`
	Analytics          []AnalyticsDelta `json:"analytics"`
	AddedWeakBridges   []BridgeKey      `json:"added_weak_bridges"`
	RemovedWeakBridges []BridgeKey      `json:"removed_weak_bridges"`
	AffectedCallers    []SymbolKey      `json:"affected_callers"`
}

func Decode(raw []byte) (Input, error) {
	if len(raw) == 0 || len(raw) > 2*1024*1024 || strictjson.RejectDuplicates(raw) != nil {
		return Input{}, &DomainError{CodeInvalidInput, "input must be strict bounded JSON"}
	}
	var in Input
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return Input{}, &DomainError{CodeInvalidInput, err.Error()}
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Input{}, &DomainError{CodeInvalidInput, "trailing JSON"}
	}
	if err := tsr.ValidateV2(in.Before); err != nil {
		return Input{}, &DomainError{CodeInvalidInput, "before: " + err.Error()}
	}
	if err := tsr.ValidateV2(in.After); err != nil {
		return Input{}, &DomainError{CodeInvalidInput, "after: " + err.Error()}
	}
	return in, nil
}
func Compare(in Input) (Result, error) {
	bm, bi, err := index(in.Before)
	if err != nil {
		return Result{}, err
	}
	am, ai, err := index(in.After)
	if err != nil {
		return Result{}, err
	}
	if err := tsr.ValidateV2(in.Before); err != nil {
		return Result{}, &DomainError{CodeInvalidInput, "before: " + err.Error()}
	}
	if err := tsr.ValidateV2(in.After); err != nil {
		return Result{}, &DomainError{CodeInvalidInput, "after: " + err.Error()}
	}
	if in.Before.PositionEncoding != in.After.PositionEncoding {
		return Result{}, &DomainError{CodePositionEncodingMismatch, "position encodings differ"}
	}
	if err := validateRelations(in.Before, bi); err != nil {
		return Result{}, err
	}
	if err := validateRelations(in.After, ai); err != nil {
		return Result{}, err
	}
	if bi[in.Before.TargetID] != ai[in.After.TargetID] {
		return Result{}, &DomainError{CodeTargetMismatch, "canonical targets differ"}
	}
	r := Result{SchemaVersion: SchemaVersion, Qualification: Qualification{0, "UNKNOWN", "TWO_BOUNDED_LOCAL_RESULTS", "UNKNOWN", sameKeys(bm, am), policiesEqual(in.Before, in.After), []string{"HITS", "PAGERANK"}}}
	for k := range bm {
		if _, ok := am[k]; ok {
			r.MatchedSymbols = append(r.MatchedSymbols, k)
		} else {
			r.RemovedSymbols = append(r.RemovedSymbols, k)
		}
	}
	for k := range am {
		if _, ok := bm[k]; !ok {
			r.AddedSymbols = append(r.AddedSymbols, k)
		}
	}
	sortSymbols(r.AddedSymbols)
	sortSymbols(r.RemovedSymbols)
	sortSymbols(r.MatchedSymbols)
	bc := calls(in.Before, bi)
	ac := calls(in.After, ai)
	changedCallers := map[SymbolKey]bool{}
	for k, n := range bc {
		m := ac[k]
		if m == 0 {
			r.Calls.Removed = append(r.Calls.Removed, CallCount{k, n})
			changedCallers[k.Caller] = true
		} else if m != n {
			r.Calls.CountChanged = append(r.Calls.CountChanged, CountChange{k, n, m})
			changedCallers[k.Caller] = true
		}
	}
	for k, n := range ac {
		if bc[k] == 0 {
			r.Calls.Added = append(r.Calls.Added, CallCount{k, n})
			changedCallers[k.Caller] = true
		}
	}
	for _, k := range r.AddedSymbols {
		for c := range ac {
			if c.Callee == k {
				changedCallers[c.Caller] = true
			}
		}
	}
	for _, k := range r.RemovedSymbols {
		for c := range bc {
			if c.Callee == k {
				changedCallers[c.Caller] = true
			}
		}
	}
	sort.Slice(r.Calls.Added, func(i, j int) bool { return callLess(r.Calls.Added[i].Call, r.Calls.Added[j].Call) })
	sort.Slice(r.Calls.Removed, func(i, j int) bool { return callLess(r.Calls.Removed[i].Call, r.Calls.Removed[j].Call) })
	sort.Slice(r.Calls.CountChanged, func(i, j int) bool { return callLess(r.Calls.CountChanged[i].Call, r.Calls.CountChanged[j].Call) })
	ba := analytics(in.Before, bi)
	aa := analytics(in.After, ai)
	for _, k := range r.MatchedSymbols {
		x, y := ba[k], aa[k]
		if x != y {
			r.Analytics = append(r.Analytics, AnalyticsDelta{k, IntTransition{x.ca, y.ca}, IntTransition{x.ce, y.ce}, FloatTransition{x.in, y.in}, BoolTransition{x.cyc, y.cyc}, BoolTransition{x.art, y.art}, FloatTransition{x.pr, y.pr}, FloatTransition{x.hub, y.hub}, FloatTransition{x.auth, y.auth}})
		}
	}
	bb := bridges(in.Before, bi)
	ab := bridges(in.After, ai)
	for k := range bb {
		if !ab[k] {
			r.RemovedWeakBridges = append(r.RemovedWeakBridges, k)
		}
	}
	for k := range ab {
		if !bb[k] {
			r.AddedWeakBridges = append(r.AddedWeakBridges, k)
		}
	}
	sortBridges(r.AddedWeakBridges)
	sortBridges(r.RemovedWeakBridges)
	for k := range changedCallers {
		r.AffectedCallers = append(r.AffectedCallers, k)
	}
	sortSymbols(r.AffectedCallers)
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	return r, nil
}
func Execute(raw []byte) (Result, error) {
	in, e := Decode(raw)
	if e != nil {
		return Result{}, e
	}
	return Compare(in)
}
func key(n tsr.LocatorNodeV2) SymbolKey { return SymbolKey{n.Path, n.Kind, n.Name} }
func validPath(p string) bool {
	return p != "" && !path.IsAbs(p) && path.Clean(p) == p && p != "." && p != ".." && len(p) < 4096 && !(len(p) >= 3 && p[:3] == "../")
}
func index(r tsr.LocatorResultV2) (map[SymbolKey]string, map[string]SymbolKey, error) {
	m := map[SymbolKey]string{}
	ids := map[string]SymbolKey{}
	for _, n := range r.Nodes {
		k := key(n)
		if !validPath(k.Path) {
			return nil, nil, &DomainError{CodeInvalidInput, "invalid relative path"}
		}
		if _, ok := m[k]; ok {
			return nil, nil, &DomainError{CodeAmbiguousSymbolKey, fmt.Sprintf("duplicate canonical symbol key: %v", k)}
		}
		if _, ok := ids[n.ID]; ok {
			return nil, nil, &DomainError{CodeInvalidInput, "duplicate node ID"}
		}
		m[k] = n.ID
		ids[n.ID] = k
	}
	return m, ids, nil
}
func validateRelations(r tsr.LocatorResultV2, ids map[string]SymbolKey) error {
	for _, c := range r.Calls {
		if _, ok := ids[c.CallerID]; !ok {
			return &DomainError{CodeInvalidInput, "call caller absent"}
		}
		if _, ok := ids[c.CalleeID]; !ok {
			return &DomainError{CodeInvalidInput, "call callee absent"}
		}
		if !validPath(c.Path) {
			return &DomainError{CodeInvalidInput, "invalid call-site relative path"}
		}
	}
	return nil
}

func sameKeys(a, b map[SymbolKey]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
func policiesEqual(a, b tsr.LocatorResultV2) bool {
	return a.AnalyticsScope == b.AnalyticsScope && a.WeakProjection == b.WeakProjection && a.PageRankDamping == b.PageRankDamping && a.AnalyticsTolerance == b.AnalyticsTolerance
}
func calls(r tsr.LocatorResultV2, ids map[string]SymbolKey) map[CallKey]int {
	m := map[CallKey]int{}
	for _, c := range r.Calls {
		m[CallKey{ids[c.CallerID], ids[c.CalleeID], c.Path}]++
	}
	return m
}

type av struct {
	ca, ce        int
	in            float64
	cyc, art      bool
	pr, hub, auth float64
}

func analytics(r tsr.LocatorResultV2, ids map[string]SymbolKey) map[SymbolKey]av {
	m := map[SymbolKey]av{}
	for _, x := range r.Coupling {
		v := m[ids[x.NodeID]]
		v.ca = x.Ca
		v.ce = x.Ce
		v.in = x.Instability
		m[ids[x.NodeID]] = v
	}
	for _, c := range r.StrongComponents {
		if c.Cyclic {
			for _, id := range c.Nodes {
				v := m[ids[id]]
				v.cyc = true
				m[ids[id]] = v
			}
		}
	}
	for _, id := range r.ArticulationPoints {
		v := m[ids[id]]
		v.art = true
		m[ids[id]] = v
	}
	for _, x := range r.PageRank {
		v := m[ids[x.NodeID]]
		v.pr = x.Score
		m[ids[x.NodeID]] = v
	}
	for _, x := range r.HITS {
		v := m[ids[x.NodeID]]
		v.hub = x.Hub
		v.auth = x.Authority
		m[ids[x.NodeID]] = v
	}
	return m
}
func bridges(r tsr.LocatorResultV2, ids map[string]SymbolKey) map[BridgeKey]bool {
	m := map[BridgeKey]bool{}
	for _, b := range r.WeakBridges {
		a, z := ids[b.NodeA], ids[b.NodeB]
		if symLess(z, a) {
			a, z = z, a
		}
		m[BridgeKey{a, z}] = true
	}
	return m
}
func symLess(a, b SymbolKey) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.Name < b.Name
}
func sortSymbols(x []SymbolKey) { sort.Slice(x, func(i, j int) bool { return symLess(x[i], x[j]) }) }
func callLess(a, b CallKey) bool {
	if a.Caller != b.Caller {
		return symLess(a.Caller, b.Caller)
	}
	if a.Callee != b.Callee {
		return symLess(a.Callee, b.Callee)
	}
	return a.Path < b.Path
}
func sortBridges(x []BridgeKey) {
	sort.Slice(x, func(i, j int) bool {
		if x[i].A != x[j].A {
			return symLess(x[i].A, x[j].A)
		}
		return symLess(x[i].B, x[j].B)
	})
}
func Validate(r Result) error {
	if r.SchemaVersion != SchemaVersion || r.Qualification.Authority != 0 || r.Qualification.SourceGraphComplete != "UNKNOWN" || r.Qualification.ComparisonScope != "TWO_BOUNDED_LOCAL_RESULTS" || r.Qualification.AcquisitionScopeComparable != "UNKNOWN" {
		return errors.New("invalid qualification")
	}
	for _, a := range r.Analytics {
		for _, v := range []float64{a.Instability.Before, a.Instability.After, a.PageRank.Before, a.PageRank.After, a.HITSHub.Before, a.HITSHub.After, a.HITSAuthority.Before, a.HITSAuthority.After} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return errors.New("non-finite analytics")
			}
		}
	}
	return nil
}
