//go:build darwin

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

type retainedProcessEnvelopeV3 struct {
	Manifest   []byte
	Spec       ScopeSpecPrivateV2
	LedgerPath string
	Expected   [20][4]string
}

func retainedProcessGrantV3() PageCostPrivateV3 {
	return PageCostPrivateV3{1 << 40, 1 << 40, 1 << 34, 1 << 32}
}
func TestRetainedCheckpointPrivateV3TwoProcesses(t *testing.T) {
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	transfer := filepath.Join(dir, "transfer.json")
	for _, role := range []string{"A", "B"} {
		cmd := exec.Command(exe, "-test.run=^TestRetainedCheckpointPrivateV3ProcessChild$", "-test.v")
		cmd.Env = []string{"LSP_RETAINED_CHECKPOINT_PROCESS=" + role, "LSP_RETAINED_CHECKPOINT_TRANSFER=" + transfer, "TMPDIR=" + dir, "HOME=" + dir}
		raw, e := cmd.CombinedOutput()
		t.Logf("PROCESS_%s:\n%s", role, raw)
		if e != nil {
			t.Fatalf("ASSERT_RETAINED_TWO_PROCESS_%s: %v", role, e)
		}
	}
	t.Log("ASSERT_RETAINED_TWO_PROCESS_SELECTED_TWENTY: PASS")
}
func TestRetainedCheckpointPrivateV3ProcessChild(t *testing.T) {
	role := os.Getenv("LSP_RETAINED_CHECKPOINT_PROCESS")
	if role == "" {
		t.Skip("owned subprocess only")
	}
	ctx := context.Background()
	grant := retainedProcessGrantV3()
	path := os.Getenv("LSP_RETAINED_CHECKPOINT_TRANSFER")
	if role == "A" {
		p, r := ControlledTwentyPagesPrivateV3(t, grant)
		raw, e := p.ExportRetainedCheckpointPrivateV3(ctx, r, Digest([]byte("controlled-retained-continuation")), grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PROCESS_EXPORT", e)
		}
		x := retainedProcessEnvelopeV3{Manifest: raw, Spec: p.state.frozen.state.spec, LedgerPath: p.state.ledger.state.path}
		for i, c := range p.state.captures {
			for j, b := range [][]byte{c.state.seeds, c.state.origin, c.state.artifact, p.state.origin.state.batches[i].binding.Input} {
				x.Expected[i][j] = Digest(b)
			}
		}
		out, _ := json.Marshal(x)
		if e = os.WriteFile(path, out, 0600); e != nil {
			t.Fatal(e)
		}
		file, e := os.OpenFile(path, os.O_WRONLY, 0)
		if e != nil {
			t.Fatal(e)
		}
		if e = file.Sync(); e != nil {
			t.Fatal(e)
		}
		file.Close()
		// Real process exit drops EVERY original opaque object. Test-only temp roots
		// remain under the parent's owned scratch directory, cleaned by that parent.
		fmt.Println("ASSERT_RETAINED_PROCESS_A_ORIGINAL_HANDLES_RELEASED_BY_EXIT: PASS")
		os.Exit(0)
	}
	if role != "B" {
		t.Fatal("unknown controlled role")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if len(raw) > 128<<10 {
		t.Fatal("transfer bound")
	}
	var x retainedProcessEnvelopeV3
	if e = json.Unmarshal(raw, &x); e != nil {
		t.Fatal(e)
	}
	var m RetainedPageCheckpointManifestPrivateV3
	if e = canonicalRetainedPrivateV3(x.Manifest, &m); e != nil {
		t.Fatal(e)
	}
	l, e := openRetainedTestLedgerPrivateV3(ctx, x.LedgerPath, m.Application, grant, m.LedgerGeneration, m.LedgerTip)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	f, e := FreezeRetainedCheckpointSourcePrivateV3(ctx, x.Spec, l, grant)
	if e != nil {
		t.Fatal(e)
	}
	var outputs [2]*publication.ContentRetentionRoot
	for i, v := range m.Roots {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, v.Path)
		if e != nil {
			t.Fatal(e)
		}
		defer outputs[i].Close()
	}
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, x.Spec.Root)
	if e != nil {
		t.Fatal(e)
	}
	defer source.Close()
	roots, e := NewProducerPacketRootsPrivateV2(ctx, outputs, source, nil)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := AdmitRetainedPageCheckpointPrivateV3(ctx, x.Manifest, f, roots, l, RetainedCheckpointDisclosurePrivateV3{m.Application, m.Revision.Policy.ConfigSHA256, m.Intent, RetainedCheckpointPreparationRevisionPrivateV3}, grant)
	if e != nil {
		t.Fatal("ASSERT_RETAINED_PROCESS_ADMISSION", e)
	}
	if owner.Profile() != RetainedCheckpointProfilePrivateV3 {
		t.Fatal("ASSERT_RETAINED_NEW_INPUT_FAMILY")
	}
	var first []byte
	for i := 0; i < 20; i++ {
		b, e := owner.SelectedBatch(ctx, i, grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_SELECTED_TWENTY", i, e)
		}
		v, e := b.Read(ctx, grant)
		if e != nil {
			t.Fatal(e)
		}
		if v.Profile != RetainedCheckpointProfilePrivateV3 || v.ObjectSHA256 != x.Expected[i] || v.Authority != 0 || v.Accepted || v.SourceCompleteness != "UNKNOWN" || v.FeatureIdentity != "UNRESOLVED" || v.AbsenceInference != "PROHIBITED" || !bytes.Contains(v.Source.Evidence.GraphBytes, []byte("1.000e+2")) {
			t.Fatal("ASSERT_RETAINED_EXACT_OBSERVATIONS", i)
		}
		target := v.Source.Evidence.Acquisition.Targets[0].Resolution.Identity
		if target == nil {
			t.Fatal("missing controlled target")
		}
		body, e := b.OwnSource(ctx, target.URI, grant)
		if e != nil || !bytes.Contains(body, []byte(fmt.Sprintf("func Owned%02d() {}", i))) {
			t.Fatal("ASSERT_RETAINED_FRESH_OWN_SOURCE", i, e)
		}
		if i == 0 {
			first = bytes.Clone(v.SourceArtifact)
		}
		next, e := owner.CheckpointCompletedBatch(ctx, b, Digest([]byte(fmt.Sprintf("next-%d", i))), grant)
		if e != nil {
			t.Fatal("ASSERT_RETAINED_PROGRESS_CHECKPOINT", i, e)
		}
		if canonicalRetainedPrivateV3(next, &m) != nil || m.NextBatch != i+1 || m.Generation != uint64(i+2) {
			t.Fatal("ASSERT_RETAINED_PROGRESS", i)
		}
		if i == 19 {
			prior, e := owner.SelectedObject(ctx, 7+m.Members+2, grant)
			if e != nil || !bytes.Equal(prior, first) {
				t.Fatal("ASSERT_RETAINED_PRIOR_BYTES_IMMUTABLE", e)
			}
		}
	}
	t.Logf("ASSERT_RETAINED_SELECTED_TWENTY: PASS; profile=%s cumulative=%+v; controlled reservation, not telemetry", owner.Profile(), owner.BudgetSpent())
}
