package describeworker

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/targetpacket"
)

func suppliedPlanFixture(t *testing.T) (*Runner, Config, describerequest.Record, SourceAcquisitionContext) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("NewRunner requires Darwin Preflight; compile-only elsewhere")
	}
	c := fixtureConfig(t)
	c.ResponseVersion = ResponseVersionV2
	c.Limits.WorkBytes, c.Limits.TempBytes, c.Limits.StdoutBytes = 1<<20, 1<<20, 512<<10
	r, err := NewRunner(c)
	if err != nil {
		t.Fatal(err)
	}
	u := targetpacket.PrivateSourceUnit{Version: targetpacket.PrivateSourceVersion, ClaimCeiling: targetpacket.PrivateSourceCeiling, ID: digestBytes([]byte("controlled unit")), SourceDigest: digestBytes([]byte("controlled body")), Body: "controlled body", URI: "file:///controlled.go"}
	record, err := describerequest.FormatPrivateSourceRequestCandidate(u, 1000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	s := SourceAcquisitionContext{HandoffID: "controlled-handoff", CheckpointID: "controlled-checkpoint", SourceUnitID: u.ID, SourceDigest: u.SourceDigest, Workspace: "controlled-workspace", PolicyID: "controlled-policy", Deadline: time.Now().Add(10 * time.Second), SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}, MaxWireBytes: 1 << 20, MaxParentBytes: 8 << 20}
	return r, c, record, s
}

func TestSuppliedPlan(t *testing.T) {
	r, c, q, s := suppliedPlanFixture(t)
	ctx := context.Background()
	p, err := r.PrepareSourceAcquisition(ctx, q, "controlled", s)
	if err != nil || p.ID() == "" {
		t.Fatal("ASSERT_PLAN_PREPARED", err)
	}
	upper, err := r.ValidateSourceAcquisition(ctx, p, q, "controlled", s)
	if err != nil || upper == 0 {
		t.Fatal("ASSERT_PLAN_VALID", err)
	}
	foreign, err := NewRunner(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = foreign.ValidateSourceAcquisition(ctx, p, q, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_OWNER")
	}
	t.Log("ASSERT_PLAN_OWNER: PASS")
	for _, mutate := range []func(*describerequest.Record){func(q *describerequest.Record) { q.Envelope.Prompt += "changed" }, func(q *describerequest.Record) { q.Lineage.Authority++ }, func(q *describerequest.Record) { q.Envelope.MessageID = "foreign" }, func(q *describerequest.Record) { q.RecordID = "foreign" }} {
		bad := q
		mutate(&bad)
		if _, err = r.ValidateSourceAcquisition(ctx, p, bad, "controlled", s); err == nil {
			t.Fatal("ASSERT_PLAN_REQUEST")
		}
	}
	t.Log("ASSERT_PLAN_REQUEST: PASS")
	badSource := s
	badSource.SourceUnitID = "foreign"
	if _, err = r.ValidateSourceAcquisition(ctx, p, q, "controlled", badSource); err == nil {
		t.Fatal("ASSERT_PLAN_SOURCE")
	}
	t.Log("ASSERT_PLAN_SOURCE: PASS")
	drift := s
	drift.Deadline = s.Deadline.Add(time.Millisecond)
	if p.state.source.Deadline != s.Deadline || p.state.config != c {
		t.Fatal("ASSERT_PLAN_ORIGINAL_SNAPSHOT")
	}
	if _, err = r.ValidateSourceAcquisition(ctx, p, q, "controlled", drift); err == nil {
		t.Fatal("ASSERT_PLAN_DEADLINE")
	}
	t.Log("ASSERT_PLAN_ORIGINAL_SNAPSHOT: PASS; ASSERT_PLAN_DEADLINE: PASS")
	if _, err = r.ValidateSourceAcquisition(ctx, p, q, "foreign", s); err == nil {
		t.Fatal("ASSERT_PLAN_ATTEMPT")
	}
	t.Log("ASSERT_PLAN_ATTEMPT: PASS")
	r.validated.config.RuntimeIdentity = "changed snapshot"
	_, err = r.ValidateSourceAcquisition(ctx, p, q, "controlled", s)
	r.validated.config = c
	if err == nil {
		t.Fatal("ASSERT_PLAN_CONFIG")
	}
	var public SourceAcquisitionPlan
	if json.Unmarshal([]byte(`{"state":{},"id":"public"}`), &public) != nil || public.ID() != "" {
		t.Fatal("ASSERT_PLAN_OPAQUE")
	}
	if _, err = r.ValidateSourceAcquisition(ctx, public, q, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_OPAQUE")
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = r.PrepareSourceAcquisition(stopped, q, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_CANCEL")
	}
	expired := s
	expired.Deadline = time.Now().Add(-time.Second)
	if _, err = r.PrepareSourceAcquisition(ctx, q, "controlled", expired); err == nil {
		t.Fatal("ASSERT_PLAN_EXPIRE")
	}
	if _, err = r.PrepareSourceAcquisition(ctx, q, strings.Repeat("x", 257), s); err == nil {
		t.Fatal("ASSERT_PLAN_ATTEMPT_BOUND")
	}
	for _, grow := range []func(*Config){
		func(c *Config) { c.RuntimeIdentity += "x" }, func(c *Config) { c.AdapterIdentity += "x" }, func(c *Config) { c.ModelIdentity += "x" },
		func(c *Config) { c.Worker.Path += "x" }, func(c *Config) { c.Model.Path += "x" }, func(c *Config) { c.Library.Path += "x" },
		func(c *Config) { c.SandboxExecutable.Path += "x" }, func(c *Config) { c.SandboxProfile.Path += "x" }, func(c *Config) { c.Grammar.Path += "x" },
	} {
		changed := c
		grow(&changed)
		r.validated.config = changed
		larger, e := r.SourceAcquisitionRetentionUpper(q, "controlled", s)
		r.validated.config = c
		if e != nil || larger != upper+6*4 {
			t.Fatal("ASSERT_CONFIG_RETENTION", larger, upper, e)
		}
	}
	r.validated.config.RuntimeIdentity = strings.Repeat("x", 1<<20)
	_, sizeErr := r.SourceAcquisitionRetentionUpper(q, "controlled", s)
	_, err = r.PrepareSourceAcquisition(ctx, q, "controlled", s)
	r.validated.config = c
	if err == nil || sizeErr == nil {
		t.Fatal("ASSERT_PLAN_CONFIG_BOUND")
	}
	t.Log("ASSERT_CONFIG_RETENTION: PASS; ASSERT_PLAN_CONFIG_BOUND: PASS")
	big := q
	big.Envelope.Prompt = strings.Repeat("x", 1<<20)
	if _, err = r.PrepareSourceAcquisition(ctx, big, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_RECORD_BOUND")
	}
	v1 := c
	v1.ResponseVersion = ResponseVersionV1
	old, err := NewRunner(v1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = old.PrepareSourceAcquisition(ctx, q, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_V2_ONLY")
	}
	var zero Runner
	if _, err = zero.PrepareSourceAcquisition(ctx, q, "controlled", s); err == nil {
		t.Fatal("ASSERT_PLAN_CONSTRUCTED")
	}
	if err = r.ValidateSourceAcquisitionResponseLimits(p, SuppliedResponseLimits{512 << 10, 64 << 10}); err != nil {
		t.Fatal("ASSERT_PLAN_LIMITS", err)
	}
	r.validated.config.Limits.StdoutBytes = 1024
	err = r.ValidateSourceAcquisitionResponseLimits(p, SuppliedResponseLimits{512 << 10, 64 << 10})
	r.validated.config = c
	if err == nil {
		t.Fatal("ASSERT_PLAN_LIMITS")
	}
	tightConfig := c
	tightConfig.Limits.StdoutBytes = 512
	tight, e := NewRunner(tightConfig)
	if e != nil {
		t.Fatal(e)
	}
	tightPlan, e := tight.PrepareSourceAcquisition(ctx, q, "controlled", s)
	if e != nil {
		t.Fatal(e)
	}
	if e = tight.ValidateSourceAcquisitionResponseLimits(tightPlan, SuppliedResponseLimits{512 << 10, 64 << 10}); e == nil {
		t.Fatal("ASSERT_PLAN_LIMIT_CEILING")
	}
	t.Log("ASSERT_PLAN_LIMIT_CEILING: PASS")
	if _, err = r.ValidateSourceAcquisition(ctx, p, q, "controlled", s); err != nil {
		t.Fatal("ASSERT_PLAN_RESTORE", err)
	}
	// Pin files may disappear AFTER construction. Preparation still describes
	// that original validated config; it must not read or claim current pin state.
	if err = os.Remove(c.SandboxProfile.Path); err != nil {
		t.Fatal(err)
	}
	if _, err = r.PrepareSourceAcquisition(ctx, q, "controlled", s); err != nil {
		t.Fatal("ASSERT_PLAN_NO_PIN_REREAD", err)
	}
	t.Log("ASSERT_PLAN_CONFIG: PASS; ASSERT_PLAN_OPAQUE: PASS; ASSERT_PLAN_CANCEL: PASS; ASSERT_PLAN_EXPIRE: PASS; ASSERT_PLAN_CONFIG_BOUND: PASS; ASSERT_PLAN_RECORD_BOUND: PASS; ASSERT_PLAN_V2_ONLY: PASS; ASSERT_PLAN_CONSTRUCTED: PASS; ASSERT_PLAN_LIMITS: PASS; ASSERT_PLAN_NO_PIN_REREAD: PASS; ASSERT_PLAN_RESTORE: PASS")
}

// Separate subtests make the tuple countercontrols assertion-specific: deleting
// one join must fail only its named subtest while the other joins still pass.
func TestSuppliedJoins(t *testing.T) {
	r, c, q, s := suppliedPlanFixture(t)
	p, e := r.PrepareSourceAcquisition(context.Background(), q, "controlled", s)
	if e != nil {
		t.Fatal(e)
	}
	foreign, e := NewRunner(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"owner", "request", "source", "attempt", "config"} {
		t.Run(name, func(t *testing.T) {
			owner, record, source, attempt := r, q, s, "controlled"
			switch name {
			case "owner":
				owner = foreign
			case "request":
				record.Lineage.Authority++
			case "source":
				source.SourceUnitID = "foreign"
			case "attempt":
				attempt = "foreign"
			case "config":
				r.validated.config.RuntimeIdentity = "foreign"
			}
			_, err := owner.ValidateSourceAcquisition(context.Background(), p, record, attempt, source)
			r.validated.config = c
			if err == nil {
				t.Fatal("ASSERT_JOIN_" + name)
			}
			t.Log("ASSERT_JOIN_" + name + ": PASS")
		})
	}
}

func TestSuppliedDisabled(t *testing.T) {
	r, _, q, s := suppliedPlanFixture(t)
	p, err := r.PrepareSourceAcquisition(context.Background(), q, "controlled", s)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(r.RunSourceAcquisition(nil, p), ErrSourceExecutionDisabled) || !errors.Is((*Runner)(nil).RunSourceAcquisition(nil, SourceAcquisitionPlan{}), ErrSourceExecutionDisabled) {
		t.Fatal("ASSERT_LIVE_DISABLED")
	}
	t.Log("ASSERT_LIVE_DISABLED: PASS")
	// Structural boundary guard: an unconditional disabled return with no IO or
	// receiver/context/plan inspection. Permit an ignored, constant filepath.Clean
	// as a pure lexical control, rather than misclassifying harmless work as IO.
	// This guard cannot itself start an executable, supervisor or model.
	f, err := parser.ParseFile(token.NewFileSet(), "private_source_acquisition.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "RunSourceAcquisition" {
			continue
		}
		found = true
		if len(fn.Body.List) == 0 {
			t.Fatal("ASSERT_LIVE_BEFORE_IO")
		}
		for _, statement := range fn.Body.List[:len(fn.Body.List)-1] {
			assign, ok := statement.(*ast.AssignStmt)
			if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			ignored, ok := assign.Lhs[0].(*ast.Ident)
			if !ok || ignored.Name != "_" {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Clean" {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "filepath" {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
			standardImport := false
			for _, spec := range f.Imports {
				if spec.Path.Value == `"path/filepath"` && (spec.Name == nil || spec.Name.Name == "filepath") {
					standardImport = true
				}
			}
			if !standardImport {
				t.Fatal("ASSERT_LIVE_BEFORE_IO")
			}
		}
		ret, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			t.Fatal("ASSERT_LIVE_BEFORE_IO")
		}
		id, ok := ret.Results[0].(*ast.Ident)
		if !ok || id.Name != "ErrSourceExecutionDisabled" {
			t.Fatal("ASSERT_LIVE_BEFORE_IO")
		}
	}
	if !found {
		t.Fatal("ASSERT_LIVE_BEFORE_IO")
	}
	t.Log("ASSERT_LIVE_DISABLED: PASS; ASSERT_LIVE_BEFORE_IO: PASS; starts=0")
	immutable := false
	for _, decl := range f.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, spec := range group.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range value.Names {
				if name.Name == "ErrSourceExecutionDisabled" {
					immutable = true
				}
			}
		}
	}
	if !immutable {
		t.Fatal("ASSERT_LIVE_IMMUTABLE_ERROR")
	}
	t.Log("ASSERT_LIVE_IMMUTABLE_ERROR: PASS")
}
