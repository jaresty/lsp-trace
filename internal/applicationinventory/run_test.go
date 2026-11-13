package applicationinventory

import (
	"context"
	"errors"
	"fmt"
	"io"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplicationInventoryV3RefusalsAndDispositions(t *testing.T) {
	calls, hosts := 0, 0
	q := Request{Manifest: Manifest{Scope: "fixture", Provenance: "supplied bounded leads", Leads: []Lead{{ID: "empty"}, {ID: "unavailable"}, {ID: "failed"}, {ID: "zero"}, {ID: "tail"}}}, MaxNativeAttempts: 4, NativeRetainedUpper: 8 << 20, Interpreter: "actor", HostWindow: time.Second,
		Preparer: func(_ context.Context, l Lead) (NativeResult, error) {
			calls++
			switch l.ID {
			case "empty":
				return NativeResult{Status: "OBSERVED_EMPTY"}, nil
			case "unavailable":
				return NativeResult{Status: "UNAVAILABLE"}, nil
			case "failed":
				return NativeResult{}, errors.New("controlled failure")
			default:
				return NativeResult{Status: "SELECTED"}, nil
			}
		},
		DescribeHost: func(context.Context, []byte, io.Writer) error { hosts++; return nil }}
	r, e := Run(context.Background(), q)
	if e == nil || r.Published || calls != 4 || hosts != 0 {
		t.Fatal("ASSERT_V3_ZERO_PROOF_NO_HOST", e, calls, hosts)
	}
	for i, status := range []string{"OBSERVED_EMPTY", "UNAVAILABLE", "PREPARATION_FAILED", "PREPARATION_FAILED", "UNTOUCHED"} {
		if r.Dispositions[i].Status != status {
			t.Fatal("ASSERT_V3_DISTINCT_LEAD_STATES", r.Dispositions)
		}
	}
	for name, s := range r.Stages {
		if s.Requested != s.Successful+s.Failed {
			t.Fatal("ASSERT_V3_FAILURE_STAGE_CONSERVATION", name, s)
		}
	}
	q.Manifest.Leads = []Lead{{ID: "duplicate"}, {ID: "duplicate"}}
	calls = 0
	if _, e = Run(context.Background(), q); e == nil || calls != 0 {
		t.Fatal("ASSERT_V3_DUPLICATE_LEADS_BEFORE_EFFECT")
	}
	q.Manifest.Leads = []Lead{{ID: "one"}}
	q.BudgetBytes = 1
	calls = 0
	if _, e = Run(context.Background(), q); e == nil || calls != 0 {
		t.Fatal("ASSERT_V3_RESERVE_BEFORE_PREPARER")
	}
	t.Log("ASSERT_V3_ZERO_PROOF_NO_HOST: PASS; ASSERT_V3_DISTINCT_LEAD_STATES: PASS; ASSERT_V3_FAILURE_STAGE_CONSERVATION: PASS; ASSERT_V3_DUPLICATE_LEADS_BEFORE_EFFECT: PASS; ASSERT_V3_RESERVE_BEFORE_PREPARER: PASS")
}

func TestApplicationInventoryV3CallbackBounds(t *testing.T) {
	ctx := context.Background()
	b, _ := newTestBudgetV3()
	if _, e := exchangeV3(ctx, b, func(_ context.Context, _ []byte, w io.Writer) error { _, _ = w.Write(make([]byte, 17)); return nil }, []byte("input"), 16, time.Second); e == nil {
		t.Fatal("ASSERT_V3_IGNORED_WRITER_OVERFLOW_REFUSED")
	}
	cx, cancel := context.WithCancel(ctx)
	b, _ = newTestBudgetV3()
	if _, e := exchangeV3(cx, b, func(context.Context, []byte, io.Writer) error { cancel(); return nil }, []byte("input"), 16, time.Second); e == nil {
		t.Fatal("ASSERT_V3_CALLBACK_CANCELLATION_REFUSED")
	}
	var late io.Writer
	b, _ = newTestBudgetV3()
	raw, e := exchangeV3(ctx, b, func(_ context.Context, _ []byte, w io.Writer) error {
		late = w
		_, e := w.Write([]byte("ok"))
		return e
	}, []byte("input"), 16, time.Second)
	if e != nil || string(raw) != "ok" {
		t.Fatal(e)
	}
	if _, e = late.Write([]byte("late")); e == nil {
		t.Fatal("ASSERT_V3_LATE_CALLBACK_WRITE_REFUSED")
	}
	if _, ok := late.(interface{ Reset() }); ok {
		t.Fatal("ASSERT_V3_NO_BUFFER_METHOD_ESCAPE")
	}
	t.Log("ASSERT_V3_LATE_CALLBACK_WRITE_REFUSED: PASS; ASSERT_V3_NO_BUFFER_METHOD_ESCAPE: PASS")
	t.Log("ASSERT_V3_IGNORED_WRITER_OVERFLOW_REFUSED: PASS; ASSERT_V3_CALLBACK_CANCELLATION_REFUSED: PASS")
}
func newTestBudgetV3() (*sc.CallerBudgetV3, error) {
	return sc.NewCallerBudgetV3(time.Now().Add(time.Minute), 1<<20)
}

func TestApplicationInventoryV3HardCeilings(t *testing.T) {
	calls := 0
	q := Request{Manifest: Manifest{Scope: "controlled", Provenance: "fixture leads"}, Interpreter: "actor", MaxNativeAttempts: 1, NativeRetainedUpper: 4096, HostWindow: time.Second, Preparer: func(context.Context, Lead) (NativeResult, error) { calls++; return NativeResult{}, nil }}
	q.Manifest.Leads = make([]Lead, 10001)
	for i := range q.Manifest.Leads {
		q.Manifest.Leads[i].ID = fmt.Sprint(i)
	}
	if _, e := Run(context.Background(), q); e == nil || calls != 0 {
		t.Fatal("ASSERT_V3_LEAD_CEILING_BEFORE_EFFECT")
	}
	q.Manifest.Leads = []Lead{{ID: "one"}}
	q.MaxNativeAttempts = 101
	if _, e := Run(context.Background(), q); e == nil || calls != 0 {
		t.Fatal("ASSERT_V3_NATIVE_ATTEMPT_CEILING")
	}
	if _, e := sc.NewCallerBudgetV3(time.Now().Add(time.Minute), (64<<20)+1); e == nil {
		t.Fatal("ASSERT_V3_AGGREGATE_CEILING")
	}
	if _, e := sc.NewCallerBudgetV3(time.Now().Add(181*time.Second), 1024); e == nil {
		t.Fatal("ASSERT_V3_DRIVER_WINDOW_CEILING")
	}
	b, _ := newTestBudgetV3()
	if _, _, e := b.Exchange(context.Background(), 30*time.Second+1); e == nil {
		t.Fatal("ASSERT_V3_HOST_WINDOW_CEILING")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	destination := filepath.Join(root, "oversized")
	b, _ = newTestBudgetV3()
	if _, e := PublishV3(context.Background(), destination, []ArtifactV3{{"body.json", make([]byte, (8<<20)+1)}}, b); e == nil {
		t.Fatal("ASSERT_V3_BODY_CEILING_BEFORE_WRITE")
	}
	if _, e := os.Stat(destination); !os.IsNotExist(e) {
		t.Fatal("ASSERT_V3_BODY_CEILING_BEFORE_WRITE", e)
	}
	t.Log("ASSERT_V3_LEAD_CEILING_BEFORE_EFFECT: PASS; ASSERT_V3_NATIVE_ATTEMPT_CEILING: PASS; ASSERT_V3_AGGREGATE_CEILING: PASS; ASSERT_V3_DRIVER_WINDOW_CEILING: PASS; ASSERT_V3_HOST_WINDOW_CEILING: PASS; ASSERT_V3_BODY_CEILING_BEFORE_WRITE: PASS")
}

func TestApplicationInventoryV3UntouchedConservation(t *testing.T) {
	q := Request{Manifest: Manifest{Scope: "fixture-only", Provenance: "controlled supplied lead manifest", Leads: []Lead{{ID: "a", Provenance: "fixture"}, {ID: "b", Provenance: "fixture"}}}}
	r, _ := Run(context.Background(), q)
	if len(r.Dispositions) != 2 {
		t.Fatal("ASSERT_V3_EVERY_LEAD_DISPOSITION", len(r.Dispositions))
	}
	for _, d := range r.Dispositions {
		if d.Status != "UNTOUCHED" {
			t.Fatal("ASSERT_V3_UNTOUCHED_DISTINCT", d)
		}
	}
	t.Log("ASSERT_V3_EVERY_LEAD_DISPOSITION: PASS; ASSERT_V3_UNTOUCHED_DISTINCT: PASS")
}
