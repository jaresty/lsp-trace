package acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/retainedpath"
)

const Policy = "ROOTS_FIRST_ORDERED_RESOLUTION_SORTED_ADMISSION_ROUND_ROBIN_BFS_V1"
const MaxPrepareProbes = 65

var errCapture = errors.New("EVIDENCE_BYTES")

// ValidateRequest rejects malformed selectors before making any client calls.
// Zero global count limits are meaningful: all affected work is accounted blocked.
func ValidateRequest(r Request) error {
	if r.Mode != Slice && r.Mode != Incoming {
		return errors.New("mode must be SLICE or INCOMING")
	}
	if r.Context.ID == "" || r.Context.SessionID == "" || r.Context.Generation == 0 {
		return errors.New("exact declared acquisition context required")
	}
	switch r.Context.PositionEncoding {
	case "utf-8", "utf-16", "utf-32":
	default:
		return errors.New("unsupported position encoding")
	}
	l := r.Limits
	if l.MaxNodes < 0 || l.MaxNodes > 10000 || l.MaxRequests < 0 || l.MaxRequests > 100000 || l.MaxEvidenceBytes < 0 || l.MaxEvidenceBytes > 64<<20 || l.MaxPathWork < 0 || l.MaxPathWork > 100000000 || l.Timeout <= 0 || l.RequestTimeout <= 0 || l.MaxResponseBytes < 1 || l.MaxResponseBytes > 16<<20 || l.MaxMessages < 1 || l.MaxMessages > 4096 {
		return errors.New("invalid acquisition limits")
	}
	if r.Root.ID != "root" {
		return errors.New("primary ID must be root")
	}
	if len(r.RequiredTargets) > 63 {
		return errors.New("at most 64 total targets")
	}
	seen := map[string]bool{}
	for _, t := range append([]Target{r.Root}, r.RequiredTargets...) {
		if strings.TrimSpace(t.ID) == "" || seen[t.ID] {
			return errors.New("target IDs must be unique and nonempty")
		}
		seen[t.ID] = true
		p := t.Locator
		if !canonicalURI(p.URI) {
			return fmt.Errorf("noncanonical absolute URI %q", p.URI)
		}
		if (p.Line == nil) != (p.Character == nil) || (p.Line != nil) == (p.Symbol != "") {
			return errors.New("exclusive symbol or complete zero-based position required")
		}
		if t.DownDepth < 0 || t.DownDepth > 64 || t.UpDepth < 0 || t.UpDepth > 64 {
			return errors.New("depth must be between zero and 64")
		}
	}
	return nil
}
func canonicalURI(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || !u.IsAbs() || u.Fragment != "" || u.RawQuery != "" || u.Opaque != "" {
		return false
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if !strings.HasPrefix(u.Path, "/") {
		return false
	}
	u.Path = path.Clean(u.Path)
	u.RawPath = ""
	return u.String() == raw
}

type runner struct {
	ctx         context.Context
	client      Client
	result      Result
	items       map[string]lsp.CallHierarchyItem
	nodes       map[string]graph.Node
	resolutions map[string]Resolution
	queries     map[string]queryResult
	members     []map[string]bool
	relations   []map[string]bool
	replay      *recordReplay // offline validation only; never invokes a client
}
type neighbor struct {
	item lsp.CallHierarchyItem
	edge graph.Edge
}
type queryResult struct {
	status            ExpansionStatus
	reason, requestID string
	neighbors         []neighbor
}

// Acquire performs all resolution and seed admission before any neighborhood
// traversal, then evaluates paths only on the final admitted graph. Provider
// failures are result data; malformed requests and internal invariant failures
// are errors. It neither calls legacy traversals nor registers public operations.
func Acquire(parent context.Context, client Client, request Request) (Result, error) {
	if parent == nil || client == nil {
		return Result{}, errors.New("context and client required")
	}
	if err := ValidateRequest(request); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(parent, request.Limits.Timeout)
	defer cancel()
	c := &runner{ctx: ctx, client: client, result: Result{Request: request, Policy: Policy, PathProjection: "NATIVE_CALLS_CALLER_TO_CALLEE_UNIT_GROUP_HOPS_V1", AcquisitionComplete: true}, items: map[string]lsp.CallHierarchyItem{}, nodes: map[string]graph.Node{}, resolutions: map[string]Resolution{}, queries: map[string]queryResult{}}
	if err := c.acquireEvidence(); err != nil {
		return Result{}, err
	}
	if err := c.connect(); err != nil {
		return Result{}, err
	}
	if err := ValidateResult(c.result); err != nil {
		return Result{}, fmt.Errorf("coordinator invariant: %w", err)
	}
	return c.result, nil
}

// Shared event interpreter: live acquisition invokes a client; validation
// consumes retained normalized request records at the same operation boundary.
// Neither this method nor the record interpreter calls Acquire/ValidateResult.
func (c *runner) acquireEvidence() error {
	request := c.result.Request
	targets := append([]Target{request.Root}, request.RequiredTargets...)
	for _, t := range targets {
		c.result.Targets = append(c.result.Targets, TargetResult{Requested: t, Admission: NotApplicable, Outgoing: DirectionResult{Status: ExpansionNotApplicable}, Incoming: DirectionResult{Status: ExpansionNotApplicable}, Connection: Connection{Status: "NOT_EVALUABLE", Direction: "CALLER_TO_CALLEE", Reason: "endpoint unresolved or not admitted", Path: emptyPath()}})
		c.members = append(c.members, map[string]bool{})
		c.relations = append(c.relations, map[string]bool{})
	}
	for i := range c.result.Targets {
		c.result.Targets[i].Resolution = c.resolve(c.result.Targets[i].Requested)
	}
	// Root first; subsequent unique identities in stable lexical order. Aliases
	// consume no extra node slots but retain distinct requested rows.
	order := make([]int, 0, len(targets))
	order = append(order, 0)
	rest := make([]int, 0, len(targets)-1)
	for i := 1; i < len(targets); i++ {
		rest = append(rest, i)
	}
	sort.SliceStable(rest, func(i, j int) bool {
		a, b := c.result.Targets[rest[i]], c.result.Targets[rest[j]]
		return resolutionID(a.Resolution) < resolutionID(b.Resolution)
	})
	order = append(order, rest...)
	for _, i := range order {
		t := &c.result.Targets[i]
		if t.Resolution.Status != Resolved {
			c.result.AcquisitionComplete = false
			continue
		}
		if c.admit(*t.Resolution.Prepared) {
			t.Admission = Admitted
			c.members[i][t.Resolution.Identity.ID] = true
		} else {
			t.Admission = AdmissionBlocked
			c.result.AcquisitionComplete = false
		}
	}
	if request.TopmostSiblings {
		c.expandTopmostSiblings()
	}
	if request.Mode == Slice {
		c.traverse(Outgoing)
	}
	c.traverse(IncomingDirection)
	return c.finishGraph()
}
func resolutionID(r Resolution) string {
	if r.Identity == nil {
		return ""
	}
	return r.Identity.ID
}
func emptyPath() retainedpath.Path {
	return retainedpath.Path{Nodes: []string{}, GroupIDs: []string{}, OccurrenceIDs: [][]string{}}
}
func cloneItem(i lsp.CallHierarchyItem) lsp.CallHierarchyItem {
	i.Data = append(json.RawMessage(nil), i.Data...)
	i.Tags = append([]int(nil), i.Tags...)
	return i
}

func node(i lsp.CallHierarchyItem) graph.Node {
	return graph.NewNode(graph.Item{Name: i.Name, Kind: i.Kind, Detail: i.Detail, URI: i.URI, Range: toRange(i.Range), SelectionRange: toRange(i.SelectionRange), Data: append(json.RawMessage(nil), i.Data...)})
}
func toRange(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
func less(a, b lsp.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}
func contains(r lsp.Range, p lsp.Position) bool {
	return !less(p, r.Start) && (less(p, r.End) || r.Start == r.End && p == r.Start)
}
func (c *runner) admit(i lsp.CallHierarchyItem) bool {
	n := node(i)
	if _, ok := c.nodes[n.ID]; ok {
		return true
	}
	if len(c.nodes) >= c.result.Request.Limits.MaxNodes {
		return false
	}
	c.nodes[n.ID] = n
	c.items[n.ID] = cloneItem(i)
	c.result.Usage.Nodes++
	return true
}

// boundedJSON retains at most limit bytes even when a client returns too much.
// It does not claim raw transport byte accounting: the budget covers canonical
// request parameters and normalized response/error evidence (see package doc).
type cappedBuffer struct {
	bytes.Buffer
	left int
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > w.left {
		n := w.left
		w.Buffer.Write(p[:n])
		w.left = 0
		return n, errCapture
	}
	n, _ := w.Buffer.Write(p)
	w.left -= n
	return n, nil
}
func boundedJSON(v any, limit int) (json.RawMessage, int, error) {
	w := &cappedBuffer{left: limit}
	e := json.NewEncoder(w).Encode(v)
	if e != nil {
		return nil, w.Len(), e
	}
	return append(json.RawMessage(nil), bytes.TrimSuffix(w.Bytes(), []byte("\n"))...), w.Len(), nil
}
func (c *runner) invoke(target, method, id string, params any, call func(context.Context) (any, error)) (any, *RequestRecord) {
	if c.replay != nil {
		return c.replay.invoke(c, target, method, id, params)
	}
	rec := RequestRecord{ID: fmt.Sprintf("request-%06d", len(c.result.Requests)+1), TargetID: target, Method: method, NodeID: id, Context: c.result.Request.Context, Before: c.result.Usage, Outcome: "BUDGET_BLOCKED"}
	l := c.result.Request.Limits
	blocked := ""
	if err := c.ctx.Err(); err != nil {
		blocked = err.Error()
	} else if c.result.Usage.Requests >= l.MaxRequests {
		blocked = "MAX_REQUESTS"
	} else if c.result.Usage.EvidenceBytes >= l.MaxEvidenceBytes {
		blocked = "EVIDENCE_BYTES"
	} else if method == "textDocument/prepareCallHierarchy" && c.result.Usage.PrepareProbes >= MaxPrepareProbes {
		blocked = "MAX_PREPARE_PROBES"
	}
	if blocked == "" {
		raw, n, e := boundedJSON(params, l.MaxEvidenceBytes-c.result.Usage.EvidenceBytes)
		c.result.Usage.EvidenceBytes += n
		if e != nil {
			blocked = "EVIDENCE_BYTES"
		} else {
			rec.Params = raw
			if c.result.Usage.EvidenceBytes >= l.MaxEvidenceBytes {
				blocked = "EVIDENCE_BYTES"
			}
		}
	}
	var value any
	if blocked != "" {
		rec.Reason = blocked
	} else {
		rec.Attempted = true
		c.result.Usage.Requests++
		if method == "textDocument/prepareCallHierarchy" {
			c.result.Usage.PrepareProbes++
		}
		ctx, cancel := context.WithTimeout(c.ctx, l.RequestTimeout)
		wireLimits := l
		if left := l.MaxEvidenceBytes - c.result.Usage.EvidenceBytes; left < wireLimits.MaxResponseBytes {
			wireLimits.MaxResponseBytes = left
		}
		ctx = context.WithValue(ctx, callScopeKey{}, callScope{context: c.result.Request.Context, limits: wireLimits})
		var err error
		value, err = call(ctx)
		if err == nil {
			err = ctx.Err()
		}
		cancel()
		rec.Outcome = "SUCCESS"
		capture := value
		if err != nil {
			rec.Outcome = "FAILED"
			rec.Reason = err.Error()
			if strings.Contains(rec.Reason, "-32601") {
				rec.Outcome = "UNSUPPORTED"
			}
			capture = struct {
				Error string `json:"error"`
			}{rec.Reason}
		}
		left := l.MaxEvidenceBytes - c.result.Usage.EvidenceBytes
		if left > l.MaxResponseBytes {
			left = l.MaxResponseBytes
		}
		raw, n, e := boundedJSON(capture, left)
		c.result.Usage.EvidenceBytes += n
		if e != nil {
			rec.Outcome = "CAPTURE_INCOMPLETE"
			rec.Reason = "EVIDENCE_BYTES"
			if !errors.Is(e, errCapture) {
				rec.Outcome = "CAPTURE_FAILED"
				rec.Reason = "MALFORMED_EVIDENCE: " + e.Error()
			}
			value = nil
		} else {
			rec.Response = raw
			rec.CaptureComplete = true
		}
		if err != nil {
			value = nil
		}
	}
	rec.EvidenceBytes = c.result.Usage.EvidenceBytes - rec.Before.EvidenceBytes
	c.result.Requests = append(c.result.Requests, rec)
	return value, &c.result.Requests[len(c.result.Requests)-1]
}
func recordResolution(rec *RequestRecord) Resolution {
	status := ResolutionFailed
	if rec.Outcome == "BUDGET_BLOCKED" || rec.Outcome == "CAPTURE_INCOMPLETE" {
		status = ResolutionBlocked
	}
	if rec.Outcome == "UNSUPPORTED" {
		status = ResolutionUnsupported
	}
	return Resolution{Status: status, Reason: rec.Reason}
}
func (c *runner) resolve(t Target) Resolution {
	raw, _ := json.Marshal(t.Locator)
	key := string(raw)
	if r, ok := c.resolutions[key]; ok {
		return r
	}
	r := Resolution{Status: Missing, RequestIDs: []string{}}
	defer func() { c.resolutions[key] = r }()
	add := func(rec *RequestRecord) { r.RequestIDs = append(r.RequestIDs, rec.ID) }
	failure := func(rec *RequestRecord) { ids := r.RequestIDs; r = recordResolution(rec); r.RequestIDs = ids }
	if supplier, ok := c.client.(DocumentSupplier); ok {
		v, rec := c.invoke(t.ID, "source/prepareDocument", "", t.Locator, func(ctx context.Context) (any, error) {
			return supplier.PrepareDocument(ctx, c.result.Request.Context, t.Locator)
		})
		add(rec)
		if rec.Outcome != "SUCCESS" {
			failure(rec)
			return r
		}
		supply := v.(Supply)
		if supply.URI != t.Locator.URI {
			r.Status = ResolutionFailed
			r.Reason = "SUPPLY_URI_MISMATCH"
			return r
		}
		c.result.Supplies = append(c.result.Supplies, supply)
	}
	var symbol *lsp.DocumentSymbol
	position := lsp.Position{}
	if t.Locator.Symbol != "" {
		params := lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: t.Locator.URI}}
		v, rec := c.invoke(t.ID, "textDocument/documentSymbol", "", params, func(ctx context.Context) (any, error) { return c.client.DocumentSymbols(ctx, params) })
		add(rec)
		if rec.Outcome != "SUCCESS" {
			failure(rec)
			return r
		}
		// Iterative forest scan, bounded by the captured response; no recursive DFS.
		stack := append([]lsp.DocumentSymbol(nil), v.([]lsp.DocumentSymbol)...)
		matches := []lsp.DocumentSymbol{}
		for len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if s.Name == t.Locator.Symbol {
				matches = append(matches, s)
			}
			stack = append(stack, s.Children...)
		}
		if len(matches) == 0 {
			r.Reason = "DOCUMENT_SYMBOL_ABSENT"
			return r
		}
		if len(matches) > 1 {
			r.Status = Ambiguous
			r.Reason = "DOCUMENT_SYMBOL_AMBIGUOUS"
			return r
		}
		symbol = &matches[0]
		if graph.ValidateItem(node(lsp.CallHierarchyItem{Name: symbol.Name, URI: t.Locator.URI, Range: symbol.Range, SelectionRange: symbol.SelectionRange}).Item) != nil {
			r.Status = ResolutionFailed
			r.Reason = "DOCUMENT_SYMBOL_MALFORMED_RANGE"
			return r
		}
		position = symbol.SelectionRange.Start
	} else {
		position = lsp.Position{Line: *t.Locator.Line, Character: *t.Locator.Character}
	}
	for delta := uint32(0); delta < MaxPrepareProbes; delta++ {
		if delta > 0 {
			if symbol == nil || position.Character == ^uint32(0) {
				break
			}
			position.Character++
		}
		if symbol != nil && !contains(symbol.Range, position) {
			break
		}
		params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: t.Locator.URI}, Position: position}
		v, rec := c.invoke(t.ID, "textDocument/prepareCallHierarchy", "", params, func(ctx context.Context) (any, error) { return c.client.PrepareCallHierarchy(ctx, params) })
		add(rec)
		if rec.Outcome != "SUCCESS" {
			// A known identifier-miss is still an attempted failed request, not empty.
			if rec.CaptureComplete && strings.Contains(rec.Reason, "json-rpc error 0: identifier not found") && symbol != nil {
				continue
			}
			failure(rec)
			return r
		}
		items := v.([]lsp.CallHierarchyItem)
		valid := map[string]lsp.CallHierarchyItem{}
		for _, i := range items {
			n := node(i)
			if graph.ValidateItem(n.Item) != nil || i.Kind < 1 || i.Kind > 26 || !canonicalURI(i.URI) || i.URI != t.Locator.URI || !contains(i.Range, position) {
				r.Status = ResolutionFailed
				r.Reason = "PREPARED_IDENTITY_MISMATCH"
				return r
			}
			if symbol != nil && (i.Name != symbol.Name || i.Kind != symbol.Kind || (!symbol.Flat && i.SelectionRange != symbol.SelectionRange) || !graph.RangeContains(toRange(symbol.Range), toRange(i.Range))) {
				r.Status = ResolutionFailed
				r.Reason = "PREPARED_SYMBOL_MISMATCH"
				return r
			}
			if _, ok := valid[n.ID]; !ok {
				valid[n.ID] = i
			}
		}
		if len(valid) > 1 {
			r.Status = Ambiguous
			r.Reason = "MULTIPLE_PREPARED_IDENTITIES"
			return r
		}
		for _, i := range valid {
			n := node(i)
			r.Status = Resolved
			r.Identity = &n
			prepared := cloneItem(i)
			r.Prepared = &prepared
			p := position
			r.Position = &p
			return r
		}
		if symbol == nil {
			break
		}
	}
	r.Reason = "PREPARE_RETURNED_NO_ITEM"
	if symbol != nil && c.result.Usage.PrepareProbes >= MaxPrepareProbes && position.Character < ^uint32(0) && contains(symbol.Range, lsp.Position{Line: position.Line, Character: position.Character + 1}) {
		r.Status = ResolutionBlocked
		r.Reason = "MAX_PREPARE_PROBES"
	}
	return r
}

func flattenSymbols(symbols []lsp.DocumentSymbol) []lsp.DocumentSymbol {
	out := []lsp.DocumentSymbol{}
	stack := append([]lsp.DocumentSymbol(nil), symbols...)
	for len(stack) > 0 {
		s := stack[0]
		stack = stack[1:]
		out = append(out, s)
		stack = append(stack, s.Children...)
	}
	return out
}

func callableSymbolKind(kind int) bool { return kind == 6 || kind == 9 || kind == 12 }

func sameDocumentSymbol(a, b lsp.DocumentSymbol) bool {
	return a.Name == b.Name && a.Kind == b.Kind && a.Range == b.Range && a.SelectionRange == b.SelectionRange
}

func nearestStrictContainer(symbols []lsp.DocumentSymbol, child lsp.DocumentSymbol) (*lsp.DocumentSymbol, bool) {
	containers := []lsp.DocumentSymbol{}
	for _, symbol := range symbols {
		if symbol.Range != child.Range && graph.RangeContains(toRange(symbol.Range), toRange(child.Range)) {
			containers = append(containers, symbol)
		}
	}
	nearest := []lsp.DocumentSymbol{}
	for i, candidate := range containers {
		containsNarrower := false
		for j, other := range containers {
			if i != j && candidate.Range != other.Range && graph.RangeContains(toRange(candidate.Range), toRange(other.Range)) {
				containsNarrower = true
				break
			}
		}
		if !containsNarrower {
			nearest = append(nearest, candidate)
		}
	}
	if len(nearest) != 1 {
		return nil, false
	}
	return &nearest[0], true
}

func childlessTopmostSiblings(seed lsp.CallHierarchyItem, symbols []lsp.DocumentSymbol) ([]lsp.DocumentSymbol, bool) {
	seedDeclarations := []lsp.DocumentSymbol{}
	for _, symbol := range symbols {
		if callableSymbolKind(symbol.Kind) && symbol.Kind == seed.Kind && symbol.SelectionRange == seed.SelectionRange && graph.RangeContains(toRange(symbol.Range), toRange(seed.Range)) {
			seedDeclarations = append(seedDeclarations, symbol)
		}
	}
	if len(seedDeclarations) != 1 {
		return nil, false
	}
	seedDeclaration := seedDeclarations[0]
	seedContainer, ok := nearestStrictContainer(symbols, seedDeclaration)
	if !ok {
		return nil, false
	}
	peers := []lsp.DocumentSymbol{}
	for _, symbol := range symbols {
		if !callableSymbolKind(symbol.Kind) || sameDocumentSymbol(symbol, seedDeclaration) {
			continue
		}
		container, exact := nearestStrictContainer(symbols, symbol)
		if exact && sameDocumentSymbol(*container, *seedContainer) {
			peers = append(peers, symbol)
		}
	}
	return peers, true
}

// expandTopmostSiblings runs inside the coordinator so every document-symbol and
// prepare request consumes the same global time/request/evidence/node budgets as
// resolution and traversal. It emits discovery evidence only, never CALLS edges.
func (c *runner) expandTopmostSiblings() {
	seenSeed := map[string]bool{}
	for targetIndex := range c.result.Targets {
		t := &c.result.Targets[targetIndex]
		if t.Admission != Admitted || t.Resolution.Prepared == nil || seenSeed[t.Resolution.Identity.ID] {
			continue
		}
		seenSeed[t.Resolution.Identity.ID] = true
		uri := t.Resolution.Prepared.URI
		params := lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}
		value, record := c.invoke(t.Requested.ID, "textDocument/documentSymbol", t.Resolution.Identity.ID, params, func(ctx context.Context) (any, error) {
			return c.client.DocumentSymbols(ctx, params)
		})
		if record.Outcome != "SUCCESS" {
			c.result.AcquisitionComplete = false
			continue
		}
		all := flattenSymbols(value.([]lsp.DocumentSymbol))
		children := []lsp.DocumentSymbol{}
		var container *lsp.DocumentSymbol
		for i := range all {
			if graph.RangeContains(toRange(all[i].Range), toRange(t.Resolution.Prepared.Range)) && len(all[i].Children) > 0 {
				if container == nil || graph.RangeContains(toRange(container.Range), toRange(all[i].Range)) {
					copy := all[i]
					container = &copy
				}
			}
		}
		if container != nil {
			children = append(children, container.Children...)
		} else {
			var exact bool
			children, exact = childlessTopmostSiblings(*t.Resolution.Prepared, all)
			if !exact {
				continue
			}
		}
		sort.SliceStable(children, func(i, j int) bool {
			if children[i].SelectionRange.Start != children[j].SelectionRange.Start {
				return less(children[i].SelectionRange.Start, children[j].SelectionRange.Start)
			}
			return children[i].Name < children[j].Name
		})
		for _, sibling := range children {
			if uri == t.Resolution.Prepared.URI && sibling.SelectionRange == t.Resolution.Prepared.SelectionRange {
				continue
			}
			declaration := node(lsp.CallHierarchyItem{Name: sibling.Name, Kind: sibling.Kind, URI: uri, Range: sibling.Range, SelectionRange: sibling.SelectionRange})
			if graph.ValidateItem(declaration.Item) != nil {
				c.result.AcquisitionComplete = false
				continue
			}
			prepare := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: sibling.SelectionRange.Start}
			prepared, prepRecord := c.invoke(t.Requested.ID, "textDocument/prepareCallHierarchy", t.Resolution.Identity.ID, prepare, func(ctx context.Context) (any, error) {
				return c.client.PrepareCallHierarchy(ctx, prepare)
			})
			if prepRecord.Outcome != "SUCCESS" {
				c.result.AcquisitionComplete = false
				continue
			}
			matches := []lsp.CallHierarchyItem{}
			for _, item := range prepared.([]lsp.CallHierarchyItem) {
				candidate := node(item)
				if canonicalURI(item.URI) && item.URI == uri && item.SelectionRange == sibling.SelectionRange && item.Kind == sibling.Kind && graph.ValidateItem(candidate.Item) == nil {
					matches = append(matches, item)
				}
			}
			if len(matches) != 1 || !c.admit(matches[0]) {
				c.result.AcquisitionComplete = false
				continue
			}
			candidate := node(matches[0])
			c.result.Graph.SiblingCandidates = append(c.result.Graph.SiblingCandidates, graph.SiblingCandidate{SeedURI: uri, SeedLabel: t.Requested.ID, Origin: *t.Resolution.Identity, Declaration: &declaration, Candidate: candidate, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}})
		}
	}
}

func (c *runner) query(target, id string, d Direction) (queryResult, bool) {
	if c.replay != nil {
		c.replay.beforeQuery(c, target, id, d)
	}
	key := string(d) + ":" + id
	if q, ok := c.queries[key]; ok {
		if err := c.ctx.Err(); err != nil {
			return queryResult{status: BudgetBlocked, reason: err.Error(), requestID: q.requestID}, true
		}
		return q, true
	}
	item := c.items[id]
	method := "callHierarchy/incomingCalls"
	if d == Outgoing {
		method = "callHierarchy/outgoingCalls"
	}
	v, rec := c.invoke(target, method, id, struct {
		Item lsp.CallHierarchyItem `json:"item"`
	}{item}, func(ctx context.Context) (any, error) {
		if d == Outgoing {
			calls, null, e := c.client.OutgoingCalls(ctx, item)
			if null && e == nil {
				calls = nil
			}
			return calls, e
		}
		calls, null, e := c.client.IncomingCalls(ctx, item)
		if null && e == nil {
			calls = nil
		}
		return calls, e
	})
	q := queryResult{requestID: rec.ID, reason: rec.Reason}
	switch rec.Outcome {
	case "BUDGET_BLOCKED", "CAPTURE_INCOMPLETE":
		q.status = BudgetBlocked
	case "UNSUPPORTED":
		q.status = Unsupported
	case "FAILED", "CAPTURE_FAILED":
		q.status = Failed
	default:
		q.status = SuccessEmpty
	}
	if rec.Outcome == "SUCCESS" {
		candidates := []neighbor{}
		if d == Outgoing {
			for _, call := range v.([]lsp.CallHierarchyOutgoingCall) {
				candidates = append(candidates, neighbor{item: call.To, edge: graph.Edge{CallerNodeID: id, CalleeNodeID: node(call.To).ID, CallSites: ranges(call.FromRanges)}})
			}
		} else {
			for _, call := range v.([]lsp.CallHierarchyIncomingCall) {
				candidates = append(candidates, neighbor{item: call.From, edge: graph.Edge{CallerNodeID: node(call.From).ID, CalleeNodeID: id, CallSites: ranges(call.FromRanges)}})
			}
		}
		if len(candidates) > 0 {
			q.status = SuccessNonempty
		}
		sort.SliceStable(candidates, func(i, j int) bool { return node(candidates[i].item).ID < node(candidates[j].item).ID })
		for _, n := range candidates {
			bad := n.edge.CallSites == nil || graph.ValidateItem(node(n.item).Item) != nil || !canonicalURI(n.item.URI) || n.item.Kind < 1 || n.item.Kind > 26
			for _, r := range n.edge.CallSites {
				if graph.ValidateRange(r) != nil {
					bad = true
				}
			}
			if bad {
				q.status = Partial
				q.reason = "INVALID_SERVER_RESPONSE"
				continue
			}
			if !c.admit(n.item) {
				q.status = Partial
				q.reason = "MAX_NODES"
				continue
			}
			// Endpoint admission precedes edge retention, including cache admission.
			c.result.Graph.Edges = graph.MergeEdge(c.result.Graph.Edges, n.edge)
			one := graph.MergeEdge(nil, n.edge)[0]
			n.edge = one
			q.neighbors = append(q.neighbors, n)
			c.observe(rec.ID, one)
		}
	}
	c.queries[key] = q
	return q, false
}
func ranges(rs []lsp.Range) []graph.Range {
	if rs == nil {
		return nil
	}
	out := make([]graph.Range, len(rs))
	for i, r := range rs {
		out[i] = toRange(r)
	}
	return out
}

type queued struct {
	id    string
	depth int
}

func (c *runner) traverse(d Direction) {
	count := len(c.result.Targets)
	queues := make([][]queued, count)
	depths := make([]map[string]int, count)
	for i := range c.result.Targets {
		t := &c.result.Targets[i]
		out := &t.Incoming
		if d == Outgoing {
			out = &t.Outgoing
		}
		depths[i] = map[string]int{}
		if t.Admission != Admitted {
			if t.Admission == AdmissionBlocked || t.Resolution.Status == ResolutionBlocked {
				out.Status = BudgetBlocked
			}
			continue
		}
		starts := []string{t.Resolution.Identity.ID}
		if d == IncomingDirection && c.result.Request.Mode == Slice {
			starts = append(append([]string{}, t.Outgoing.FrontierIDs...), t.Outgoing.SuccessfulEmptyIDs...)
		}
		sort.Strings(starts)
		starts = unique(starts)
		out.StartIDs = starts
		for _, id := range starts {
			depths[i][id] = 0
			queues[i] = append(queues[i], queued{id, 0})
		}
	}
	// A visit is one target's next BFS node; targets rotate even after cache hits.
	for {
		progress := false
		for i := 0; i < count; i++ {
			if len(queues[i]) == 0 {
				continue
			}
			progress = true
			sort.Slice(queues[i], func(a, b int) bool {
				u, v := queues[i][a], queues[i][b]
				if u.depth != v.depth {
					return u.depth < v.depth
				}
				return u.id < v.id
			})
			q := queues[i][0]
			queues[i] = queues[i][1:]
			t := &c.result.Targets[i]
			out := &t.Incoming
			limit := t.Requested.UpDepth
			if d == Outgoing {
				out = &t.Outgoing
				limit = t.Requested.DownDepth
			}
			e := Expansion{NodeID: q.id, Depth: q.depth}
			if q.depth >= limit {
				e.Status = Frontier
				out.FrontierIDs = append(out.FrontierIDs, q.id)
			} else {
				observed, cached := c.query(t.Requested.ID, q.id, d)
				e.Status = observed.status
				e.RequestID = observed.requestID
				e.Cached = cached
				e.Reason = observed.reason
				if e.Status == SuccessEmpty {
					out.SuccessfulEmptyIDs = append(out.SuccessfulEmptyIDs, q.id)
				}
				for _, n := range observed.neighbors {
					next := node(n.item).ID
					c.members[i][next] = true
					c.members[i][q.id] = true
					c.relations[i][n.edge.RelationID] = true
					if _, seen := depths[i][next]; !seen {
						depths[i][next] = q.depth + 1
						queues[i] = append(queues[i], queued{next, q.depth + 1})
					}
				}
			}
			out.Expansions = append(out.Expansions, e)
		}
		if !progress {
			break
		}
	}
	for i := range c.result.Targets {
		t := &c.result.Targets[i]
		out := &t.Incoming
		if d == Outgoing {
			out = &t.Outgoing
		}
		for id, depth := range depths[i] {
			for len(out.Layers) <= depth {
				out.Layers = append(out.Layers, graph.SliceLayer{Depth: len(out.Layers), NodeIDs: []string{}})
			}
			out.Layers[depth].NodeIDs = append(out.Layers[depth].NodeIDs, id)
		}
		for j := range out.Layers {
			sort.Strings(out.Layers[j].NodeIDs)
		}
		sort.Strings(out.FrontierIDs)
		sort.Strings(out.SuccessfulEmptyIDs)
		if len(out.Expansions) > 0 {
			out.Status = aggregate(out.Expansions)
		}
		if out.Status != SuccessEmpty && out.Status != SuccessNonempty && out.Status != ExpansionNotApplicable {
			c.result.AcquisitionComplete = false
		}
	}
}
func aggregate(es []Expansion) ExpansionStatus {
	first := es[0].Status
	all := true
	nonempty := false
	allSuccess := true
	for _, e := range es {
		if e.Status != first {
			all = false
		}
		if e.Status == SuccessNonempty {
			nonempty = true
		}
		if e.Status != SuccessEmpty && e.Status != SuccessNonempty {
			allSuccess = false
		}
	}
	if all {
		return first
	}
	if allSuccess {
		if nonempty {
			return SuccessNonempty
		}
		return SuccessEmpty
	}
	return Partial
}
func unique(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func (c *runner) finishGraph() error {
	g := &c.result.Graph
	g.SchemaVersion = graph.SchemaVersionV3
	g.Nodes = make([]graph.Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		g.Nodes = append(g.Nodes, n)
	}
	r := c.result.Request
	g.Invocation.Limits = graph.Limits{MaxDepth: r.Root.UpDepth, MaxNodes: r.Limits.MaxNodes, TimeoutMS: r.Limits.Timeout.Milliseconds()}
	g.Invocation.RequestTimeoutMS = r.Limits.RequestTimeout.Milliseconds()
	g.Invocation.Concurrency = 1
	g.Invocation.Provenance.InvocationID = r.Context.ID
	for i, t := range c.result.Targets {
		p := graph.Target{URI: t.Requested.Locator.URI}
		if t.Resolution.Position != nil {
			p.Line = int(t.Resolution.Position.Line)
			p.Column = int(t.Resolution.Position.Character)
		} else if t.Requested.Locator.Line != nil {
			p.Line = int(*t.Requested.Locator.Line)
			p.Column = int(*t.Requested.Locator.Character)
		}
		at, _ := json.Marshal(t.Requested.Locator)
		seed := graph.SeedResult{Label: t.Requested.ID, Requested: p, PreparedTargetIDs: []string{}, ReachedNodeIDs: keys(c.members[i]), ReachedRelationIDs: keys(c.relations[i])}
		inv := graph.InvocationSeed{Label: t.Requested.ID, At: string(at), LanguageID: t.Requested.Locator.LanguageID}
		if t.Admission == Admitted {
			seed.PreparedTargetIDs = []string{t.Resolution.Identity.ID}
			g.Targets = append(g.Targets, t.Resolution.Identity.ID)
			inv.ResolvedURI = t.Resolution.Identity.URI
		} else {
			seed.Failure = &graph.SeedFailure{Phase: "prepare", Message: string(t.Resolution.Status) + ":" + t.Resolution.Reason + ":" + string(t.Admission)}
		}
		g.Seeds = append(g.Seeds, seed)
		g.Invocation.Seeds = append(g.Invocation.Seeds, inv)
		if i == 0 {
			g.Invocation.Target = p
			g.Invocation.LanguageID = inv.LanguageID
		}
	}
	// Preserve native historical counters/receipts, but do not use them as the
	// coordinator's directional acquisition truth. One diagnostic prevents the
	// historical canonicalizer from hiding acquisition partiality.
	if !c.result.AcquisitionComplete {
		g.Diagnostics = append(g.Diagnostics, graph.Diagnostic{Phase: "prepare", Message: "MULTI_TARGET_ACQUISITION_PARTIAL: consult coordinator accounting"})
	}
	g.Canonicalize()
	if err := g.ValidateReferences(); err != nil {
		return err
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return graph.ValidateSemanticBundle(raw)
}

// PathInput binds native relation IDs and final canonical call-site pointers.
// No artifact digest enters the witness preimage, avoiding a digest cycle.
func PathInput(g graph.Result, contextID string) ([]string, []retainedpath.Edge) {
	nodes := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = n.ID
	}
	sort.Strings(nodes)
	edges := make([]retainedpath.Edge, 0, len(g.Edges))
	for i, e := range g.Edges {
		occ := make([]string, len(e.CallSites))
		for j := range occ {
			occ[j] = fmt.Sprintf("/edges/%d/call_sites/%d", i, j)
		}
		state := "PRESENT"
		if len(occ) == 0 {
			state = "EMPTY"
		}
		edges = append(edges, retainedpath.Edge{GroupID: e.RelationID, ContextID: contextID, ExecutionBundleID: e.ExecutionBundleID, Caller: e.CallerNodeID, Callee: e.CalleeNodeID, Weight: 1, CallsiteState: state, OccurrenceIDs: occ})
	}
	return nodes, edges
}
func (c *runner) connect() error {
	nodes, edges := PathInput(c.result.Graph, c.result.Request.Context.ID)
	budget := &retainedpath.Budget{Context: c.ctx, Left: c.result.Request.Limits.MaxPathWork}
	root := c.result.Targets[0]
	for i := 1; i < len(c.result.Targets); i++ {
		t := &c.result.Targets[i]
		if root.Admission != Admitted || t.Admission != Admitted {
			continue
		}
		from, to := root.Resolution.Identity.ID, t.Resolution.Identity.ID
		if c.result.Request.Mode == Incoming {
			from, to = to, from
		}
		before := budget.Left
		result, err := retainedpath.Search(nodes, edges, from, to, budget)
		if err != nil {
			return err
		}
		t.Connection = Connection{From: from, To: to, Direction: "CALLER_TO_CALLEE", Status: result.Status, Reason: result.Reason, Path: result.Path, Work: before - budget.Left}
		if result.Status != "INCOMPLETE" {
			if err := retainedpath.Prove(edges, from, to, result.Status, result.Path); err != nil {
				return err
			}
		}
	}
	c.result.Usage.PathWork = c.result.Request.Limits.MaxPathWork - budget.Left
	return nil
}
