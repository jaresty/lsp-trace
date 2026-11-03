//go:build darwin

package retainedpartialprogramc

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
)

type reuseTransferPrivateV3 struct {
	Manifest   []byte
	Spec       pilot.ScopeSpecPrivateV2
	LedgerPath string
	Expected   [20][4]string
}

// Use the existing controlled process fixture without importing dependency
// test-only exports. All ownership is reopened through the production APIs.
func reuseFixtureFactoryPrivateV3(t *testing.T) func(*testing.T) (ProjectionRetainedCheckpointPrivateV3, AssessmentRetainedCheckpointPrivateV3, pilot.RetainedSelectedBatchPrivateV3, reuseTransferPrivateV3, pilot.PageCostPrivateV3) {
	t.Helper()
	root := t.TempDir()
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(root, "controlled.test")
	args := []string{"test", "-c", "-o", exe, "../censuslineagepilot"}
	if os.Getenv("LSP_COST_CHILD_RACE") == "1" {
		args = append(args, "-race")
	}
	cmd := exec.Command("go", args...)
	for _, v := range os.Environ() {
		key := strings.SplitN(v, "=", 2)[0]
		switch key {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOWORK", "GOFLAGS":
		default:
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("COMBINED_FIXTURE_COMPILE: %v %s", e, out)
	}
	return func(t *testing.T) (ProjectionRetainedCheckpointPrivateV3, AssessmentRetainedCheckpointPrivateV3, pilot.RetainedSelectedBatchPrivateV3, reuseTransferPrivateV3, pilot.PageCostPrivateV3) {
		t.Helper()
		ctx := context.Background()
		grant := pilot.PageCostPrivateV3{LogicalBytes: 1 << 40, ReadHashBytes: 1 << 40, WriteBytes: 1 << 34, Work: 1 << 32}
		dir := t.TempDir()
		if e := os.Chmod(dir, 0700); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(dir, "transfer.json")
		child := exec.Command(exe, "-test.run=^TestRetainedCheckpointPrivateV3ProcessChild$", "-test.v")
		child.Env = append(cmd.Env, "TMPDIR="+dir, "LSP_RETAINED_CHECKPOINT_PROCESS=A", "LSP_RETAINED_CHECKPOINT_TRANSFER="+path)
		out, e := child.CombinedOutput()
		if e != nil || !bytes.Contains(out, []byte("ASSERT_RETAINED_PROCESS_A_ORIGINAL_HANDLES_RELEASED_BY_EXIT: PASS")) {
			t.Fatalf("COMBINED_FIXTURE_EXPORT: %v %s", e, out)
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var x reuseTransferPrivateV3
		if e = json.Unmarshal(raw, &x); e != nil {
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
		p, e := ProjectRetainedCheckpointPrivateV3(ctx, b, grant)
		if e != nil {
			t.Fatal(e)
		}
		a, e := ComputeInstabilityRetainedCheckpointPrivateV3(ctx, p, []uint64{3, 7}, grant)
		if e != nil {
			t.Fatal(e)
		}
		return p, a, b, x, grant
	}
}

func TestRetainedCheckpointPrivateV3CombinedEvidence(t *testing.T) {
	factory := reuseFixtureFactoryPrivateV3(t)
	ctx := context.Background()
	p, a, b, x, grant := factory(t)
	legacy, oldRaw, e := ValidateInstabilityRetainedCheckpointPrivateV3(ctx, p, a, grant)
	if e != nil {
		t.Fatal(e)
	}
	oldView, e := p.Read(ctx, grant)
	if e != nil {
		t.Fatal(e)
	}
	v, raw, e := ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant)
	if e != nil || !reflect.DeepEqual(v, oldView) || !bytes.Equal(raw, oldRaw) || !reflect.DeepEqual(legacy.Source, v.Source) {
		t.Fatal("COMBINED_CANONICAL", e)
	}
	t.Log("COMBINED_CANONICAL: A-pass legacy bytes, source, math and selected evidence identical")
	original := bytes.Clone(raw)
	raw[0] ^= 1
	v.DerivedCanonicalV5[0] ^= 1
	v.OriginalNativeV3[0] ^= 1
	v.Selected.SourceArtifact[0] ^= 1
	fresh, freshRaw, e := ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant)
	if e != nil || !bytes.Equal(freshRaw, original) || !reflect.DeepEqual(fresh, oldView) {
		t.Fatal("COMBINED_DETACHED", e)
	}
	checkZero := func(t *testing.T, v ProjectionEvidenceRetainedCheckpointPrivateV3, raw []byte, e error) {
		t.Helper()
		if e == nil || raw != nil || !reflect.DeepEqual(v, ProjectionEvidenceRetainedCheckpointPrivateV3{}) {
			t.Fatal("COMBINED_ZERO_ON_ERROR", e)
		}
	}
	t.Run("foreign-before-read", func(t *testing.T) {
		copied := *p.state
		copied.self = &copied
		foreign := ProjectionRetainedCheckpointPrivateV3{&copied}
		before := p.BudgetSpent()
		v, raw, e := ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, foreign, a, grant)
		checkZero(t, v, raw, e)
		if p.BudgetSpent() != before {
			t.Fatal("COMBINED_FOREIGN_BEFORE_READ")
		}
		copiedA := *a.state
		bad := AssessmentRetainedCheckpointPrivateV3{&copiedA}
		v, raw, e = ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, bad, grant)
		checkZero(t, v, raw, e)
		if p.BudgetSpent() != before {
			t.Fatal("COMBINED_SELF_BEFORE_READ")
		}
	})
	t.Run("forged-assessment", func(t *testing.T) {
		var report InstabilityReportRetainedCheckpointPrivateV3
		if e := json.Unmarshal(a.state.raw, &report); e != nil {
			t.Fatal(e)
		}
		report.Source.CheckpointSHA256 = pilot.Digest([]byte("forged"))
		forged, e := json.Marshal(report)
		if e != nil {
			t.Fatal(e)
		}
		state := &assessmentCheckpointStatePrivateV3{projection: p.state, raw: forged}
		state.self = state
		called := false
		v, raw, e := validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, AssessmentRetainedCheckpointPrivateV3{state}, grant, func(context.Context, pilot.RetainedSelectedBatchBindingPrivateV3, pilot.PageCostPrivateV3) error {
			called = true
			return nil
		})
		checkZero(t, v, raw, e)
		if called {
			t.Fatal("COMBINED_FORGED_RECOMPUTE_BEFORE_EXIT")
		}
	})
	t.Run("remaining-exchange", func(t *testing.T) {
		before := p.BudgetSpent()
		v, raw, e := validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant, func(c context.Context, w pilot.RetainedSelectedBatchBindingPrivateV3, left pilot.PageCostPrivateV3) error {
			want, e := pilot.RemainingPageExchangePrivateV3(before, p.BudgetSpent(), grant)
			if e != nil || left != want || left == grant {
				t.Fatal("COMBINED_REMAINING", e)
			}
			return b.RevalidateBindingPrivateV3(c, w, left)
		})
		if e != nil || len(raw) == 0 || v.Source.CheckpointSHA256 == "" {
			t.Fatal(e)
		}
		// Exactly the validation prefix is insufficient for the mandatory exit.
		start := p.BudgetSpent()
		_, _, e = ValidateInstabilityRetainedCheckpointPrivateV3(ctx, p, a, grant)
		if e != nil {
			t.Fatal(e)
		}
		after := p.BudgetSpent()
		prefix := pilot.PageCostPrivateV3{LogicalBytes: after.LogicalBytes - start.LogicalBytes, ReadHashBytes: after.ReadHashBytes - start.ReadHashBytes, WriteBytes: after.WriteBytes - start.WriteBytes, Work: after.Work - start.Work}
		start = p.BudgetSpent()
		v, raw, e = ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, prefix)
		checkZero(t, v, raw, e)
		if !errors.Is(e, pilot.ErrPagesPausedPrivateV3) || p.BudgetSpent() == start {
			t.Fatal("COMBINED_EXIT_BUDGET_PREFIX", e)
		}
	})
	for _, when := range []string{"before", "between", "after"} {
		t.Run("cancel-"+when, func(t *testing.T) {
			c, cancel := context.WithCancel(ctx)
			defer cancel()
			if when == "before" {
				cancel()
			}
			v, raw, e := validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(c, p, a, grant, func(c context.Context, w pilot.RetainedSelectedBatchBindingPrivateV3, left pilot.PageCostPrivateV3) error {
				if when == "between" {
					cancel()
				}
				e := b.RevalidateBindingPrivateV3(c, w, left)
				if when == "after" {
					cancel()
				}
				return e
			})
			checkZero(t, v, raw, e)
			if !errors.Is(e, context.Canceled) {
				t.Fatal("COMBINED_CANCEL", e)
			}
		})
	}
	// Every mutation occurs after recomputation, immediately before the real exit.
	for _, kind := range []string{"source", "grown", "runtime", "root"} {
		t.Run("exit-"+kind, func(t *testing.T) {
			p, a, b, x, grant := factory(t)
			var restore func()
			v, raw, e := validateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant, func(c context.Context, w pilot.RetainedSelectedBatchBindingPrivateV3, left pilot.PageCostPrivateV3) error {
				if kind == "source" || kind == "grown" {
					path := filepath.Join(x.Spec.Root, x.Spec.Members[0].CanonicalRelativePath)
					old, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					changed := bytes.Repeat([]byte("x"), len(old))
					if kind == "grown" {
						changed = bytes.Repeat([]byte("x"), x.Spec.EffectivePolicy.FileBytes+1)
					}
					if e = os.WriteFile(path, changed, 0600); e != nil {
						t.Fatal(e)
					}
					restore = func() {
						if e := os.WriteFile(path, old, 0600); e != nil {
							t.Fatal(e)
						}
					}
				} else {
					var m pilot.RetainedPageCheckpointManifestPrivateV3
					if e := json.Unmarshal(x.Manifest, &m); e != nil {
						t.Fatal(e)
					}
					if kind == "runtime" {
						// Runtime metadata is one page in this controlled fixture. Locate that
						// paired page by its canonical header, bounded by the manifest page count.
						entries, e := os.ReadDir(m.Roots[0].Path)
						if e != nil {
							t.Fatal(e)
						}
						found := false
						for _, entry := range entries {
							if entry.IsDir() {
								continue
							}
							path := filepath.Join(m.Roots[0].Path, entry.Name())
							old, e := os.ReadFile(path)
							if e != nil {
								t.Fatal(e)
							}
							if !bytes.Contains(old, []byte(`"Role":"RUNTIME_METADATA"`)) && !bytes.Contains(old, []byte(`"role":"RUNTIME_METADATA"`)) {
								continue
							}
							if e = os.WriteFile(path, []byte("corrupt"), 0600); e != nil {
								t.Fatal(e)
							}
							restore = func() {
								if e := os.WriteFile(path, old, 0600); e != nil {
									t.Fatal(e)
								}
							}
							found = true
							break
						}
						if !found {
							t.Fatal("COMBINED_RUNTIME_PAGE_FIXTURE")
						}
					} else {
						path := m.Roots[0].Path
						moved := path + "-moved"
						if e := os.Rename(path, moved); e != nil {
							t.Fatal(e)
						}
						if e := os.Mkdir(path, 0700); e != nil {
							t.Fatal(e)
						}
						restore = func() {
							if e := os.Remove(path); e != nil {
								t.Fatal(e)
							}
							if e := os.Rename(moved, path); e != nil {
								t.Fatal(e)
							}
						}
					}
				}
				return b.RevalidateBindingPrivateV3(c, w, left)
			})
			if restore != nil {
				restore()
			}
			checkZero(t, v, raw, e)
			if kind == "runtime" || kind == "root" {
				_, _, e = ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant)
				if e != nil {
					t.Fatal("COMBINED_RESTORED", e)
				}
			}
			t.Log("COMBINED_ZERO_ON_ERROR: mutation refused before disclosure; restored", kind)
		})
	}
	// A later call still checks current source; previous successful evidence cannot
	// become a cross-call cache. Content restoration does not restore sealed stamps.
	path := filepath.Join(x.Spec.Root, x.Spec.Members[0].CanonicalRelativePath)
	old, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, bytes.Repeat([]byte("x"), len(old)), 0600); e != nil {
		t.Fatal(e)
	}
	v, raw, e = ValidateInstabilityWithEvidenceRetainedCheckpointPrivateV3(ctx, p, a, grant)
	checkZero(t, v, raw, e)
	if e = os.WriteFile(path, old, 0600); e != nil {
		t.Fatal(e)
	}
}
