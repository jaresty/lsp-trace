//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// Owned pipe provider: real CLI, Census, Manager lease, guarded supply and
// original acquisition. No model, external provider or saved owner is involved.
type inventoryPipePrivate struct {
	in, out         *io.PipeReader
	input, output   *io.PipeWriter
	once            sync.Once
	done            chan struct{}
	queries         atomic.Int32
	failCalls       atomic.Bool
	failCallOrdinal int           // owned diagnostic fixture only; zero leaves legacy behavior
	callOrdinal     int           // serve goroutine owns this counter
	shutdownDelay   time.Duration // owned lifecycle counterfactual only
}

func (c *inventoryPipePrivate) Stdin() io.WriteCloser { return c.input }
func (c *inventoryPipePrivate) Stdout() io.ReadCloser { return c.out }
func (c *inventoryPipePrivate) Close() managedprocess.ResourceObservation {
	c.once.Do(func() { c.in.Close(); c.out.Close(); c.input.Close(); c.output.Close() })
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *inventoryPipePrivate) Teardown(context.Context) managedprocess.TeardownObservation {
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *inventoryPipePrivate) serve() {
	defer close(c.done)
	r := lspwire.NewReader(c.in, lspwire.DefaultLimits())
	w := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		m, e := r.Read()
		if e != nil {
			return
		}
		if m.Method == "exit" {
			return
		}
		if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
			continue
		}
		if os.Getenv("LSP_TRACE_OWNED_TERMINAL_SERVER") == "1" {
			if raw, handled := terminalOwnedResponsePrivate(m); handled {
				if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
					return
				}
				continue
			}
		}
		var value any = []any{}
		rng := lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 12}}
		sel := lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 7}}
		switch m.Method {
		case "initialize":
			value = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}}
		case "shutdown":
			time.Sleep(c.shutdownDelay)
			value = nil
		case "callHierarchy/incomingCalls":
			c.callOrdinal++
			if c.failCalls.Load() || (c.failCallOrdinal > 0 && c.callOrdinal == c.failCallOrdinal) || os.Getenv("LSP_TRACE_OWNED_TERMINAL_FAIL") == "1" {
				if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: -32603, Message: "owned failure"}}) != nil {
					return
				}
				continue
			}
		case "textDocument/documentSymbol":
			c.queries.Add(1)
			var p lsp.DocumentSymbolParams
			_ = json.Unmarshal(m.Params, &p)
			kind, name := 12, "A0"
			if strings.HasSuffix(p.TextDocument.URI, "/b.go") {
				kind, name = 13, "V0"
			}
			value = []lsp.DocumentSymbol{{Name: name, Kind: kind, Range: rng, SelectionRange: sel}}
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			_ = json.Unmarshal(m.Params, &p)
			value = []lsp.CallHierarchyItem{{Name: "A0", Kind: 12, URI: p.TextDocument.URI, Range: rng, SelectionRange: sel}}
		}
		raw, _ := json.Marshal(value)
		if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
			return
		}
	}
}

type inventoryStarterPrivate struct {
	child  *inventoryPipePrivate
	starts *atomic.Int32
}

func (s inventoryStarterPrivate) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.starts.Add(1)
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func inventoryDepsPrivate(t *testing.T) (censusRunnerDependencies, *atomic.Int32) {
	t.Helper()
	a, b := io.Pipe()
	c, d := io.Pipe()
	child := &inventoryPipePrivate{in: a, input: b, out: c, output: d, done: make(chan struct{})}
	go child.serve()
	t.Cleanup(func() {
		child.Close()
		select {
		case <-child.done:
		case <-time.After(time.Second):
			t.Error("owned pipe not joined")
		}
	})
	starts := &atomic.Int32{}
	deps := productionCensusRunnerDependencies()
	deps.newStarter = func() (sessionruntime.Starter, error) { return inventoryStarterPrivate{child, starts}, nil }
	deps.lookPath = func(string) (string, error) { return "/owned/server", nil }
	return deps, starts
}
func inventoryConfigPrivate() censusInventoryConfigPrivate {
	return censusInventoryConfigPrivate{Profile: censusInventoryProfilePrivate, RevisionAssertion: strings.Repeat("a", 40), MaxOrigins: 1, OperationTimeoutMS: 240000,
		Policy:      pilot.ScopeEffectivePolicyPrivateV2{MaxNominationFiles: 2, MaxContextFiles: 1, MaxMembers: 3, FileBytes: 1 << 20, TotalBytes: 2 << 20, MaxPathDepth: 32, Discovery: censusacquisition.DiscoveryLimits{MaxFiles: 2, MaxSymbols: 256, MaxDepth: 64, MaxStringBytes: 1 << 20}, MaxTargets: 63, Planning: censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: 1}, MaxNodes: 128, MaxRequests: 256, MaxEvidenceBytes: 4 << 20, MaxPathWork: 100000, TimeoutMS: 30000, RequestTimeoutMS: 5000, MaxResponseBytes: 2 << 20, MaxMessages: 64},
		Preparation: prep.Limits{MaxTargets: 64, MaxPartBytes: 2 << 20, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20},
		Post:        prep.PostPreparationLimits{MaxConstituents: 64, MaxLedgerTargets: 4096, MaxSubmissions: 256, MaxReviews: 256, MaxClaimsPerItem: 16, MaxCitationsPerClaim: 32, MaxStringBytes: 64 << 10, MaxItemBytes: 1 << 20, MaxInputBytes: 16 << 20, MaxOutputBytes: 16 << 20, MaxEvidenceRecords: 262144, MaxWork: 10000000}, Seeds: []uint64{42, 43}}
}
func inventoryFixturePrivate(t *testing.T) (string, string, string, []string) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	// Normal fixtures never depend on or create a personal runtime/evidence root.
	// Explicit historical qualification retains its original opt-in location.
	parent := filepath.Join(home, ".local/state/lsp-trace-evidence")
	if os.Getenv("LSP_TRACE_RETAIN_CENSUS_TEST_EVIDENCE") != "1" {
		parent, e = filepath.EvalSymlinks(home)
		if e != nil {
			t.Fatal(e)
		}
	}
	base, e := os.MkdirTemp(parent, "census-cli-owned-")
	if e != nil {
		t.Fatal(e)
	}
	// Retain test outputs when explicitly requested for this qualification. Normal
	// tests clean their own namespaces; historical attempts are never overwritten.
	if os.Getenv("LSP_TRACE_RETAIN_CENSUS_TEST_EVIDENCE") != "1" {
		t.Cleanup(func() { os.RemoveAll(base) })
		if e = pilot.ValidateDurableRoot(base); e != nil {
			t.Fatal("fixture root not admitted", e)
		}
	}
	source, root := filepath.Join(base, "source"), filepath.Join(base, "publication")
	for _, p := range []string{source, root} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"a.go", "b.go", "c.go", "excluded.go"} {
		body := "package owned\n\nfunc A0() {}\n"
		if name == "b.go" {
			body = "package owned\n\nvar V0 = 1\n"
		}
		if e = os.WriteFile(filepath.Join(source, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	config := filepath.Join(base, "config.json")
	saveInventoryConfigPrivate(t, config, inventoryConfigPrivate())
	args := []string{"--workspace", source, "--server", "owned", "--source", ".", "--exclude", "excluded.go", "--publication-root", root, "--machine", "--private-enumeration-projection", "--private-inventory-config", config, "--batch-targets", "1", "--down-depth", "1", "--up-depth", "1", "--max-nodes", "128", "--timeout", "30s", "--request-timeout", "5s"}
	return base, source, config, args
}
func saveInventoryConfigPrivate(t *testing.T, path string, c censusInventoryConfigPrivate) {
	t.Helper()
	b, _ := json.Marshal(c)
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func runInventoryCLIPrivate(t *testing.T, args []string, deps censusRunnerDependencies) (censusInventoryReportPrivate, int) {
	t.Helper()
	var out, errout bytes.Buffer
	code := runCensusWithDependencies(args, &out, &errout, deps)
	var meta struct{ AttemptDirectory string }
	if json.Unmarshal(out.Bytes(), &meta) != nil || meta.AttemptDirectory == "" {
		t.Fatalf("missing private result: code=%d out=%s err=%s", code, out.String(), errout.String())
	}
	raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	var report censusInventoryReportPrivate
	if json.Unmarshal(raw, &report) != nil {
		t.Fatal("report decode")
	}
	t.Logf("private attempt: %s status=%s failure=%s code=%d", meta.AttemptDirectory, report.Status, report.Failure, code)
	return report, code
}
func assertInventoryPrivate(t *testing.T, name string, ok bool) {
	t.Helper()
	if !ok {
		t.Errorf("%s: FAIL", name)
	} else {
		t.Logf("%s: PASS", name)
	}
}

func TestCensusPrivateInventoryCLI(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	deps, starts := inventoryDepsPrivate(t)
	r, code := runInventoryCLIPrivate(t, args, deps)
	t.Log("procedure: runCensusWithDependencies -> real enumeration/discovery/guarded original capture -> per-batch computation -> own-AST preparation -> native pair")
	assertInventoryPrivate(t, "ASSERT_CENSUS_APPLICATION_DENOMINATOR", len(r.Files) == 4 && len(r.Origins) == 1 && r.Files[0].File.Path == "a.go" && r.Files[1].File.Path == "b.go" && r.Files[2].Disposition == "RESOURCE_GAP_ORIGIN_LIMIT" && r.Files[3].Disposition == "EXCLUDED" && len(r.Origins[0].Discovery) > 0)
	var second censusacquisition.Discovery
	_ = json.Unmarshal(r.Origins[0].Discovery, &second)
	assertInventoryPrivate(t, "ASSERT_CENSUS_NONCALLABLE_NOT_DROPPED", len(second.EnumerationSymbols) == 2 && string(second.EnumerationSymbols[1].Disposition) == "non-callable")
	assertInventoryPrivate(t, "ASSERT_CENSUS_PRIVATE_PREPARATION_PAIR", code == 0 && r.Status == "PENDING_INTERPRETATION_PRIVATE_V2" && len(r.Index) == 1 && r.Index[0].Status == "PREPARED" && r.Outcome != nil && r.Outcome.Status == "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" && r.StopCalls == 1 && r.StopFailure == "" && r.ShutdownCalls == 1 && r.ShutdownJoined && r.CleanupVerified && starts.Load() == 1)
	assertInventoryPrivate(t, "ASSERT_CENSUS_PRIVATE_FAMILY", r.Profile == censusInventoryProfilePrivate && r.InputFamily == "PRIVATE_AST" && !r.Qualified && !r.Accepted && r.Authority == 0 && r.FeatureIdentity == "UNRESOLVED" && r.SourceCompleteness == "UNKNOWN" && r.AbsenceInference == "PROHIBITED" && r.Inventory == nil)
}

type inventorySemanticFixturePrivate struct{ id string }

func (b inventorySemanticFixturePrivate) Identity() string { return b.id }
func (b inventorySemanticFixturePrivate) Exchange(ctx context.Context, r prep.AutomaticRequestPrivateV2, cap int) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var v any
	switch r.Stage {
	case "INTERPRET":
		body := r.Bodies[0]
		v = prep.AutomaticInterpretationPrivateV2{Disposition: "DESCRIBED", Label: "owned no-op", Description: "Declares an empty function body.", Limitations: "Owned mechanical fixture only; not semantic qualification or runtime evidence", Alternatives: "May be unused", RejectionCondition: "The own body contains executable work", Claims: []prep.AutomaticClaimPrivateV2{{Text: "The retained own declaration has an empty body", Citations: []prep.ExactQuotePrivateV2{{Handle: body.Handle, Start: 0, End: len(body.Body), Quote: body.Body}}}}}
	case "NAVIGATE":
		v = []prep.NavigationPrivateV2{{ID: "owned", Candidates: []string{r.Candidates[0].ID}, Rationale: "Owned navigation fixture, not feature identity", WithdrawalCondition: "A stakeholder rejects it"}}
	case "REVIEW_TECHNICAL", "REVIEW_NAVIGATION":
		v = prep.AutomaticReviewPrivateV2{Digest: r.RevisionDigest, Verdict: "APPROVE", Findings: "Owned fixture checks exact pins only; unauthenticated independence"}
	default:
		return nil, errors.New("unexpected fixture stage")
	}
	return json.Marshal(v)
}
func TestCensusPrivateInventoryOwnedSemanticContinuation(t *testing.T) {
	base, _, config, args := inventoryFixturePrivate(t)
	deps, _ := inventoryDepsPrivate(t)
	first, code := runInventoryCLIPrivate(t, args, deps)
	if code != 0 {
		t.Fatal("first preparation failed", first.Failure)
	}
	c := inventoryConfigPrivate()
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(base, "mailbox"), Invocation: "owned-cli", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: first.ScopeSHA256, TimeoutMS: 60000, OverallTimeoutMS: 180000, PollMS: 10, Limits: prep.AutomaticLimitsPrivateV2{MaxCorrections: 1, MaxCalls: 12, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}}
	if e := os.Mkdir(c.Automation.Directory, 0700); e != nil {
		t.Fatal(e)
	}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	deps, _ = inventoryDepsPrivate(t)
	exchanges := 0
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, err io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.automatic = func(a prep.AutomaticHandoffConfigPrivateV2, scope string, h prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
			exchanges++
			return inventorySemanticFixturePrivate{"owned-interpreter"}, inventorySemanticFixturePrivate{"owned-reviewer"}, nil
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, err, d)
	}
	r, code := runInventoryCLIPrivate(t, args, deps)
	paired := false
	if r.Outcome != nil {
		var previous []byte
		var identities [2]os.FileInfo
		paired = true
		for i := 0; i < 2; i++ {
			path := filepath.Join(r.AttemptDirectory, fmt.Sprintf("inventory-%d", i))
			root, e := publication.OpenContentRetentionRoot(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := root.ReadPostPreparationPrivateV2(context.Background(), r.Outcome.EnvelopeSHA256, int64(r.Outcome.ByteLength))
			ce := root.Close()
			var envelope struct {
				Inventory      *prep.AutomaticInventoryPrivateV2
				HumanInventory string
			}
			if e != nil || ce != nil || json.Unmarshal(raw, &envelope) != nil || envelope.Inventory == nil || len(envelope.Inventory.Candidates) != 1 || len(envelope.Inventory.Navigation) != 1 || !strings.Contains(envelope.HumanInventory, "Declares an empty function body") {
				paired = false
			}
			if i > 0 && !bytes.Equal(previous, raw) {
				paired = false
			}
			previous = raw
			entries, _ := os.ReadDir(path)
			if len(entries) != 1 {
				paired = false
			} else {
				identities[i], _ = os.Stat(filepath.Join(path, entries[0].Name()))
			}
		}
		if identities[0] == nil || identities[1] == nil || os.SameFile(identities[0], identities[1]) {
			paired = false
		}
	}
	assertInventoryPrivate(t, "ASSERT_CENSUS_EXISTING_SEMANTIC_RETENTION", code == 0 && r.Inventory != nil && r.Inventory.Status == "PROVISIONAL_COMPLETE" && len(r.Inventory.Candidates) == 1 && len(r.Inventory.Navigation) == 1 && paired && exchanges == 1 && r.AttemptDirectory != first.AttemptDirectory)
	// Same paths and approval, different exact source bytes: no semantic adapter.
	if e := os.WriteFile(filepath.Join(filepath.Dir(config), "source", "a.go"), []byte("package owned\n\nfunc A0() { /* changed */ }\n"), 0600); e != nil {
		t.Fatal(e)
	}
	c.Automation.Directory = filepath.Join(base, "unapproved-mailbox")
	c.Automation.Invocation = "owned-unapproved"
	if e := os.Mkdir(c.Automation.Directory, 0700); e != nil {
		t.Fatal(e)
	}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	deps, starts := inventoryDepsPrivate(t)
	deps.runPrivateInventory = func(o censusCLIOptions, cfg censusCoreConfig, out, err io.Writer) int {
		d := productionCensusInventoryDependenciesPrivate()
		d.automatic = func(prep.AutomaticHandoffConfigPrivateV2, string, prep.PostPreparationPrivateV2) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
			t.Error("unapproved packet sent")
			return nil, nil, errors.New("unapproved")
		}
		return runCensusInventoryWithDependenciesPrivate(o, cfg, out, err, d)
	}
	changed, code := runInventoryCLIPrivate(t, args, deps)
	assertInventoryPrivate(t, "ASSERT_CENSUS_DISCLOSURE_HASH_BOUND", code == 1 && changed.Failure == "DISCLOSURE_SCOPE_MISMATCH" && changed.ScopeSHA256 != first.ScopeSHA256 && starts.Load() == 0)
}
func TestCensusPrivateInventoryGrammarAndCompatibility(t *testing.T) {
	_, _, _, args := inventoryFixturePrivate(t)
	for _, extra := range [][]string{{"--catalog"}, {"--resume", "g-" + strings.Repeat("a", 64) + ".selector.json"}, {"--private-inventory-config", "another"}} {
		_, e := parseCensusCLIOptions(append(append([]string{}, args...), extra...))
		if e == nil {
			t.Errorf("private incompatible syntax admitted: %v", extra)
		}
	}
	var out, err bytes.Buffer
	code := runCensus([]string{"--help"}, &out, &err)
	// Public help is intentionally owner-default now; this is a display contract,
	// not a source-qualified provenance pin to regenerate.
	assertInventoryPrivate(t, "ASSERT_CENSUS_ORDINARY_HELP_CONTRACT", code == 0 && err.Len() == 0 && out.String() == censusUsage+"\n" && strings.Contains(out.String(), "does not return a completed inventory") && strings.Contains(out.String(), "--storage-bytes BYTES") && strings.Contains(out.String(), censusLegacyInvocationUsage) && !strings.Contains(out.String(), "private-inventory"))
	c := inventoryConfigPrivate()
	c.Profile = "CALLS_ONLY_FROZEN_CENSUS_V1"
	_, _, config, _ := inventoryFixturePrivate(t)
	saveInventoryConfigPrivate(t, config, c)
	_, e := loadCensusInventoryConfigPrivate(config)
	assertInventoryPrivate(t, "ASSERT_CENSUS_QUALIFIED_PROFILE_NOT_RELABELED", e != nil)
}
