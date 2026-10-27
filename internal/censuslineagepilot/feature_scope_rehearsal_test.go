//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/publication"
)

// This test NEVER opts in implicitly. Its only real-input entry is the exact env
// sentinel; all other tests below use existing synthetic fixture utilities.
func TestScopeRealInputRetentionRehearsal(t *testing.T) {
	opts, enabled, err := scopeRehearsalEnvironment(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Skip("opt-in only: LSP_TRACE_SCOPE_REHEARSAL=retain-inputs")
	}
	report, err := runScopeRehearsal(context.Background(), opts)
	raw, encodeErr := json.Marshal(report)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	// Stdout is not claimed durable; the caller must retain the test output if wanted.
	t.Log(string(raw))
	if err != nil {
		t.Fatal(err)
	}
}
func TestScopeRehearsalOptions(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if _, on, e := scopeRehearsalEnvironment(get); on || e != nil {
		t.Fatal(on, e)
	}
	env["LSP_TRACE_SCOPE_REHEARSAL"] = "1"
	if _, _, e := scopeRehearsalEnvironment(get); e == nil {
		t.Fatal("weak opt-in")
	}
	env["LSP_TRACE_SCOPE_REHEARSAL"] = "retain-inputs"
	if _, on, e := scopeRehearsalEnvironment(get); !on || e == nil {
		t.Fatal("missing input accepted")
	}
	for k, v := range map[string]string{"LSP_TRACE_SCOPE_SOURCE": "/source", "LSP_TRACE_SCOPE_REVISION": fixtureRevision, "LSP_TRACE_SCOPE_PRIMARY": "/primary", "LSP_TRACE_SCOPE_BACKUP": "/backup", "LSP_TRACE_SCOPE_JOURNAL": "/journal"} {
		env[k] = v
	}
	if _, on, e := scopeRehearsalEnvironment(get); !on || e != nil {
		t.Fatal(on, e)
	}
}
func rehearsalFixture(t *testing.T, equal bool) (scopeRehearsalOptions, *scopeBundleOwner, featureScope) {
	t.Helper()
	o, s := bundleFixture(t, equal)
	var m scopeManifest
	if err := json.Unmarshal(s.ManifestBytes(), &m); err != nil {
		t.Fatal(err)
	}
	return scopeRehearsalOptions{source: m.Root, revision: fixtureRevision, roots: o.paths}, o, s
}
func TestScopeRehearsalSuccessCountsAndResume(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(map[bool]string{false: "distinct", true: "dedup"}[equal], func(t *testing.T) {
			opts, o, s := rehearsalFixture(t, equal)
			r, e := runScopeRehearsal(context.Background(), opts)
			if e != nil {
				t.Fatal(e)
			}
			n := len(o.plan.state.objects)
			if r.UniqueObjects != n || r.References != 9 || r.Nomination != 4 || r.Context != 2 || r.Authority != 0 || r.Accepted || r.ProcessRestartTested {
				t.Fatalf("bad counts: %+v", r)
			}
			if len(r.Steps) != 2*(n+1)+1 || r.Primary.Files != uint64(n+1) || r.Backup.Files != uint64(n+1) || r.Journal.Files != uint64(n+3) {
				t.Fatal("wrong actual counts", r)
			}
			if r.PrimaryDataBytes != r.UniquePayloadBytes || r.BackupDataBytes != r.UniquePayloadBytes || r.ReservedLogicalBytes <= r.Primary.LogicalBytes+r.Backup.LogicalBytes {
				t.Fatal("budget mistaken for measurement")
			}
			if r.Scope != rehearsalBlob(s.ManifestBytes()) || r.Selection != rehearsalBlob(s.SelectionBytes()) || r.Plan != rehearsalBlob(s.PlanBytes()) {
				t.Fatal("raw IDs")
			}
			for i, step := range r.Steps {
				if step.Error != "" || step.CASCommitted != (i < len(r.Steps)-1) {
					t.Fatal("CAS accounting", i, step)
				}
			}
			again, e := runScopeRehearsal(context.Background(), opts)
			if e != nil || len(again.Steps) != 2 || again.FinalHead != r.FinalHead {
				t.Fatal("terminal resume", e)
			}
			for _, step := range again.Steps {
				if step.CASCommitted || step.CAS != nil {
					t.Fatal("terminal wrote")
				}
			}
			raw, e := json.Marshal(r)
			if e != nil || !json.Valid(raw) {
				t.Fatal(e)
			}
			// Changing even the assertion must not start another absent key.
			opts.revision = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			changed, e := runScopeRehearsal(context.Background(), opts)
			if e == nil || len(changed.Steps) != 0 {
				t.Fatal("changed pins started run")
			}
		})
	}
}
func TestScopeRehearsalStopsNoRetryAndPreservesFacts(t *testing.T) {
	for _, kind := range []string{"conflict", "known-commit-error", "pair-close", "final-close", "failed", "indeterminate"} {
		t.Run(kind, func(t *testing.T) {
			_, o, _ := rehearsalFixture(t, false)
			normal := o.ops
			calls, closes := 0, 0
			sentinel := errors.New("fixture fault")
			o.ops.cas = func(ctx context.Context, r *publication.ContentRetentionRoot, key string, old, new []byte) (*publication.CompareAndReplaceReceipt, error) {
				calls++
				if kind == "conflict" {
					return &publication.CompareAndReplaceReceipt{}, publication.ErrStalePredecessor
				}
				result, e := normal.cas(ctx, r, key, old, new)
				if kind == "known-commit-error" {
					e = errors.Join(e, sentinel)
				}
				return result, e
			}
			o.ops.close = func(r *publication.ContentRetentionRoot) error {
				closes++
				e := normal.close(r)
				if kind == "pair-close" && closes == 1 || kind == "final-close" && closes == 6 {
					return errors.Join(e, sentinel)
				}
				return e
			}
			if kind == "failed" || kind == "indeterminate" {
				o.ops.open = func(context.Context, string) (*publication.ContentRetentionRoot, error) { return nil, sentinel }
			}
			var r scopeRehearsalReport
			out, e := scopeRehearsalLoop(context.Background(), o, &r)
			want := 1
			if kind == "final-close" {
				want = 2
			}
			if e == nil || len(r.Steps) != want || calls > want {
				t.Fatal("retried failure", kind, calls, len(r.Steps), e)
			}
			last := r.Steps[len(r.Steps)-1]
			if kind == "known-commit-error" || kind == "pair-close" || kind == "final-close" {
				if !out.casCommitted || len(last.Head) == 0 {
					t.Fatal("lost commit")
				}
			}
			if kind == "pair-close" || kind == "final-close" {
				if len(last.Receipt) == 0 {
					t.Fatal("lost close/attempt facts")
				}
			}
		})
	}
	// Explicit terminal outcomes are stops even when there is no Go error.
	for _, state := range []string{"FAILED", "INDETERMINATE_EXECUTION"} {
		o, s := bundleFixture(t, false)
		if state == "FAILED" {
			o.ops.pair = func(context.Context, scopeRetentionPlan, int, *publication.ContentRetentionRoot, *publication.ContentRetentionRoot) (scopeRetentionAttempt, error) {
				return scopeRetentionAttempt{phase: "PREFLIGHT", primary: scopeCopyAttempt{outcome: "NOT_ATTEMPTED"}, backup: scopeCopyAttempt{outcome: "NOT_ATTEMPTED"}}, errors.New("fixture pair failure")
			}
		}
		first, e := o.advance(context.Background(), nil)
		if state == "FAILED" {
			if e == nil || first.pending == nil {
				t.Fatal("missing failure receipt")
			}
			final, e := o.advance(context.Background(), first.pending)
			if e != nil || final.outcome != "FAILED" {
				t.Fatal(e)
			}
		} else if e != nil {
			t.Fatal(e)
		}
		fresh, e := newScopeBundleOwner(s, o.paths[0], o.paths[1], o.paths[2])
		if e != nil {
			t.Fatal(e)
		}
		fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
			t.Fatal("existing stop CAS")
			return nil, ErrInvalid
		}
		var r scopeRehearsalReport
		_, e = scopeRehearsalLoop(context.Background(), fresh, &r)
		if e == nil || len(r.Steps) != 1 || r.Steps[0].Outcome != state {
			t.Fatal("terminal stop retried", state, e)
		}
	}
}
func TestScopeRehearsalFrozenScopeSurvivesCheckoutChange(t *testing.T) {
	opts, o, scope := rehearsalFixture(t, true)
	if e := os.WriteFile(filepath.Join(opts.source, featureScopePaths()[0]), []byte("package changed\n"), 0644); e != nil {
		t.Fatal(e)
	}
	var report scopeRehearsalReport
	final, e := scopeRehearsalLoop(context.Background(), o, &report)
	if e != nil {
		t.Fatal(e)
	}
	fresh, e := newScopeBundleOwner(scope, opts.roots[0], opts.roots[1], opts.roots[2])
	if e != nil {
		t.Fatal(e)
	}
	fresh.ops.cas = func(context.Context, *publication.ContentRetentionRoot, string, []byte, []byte) (*publication.CompareAndReplaceReceipt, error) {
		t.Fatal("fresh terminal CAS")
		return nil, ErrInvalid
	}
	checked, e := fresh.advance(context.Background(), nil)
	if e != nil || checked.outcome != "RETAINED_INPUT_BUNDLE" || !bytes.Equal(checked.head, final.head) {
		t.Fatal("checkout substituted for frozen scope", e)
	}
	// A new invocation observing the changed checkout must refuse a new key.
	r, e := runScopeRehearsal(context.Background(), opts)
	if e == nil || len(r.Steps) != 0 {
		t.Fatal("changed source pins admitted")
	}
}

func TestScopeRehearsalFailureReportCanonical(t *testing.T) {
	opts, o, _ := rehearsalFixture(t, true)
	final := runBundle(t, o)
	path := filepath.Join(opts.roots[2], scopeBundleReceiptSelector(final.receiptSelector))
	// Malformed observed evidence must not prevent reporting the known head/failure.
	if e := os.WriteFile(path, []byte("not json"), 0600); e != nil {
		t.Fatal(e)
	}
	r, e := runScopeRehearsal(context.Background(), opts)
	if e == nil || r.Failure == "" || len(r.Steps) != 1 || len(r.Steps[0].Head) == 0 {
		t.Fatal("lost refusal evidence")
	}
	raw, e := json.Marshal(r)
	if e != nil || !json.Valid(raw) {
		t.Fatal("unsafe failure report", e)
	}
	step := rehearsalStep(scopeBundleResult{head: []byte("known"), linked: [][]byte{[]byte("malformed")}}, errors.New("fault"))
	if raw, e = json.Marshal(step); e != nil || !json.Valid(raw) {
		t.Fatal("raw evidence not encodable", e)
	}
}

func TestScopeRehearsalExistingClaimAndBudgetAndRoots(t *testing.T) {
	opts, o, _ := rehearsalFixture(t, false)
	claim, e := o.advance(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	r, e := runScopeRehearsal(context.Background(), opts)
	if e == nil || len(r.Steps) != 1 || !bytes.Equal(r.Steps[0].Head, claim.head) || r.Steps[0].CASCommitted {
		t.Fatal("existing active claim", e)
	}
	opts, _, _ = rehearsalFixture(t, false)
	if e = os.WriteFile(filepath.Join(opts.source, featureScopePaths()[0]), bytes.Repeat([]byte("x"), featureScopeFileLimit+1), 0644); e != nil {
		t.Fatal(e)
	}
	r, e = runScopeRehearsal(context.Background(), opts)
	if e == nil || len(r.Steps) != 0 {
		t.Fatal("input budget")
	}
	opts, _, _ = rehearsalFixture(t, false)
	opts.roots[1] = opts.roots[0]
	if _, e = runScopeRehearsal(context.Background(), opts); e == nil {
		t.Fatal("alias admitted")
	}
	opts, _, _ = rehearsalFixture(t, false)
	if e = os.WriteFile(filepath.Join(opts.roots[0], "unrelated"), []byte("retain me"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = runScopeRehearsal(context.Background(), opts); e == nil {
		t.Fatal("nonempty fresh roots")
	}
	if raw, e := os.ReadFile(filepath.Join(opts.roots[0], "unrelated")); e != nil || string(raw) != "retain me" {
		t.Fatal("existing content changed")
	}
}
