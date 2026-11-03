//go:build darwin || linux

package censuslineagepilot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

// These are executed-code-site observations, not physical IO, memory, billing,
// or a proof that any reservation multiplier is necessary or minimal.
// Coverage is supplied by ordinary Go instrumentation; no production hook,
// runtime/coverage export, source overlay, or restored capture owner is used.
type costWitnessRecordV3 struct {
	Files                     int
	Stage, Source, Allocation PageCostPrivateV3
	Expected                  []PageCostPrivateV3
	Records                   []PageCostPrivateV3
	Delta                     PageCostPrivateV3
	Outcome                   string
	SourceHashes              map[string]string
}
type costWitnessBlockV3 struct {
	File                       string
	SL, SC, EL, EC, Statements int
}
type costWitnessProfileV3 map[costWitnessBlockV3]uint64

type costWitnessSiteV3 struct{ file, receiver, function, call string }

var costWitnessSitesV3 = map[string]costWitnessSiteV3{
	"read":       {"producer_pages_checkpoint_admission_private_v3.go", "RetainedSelectedBatchPrivateV3", "Read", ""},
	"stage":      {"producer_pages_checkpoint_admission_private_v3.go", "retainedCheckpointStatePrivateV3", "stageCost", ""},
	"source":     {"variable_scope_private_v2.go", "FrozenScopePrivateV2", "revalidate", ""},
	"file":       {"feature_scope_source_unix.go", "", "readScopeDescriptorsPrivate", "ReadAll"},
	"pair":       {"producer_pages_checkpoint_admission_private_v3.go", "retainedCheckpointStatePrivateV3", "readPair", ""},
	"descriptor": {"producer_pages_checkpoint_admission_private_v3.go", "retainedCheckpointStatePrivateV3", "descriptor", ""},
	"commit":     {"producer_pages_ledger_private_v3.go", "pageLedgerStatePrivateV3", "reserve", "createRecord"},
}

func costWitnessParseV3(raw []byte) (costWitnessProfileV3, error) {
	if len(raw) > 16<<20 || !bytes.HasPrefix(raw, []byte("mode: atomic\n")) || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, fmt.Errorf("CW_PROFILE: incomplete/non-atomic profile")
	}
	p := costWitnessProfileV3{}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Scan()
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 3 {
			return nil, fmt.Errorf("CW_PROFILE: malformed block")
		}
		colon := strings.LastIndex(fields[0], ":")
		if colon < 0 {
			return nil, fmt.Errorf("CW_PROFILE: location")
		}
		b := costWitnessBlockV3{File: fields[0][:colon]}
		if n, e := fmt.Sscanf(fields[0][colon+1:], "%d.%d,%d.%d", &b.SL, &b.SC, &b.EL, &b.EC); e != nil || n != 4 {
			return nil, fmt.Errorf("CW_PROFILE: range")
		}
		n, e := strconv.Atoi(fields[1])
		if e != nil || n < 0 {
			return nil, fmt.Errorf("CW_PROFILE: statements")
		}
		b.Statements = n
		count, e := strconv.ParseUint(fields[2], 10, 64)
		if e != nil {
			return nil, e
		}
		if _, ok := p[b]; ok {
			return nil, fmt.Errorf("CW_PROFILE: duplicate block")
		}
		p[b] = count
	}
	if scan.Err() != nil || len(p) == 0 {
		return nil, fmt.Errorf("CW_PROFILE: empty/unreadable")
	}
	return p, nil
}

// Resolve the specific executed statement, not a string's mere presence in a
// profile. Ambiguous or absent AST/profile bindings refuse the measurement.
func costWitnessResolveV3(p costWitnessProfileV3, site costWitnessSiteV3) (costWitnessBlockV3, error) {
	fs := token.NewFileSet()
	f, e := parser.ParseFile(fs, site.file, nil, 0)
	if e != nil {
		return costWitnessBlockV3{}, e
	}
	var points []token.Pos
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != site.function || fn.Body == nil {
			continue
		}
		receiver := ""
		if fn.Recv != nil {
			x := fn.Recv.List[0].Type
			if ptr, ok := x.(*ast.StarExpr); ok {
				x = ptr.X
			}
			if id, ok := x.(*ast.Ident); ok {
				receiver = id.Name
			}
		}
		if receiver != site.receiver {
			continue
		}
		if site.call == "" {
			points = append(points, fn.Body.List[0].Pos())
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == site.call {
				points = append(points, c.Pos())
			}
			return true
		})
	}
	if len(points) != 1 {
		return costWitnessBlockV3{}, fmt.Errorf("CW_SITE: %s.%s/%s points=%d", site.receiver, site.function, site.call, len(points))
	}
	pos := fs.Position(points[0])
	var matches []costWitnessBlockV3
	for b := range p {
		if filepath.Base(b.File) != site.file {
			continue
		}
		after := pos.Line > b.SL || pos.Line == b.SL && pos.Column >= b.SC
		before := pos.Line < b.EL || pos.Line == b.EL && pos.Column < b.EC
		if after && before && b.Statements > 0 {
			matches = append(matches, b)
		}
	}
	if len(matches) != 1 {
		return costWitnessBlockV3{}, fmt.Errorf("CW_SITE: %s:%d:%d blocks=%d", site.file, pos.Line, pos.Column, len(matches))
	}
	return matches[0], nil
}
func costWitnessCountersV3(base, repeat, measured costWitnessProfileV3) (map[string]uint64, error) {
	if len(base) != len(repeat) || len(base) != len(measured) {
		return nil, fmt.Errorf("CW_PROFILE: block inventory mismatch")
	}
	for b := range base {
		if _, ok := repeat[b]; !ok {
			return nil, fmt.Errorf("CW_PROFILE: baseline block missing")
		}
		if _, ok := measured[b]; !ok {
			return nil, fmt.Errorf("CW_PROFILE: measured block missing")
		}
	}
	result := map[string]uint64{}
	for name, site := range costWitnessSitesV3 {
		b, e := costWitnessResolveV3(base, site)
		if e != nil {
			return nil, e
		}
		if base[b] != repeat[b] {
			return nil, fmt.Errorf("CW_STABILITY: %s baseline drift %d/%d", name, base[b], repeat[b])
		}
		if measured[b] < base[b] {
			return nil, fmt.Errorf("CW_PROFILE: negative delta %s", name)
		}
		result[name] = measured[b] - base[b]
	}
	return result, nil
}
func costWitnessReadPriceV3(n uint64) PageCostPrivateV3 {
	return PageCostPrivateV3{LogicalBytes: 8 * (n + 1), ReadHashBytes: 16 * (n + 1), Work: producerPacketTopologyWorkPrivateV2 + 64}
}
func costWitnessSumV3(costs ...PageCostPrivateV3) PageCostPrivateV3 {
	var out PageCostPrivateV3
	for _, c := range costs {
		var e error
		out, e = out.plus(c)
		if e != nil {
			panic(e)
		}
	}
	return out
}
func costWitnessJournalV3(t *testing.T, x retainedAdmissionFixtureV3, g uint64, tip string, before PageCostPrivateV3) ([]PageCostPrivateV3, PageCostPrivateV3) {
	t.Helper()
	end, hash, after := x.l.Tip()
	if end < g || end-g > 128 {
		t.Fatal("CW_LEDGER: unbounded delta")
	}
	var records []PageCostPrivateV3
	cumulative := before
	previous := tip
	for i := g + 1; i <= end; i++ {
		raw, e := os.ReadFile(filepath.Join(x.l.state.path, ledgerNamePrivateV3(i)))
		if e != nil {
			t.Fatal(e)
		}
		var r pageLedgerRecordPrivateV3
		if canonicalRetainedPrivateV3(raw, &r) != nil || r.Generation != i || r.Previous != previous || r.Application != x.m.Application || r.Allowance != x.l.state.allowance {
			t.Fatal("CW_LEDGER: invalid bounded journal")
		}
		cumulative = costWitnessSumV3(cumulative, r.Reserved)
		if cumulative != r.Cumulative {
			t.Fatal("CW_LEDGER: cumulative")
		}
		previous = Digest(raw)
		records = append(records, r.Reserved)
	}
	if previous != hash || cumulative != after || !before.fits(after) {
		t.Fatal("CW_LEDGER: frontier")
	}
	// A denied reservation must not have created even a journal outside the tip.
	if _, e := os.Stat(filepath.Join(x.l.state.path, ledgerNamePrivateV3(end+1))); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("CW_LEDGER: record beyond tip", e)
	}
	return records, after.minus(before)
}

func TestRetainedCheckpointPrivateV3CostWitnessFixture(t *testing.T) {
	mode := os.Getenv("LSP_COST_MODE")
	if mode == "" {
		t.Skip("isolated coverage helper only")
	}
	switch mode {
	case "baseline", "read", "outer", "nested", "changed", "grown", "corrupt":
	default:
		t.Fatal("unknown cost mode")
	}
	ctx := context.Background()
	x := retainedFixtureV3(t)
	grant := x.l.state.allowance
	owner, e := x.admit(ctx, x.raw, x.d)
	if e != nil {
		t.Fatal(e)
	}
	b, e := owner.SelectedBatch(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	// Identical setup in every mode, including resolving the metadata extent.
	left := grant
	metadata, e := owner.state.findObject(ctx, 6, &left)
	if e != nil {
		t.Fatal(e)
	}
	// Freeze the characterized prices independently from production pricing
	// helpers. These policy reservations are NOT inferred physical cost bounds.
	stageN := uint64(scopeMetadataPrivateV2) + 4*x.m.Revision.Policy.ComputationUnitBytes
	for _, f := range x.fresh.state.files {
		stageN += uint64(len(f.Bytes))
	}
	stage := PageCostPrivateV3{LogicalBytes: 256 * stageN, ReadHashBytes: 256 * stageN, Work: 1000000}
	policy := x.fresh.state.spec.EffectivePolicy
	sourceN := uint64(policy.TotalBytes)
	if n := uint64(len(x.fresh.state.files) * policy.FileBytes); n < sourceN {
		sourceN = n
	}
	sourceN += uint64(len(x.fresh.state.files)) + scopeMetadataPrivateV2
	source := PageCostPrivateV3{LogicalBytes: 24 * sourceN, ReadHashBytes: 32 * sourceN, Work: 2 * producerPacketTopologyWorkPrivateV2}
	rec := costWitnessRecordV3{Files: len(x.fresh.state.files), Stage: stage, Source: source, Allocation: PageCostPrivateV3{LogicalBytes: 4 * metadata.Length, ReadHashBytes: 2 * metadata.Length, Work: 1}, Outcome: "BASELINE"}
	if mode == "changed" || mode == "grown" {
		f := x.fresh.state.files[0]
		raw := bytes.Repeat([]byte("x"), len(f.Bytes))
		if mode == "grown" {
			raw = bytes.Repeat([]byte("x"), x.fresh.state.spec.EffectivePolicy.FileBytes+1)
		}
		if e = os.WriteFile(filepath.Join(x.fresh.state.spec.Root, f.Path), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if mode == "corrupt" {
		if e = os.WriteFile(filepath.Join(x.m.Roots[0].Path, metadata.LastPage), []byte("corrupt-runtime-pair"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	g, tip, before := x.l.Tip()
	if mode != "baseline" {
		exchange := grant
		if mode == "outer" {
			exchange = PageCostPrivateV3{}
		}
		if mode == "nested" {
			exchange = costWitnessSumV3(stage, source, pageLedgerOverheadPrivateV3, pageLedgerOverheadPrivateV3)
		}
		_, e = b.Read(ctx, exchange)
		switch mode {
		case "read":
			if e != nil {
				t.Fatal("CW_SUCCESS", e)
			}
			rec.Outcome = "SUCCESS"
		case "outer", "nested":
			if !errors.Is(e, ErrPagesPausedPrivateV3) {
				t.Fatal("CW_PAUSE_IDENTITY", e)
			}
			rec.Outcome = "PAUSED"
		default:
			want := ErrInvalid
			if mode == "corrupt" {
				want = publication.ErrContentObservation
			}
			if !errors.Is(e, want) {
				t.Fatal("CW_REFUSAL_IDENTITY", mode, e)
			}
			rec.Outcome = "REFUSED"
			t.Logf("CW_REFUSAL %s: %T %v", mode, e, e)
		}
	}
	rec.Records, rec.Delta = costWitnessJournalV3(t, x, g, tip, before)
	// Expected amounts come from the operation's policy contract and metadata,
	// not from classifying ledger records into their own expected work counts.
	switch mode {
	case "baseline", "outer":
	case "nested", "changed", "grown":
		rec.Expected = []PageCostPrivateV3{stage, source}
	case "read", "corrupt":
		rec.Expected = []PageCostPrivateV3{stage, source, source}
		for i := 0; i <= 6; i++ {
			rec.Expected = append(rec.Expected, costWitnessReadPriceV3(producerPageHeaderPrivateV3))
		}
		rec.Expected = append(rec.Expected, rec.Allocation)
		pages := (metadata.Length + x.m.Revision.Policy.PayloadBytes - 1) / x.m.Revision.Policy.PayloadBytes
		if mode == "corrupt" {
			pages = 1
		}
		for i := uint64(0); i < pages; i++ {
			rec.Expected = append(rec.Expected, costWitnessReadPriceV3(x.m.Revision.Policy.PayloadBytes+producerPageHeaderPrivateV3+4))
		}
		if mode == "read" {
			rec.Expected = append(rec.Expected, source)
		}
	}
	rec.SourceHashes = costWitnessSourceHashesV3(t)
	raw, e := json.Marshal(rec)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(os.Getenv("LSP_COST_RESULT"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("CW_FIXTURE_COMPLETED", mode)
}

func costWitnessRunV3(t *testing.T, helper, mode string) (costWitnessProfileV3, costWitnessRecordV3) {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	profile := filepath.Join(dir, "operations.cover")
	result := filepath.Join(dir, "result.json")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	args := []string{"test", ".", "-covermode=atomic", "-coverprofile=" + profile, "-run=^" + helper + "$", "-count=1", "-timeout=3m", "-json"}
	// Explicit opt-in propagates race instrumentation to fixture children.
	// Without it, an outer -race run alone is NOT inner-fixture race evidence.
	if os.Getenv("LSP_COST_CHILD_RACE") == "1" {
		args = append(args, "-race")
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	env := []string{}
	for _, v := range os.Environ() {
		key := strings.SplitN(v, "=", 2)[0]
		switch key {
		case "LSP_COST_MODE", "LSP_COST_RESULT", "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOWORK", "GOFLAGS":
		default:
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "LSP_COST_MODE="+mode, "LSP_COST_RESULT="+result, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly")
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("CW_CHILD %s: %v\n%s", mode, e, out)
	}
	passes, skips := 0, 0
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var event struct{ Action, Test, Output string }
		e := dec.Decode(&event)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal("CW_CHILD_JSON", e, string(out))
		}
		if event.Test == helper {
			if event.Action == "pass" {
				passes++
			}
			if event.Action == "skip" {
				skips++
			}
		}
	}
	if passes != 1 || skips != 0 {
		t.Fatalf("CW_CHILD_IDENTITY pass=%d skip=%d", passes, skips)
	}
	if e = os.Chmod(profile, 0600); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(profile)
	if e != nil {
		t.Fatal(e)
	}
	p, e := costWitnessParseV3(raw)
	if e != nil {
		t.Fatal(e)
	}
	var rec costWitnessRecordV3
	raw, e = os.ReadFile(result)
	if e != nil {
		t.Fatal(e)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&rec); e != nil {
		t.Fatal(e)
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		t.Fatal("CW_RESULT: trailing data")
	}
	t.Logf("CW_CHILD mode=%s exact-helper-pass=1 skips=0 inner-race=%t", mode, os.Getenv("LSP_COST_CHILD_RACE") == "1")
	if !reflect.DeepEqual(rec.SourceHashes, costWitnessSourceHashesV3(t)) {
		t.Fatal("CW_IDENTITY: source changed")
	}
	if root := os.Getenv("LSP_COST_EVIDENCE"); root != "" {
		dst, err := os.MkdirTemp(root, mode+"-")
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"result.json": raw, "child.json": out} {
			if err = os.WriteFile(filepath.Join(dst, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(profile)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dst, "operations.cover"), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("CW_EVIDENCE", dst)
	}
	return p, rec
}
func costWitnessSourceHashesV3(t *testing.T) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	for _, site := range costWitnessSitesV3 {
		if _, ok := hashes[site.file]; ok {
			continue
		}
		raw, e := os.ReadFile(site.file)
		if e != nil {
			t.Fatal(e)
		}
		hashes[site.file] = Digest(raw)
	}
	return hashes
}
func costWitnessMatchV3(mode string, c map[string]uint64, r costWitnessRecordV3) error {
	want := map[string]uint64{"read": 1, "stage": 1, "source": 3, "file": uint64(3 * r.Files), "descriptor": 7}
	switch mode {
	case "outer":
		want["source"] = 0
		want["file"] = 0
		want["descriptor"] = 0
	case "nested", "changed":
		want["source"] = 1
		want["file"] = uint64(r.Files)
		want["descriptor"] = 0
	case "grown":
		want["source"] = 1
		want["file"] = 0
		want["descriptor"] = 0
	case "corrupt":
		want["source"] = 2
		want["file"] = uint64(2 * r.Files)
	}
	for _, name := range []string{"read", "stage", "source", "file", "descriptor"} {
		v, ok := c[name]
		if !ok || v != want[name] {
			return fmt.Errorf("CW_WORK_%s: got %d present=%t want %d", name, v, ok, want[name])
		}
	}
	// Successful record creation is itself an independent executed code site.
	if c["commit"] != uint64(len(r.Expected)) {
		return fmt.Errorf("CW_WORK_commit: got %d want %d", c["commit"], len(r.Expected))
	}
	pairs := uint64(0)
	if mode == "read" || mode == "corrupt" {
		pairs = uint64(len(r.Expected) - 5)
	} // 3 sources/stage/allocation versus 2 on corruption
	if mode == "corrupt" {
		pairs = uint64(len(r.Expected) - 4)
	}
	if c["pair"] != pairs {
		return fmt.Errorf("CW_WORK_pair: got %d want %d", c["pair"], pairs)
	}
	if len(r.Records) != len(r.Expected) {
		return fmt.Errorf("CW_LEDGER_COUNT: got %d want %d", len(r.Records), len(r.Expected))
	}
	var total PageCostPrivateV3
	for i, cost := range r.Expected {
		charged := costWitnessSumV3(cost, pageLedgerOverheadPrivateV3)
		if r.Records[i] != charged {
			return fmt.Errorf("CW_LEDGER_CHARGE: reservation %d got %+v want %+v", i, r.Records[i], charged)
		}
		total = costWitnessSumV3(total, charged)
	}
	if total != r.Delta {
		return fmt.Errorf("CW_LEDGER_DELTA: got %+v want %+v", r.Delta, total)
	}
	outcome := "SUCCESS"
	if mode == "outer" || mode == "nested" {
		outcome = "PAUSED"
	}
	if mode == "changed" || mode == "grown" || mode == "corrupt" {
		outcome = "REFUSED"
	}
	if r.Outcome != outcome {
		return fmt.Errorf("CW_OUTCOME: got %s want %s", r.Outcome, outcome)
	}
	return nil
}

// Full-flow matching is deliberately narrower than the single-Read recipe:
// all four journal currencies reconcile, stage and fresh-source charges match
// independently executed sites, but other full-flow costs are not minimized.
func costWitnessFlowMatchV3(c map[string]uint64, r costWitnessRecordV3) error {
	if c["read"] != 8 || c["stage"] != 10 {
		return fmt.Errorf("CW_FLOW_STAGES: read=%d stage=%d want 8/10", c["read"], c["stage"])
	}
	if c["source"] < 24 || c["file"] != c["source"]*uint64(r.Files) {
		return fmt.Errorf("CW_FLOW_SOURCE: source=%d file=%d members=%d", c["source"], c["file"], r.Files)
	}
	if c["commit"] != uint64(len(r.Records)) {
		return fmt.Errorf("CW_FLOW_COMMITS: counter=%d records=%d", c["commit"], len(r.Records))
	}
	source := r.Source
	source.Work = 2 * producerPacketTopologyWorkPrivateV2
	stageCharge := costWitnessSumV3(r.Stage, pageLedgerOverheadPrivateV3)
	sourceCharge := costWitnessSumV3(source, pageLedgerOverheadPrivateV3)
	var stages, sources uint64
	var total PageCostPrivateV3
	for _, charged := range r.Records {
		if charged == stageCharge {
			stages++
		}
		if charged == sourceCharge {
			sources++
		}
		total = costWitnessSumV3(total, charged)
	}
	if stages != c["stage"] || sources != c["source"] {
		return fmt.Errorf("CW_FLOW_MATCH: stage=%d/%d source=%d/%d", stages, c["stage"], sources, c["source"])
	}
	if total != r.Delta {
		return fmt.Errorf("CW_FLOW_DELTA: got %+v want %+v", r.Delta, total)
	}
	return nil
}
func costWitnessFlowV3(t *testing.T) {
	t.Helper()
	const helper = "TestRetainedCheckpointPrivateV3CostWitnessExternalFixture"
	base, br := costWitnessRunV3(t, helper, "flow-baseline")
	repeat, rr := costWitnessRunV3(t, helper, "flow-baseline")
	if !reflect.DeepEqual(br, rr) {
		t.Fatal("CW_FLOW_STABILITY: contract drift")
	}
	measured, r := costWitnessRunV3(t, helper, "flow")
	c, e := costWitnessCountersV3(base, repeat, measured)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(br.SourceHashes, r.SourceHashes) {
		t.Fatal("CW_FLOW_IDENTITY: source drift")
	}
	if e = costWitnessFlowMatchV3(c, r); e != nil {
		t.Fatal(e)
	}
	t.Logf("CW_FLOW_MATCH: A-pass counters=%v reservations=%d delta=%+v boundary=one-selected-target-through-three-reference-publication", c, len(r.Records), r.Delta)
	for _, field := range []string{"read", "stage", "source", "delta"} {
		cc := map[string]uint64{}
		for k, v := range c {
			cc[k] = v
		}
		rc := r
		if field == "delta" {
			rc.Delta.LogicalBytes++
		} else {
			cc[field]++
		}
		bad := costWitnessFlowMatchV3(cc, rc)
		if bad == nil {
			t.Fatal("CW_FLOW_NEGATIVE_ACCEPTED", field)
		}
		t.Logf("CW_FLOW_CONTROL_%s: A-fail %s", field, bad)
		if e = costWitnessFlowMatchV3(c, r); e != nil {
			t.Fatal(e)
		}
		t.Logf("CW_FLOW_CONTROL_%s: restored A-pass", field)
	}
}

func TestRetainedCheckpointPrivateV3CostWitnessPairedOperations(t *testing.T) {
	if os.Getenv("LSP_COST_MODE") != "" {
		t.Fatal("CW_RECURSION")
	}
	const helper = "TestRetainedCheckpointPrivateV3CostWitnessFixture"
	base, br := costWitnessRunV3(t, helper, "baseline")
	repeat, rr := costWitnessRunV3(t, helper, "baseline")
	if !reflect.DeepEqual(br, rr) {
		t.Fatal("CW_STABILITY: baseline contract drift")
	}
	for _, mode := range []string{"read", "outer", "nested", "changed", "grown", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			measured, r := costWitnessRunV3(t, helper, mode)
			c, e := costWitnessCountersV3(base, repeat, measured)
			if e != nil {
				t.Fatal(e)
			}
			if e = costWitnessMatchV3(mode, c, r); e != nil {
				t.Fatal(e)
			}
			t.Logf("CW_MATCH_%s: A-pass counters=%v reservations=%d delta=%+v", mode, c, len(r.Records), r.Delta)
			if !reflect.DeepEqual(br.SourceHashes, r.SourceHashes) {
				t.Fatal("CW_IDENTITY: paired sources differ")
			}
			if mode == "outer" || mode == "nested" {
				badCounters := map[string]uint64{}
				for k, v := range c {
					badCounters[k] = v
				}
				badCounters["file"]++
				bad := costWitnessMatchV3(mode, badCounters, r)
				if bad == nil {
					t.Fatal("CW_DENIED_WORK_GUARD_ACCEPTED")
				}
				t.Logf("CW_DENIED_%s: A-fail %s", mode, bad)
				if e = costWitnessMatchV3(mode, c, r); e != nil {
					t.Fatal(e)
				}
				t.Logf("CW_DENIED_%s: restored A-pass", mode)
			}
			if mode != "read" {
				return
			}
			// Each control starts from actual independently captured, satisfying data.
			for _, control := range []string{"missing-source", "extra-source", "missing-site", "wrong-delta", "wrong-charge", "missing-record", "wrong-outcome", "profile-truncated", "profile-drift", "profile-missing-block", "profile-negative"} {
				cc := map[string]uint64{}
				for k, v := range c {
					cc[k] = v
				}
				rc := r
				rc.Records = append([]PageCostPrivateV3(nil), r.Records...)
				var bad error
				switch control {
				case "missing-source":
					cc["source"]--
					bad = costWitnessMatchV3(mode, cc, rc)
				case "extra-source":
					cc["source"]++
					bad = costWitnessMatchV3(mode, cc, rc)
				case "missing-site":
					delete(cc, "file")
					bad = costWitnessMatchV3(mode, cc, rc)
				case "wrong-delta":
					rc.Delta.Work++
					bad = costWitnessMatchV3(mode, cc, rc)
				case "wrong-charge":
					rc.Records[0].WriteBytes++
					bad = costWitnessMatchV3(mode, cc, rc)
				case "missing-record":
					rc.Records = rc.Records[1:]
					bad = costWitnessMatchV3(mode, cc, rc)
				case "wrong-outcome":
					rc.Outcome = "REFUSED"
					bad = costWitnessMatchV3(mode, cc, rc)
				case "profile-truncated":
					var encoded bytes.Buffer
					encoded.WriteString("mode: atomic\n")
					for block, count := range measured {
						fmt.Fprintf(&encoded, "%s:%d.%d,%d.%d %d %d\n", block.File, block.SL, block.SC, block.EL, block.EC, block.Statements, count)
					}
					_, bad = costWitnessParseV3(encoded.Bytes()[:encoded.Len()-1])
				default:
					altered := costWitnessProfileV3{}
					for k, v := range measured {
						altered[k] = v
					}
					b, e := costWitnessResolveV3(base, costWitnessSitesV3["source"])
					if e != nil {
						t.Fatal(e)
					}
					if control == "profile-drift" {
						alteredRepeat := costWitnessProfileV3{}
						for k, v := range repeat {
							alteredRepeat[k] = v
						}
						alteredRepeat[b]++
						_, bad = costWitnessCountersV3(base, alteredRepeat, measured)
					}
					if control == "profile-missing-block" {
						delete(altered, b)
						_, bad = costWitnessCountersV3(base, repeat, altered)
					}
					if control == "profile-negative" {
						if base[b] == 0 {
							t.Fatal("CW_CONTROL: baseline source counter must be nonzero")
						}
						altered[b] = base[b] - 1
						_, bad = costWitnessCountersV3(base, repeat, altered)
					}
				}
				if bad == nil {
					t.Fatal("CW_NEGATIVE_CONTROL_ACCEPTED", control)
				}
				t.Logf("CW_CONTROL_%s: A-fail %s", control, bad)
				if strings.HasPrefix(control, "profile-") {
					// Restoration re-applies the same parser AND counter guard,
					// not merely the downstream reservation matcher.
					var valid bytes.Buffer
					valid.WriteString("mode: atomic\n")
					for block, count := range measured {
						fmt.Fprintf(&valid, "%s:%d.%d,%d.%d %d %d\n", block.File, block.SL, block.SC, block.EL, block.EC, block.Statements, count)
					}
					restored, err := costWitnessParseV3(valid.Bytes())
					if err != nil {
						t.Fatal("CW_PROFILE_RESTORE", err)
					}
					restoredCounts, err := costWitnessCountersV3(base, repeat, restored)
					if err != nil {
						t.Fatal("CW_COUNTER_RESTORE", err)
					}
					if err = costWitnessMatchV3(mode, restoredCounts, r); err != nil {
						t.Fatal("CW_MATCH_RESTORE", err)
					}
				}
				if e = costWitnessMatchV3(mode, c, r); e != nil {
					t.Fatal("CW_RESTORE", e)
				}
				t.Logf("CW_CONTROL_%s: restored A-pass", control)
			}
		})
	}
	if runtime.GOOS == "darwin" {
		costWitnessFlowV3(t)
	}
}
