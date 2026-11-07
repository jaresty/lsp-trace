package sourcecontinuation

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	cc "lsp-trace/internal/censuscontinuation"
	inv "lsp-trace/internal/inventorysuccessor"
	"path/filepath"
	"runtime"
	"testing"
)

// This finite body guard supplements (not replaces) executed all-seven tests in
// inventorysuccessor's controlled SELF fixture. Initial producer paid admission
// is intentionally outside the guarded source-stage dependency closure.
func TestSourceStageNoNativeReplayCalls(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	forbidden := map[string]bool{"ParseAndAdmit": true, "ReplayPrivateNativeEvidenceV2": true, "Validate": true, "Compose": true, "CheckGroupingEvaluationPrivate": true, "EvaluateGroupingCasePrivate": true, "replayPrivate": true, "BuildDirectional": true}
	for _, path := range []string{"targetpacket/private_source_admission.go", "inventorysuccessor/private_source_originals_v1.go", "inventorysuccessor/private_source_admission_v1.go", "inventorysuccessor/sourcecontinuation/continuation.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch x := c.Fun.(type) {
			case *ast.SelectorExpr:
				name = x.Sel.Name
			case *ast.Ident:
				name = x.Name
			}
			if forbidden[name] {
				t.Errorf("ASSERT_SOURCE_STAGE_NO_NATIVE_REPLAY: %s calls %s", path, name)
			}
			return true
		})
	}
	if !t.Failed() {
		t.Log("ASSERT_SOURCE_STAGE_NO_NATIVE_REPLAY: PASS")
	}
}

type noPutStore struct{ puts int }

func (s *noPutStore) Put(context.Context, []byte) (string, error) { s.puts++; return "", nil }
func (s *noPutStore) Get(context.Context, string) ([]byte, error) { return nil, nil }
func (s *noPutStore) MaxObjectBytes() int64                       { return 8 << 20 }
func TestZeroUpperProducerCannotPrepareOrResume(t *testing.T) {
	if (inv.PrivateSourceAdmission{}).Valid() {
		t.Fatal("ASSERT_ZERO_UPPER_ADMISSION_REFUSED")
	}
	if _, err := (inv.PrivateSourceAdmission{}).Unit(); err == nil {
		t.Fatal("ASSERT_ZERO_UPPER_UNIT_REFUSED")
	}
	t.Log("ASSERT_ZERO_UPPER_ADMISSION_REFUSED: PASS")
}
func TestUpperResumeRequiresExpectedProducer(t *testing.T) {
	s := &noPutStore{}
	out := Resume(context.Background(), ResumeRequest{PrivateSources: &PrivateAllModeSourcesResume{Version: PrivateAllModeSourcesVersion}, Store: s, StopAfter: StopAfterPrivateSourceRequests})
	if out.Err == nil || out.PrivateSources != nil || s.puts != 0 {
		t.Fatal("ASSERT_UPPER_ZERO_EXPECTATION_REFUSED")
	}
	run := Run(context.Background(), Request{PrivateSources: &PrivateAllModeSourcesRequest{Originals: []inv.PrivateProducerOriginals{{}}}, PrivateAllMode: &cc.PrivateAllModeHandoff{}, Store: s, StopAfter: StopAfterPrivateSourceRequests})
	if run.Err == nil || run.PrivateSources != nil || s.puts != 0 {
		t.Fatal("ASSERT_UPPER_ZERO_ORIGINAL_REFUSED")
	}
	t.Log("ASSERT_UPPER_ZERO_EXPECTATION_REFUSED: PASS; ASSERT_UPPER_ZERO_ORIGINAL_REFUSED: PASS")
}
