// Package boundedanalysis analyzes historical retained CALLS, never authenticated
// dependencies or normative Program B projections. All identity is artifact scoped.
package boundedanalysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/retainedpath"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
)

const Family = "bounded-retained-analysis"
const Version = "lsp-trace.bounded-retained-analysis.v1"
const Policy = "retained-CALLS-unit-group/v1"
const Scope = "HISTORICAL_ARTIFACT_SCOPED_UNVERIFIED_INCOMPLETE"
const MaxInputBytes = 8 << 20
const MaxBytes = 16 << 20
const MaxNodes = 4096
const MaxEdges = 8192
const MaxWork = 1000000

type Parameters struct {
	Operation string `json:"operation"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Mode      string `json:"mode"`
	MaxWork   int    `json:"max_work"`
}
type Edge = retainedpath.Edge
type Path = retainedpath.Path
type Component struct {
	ID      string   `json:"id"`
	Members []string `json:"members"`
}
type Evidence struct {
	SchemaVersion string      `json:"schema_version"`
	Policy        string      `json:"policy"`
	Scope         string      `json:"scope"`
	InputBytes    []byte      `json:"input_bytes"`
	BasisDigest   string      `json:"basis_digest"`
	Parameters    Parameters  `json:"parameters"`
	Nodes         []string    `json:"nodes"`
	Edges         []Edge      `json:"edges"`
	Status        string      `json:"status"`
	Reason        string      `json:"reason"`
	Path          Path        `json:"path"`
	Components    []Component `json:"components"`
	Digest        string      `json:"digest"`
}

func hash(domain string, raw []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), raw...)))
}
func canonical(v any) []byte {
	raw, _ := json.Marshal(v)
	var x any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	_ = d.Decode(&x)
	raw, _ = json.Marshal(x)
	return raw
}
func basis(raw []byte, p Parameters) string {
	return hash(Version+":basis", canonical(struct {
		Policy     string     `json:"policy"`
		Input      []byte     `json:"input_bytes"`
		Parameters Parameters `json:"parameters"`
	}{Policy, raw, p}))
}
func seal(e Evidence) string { e.Digest = ""; return hash(Version+":result", canonical(e)) }

// Preflight caps bytes and nesting before recursive duplicate/schema decoders.
// It scans raw bytes with constant space; decoded table/graph allocation is thus
// bounded even when a caller supplies hostile nested values or enormous arrays.
func Preflight(raw []byte, limit int) error {
	if len(raw) > limit {
		return errors.New("bounded analysis byte LIMIT")
	}
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8")
	}
	depth := 0
	quoted, escape := false, false
	for _, c := range raw {
		if quoted {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
		}
		if c == '{' || c == '[' {
			depth++
			if depth > 64 {
				return errors.New("bounded analysis nesting LIMIT")
			}
		}
		if c == '}' || c == ']' {
			depth--
		}
	}
	return strictjson.RejectDuplicates(raw)
}
func normalize(p Parameters) (Parameters, error) {
	if p.MaxWork == 0 {
		p.MaxWork = MaxWork
	}
	if p.MaxWork < 1 || p.MaxWork > MaxWork {
		return p, errors.New("max_work must be 1..1000000")
	}
	switch p.Operation {
	case "PROJECT":
		if p.Start != "" || p.End != "" || p.Mode != "" {
			return p, errors.New("PROJECT forbids start/end/mode")
		}
	case "PATH":
		if p.Start == "" || p.End == "" || p.Mode != "" {
			return p, errors.New("PATH requires exact start/end and forbids mode")
		}
	case "COMPONENTS":
		if p.Start != "" || p.End != "" || (p.Mode != "WEAK" && p.Mode != "STRONG") {
			return p, errors.New("COMPONENTS requires explicit WEAK or STRONG")
		}
	default:
		return p, errors.New("operation must be PROJECT, PATH or COMPONENTS")
	}
	return p, nil
}

// AdmitRetained shares only historical input admission, never analytical policy.
// Known encoded carriers are preflighted before recursive historical validation.
func AdmitRetained(raw []byte) (retainedcalls.Evidence, error) {
	if err := preflightAdmission(raw, false); err != nil {
		return retainedcalls.Evidence{}, err
	}
	if _, err := retainedcalls.ValidateFor(raw, retainedcalls.Family, "v1"); err != nil {
		return retainedcalls.Evidence{}, err
	}
	var input retainedcalls.Evidence
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, err
	}
	if len(input.Tables.Endpoints) > MaxNodes || len(input.Tables.Groups) > MaxEdges {
		return input, errors.New("bounded analysis node/group LIMIT")
	}
	if _, err := retainedcalls.Reconstruct(input.Tables); err != nil {
		return input, err
	}
	return input, nil
}

// PreflightArtifact bounds an analytical artifact and its known encoded chain.
// Source content and opaque receipts are not recursively interpreted as JSON.
func PreflightArtifact(raw []byte) error { return preflightAdmission(raw, true) }

func project(raw []byte, p Parameters) (Evidence, error) {
	input, err := AdmitRetained(raw)
	if err != nil {
		return Evidence{}, err
	}
	e := Evidence{SchemaVersion: Version, Policy: Policy, Scope: Scope, InputBytes: append([]byte{}, raw...), BasisDigest: basis(raw, p), Parameters: p, Nodes: []string{}, Edges: []Edge{}, Status: "COMPLETE", Path: Path{[]string{}, []string{}, [][]string{}}, Components: []Component{}}
	for _, n := range input.Tables.Endpoints {
		e.Nodes = append(e.Nodes, n.ID)
	}
	sort.Strings(e.Nodes)
	for _, g := range input.Tables.Groups {
		e.Edges = append(e.Edges, Edge{g.RelationID, g.ContextID, g.ExecutionBundleID, g.CallerNodeID, g.CalleeNodeID, 1, g.CallsiteState, append([]string{}, g.OccurrenceIDs...)})
	}
	sort.Slice(e.Edges, func(i, j int) bool { return e.Edges[i].GroupID < e.Edges[j].GroupID })
	if p.Operation == "PATH" {
		for _, id := range []string{p.Start, p.End} {
			i := sort.SearchStrings(e.Nodes, id)
			if i == len(e.Nodes) || e.Nodes[i] != id {
				return Evidence{}, fmt.Errorf("missing exact node ID %q", id)
			}
		}
	}
	return e, nil
}

type budget struct {
	ctx    context.Context
	left   int
	reason string
}

func (b *budget) tick() bool {
	if b.ctx.Err() != nil {
		b.reason = "CANCELLED"
		return false
	}
	if b.left == 0 {
		b.reason = "LIMIT"
		return false
	}
	b.left--
	return true
}

type adjacency = retainedpath.Adjacency

func indexes(e Evidence) (adjacency, adjacency) {
	return retainedpath.Indexes(e.Edges)
}
func components(e *Evidence, out, in adjacency, b *budget) {
	order := append([]string{}, e.Nodes...)
	if e.Parameters.Mode == "STRONG" {
		// Iterative postorder DFS, followed by transpose traversal (Kosaraju).
		order = []string{}
		seen := map[string]bool{}
		type frame struct {
			v    string
			next int
		}
		for _, root := range e.Nodes {
			if !b.tick() {
				return
			}
			if seen[root] {
				continue
			}
			seen[root] = true
			stack := []frame{{root, 0}}
			for len(stack) > 0 {
				if !b.tick() {
					return
				}
				i := len(stack) - 1
				f := &stack[i]
				if f.next == len(out[f.v]) {
					order = append(order, f.v)
					stack = stack[:i]
					continue
				}
				edge := out[f.v][f.next]
				f.next++
				if !seen[edge.Callee] {
					seen[edge.Callee] = true
					stack = append(stack, frame{edge.Callee, 0})
				}
			}
		}
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	seen := map[string]bool{}
	for _, root := range order {
		if !b.tick() {
			return
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		queue := []string{root}
		for head := 0; head < len(queue); head++ {
			if !b.tick() {
				return
			}
			v := queue[head]
			for _, edge := range in[v] {
				if !b.tick() {
					return
				}
				if !seen[edge.Caller] {
					seen[edge.Caller] = true
					queue = append(queue, edge.Caller)
				}
			}
			if e.Parameters.Mode == "WEAK" {
				for _, edge := range out[v] {
					if !b.tick() {
						return
					}
					if !seen[edge.Callee] {
						seen[edge.Callee] = true
						queue = append(queue, edge.Callee)
					}
				}
			}
		}
		sort.Strings(queue)
		e.Components = append(e.Components, Component{hash(Version+":component", canonical([]any{e.BasisDigest, e.Parameters.Mode, queue})), queue})
	}
	sort.Slice(e.Components, func(i, j int) bool { return e.Components[i].Members[0] < e.Components[j].Members[0] })
}
func run(ctx context.Context, e *Evidence) error {
	if e.Parameters.Operation == "PATH" {
		work := &retainedpath.Budget{Context: ctx, Left: e.Parameters.MaxWork}
		// project has already admitted these exact endpoints. No new admission
		// policy or artifact identity participates in the neutral kernel.
		result, err := retainedpath.Search(e.Nodes, e.Edges, e.Parameters.Start, e.Parameters.End, work)
		if err != nil {
			return err
		}
		e.Status, e.Reason, e.Path = result.Status, result.Reason, result.Path
		return nil
	}
	b := &budget{ctx: ctx, left: e.Parameters.MaxWork}
	out, in := indexes(*e)
	if ctx.Err() != nil {
		e.Status = "INCOMPLETE"
		e.Reason = "CANCELLED"
		return nil
	}
	switch e.Parameters.Operation {
	case "PROJECT":
		for range e.Nodes {
			if !b.tick() {
				break
			}
		}
		if b.reason == "" {
			for range e.Edges {
				if !b.tick() {
					break
				}
			}
		}
		if ctx.Err() != nil {
			b.reason = "CANCELLED"
		}
	case "COMPONENTS":
		components(e, out, in, b)
	}
	if b.reason != "" {
		e.Status = "INCOMPLETE"
		e.Reason = b.reason
		e.Path = Path{[]string{}, []string{}, [][]string{}}
		e.Components = []Component{}
	}
	return nil
}
func Analyze(ctx context.Context, raw []byte, p Parameters) ([]byte, error) {
	p, err := normalize(p)
	if err != nil {
		return nil, err
	}
	e, err := project(raw, p)
	if err != nil {
		return nil, err
	}
	if err := run(ctx, &e); err != nil {
		return nil, err
	}
	e.Digest = seal(e)
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > MaxBytes {
		return nil, errors.New("bounded analysis output LIMIT")
	}
	return append(encoded, '\n'), nil
}

func ValidateFor(raw []byte, family, version string) (string, error) {
	if family != Family {
		return retainedcalls.ValidateFor(raw, family, version)
	}
	if err := preflightAdmission(raw, true); err != nil {
		return "", err
	}
	if _, err := schema.ValidateStructure(raw, family, version); err != nil {
		return "", err
	}
	var e Evidence
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return "", err
	}
	if !bytes.Equal(canonical(json.RawMessage(raw)), canonical(e)) {
		return "", errors.New("exact members required recursively")
	}
	p, err := normalize(e.Parameters)
	if err != nil {
		return "", err
	}
	if p != e.Parameters {
		return "", errors.New("noncanonical parameters")
	}
	expected, err := project(e.InputBytes, p)
	if err != nil {
		return "", err
	}
	if e.SchemaVersion != Version || e.Policy != Policy || e.Scope != Scope || e.BasisDigest != expected.BasisDigest || e.Digest != seal(e) || !bytes.Equal(canonical(e.Nodes), canonical(expected.Nodes)) || !bytes.Equal(canonical(e.Edges), canonical(expected.Edges)) {
		return "", errors.New("projection/policy/basis/digest mismatch")
	}
	// Independent graph proofs are separate from deterministic producer replay.
	if e.Status != "INCOMPLETE" {
		if err = prove(e); err != nil {
			return "", err
		}
	}
	if e.Status == "INCOMPLETE" && e.Reason == "CANCELLED" {
		expected.Status = "INCOMPLETE"
		expected.Reason = "CANCELLED"
	} else {
		if err := run(context.Background(), &expected); err != nil {
			return "", err
		}
	}
	expected.Digest = seal(expected)
	if !bytes.Equal(canonical(e), canonical(expected)) {
		return "", errors.New("analytical result or deterministic resource accounting mismatch")
	}
	return Version, nil
}

// prove checks shortestness via reverse distances (not producer predecessor BFS),
// and partitions via per-block connectivity plus quotient acyclicity (not SCC).
func prove(e Evidence) error {
	if e.Parameters.Operation == "PROJECT" {
		return nil
	}
	if e.Parameters.Operation == "PATH" {
		return retainedpath.Prove(e.Edges, e.Parameters.Start, e.Parameters.End, e.Status, e.Path)
	}
	out, in := indexes(e)
	membership := map[string]int{}
	for i, c := range e.Components {
		if len(c.Members) == 0 {
			return errors.New("empty component")
		}
		for _, v := range c.Members {
			if _, ok := membership[v]; ok {
				return errors.New("overlapping partition")
			}
			membership[v] = i
		}
	}
	if len(membership) != len(e.Nodes) {
		return errors.New("incomplete partition")
	}
	for _, v := range e.Nodes {
		if _, ok := membership[v]; !ok {
			return errors.New("lost node")
		}
	}
	for i, c := range e.Components {
		passes := 1
		if e.Parameters.Mode == "STRONG" {
			passes = 2
		}
		for pass := 0; pass < passes; pass++ {
			seen := map[string]bool{c.Members[0]: true}
			q := []string{c.Members[0]}
			for h := 0; h < len(q); h++ {
				visit := func(v string) {
					if membership[v] == i && !seen[v] {
						seen[v] = true
						q = append(q, v)
					}
				}
				if pass == 0 {
					for _, edge := range out[q[h]] {
						visit(edge.Callee)
					}
				}
				if pass == 1 || e.Parameters.Mode == "WEAK" {
					for _, edge := range in[q[h]] {
						visit(edge.Caller)
					}
				}
			}
			if len(seen) != len(c.Members) {
				return errors.New("component is not connected in required direction")
			}
		}
	}
	quotient := make([][]int, len(e.Components))
	degree := make([]int, len(e.Components))
	for _, edge := range e.Edges {
		a, b := membership[edge.Caller], membership[edge.Callee]
		if a != b {
			if e.Parameters.Mode == "WEAK" {
				return errors.New("weak component split across edge")
			}
			quotient[a] = append(quotient[a], b)
			degree[b]++
		}
	}
	q := []int{}
	for i, d := range degree {
		if d == 0 {
			q = append(q, i)
		}
	}
	for h := 0; h < len(q); h++ {
		for _, v := range quotient[q[h]] {
			degree[v]--
			if degree[v] == 0 {
				q = append(q, v)
			}
		}
	}
	if len(q) != len(e.Components) {
		return errors.New("strong partition quotient has cycle")
	}
	return nil
}
