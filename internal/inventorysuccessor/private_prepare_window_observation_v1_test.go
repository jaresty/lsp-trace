package inventorysuccessor

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"lsp-trace/sessionruntime"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func prepareObservationAwaitV1(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("ASSERT_CALLBACK_BARRIER: FAIL")
	}
}
func TestPrivatePrepareWindowPostReturnFreeze(t *testing.T) {
	// Unit isolation of the private callback record, not an actual capture. The
	// simulated wrapper-return barrier is separate from the first caller lock.
	r := privatePrepareCallbackObservationV1{value: PrivatePrepareLeaseObservationV1{CallbackEntered: PrivatePrepareObservedNoV1}}
	wrapperReturn, progress, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); prepareObservationAwaitV1(t, done) })
	go func() {
		<-wrapperReturn
		r.enter()
		r.captured(sessionruntime.PrivateGroupingCaptureOK)
		close(progress)
		<-release
		r.captured(sessionruntime.PrivateGroupingCaptureTargetSelectionRefused)
		r.called(context.DeadlineExceeded)
		r.issued(nil)
		close(done)
	}()
	close(wrapperReturn)
	prepareObservationAwaitV1(t, progress)
	// Progress happened AFTER wrapper boundary but BEFORE caller lock: it must be
	// visible. CALLS and issuance have not returned; neither is a failure claim.
	snapshot := r.freeze(sessionruntime.PrivateLeaseBoundaryObservation{})
	if snapshot.CallbackSample != PrivatePrepareCallbackCallerPostReturnLockV1 || snapshot.CallbackEntered != PrivatePrepareObservedYesV1 || snapshot.CaptureResult != sessionruntime.PrivateGroupingCaptureOK || snapshot.CallsResult != PrivatePrepareCheckpointUnobservedV1 || snapshot.IssuanceResult != PrivatePrepareCheckpointUnobservedV1 {
		t.Fatal("ASSERT_CALLBACK_POST_RETURN_FREEZE: FAIL sampling")
	}
	saved := snapshot
	unblock()
	prepareObservationAwaitV1(t, done)
	if snapshot != saved || r.value.CaptureResult != sessionruntime.PrivateGroupingCaptureTargetSelectionRefused || r.value.CallsResult != PrivatePrepareCheckpointDeadlineV1 || r.value.IssuanceResult != PrivatePrepareCheckpointOKV1 {
		t.Fatal("ASSERT_CALLBACK_POST_RETURN_FREEZE: FAIL detached copy")
	}
	t.Log("ASSERT_CALLBACK_POST_RETURN_FREEZE: PASS")
}
func TestPrivatePrepareWindowCallsGap(t *testing.T) {
	r := privatePrepareCallbackObservationV1{value: PrivatePrepareLeaseObservationV1{CallbackEntered: PrivatePrepareObservedNoV1}}
	first := r.freeze(sessionruntime.PrivateLeaseBoundaryObservation{})
	if first.CallbackEntered != PrivatePrepareObservedNoV1 || first.CaptureResult != sessionruntime.PrivateGroupingCaptureUnobserved {
		t.Fatal("ASSERT_CALLBACK_NO_ENTRY: FAIL")
	}
	r.enter()
	r.captured(sessionruntime.PrivateGroupingCaptureOK)
	r.called(context.Canceled)
	gap := r.freeze(sessionruntime.PrivateLeaseBoundaryObservation{})
	if gap.CaptureResult != sessionruntime.PrivateGroupingCaptureOK || gap.CallsResult != PrivatePrepareCheckpointCanceledV1 || gap.IssuanceResult != PrivatePrepareCheckpointUnobservedV1 {
		t.Fatal("ASSERT_CALLBACK_CALLS_GAP: FAIL")
	}
	t.Log("ASSERT_CALLBACK_NO_ENTRY: PASS; ASSERT_CALLBACK_CALLS_GAP: PASS")
}
func TestPrivatePrepareWindowPropagation(t *testing.T) {
	v := PrivatePrepareLeaseObservationV1{Boundary: sessionruntime.PrivateLeaseBoundaryContextDone, ResultObserved: PrivatePrepareObservedNoV1, CallbackSample: PrivatePrepareCallbackCallerPostReturnLockV1, CallbackEntered: PrivatePrepareObservedYesV1, CaptureResult: sessionruntime.PrivateGroupingCaptureUnobserved, CallsResult: PrivatePrepareCheckpointNotApplicableV1}
	expected := v
	base := privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPrivateMixedInputV2, sessionruntime.ErrPrivateProviderWindowUnjoined)
	e := privatePrepareFailureWithLeaseV1(base, v)
	v.CallbackEntered = PrivatePrepareObservedNoV1
	// Frozen attribution survives wrappers/cleanup; first scoped stage and error
	// sentinel identities remain the old ones, and an inner sample wins.
	e = privatePrepareFailureAtV1(privatePreparePacketV1, ErrPreparedDescribeV1, e)
	e = privatePrepareFailureWithLeaseV1(e, v)
	e = privatePrepareDeferredFinishV1(e, sessionruntime.ErrPrivateProviderWindowUnjoined)
	e = PrivatePrepareTerminalFailureV1(e, errors.Join(e, context.Canceled), nil, nil, context.Canceled, nil, nil)
	codes := ProjectPrivatePrepareFailureV1(e)
	if codes.Stage != privatePrepareCaptureV1 || codes.Reason != privatePrepareUnjoinedV1 || codes.Lease != expected || codes.Cleanup[0] != privatePrepareUnjoinedV1 || codes.Cleanup[3] != privatePrepareCanceledV1 || !errors.Is(e, ErrPreparedDescribeV1) || !errors.Is(e, ErrPrivateMixedInputV2) || !errors.Is(e, sessionruntime.ErrPrivateProviderWindowUnjoined) {
		t.Fatal("ASSERT_LEASE_PROPAGATION: FAIL")
	}
	codes.Lease.CallbackEntered = PrivatePrepareObservedNoV1
	if ProjectPrivatePrepareFailureV1(e).Lease != expected || ProjectPrivatePrepareFailureV1(base).Lease != (PrivatePrepareLeaseObservationV1{}) {
		t.Fatal("ASSERT_LEASE_PROPAGATION: FAIL detached")
	}
	_, early := ExecuteMixedInputV2(context.Background(), PrivateMixedInputOperationV2{}, PrivateMixedInputRequestV2{})
	if ProjectPrivatePrepareFailureV1(early).Lease != (PrivatePrepareLeaseObservationV1{}) {
		t.Fatal("ASSERT_LEASE_EARLY_UNOBSERVED: FAIL")
	}
	t.Log("ASSERT_LEASE_PROPAGATION: PASS; ASSERT_LEASE_EARLY_UNOBSERVED: PASS")
}

type prepareObservationOpaqueV1 struct {
	payload []byte
	calls   *atomic.Int32
}

func (e prepareObservationOpaqueV1) Error() string { e.calls.Add(1); panic("opaque Error") }
func (e prepareObservationOpaqueV1) Is(error) bool { e.calls.Add(1); panic("opaque Is") }
func (e prepareObservationOpaqueV1) Unwrap() error { e.calls.Add(1); panic("opaque Unwrap") }
func TestPrivatePrepareWindowOpaque(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Error("ASSERT_LEASE_OPAQUE: FAIL method invoked")
		}
	}()
	var calls atomic.Int32
	e := prepareObservationOpaqueV1{[]byte("never-output"), &calls}
	nodes := make([]error, 4096)
	for i := range nodes {
		nodes[i] = e
	}
	for _, err := range []error{e, errors.Join(nodes...)} {
		if privatePrepareCheckpointV1(err) != PrivatePrepareCheckpointUnknownV1 || ProjectPrivatePrepareFailureV1(err).Lease != (PrivatePrepareLeaseObservationV1{}) {
			t.Fatal("ASSERT_LEASE_OPAQUE: FAIL classification")
		}
		typed := privatePrepareFailureAtV1(privatePrepareCaptureV1, ErrPrivateMixedInputV2, err)
		if ProjectPrivatePrepareFailureV1(typed).Reason != privatePrepareRefusedV1 {
			t.Fatal("ASSERT_LEASE_OPAQUE: FAIL projection")
		}
	}
	if calls.Load() != 0 || PrivatePrepareCheckpointV1(255).String() != "UNKNOWN" || PrivatePrepareObservedV1(255).String() != "UNKNOWN" || PrivatePrepareCallbackSampleV1(255).String() != "UNKNOWN" {
		t.Fatal("ASSERT_LEASE_OPAQUE: FAIL")
	}
	t.Log("ASSERT_LEASE_OPAQUE: PASS")
}
func TestPrivatePrepareWindowAllowance(t *testing.T) {
	// Source contract: four one-assignment callback locks plus one caller lock;
	// no unknown work, waits, or observer invocation is added under these locks.
	f, err := parser.ParseFile(token.NewFileSet(), "private_mixed_input_operation_v2.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{"enter": true, "captured": true, "called": true, "issued": true, "freeze": true}
	locks := 0
	methods := 0
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !names[fn.Name.Name] || fn.Recv == nil {
			continue
		}
		methods++
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Lock" {
					locks++
				}
			}
			return true
		})
	}
	scalarSlots := 2 + 6 + reflect.TypeOf(PrivatePrepareLeaseObservationV1{}).NumField()
	// Capture/issuance/calls grouped declaration is three scalar fields in NumField.
	if methods != 5 || locks != 5 || scalarSlots != 16 {
		t.Fatal("ASSERT_OBSERVATION_ALLOWANCE: FAIL", methods, locks, scalarSlots)
	}
	var fixed func(reflect.Type) bool
	fixed = func(typ reflect.Type) bool {
		switch typ.Kind() {
		case reflect.Bool, reflect.Uint8:
			return true
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				if !fixed(typ.Field(i).Type) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if !fixed(reflect.TypeOf(PrivatePrepareLeaseObservationV1{})) || !fixed(reflect.TypeOf(sessionruntime.PrivateLeaseBoundaryObservation{})) {
		t.Fatal("ASSERT_OBSERVATION_ALLOWANCE: FAIL pointers")
	}
	t.Log("ASSERT_OBSERVATION_ALLOWANCE: PASS; caller-funded <=16 fixed updates/256 local steps, <=4 callback locks+1 caller lock; report separately <=256 steps/32 nodes/16 slots/4096 bytes/one Write; no native grant change")
}
