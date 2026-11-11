//go:build lsp_trace_controlled_describe_fixture

package sessionruntime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

func TestPrivateProviderSliceFreeze(t *testing.T) {
	t.Log("PROCEDURE: TestPrivateProviderSliceFreeze; PrivateProviderWindowEnvelope -> inspect actual spec -> mutate caller arrays")
	for _, shape := range []string{"nil", "empty", "populated"} {
		t.Run(shape, func(t *testing.T) {
			f := controlledDescribeFixtureTest(t)
			q := f.StartRequest(time.Now().Add(10 * time.Second))
			switch shape {
			case "empty":
				q.Process.Args = []string{}
				q.Process.Env = []string{}
				q.RelationProviders = []string{}
			case "populated":
				q.Process.Args = []string{"freeze-only-arg"}
				q.Process.Env = []string{"SLICE_LITERAL=1"}
				q.RelationProviders = []string{"builtin"}
			}
			e, err := f.Manager().PrivateProviderWindowEnvelope(q, PrivateProviderWindowPlan{References: true})
			if err != nil || e.spec == nil {
				t.Fatal("SETUP_SLICE_ENVELOPE", err)
			}
			if !reflect.DeepEqual(q, e.spec.start) {
				t.Error("ASSERT_RUNTIME_SLICE_SHAPE: FAIL")
			} else {
				t.Log("ASSERT_RUNTIME_SLICE_SHAPE: PASS")
			}
			if shape != "populated" {
				return
			}
			q.Process.Args[0] = "changed-arg"
			q.Process.Env[0] = "CHANGED=1"
			q.RelationProviders[0] = "changed-provider"
			if e.spec.start.Process.Args[0] != "freeze-only-arg" || e.spec.start.Process.Env[0] != "SLICE_LITERAL=1" || e.spec.start.RelationProviders[0] != "builtin" {
				t.Error("ASSERT_RUNTIME_SLICE_COPY: FAIL")
			} else {
				t.Log("ASSERT_RUNTIME_SLICE_COPY: PASS")
			}
		})
	}
}

func TestPrivateProviderSliceChangedReplay(t *testing.T) {
	t.Log("PROCEDURE: TestPrivateProviderSliceChangedReplay; real envelope/window -> mutate original Args -> inspect both original equality boundaries")
	f := controlledDescribeFixtureTest(t)
	m := f.Manager()
	q := f.StartRequest(time.Now().Add(10 * time.Second))
	q.Process.Args = []string{"original-caller-arg"}
	e, err := m.PrivateProviderWindowEnvelope(q, PrivateProviderWindowPlan{References: true})
	if err != nil {
		t.Fatal("SETUP_SLICE_ENVELOPE", err)
	}
	ctx, err := m.PrivateProviderWindowContext(context.Background(), e, q.Deadline)
	if err != nil {
		t.Fatal("SETUP_SLICE_WINDOW", err)
	}
	defer func() { _, _ = m.FinishPrivateProviderWindow(ctx) }() // no child: completion must NOT be fabricated
	changed := q
	changed.Process.Args[0] = "mutated-caller-arg"
	result := m.Start(ctx, changed)
	w := privateProviderWindowFrom(ctx)
	w.mu.Lock()
	started := w.started
	w.mu.Unlock()
	if result.Failure != session.LifecycleConflict || started {
		t.Error("ASSERT_RUNTIME_SLICE_REPLAY_START: FAIL", result.Failure, started)
	} else {
		t.Log("ASSERT_RUNTIME_SLICE_REPLAY_START: PASS")
	}
	child, observation := m.privateProviderWindowStartChild(ctx, changed.Process)
	if child != nil || observation.Kind != managedprocess.StartUnavailable {
		t.Error("ASSERT_RUNTIME_SLICE_REPLAY_CHILD: FAIL", observation.Kind)
	} else {
		t.Log("ASSERT_RUNTIME_SLICE_REPLAY_CHILD: PASS")
	}
}
