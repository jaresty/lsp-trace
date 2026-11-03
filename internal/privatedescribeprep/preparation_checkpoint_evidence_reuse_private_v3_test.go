//go:build darwin

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

// Test-local legacy composition: the old Validate + second Read, then exactly
// the admitted header/target state. It introduces no production legacy API.
func legacyASTEvidenceReusePrivateV3(t *testing.T, ctx context.Context, p derived.ProjectionRetainedCheckpointPrivateV3, a derived.AssessmentRetainedCheckpointPrivateV3, limits Limits, grant pilot.PageCostPrivateV3) OwnASTReaderRetainedCheckpointPrivateV3 {
	t.Helper()
	before := p.BudgetSpent()
	_, assessment, e := derived.ValidateInstabilityRetainedCheckpointPrivateV3(ctx, p, a, grant)
	if e != nil {
		t.Fatal(e)
	}
	left, e := pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), grant)
	if e != nil {
		t.Fatal(e)
	}
	v, e := p.Read(ctx, left)
	if e != nil {
		t.Fatal(e)
	}
	r := OwnASTReportRetainedCheckpointPrivateV3{RequestType: RequestTypeRetainedCheckpointPrivateV3, Source: v.Source, AssessmentSHA256: pilot.Digest(assessment), Assessment: assessment, Limits: limits, FullTargetCount: v.Source.FullTargetCount, FullBatchCount: v.Source.FullBatchCount, BatchTargetCount: v.Source.BatchTargetCount, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	targets := v.Selected.Source.Evidence.Acquisition.Targets
	if len(targets) > limits.MaxTargets {
		t.Fatal("legacy target bound")
	}
	maxBytes := limits.MaxPartBytes
	if limits.MaxTotalBytes < maxBytes {
		maxBytes = limits.MaxTotalBytes
	}
	if int(v.Selected.Revision.Policy.ComputationUnitBytes) < maxBytes {
		maxBytes = int(v.Selected.Revision.Policy.ComputationUnitBytes)
	}
	r.SourceInputID = pilot.Digest(nil)
	if e = admitOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes); e != nil {
		t.Fatal(e)
	}
	r.SourceInputID = digest(struct {
		Request    string
		Source     derived.SourceBindingRetainedCheckpointPrivateV3
		Assessment string
	}{RequestTypeRetainedCheckpointPrivateV3, v.Source, r.AssessmentSHA256})
	header, e := marshalOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes, json.Marshal)
	if e != nil {
		t.Fatal(e)
	}
	s := &ownASTCheckpointStatePrivateV3{projection: p, canonical: v.DerivedCanonicalV5, targets: targets, report: r, maxBytes: maxBytes, reservedBytes: len(header) + len(assessment) + 256}
	s.self = s
	return OwnASTReaderRetainedCheckpointPrivateV3{s}
}

func TestRetainedCheckpointPrivateV3ASTEvidenceReuse(t *testing.T) {
	ctx := context.Background()
	grant := pilot.PageCostPrivateV3{LogicalBytes: 1 << 40, ReadHashBytes: 1 << 40, WriteBytes: 1 << 34, Work: 1 << 32}
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(dir, "controlled.test")
	path := filepath.Join(dir, "transfer.json")
	args := []string{"test", "-c", "-o", exe, "../censuslineagepilot"}
	if os.Getenv("LSP_COST_CHILD_RACE") == "1" {
		args = append(args, "-race")
	}
	compile := exec.Command("go", args...)
	for _, v := range os.Environ() {
		key := strings.SplitN(v, "=", 2)[0]
		switch key {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOWORK", "GOFLAGS":
		default:
			compile.Env = append(compile.Env, v)
		}
	}
	compile.Env = append(compile.Env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly")
	if out, e := compile.CombinedOutput(); e != nil {
		t.Fatalf("AST_FIXTURE_COMPILE %v %s", e, out)
	}
	child := exec.Command(exe, "-test.run=^TestRetainedCheckpointPrivateV3ProcessChild$", "-test.v")
	child.Env = append(compile.Env, "TMPDIR="+dir, "LSP_RETAINED_CHECKPOINT_PROCESS=A", "LSP_RETAINED_CHECKPOINT_TRANSFER="+path)
	if out, e := child.CombinedOutput(); e != nil || !bytes.Contains(out, []byte("ASSERT_RETAINED_PROCESS_A_ORIGINAL_HANDLES_RELEASED_BY_EXIT: PASS")) {
		t.Fatalf("AST_FIXTURE_EXPORT %v %s", e, out)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var x struct {
		Manifest   []byte
		Spec       pilot.ScopeSpecPrivateV2
		LedgerPath string
		Expected   [20][4]string
	}
	if e = json.Unmarshal(data, &x); e != nil {
		t.Fatal(e)
	}
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if e = json.Unmarshal(x.Manifest, &m); e != nil {
		t.Fatal(e)
	}
	bootstrap := grant
	l, e := pilot.OpenValidatedCurrentLedgerPrivateV3(ctx, x.LedgerPath, m.Application, grant, m.LedgerGeneration, m.LedgerTip, &bootstrap)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = l.Close() })
	f, e := pilot.FreezeRetainedCheckpointSourcePrivateV3(ctx, x.Spec, l, grant)
	if e != nil {
		t.Fatal(e)
	}
	var outputs [2]*publication.ContentRetentionRoot
	for i, pin := range m.Roots {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, pin.Path)
		if e != nil {
			t.Fatal(e)
		}
		r := outputs[i]
		t.Cleanup(func() { _ = r.Close() })
	}
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, x.Spec.Root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = source.Close() })
	roots, e := pilot.NewProducerPacketRootsPrivateV2(ctx, outputs, source, nil)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := pilot.AdmitRetainedPageCheckpointPrivateV3(ctx, x.Manifest, f, roots, l, pilot.RetainedCheckpointDisclosurePrivateV3{Application: m.Application, ConfigSHA256: m.Revision.Policy.ConfigSHA256, ContinuationIntent: m.Intent, PreparationRevision: pilot.RetainedCheckpointPreparationRevisionPrivateV3}, grant)
	if e != nil {
		t.Fatal(e)
	}
	b, e := owner.SelectedBatch(ctx, 0, grant)
	if e != nil {
		t.Fatal(e)
	}
	p, e := derived.ProjectRetainedCheckpointPrivateV3(ctx, b, grant)
	if e != nil {
		t.Fatal(e)
	}
	a, e := derived.ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, []uint64{3, 7}, grant)
	if e != nil {
		t.Fatal(e)
	}
	limits := Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	old := legacyASTEvidenceReusePrivateV3(t, ctx, p, a, limits, grant)
	current, e := NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, limits, grant)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(old.state.report, current.state.report) || !reflect.DeepEqual(old.state.targets, current.state.targets) || !bytes.Equal(old.state.canonical, current.state.canonical) || old.state.reservedBytes != current.state.reservedBytes {
		t.Fatal("AST_REUSE_HEADER_TARGETS")
	}
	if !bytes.Contains(current.state.canonical, []byte("1.000e+2")) {
		t.Fatal("AST_REUSE_NUMERIC_LEXEME")
	}
	_, legacyPrepared, e := old.Advance(ctx, 1, grant)
	if e != nil {
		t.Fatal(e)
	}
	_, prepared, e := current.Advance(ctx, 1, grant)
	if e != nil {
		t.Fatal(e)
	}
	legacyRaw, e := legacyPrepared.Read(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := prepared.Read(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(raw, legacyRaw) {
		t.Fatal("AST_REUSE_CANONICAL")
	}
	var report OwnASTReportRetainedCheckpointPrivateV3
	if e = json.Unmarshal(raw, &report); e != nil {
		t.Fatal(e)
	}
	if report.Authority != 0 || report.Accepted || report.FeatureIdentity != "UNRESOLVED" || report.SourceCompleteness != "UNKNOWN" || report.AbsenceInference != "PROHIBITED" || report.PreparedCount != 1 || len(report.Bodies) != 1 || report.Bodies[0].Declaration.Body != "func Owned00() {}" {
		t.Fatal("AST_REUSE_CEILING")
	}
	t.Logf("AST_REUSE_CANONICAL: A-pass identical legacy=%s new=%s bytes=%d targets=%d", pilot.Digest(legacyRaw), pilot.Digest(raw), len(raw), len(current.state.targets))
	// Canonical equality guard rejects a byte-level perturbation and accepts the
	// restored report. The same comparison is used above on actual outputs.
	mutated := bytes.Clone(raw)
	mutated[len(mutated)-1] ^= 1
	if bytes.Equal(mutated, legacyRaw) {
		t.Fatal("AST_REUSE_CANONICAL_CONTROL")
	}
	t.Log("AST_REUSE_CANONICAL: A-fail on mutated output")
	if !bytes.Equal(raw, legacyRaw) {
		t.Fatal("AST_REUSE_CANONICAL_RESTORE")
	}
	t.Log("AST_REUSE_CANONICAL: restored A-pass")
	for _, kind := range []string{"nil-context", "canceled", "zero-assessment", "zero-exchange", "invalid-limits"} {
		t.Run(kind, func(t *testing.T) {
			c := ctx
			assessment := a
			exchange := grant
			lim := limits
			switch kind {
			case "nil-context":
				c = nil
			case "canceled":
				var cancel context.CancelFunc
				c, cancel = context.WithCancel(ctx)
				cancel()
			case "zero-assessment":
				assessment = derived.AssessmentRetainedCheckpointPrivateV3{}
			case "zero-exchange":
				exchange = pilot.PageCostPrivateV3{}
			case "invalid-limits":
				lim = Limits{}
			}
			r, e := NewOwnASTReaderRetainedCheckpointPrivateV3(c, p, assessment, lim, exchange)
			if e == nil || r.state != nil {
				t.Fatal("AST_REUSE_REFUSE_NO_OWNER", kind, e)
			}
		})
	}
	// Constructors do not waive the existing Advance/Prepared.Read source checks.
	reader, e := NewOwnASTReaderRetainedCheckpointPrivateV3(ctx, p, a, limits, grant)
	if e != nil {
		t.Fatal(e)
	}
	sourcePath := filepath.Join(x.Spec.Root, x.Spec.Members[0].CanonicalRelativePath)
	original, e := os.ReadFile(sourcePath)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(sourcePath, bytes.Repeat([]byte("x"), len(original)), 0600); e != nil {
		t.Fatal(e)
	}
	_, zero, e := reader.Advance(ctx, 1, grant)
	if e == nil || zero.state != nil {
		t.Fatal("AST_REUSE_ADVANCE_FRESH", e)
	}
	if data, e := prepared.Read(ctx, grant); e == nil || data != nil {
		t.Fatal("AST_REUSE_PREPARED_FRESH", e)
	}
	if _, e = prepared.Publication(ctx, grant); e == nil {
		t.Fatal("AST_REUSE_PUBLICATION_FRESH")
	}
	if e = os.WriteFile(sourcePath, original, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = prepared.Read(ctx, grant); !errors.Is(e, pilot.ErrInvalid) {
		t.Fatal("AST_REUSE_CONTENT_RESTORE_NOT_AUTHORITY", e)
	}
}
