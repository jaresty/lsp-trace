package inventorytracediag

import (
	"bytes"
	"go/ast"
	"os"
	"path/filepath"
	"testing"
)

type currentSite struct {
	fn, code, ctx, guard, next, prev, prior, priorCall string
	region                                             bool
}

var mixedSites = []currentSite{
	{fn: "ExecuteMixedInputV2", code: "Execute", ctx: "ctx", region: true, next: "execution, cancel := context.WithDeadline(ctx, q.Start.Deadline)"},
	{fn: "ExecuteMixedInputV2", code: "LeaseCallback", ctx: "base", region: true, prev: "checkpoints.enter()"},
	{fn: "ExecuteMixedInputV2", code: "DeferredFinish", ctx: "ctx", next: "_, finishErr := q.Manager.FinishPrivateProviderWindow(window)"},
	{fn: "ExecuteMixedInputV2", code: "DeferredFinishDone", ctx: "ctx", prev: "operation.abortFixed()"},
	{fn: "ExecuteMixedInputV2", code: "CaptureRefused", ctx: "base", guard: "e != nil", next: "return e", prior: "checkpoints.captured(captureResult)"},
	{fn: "ExecuteMixedInputV2", code: "NativeCallsRefused", ctx: "base", guard: "e != nil", next: "return e", prior: "checkpoints.called(e)"},
	{fn: "ExecuteMixedInputV2", code: "IssueRefused", ctx: "base", guard: "e != nil", prior: "checkpoints.issued(e)"},
	{fn: "ExecuteMixedInputV2", code: "LedgerUnavailable", ctx: "ctx", guard: `out.Dispositions[method] == "UNAVAILABLE"`, next: "return fail(ErrPrivateMixedInputUnavailableV2)"},
	{fn: "ExecuteMixedInputV2", code: "LedgerEmpty", ctx: "ctx", guard: "len(out.Ledger) == 0", next: "return fail(ErrPrivateMixedInputEmptyV2)"},
	{fn: "ExecuteMixedInputV2", code: "Confirmed", ctx: "ctx", prev: "out.sourceConfirmed = true"},
}
var managedSites = []currentSite{
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "Issue", ctx: "ctx", region: true, prev: "@first"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "OwnedDeadlineRefused", ctx: "ctx", guard: "!q.Deadline.Equal(v.Deadline)", next: "return ErrPrivateManagedGrouping"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "OwnedIdentityRefused", ctx: "ctx", guard: "v.Workspace != q.Session.Workspace || v.Query.SessionID != q.Session.SessionID || v.Query.Generation != q.Session.Generation || v.Query.URI != q.URI || v.Query.OccurrenceID != q.OccurrenceID || v.Query.Line != q.Line || v.Query.Character != q.Character || v.Revision != q.Revision || v.Privacy != q.Privacy", next: "return ErrPrivateManagedGrouping"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "OwnedTargetRefused", ctx: "ctx", guard: "err != nil || target != v.Target || v.Symbol.Source == nil || v.Query.SourceDigest != v.Symbol.Source.SHA256 || v.Query.SourceDigest != privateGroupingDigest(v.Source)", next: "return ErrPrivateManagedGrouping"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "BindOwnedRefused", ctx: "ctx", guard: "err := bindGroupingProducerSourceV1(q.Operation, h, detached, v.Query.SourceDigest); err != nil", next: "return err"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "QueryBytesRefused", ctx: "ctx", guard: "err != nil", next: "return err", prior: "queryOriginal, err := privateGroupingQueryBytes(v)"},
	{fn: "privateGroupingBuild", code: "Build", ctx: "context.Background()", region: true, prev: "@first"},
	{fn: "issuePrivateManagedGroupingOwnedV1", code: "BuildRefused", ctx: "ctx", guard: "err != nil", next: `return fmt.Errorf("private grouping build: %w", err)`, prior: "out.sourceQueryOriginal = queryOriginal"},
	{fn: "privateGroupingBuild", code: "TypedCallsRefused", ctx: "context.Background()", guard: "err != nil", next: "return PrivateManagedGroupingInput{}, err", prior: "typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Kinds: q.Kinds, Calls: q.Calls})"},
	{fn: "privateGroupingBuild", code: "TypedMethodsRefused", ctx: "context.Background()", guard: "err != nil", next: "return PrivateManagedGroupingInput{}, err", prior: "typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: methods, EnableMethods: true, Kinds: q.Kinds, Calls: q.Calls})"},
	{fn: "privateGroupingAssemble", code: "Assemble", ctx: "context.Background()", region: true, prev: "@first"},
	{fn: "privateGroupingAssemble", code: "ProjectionRefused", ctx: "context.Background()", guard: "err != nil", next: "return PrivateManagedGroupingInput{}, err", prior: "projection, err := programcadmission.PreparePrivateLeidenProjection(out.TypedInput)"},
	{fn: "privateGroupingAssemble", code: "BuildInputRefused", ctx: "context.Background()", guard: "err != nil", next: "return PrivateManagedGroupingInput{}, err", priorCall: "referencegrouping.BuildInput"},
	{fn: "privateGroupingAssemble", code: "ParseAdmitRefused", ctx: "context.Background()", guard: "err != nil", next: "return PrivateManagedGroupingInput{}, err", prior: "out.Input, err = referencegrouping.ParseAndAdmit(artifact.CanonicalBytes(), out.Predecessors, privateGroupingParseLimits())"},
}
var providerSites = []currentSite{
	{fn: "runPrivateProviderWindowReadiness", code: "ReadinessRegion", ctx: "parent", region: true, prev: "@first"},
	{fn: "runPrivateProviderWindowReadiness", code: "ReadinessRefused", ctx: "parent", next: "owner.retire()", prev: "@first"},
	{fn: "runPrivateProviderWindowReadiness", code: "CapabilityUnsupported", ctx: "parent", guard: "!metadata.DocumentSymbolSupport || w.spec.plan.Calls && !metadata.CallHierarchySupport || w.spec.plan.References && !metadata.ReferencesSupport || w.spec.plan.Definitions && !metadata.DefinitionSupport", next: "failureKind = session.ToolNotImplemented"},
}
var referenceSites = []currentSite{
	{fn: "validateRequiredPredecessors", code: "NativeProofWithoutCalls", ctx: "context.Background()", guard: `!required["RETAINED_COMPOSITE"]`, next: `return errors.New("native proof without calls")`},
}

func (s currentSite) statement() string {
	if s.region {
		return "defer inventorytracediag.Begin(" + s.ctx + ", inventorytracediag." + s.code + ").End()"
	}
	return "inventorytracediag.Event(" + s.ctx + ", inventorytracediag." + s.code + ")"
}

// Bounded AST contract: exact calls, named owner, immediate native neighbors,
// and direct return-owning conditions. This is not a runtime emission observer.
func currentSiteOK(f *ast.File, s currentSite) bool {
	refs, err := importReferences(f, "lsp-trace/internal/inventorytracediag")
	if err != nil || len(refs) == 0 {
		return false
	}
	for sel := range refs {
		sel.X.(*ast.Ident).Name = "inventorytracediag"
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != s.fn {
			continue
		}
		guards := map[*ast.BlockStmt]string{}
		priors := map[*ast.BlockStmt]ast.Stmt{}
		ast.Inspect(fn, func(n ast.Node) bool {
			if b, ok := n.(*ast.BlockStmt); ok {
				for i, st := range b.List {
					if in, ok := st.(*ast.IfStmt); ok {
						g := render(in.Cond)
						if in.Init != nil {
							g = render(in.Init) + "; " + g
						}
						guards[in.Body] = g
						if i > 0 {
							priors[in.Body] = b.List[i-1]
						}
					}
				}
			}
			return true
		})
		uses := 0
		ast.Inspect(fn, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && len(c.Args) == 2 && (selector(c.Fun, "inventorytracediag", "Event") || selector(c.Fun, "inventorytracediag", "Begin")) && selector(c.Args[1], "inventorytracediag", s.code) {
				uses++
			}
			return true
		})
		if uses != 1 {
			return false
		}
		found := false
		ast.Inspect(fn, func(n ast.Node) bool {
			b, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			if s.guard != "" && guards[b] != s.guard {
				return true
			}
			if s.prior != "" && (priors[b] == nil || render(priors[b]) != s.prior) {
				return true
			}
			if s.priorCall != "" {
				a, ok := priors[b].(*ast.AssignStmt)
				if !ok || len(a.Rhs) != 1 {
					return true
				}
				c, ok := a.Rhs[0].(*ast.CallExpr)
				if !ok || render(c.Fun) != s.priorCall {
					return true
				}
			}
			for i, st := range b.List {
				if render(st) != s.statement() || !diagnosticSyntax(st) {
					continue
				}
				if s.prev == "@first" && i != 0 {
					continue
				}
				if s.prev != "" && s.prev != "@first" && (i == 0 || render(b.List[i-1]) != s.prev) {
					continue
				}
				if s.next != "" && (i+1 == len(b.List) || render(b.List[i+1]) != s.next) {
					continue
				}
				found = true
			}
			return true
		})
		return found
	}
	return false
}
func TestTraceSiteDiscriminators(t *testing.T) {
	disabled(t)
	s := mixedSites[0]
	bad := parseCurrent(t, []byte(`package p;import "lsp-trace/internal/inventorytracediag";func ExecuteMixedInputV2(){ _ = inventorytracediag.Execute }`))
	if currentSiteOK(bad, s) {
		t.Fatal("CURRENT_SITE_SELECTOR_ONLY_ACCEPTED")
	}
}
func TestTraceCurrentSites(t *testing.T) {
	disabled(t)
	groups := [][]currentSite{mixedSites, managedSites, providerSites, referenceSites}
	for i, sites := range groups {
		src, e := os.ReadFile(filepath.Join("../..", currentPaths[i]))
		if e != nil {
			t.Fatal(e)
		}
		f := parseCurrent(t, src)
		for _, s := range sites {
			if !currentSiteOK(f, s) {
				t.Errorf("CURRENT_SITE %s %s", s.fn, s.code)
			}
		}
	}
}

// Every required live call must be rejected when removed, replaced by a selector
// reference, or detached into a different function. Mutations are AST inputs only.
func TestTraceCurrentSitePerturbations(t *testing.T) {
	disabled(t)
	groups := [][]currentSite{mixedSites, managedSites, providerSites, referenceSites}
	for i, sites := range groups {
		src, e := os.ReadFile(filepath.Join("../..", currentPaths[i]))
		if e != nil {
			t.Fatal(e)
		}
		for _, s := range sites {
			t.Run(s.code, func(t *testing.T) {
				needle := []byte(s.statement())
				if bytes.Count(src, needle) != 1 {
					t.Fatalf("CURRENT_SITE_MUTATION_SETUP %s", s.code)
				}
				for name, replacement := range map[string][]byte{"removed": nil, "selector": []byte("_ = inventorytracediag." + s.code), "detached": nil} {
					changed := bytes.Replace(src, needle, replacement, 1)
					if name == "detached" {
						changed = append(changed, []byte("\nfunc detached23(){"+s.statement()+"}\n")...)
					}
					if currentSiteOK(parseCurrent(t, changed), s) {
						t.Errorf("CURRENT_SITE_%s_ACCEPTED %s", name, s.code)
					}
				}
			})
		}
	}
}
