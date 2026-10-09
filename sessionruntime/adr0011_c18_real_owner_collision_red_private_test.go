package sessionruntime

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// The C18 barrier is observational synchronization only. It reports the real
// owner boundary and stable C15 identity, waits for the test permit, and may
// return only an infrastructure error. It cannot choose the arbitration winner.
type privateC18FixtureOwnerBoundary uint8

const (
	privateC18FixtureObjectEventCommit privateC18FixtureOwnerBoundary = iota + 1
	privateC18FixtureRetentionCommit
	privateC18FixtureReadbackBorrow
	privateC18FixtureRelease
)

type privateC18FixtureOwnerBarrierEvent struct {
	TransactionID string
	ObjectID      string
	Sequence      uint64
	Boundary      privateC18FixtureOwnerBoundary
}

type privateC18FixtureOwnerBarrier func(privateC18FixtureOwnerBarrierEvent) error

type privateC18ReflectAdapter struct {
	t     *testing.T
	value reflect.Value
	label string
}

func newPrivateC18ReflectAdapter(t *testing.T, label string, owner any) privateC18ReflectAdapter {
	t.Helper()
	value := reflect.ValueOf(owner)
	if !value.IsValid() || (value.Kind() == reflect.Pointer && value.IsNil()) {
		t.Fatalf("ASSERT_C18_%s_OWNER_ABSENT", label)
	}
	return privateC18ReflectAdapter{t: t, value: value, label: label}
}

func (a privateC18ReflectAdapter) requiredMethodNames(contracts map[string][2]int) (names, missing []string) {
	for name := range contracts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !a.value.MethodByName(name).IsValid() {
			missing = append(missing, name)
		}
	}
	return names, missing
}

func (a privateC18ReflectAdapter) require(t *testing.T, contracts map[string][2]int) {
	t.Helper()
	names, missing := a.requiredMethodNames(contracts)
	if len(missing) != 0 {
		t.Fatalf("ASSERT_C18_%s_PRODUCTION_METHODS_ABSENT methods=%v", a.label, missing)
	}
	for _, name := range names {
		arity := contracts[name]
		a.method(t, name, arity[0], arity[1])
	}
}

func (a privateC18ReflectAdapter) method(t *testing.T, name string, in, out int) reflect.Value {
	t.Helper()
	method := a.value.MethodByName(name)
	if !method.IsValid() {
		t.Fatalf("ASSERT_C18_%s_PRODUCTION_METHOD_ABSENT method=%s", a.label, name)
	}
	typeOf := method.Type()
	if typeOf.IsVariadic() || typeOf.NumIn() != in || typeOf.NumOut() != out {
		t.Fatalf("ASSERT_C18_%s_PRODUCTION_METHOD_SIGNATURE method=%s type=%v want_in=%d want_out=%d", a.label, name, typeOf, in, out)
	}
	return method
}

func privateC18ReflectCall(t *testing.T, method reflect.Value, args ...reflect.Value) []reflect.Value {
	t.Helper()
	for i, arg := range args {
		want := method.Type().In(i)
		if !arg.IsValid() || !arg.Type().AssignableTo(want) {
			t.Fatalf("ASSERT_C18_PRODUCTION_ARGUMENT_TYPE index=%d got=%v want=%v", i, privateC18ReflectType(arg), want)
		}
	}
	return method.Call(args)
}

func privateC18ReflectType(value reflect.Value) any {
	if !value.IsValid() {
		return "<invalid>"
	}
	return value.Type()
}

func privateC18PrimitiveValue(t *testing.T, want reflect.Type, value any) reflect.Value {
	t.Helper()
	got := reflect.ValueOf(value)
	if got.IsValid() && (got.Type() == want || want.Kind() == reflect.Interface && got.Type().AssignableTo(want)) {
		return got
	}
	t.Fatalf("ASSERT_C18_PRODUCTION_PRIMITIVE_TYPE got=%v want=%v", privateC18ReflectType(got), want)
	return reflect.Value{}
}

func privateC18NamedPrimitiveTypeError(got reflect.Type, name, pkgPath string, kind reflect.Kind, bits int) error {
	if got == nil || got.Name() != name || got.PkgPath() != pkgPath || got.Kind() != kind || privateC18TypeBits(got) != bits {
		return fmt.Errorf("ASSERT_C18_PRODUCTION_NAMED_PRIMITIVE_TYPE got=%v name=%q package=%q kind=%v bits=%d want=%s package=%q kind=%v bits=%d", got, privateC18TypeName(got), privateC18TypePkgPath(got), privateC18TypeKind(got), privateC18TypeBits(got), name, pkgPath, kind, bits)
	}
	return nil
}

func privateC18TypeName(value reflect.Type) string {
	if value == nil {
		return ""
	}
	return value.Name()
}
func privateC18TypePkgPath(value reflect.Type) string {
	if value == nil {
		return ""
	}
	return value.PkgPath()
}
func privateC18TypeKind(value reflect.Type) reflect.Kind {
	if value == nil {
		return reflect.Invalid
	}
	return value.Kind()
}
func privateC18TypeBits(value reflect.Type) int {
	if value == nil {
		return 0
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return value.Bits()
	default:
		return 0
	}
}

func privateC18BoundaryRangeError(raw uint64) error {
	if raw < uint64(privateC18FixtureObjectEventCommit) || raw > uint64(privateC18FixtureRelease) {
		return fmt.Errorf("ASSERT_C18_PRODUCTION_NAMED_PRIMITIVE_RANGE value=%d", raw)
	}
	return nil
}

func privateC18ProductionBoundary(t *testing.T, value reflect.Value) privateC18FixtureOwnerBoundary {
	t.Helper()
	wantPackage := reflect.TypeOf(privateC18FixtureOwnerBoundary(0)).PkgPath()
	if err := privateC18NamedPrimitiveTypeError(value.Type(), "privateC18OwnerBoundary", wantPackage, reflect.Uint8, 8); err != nil {
		t.Fatal(err)
	}
	raw := value.Uint()
	if err := privateC18BoundaryRangeError(raw); err != nil {
		t.Fatalf("%v type=%v", err, value.Type())
	}
	return privateC18FixtureOwnerBoundary(raw)
}

func privateC18ContenderRangeError(value privateC18FixtureContender, allowEmpty bool) error {
	switch value {
	case privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback:
		return nil
	case "":
		if allowEmpty {
			return nil
		}
	}
	return fmt.Errorf("ASSERT_C18_PRODUCTION_NAMED_PRIMITIVE_RANGE value=%q", value)
}

func privateC18ProductionContenderWithContext(t *testing.T, value reflect.Value, allowEmpty bool) privateC18FixtureContender {
	t.Helper()
	wantPackage := reflect.TypeOf(privateC18FixtureContender("")).PkgPath()
	if err := privateC18NamedPrimitiveTypeError(value.Type(), "privateC18Contender", wantPackage, reflect.String, 0); err != nil {
		t.Fatal(err)
	}
	contender := privateC18FixtureContender(value.String())
	if err := privateC18ContenderRangeError(contender, allowEmpty); err != nil {
		t.Fatalf("%v type=%v", err, value.Type())
	}
	return contender
}

func privateC18ProductionContender(t *testing.T, value reflect.Value) privateC18FixtureContender {
	return privateC18ProductionContenderWithContext(t, value, false)
}

func privateC18ProductionSnapshotContender(t *testing.T, value reflect.Value) privateC18FixtureContender {
	return privateC18ProductionContenderWithContext(t, value, true)
}

func privateC18ProductionBoundaryValue(t *testing.T, want reflect.Type, value privateC18FixtureOwnerBoundary) reflect.Value {
	t.Helper()
	wantPackage := reflect.TypeOf(value).PkgPath()
	if err := privateC18NamedPrimitiveTypeError(want, "privateC18OwnerBoundary", wantPackage, reflect.Uint8, 8); err != nil {
		t.Fatal(err)
	}
	if err := privateC18BoundaryRangeError(uint64(value)); err != nil {
		t.Fatalf("%v type=%v", err, want)
	}
	result := reflect.New(want).Elem()
	result.SetUint(uint64(value))
	return result
}

func privateC18ProductionContenderValue(t *testing.T, want reflect.Type, value privateC18FixtureContender) reflect.Value {
	t.Helper()
	wantPackage := reflect.TypeOf(value).PkgPath()
	if err := privateC18NamedPrimitiveTypeError(want, "privateC18Contender", wantPackage, reflect.String, 0); err != nil {
		t.Fatal(err)
	}
	switch value {
	case privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback:
	default:
		t.Fatalf("ASSERT_C18_PRODUCTION_NAMED_PRIMITIVE_RANGE type=%v value=%q", want, value)
	}
	result := reflect.New(want).Elem()
	result.SetString(string(value))
	return result
}

func privateC18ProductionStruct(t *testing.T, want reflect.Type, fields map[string]any) reflect.Value {
	t.Helper()
	if want.Kind() == reflect.Pointer {
		value := privateC18ProductionStruct(t, want.Elem(), fields)
		pointer := reflect.New(want.Elem())
		pointer.Elem().Set(value)
		return pointer
	}
	if want.Kind() != reflect.Struct {
		t.Fatalf("ASSERT_C18_PRODUCTION_STRUCT_TYPE got=%v", want)
	}
	value := reflect.New(want).Elem()
	if want.NumField() != len(fields) {
		t.Fatalf("ASSERT_C18_PRODUCTION_STRUCT_FIELD_COUNT type=%v got=%d want=%d", want, want.NumField(), len(fields))
	}
	for name, primitive := range fields {
		field, ok := want.FieldByName(name)
		if !ok || field.PkgPath != "" {
			t.Fatalf("ASSERT_C18_PRODUCTION_STRUCT_FIELD type=%v field=%s", want, name)
		}
		destination := value.FieldByIndex(field.Index)
		destination.Set(privateC18PrimitiveValue(t, destination.Type(), primitive))
	}
	return value
}

func privateC18ExtractStruct(t *testing.T, value reflect.Value, fields map[string]any, exactFieldCount ...int) {
	t.Helper()
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_NIL type=%v", value.Type())
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_STRUCT got=%v", value.Type())
	}
	wantFieldCount := len(fields)
	if len(exactFieldCount) == 1 {
		wantFieldCount = exactFieldCount[0]
	} else if len(exactFieldCount) > 1 {
		t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_FIELD_COUNT_ARGUMENT got=%d", len(exactFieldCount))
	}
	if value.NumField() != wantFieldCount {
		t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_FIELD_COUNT type=%v got=%d want=%d", value.Type(), value.NumField(), wantFieldCount)
	}
	for name, destination := range fields {
		field := value.FieldByName(name)
		if !field.IsValid() || !field.CanInterface() {
			t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_FIELD type=%v field=%s", value.Type(), name)
		}
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Pointer || target.IsNil() {
			t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_DESTINATION field=%s want_pointer=%v", name, target.Type())
		}
		want := target.Elem().Type()
		if field.Type() != want && (want.Kind() != reflect.Interface || !field.Type().AssignableTo(want)) {
			t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_FIELD_TYPE field=%s got=%v want=%v", name, field.Type(), want)
		}
		target.Elem().Set(field)
	}
}

type privateC18FixtureContender string

const (
	privateC18FixtureContenderObjectEvent privateC18FixtureContender = "object-event"
	privateC18FixtureContenderRetention   privateC18FixtureContender = "retention"
	privateC18FixtureContenderReadback    privateC18FixtureContender = "readback"
)

type privateC18Result struct {
	Contender         privateC18FixtureContender
	Stage             privateC18FixtureOwnerBoundary
	Failure           session.Failure
	Err               error
	NotificationLease RetainedNotificationLease
	ResponseLease     RetainedResponseLease
}

const privateC18HarnessTimeout = time.Second

type privateC18TaskResult[T any] struct {
	Value T
	Err   error
}

type privateC18TaskError struct {
	err      error
	consumed bool
}

type privateC18TaskGroup struct {
	mu     sync.Mutex
	active int
	closed bool
	done   chan struct{}
	abort  []func()
	errs   []*privateC18TaskError
}

func newPrivateC18TaskGroup(abort ...func()) *privateC18TaskGroup {
	return &privateC18TaskGroup{done: make(chan struct{}), abort: abort}
}

func (g *privateC18TaskGroup) Start(fn func() error) <-chan error {
	result := make(chan error, 1)
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		result <- errors.New("C18 task group closed")
		close(result)
		return result
	}
	g.active++
	g.mu.Unlock()
	go func() {
		err := fn()
		g.mu.Lock()
		if err != nil {
			g.errs = append(g.errs, &privateC18TaskError{err: err})
		}
		result <- err
		g.active--
		if g.closed && g.active == 0 {
			select {
			case <-g.done:
			default:
				close(g.done)
			}
		}
		g.mu.Unlock()
	}()
	return result
}

func privateC18StartValue[T any](g *privateC18TaskGroup, fn func() (T, error)) <-chan privateC18TaskResult[T] {
	result := make(chan privateC18TaskResult[T], 1)
	g.Start(func() error {
		value, err := fn()
		result <- privateC18TaskResult[T]{Value: value, Err: err}
		return err
	})
	return result
}

func (g *privateC18TaskGroup) Acknowledge(err error) bool {
	if err == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, reported := range g.errs {
		if !reported.consumed && errors.Is(reported.err, err) {
			reported.consumed = true
			return true
		}
	}
	return false
}

func privateC18OwnTaskGroup(t *testing.T, abort ...func()) *privateC18TaskGroup {
	t.Helper()
	group := newPrivateC18TaskGroup(abort...)
	t.Cleanup(func() {
		if err := group.Close(); err != nil {
			t.Errorf("%v", err)
		}
	})
	return group
}

func (g *privateC18TaskGroup) Close() error {
	g.mu.Lock()
	if !g.closed {
		g.closed = true
		for _, abort := range g.abort {
			abort()
		}
		if g.active == 0 {
			close(g.done)
		}
	}
	done := g.done
	g.mu.Unlock()
	select {
	case <-done:
		g.mu.Lock()
		var unconsumed []error
		for _, reported := range g.errs {
			if !reported.consumed {
				unconsumed = append(unconsumed, reported.err)
			}
		}
		err := errors.Join(unconsumed...)
		g.mu.Unlock()
		return err
	case <-time.After(privateC18HarnessTimeout):
		return errors.New("ASSERT_C18_TASK_JOIN_TIMEOUT")
	}
}

type privateC18GateScope struct {
	once            sync.Once
	decisionRelease chan struct{}
	effectRelease   chan struct{}
}

func newPrivateC18GateScope() *privateC18GateScope {
	return &privateC18GateScope{decisionRelease: make(chan struct{}), effectRelease: make(chan struct{})}
}
func (g *privateC18GateScope) ReleaseAll() {
	g.once.Do(func() { close(g.decisionRelease); close(g.effectRelease) })
}

type privateC18ReachedBarrier struct {
	mu       sync.Mutex
	cond     *sync.Cond
	reached  chan privateC18FixtureOwnerBarrierEvent
	permit   map[privateC18FixtureOwnerBoundary]chan struct{}
	aborted  chan struct{}
	closed   bool
	released map[privateC18FixtureOwnerBoundary]bool
	events   []privateC18FixtureOwnerBarrierEvent
	active   int
}

func newPrivateC18ReachedBarrier() *privateC18ReachedBarrier {
	b := &privateC18ReachedBarrier{
		reached: make(chan privateC18FixtureOwnerBarrierEvent, 4),
		aborted: make(chan struct{}),
		permit: map[privateC18FixtureOwnerBoundary]chan struct{}{
			privateC18FixtureObjectEventCommit: make(chan struct{}),
			privateC18FixtureRetentionCommit:   make(chan struct{}),
			privateC18FixtureReadbackBorrow:    make(chan struct{}),
			privateC18FixtureRelease:           make(chan struct{}),
		},
		released: make(map[privateC18FixtureOwnerBoundary]bool),
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *privateC18ReachedBarrier) observe(event privateC18FixtureOwnerBarrierEvent) error {
	if event.TransactionID == "" || event.ObjectID == "" || event.Sequence == 0 {
		return errors.New("invalid C18 owner-barrier metadata")
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("C18 owner barrier aborted")
	}
	permit, ok := b.permit[event.Boundary]
	if !ok {
		b.mu.Unlock()
		return errors.New("unknown C18 owner boundary")
	}
	alreadyReleased := b.released[event.Boundary]
	b.active++
	b.events = append(b.events, event)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.active--
		b.cond.Broadcast()
		b.mu.Unlock()
	}()
	select {
	case b.reached <- event:
	case <-b.aborted:
		return errors.New("C18 owner barrier aborted")
	}
	if alreadyReleased {
		return nil
	}
	select {
	case <-permit:
	case <-b.aborted:
	}
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return errors.New("C18 owner barrier aborted")
	}
	return nil
}

func (b *privateC18ReachedBarrier) await(t *testing.T, want privateC18FixtureOwnerBoundary, labels ...string) privateC18FixtureOwnerBarrierEvent {
	t.Helper()
	label := "REAL_OWNER_REACHED"
	if len(labels) != 0 {
		label = labels[0]
	}
	event := privateC18Receive(t, b.reached, label)
	if event.Boundary != want {
		t.Fatalf("ASSERT_C18_REAL_OWNER_REACHED want=%d got=%+v", want, event)
	}
	return event
}

func (b *privateC18ReachedBarrier) release(point privateC18FixtureOwnerBoundary) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if permit, ok := b.permit[point]; ok && !b.released[point] {
		close(permit)
		b.released[point] = true
	}
}

func (b *privateC18ReachedBarrier) Abort() {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.aborted)
		for point, permit := range b.permit {
			if !b.released[point] {
				close(permit)
				b.released[point] = true
			}
		}
	}
	for b.active != 0 {
		b.cond.Wait()
	}
	b.mu.Unlock()
}

func (b *privateC18ReachedBarrier) CloseAll() { b.Abort() }

func privateC18OwnBarrier(t *testing.T, barrier *privateC18ReachedBarrier) *privateC18ReachedBarrier {
	t.Helper()
	t.Cleanup(barrier.Abort)
	return barrier
}

func privateC18Receive[T any](t *testing.T, source <-chan T, label string) T {
	t.Helper()
	select {
	case value := <-source:
		return value
	case <-time.After(privateC18HarnessTimeout):
		t.Fatalf("ASSERT_C18_%s_RECEIVE_TIMEOUT", label)
		var zero T
		return zero
	}
}

func TestADR0011C18ReachedBarrierOneShotFixtureAllocationsAreFresh(t *testing.T) {
	first := newPrivateC18ReachedBarrier()
	second := newPrivateC18ReachedBarrier()
	if first == second {
		t.Fatal("ASSERT_C18_FRESH_BARRIER_ALLOCATION")
	}
	for _, boundary := range []privateC18FixtureOwnerBoundary{privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow, privateC18FixtureRelease} {
		if first.permit[boundary] == second.permit[boundary] {
			t.Fatalf("ASSERT_C18_FRESH_ONE_SHOT_PERMIT boundary=%d", boundary)
		}
	}
	first.release(privateC18FixtureRelease)
	select {
	case <-first.permit[privateC18FixtureRelease]:
	default:
		t.Fatal("ASSERT_C18_RELEASE_CLOSES_ONE_SHOT_PERMIT")
	}
	select {
	case <-second.permit[privateC18FixtureRelease]:
		t.Fatal("ASSERT_C18_FRESH_BARRIER_INHERITED_CLOSED_PERMIT")
	default:
	}
	// Failure cleanup may race ordinary release; repeated closure is therefore
	// deliberately idempotent rather than a panic-based one-shot contract.
	first.release(privateC18FixtureRelease)
	first.Abort()
	first.CloseAll()
	select {
	case <-first.permit[privateC18FixtureRelease]:
	default:
		t.Fatal("ASSERT_C18_IDEMPOTENT_CLOSE_ALL_LEFT_PERMIT_BLOCKED")
	}
}

func TestADR0011C18ReachedBarrierRepeatedReleasedBoundaryPublishesAndAbortRejects(t *testing.T) {
	const occurrences = 64
	boundary := privateC18FixtureReadbackBorrow
	barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
	barrier.release(boundary)

	want := make([]privateC18FixtureOwnerBarrierEvent, 0, occurrences)
	for sequence := 1; sequence <= occurrences; sequence++ {
		event := privateC18FixtureOwnerBarrierEvent{
			TransactionID: "pair9-repeat",
			ObjectID:      "pair9-owner",
			Sequence:      uint64(sequence),
			Boundary:      boundary,
		}
		if err := barrier.observe(event); err != nil {
			t.Fatalf("ASSERT_C18_RELEASED_REPEAT_OBSERVE sequence=%d err=%v", sequence, err)
		}
		got := privateC18Receive(t, barrier.reached, "RELEASED_REPEAT_PHYSICAL_EVENT")
		if !reflect.DeepEqual(got, event) {
			t.Fatalf("ASSERT_C18_RELEASED_REPEAT_PHYSICAL_EVENT sequence=%d got=%+v want=%+v", sequence, got, event)
		}
		want = append(want, event)
	}
	if got := barrier.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_C18_RELEASED_REPEAT_CARDINALITY_ORDER got=%+v want=%+v", got, want)
	}

	aborted := newPrivateC18ReachedBarrier()
	aborted.Abort()
	abortedEvent := privateC18FixtureOwnerBarrierEvent{
		TransactionID: "pair9-abort",
		ObjectID:      "pair9-owner",
		Sequence:      1,
		Boundary:      boundary,
	}
	if err := aborted.observe(abortedEvent); err == nil {
		t.Fatal("ASSERT_C18_ABORT_OBSERVE_ACCEPTED")
	}
	select {
	case got := <-aborted.reached:
		t.Fatalf("ASSERT_C18_ABORT_PUBLISHED_REACHED_EVENT got=%+v", got)
	default:
	}
	if got := aborted.snapshot(); len(got) != 0 {
		t.Fatalf("ASSERT_C18_ABORT_RECORDED_PHYSICAL_EVENT got=%+v", got)
	}
}

func (b *privateC18ReachedBarrier) awaitPairAnyOrder(t *testing.T, wantA, wantB privateC18FixtureOwnerBoundary, labels ...string) [2]privateC18FixtureOwnerBarrierEvent {
	t.Helper()
	var got [2]privateC18FixtureOwnerBarrierEvent
	seen := map[privateC18FixtureOwnerBoundary]bool{}
	for i := range got {
		if len(labels) == 0 {
			select {
			case got[i] = <-b.reached:
			case <-t.Context().Done():
				t.Fatalf("BLOCKED_NOT_RED simultaneous owner boundaries were not both reached: %v", t.Context().Err())
			}
		} else {
			got[i] = privateC18Receive(t, b.reached, fmt.Sprintf("%s_%s", labels[0], []string{"FIRST", "SECOND"}[i]))
		}
		if got[i].Boundary != wantA && got[i].Boundary != wantB {
			t.Fatalf("ASSERT_C18_SIMULTANEOUS_UNEXPECTED_BOUNDARY got=%+v want={%d,%d}", got[i], wantA, wantB)
		}
		if seen[got[i].Boundary] {
			t.Fatalf("ASSERT_C18_SIMULTANEOUS_DUPLICATE_BOUNDARY got=%+v", got)
		}
		seen[got[i].Boundary] = true
	}
	return got
}

func (b *privateC18ReachedBarrier) snapshot() []privateC18FixtureOwnerBarrierEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]privateC18FixtureOwnerBarrierEvent(nil), b.events...)
}

func (b *privateC18ReachedBarrier) assertExactCounts(t *testing.T, want map[privateC18FixtureOwnerBoundary]int) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	got := make(map[privateC18FixtureOwnerBoundary]int, len(b.events))
	for _, event := range b.events {
		got[event.Boundary]++
	}
	for _, boundary := range []privateC18FixtureOwnerBoundary{privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow, privateC18FixtureRelease} {
		if got[boundary] != want[boundary] {
			t.Fatalf("ASSERT_C18_EXACT_BOUNDARY_COUNT boundary=%d got=%d want=%d duplicate=%d events=%+v", boundary, got[boundary], want[boundary], max(0, got[boundary]-want[boundary]), b.events)
		}
	}
}

func installPrivateC18Barrier(t *testing.T, owner any, callback privateC18FixtureOwnerBarrier) func() {
	t.Helper()
	adapter := newPrivateC18ReflectAdapter(t, "OWNER_BARRIER", owner)
	method := adapter.method(t, "InstallPrivateC18OwnerBarrierForTest", 1, 1)
	callbackType := method.Type().In(0)
	if callbackType.Kind() != reflect.Func || callbackType.NumIn() != 1 || callbackType.NumOut() != 1 || callbackType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		t.Fatalf("ASSERT_C18_OWNER_BARRIER_CALLBACK_SIGNATURE type=%v", callbackType)
	}
	currentPackage := reflect.TypeOf(privateC18FixtureOwnerBarrierEvent{}).PkgPath()
	eventType := callbackType.In(0)
	if eventType.Kind() != reflect.Struct || eventType.Name() != "privateC18OwnerBarrierEvent" || eventType.PkgPath() != currentPackage || eventType.NumField() != 4 {
		t.Fatalf("ASSERT_C18_OWNER_BARRIER_EVENT_TYPE type=%v name=%q package=%q fields=%d", eventType, eventType.Name(), eventType.PkgPath(), eventType.NumField())
	}
	wantFields := []struct {
		name   string
		typeOf reflect.Type
	}{
		{name: "TransactionID", typeOf: reflect.TypeOf("")},
		{name: "ObjectID", typeOf: reflect.TypeOf("")},
		{name: "Sequence", typeOf: reflect.TypeOf(uint64(0))},
	}
	for index, want := range wantFields {
		field := eventType.Field(index)
		if field.Name != want.name || field.PkgPath != "" || field.Type != want.typeOf {
			t.Fatalf("ASSERT_C18_OWNER_BARRIER_EVENT_FIELD index=%d got=%s:%v package=%q want=%s:%v", index, field.Name, field.Type, field.PkgPath, want.name, want.typeOf)
		}
	}
	boundary := eventType.Field(3)
	if boundary.Name != "Boundary" || boundary.PkgPath != "" || boundary.Type.Kind() != reflect.Uint8 || boundary.Type.Name() != "privateC18OwnerBoundary" || boundary.Type.PkgPath() != currentPackage {
		t.Fatalf("ASSERT_C18_OWNER_BARRIER_EVENT_FIELD index=3 got=%s:%v package=%q want=Boundary:privateC18OwnerBoundary", boundary.Name, boundary.Type, boundary.PkgPath)
	}
	wrapped := reflect.Zero(callbackType)
	if callback != nil {
		wrapped = reflect.MakeFunc(callbackType, func(args []reflect.Value) []reflect.Value {
			event, err := privateC18DecodeBarrierEvent(args[0])
			if err == nil {
				err = callback(event)
			}
			if err == nil {
				return []reflect.Value{reflect.Zero(callbackType.Out(0))}
			}
			return []reflect.Value{reflect.ValueOf(err)}
		})
	}
	if gotNil := wrapped.IsNil(); gotNil != (callback == nil) {
		t.Fatalf("ASSERT_C18_OWNER_BARRIER_EXACT_NIL_INSTALLATION got_nil=%t callback_nil=%t type=%v", gotNil, callback == nil, callbackType)
	}
	result := privateC18ReflectCall(t, method, wrapped)
	restoreType := result[0].Type()
	if restoreType.Kind() != reflect.Func || restoreType.NumIn() != 0 || restoreType.NumOut() != 0 || result[0].IsNil() {
		t.Fatalf("ASSERT_C18_OWNER_BARRIER_RESTORE_SIGNATURE type=%v nil=%t", restoreType, result[0].IsNil())
	}
	var once sync.Once
	restore := func() { once.Do(func() { result[0].Call(nil) }) }
	// Cleanup registration order is installation order, so testing executes
	// nested/repeated restores in the required LIFO order.
	t.Cleanup(restore)
	return restore
}

func privateC18DecodeBarrierEvent(value reflect.Value) (privateC18FixtureOwnerBarrierEvent, error) {
	var event privateC18FixtureOwnerBarrierEvent
	if value.Kind() != reflect.Struct || value.NumField() != 4 {
		return event, fmt.Errorf("ASSERT_C18_WORKER_BARRIER_EVENT_SHAPE type=%v", value.Type())
	}
	transaction, object, sequence, boundary := value.FieldByName("TransactionID"), value.FieldByName("ObjectID"), value.FieldByName("Sequence"), value.FieldByName("Boundary")
	if !transaction.IsValid() || transaction.Kind() != reflect.String || !object.IsValid() || object.Kind() != reflect.String || !sequence.IsValid() || sequence.Kind() != reflect.Uint64 || !boundary.IsValid() || boundary.Kind() != reflect.Uint8 {
		return event, fmt.Errorf("ASSERT_C18_WORKER_BARRIER_EVENT_FIELDS type=%v", value.Type())
	}
	event.TransactionID, event.ObjectID, event.Sequence = transaction.String(), object.String(), sequence.Uint()
	event.Boundary = privateC18FixtureOwnerBoundary(boundary.Uint())
	if err := privateC18BoundaryRangeError(uint64(event.Boundary)); err != nil {
		return privateC18FixtureOwnerBarrierEvent{}, err
	}
	return event, nil
}

func privateC18ResultsByContender(t *testing.T, result <-chan privateC18Result, want ...privateC18FixtureContender) map[privateC18FixtureContender]privateC18Result {
	t.Helper()
	got := make(map[privateC18FixtureContender]privateC18Result, len(want))
	for range want {
		r := privateC18Receive(t, result, "CONTENDER_RESULT")
		if r.Contender == "" || r.Stage == 0 {
			t.Fatalf("ASSERT_C18_RESULT_IDENTITY_MISSING result=%+v", r)
		}
		if _, duplicate := got[r.Contender]; duplicate {
			t.Fatalf("ASSERT_C18_DUPLICATE_CONTENDER_RESULT contender=%q", r.Contender)
		}
		got[r.Contender] = r
	}
	for _, contender := range want {
		if _, ok := got[contender]; !ok {
			t.Fatalf("ASSERT_C18_CONTENDER_RESULT_MISSING contender=%q got=%+v", contender, got)
		}
	}
	return got
}

type privateC18C15Tuple struct {
	SiteID   string
	Kind     string
	Terminal bool
}

type privateC18ExpectedBoundary struct {
	Boundary privateC18FixtureOwnerBoundary
	Tuple    privateC18C15Tuple
}

func privateC18RelevantC15Events(t *testing.T, observer *privateC15AtomicBatchObserver, reached []privateC18FixtureOwnerBarrierEvent, expected []privateC18ExpectedBoundary) []privateB4C15Event {
	t.Helper()
	if len(reached) == 0 || len(expected) == 0 || len(expected) > len(reached) {
		t.Fatalf("ASSERT_C18_BARRIER_EXPECTATION_CARDINALITY reached=%+v expected=%+v", reached, expected)
	}
	byBoundary := make(map[privateC18FixtureOwnerBoundary]privateC18C15Tuple, len(expected))
	for _, want := range expected {
		if _, duplicate := byBoundary[want.Boundary]; duplicate {
			t.Fatalf("ASSERT_C18_DUPLICATE_EXPECTED_BOUNDARY boundary=%d expected=%+v", want.Boundary, expected)
		}
		byBoundary[want.Boundary] = want.Tuple
	}
	snapshot := observer.snapshot()
	relevant := make([]privateB4C15Event, 0, len(reached))
	used := make(map[int]bool, len(reached))
	seenBoundary := make(map[privateC18FixtureOwnerBoundary]bool, len(reached))
	for _, barrier := range reached {
		want, hasEffect := byBoundary[barrier.Boundary]
		if seenBoundary[barrier.Boundary] {
			t.Fatalf("ASSERT_C18_EXACT_BOUNDARY_IDENTITY boundary=%d duplicate=true reached=%+v expected=%+v", barrier.Boundary, reached, expected)
		}
		seenBoundary[barrier.Boundary] = true
		if !hasEffect {
			forbiddenKind := ""
			switch barrier.Boundary {
			case privateC18FixtureRetentionCommit:
				forbiddenKind = "retain"
			case privateC18FixtureReadbackBorrow:
				forbiddenKind = "readback"
			}
			for _, committed := range snapshot {
				if committed.TransactionID == barrier.TransactionID && committed.ObjectID == barrier.ObjectID && committed.Kind == forbiddenKind {
					t.Fatalf("ASSERT_C18_LOSING_BOUNDARY_HAS_C15_EFFECT boundary=%d barrier=%+v event=%+v", barrier.Boundary, barrier, committed)
				}
			}
			continue
		}
		matches := 0
		for j, committed := range snapshot {
			if committed.TransactionID != barrier.TransactionID || committed.ObjectID != barrier.ObjectID || committed.Sequence != barrier.Sequence {
				continue
			}
			if committed.SiteID == want.SiteID && committed.Kind == want.Kind && committed.Terminal == want.Terminal {
				matches++
				if used[j] {
					t.Fatalf("ASSERT_C18_BARRIER_EVENT_BINDING_NOT_ONE_TO_ONE barrier=%+v event=%+v", barrier, committed)
				}
				used[j] = true
				relevant = append(relevant, committed)
			}
		}
		if matches != 1 {
			t.Fatalf("ASSERT_C18_BARRIER_EVENT_EXACT_TUPLE_MATCH boundary=%d barrier=%+v want=%+v matches=%d snapshot=%+v", barrier.Boundary, barrier, want, matches, snapshot)
		}
	}
	for boundary := range byBoundary {
		if !seenBoundary[boundary] {
			t.Fatalf("ASSERT_C18_EXPECTED_EFFECT_BOUNDARY_NOT_REACHED boundary=%d reached=%+v", boundary, reached)
		}
	}
	return relevant
}

func privateC18AssertExactAccounting(t *testing.T, snapshot []privateB4C15Event, expected, acceptedCreations []privateC18C15Tuple, expectedObject string) {
	t.Helper()
	if expectedObject == "" {
		t.Fatal("ASSERT_C18_EXPECTED_OBJECT_EMPTY")
	}
	want := make(map[privateC18C15Tuple]int, len(expected))
	for _, tuple := range expected {
		if tuple.SiteID == "" || tuple.Kind == "" {
			t.Fatalf("ASSERT_C18_INVALID_EXPECTED_SEMANTIC_TUPLE tuple=%+v", tuple)
		}
		want[tuple]++
	}
	creationTuples := make(map[privateC18C15Tuple]bool, len(acceptedCreations))
	for _, tuple := range acceptedCreations {
		if want[tuple] == 0 || tuple.Terminal {
			t.Fatalf("ASSERT_C18_INVALID_ACCEPTED_CREATION_TUPLE tuple=%+v expected=%+v", tuple, expected)
		}
		creationTuples[tuple] = true
	}
	got := make(map[privateC18C15Tuple]int, len(snapshot))
	created := make(map[string]int)
	terminals := make(map[string]int)
	for _, event := range snapshot {
		tuple := privateC18C15Tuple{SiteID: event.SiteID, Kind: event.Kind, Terminal: event.Terminal}
		if want[tuple] == 0 {
			t.Fatalf("ASSERT_C18_UNKNOWN_SEMANTIC_TUPLE tuple=%+v event=%+v expected=%+v", tuple, event, expected)
		}
		got[tuple]++
		if creationTuples[tuple] {
			created[event.ObjectID]++
		}
		if event.Terminal {
			terminals[event.ObjectID]++
			if event.Kind != "release" {
				t.Fatalf("ASSERT_C18_TERMINAL_NOT_RELEASE event=%+v", event)
			}
		}
	}
	for tuple, count := range want {
		if got[tuple] != count {
			t.Fatalf("ASSERT_C18_EXACT_SEMANTIC_TUPLE_COUNT tuple=%+v got=%d want=%d snapshot=%+v", tuple, got[tuple], count, snapshot)
		}
	}
	if len(snapshot) != len(expected) {
		t.Fatalf("ASSERT_C18_EXACT_SEMANTIC_EVENT_TOTAL got=%d want=%d snapshot=%+v", len(snapshot), len(expected), snapshot)
	}
	if created[expectedObject] != 1 {
		t.Fatalf("ASSERT_C18_EXPECTED_OBJECT_CREATED_EXACTLY_ONCE object=%q got=%d created=%v snapshot=%+v", expectedObject, created[expectedObject], created, snapshot)
	}
	for object, count := range created {
		if count != 1 || terminals[object] != 1 {
			t.Fatalf("ASSERT_C18_CREATED_OBJECT_EXACTLY_ONE_TERMINAL_RELEASE object=%q creations=%d terminals=%d snapshot=%+v", object, count, terminals[object], snapshot)
		}
	}
	for object, count := range terminals {
		if created[object] == 0 {
			t.Fatalf("ASSERT_C18_TERMINAL_FOR_UNCREATED_OBJECT object=%q terminals=%d snapshot=%+v", object, count, snapshot)
		}
	}
}

func privateC18AssertOneWinnerOneTypedLoser(t *testing.T, results map[privateC18FixtureContender]privateC18Result, loser privateC18FixtureContender, typedFailure session.Failure) {
	t.Helper()
	winners, losers := 0, 0
	for contender, result := range results {
		if result.Failure == "" && result.Err == nil {
			winners++
			continue
		}
		if contender == loser && result.Failure == typedFailure {
			losers++
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("ASSERT_C18_EXACT_ARBITRATION winners=%d typed_losers=%d want_loser=%q want_failure=%q results=%+v", winners, losers, loser, typedFailure, results)
	}
}

func TestADR0011C18ObjectEventBeforeRetentionRealOwnerCollision(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)
	owner, failure := newPrivateB4RetainedNotificationOwner(account, notificationFixture())
	if failure != "" || owner == nil {
		t.Fatalf("BLOCKED_NOT_RED retained notification owner: failure=%q owner=%v", failure, owner)
	}
	host := newPrivateC18ReflectAdapter(t, "NOTIFICATION_COLLISION", owner)
	host.require(t, map[string][2]int{"CommitPrivateC18ObjectEventForTest": {1, 1}, "CommitPrivateC18RetentionForTest": {0, 2}, "InstallPrivateC18OwnerBarrierForTest": {1, 1}})
	barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
	installPrivateC18Barrier(t, owner, barrier.observe)
	group := privateC18OwnTaskGroup(t, barrier.Abort)
	objectEventCall := privateC18PrepareLegacyPair8ObjectEvent(t, host)
	retentionCall := privateC18PrepareLegacyPair8Retention(t, host)
	result := make(chan privateC18Result, 2)
	group.Start(func() error {
		failure, err := objectEventCall(privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
		result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: failure, Err: err}
		return err
	})
	objectEvent := barrier.await(t, privateC18FixtureObjectEventCommit)
	group.Start(func() error {
		lease, failure, err := retentionCall()
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: failure, Err: err, NotificationLease: lease}
		return err
	})
	retention := barrier.await(t, privateC18FixtureRetentionCommit)
	barrier.release(privateC18FixtureObjectEventCommit)
	barrier.release(privateC18FixtureRetentionCommit)
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	privateC18AssertOneWinnerOneTypedLoser(t, results, privateC18FixtureContenderRetention, session.ResourceExhausted)
	privateC18RelevantC15Events(t, observer, []privateC18FixtureOwnerBarrierEvent{objectEvent, retention}, []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureObjectEventCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "publication"}},
	})
	if got := results[privateC18FixtureContenderObjectEvent].Stage; got != privateC18FixtureObjectEventCommit {
		t.Fatalf("ASSERT_C18_OBJECT_EVENT_STAGE got=%d", got)
	}
	lease := RetainedNotificationLease{owner: owner}
	released := privateC18StartValue(group, privateC18PrepareNotificationRelease(lease))
	release := barrier.await(t, privateC18FixtureRelease)
	barrier.release(privateC18FixtureRelease)
	releaseResult := privateC18Receive(t, released, "NOTIFICATION_RELEASE")
	if releaseResult.Err != nil || !releaseResult.Value || lease.Release() {
		t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
	}
	reached := []privateC18FixtureOwnerBarrierEvent{objectEvent, retention, release}
	boundaries := []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureObjectEventCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "publication"}},
		{Boundary: privateC18FixtureRelease, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "release", Terminal: true}},
	}
	privateC18RelevantC15Events(t, observer, reached, boundaries)
	creation := privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "copy"}
	privateC18AssertExactAccounting(t, observer.snapshot(), []privateC18C15Tuple{
		creation,
		boundaries[0].Tuple,
		boundaries[1].Tuple,
	}, []privateC18C15Tuple{creation}, release.ObjectID)
	barrier.assertExactCounts(t, map[privateC18FixtureOwnerBoundary]int{
		privateC18FixtureObjectEventCommit: 1,
		privateC18FixtureRetentionCommit:   1,
		privateC18FixtureReadbackBorrow:    0,
		privateC18FixtureRelease:           1,
	})
}

func TestADR0011C18RetentionBeforeReadbackRealOwnerCollision(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)
	owner, failure := newPrivateB4RetainedResponseOwner(account, retainedResponseFixture()[:1])
	if failure != "" || owner == nil {
		t.Fatalf("BLOCKED_NOT_RED retained response owner: failure=%q owner=%v", failure, owner)
	}
	host := privateC18TestPair9CoordinatorAdapter{newPrivateC18ReflectAdapter(t, "RESPONSE_COLLISION", owner)}
	host.require(t, map[string][2]int{"CommitPrivateC18RetentionForTest": {0, 2}, "BorrowPrivateC18ReadbackForPair9Test": {2, 2}, "InstallPrivateC18OwnerBarrierForTest": {1, 1}})
	barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
	installPrivateC18Barrier(t, owner, barrier.observe)
	var readbacks int
	group := privateC18OwnTaskGroup(t, barrier.Abort)
	retentionCall := privateC18PrepareLegacyPair9Retention(t, host.privateC18ReflectAdapter)
	readbackCall := privateC18PreparePair9ReadbackCall(t, host)
	result := make(chan privateC18Result, 2)
	group.Start(func() error {
		retainedLease, failure, err := retentionCall()
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: failure, Err: err, ResponseLease: retainedLease}
		return err
	})
	retention := barrier.await(t, privateC18FixtureRetentionCommit)
	group.Start(func() error {
		operation, err := readbackCall(context.Background(), func([]lspwire.Message) error { readbacks++; return nil })
		if err == nil {
			err = operation.Err
		}
		result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
		return err
	})
	readback := barrier.await(t, privateC18FixtureReadbackBorrow)
	barrier.release(privateC18FixtureRetentionCommit)
	barrier.release(privateC18FixtureReadbackBorrow)
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
	if results[privateC18FixtureContenderRetention].Failure != "" || !errors.Is(results[privateC18FixtureContenderReadback].Err, ErrRetainedResponseLeaseClosed) || readbacks != 0 {
		t.Fatalf("ASSERT_C18_EXACT_RETENTION_READBACK_ARBITRATION results=%+v readbacks=%d", results, readbacks)
	}
	if !group.Acknowledge(results[privateC18FixtureContenderReadback].Err) {
		t.Fatal("ASSERT_C18_EXPECTED_CLOSED_READBACK_ERROR_NOT_ACKNOWLEDGED")
	}
	privateC18RelevantC15Events(t, observer, []privateC18FixtureOwnerBarrierEvent{retention, readback}, []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureRetentionCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "retain"}},
	})
	lease := results[privateC18FixtureContenderRetention].ResponseLease
	released := privateC18StartValue(group, privateC18PrepareResponseRelease(lease))
	release := barrier.await(t, privateC18FixtureRelease)
	barrier.release(privateC18FixtureRelease)
	releaseResult := privateC18Receive(t, released, "RESPONSE_RELEASE")
	if releaseResult.Err != nil || !releaseResult.Value || lease.Release() {
		t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
	}
	reached := []privateC18FixtureOwnerBarrierEvent{retention, readback, release}
	boundaries := []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureRetentionCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "retain"}},
		{Boundary: privateC18FixtureRelease, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "release", Terminal: true}},
	}
	privateC18RelevantC15Events(t, observer, reached, boundaries)
	creation := privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "copy"}
	privateC18AssertExactAccounting(t, observer.snapshot(), []privateC18C15Tuple{
		creation,
		boundaries[0].Tuple,
		boundaries[1].Tuple,
	}, []privateC18C15Tuple{creation}, release.ObjectID)
	barrier.assertExactCounts(t, map[privateC18FixtureOwnerBoundary]int{
		privateC18FixtureObjectEventCommit: 0,
		privateC18FixtureRetentionCommit:   1,
		privateC18FixtureReadbackBorrow:    1,
		privateC18FixtureRelease:           1,
	})
}

func TestADR0011C18RealOwnerBarrierNilAndDefaultOff(t *testing.T) {
	makeOwner := func(t *testing.T) (*privateB4RetainedResponseOwner, privateC15AtomicContextSnapshot) {
		t.Helper()
		account := mustPrivateB4ByteAccountV2(t)
		owner, failure := newPrivateB4RetainedResponseOwner(account, retainedResponseFixture()[:1])
		if failure != "" || owner == nil {
			t.Fatalf("BLOCKED_NOT_RED default-off owner: failure=%q owner=%v", failure, owner)
		}
		return owner, privateC15AtomicSnapshot(t, account)
	}
	plain, plainSnapshot := makeOwner(t)
	nilInstalled, nilSnapshot := makeOwner(t)
	var callbackCalls int
	restore := installPrivateC18Barrier(t, nilInstalled, nil)
	if callbackCalls != 0 {
		t.Fatalf("ASSERT_C18_NIL_INSTALLER_CALLBACK calls=%d", callbackCalls)
	}
	restore()
	if callbackCalls != 0 {
		t.Fatalf("ASSERT_C18_NIL_INSTALLER_RESTORE_CALLBACK calls=%d", callbackCalls)
	}
	if !reflect.DeepEqual(plainSnapshot, nilSnapshot) {
		t.Fatalf("ASSERT_C18_NIL_INSTALLER_ALLOCATION_CHANGED plain=%+v nil=%+v", plainSnapshot, nilSnapshot)
	}
	for label, owner := range map[string]*privateB4RetainedResponseOwner{"default-off": plain, "nil-installed-restored": nilInstalled} {
		lease := RetainedResponseLease{owner: owner}
		called := 0
		if err := lease.WithResponses(func(messages []lspwire.Message) error { called = len(messages); return nil }); err != nil || called != 1 {
			t.Fatalf("ASSERT_C18_REAL_OWNER_DEFAULT_OFF label=%s err=%v called=%d", label, err, called)
		}
		if !lease.Release() {
			t.Fatalf("ASSERT_C18_REAL_OWNER_DEFAULT_OFF_RELEASE label=%s", label)
		}
	}
}

func TestADR0011C18Pair1Through7PreservationControls(t *testing.T) {
	t.Run("pair1-stage-order", TestADR0011C18RuntimePathOrdersReachedStagesAndSealsBeforeHandoff)
	t.Run("pair2-earlier-refusal", TestADR0011C18RuntimePathEarlierRefusalSuppressesLaterEffects)
	t.Run("pair3-through6-coordinator-counterexamples", TestADR0011C18CoordinatorRejectsReviewerCounterexamples)
	t.Run("pair7-default-off", TestADR0011C18RuntimeObserverDefaultOffPreservesResultAndCustody)
}

func privateC18CallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

var privateC18BoundaryTags = map[string]bool{
	"privateC18ObjectEventCommit": true,
	"privateC18RetentionCommit":   true,
	"privateC18ReadbackBorrow":    true,
	"privateC18Release":           true,
}

func privateC18DirectCall(expr ast.Expr) *ast.CallExpr {
	call, _ := expr.(*ast.CallExpr)
	return call
}

func privateC18BarrierStatementCall(stmt ast.Stmt) *ast.CallExpr {
	expression, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call := privateC18DirectCall(expression.X)
	if call == nil || privateC18CallName(call) != "reachPrivateC18OwnerBarrier" {
		return nil
	}
	return call
}

func privateC18CheckedBarrierStatementCall(stmt ast.Stmt, function string) *ast.CallExpr {
	branch, ok := stmt.(*ast.IfStmt)
	if !ok || branch.Else != nil || len(branch.Body.List) != 1 {
		return nil
	}
	assignment, ok := branch.Init.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil
	}
	errName, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok || errName.Name != "err" {
		return nil
	}
	call := privateC18DirectCall(assignment.Rhs[0])
	if call == nil {
		return nil
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "reachPrivateC18OwnerBarrier" {
		return nil
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok || receiver.Name != "o" {
		return nil
	}
	condition, ok := branch.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ {
		return nil
	}
	conditionName, ok := condition.X.(*ast.Ident)
	conditionNil, nilOK := condition.Y.(*ast.Ident)
	if !ok || !nilOK || conditionName.Name != "err" || conditionNil.Name != "nil" {
		return nil
	}
	termination, ok := branch.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(termination.Results) != 1 {
		return nil
	}
	returned, ok := termination.Results[0].(*ast.Ident)
	if !ok {
		return nil
	}
	expected := map[string]string{
		"WithNotifications": "ErrRetainedNotificationLeaseClosed",
		"WithResponses":     "ErrRetainedResponseLeaseClosed",
	}[function]
	if expected == "" || returned.Name != expected {
		return nil
	}
	return call
}

func privateC18CheckedReleaseBarrierStatementCall(stmt ast.Stmt) *ast.CallExpr {
	branch, ok := stmt.(*ast.IfStmt)
	if !ok || branch.Else != nil || len(branch.Body.List) != 2 {
		return nil
	}
	assignment, ok := branch.Init.(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
		return nil
	}
	errName, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok || errName.Name != "err" {
		return nil
	}
	call := privateC18DirectCall(assignment.Rhs[0])
	if call == nil {
		return nil
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "reachPrivateC18OwnerBarrier" {
		return nil
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok || receiver.Name != "o" {
		return nil
	}
	condition, ok := branch.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ {
		return nil
	}
	conditionName, ok := condition.X.(*ast.Ident)
	conditionNil, nilOK := condition.Y.(*ast.Ident)
	if !ok || !nilOK || conditionName.Name != "err" || conditionNil.Name != "nil" {
		return nil
	}
	rollback, ok := branch.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil
	}
	rollbackCall := privateC18DirectCall(rollback.X)
	if rollbackCall == nil || len(rollbackCall.Args) != 0 {
		return nil
	}
	rollbackSelector, ok := rollbackCall.Fun.(*ast.SelectorExpr)
	if !ok || rollbackSelector.Sel.Name != "cancelPrivateC18Release" {
		return nil
	}
	rollbackReceiver, ok := rollbackSelector.X.(*ast.Ident)
	if !ok || rollbackReceiver.Name != "o" {
		return nil
	}
	termination, ok := branch.Body.List[1].(*ast.ReturnStmt)
	if !ok || len(termination.Results) != 1 {
		return nil
	}
	returned, ok := termination.Results[0].(*ast.Ident)
	if !ok || returned.Name != "false" {
		return nil
	}
	return call
}

func privateC18RequiresCheckedBarrier(function string) bool {
	return function == "WithNotifications" || function == "WithResponses" || function == "Release"
}

func privateC18DirectOwnerOperation(stmt ast.Stmt, operation string) bool {
	var expressions []ast.Expr
	switch statement := stmt.(type) {
	case *ast.ExprStmt:
		expressions = []ast.Expr{statement.X}
	case *ast.AssignStmt:
		expressions = statement.Rhs
	case *ast.ReturnStmt:
		expressions = statement.Results
	default:
		return false
	}
	for _, expression := range expressions {
		if call := privateC18DirectCall(expression); call != nil && privateC18CallName(call) == operation {
			return true
		}
	}
	return false
}

func privateC18BoundaryTagOccurrences(call *ast.CallExpr) [][2]any {
	var occurrences [][2]any
	for position, arg := range call.Args {
		ast.Inspect(arg, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && privateC18BoundaryTags[ident.Name] {
				occurrences = append(occurrences, [2]any{position, ident})
			}
			return true
		})
	}
	return occurrences
}

func privateC18ValidBarrierCall(call *ast.CallExpr, tag string) bool {
	if call == nil || privateC18CallName(call) != "reachPrivateC18OwnerBarrier" || len(call.Args) != 4 {
		return false
	}
	for position, name := range []string{"transactionID", "objectID", "sequence"} {
		ident, ok := call.Args[position].(*ast.Ident)
		if !ok || ident.Name != name {
			return false
		}
	}
	boundary, ok := call.Args[3].(*ast.Ident)
	if !ok || boundary.Name != tag {
		return false
	}
	occurrences := privateC18BoundaryTagOccurrences(call)
	if len(occurrences) != 1 || occurrences[0][0] != 3 {
		return false
	}
	ident := occurrences[0][1].(*ast.Ident)
	return ident == boundary
}

func privateC18MappedBarrierSourceError(source []byte, function, operation, tag string) error {
	file, err := parser.ParseFile(token.NewFileSet(), function+".go", source, 0)
	if err != nil {
		return fmt.Errorf("ASSERT_C18_AST_PARSE function=%s err=%w", function, err)
	}
	var target *ast.FuncDecl
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == function {
			target = fn
		}
	}
	if target == nil {
		return fmt.Errorf("ASSERT_C18_MAPPED_FUNCTION_MISSING function=%s", function)
	}
	barriers, valid := 0, 0
	ast.Inspect(target.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && privateC18CallName(call) == "reachPrivateC18OwnerBarrier" {
			barriers++
		}
		return true
	})
	ast.Inspect(target.Body, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, statement := range block.List {
			call := privateC18BarrierStatementCall(statement)
			if privateC18RequiresCheckedBarrier(function) {
				if function == "Release" {
					call = privateC18CheckedReleaseBarrierStatementCall(statement)
				} else {
					call = privateC18CheckedBarrierStatementCall(statement, function)
				}
			}
			if call != nil && privateC18ValidBarrierCall(call, tag) && i+1 < len(block.List) && privateC18DirectOwnerOperation(block.List[i+1], operation) {
				valid++
			}
		}
		return true
	})
	if barriers != 1 || valid != 1 {
		return fmt.Errorf("ASSERT_C18_EXACT_LIVE_PRECEDING_OWNER_BARRIER function=%s operation=%s tag=%s barriers=%d valid=%d", function, operation, tag, barriers, valid)
	}
	return nil
}

func privateC18AssertMappedBarrierSource(t *testing.T, source []byte, function, operation, tag string) {
	t.Helper()
	if err := privateC18MappedBarrierSourceError(source, function, operation, tag); err != nil {
		t.Fatal(err)
	}
}

func TestADR0011C18MappedBarrierASTOracle(t *testing.T) {
	const operation = "reservePrivateB4C15"
	checked := func(function, receiver, args, tag, condition, returned string) string {
		return fmt.Sprintf(`package synthetic
func %s() {
	if err := %s.reachPrivateC18OwnerBarrier(%s, %s); %s {
		return %s
	}
	result := reservePrivateB4C15()
	_ = result
}`, function, receiver, args, tag, condition, returned)
	}
	release := func(receiver, args, tag, condition, rollbackReceiver, returned string) string {
		return fmt.Sprintf(`package synthetic
func Release() {
	if err := %s.reachPrivateC18OwnerBarrier(%s, %s); %s {
		%s.cancelPrivateC18Release()
		return %s
	}
	o.charge.release()
}`, receiver, args, tag, condition, rollbackReceiver, returned)
	}
	validArgs := "transactionID, objectID, sequence"
	cases := []struct {
		name, function, source, tag string
		want                        bool
	}{
		{"correct-notifications", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", true},
		{"correct-responses", "WithResponses", checked("WithResponses", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrRetainedResponseLeaseClosed"), "privateC18ReadbackBorrow", true},
		{"authorized-constructor-marker", "mapped", `package synthetic
func mapped() {
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18RetentionCommit)
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18RetentionCommit", true},
		{"correct-release", "Release", release("o", validArgs, "privateC18Release", "err != nil", "o", "false"), "privateC18Release", true},
		{"release-bare-unhandled", "Release", `package synthetic
func Release() {
	o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release)
	o.charge.release()
}`, "privateC18Release", false},
		{"release-missing-rollback", "Release", `package synthetic
func Release() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release); err != nil { return false }
	o.charge.release()
}`, "privateC18Release", false},
		{"release-return-true", "Release", release("o", validArgs, "privateC18Release", "err != nil", "o", "true"), "privateC18Release", false},
		{"release-wrong-receiver", "Release", release("owner", validArgs, "privateC18Release", "err != nil", "o", "false"), "privateC18Release", false},
		{"release-wrong-tag", "Release", release("o", validArgs, "privateC18ReadbackBorrow", "err != nil", "o", "false"), "privateC18Release", false},
		{"release-wrong-condition", "Release", release("o", validArgs, "privateC18Release", "err == nil", "o", "false"), "privateC18Release", false},
		{"release-wrong-rollback-receiver", "Release", release("o", validArgs, "privateC18Release", "err != nil", "owner", "false"), "privateC18Release", false},
		{"release-duplicate", "Release", `package synthetic
func Release() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release); err != nil { o.cancelPrivateC18Release(); return false }
	o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release)
	o.charge.release()
}`, "privateC18Release", false},
		{"release-nonadjacent", "Release", `package synthetic
func Release() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18Release); err != nil { o.cancelPrivateC18Release(); return false }
	observe()
	o.charge.release()
}`, "privateC18Release", false},
		{"bare-with-rejected", "WithNotifications", `package synthetic
func WithNotifications() {
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow)
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18ReadbackBorrow", false},
		{"wrong-receiver", "WithNotifications", checked("WithNotifications", "owner", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"wrong-arguments", "WithNotifications", checked("WithNotifications", "o", "objectID, transactionID, sequence", "privateC18ReadbackBorrow", "err != nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"wrong-tag", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18Release", "err != nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"wrong-condition", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18ReadbackBorrow", "err == nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"notifications-raw-error", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "err"), "privateC18ReadbackBorrow", false},
		{"notifications-wrong-typed-error", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrRetainedResponseLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"notifications-arbitrary-error", "WithNotifications", checked("WithNotifications", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrArbitrary"), "privateC18ReadbackBorrow", false},
		{"notifications-no-termination", "WithNotifications", `package synthetic
func WithNotifications() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil { observe() }
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18ReadbackBorrow", false},
		{"responses-raw-error", "WithResponses", checked("WithResponses", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "err"), "privateC18ReadbackBorrow", false},
		{"responses-wrong-typed-error", "WithResponses", checked("WithResponses", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrRetainedNotificationLeaseClosed"), "privateC18ReadbackBorrow", false},
		{"responses-arbitrary-error", "WithResponses", checked("WithResponses", "o", validArgs, "privateC18ReadbackBorrow", "err != nil", "ErrArbitrary"), "privateC18ReadbackBorrow", false},
		{"responses-no-termination", "WithResponses", `package synthetic
func WithResponses() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil { observe() }
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18ReadbackBorrow", false},
		{"intervening-statement", "WithNotifications", `package synthetic
func WithNotifications() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil { return ErrRetainedNotificationLeaseClosed }
	observe()
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18ReadbackBorrow", false},
		{"duplicate", "WithNotifications", `package synthetic
func WithNotifications() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil { return ErrRetainedNotificationLeaseClosed }
	o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow)
	result := reservePrivateB4C15()
	_ = result
}`, "privateC18ReadbackBorrow", false},
		{"nested-operation", "WithNotifications", `package synthetic
func WithNotifications() {
	if err := o.reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18ReadbackBorrow); err != nil { return ErrRetainedNotificationLeaseClosed }
	consume(reservePrivateB4C15())
}`, "privateC18ReadbackBorrow", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			testOperation := operation
			if test.function == "Release" {
				testOperation = "release"
			}
			err := privateC18MappedBarrierSourceError([]byte(test.source), test.function, testOperation, test.tag)
			if got := err == nil; got != test.want {
				t.Fatalf("ASSERT_C18_AST_ORACLE got=%t want=%t err=%v", got, test.want, err)
			}
		})
	}
}

func privateC18BarrierOutsideOwnerLockError(source []byte, function string) error {
	file, err := parser.ParseFile(token.NewFileSet(), function+".go", source, 0)
	if err != nil {
		return fmt.Errorf("ASSERT_C18_LOCK_AST_PARSE function=%s err=%w", function, err)
	}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function || fn.Body == nil {
			continue
		}
		aliases := map[string]string{}
		held := map[string]int{}
		lockKey := func(expr ast.Expr) string {
			switch value := expr.(type) {
			case *ast.Ident:
				if alias := aliases[value.Name]; alias != "" {
					return alias
				}
				return value.Name
			case *ast.SelectorExpr:
				if base, ok := value.X.(*ast.Ident); ok {
					return base.Name + "." + value.Sel.Name
				}
			}
			return ""
		}
		var walkBlock func(*ast.BlockStmt) error
		walkBlock = func(block *ast.BlockStmt) error {
			for _, statement := range block.List {
				if assignment, ok := statement.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
					if name, ok := assignment.Lhs[0].(*ast.Ident); ok {
						if key := lockKey(assignment.Rhs[0]); key != "" {
							aliases[name.Name] = key
						}
					}
				}
				var call *ast.CallExpr
				deferred := false
				switch value := statement.(type) {
				case *ast.ExprStmt:
					call = privateC18DirectCall(value.X)
				case *ast.DeferStmt:
					call, deferred = value.Call, true
				}
				if privateC18BarrierStatementCall(statement) != nil {
					for key, depth := range held {
						if depth > 0 {
							return fmt.Errorf("ASSERT_C18_BARRIER_CALLBACK_UNDER_KNOWN_LOCK function=%s lock=%s depth=%d", function, key, depth)
						}
					}
				}
				if call != nil {
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
						key := lockKey(selector.X)
						switch selector.Sel.Name {
						case "Lock", "RLock":
							held[key]++
						case "Unlock", "RUnlock":
							if !deferred {
								held[key]--
								if held[key] < 0 {
									return fmt.Errorf("ASSERT_C18_OWNER_LOCK_UNBALANCED function=%s lock=%s", function, key)
								}
							}
						}
					}
				}
				switch value := statement.(type) {
				case *ast.BlockStmt:
					if err := walkBlock(value); err != nil {
						return err
					}
				case *ast.IfStmt:
					if err := walkBlock(value.Body); err != nil {
						return err
					}
					if other, ok := value.Else.(*ast.BlockStmt); ok {
						if err := walkBlock(other); err != nil {
							return err
						}
					}
				case *ast.ForStmt:
					if err := walkBlock(value.Body); err != nil {
						return err
					}
				case *ast.RangeStmt:
					if err := walkBlock(value.Body); err != nil {
						return err
					}
				}
			}
			return nil
		}
		return walkBlock(fn.Body)
	}
	return fmt.Errorf("ASSERT_C18_LOCK_FUNCTION_MISSING function=%s", function)
}

func privateC18DeclaredTypes(t *testing.T, source []byte, filename string) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		t.Fatalf("ASSERT_C18_DECLARATION_AST_PARSE path=%s err=%v", filename, err)
	}
	result := map[string]bool{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			if named, ok := spec.(*ast.TypeSpec); ok {
				result[named.Name.Name] = true
			}
		}
	}
	return result
}

func privateC18ProductionIdentifierContractError(file *ast.File) error {
	var identifierError error
	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if strings.Contains(strings.ToLower(identifier.Name), "_test") {
			identifierError = fmt.Errorf("ASSERT_C18_PRODUCTION_IDENTIFIER_TEST identifier=%s", identifier.Name)
			return false
		}
		return true
	})
	return identifierError
}

func privateC18OwnerBarrierContractError(source []byte, testTypes, allowedTypes map[string]bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), "c18_owner_barrier_private.go", source, 0)
	if err != nil {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_AST_PARSE err=%w", err)
	}
	if err := privateC18ProductionIdentifierContractError(file); err != nil {
		return err
	}
	declarations := map[string]*ast.TypeSpec{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			named, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(named.Name.Name), "_test") {
				return fmt.Errorf("ASSERT_C18_PRODUCTION_IDENTIFIER_TEST declaration=%s", named.Name.Name)
			}
			declarations[named.Name.Name] = named
		}
	}
	boundary := declarations["privateC18OwnerBoundary"]
	boundaryType, boundaryTypeOK := func() (*ast.Ident, bool) {
		if boundary == nil || boundary.Assign.IsValid() {
			return nil, false
		}
		identifier, ok := boundary.Type.(*ast.Ident)
		return identifier, ok
	}()
	if !boundaryTypeOK || boundaryType.Name != "uint8" || testTypes[boundary.Name.Name] {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_BOUNDARY_TYPE")
	}
	event := declarations["privateC18OwnerBarrierEvent"]
	if event == nil {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS")
	}
	structure, ok := event.Type.(*ast.StructType)
	wantFields := []struct{ name, typeName string }{{"TransactionID", "string"}, {"ObjectID", "string"}, {"Sequence", "uint64"}, {"Boundary", "privateC18OwnerBoundary"}}
	if testTypes[event.Name.Name] || !ok || structure.Fields == nil || len(structure.Fields.List) != len(wantFields) {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS")
	}
	for index, want := range wantFields {
		field := structure.Fields.List[index]
		typeName, typeOK := field.Type.(*ast.Ident)
		if len(field.Names) != 1 || field.Names[0].Name != want.name || !typeOK || typeName.Name != want.typeName || testTypes[typeName.Name] {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_EVENT_FIELD index=%d want=%s:%s", index, want.name, want.typeName)
		}
	}
	callback := declarations["privateC18OwnerBarrier"]
	if callback == nil {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_CALLBACK_SIGNATURE")
	}
	function, ok := callback.Type.(*ast.FuncType)
	if testTypes[callback.Name.Name] || !ok || function.Params == nil || len(function.Params.List) != 1 || len(function.Params.List[0].Names) != 0 || function.Results == nil || len(function.Results.List) != 1 || len(function.Results.List[0].Names) != 0 {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_CALLBACK_SIGNATURE")
	}
	input, inputOK := function.Params.List[0].Type.(*ast.Ident)
	output, outputOK := function.Results.List[0].Type.(*ast.Ident)
	if !inputOK || input.Name != "privateC18OwnerBarrierEvent" || !outputOK || output.Name != "error" {
		return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_CALLBACK_SIGNATURE")
	}
	required := map[string]bool{"privateB4RetainedNotificationOwner": false, "privateB4RetainedResponseOwner": false}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "InstallPrivateC18OwnerBarrierForTest" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		pointer, pointerOK := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !pointerOK {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RECEIVER")
		}
		receiverID, receiverOK := pointer.X.(*ast.Ident)
		if !receiverOK {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RECEIVER")
		}
		if _, wanted := required[receiverID.Name]; !wanted {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RECEIVER receiver=%s", receiverID.Name)
		}
		if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_INPUT_ARITY receiver=%s", receiverID.Name)
		}
		callbackID, callbackOK := fn.Type.Params.List[0].Type.(*ast.Ident)
		if !callbackOK || callbackID.Name != "privateC18OwnerBarrier" {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_CALLBACK receiver=%s", receiverID.Name)
		}
		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_OUTPUT_ARITY receiver=%s", receiverID.Name)
		}
		restore, restoreOK := fn.Type.Results.List[0].Type.(*ast.FuncType)
		if !restoreOK || (restore.Params != nil && len(restore.Params.List) != 0) || (restore.Results != nil && len(restore.Results.List) != 0) {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RESTORE_SIGNATURE receiver=%s", receiverID.Name)
		}
		required[receiverID.Name] = true
	}
	for receiver, present := range required {
		if !present {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_MISSING receiver=%s", receiver)
		}
	}
	_ = allowedTypes
	return nil
}

func privateC18ExactNamedStructError(named *ast.TypeSpec, wantName string, fields []struct{ name, typeName string }) error {
	structure, ok := named.Type.(*ast.StructType)
	if named.Name.Name != wantName || !ok || structure.Fields == nil || len(structure.Fields.List) != len(fields) {
		return fmt.Errorf("ASSERT_C18_PRODUCTION_EVENT_DECLARATION type=%s", wantName)
	}
	for index, want := range fields {
		field := structure.Fields.List[index]
		identifier, typeOK := field.Type.(*ast.Ident)
		if len(field.Names) != 1 || field.Names[0].Name != want.name || !typeOK || identifier.Name != want.typeName {
			return fmt.Errorf("ASSERT_C18_PRODUCTION_EVENT_FIELD type=%s index=%d", wantName, index)
		}
	}
	return nil
}

func privateC18SplitPhaseDeclarationError(named *ast.TypeSpec) error {
	structure, ok := named.Type.(*ast.StructType)
	want := []struct {
		name      string
		direction ast.ChanDir
		any       bool
	}{{"Reached", ast.SEND, true}, {"Decision", ast.SEND, true}, {"DecisionRelease", ast.RECV, false}, {"EffectRelease", ast.RECV, false}}
	if !ok || structure.Fields == nil || len(structure.Fields.List) != len(want) {
		return fmt.Errorf("ASSERT_C18_SPLIT_PHASE_SEAM_DECLARATION")
	}
	for index, expected := range want {
		field := structure.Fields.List[index]
		channel, channelOK := field.Type.(*ast.ChanType)
		if len(field.Names) != 1 || field.Names[0].Name != expected.name || !channelOK || channel.Dir != expected.direction {
			return fmt.Errorf("ASSERT_C18_SPLIT_PHASE_SEAM_FIELD index=%d", index)
		}
		if expected.any {
			identifier, identifierOK := channel.Value.(*ast.Ident)
			if !identifierOK || identifier.Name != "any" {
				return fmt.Errorf("ASSERT_C18_SPLIT_PHASE_SEAM_NEUTRAL_CHANNEL field=%s", expected.name)
			}
		} else if structureType, structureOK := channel.Value.(*ast.StructType); !structureOK || structureType.Fields == nil || len(structureType.Fields.List) != 0 {
			return fmt.Errorf("ASSERT_C18_SPLIT_PHASE_SEAM_RELEASE_CHANNEL field=%s", expected.name)
		}
	}
	return nil
}

func privateC18SyntheticCompanionEffectsError(file *ast.File) error {
	forbiddenDirectCalls := map[string]map[string]bool{
		"commitPrivateC18ObjectEventForPair8Test": {"commitPrivateC18RetentionForPair8Test": true, "reservePrivateB4C15": true},
		"commitPrivateC18RetentionForPair9Test":   {"borrowPrivateC18ReadbackForPair9Test": true, "aliasPrivateB4C15": true},
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := function.Name.Name
		if strings.HasPrefix(name, "commitPrivateC18") && strings.HasSuffix(name, "Companion") {
			return fmt.Errorf("ASSERT_C18_SYNTHETIC_COMPANION_DECLARATION function=%s", name)
		}
		var callErr error
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			called := privateC18OperationCallName(call)
			if strings.HasPrefix(called, "commitPrivateC18") && strings.HasSuffix(called, "Companion") {
				callErr = fmt.Errorf("ASSERT_C18_SYNTHETIC_COMPANION_CALL function=%s call=%s", name, called)
				return false
			}
			if forbiddenDirectCalls[name][called] {
				callErr = fmt.Errorf("ASSERT_C18_WINNER_DIRECT_OPPOSING_TRANSITION function=%s call=%s", name, called)
				return false
			}
			return true
		})
		if callErr != nil {
			return callErr
		}
	}
	return nil
}

func privateC18ProductionCoordinatorContractError(source []byte, testTypes, allowedTypes map[string]bool) error {
	if err := privateC18OwnerBarrierContractError(source, testTypes, allowedTypes); err != nil {
		return err
	}
	file, err := parser.ParseFile(token.NewFileSet(), "c18_pair_coordinator_private.go", source, 0)
	if err != nil {
		return fmt.Errorf("ASSERT_C18_COORDINATOR_AST_PARSE err=%w", err)
	}
	if err := privateC18SyntheticCompanionEffectsError(file); err != nil {
		return err
	}
	if err := privateC18ProductionIdentifierContractError(file); err != nil {
		return err
	}
	requiredTypes := map[string]bool{
		"privateC18OwnerBoundary": false, "privateC18OwnerBarrierEvent": false, "privateC18OwnerBarrier": false,
		"privateC18Authority": false, "privateC18ReachedEvent": false, "privateC18DecisionEvent": false,
		"privateC18SplitPhaseSeam": false, "privateC18CoordinatorSnapshot": false,
	}
	requiredFunctions := map[string][]string{
		"commitPrivateC18ObjectEventForPair8Test":           {"privateC18ObjectEventCommit", "privateC18RetentionCommit"},
		"commitPrivateC18RetentionForPair8Test":             {"privateC18ObjectEventCommit", "privateC18RetentionCommit"},
		"commitPrivateC18RetentionForPair9Test":             {"privateC18RetentionCommit", "privateC18ReadbackBorrow"},
		"borrowPrivateC18ReadbackForPair9Test":              {"privateC18RetentionCommit", "privateC18ReadbackBorrow"},
		"runPrivateC18Pair8PostDecisionCancellationForTest": {"privateC18ObjectEventCommit", "privateC18RetentionCommit"},
		"runPrivateC18Pair9PostDecisionCancellationForTest": {"privateC18RetentionCommit", "privateC18ReadbackBorrow"},
	}
	type methodContract struct {
		receiver string
		in, out  int
	}
	requiredInstallers := map[string]bool{
		"privateB4RetainedNotificationOwner": false,
		"privateB4RetainedResponseOwner":     false,
	}
	requiredMethods := map[string]methodContract{
		"CommitPrivateC18ObjectEventForPair8Test":            {"privateB4RetainedNotificationOwner", 2, 2},
		"CommitPrivateC18RetentionForPair8Test":              {"privateB4RetainedNotificationOwner", 1, 2},
		"RetryPrivateC18Pair8LoserForTest":                   {"privateB4RetainedNotificationOwner", 2, 1},
		"PrivateC18Pair8CoordinatorSnapshotForTest":          {"privateB4RetainedNotificationOwner", 0, 1},
		"InstallPrivateC18Pair8SplitPhaseSeamForTest":        {"privateB4RetainedNotificationOwner", 1, 2},
		"CommitPrivateC18Pair8ContenderWithAuthorityForTest": {"privateB4RetainedNotificationOwner", 3, 2},
		"StopPrivateC18Pair8CoordinatorForTest":              {"privateB4RetainedNotificationOwner", 1, 1},
		"AwaitPrivateC18Pair8StoppedForTest":                 {"privateB4RetainedNotificationOwner", 1, 1},
		"RestartPrivateC18Pair8CoordinatorForTest":           {"privateB4RetainedNotificationOwner", 1, 1},
		"ProbePrivateC18Pair8LockSafetyForTest":              {"privateB4RetainedNotificationOwner", 2, 1},
		"CommitPrivateC18RetentionForPair9Test":              {"privateB4RetainedResponseOwner", 1, 2},
		"BorrowPrivateC18ReadbackForPair9Test":               {"privateB4RetainedResponseOwner", 2, 2},
		"RetryPrivateC18Pair9LoserForTest":                   {"privateB4RetainedResponseOwner", 2, 1},
		"PrivateC18Pair9CoordinatorSnapshotForTest":          {"privateB4RetainedResponseOwner", 0, 1},
		"InstallPrivateC18Pair9SplitPhaseSeamForTest":        {"privateB4RetainedResponseOwner", 1, 2},
		"CommitPrivateC18Pair9ContenderWithAuthorityForTest": {"privateB4RetainedResponseOwner", 4, 2},
		"StopPrivateC18Pair9CoordinatorForTest":              {"privateB4RetainedResponseOwner", 1, 1},
		"AwaitPrivateC18Pair9StoppedForTest":                 {"privateB4RetainedResponseOwner", 1, 1},
		"RestartPrivateC18Pair9CoordinatorForTest":           {"privateB4RetainedResponseOwner", 1, 1},
		"ProbePrivateC18Pair9LockSafetyForTest":              {"privateB4RetainedResponseOwner", 2, 1},
	}
	validFunctions, validMethods := map[string]bool{}, map[string]bool{}
	productionNames := map[string]bool{}
	for name := range allowedTypes {
		productionNames[name] = true
	}
	for name := range requiredTypes {
		productionNames[name] = true
	}
	for _, name := range []string{"bool", "byte", "error", "int", "string", "uint8", "uint64", "any"} {
		productionNames[name] = true
	}
	checkType := func(owner string, expression ast.Expr) error {
		selectors, fieldNames := map[*ast.Ident]bool{}, map[*ast.Ident]bool{}
		ast.Inspect(expression, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.SelectorExpr:
				selectors[value.Sel] = true
			case *ast.Field:
				for _, name := range value.Names {
					fieldNames[name] = true
				}
			}
			return true
		})
		var typeErr error
		ast.Inspect(expression, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if !ok || selectors[identifier] || fieldNames[identifier] {
				return true
			}
			if strings.Contains(strings.ToLower(identifier.Name), "_test") {
				typeErr = fmt.Errorf("ASSERT_C18_PRODUCTION_IDENTIFIER_TEST declaration=%s identifier=%s", owner, identifier.Name)
				return false
			}
			if testTypes[identifier.Name] {
				typeErr = fmt.Errorf("ASSERT_C18_PRODUCTION_SIGNATURE_TEST_IDENTIFIER declaration=%s identifier=%s", owner, identifier.Name)
				return false
			}
			if !productionNames[identifier.Name] {
				typeErr = fmt.Errorf("ASSERT_C18_PRODUCTION_SIGNATURE_UNKNOWN_IDENTIFIER declaration=%s identifier=%s", owner, identifier.Name)
				return false
			}
			return true
		})
		return typeErr
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, spec := range general.Specs {
			if named, ok := spec.(*ast.TypeSpec); ok {
				productionNames[named.Name.Name] = true
			}
		}
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok != token.TYPE {
				continue
			}
			for _, spec := range declaration.Specs {
				named, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				productionNames[named.Name.Name] = true
				if _, required := requiredTypes[named.Name.Name]; required {
					requiredTypes[named.Name.Name] = true
				}
				switch named.Name.Name {
				case "privateC18ReachedEvent":
					if err := privateC18ExactNamedStructError(named, named.Name.Name, []struct{ name, typeName string }{{"Contender", "privateC18Contender"}, {"Boundary", "privateC18OwnerBoundary"}}); err != nil {
						return err
					}
				case "privateC18DecisionEvent":
					if err := privateC18ExactNamedStructError(named, named.Name.Name, []struct{ name, typeName string }{{"Winner", "privateC18Contender"}, {"Loser", "privateC18Contender"}, {"Authority", "privateC18Authority"}}); err != nil {
						return err
					}
				case "privateC18Authority":
					if err := privateC18ExactNamedStructError(named, named.Name.Name, []struct{ name, typeName string }{{"Generation", "uint64"}, {"TransactionID", "string"}, {"ObjectID", "string"}}); err != nil {
						return err
					}
				case "privateC18SplitPhaseSeam":
					if err := privateC18SplitPhaseDeclarationError(named); err != nil {
						return err
					}
				}
				if err := checkType(named.Name.Name, named.Type); err != nil {
					return err
				}
			}
		case *ast.FuncDecl:
			if declaration.Type != nil {
				if err := checkType(declaration.Name.Name, declaration.Type); err != nil {
					return err
				}
			}
			var receiverName string
			receiverPointer := false
			if declaration.Recv != nil && len(declaration.Recv.List) == 1 {
				receiver := declaration.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiverPointer = true
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					receiverName = identifier.Name
				}
			}
			if declaration.Name.Name == "InstallPrivateC18OwnerBarrierForTest" {
				if _, required := requiredInstallers[receiverName]; !required || !receiverPointer {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RECEIVER receiver=%s", receiverName)
				}
				if declaration.Type.Params == nil || len(declaration.Type.Params.List) != 1 || len(declaration.Type.Params.List[0].Names) > 1 {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_INPUT_ARITY receiver=%s", receiverName)
				}
				callback, ok := declaration.Type.Params.List[0].Type.(*ast.Ident)
				if !ok || callback.Name != "privateC18OwnerBarrier" || testTypes[callback.Name] || len(declaration.Type.Params.List[0].Names) != 1 {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_CALLBACK receiver=%s", receiverName)
				}
				callbackName := declaration.Type.Params.List[0].Names[0].Name
				if declaration.Type.Results == nil || len(declaration.Type.Results.List) != 1 || len(declaration.Type.Results.List[0].Names) > 1 {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_OUTPUT_ARITY receiver=%s", receiverName)
				}
				restore, ok := declaration.Type.Results.List[0].Type.(*ast.FuncType)
				if !ok || (restore.Params != nil && len(restore.Params.List) != 0) || (restore.Results != nil && len(restore.Results.List) != 0) {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RESTORE_SIGNATURE receiver=%s", receiverName)
				}
				var previousName, barrierField string
				installed, restored := false, false
				if declaration.Body != nil {
					for _, statement := range declaration.Body.List {
						switch statement := statement.(type) {
						case *ast.AssignStmt:
							if len(statement.Lhs) != 1 || len(statement.Rhs) != 1 {
								continue
							}
							if previous, ok := statement.Lhs[0].(*ast.Ident); ok {
								if selector, ok := statement.Rhs[0].(*ast.SelectorExpr); ok {
									previousName, barrierField = previous.Name, selector.Sel.Name
								}
							}
							if selector, ok := statement.Lhs[0].(*ast.SelectorExpr); ok {
								if value, ok := statement.Rhs[0].(*ast.Ident); ok && value.Name == callbackName && selector.Sel.Name == barrierField {
									installed = true
								}
							}
						case *ast.ReturnStmt:
							if len(statement.Results) != 1 {
								continue
							}
							literal, ok := statement.Results[0].(*ast.FuncLit)
							if !ok {
								continue
							}
							ast.Inspect(literal.Body, func(node ast.Node) bool {
								assignment, ok := node.(*ast.AssignStmt)
								if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
									return true
								}
								selector, selectorOK := assignment.Lhs[0].(*ast.SelectorExpr)
								previous, previousOK := assignment.Rhs[0].(*ast.Ident)
								if selectorOK && previousOK && selector.Sel.Name == barrierField && previous.Name == previousName {
									restored = true
								}
								return !restored
							})
						}
					}
				}
				if previousName == "" || barrierField == "" || !installed || !restored {
					return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_RESTORE_BODY receiver=%s", receiverName)
				}
				requiredInstallers[receiverName] = true
			}
			if contract, required := requiredMethods[declaration.Name.Name]; required && receiverName != "" {
				params, results := 0, 0
				if declaration.Type.Params != nil {
					for _, field := range declaration.Type.Params.List {
						if len(field.Names) == 0 {
							params++
						} else {
							params += len(field.Names)
						}
					}
				}
				if declaration.Type.Results != nil {
					for _, field := range declaration.Type.Results.List {
						if len(field.Names) == 0 {
							results++
						} else {
							results += len(field.Names)
						}
					}
				}
				receiverOK := receiverName == contract.receiver
				validMethods[declaration.Name.Name] = receiverOK && params == contract.in && results == contract.out
			}
			boundaries, required := requiredFunctions[declaration.Name.Name]
			if !required || declaration.Body == nil {
				continue
			}
			hasCoordinatorCall := false
			seen := map[string]bool{}
			ast.Inspect(declaration.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || privateC18CallName(call) != "coordinatePrivateC18Pair" {
					return true
				}
				hasCoordinatorCall = true
				ast.Inspect(call, func(inner ast.Node) bool {
					if id, ok := inner.(*ast.Ident); ok {
						seen[id.Name] = true
					}
					return true
				})
				return true
			})
			validFunctions[declaration.Name.Name] = hasCoordinatorCall && seen[boundaries[0]] && seen[boundaries[1]]
		}
	}
	for name, present := range requiredTypes {
		if !present {
			return fmt.Errorf("ASSERT_C18_PRODUCTION_OWNED_TYPE_MISSING type=%s", name)
		}
	}
	for name := range requiredFunctions {
		if !validFunctions[name] {
			return fmt.Errorf("ASSERT_C18_COORDINATOR_CALL_SHAPE_MISSING function=%s callee=coordinatePrivateC18Pair", name)
		}
	}
	for receiver, present := range requiredInstallers {
		if !present {
			return fmt.Errorf("ASSERT_C18_OWNER_BARRIER_INSTALLER_MISSING method=InstallPrivateC18OwnerBarrierForTest receiver=%s", receiver)
		}
	}
	for name, contract := range requiredMethods {
		if !validMethods[name] {
			return fmt.Errorf("ASSERT_C18_PRODUCTION_METHOD_CONTRACT method=%s receiver=%s in=%d out=%d", name, contract.receiver, contract.in, contract.out)
		}
	}
	return nil
}

type privateC18WrongEnumName uint8
type privateC18WrongEnumKind string
type privateC18WrongEnumWidth uint16

func TestADR0011C18NamedPrimitiveNegativeControls(t *testing.T) {
	pkgPath := reflect.TypeOf(privateC18FixtureOwnerBoundary(0)).PkgPath()
	for _, test := range []struct {
		name        string
		typeOf      reflect.Type
		wantName    string
		wantPackage string
		wantKind    reflect.Kind
		wantBits    int
	}{
		{name: "wrong-production-type-name", typeOf: reflect.TypeOf(privateC18WrongEnumName(0)), wantName: "privateC18OwnerBoundary", wantPackage: pkgPath, wantKind: reflect.Uint8, wantBits: 8},
		{name: "wrong-production-package", typeOf: reflect.TypeOf(time.Duration(0)), wantName: "Duration", wantPackage: pkgPath, wantKind: reflect.Int64, wantBits: 64},
		{name: "wrong-production-kind", typeOf: reflect.TypeOf(privateC18WrongEnumKind("")), wantName: "privateC18WrongEnumKind", wantPackage: pkgPath, wantKind: reflect.Uint8, wantBits: 8},
		{name: "wrong-production-width", typeOf: reflect.TypeOf(privateC18WrongEnumWidth(0)), wantName: "privateC18WrongEnumWidth", wantPackage: pkgPath, wantKind: reflect.Uint16, wantBits: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := privateC18NamedPrimitiveTypeError(test.typeOf, test.wantName, test.wantPackage, test.wantKind, test.wantBits); err == nil {
				t.Fatal("ASSERT_C18_NAMED_PRIMITIVE_NEGATIVE_CONTROL_ACCEPTED")
			}
		})
	}
	if err := privateC18BoundaryRangeError(0); err == nil {
		t.Fatal("ASSERT_C18_NAMED_PRIMITIVE_NEGATIVE_RANGE_ZERO_ACCEPTED")
	}
	if err := privateC18BoundaryRangeError(uint64(privateC18FixtureRelease) + 1); err == nil {
		t.Fatal("ASSERT_C18_NAMED_PRIMITIVE_NEGATIVE_RANGE_OVERFLOW_ACCEPTED")
	}
	t.Run("empty-snapshot-winner-loser-accepted", func(t *testing.T) {
		if err := privateC18ContenderRangeError("", true); err != nil {
			t.Fatalf("ASSERT_C18_EMPTY_SNAPSHOT_CONTENDER_REJECTED err=%v", err)
		}
	})
	t.Run("empty-operation-contender-rejected", func(t *testing.T) {
		if err := privateC18ContenderRangeError("", false); err == nil {
			t.Fatal("ASSERT_C18_EMPTY_OPERATION_CONTENDER_ACCEPTED")
		}
	})
	t.Run("unknown-nonempty-snapshot-contender-rejected", func(t *testing.T) {
		if err := privateC18ContenderRangeError("unknown", true); err == nil {
			t.Fatal("ASSERT_C18_UNKNOWN_SNAPSHOT_CONTENDER_ACCEPTED")
		}
	})
}

func TestADR0011C18OwnerBarrierInstallerSignatureOracle(t *testing.T) {
	fixture := func(declarations string) string {
		return `package sessionruntime
	type privateC18OwnerBoundary uint8
	type privateB4RetainedNotificationOwner struct{ barrier privateC18OwnerBarrier }
	type privateB4RetainedResponseOwner struct{ barrier privateC18OwnerBarrier }
	` + declarations + `
	func (o *privateB4RetainedNotificationOwner) InstallPrivateC18OwnerBarrierForTest(callback privateC18OwnerBarrier) func() { previous := o.barrier; o.barrier = callback; return func() { o.barrier = previous } }
	func (o *privateB4RetainedResponseOwner) InstallPrivateC18OwnerBarrierForTest(callback privateC18OwnerBarrier) func() { previous := o.barrier; o.barrier = callback; return func() { o.barrier = previous } }`
	}
	validEvent := `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary }`
	cases := []struct{ name, declarations, reason string }{
		{"wrong-named-callback-signature", validEvent + `; type privateC18OwnerBarrier func(int) error`, "ASSERT_C18_OWNER_BARRIER_CALLBACK_SIGNATURE"},
		{"wrong-event-field-name", `type privateC18OwnerBarrierEvent struct { Transaction string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"lowercase-transaction-id-event-field", `type privateC18OwnerBarrierEvent struct { transactionID string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"lowercase-object-id-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; objectID string; Sequence uint64; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"lowercase-sequence-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; sequence uint64; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"lowercase-boundary-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64; boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"wrong-event-field-order", `type privateC18OwnerBarrierEvent struct { ObjectID string; TransactionID string; Sequence uint64; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"wrong-event-field-type", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence int; Boundary privateC18OwnerBoundary }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELD"},
		{"extra-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary; Extra bool }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS"},
		{"extra-duplicate-exported-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary; TransactionID string }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS"},
		{"extra-duplicate-unexported-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64; Boundary privateC18OwnerBoundary; transactionID string }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS"},
		{"missing-event-field", `type privateC18OwnerBarrierEvent struct { TransactionID string; ObjectID string; Sequence uint64 }; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_OWNER_BARRIER_EVENT_FIELDS"},
		{"test-identifier", validEvent + `; type helper_TEST string; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`, "ASSERT_C18_PRODUCTION_IDENTIFIER_TEST"},
		{"non-type-var-const-test-identifier", validEvent + `; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error; var state_test = 1; const limit_TEST = 2`, "ASSERT_C18_PRODUCTION_IDENTIFIER_TEST"},
		{"function-signature-test-identifiers", validEvent + `; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error; func (receiver_test *privateB4RetainedNotificationOwner) helper(parameter_TEST int) {}`, "ASSERT_C18_PRODUCTION_IDENTIFIER_TEST"},
	}
	testTypes := map[string]bool{"privateC18FixtureOwnerBarrier": true}
	allowedTypes := map[string]bool{"privateB4RetainedNotificationOwner": true, "privateB4RetainedResponseOwner": true}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := privateC18OwnerBarrierContractError([]byte(fixture(test.declarations)), testTypes, allowedTypes)
			if err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("ASSERT_C18_OWNER_BARRIER_NEGATIVE_ORACLE reason=%s err=%v", test.reason, err)
			}
		})
	}
	validSource := fixture(validEvent + `; type privateC18OwnerBarrier func(privateC18OwnerBarrierEvent) error`)
	for _, test := range []struct{ name, source, reason string }{
		{
			"value-receiver",
			strings.Replace(validSource, "func (o *privateB4RetainedNotificationOwner) InstallPrivateC18OwnerBarrierForTest", "func (o privateB4RetainedNotificationOwner) InstallPrivateC18OwnerBarrierForTest", 1),
			"ASSERT_C18_OWNER_BARRIER_INSTALLER_RECEIVER",
		},
		{
			"wrong-boundary-underlying-type",
			strings.Replace(validSource, "type privateC18OwnerBoundary uint8", "type privateC18OwnerBoundary uint16", 1),
			"ASSERT_C18_OWNER_BARRIER_BOUNDARY_TYPE",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := privateC18OwnerBarrierContractError([]byte(test.source), testTypes, allowedTypes)
			if err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("ASSERT_C18_OWNER_BARRIER_NEGATIVE_ORACLE reason=%s err=%v", test.reason, err)
			}
		})
	}
}

func TestADR0011C18SyntheticCompanionEffectsStaticGuard(t *testing.T) {
	for _, test := range []struct {
		name, source, reason string
	}{
		{"companion-declaration", `package p; func commitPrivateC18Pair8Companion() {}`, "ASSERT_C18_SYNTHETIC_COMPANION_DECLARATION"},
		{"companion-call", `package p; func helper() { commitPrivateC18Pair9Companion() }`, "ASSERT_C18_SYNTHETIC_COMPANION_CALL"},
		{"pair8-direct-retention", `package p; func commitPrivateC18ObjectEventForPair8Test() { reservePrivateB4C15() }`, "ASSERT_C18_WINNER_DIRECT_OPPOSING_TRANSITION"},
		{"pair9-direct-readback", `package p; func commitPrivateC18RetentionForPair9Test() { aliasPrivateB4C15() }`, "ASSERT_C18_WINNER_DIRECT_OPPOSING_TRANSITION"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), test.name+".go", test.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := privateC18SyntheticCompanionEffectsError(file); err == nil || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("ASSERT_C18_STATIC_NO_SYNTHETIC_NEGATIVE_ORACLE reason=%s err=%v", test.reason, err)
			}
		})
	}
}

func TestADR0011C18ProductionOwnsCoordinatorContract(t *testing.T) {
	source, err := os.ReadFile("c18_pair_coordinator_private.go")
	if err != nil {
		t.Fatalf("ASSERT_C18_COORDINATOR_SOURCE_ABSENT path=c18_pair_coordinator_private.go err=%v", err)
	}
	testSource, err := os.ReadFile("adr0011_c18_real_owner_collision_red_private_test.go")
	if err != nil {
		t.Fatal(err)
	}
	testTypes := privateC18DeclaredTypes(t, testSource, "adr0011_c18_real_owner_collision_red_private_test.go")
	allowedTypes := map[string]bool{"context": true, "session": true, "lspwire": true, "Context": true, "Failure": true, "Message": true, "RetainedNotificationLease": true, "RetainedResponseLease": true, "privateB4EventIdentityC17": true, "privateB4RetainedNotificationOwner": true, "privateB4RetainedResponseOwner": true}
	if err := privateC18ProductionCoordinatorContractError(source, testTypes, allowedTypes); err != nil {
		t.Fatal(err)
	}
}

func TestADR0011C18ReleaseCallsUseCheckedMappedOwnerBoundary(t *testing.T) {
	for _, check := range []struct{ path string }{
		{"b4_retained_notification_c15_private.go"},
		{"b4_retained_response_c15_private.go"},
	} {
		t.Run(check.path, func(t *testing.T) {
			source, err := os.ReadFile(check.path)
			if err != nil {
				t.Fatal(err)
			}
			privateC18AssertMappedBarrierSource(t, source, "Release", "release", "privateC18Release")
		})
	}
}

func TestADR0011C18BarrierCallsAreAtMappedOwnerBoundaries(t *testing.T) {
	checks := []struct {
		path, function, operation, tag string
	}{
		{"b4_retained_notification_c15_private.go", "newPrivateB4RetainedNotificationOwner", "reservePrivateB4C15", "privateC18RetentionCommit"},
		{"b4_retained_notification_c15_private.go", "WithNotifications", "aliasPrivateB4C15", "privateC18ReadbackBorrow"},
		{"b4_retained_notification_c15_private.go", "Release", "release", "privateC18Release"},
		{"b4_retained_response_c15_private.go", "newPrivateB4RetainedResponseOwner", "reservePrivateB4C15", "privateC18RetentionCommit"},
		{"b4_retained_response_c15_private.go", "WithResponses", "aliasPrivateB4C15", "privateC18ReadbackBorrow"},
		{"b4_retained_response_c15_private.go", "Release", "release", "privateC18Release"},
	}
	for _, check := range checks {
		source, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatal(err)
		}
		privateC18AssertMappedBarrierSource(t, source, check.function, check.operation, check.tag)
		if err := privateC18BarrierOutsideOwnerLockError(source, check.function); err != nil {
			t.Fatal(err)
		}
	}
	// These coordinator functions are part of the expected production contract.
	// Their callbacks must pass the same recursive lock-region oracle; omission is
	// a causal RED seam, not a report-counter substitute.
	for _, check := range []struct{ path, function string }{
		{"c18_pair_coordinator_private.go", "commitPrivateC18ObjectEventForPair8Test"},
		{"c18_pair_coordinator_private.go", "commitPrivateC18RetentionForPair8Test"},
		{"c18_pair_coordinator_private.go", "commitPrivateC18RetentionForPair9Test"},
		{"c18_pair_coordinator_private.go", "borrowPrivateC18ReadbackForPair9Test"},
		{"c18_pair_coordinator_private.go", "runPrivateC18Pair8PostDecisionCancellationForTest"},
		{"c18_pair_coordinator_private.go", "runPrivateC18Pair9PostDecisionCancellationForTest"},
	} {
		source, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("ASSERT_C18_COORDINATOR_SOURCE_ABSENT path=%s function=%s err=%v", check.path, check.function, err)
		}
		if err := privateC18BarrierOutsideOwnerLockError(source, check.function); err != nil {
			t.Fatal(err)
		}
	}
}

func TestADR0011C18BarrierOwnerLockOracle(t *testing.T) {
	const function = "mapped"
	cases := []struct {
		name, source string
		want         bool
	}{
		{"outside-lock", `package synthetic
func mapped() {
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
	o.mu.Lock()
	o.mu.Unlock()
}`, true},
		{"under-lock", `package synthetic
func mapped() {
	o.mu.Lock()
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
	o.mu.Unlock()
}`, false},
		{"nested-under-lock", `package synthetic
func mapped() {
	o.mu.Lock()
	if ready {
		reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
	}
	o.mu.Unlock()
}`, false},
		{"deferred-unlock-still-held", `package synthetic
func mapped() {
	o.mu.Lock()
	defer o.mu.Unlock()
	reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
}`, false},
		{"selector-alias-under-lock", `package synthetic
func mapped() {
	lock := o.coordinatorMu
	lock.Lock()
	for ready {
		reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
	}
	lock.Unlock()
}`, false},
		{"nested-rlock-deferred-runlock", `package synthetic
func mapped() {
	if ready {
		o.coordinatorMu.RLock()
		defer o.coordinatorMu.RUnlock()
		for contender := range contenders {
			_ = contender
			reachPrivateC18OwnerBarrier(transactionID, objectID, sequence, privateC18FixtureRetentionCommit)
		}
	}
}`, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := privateC18BarrierOutsideOwnerLockError([]byte(test.source), function)
			if got := err == nil; got != test.want {
				t.Fatalf("ASSERT_C18_OWNER_LOCK_ORACLE got=%t want=%t err=%v", got, test.want, err)
			}
		})
	}
}

// The adapters deliberately own no production contract types. They discover
// production-owned methods, construct their arguments from exact named fields,
// and mechanically project production returns into test-only observation values.
type privateC18TestPair8CoordinatorAdapter struct{ privateC18ReflectAdapter }
type privateC18TestPair9CoordinatorAdapter struct{ privateC18ReflectAdapter }

type privateC18Pair8OperationResult struct {
	Lease   RetainedNotificationLease
	Failure session.Failure
}

type privateC18Pair9OperationResult struct {
	Lease   RetainedResponseLease
	Failure session.Failure
	Err     error
}

type privateC18PreparedPair8ObjectEventCall func(context.Context, privateB4EventIdentityC17) (privateC18Pair8OperationResult, error)
type privateC18PreparedPair8RetentionCall func(context.Context) (privateC18Pair8OperationResult, error)
type privateC18PreparedPair9RetentionCall func(context.Context) (privateC18Pair9OperationResult, error)
type privateC18PreparedPair9ReadbackCall func(context.Context, func([]lspwire.Message) error) (privateC18Pair9OperationResult, error)
type privateC18PreparedPair8AuthorityCall func(context.Context, privateC18FixtureContender, privateC18TestAuthority) (privateC18Pair8OperationResult, error)
type privateC18PreparedPair9AuthorityCall func(context.Context, privateC18FixtureContender, privateC18TestAuthority, func([]lspwire.Message) error) (privateC18Pair9OperationResult, error)
type privateC18PreparedPair8LockProbe func(context.Context, func()) (session.Failure, error)
type privateC18PreparedPair9LockProbe func(context.Context, func()) error
type privateC18PreparedSnapshotCall func() error

var (
	privateC18ContextType            = reflect.TypeOf((*context.Context)(nil)).Elem()
	privateC18FailureType            = reflect.TypeOf(session.Failure(""))
	privateC18ErrorType              = reflect.TypeOf((*error)(nil)).Elem()
	privateC18NotificationLeaseType  = reflect.TypeOf(RetainedNotificationLease{})
	privateC18ResponseLeaseType      = reflect.TypeOf(RetainedResponseLease{})
	privateC18MessagesCallbackType   = reflect.TypeOf((func([]lspwire.Message) error)(nil))
	privateC18NoArgumentCallbackType = reflect.TypeOf((func())(nil))
)

func privateC18RequireMethodTypes(t *testing.T, adapter privateC18ReflectAdapter, name string, in, out []reflect.Type) reflect.Value {
	t.Helper()
	method := adapter.method(t, name, len(in), len(out))
	for i, want := range in {
		if method.Type().In(i) != want {
			t.Fatalf("ASSERT_C18_%s_PRODUCTION_METHOD_INPUT method=%s index=%d got=%v want=%v", adapter.label, name, i, method.Type().In(i), want)
		}
	}
	for i, want := range out {
		if method.Type().Out(i) != want {
			t.Fatalf("ASSERT_C18_%s_PRODUCTION_METHOD_OUTPUT method=%s index=%d got=%v want=%v", adapter.label, name, i, method.Type().Out(i), want)
		}
	}
	return method
}

func privateC18RequireStructFields(t *testing.T, typ reflect.Type, fields map[string]reflect.Type) {
	t.Helper()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || typ.NumField() != len(fields) {
		t.Fatalf("ASSERT_C18_PRODUCTION_STRUCT_TYPE got=%v fields=%d want=%d", typ, typ.NumField(), len(fields))
	}
	for name, want := range fields {
		field, ok := typ.FieldByName(name)
		if !ok || field.PkgPath != "" || field.Type != want {
			t.Fatalf("ASSERT_C18_PRODUCTION_STRUCT_FIELD type=%v field=%s got=%v want=%v", typ, name, field.Type, want)
		}
	}
}

func privateC18SafeReflectCall(method reflect.Value, args ...reflect.Value) (out []reflect.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("ASSERT_C18_PRODUCTION_REFLECT_CALL: %v", recovered)
		}
	}()
	if len(args) != method.Type().NumIn() {
		return nil, fmt.Errorf("ASSERT_C18_PRODUCTION_ARGUMENT_COUNT got=%d want=%d", len(args), method.Type().NumIn())
	}
	for i, arg := range args {
		if !arg.IsValid() || !arg.Type().AssignableTo(method.Type().In(i)) {
			return nil, fmt.Errorf("ASSERT_C18_PRODUCTION_ARGUMENT_TYPE index=%d got=%v want=%v", i, privateC18ReflectType(arg), method.Type().In(i))
		}
	}
	return method.Call(args), nil
}

func privateC18PreparedStruct(typ reflect.Type, values map[string]any) (reflect.Value, error) {
	pointer := typ.Kind() == reflect.Pointer
	if pointer {
		typ = typ.Elem()
	}
	value := reflect.New(typ).Elem()
	for name, raw := range values {
		field := value.FieldByName(name)
		got := reflect.ValueOf(raw)
		if !field.IsValid() || !field.CanSet() || !got.IsValid() || !got.Type().AssignableTo(field.Type()) {
			return reflect.Value{}, fmt.Errorf("ASSERT_C18_PRODUCTION_STRUCT_FIELD_VALUE field=%s got=%v want=%v", name, privateC18ReflectType(got), field.Type())
		}
		field.Set(got)
	}
	if pointer {
		result := reflect.New(typ)
		result.Elem().Set(value)
		return result, nil
	}
	return value, nil
}

func privateC18PreparedCallback(want reflect.Type, callback any) (reflect.Value, error) {
	if callback == nil {
		return reflect.Zero(want), nil
	}
	got := reflect.ValueOf(callback)
	if want.Kind() != reflect.Func || got.Type() != want {
		return reflect.Value{}, fmt.Errorf("ASSERT_C18_PRODUCTION_CALLBACK_SIGNATURE got=%v want=%v", got.Type(), want)
	}
	return got, nil
}

func privateC18Pair8Result(out []reflect.Value) (privateC18Pair8OperationResult, error) {
	lease, ok := out[0].Interface().(RetainedNotificationLease)
	if !ok || out[1].Type() != privateC18FailureType {
		return privateC18Pair8OperationResult{}, fmt.Errorf("ASSERT_C18_PAIR8_RETURN_TYPES lease=%v failure=%v", out[0].Type(), out[1].Type())
	}
	return privateC18Pair8OperationResult{Lease: lease, Failure: out[1].Interface().(session.Failure)}, nil
}

func privateC18Pair9FailureResult(out []reflect.Value) (privateC18Pair9OperationResult, error) {
	lease, ok := out[0].Interface().(RetainedResponseLease)
	if !ok || out[1].Type() != privateC18FailureType {
		return privateC18Pair9OperationResult{}, fmt.Errorf("ASSERT_C18_PAIR9_RETURN_TYPES lease=%v failure=%v", out[0].Type(), out[1].Type())
	}
	return privateC18Pair9OperationResult{Lease: lease, Failure: out[1].Interface().(session.Failure)}, nil
}

func privateC18Pair9ErrorResult(out []reflect.Value) (privateC18Pair9OperationResult, error) {
	lease, ok := out[0].Interface().(RetainedResponseLease)
	if !ok || out[1].Type() != privateC18ErrorType {
		return privateC18Pair9OperationResult{}, fmt.Errorf("ASSERT_C18_PAIR9_RETURN_TYPES lease=%v error=%v", out[0].Type(), out[1].Type())
	}
	var callErr error
	if !out[1].IsNil() {
		callErr = out[1].Interface().(error)
	}
	return privateC18Pair9OperationResult{Lease: lease, Err: callErr}, nil
}

func privateC18PreparePair8ObjectEventCall(t *testing.T, adapter privateC18TestPair8CoordinatorAdapter) privateC18PreparedPair8ObjectEventCall {
	t.Helper()
	method := adapter.method(t, "CommitPrivateC18ObjectEventForPair8Test", 2, 2)
	if method.Type().In(0) != privateC18ContextType || method.Type().Out(0) != privateC18NotificationLeaseType || method.Type().Out(1) != privateC18FailureType {
		t.Fatalf("ASSERT_C18_PAIR8_OBJECT_EVENT_SIGNATURE type=%v", method.Type())
	}
	identityType := method.Type().In(1)
	privateC18RequireStructFields(t, identityType, map[string]reflect.Type{"Family": reflect.TypeOf(""), "Kind": reflect.TypeOf(""), "Ordinal": reflect.TypeOf(0)})
	return func(ctx context.Context, identity privateB4EventIdentityC17) (privateC18Pair8OperationResult, error) {
		argument, err := privateC18PreparedStruct(identityType, map[string]any{"Family": identity.Family, "Kind": identity.Kind, "Ordinal": identity.Ordinal})
		if err != nil {
			return privateC18Pair8OperationResult{}, err
		}
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), argument)
		if err != nil {
			return privateC18Pair8OperationResult{}, err
		}
		return privateC18Pair8Result(out)
	}
}

func privateC18PreparePair8RetentionCall(t *testing.T, adapter privateC18TestPair8CoordinatorAdapter) privateC18PreparedPair8RetentionCall {
	method := privateC18RequireMethodTypes(t, adapter.privateC18ReflectAdapter, "CommitPrivateC18RetentionForPair8Test", []reflect.Type{privateC18ContextType}, []reflect.Type{privateC18NotificationLeaseType, privateC18FailureType})
	return func(ctx context.Context) (privateC18Pair8OperationResult, error) {
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx))
		if err != nil {
			return privateC18Pair8OperationResult{}, err
		}
		return privateC18Pair8Result(out)
	}
}

func privateC18PreparePair9RetentionCall(t *testing.T, adapter privateC18TestPair9CoordinatorAdapter) privateC18PreparedPair9RetentionCall {
	method := privateC18RequireMethodTypes(t, adapter.privateC18ReflectAdapter, "CommitPrivateC18RetentionForPair9Test", []reflect.Type{privateC18ContextType}, []reflect.Type{privateC18ResponseLeaseType, privateC18FailureType})
	return func(ctx context.Context) (privateC18Pair9OperationResult, error) {
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx))
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		return privateC18Pair9FailureResult(out)
	}
}

func privateC18PreparePair9ReadbackCall(t *testing.T, adapter privateC18TestPair9CoordinatorAdapter) privateC18PreparedPair9ReadbackCall {
	t.Helper()
	method := adapter.method(t, "BorrowPrivateC18ReadbackForPair9Test", 2, 2)
	if method.Type().In(0) != privateC18ContextType || method.Type().In(1) != privateC18MessagesCallbackType || method.Type().Out(0) != privateC18ResponseLeaseType || method.Type().Out(1) != privateC18ErrorType {
		t.Fatalf("ASSERT_C18_PAIR9_READBACK_SIGNATURE type=%v", method.Type())
	}
	return func(ctx context.Context, callback func([]lspwire.Message) error) (privateC18Pair9OperationResult, error) {
		cb, err := privateC18PreparedCallback(method.Type().In(1), callback)
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), cb)
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		return privateC18Pair9ErrorResult(out)
	}
}

func privateC18PrepareAuthorityArguments(t *testing.T, adapter privateC18ReflectAdapter, method reflect.Value) (reflect.Type, reflect.Type) {
	t.Helper()
	contenderType, authorityType := method.Type().In(1), method.Type().In(2)
	if err := privateC18NamedPrimitiveTypeError(contenderType, "privateC18Contender", reflect.TypeOf(privateC18FixtureContender("")).PkgPath(), reflect.String, 0); err != nil {
		t.Fatal(err)
	}
	privateC18RequireStructFields(t, authorityType, map[string]reflect.Type{"Generation": reflect.TypeOf(uint64(0)), "TransactionID": reflect.TypeOf(""), "ObjectID": reflect.TypeOf("")})
	return contenderType, authorityType
}

func privateC18PreparedAuthorityValues(contenderType, authorityType reflect.Type, contender privateC18FixtureContender, authority privateC18TestAuthority) (reflect.Value, reflect.Value, error) {
	contenderValue := reflect.New(contenderType).Elem()
	contenderValue.SetString(string(contender))
	authorityValue, err := privateC18PreparedStruct(authorityType, map[string]any{"Generation": authority.Generation, "TransactionID": authority.TransactionID, "ObjectID": authority.ObjectID})
	return contenderValue, authorityValue, err
}

func privateC18PreparePair8AuthorityCall(t *testing.T, adapter privateC18TestPair8CoordinatorAdapter) privateC18PreparedPair8AuthorityCall {
	t.Helper()
	method := adapter.method(t, "CommitPrivateC18Pair8ContenderWithAuthorityForTest", 3, 2)
	if method.Type().In(0) != privateC18ContextType || method.Type().Out(0) != privateC18NotificationLeaseType || method.Type().Out(1) != privateC18FailureType {
		t.Fatalf("ASSERT_C18_PAIR8_AUTHORITY_SIGNATURE type=%v", method.Type())
	}
	contenderType, authorityType := privateC18PrepareAuthorityArguments(t, adapter.privateC18ReflectAdapter, method)
	return func(ctx context.Context, contender privateC18FixtureContender, authority privateC18TestAuthority) (privateC18Pair8OperationResult, error) {
		cv, av, err := privateC18PreparedAuthorityValues(contenderType, authorityType, contender, authority)
		if err != nil {
			return privateC18Pair8OperationResult{}, err
		}
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), cv, av)
		if err != nil {
			return privateC18Pair8OperationResult{}, err
		}
		return privateC18Pair8Result(out)
	}
}

func privateC18PreparePair9AuthorityCall(t *testing.T, adapter privateC18TestPair9CoordinatorAdapter) privateC18PreparedPair9AuthorityCall {
	t.Helper()
	method := adapter.method(t, "CommitPrivateC18Pair9ContenderWithAuthorityForTest", 4, 2)
	if method.Type().In(0) != privateC18ContextType || method.Type().In(3) != privateC18MessagesCallbackType || method.Type().Out(0) != privateC18ResponseLeaseType || method.Type().Out(1) != privateC18ErrorType {
		t.Fatalf("ASSERT_C18_PAIR9_AUTHORITY_SIGNATURE type=%v", method.Type())
	}
	contenderType, authorityType := privateC18PrepareAuthorityArguments(t, adapter.privateC18ReflectAdapter, method)
	return func(ctx context.Context, contender privateC18FixtureContender, authority privateC18TestAuthority, callback func([]lspwire.Message) error) (privateC18Pair9OperationResult, error) {
		cv, av, err := privateC18PreparedAuthorityValues(contenderType, authorityType, contender, authority)
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		cb, err := privateC18PreparedCallback(method.Type().In(3), callback)
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), cv, av, cb)
		if err != nil {
			return privateC18Pair9OperationResult{}, err
		}
		return privateC18Pair9ErrorResult(out)
	}
}

func privateC18PrepareSnapshotCall(t *testing.T, adapter privateC18ReflectAdapter, name string) privateC18PreparedSnapshotCall {
	t.Helper()
	method := adapter.method(t, name, 0, 1)
	out := method.Type().Out(0)
	for out.Kind() == reflect.Pointer {
		out = out.Elem()
	}
	if out.Kind() != reflect.Struct {
		t.Fatalf("ASSERT_C18_SNAPSHOT_SIGNATURE method=%s type=%v", name, method.Type())
	}
	return func() error { _, err := privateC18SafeReflectCall(method); return err }
}

func privateC18PreparePair8LockProbe(t *testing.T, adapter privateC18TestPair8CoordinatorAdapter) privateC18PreparedPair8LockProbe {
	t.Helper()
	method := adapter.method(t, "ProbePrivateC18Pair8LockSafetyForTest", 2, 1)
	if method.Type().In(0) != privateC18ContextType || method.Type().In(1) != privateC18NoArgumentCallbackType || method.Type().Out(0) != privateC18FailureType {
		t.Fatalf("ASSERT_C18_PAIR8_LOCK_PROBE_SIGNATURE type=%v", method.Type())
	}
	return func(ctx context.Context, callback func()) (session.Failure, error) {
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), reflect.ValueOf(callback))
		if err != nil {
			return "", err
		}
		return out[0].Interface().(session.Failure), nil
	}
}

func privateC18PreparePair9LockProbe(t *testing.T, adapter privateC18TestPair9CoordinatorAdapter) privateC18PreparedPair9LockProbe {
	t.Helper()
	method := adapter.method(t, "ProbePrivateC18Pair9LockSafetyForTest", 2, 1)
	if method.Type().In(0) != privateC18ContextType || method.Type().In(1) != privateC18NoArgumentCallbackType || method.Type().Out(0) != privateC18ErrorType {
		t.Fatalf("ASSERT_C18_PAIR9_LOCK_PROBE_SIGNATURE type=%v", method.Type())
	}
	return func(ctx context.Context, callback func()) error {
		out, err := privateC18SafeReflectCall(method, reflect.ValueOf(ctx), reflect.ValueOf(callback))
		if err != nil {
			return err
		}
		if out[0].IsNil() {
			return nil
		}
		return out[0].Interface().(error)
	}
}

type privateC18PreparedLegacyPair8ObjectEvent func(privateB4EventIdentityC17) (session.Failure, error)
type privateC18PreparedLegacyPair8Retention func() (RetainedNotificationLease, session.Failure, error)
type privateC18PreparedLegacyPair9Retention func() (RetainedResponseLease, session.Failure, error)
type privateC18PreparedLeaseWithResponses func(func([]lspwire.Message) error) error
type privateC18PreparedNotificationRelease func() (bool, error)
type privateC18PreparedResponseRelease func() (bool, error)

func privateC18PrepareLegacyPair8ObjectEvent(t *testing.T, owner privateC18ReflectAdapter) privateC18PreparedLegacyPair8ObjectEvent {
	t.Helper()
	method := owner.method(t, "CommitPrivateC18ObjectEventForTest", 1, 1)
	if method.Type().Out(0) != privateC18FailureType {
		t.Fatalf("ASSERT_C18_LEGACY_PAIR8_OBJECT_EVENT_SIGNATURE type=%v", method.Type())
	}
	identityType := method.Type().In(0)
	privateC18RequireStructFields(t, identityType, map[string]reflect.Type{"Family": reflect.TypeOf(""), "Kind": reflect.TypeOf(""), "Ordinal": reflect.TypeOf(0)})
	return func(identity privateB4EventIdentityC17) (session.Failure, error) {
		argument, err := privateC18PreparedStruct(identityType, map[string]any{"Family": identity.Family, "Kind": identity.Kind, "Ordinal": identity.Ordinal})
		if err != nil {
			return "", err
		}
		out, err := privateC18SafeReflectCall(method, argument)
		if err != nil {
			return "", err
		}
		return out[0].Interface().(session.Failure), nil
	}
}

func privateC18PrepareLegacyPair8Retention(t *testing.T, owner privateC18ReflectAdapter) privateC18PreparedLegacyPair8Retention {
	t.Helper()
	method := privateC18RequireMethodTypes(t, owner, "CommitPrivateC18RetentionForTest", nil, []reflect.Type{privateC18NotificationLeaseType, privateC18FailureType})
	return func() (RetainedNotificationLease, session.Failure, error) {
		out, err := privateC18SafeReflectCall(method)
		if err != nil {
			return RetainedNotificationLease{}, "", err
		}
		result, err := privateC18Pair8Result(out)
		return result.Lease, result.Failure, err
	}
}

func privateC18PrepareLegacyPair9Retention(t *testing.T, owner privateC18ReflectAdapter) privateC18PreparedLegacyPair9Retention {
	t.Helper()
	method := privateC18RequireMethodTypes(t, owner, "CommitPrivateC18RetentionForTest", nil, []reflect.Type{privateC18ResponseLeaseType, privateC18FailureType})
	return func() (RetainedResponseLease, session.Failure, error) {
		out, err := privateC18SafeReflectCall(method)
		if err != nil {
			return RetainedResponseLease{}, "", err
		}
		result, err := privateC18Pair9FailureResult(out)
		return result.Lease, result.Failure, err
	}
}

func privateC18PrepareLeaseWithResponses(lease RetainedResponseLease) privateC18PreparedLeaseWithResponses {
	return func(callback func([]lspwire.Message) error) error { return lease.WithResponses(callback) }
}

func privateC18PrepareNotificationRelease(lease RetainedNotificationLease) privateC18PreparedNotificationRelease {
	return func() (bool, error) { return lease.Release(), nil }
}

func privateC18PrepareResponseRelease(lease RetainedResponseLease) privateC18PreparedResponseRelease {
	return func() (bool, error) { return lease.Release(), nil }
}

func privateC18Failure(t *testing.T, value reflect.Value) session.Failure {
	t.Helper()
	if value.Type() != reflect.TypeOf(session.Failure("")) {
		t.Fatalf("ASSERT_C18_PRODUCTION_FAILURE_TYPE got=%v", value.Type())
	}
	return value.Interface().(session.Failure)
}

func privateC18Error(t *testing.T, value reflect.Value) error {
	t.Helper()
	if value.Type() != reflect.TypeOf((*error)(nil)).Elem() {
		t.Fatalf("ASSERT_C18_PRODUCTION_ERROR_TYPE got=%v", value.Type())
	}
	if value.IsNil() {
		return nil
	}
	return value.Interface().(error)
}

func (a privateC18ReflectAdapter) callContext(t *testing.T, name string, out int, ctx context.Context, tail ...reflect.Value) []reflect.Value {
	t.Helper()
	method := a.method(t, name, 1+len(tail), out)
	args := []reflect.Value{privateC18PrimitiveValue(t, method.Type().In(0), ctx)}
	args = append(args, tail...)
	return privateC18ReflectCall(t, method, args...)
}

func privateC18Snapshot(t *testing.T, value reflect.Value) privateC18TestCoordinatorSnapshot {
	t.Helper()
	var snapshot privateC18TestCoordinatorSnapshot
	privateC18ExtractStruct(t, value, map[string]any{
		"Pair": &snapshot.Pair, "Decisions": &snapshot.Decisions,
		"AuthorizedTransitions": &snapshot.AuthorizedTransitions, "WinnerEffects": &snapshot.WinnerEffects,
		"LoserEffects": &snapshot.LoserEffects, "TerminalDispositions": &snapshot.TerminalDispositions,
		"ProvisionalAuthority": &snapshot.ProvisionalAuthority, "Stopped": &snapshot.Stopped,
		"Generation": &snapshot.Generation,
	}, 13)
	snapshot.Reached = privateC18ProductionBoundary(t, value.FieldByName("Reached"))
	snapshot.Winner = privateC18ProductionSnapshotContender(t, value.FieldByName("Winner"))
	snapshot.Loser = privateC18ProductionSnapshotContender(t, value.FieldByName("Loser"))
	history := value.FieldByName("ReachedHistory")
	if history.Kind() != reflect.Slice {
		t.Fatalf("ASSERT_C18_PRODUCTION_RETURN_FIELD_TYPE field=ReachedHistory got=%v want=slice", history.Type())
	}
	snapshot.ReachedHistory = make([]privateC18FixtureOwnerBoundary, history.Len())
	for i := range snapshot.ReachedHistory {
		snapshot.ReachedHistory[i] = privateC18ProductionBoundary(t, history.Index(i))
	}
	return snapshot
}

func privateC18AuthorityValue(t *testing.T, want reflect.Type, authority privateC18TestAuthority) reflect.Value {
	return privateC18ProductionStruct(t, want, map[string]any{
		"Generation": authority.Generation, "TransactionID": authority.TransactionID, "ObjectID": authority.ObjectID,
	})
}

func (a privateC18TestPair8CoordinatorAdapter) commitPrivateC18ObjectEventForPair8Test(ctx context.Context, identity privateB4EventIdentityC17) (RetainedNotificationLease, session.Failure) {
	method := a.method(a.t(), "CommitPrivateC18ObjectEventForPair8Test", 2, 2)
	argument := privateC18ProductionStruct(a.t(), method.Type().In(1), map[string]any{"Family": identity.Family, "Kind": identity.Kind, "Ordinal": identity.Ordinal})
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), argument)
	lease, ok := out[0].Interface().(RetainedNotificationLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR8_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Failure(a.t(), out[1])
}

func (a privateC18TestPair8CoordinatorAdapter) t() *testing.T { return a.privateC18ReflectAdapter.t }
func (a privateC18TestPair9CoordinatorAdapter) t() *testing.T { return a.privateC18ReflectAdapter.t }

func (a privateC18TestPair8CoordinatorAdapter) commitPrivateC18RetentionForPair8Test(ctx context.Context) (RetainedNotificationLease, session.Failure) {
	out := a.callContext(a.t(), "CommitPrivateC18RetentionForPair8Test", 2, ctx)
	lease, ok := out[0].Interface().(RetainedNotificationLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR8_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Failure(a.t(), out[1])
}

func (a privateC18TestPair9CoordinatorAdapter) commitPrivateC18RetentionForPair9Test(ctx context.Context) (RetainedResponseLease, session.Failure) {
	out := a.callContext(a.t(), "CommitPrivateC18RetentionForPair9Test", 2, ctx)
	lease, ok := out[0].Interface().(RetainedResponseLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR9_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Failure(a.t(), out[1])
}

func privateC18CallbackValue(t *testing.T, want reflect.Type, callback any) reflect.Value {
	t.Helper()
	got := reflect.ValueOf(callback)
	if want.Kind() != reflect.Func || got.Kind() != reflect.Func || want.NumIn() != got.Type().NumIn() || want.NumOut() != got.Type().NumOut() {
		t.Fatalf("ASSERT_C18_PRODUCTION_CALLBACK_SIGNATURE got=%v want=%v", got.Type(), want)
	}
	return reflect.MakeFunc(want, func(args []reflect.Value) []reflect.Value {
		converted := make([]reflect.Value, len(args))
		for i := range args {
			converted[i] = privateC18PrimitiveValue(t, got.Type().In(i), args[i].Interface())
		}
		return got.Call(converted)
	})
}

func (a privateC18TestPair9CoordinatorAdapter) borrowPrivateC18ReadbackForPair9Test(ctx context.Context, callback func([]lspwire.Message) error) (RetainedResponseLease, error) {
	method := a.method(a.t(), "BorrowPrivateC18ReadbackForPair9Test", 2, 2)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18CallbackValue(a.t(), method.Type().In(1), callback))
	lease, ok := out[0].Interface().(RetainedResponseLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR9_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Error(a.t(), out[1])
}

func (a privateC18TestPair8CoordinatorAdapter) retryPrivateC18Pair8LoserForTest(ctx context.Context, contender privateC18FixtureContender) session.Failure {
	method := a.method(a.t(), "RetryPrivateC18Pair8LoserForTest", 2, 1)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18ProductionContenderValue(a.t(), method.Type().In(1), contender))
	return privateC18Failure(a.t(), out[0])
}

func (a privateC18TestPair9CoordinatorAdapter) retryPrivateC18Pair9LoserForTest(ctx context.Context, contender privateC18FixtureContender) error {
	method := a.method(a.t(), "RetryPrivateC18Pair9LoserForTest", 2, 1)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18ProductionContenderValue(a.t(), method.Type().In(1), contender))
	return privateC18Error(a.t(), out[0])
}

func (a privateC18TestPair8CoordinatorAdapter) privateC18Pair8CoordinatorSnapshotForTest() privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), privateC18ReflectCall(a.t(), a.method(a.t(), "PrivateC18Pair8CoordinatorSnapshotForTest", 0, 1))[0])
}
func (a privateC18TestPair9CoordinatorAdapter) privateC18Pair9CoordinatorSnapshotForTest() privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), privateC18ReflectCall(a.t(), a.method(a.t(), "PrivateC18Pair9CoordinatorSnapshotForTest", 0, 1))[0])
}

func privateC18InstallSeamWithRestore(t *testing.T, adapter privateC18ReflectAdapter, name string, seam privateC18TestSplitPhaseSeam) (func(), error) {
	t.Helper()
	method := adapter.method(t, name, 1, 2)
	want := method.Type().In(0)
	if want.Kind() != reflect.Struct {
		t.Fatalf("ASSERT_C18_PRODUCTION_SEAM_TYPE got=%v", want)
	}
	reachedField, reachedOK := want.FieldByName("Reached")
	decisionField, decisionOK := want.FieldByName("Decision")
	neutralSend := reflect.TypeOf((chan<- any)(nil))
	if !reachedOK || !decisionOK || reachedField.Type != neutralSend || decisionField.Type != neutralSend {
		t.Fatalf("ASSERT_C18_PRODUCTION_SEAM_CHANNEL_FIELDS type=%v", want)
	}
	value := reflect.New(want).Elem()
	value.FieldByIndex(reachedField.Index).Set(reflect.ValueOf(seam.Reached))
	value.FieldByIndex(decisionField.Index).Set(reflect.ValueOf(seam.Decision))
	for fieldName, channel := range map[string]any{"DecisionRelease": seam.DecisionRelease, "EffectRelease": seam.EffectRelease} {
		field, ok := want.FieldByName(fieldName)
		if !ok {
			t.Fatalf("ASSERT_C18_PRODUCTION_SEAM_FIELD field=%s", fieldName)
		}
		value.FieldByIndex(field.Index).Set(privateC18PrimitiveValue(t, field.Type, channel))
	}
	out := privateC18ReflectCall(t, method, value)
	restoreType := out[0].Type()
	if restoreType.Kind() != reflect.Func || restoreType.NumIn() != 0 || restoreType.NumOut() != 0 || out[0].IsNil() {
		t.Fatalf("ASSERT_C18_SPLIT_PHASE_RESTORE_SIGNATURE type=%v nil=%t", restoreType, out[0].IsNil())
	}
	if err := privateC18Error(t, out[1]); err != nil {
		return nil, err
	}
	var once sync.Once
	restore := func() { once.Do(func() { out[0].Call(nil) }) }
	t.Cleanup(restore)
	return restore, nil
}

func privateC18InstallSeam(t *testing.T, adapter privateC18ReflectAdapter, name string, seam privateC18TestSplitPhaseSeam) error {
	_, err := privateC18InstallSeamWithRestore(t, adapter, name, seam)
	return err
}

func (a privateC18TestPair8CoordinatorAdapter) installPrivateC18Pair8SplitPhaseSeamForTest(seam privateC18TestSplitPhaseSeam) error {
	return privateC18InstallSeam(a.t(), a.privateC18ReflectAdapter, "InstallPrivateC18Pair8SplitPhaseSeamForTest", seam)
}
func (a privateC18TestPair9CoordinatorAdapter) installPrivateC18Pair9SplitPhaseSeamForTest(seam privateC18TestSplitPhaseSeam) error {
	return privateC18InstallSeam(a.t(), a.privateC18ReflectAdapter, "InstallPrivateC18Pair9SplitPhaseSeamForTest", seam)
}
func (a privateC18TestPair8CoordinatorAdapter) installPrivateC18Pair8SplitPhaseSeamRestoreForTest(seam privateC18TestSplitPhaseSeam) (func(), error) {
	return privateC18InstallSeamWithRestore(a.t(), a.privateC18ReflectAdapter, "InstallPrivateC18Pair8SplitPhaseSeamForTest", seam)
}
func (a privateC18TestPair9CoordinatorAdapter) installPrivateC18Pair9SplitPhaseSeamRestoreForTest(seam privateC18TestSplitPhaseSeam) (func(), error) {
	return privateC18InstallSeamWithRestore(a.t(), a.privateC18ReflectAdapter, "InstallPrivateC18Pair9SplitPhaseSeamForTest", seam)
}

func (a privateC18TestPair8CoordinatorAdapter) commitPrivateC18Pair8ContenderWithAuthorityForTest(ctx context.Context, contender privateC18FixtureContender, authority privateC18TestAuthority) (RetainedNotificationLease, session.Failure) {
	method := a.method(a.t(), "CommitPrivateC18Pair8ContenderWithAuthorityForTest", 3, 2)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18ProductionContenderValue(a.t(), method.Type().In(1), contender), privateC18AuthorityValue(a.t(), method.Type().In(2), authority))
	lease, ok := out[0].Interface().(RetainedNotificationLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR8_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Failure(a.t(), out[1])
}

func (a privateC18TestPair9CoordinatorAdapter) commitPrivateC18Pair9ContenderWithAuthorityForTest(ctx context.Context, contender privateC18FixtureContender, authority privateC18TestAuthority, callback func([]lspwire.Message) error) (RetainedResponseLease, error) {
	method := a.method(a.t(), "CommitPrivateC18Pair9ContenderWithAuthorityForTest", 4, 2)
	callbackValue := reflect.Zero(method.Type().In(3))
	if callback != nil {
		callbackValue = privateC18CallbackValue(a.t(), method.Type().In(3), callback)
	}
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18ProductionContenderValue(a.t(), method.Type().In(1), contender), privateC18AuthorityValue(a.t(), method.Type().In(2), authority), callbackValue)
	lease, ok := out[0].Interface().(RetainedResponseLease)
	if !ok {
		a.t().Fatalf("ASSERT_C18_PAIR9_LEASE_TYPE got=%v", out[0].Type())
	}
	return lease, privateC18Error(a.t(), out[1])
}

func (a privateC18TestPair8CoordinatorAdapter) stopPrivateC18Pair8CoordinatorForTest(ctx context.Context) session.Failure {
	return privateC18Failure(a.t(), a.callContext(a.t(), "StopPrivateC18Pair8CoordinatorForTest", 1, ctx)[0])
}
func (a privateC18TestPair9CoordinatorAdapter) stopPrivateC18Pair9CoordinatorForTest(ctx context.Context) session.Failure {
	return privateC18Failure(a.t(), a.callContext(a.t(), "StopPrivateC18Pair9CoordinatorForTest", 1, ctx)[0])
}
func (a privateC18TestPair8CoordinatorAdapter) awaitPrivateC18Pair8StoppedForTest(ctx context.Context) privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), a.callContext(a.t(), "AwaitPrivateC18Pair8StoppedForTest", 1, ctx)[0])
}
func (a privateC18TestPair9CoordinatorAdapter) awaitPrivateC18Pair9StoppedForTest(ctx context.Context) privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), a.callContext(a.t(), "AwaitPrivateC18Pair9StoppedForTest", 1, ctx)[0])
}
func (a privateC18TestPair8CoordinatorAdapter) restartPrivateC18Pair8CoordinatorForTest(ctx context.Context) privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), a.callContext(a.t(), "RestartPrivateC18Pair8CoordinatorForTest", 1, ctx)[0])
}
func (a privateC18TestPair9CoordinatorAdapter) restartPrivateC18Pair9CoordinatorForTest(ctx context.Context) privateC18TestCoordinatorSnapshot {
	return privateC18Snapshot(a.t(), a.callContext(a.t(), "RestartPrivateC18Pair9CoordinatorForTest", 1, ctx)[0])
}

func (a privateC18TestPair8CoordinatorAdapter) probePrivateC18Pair8LockSafetyForTest(ctx context.Context, callback func()) session.Failure {
	method := a.method(a.t(), "ProbePrivateC18Pair8LockSafetyForTest", 2, 1)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18CallbackValue(a.t(), method.Type().In(1), callback))
	return privateC18Failure(a.t(), out[0])
}
func (a privateC18TestPair9CoordinatorAdapter) probePrivateC18Pair9LockSafetyForTest(ctx context.Context, callback func()) error {
	method := a.method(a.t(), "ProbePrivateC18Pair9LockSafetyForTest", 2, 1)
	out := privateC18ReflectCall(a.t(), method, privateC18PrimitiveValue(a.t(), method.Type().In(0), ctx), privateC18CallbackValue(a.t(), method.Type().In(1), callback))
	return privateC18Error(a.t(), out[0])
}

type privateC18TestAuthority struct {
	Generation    uint64
	TransactionID string
	ObjectID      string
}

type privateC18TestReachedEvent struct {
	Contender privateC18FixtureContender
	Boundary  privateC18FixtureOwnerBoundary
}

type privateC18TestDecisionEvent struct {
	Winner    privateC18FixtureContender
	Loser     privateC18FixtureContender
	Authority privateC18TestAuthority
}

// The test owns cancellation and gate release. The coordinator alone emits the
// decision event; neither a gate nor the test supplies a winner or result.
type privateC18TestSplitPhaseSeam struct {
	Reached         chan<- any
	Decision        chan<- any
	DecisionRelease <-chan struct{}
	EffectRelease   <-chan struct{}
}

type privateC18TestCoordinatorSnapshot struct {
	Pair                  uint8
	Reached               privateC18FixtureOwnerBoundary
	Winner                privateC18FixtureContender
	Loser                 privateC18FixtureContender
	Decisions             uint64
	AuthorizedTransitions uint64
	WinnerEffects         uint64
	LoserEffects          uint64
	TerminalDispositions  uint64
	ProvisionalAuthority  bool
	Stopped               bool
	Generation            uint64
	ReachedHistory        []privateC18FixtureOwnerBoundary
}

type privateC18ArrivalOrder uint8

const (
	privateC18EarlierStageFirst privateC18ArrivalOrder = iota + 1
	privateC18LaterStageFirst
	privateC18Simultaneous
)

type privateC18Pair8Fixture struct {
	account  *privateB4ByteAccountV2
	owner    *privateB4RetainedNotificationOwner
	host     privateC18TestPair8CoordinatorAdapter
	observer *privateC15AtomicBatchObserver
	barrier  *privateC18ReachedBarrier
}

func newPrivateC18Pair8Fixture(t *testing.T) privateC18Pair8Fixture {
	t.Helper()
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)
	owner, failure := newPrivateB4RetainedNotificationOwner(account, notificationFixture())
	if failure != "" || owner == nil {
		t.Fatalf("BLOCKED_NOT_RED pair8 owner: failure=%q owner=%v", failure, owner)
	}
	host := privateC18TestPair8CoordinatorAdapter{newPrivateC18ReflectAdapter(t, "PAIR8_COORDINATOR", owner)}
	host.require(t, map[string][2]int{
		"CommitPrivateC18ObjectEventForPair8Test": {2, 2}, "CommitPrivateC18RetentionForPair8Test": {1, 2},
		"RetryPrivateC18Pair8LoserForTest": {2, 1}, "PrivateC18Pair8CoordinatorSnapshotForTest": {0, 1},
		"InstallPrivateC18Pair8SplitPhaseSeamForTest": {1, 2}, "CommitPrivateC18Pair8ContenderWithAuthorityForTest": {3, 2},
		"StopPrivateC18Pair8CoordinatorForTest": {1, 1}, "AwaitPrivateC18Pair8StoppedForTest": {1, 1},
		"RestartPrivateC18Pair8CoordinatorForTest": {1, 1}, "ProbePrivateC18Pair8LockSafetyForTest": {2, 1},
	})
	barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
	installPrivateC18Barrier(t, owner, barrier.observe)
	return privateC18Pair8Fixture{account: account, owner: owner, host: host, observer: observer, barrier: barrier}
}

type privateC18Pair9Fixture struct {
	account  *privateB4ByteAccountV2
	owner    *privateB4RetainedResponseOwner
	host     privateC18TestPair9CoordinatorAdapter
	observer *privateC15AtomicBatchObserver
	barrier  *privateC18ReachedBarrier
}

func newPrivateC18Pair9Fixture(t *testing.T) privateC18Pair9Fixture {
	t.Helper()
	account := mustPrivateB4ByteAccountV2(t)
	observer := &privateC15AtomicBatchObserver{}
	privateC15InstallAtomicBatchObserver(t, account, observer)
	owner, failure := newPrivateB4RetainedResponseOwner(account, retainedResponseFixture()[:1])
	if failure != "" || owner == nil {
		t.Fatalf("BLOCKED_NOT_RED pair9 owner: failure=%q owner=%v", failure, owner)
	}
	host := privateC18TestPair9CoordinatorAdapter{newPrivateC18ReflectAdapter(t, "PAIR9_COORDINATOR", owner)}
	host.require(t, map[string][2]int{
		"CommitPrivateC18RetentionForPair9Test": {1, 2}, "BorrowPrivateC18ReadbackForPair9Test": {2, 2},
		"RetryPrivateC18Pair9LoserForTest": {2, 1}, "PrivateC18Pair9CoordinatorSnapshotForTest": {0, 1},
		"InstallPrivateC18Pair9SplitPhaseSeamForTest": {1, 2}, "CommitPrivateC18Pair9ContenderWithAuthorityForTest": {4, 2},
		"StopPrivateC18Pair9CoordinatorForTest": {1, 1}, "AwaitPrivateC18Pair9StoppedForTest": {1, 1},
		"RestartPrivateC18Pair9CoordinatorForTest": {1, 1}, "ProbePrivateC18Pair9LockSafetyForTest": {2, 1},
	})
	barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
	installPrivateC18Barrier(t, owner, barrier.observe)
	return privateC18Pair9Fixture{account: account, owner: owner, host: host, observer: observer, barrier: barrier}
}

func privateC18CanonicalReached(pair uint8) []privateC18FixtureOwnerBoundary {
	if pair == 8 {
		return []privateC18FixtureOwnerBoundary{privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit}
	}
	return []privateC18FixtureOwnerBoundary{privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow}
}

func privateC18ValidateCanonicalReached(got privateC18TestCoordinatorSnapshot, pair uint8) error {
	want := privateC18CanonicalReached(pair)
	if !reflect.DeepEqual(got.ReachedHistory, want) || got.Reached != want[len(want)-1] {
		return fmt.Errorf("ASSERT_C18_EXACT_CANONICAL_SEMANTIC_REACHED pair=%d got=%v reached=%d want=%v", pair, got.ReachedHistory, got.Reached, want)
	}
	return nil
}

func privateC18AssertCommittedDecision(t *testing.T, got privateC18TestCoordinatorSnapshot, pair uint8, winner, loser privateC18FixtureContender) {
	t.Helper()
	if got.Pair != pair || got.Winner != winner || got.Loser != loser || got.Decisions != 1 || got.AuthorizedTransitions != 1 || got.WinnerEffects != 1 || got.LoserEffects != 0 || got.TerminalDispositions != 1 || got.ProvisionalAuthority || got.Stopped {
		t.Fatalf("ASSERT_C18_EXACT_COMMITTED_DECISION pair=%d winner=%q loser=%q snapshot=%+v", pair, winner, loser, got)
	}
	if err := privateC18ValidateCanonicalReached(got, pair); err != nil {
		t.Fatal(err)
	}
}

func privateC18AssertPhysicalArrival(t *testing.T, barrier *privateC18ReachedBarrier, order privateC18ArrivalOrder, arrived [2]privateC18FixtureOwnerBarrierEvent, earlier, later privateC18FixtureOwnerBoundary) {
	t.Helper()
	history := barrier.snapshot()
	if len(history) < 2 || !reflect.DeepEqual(history[:2], arrived[:]) {
		t.Fatalf("ASSERT_C18_PHYSICAL_ARRIVAL_RECORD got=%+v externally_observed=%+v", history, arrived)
	}
	if order == privateC18Simultaneous {
		seen := map[privateC18FixtureOwnerBoundary]int{}
		for _, event := range arrived {
			seen[event.Boundary]++
		}
		if len(seen) != 2 || seen[earlier] != 1 || seen[later] != 1 {
			t.Fatalf("ASSERT_C18_SIMULTANEOUS_PHYSICAL_ARRIVAL_MULTISET got=%+v want={%d,%d}", arrived, earlier, later)
		}
		return
	}
	want := [2]privateC18FixtureOwnerBoundary{earlier, later}
	if order == privateC18LaterStageFirst {
		want = [2]privateC18FixtureOwnerBoundary{later, earlier}
	}
	if arrived[0].Boundary != want[0] || arrived[1].Boundary != want[1] {
		t.Fatalf("ASSERT_C18_EXACT_FORCED_PHYSICAL_ARRIVAL got={%d,%d} want=%v", arrived[0].Boundary, arrived[1].Boundary, want)
	}
}

func TestADR0011C18PhysicalArrivalAndCanonicalSemanticReachedAreIndependent(t *testing.T) {
	laterFirst := [2]privateC18FixtureOwnerBarrierEvent{{Boundary: privateC18FixtureRetentionCommit}, {Boundary: privateC18FixtureObjectEventCommit}}
	barrier := newPrivateC18ReachedBarrier()
	barrier.events = append(barrier.events, laterFirst[:]...)
	privateC18AssertPhysicalArrival(t, barrier, privateC18LaterStageFirst, laterFirst, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
	canonical := privateC18TestCoordinatorSnapshot{Reached: privateC18FixtureRetentionCommit, ReachedHistory: privateC18CanonicalReached(8)}
	if err := privateC18ValidateCanonicalReached(canonical, 8); err != nil {
		t.Fatalf("ASSERT_C18_LATER_FIRST_WITH_CANONICAL_SEMANTIC_REACHED err=%v", err)
	}
	swapped := canonical
	swapped.Reached = privateC18FixtureObjectEventCommit
	swapped.ReachedHistory = []privateC18FixtureOwnerBoundary{privateC18FixtureRetentionCommit, privateC18FixtureObjectEventCommit}
	if err := privateC18ValidateCanonicalReached(swapped, 8); err == nil {
		t.Fatal("ASSERT_C18_SWAPPED_SEMANTIC_REACHED_MUST_FAIL")
	}
}

func privateC18AssertExactBarrierIdentity(t *testing.T, reached []privateC18FixtureOwnerBarrierEvent) {
	t.Helper()
	if len(reached) != 3 {
		t.Fatalf("ASSERT_C18_EXACT_BARRIER_IDENTITY_CARDINALITY got=%+v", reached)
	}
	transactionID, objectID := reached[0].TransactionID, reached[0].ObjectID
	for _, event := range reached {
		if event.TransactionID != transactionID || event.ObjectID != objectID || transactionID == "" || objectID == "" {
			t.Fatalf("ASSERT_C18_EXACT_TRANSACTION_OBJECT_IDENTITY first=%+v got=%+v", reached[0], event)
		}
	}
}

func privateC18AssertNoLateEvents(t *testing.T, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier) {
	t.Helper()
	// Every producer has been joined before this helper is called. Two exact
	// snapshots therefore prove quiescence without scheduler timing.
	beforeEvents, beforeBarriers := observer.snapshot(), barrier.snapshot()
	if after := observer.snapshot(); !reflect.DeepEqual(after, beforeEvents) {
		t.Fatalf("ASSERT_C18_LATE_C15_EFFECT before=%+v after=%+v", beforeEvents, after)
	}
	if after := barrier.snapshot(); !reflect.DeepEqual(after, beforeBarriers) {
		t.Fatalf("ASSERT_C18_LATE_BARRIER_EFFECT before=%+v after=%+v", beforeBarriers, after)
	}
}

func privateC18FixtureReleaseNotificationAndAssertClosed(t *testing.T, lease RetainedNotificationLease, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier) {
	t.Helper()
	group := privateC18OwnTaskGroup(t, barrier.Abort)
	released := privateC18StartValue(group, privateC18PrepareNotificationRelease(lease))
	release := barrier.await(t, privateC18FixtureRelease)
	barrier.release(privateC18FixtureRelease)
	releaseResult := privateC18Receive(t, released, "NOTIFICATION_RELEASE")
	if releaseResult.Err != nil || !releaseResult.Value || lease.Release() {
		t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
	}
	reached := barrier.snapshot()
	privateC18AssertExactBarrierIdentity(t, reached)
	boundaries := []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureObjectEventCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "publication"}},
		{Boundary: privateC18FixtureRelease, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "release", Terminal: true}},
	}
	privateC18RelevantC15Events(t, observer, reached, boundaries)
	creation := privateC18C15Tuple{SiteID: privateC15SiteC15Copy28RetainedNotificationSet, Kind: "copy"}
	privateC18AssertExactAccounting(t, observer.snapshot(), []privateC18C15Tuple{creation, boundaries[0].Tuple, boundaries[1].Tuple}, []privateC18C15Tuple{creation}, release.ObjectID)
	barrier.assertExactCounts(t, map[privateC18FixtureOwnerBoundary]int{privateC18FixtureObjectEventCommit: 1, privateC18FixtureRetentionCommit: 1, privateC18FixtureReadbackBorrow: 0, privateC18FixtureRelease: 1})
}

func privateC18FixtureReleaseResponseAndAssertClosed(t *testing.T, lease RetainedResponseLease, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier) {
	privateC18FixtureReleaseResponseAndAssertClosedWithBorrowKind(t, lease, observer, barrier, "readback")
}

func privateC18FixtureReleaseResponseAndAssertClosedWithBorrowKind(t *testing.T, lease RetainedResponseLease, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier, borrowKind string) {
	t.Helper()
	group := privateC18OwnTaskGroup(t, barrier.Abort)
	released := privateC18StartValue(group, privateC18PrepareResponseRelease(lease))
	release := barrier.await(t, privateC18FixtureRelease)
	barrier.release(privateC18FixtureRelease)
	releaseResult := privateC18Receive(t, released, "RESPONSE_RELEASE")
	if releaseResult.Err != nil || !releaseResult.Value || lease.Release() {
		t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
	}
	reached := barrier.snapshot()
	privateC18AssertExactBarrierIdentity(t, reached)
	boundaries := []privateC18ExpectedBoundary{
		{Boundary: privateC18FixtureRetentionCommit, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "retain"}},
		{Boundary: privateC18FixtureRelease, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "release", Terminal: true}},
	}
	creation := privateC18C15Tuple{SiteID: privateC15SiteC15Copy30RetainedResponseSet, Kind: "copy"}
	expectedAccounting := []privateC18C15Tuple{creation, boundaries[0].Tuple}
	if borrowKind != "readback" {
		alias := privateC18ExpectedBoundary{Boundary: privateC18FixtureReadbackBorrow, Tuple: privateC18C15Tuple{SiteID: privateC15SiteC15Alias31ResponsesCallback, Kind: borrowKind}}
		boundaries = append(boundaries[:1], alias, boundaries[1])
		expectedAccounting = append(expectedAccounting, alias.Tuple)
	}
	privateC18RelevantC15Events(t, observer, reached, boundaries)
	expectedAccounting = append(expectedAccounting, boundaries[len(boundaries)-1].Tuple)
	privateC18AssertExactAccounting(t, observer.snapshot(), expectedAccounting, []privateC18C15Tuple{creation}, release.ObjectID)
	barrier.assertExactCounts(t, map[privateC18FixtureOwnerBoundary]int{privateC18FixtureObjectEventCommit: 0, privateC18FixtureRetentionCommit: 1, privateC18FixtureReadbackBorrow: 1, privateC18FixtureRelease: 1})
}

func privateC18RunPair8Collision(t *testing.T, order privateC18ArrivalOrder) {
	t.Helper()
	f := newPrivateC18Pair8Fixture(t)
	start := make(chan struct{})
	var startOnce sync.Once
	releaseStart := func() { startOnce.Do(func() { close(start) }) }
	group := privateC18OwnTaskGroup(t, releaseStart, f.barrier.Abort)
	objectEventCall := privateC18PreparePair8ObjectEventCall(t, f.host)
	retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
	result := make(chan privateC18Result, 2)
	objectEvent := func() error {
		if order == privateC18Simultaneous {
			<-start
		}
		operation, err := objectEventCall(context.Background(), privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
		result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
		return err
	}
	retention := func() error {
		if order == privateC18Simultaneous {
			<-start
		}
		operation, err := retentionCall(context.Background())
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
		return err
	}
	first, second := privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit
	var arrived [2]privateC18FixtureOwnerBarrierEvent
	if order == privateC18LaterStageFirst {
		group.Start(retention)
		first, second = second, first
		arrived[0] = f.barrier.await(t, first)
		group.Start(objectEvent)
	} else {
		group.Start(objectEvent)
		if order == privateC18EarlierStageFirst {
			arrived[0] = f.barrier.await(t, first)
			group.Start(retention)
		} else {
			group.Start(retention)
			releaseStart()
			arrived = f.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
			f.barrier.release(privateC18FixtureObjectEventCommit)
			f.barrier.release(privateC18FixtureRetentionCommit)
		}
	}
	if order != privateC18Simultaneous {
		arrived[1] = f.barrier.await(t, second)
		f.barrier.release(first)
		f.barrier.release(second)
	}
	privateC18AssertPhysicalArrival(t, f.barrier, order, arrived, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	privateC18AssertOneWinnerOneTypedLoser(t, results, privateC18FixtureContenderRetention, session.ResourceExhausted)
	if retry := f.host.retryPrivateC18Pair8LoserForTest(context.Background(), privateC18FixtureContenderRetention); retry != session.ResourceExhausted {
		t.Fatalf("ASSERT_C18_PAIR8_LOSER_RETRY_IDENTITY got=%q want=%q", retry, session.ResourceExhausted)
	}
	committed := f.host.privateC18Pair8CoordinatorSnapshotForTest()
	privateC18AssertCommittedDecision(t, committed, 8, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	privateC18FixtureReleaseNotificationAndAssertClosed(t, results[privateC18FixtureContenderObjectEvent].NotificationLease, f.observer, f.barrier)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if retry := f.host.retryPrivateC18Pair8LoserForTest(cancelled, privateC18FixtureContenderRetention); retry != session.ResourceExhausted {
		t.Fatalf("ASSERT_C18_PAIR8_POSTCOMMIT_CANCEL_REVERSED_LOSER got=%q", retry)
	}
}

func TestADR0011C18Pair8CoordinatorRealOwnerCollisionMatrix(t *testing.T) {
	for _, order := range []privateC18ArrivalOrder{privateC18EarlierStageFirst, privateC18LaterStageFirst, privateC18Simultaneous} {
		for repeat := 0; repeat < 3; repeat++ {
			t.Run(fmt.Sprintf("order-%d/repeat-%d", order, repeat), func(t *testing.T) { privateC18RunPair8Collision(t, order) })
		}
	}
}

func privateC18RunPair9Collision(t *testing.T, order privateC18ArrivalOrder) {
	t.Helper()
	f := newPrivateC18Pair9Fixture(t)
	start := make(chan struct{})
	var startOnce sync.Once
	releaseStart := func() { startOnce.Do(func() { close(start) }) }
	group := privateC18OwnTaskGroup(t, releaseStart, f.barrier.Abort)
	retentionCall := privateC18PreparePair9RetentionCall(t, f.host)
	readbackCall := privateC18PreparePair9ReadbackCall(t, f.host)
	result := make(chan privateC18Result, 2)
	retention := func() error {
		if order == privateC18Simultaneous {
			<-start
		}
		operation, err := retentionCall(context.Background())
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
		return err
	}
	readback := func() error {
		if order == privateC18Simultaneous {
			<-start
		}
		called := false
		operation, err := readbackCall(context.Background(), func([]lspwire.Message) error { called = true; return nil })
		if called {
			err = errors.New("ASSERT_C18_PAIR9_LOSER_CALLBACK_EFFECT")
		} else if err == nil {
			err = operation.Err
		}
		result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
		return err
	}
	first, second := privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow
	var arrived [2]privateC18FixtureOwnerBarrierEvent
	if order == privateC18LaterStageFirst {
		group.Start(readback)
		first, second = second, first
		arrived[0] = f.barrier.await(t, first)
		group.Start(retention)
	} else {
		group.Start(retention)
		if order == privateC18EarlierStageFirst {
			arrived[0] = f.barrier.await(t, first)
			group.Start(readback)
		} else {
			group.Start(readback)
			releaseStart()
			arrived = f.barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
			f.barrier.release(privateC18FixtureRetentionCommit)
			f.barrier.release(privateC18FixtureReadbackBorrow)
		}
	}
	if order != privateC18Simultaneous {
		arrived[1] = f.barrier.await(t, second)
		f.barrier.release(first)
		f.barrier.release(second)
	}
	privateC18AssertPhysicalArrival(t, f.barrier, order, arrived, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
	if results[privateC18FixtureContenderRetention].Failure != "" || !errors.Is(results[privateC18FixtureContenderReadback].Err, ErrRetainedResponseLeaseClosed) {
		t.Fatalf("ASSERT_C18_PAIR9_EXACT_LOSER_IDENTITY results=%+v", results)
	}
	if !group.Acknowledge(results[privateC18FixtureContenderReadback].Err) {
		t.Fatal("ASSERT_C18_PAIR9_EXPECTED_READBACK_LOSER_ERROR_NOT_ACKNOWLEDGED")
	}
	if retry := f.host.retryPrivateC18Pair9LoserForTest(context.Background(), privateC18FixtureContenderReadback); !errors.Is(retry, ErrRetainedResponseLeaseClosed) {
		t.Fatalf("ASSERT_C18_PAIR9_LOSER_RETRY_IDENTITY got=%v", retry)
	}
	committed := f.host.privateC18Pair9CoordinatorSnapshotForTest()
	privateC18AssertCommittedDecision(t, committed, 9, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
	privateC18FixtureReleaseResponseAndAssertClosed(t, results[privateC18FixtureContenderRetention].ResponseLease, f.observer, f.barrier)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if retry := f.host.retryPrivateC18Pair9LoserForTest(cancelled, privateC18FixtureContenderReadback); !errors.Is(retry, ErrRetainedResponseLeaseClosed) {
		t.Fatalf("ASSERT_C18_PAIR9_POSTCOMMIT_CANCEL_REVERSED_LOSER got=%v", retry)
	}
}

func TestADR0011C18Pair9CoordinatorRealOwnerCollisionMatrix(t *testing.T) {
	for _, order := range []privateC18ArrivalOrder{privateC18EarlierStageFirst, privateC18LaterStageFirst, privateC18Simultaneous} {
		for repeat := 0; repeat < 3; repeat++ {
			t.Run(fmt.Sprintf("order-%d/repeat-%d", order, repeat), func(t *testing.T) { privateC18RunPair9Collision(t, order) })
		}
	}
}

func privateC18LifecycleContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func privateC18NewSplitPhaseSeam() (privateC18TestSplitPhaseSeam, <-chan any, <-chan any, chan struct{}, chan struct{}) {
	reached := make(chan any, 2)
	decision := make(chan any, 1)
	decisionRelease := make(chan struct{})
	effectRelease := make(chan struct{})
	return privateC18TestSplitPhaseSeam{Reached: reached, Decision: decision, DecisionRelease: decisionRelease, EffectRelease: effectRelease}, reached, decision, decisionRelease, effectRelease
}

func privateC18ReachedEventFieldsError(got reflect.Type, wantPackage string) error {
	if got.Kind() != reflect.Struct || got.NumField() != 2 {
		return fmt.Errorf("ASSERT_C18_REACHED_DYNAMIC_FIELDS type=%v", got)
	}
	contender, boundary := got.Field(0), got.Field(1)
	if contender.Name != "Contender" || contender.Type.Name() != "privateC18Contender" || contender.Type.PkgPath() != wantPackage || contender.Type.Kind() != reflect.String || boundary.Name != "Boundary" || boundary.Type.Name() != "privateC18OwnerBoundary" || boundary.Type.PkgPath() != wantPackage || boundary.Type.Kind() != reflect.Uint8 {
		return fmt.Errorf("ASSERT_C18_REACHED_DYNAMIC_FIELDS type=%v", got)
	}
	return nil
}

func privateC18DecodeReachedEvent(value any) (privateC18TestReachedEvent, error) {
	got := reflect.ValueOf(value)
	wantPackage := reflect.TypeOf(privateC18FixtureContender("")).PkgPath()
	if !got.IsValid() || got.Kind() != reflect.Struct || got.Type().Name() != "privateC18ReachedEvent" || got.Type().PkgPath() != wantPackage {
		return privateC18TestReachedEvent{}, fmt.Errorf("ASSERT_C18_REACHED_DYNAMIC_TYPE type=%T", value)
	}
	if err := privateC18ReachedEventFieldsError(got.Type(), wantPackage); err != nil {
		return privateC18TestReachedEvent{}, err
	}
	return privateC18TestReachedEvent{Contender: privateC18FixtureContender(got.Field(0).String()), Boundary: privateC18FixtureOwnerBoundary(got.Field(1).Uint())}, nil
}

func privateC18AwaitReached(t *testing.T, reached <-chan any, contender privateC18FixtureContender) privateC18TestReachedEvent {
	t.Helper()
	select {
	case value := <-reached:
		got, err := privateC18DecodeReachedEvent(value)
		if err != nil {
			t.Fatal(err)
		}
		if got.Contender != contender {
			t.Fatalf("ASSERT_C18_REACHED_CONTENDER got=%+v want=%q", got, contender)
		}
		return got
	case <-t.Context().Done():
		t.Fatalf("BLOCKED_NOT_RED waiting for contender reached: %v", t.Context().Err())
		return privateC18TestReachedEvent{}
	}
}

func privateC18DecodeDecisionEvent(value any) (privateC18TestDecisionEvent, error) {
	got := reflect.ValueOf(value)
	wantPackage := reflect.TypeOf(privateC18FixtureContender("")).PkgPath()
	if !got.IsValid() || got.Kind() != reflect.Struct || got.Type().Name() != "privateC18DecisionEvent" || got.Type().PkgPath() != wantPackage || got.NumField() != 3 {
		return privateC18TestDecisionEvent{}, fmt.Errorf("ASSERT_C18_DECISION_DYNAMIC_TYPE type=%T", value)
	}
	winner, loser, authority := got.Type().Field(0), got.Type().Field(1), got.Type().Field(2)
	if winner.Name != "Winner" || loser.Name != "Loser" || winner.Type.Name() != "privateC18Contender" || loser.Type != winner.Type || winner.Type.PkgPath() != wantPackage || winner.Type.Kind() != reflect.String || authority.Name != "Authority" || authority.Type.Name() != "privateC18Authority" || authority.Type.PkgPath() != wantPackage || authority.Type.Kind() != reflect.Struct || authority.Type.NumField() != 3 {
		return privateC18TestDecisionEvent{}, fmt.Errorf("ASSERT_C18_DECISION_DYNAMIC_FIELDS type=%v", got.Type())
	}
	generation, transactionID, objectID := authority.Type.Field(0), authority.Type.Field(1), authority.Type.Field(2)
	if generation.Name != "Generation" || generation.Type != reflect.TypeOf(uint64(0)) || transactionID.Name != "TransactionID" || transactionID.Type != reflect.TypeOf("") || objectID.Name != "ObjectID" || objectID.Type != reflect.TypeOf("") {
		return privateC18TestDecisionEvent{}, fmt.Errorf("ASSERT_C18_DECISION_AUTHORITY_FIELDS type=%v", authority.Type)
	}
	authorityValue := got.Field(2)
	return privateC18TestDecisionEvent{Winner: privateC18FixtureContender(got.Field(0).String()), Loser: privateC18FixtureContender(got.Field(1).String()), Authority: privateC18TestAuthority{Generation: authorityValue.Field(0).Uint(), TransactionID: authorityValue.Field(1).String(), ObjectID: authorityValue.Field(2).String()}}, nil
}

func TestADR0011C18NeutralTransportNegativeControls(t *testing.T) {
	if _, err := privateC18DecodeReachedEvent(privateC18TestReachedEvent{}); err == nil || !strings.Contains(err.Error(), "ASSERT_C18_REACHED_DYNAMIC_TYPE") {
		t.Fatalf("ASSERT_C18_WRONG_DYNAMIC_TYPE_ACCEPTED err=%v", err)
	}
	wrongFields := reflect.StructOf([]reflect.StructField{{Name: "Wrong", Type: reflect.TypeOf(privateC18FixtureContender(""))}, {Name: "Boundary", Type: reflect.TypeOf(privateC18FixtureOwnerBoundary(0))}})
	if err := privateC18ReachedEventFieldsError(wrongFields, reflect.TypeOf(privateC18FixtureContender("")).PkgPath()); err == nil || !strings.Contains(err.Error(), "ASSERT_C18_REACHED_DYNAMIC_FIELDS") {
		t.Fatalf("ASSERT_C18_WRONG_DYNAMIC_FIELD_ACCEPTED err=%v", err)
	}
	source := `package p; type privateC18ReachedEvent struct{}; type privateC18SplitPhaseSeam struct { Reached chan<- privateC18ReachedEvent; Decision chan<- any; DecisionRelease <-chan struct{}; EffectRelease <-chan struct{} }`
	file, err := parser.ParseFile(token.NewFileSet(), "negative.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var seam *ast.TypeSpec
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			named, ok := spec.(*ast.TypeSpec)
			if ok && named.Name.Name == "privateC18SplitPhaseSeam" {
				seam = named
			}
		}
	}
	if seam == nil {
		t.Fatal("ASSERT_C18_TYPED_CHANNEL_NEGATIVE_FIXTURE")
	}
	if err := privateC18SplitPhaseDeclarationError(seam); err == nil || !strings.Contains(err.Error(), "ASSERT_C18_SPLIT_PHASE_SEAM_NEUTRAL_CHANNEL") {
		t.Fatalf("ASSERT_C18_TYPED_CHANNEL_SEAM_ACCEPTED err=%v", err)
	}
}

func privateC18ReachedPairMappingError(got [2]privateC18TestReachedEvent, want map[privateC18FixtureContender]privateC18FixtureOwnerBoundary) error {
	if len(want) != 2 {
		return fmt.Errorf("ASSERT_C18_REACHED_MAPPING_EXPECTATION_CARDINALITY want=%+v", want)
	}
	seen := map[privateC18FixtureContender]bool{}
	for _, event := range got {
		boundary, ok := want[event.Contender]
		if !ok || event.Boundary != boundary || seen[event.Contender] {
			return fmt.Errorf("ASSERT_C18_REACHED_EXACT_MAPPING got=%+v want=%+v duplicate=%t", got, want, seen[event.Contender])
		}
		seen[event.Contender] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("ASSERT_C18_REACHED_MAPPING_CARDINALITY got=%+v want=%+v", got, want)
	}
	return nil
}

func privateC18AwaitReachedPairAnyOrder(t *testing.T, reached <-chan any, first, second privateC18FixtureContender) [2]privateC18TestReachedEvent {
	t.Helper()
	want := map[privateC18FixtureContender]privateC18FixtureOwnerBoundary{}
	switch {
	case first == privateC18FixtureContenderObjectEvent && second == privateC18FixtureContenderRetention:
		want[first], want[second] = privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit
	case first == privateC18FixtureContenderRetention && second == privateC18FixtureContenderReadback:
		want[first], want[second] = privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow
	default:
		t.Fatalf("ASSERT_C18_REACHED_MAPPING_PAIR_UNSUPPORTED first=%q second=%q", first, second)
	}
	var got [2]privateC18TestReachedEvent
	for index := range got {
		select {
		case value := <-reached:
			event, err := privateC18DecodeReachedEvent(value)
			if err != nil {
				t.Fatal(err)
			}
			got[index] = event
		case <-t.Context().Done():
			t.Fatalf("BLOCKED_NOT_RED waiting for contender reached pair: %v", t.Context().Err())
		}
	}
	if err := privateC18ReachedPairMappingError(got, want); err != nil {
		t.Fatal(err)
	}
	return got
}

func privateC18AwaitDecision(t *testing.T, decisions <-chan any, winner, loser privateC18FixtureContender) privateC18TestDecisionEvent {
	t.Helper()
	select {
	case value := <-decisions:
		got, err := privateC18DecodeDecisionEvent(value)
		if err != nil {
			t.Fatal(err)
		}
		if got.Winner != winner || got.Loser != loser || got.Authority.Generation == 0 || got.Authority.TransactionID == "" || got.Authority.ObjectID == "" {
			t.Fatalf("ASSERT_C18_EXTERNAL_DECISION got=%+v winner=%q loser=%q", got, winner, loser)
		}
		return got
	case <-t.Context().Done():
		t.Fatalf("BLOCKED_NOT_RED waiting for coordinator decision: %v", t.Context().Err())
		return privateC18TestDecisionEvent{}
	}
}

func privateC18AssertNoExternalMutation[T any](t *testing.T, beforeC15 T, beforeBarrier []privateC18FixtureOwnerBarrierEvent, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier) {
	t.Helper()
	if !reflect.DeepEqual(beforeC15, observer.snapshot()) || !reflect.DeepEqual(beforeBarrier, barrier.snapshot()) {
		t.Fatalf("ASSERT_C18_ZERO_EXTERNAL_MUTATION before_c15=%+v after_c15=%+v before_barrier=%+v after_barrier=%+v", beforeC15, observer.snapshot(), beforeBarrier, barrier.snapshot())
	}
}

func privateC18BarrierEventsEqual(left, right []privateC18FixtureOwnerBarrierEvent) bool {
	return slices.EqualFunc(left, right, func(left, right privateC18FixtureOwnerBarrierEvent) bool {
		return left.TransactionID == right.TransactionID &&
			left.ObjectID == right.ObjectID &&
			left.Sequence == right.Sequence &&
			left.Boundary == right.Boundary
	})
}

func privateC18AssertOnlyBarrierReached[T any](t *testing.T, beforeC15 T, beforeBarrier []privateC18FixtureOwnerBarrierEvent, want privateC18FixtureOwnerBarrierEvent, observer *privateC15AtomicBatchObserver, barrier *privateC18ReachedBarrier) {
	t.Helper()
	afterBarrier := barrier.snapshot()
	if !reflect.DeepEqual(beforeC15, observer.snapshot()) || len(afterBarrier) != len(beforeBarrier)+1 || !privateC18BarrierEventsEqual(afterBarrier[:len(beforeBarrier)], beforeBarrier) || !reflect.DeepEqual(afterBarrier[len(beforeBarrier)], want) {
		t.Fatalf("ASSERT_C18_ONLY_BARRIER_REACHED before_c15=%+v after_c15=%+v before_barrier=%+v after_barrier=%+v want_reached=%+v", beforeC15, observer.snapshot(), beforeBarrier, afterBarrier, want)
	}
}

func TestADR0011C18BarrierEventPrefixEqualityTreatsNilAndEmptyAsEqual(t *testing.T) {
	if !privateC18BarrierEventsEqual(nil, []privateC18FixtureOwnerBarrierEvent{}) {
		t.Fatal("ASSERT_C18_NIL_EMPTY_BARRIER_PREFIX_NOT_EQUAL")
	}
	left := []privateC18FixtureOwnerBarrierEvent{{TransactionID: "tx", ObjectID: "object", Sequence: 1, Boundary: privateC18FixtureRetentionCommit}}
	right := append([]privateC18FixtureOwnerBarrierEvent(nil), left...)
	if !privateC18BarrierEventsEqual(left, right) {
		t.Fatal("ASSERT_C18_NONEMPTY_BARRIER_PREFIX_EXACT_MATCH_REJECTED")
	}
	right[0].Sequence++
	if privateC18BarrierEventsEqual(left, right) {
		t.Fatal("ASSERT_C18_NONEMPTY_BARRIER_PREFIX_MISMATCH_ACCEPTED")
	}
}

func privateC18AwaitResult[T any](t *testing.T, result <-chan T, label string) T {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatalf("ASSERT_C18_%s_OPERATION_DEADLOCK", label)
		var zero T
		return zero
	}
}

func privateC18AwaitNoBarrierResult[T any](t *testing.T, result <-chan T, barrier *privateC18ReachedBarrier, label string) T {
	t.Helper()
	select {
	case got := <-result:
		return got
	case event := <-barrier.reached:
		barrier.release(event.Boundary)
		t.Fatalf("ASSERT_C18_%s_UNEXPECTED_BARRIER_CALLBACK event=%+v", label, event)
		var zero T
		return zero
	case <-time.After(time.Second):
		t.Fatalf("ASSERT_C18_%s_OPERATION_DEADLOCK", label)
		var zero T
		return zero
	}
}

func TestADR0011C18PreDecisionCancellationUsesRealOperations(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair8SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(privateC18LifecycleContext(t))
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, cancel, releaseDecision, releaseEffect, f.barrier.Abort)
		retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
		result := make(chan privateC18Result, 1)
		beforeC15, beforeBarrier := f.observer.snapshot(), f.barrier.snapshot()
		group.Start(func() error {
			operation, err := retentionCall(ctx)
			result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		physicalReached := f.barrier.await(t, privateC18FixtureRetentionCommit)
		if physicalReached.TransactionID == "" || physicalReached.ObjectID == "" || physicalReached.Sequence == 0 {
			t.Fatalf("ASSERT_C18_PREDECISION_PHYSICAL_OWNER_IDENTITY got=%+v", physicalReached)
		}
		f.barrier.release(physicalReached.Boundary)
		coordinatorReached := privateC18AwaitReached(t, reached, privateC18FixtureContenderRetention)
		if coordinatorReached.Boundary != physicalReached.Boundary {
			t.Fatalf("ASSERT_C18_PREDECISION_REACHED_IDENTITY physical=%+v coordinator=%+v", physicalReached, coordinatorReached)
		}
		cancel()
		releaseDecision()
		got := privateC18Receive(t, result, "PAIR8_PREDECISION")
		if got.Failure != session.RequestCancelled || got.NotificationLease.owner != nil {
			t.Fatalf("ASSERT_C18_PREDECISION_TYPED_CANCEL got=%+v", got)
		}
		select {
		case decision := <-decisions:
			t.Fatalf("ASSERT_C18_PREDECISION_DECISION got=%+v", decision)
		default:
		}
		releaseEffect()
		privateC18AssertOnlyBarrierReached(t, beforeC15, beforeBarrier, physicalReached, f.observer, f.barrier)
		cancelledC15 := f.observer.snapshot()
		fresh := newPrivateC18Pair8Fixture(t)
		freshSeam, freshCoordinatorReached, freshDecisions, freshDecisionRelease, freshEffectRelease := privateC18NewSplitPhaseSeam()
		if err := fresh.host.installPrivateC18Pair8SplitPhaseSeamForTest(freshSeam); err != nil {
			t.Fatal(err)
		}
		if fresh.account == f.account || fresh.owner == f.owner || fresh.host.value == f.host.value || fresh.observer == f.observer || fresh.barrier == f.barrier {
			t.Fatal("ASSERT_C18_FRESH_PAIR8_FIXTURE_IDENTITY_REUSED")
		}
		freshGroup := privateC18OwnTaskGroup(t, fresh.barrier.Abort)
		freshObjectEventCall := privateC18PreparePair8ObjectEventCall(t, fresh.host)
		freshRetentionCall := privateC18PreparePair8RetentionCall(t, fresh.host)
		freshResult := make(chan privateC18Result, 2)
		freshCtx := privateC18LifecycleContext(t)
		objectTask := freshGroup.Start(func() error {
			operation, err := freshObjectEventCall(freshCtx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
			freshResult <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		retentionTask := freshGroup.Start(func() error {
			operation, err := freshRetentionCall(freshCtx)
			freshResult <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		freshReached := fresh.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
		freshTransactionID, freshObjectID := freshReached[0].TransactionID, freshReached[0].ObjectID
		for _, event := range freshReached {
			if freshTransactionID == "" || freshObjectID == "" || event.TransactionID != freshTransactionID || event.ObjectID != freshObjectID {
				t.Fatalf("ASSERT_C18_FRESH_PAIR8_PHYSICAL_IDENTITY_SCOPE first=%+v got=%+v", freshReached[0], event)
			}
			fresh.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, freshCoordinatorReached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		close(freshDecisionRelease)
		privateC18AwaitDecision(t, freshDecisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		close(freshEffectRelease)
		results := privateC18ResultsByContender(t, freshResult, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		winner, loser := results[privateC18FixtureContenderObjectEvent], results[privateC18FixtureContenderRetention]
		if winner.Failure != "" || winner.NotificationLease.owner == nil || loser.Failure != session.ResourceExhausted || loser.NotificationLease.owner != nil {
			t.Fatalf("ASSERT_C18_SEPARATE_EXACT_WINNER_LOSER results=%+v", results)
		}
		if err := privateC18AwaitResult(t, objectTask, "PAIR8_PREDECISION_FRESH_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR8_PREDECISION_FRESH_LOSER_TASK"); err != nil && !freshGroup.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_SEPARATE_LOSER_TASK err=%v", err)
		}
		privateC18FixtureReleaseNotificationAndAssertClosed(t, winner.NotificationLease, fresh.observer, fresh.barrier)
		if got := f.observer.snapshot(); !reflect.DeepEqual(got, cancelledC15) {
			t.Fatalf("ASSERT_C18_CANCELLED_PAIR8_OBSERVER_SAW_FRESH_EVENTS before=%+v after=%+v", cancelledC15, got)
		}
	})
	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair9SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(privateC18LifecycleContext(t))
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, cancel, releaseDecision, releaseEffect, f.barrier.Abort)
		readbackCall := privateC18PreparePair9ReadbackCall(t, f.host)
		result := make(chan privateC18Result, 1)
		callbackReports := make(chan error, 1)
		beforeC15, beforeBarrier := f.observer.snapshot(), f.barrier.snapshot()
		group.Start(func() error {
			operation, err := readbackCall(ctx, func([]lspwire.Message) error {
				report := errors.New("ASSERT_C18_CANCELLED_CALLBACK")
				callbackReports <- report
				return report
			})
			if err == nil {
				err = operation.Err
			}
			result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Err: err, ResponseLease: operation.Lease}
			return err
		})
		physicalReached := f.barrier.await(t, privateC18FixtureReadbackBorrow)
		if physicalReached.TransactionID == "" || physicalReached.ObjectID == "" || physicalReached.Sequence == 0 {
			t.Fatalf("ASSERT_C18_PREDECISION_PHYSICAL_OWNER_IDENTITY got=%+v", physicalReached)
		}
		f.barrier.release(physicalReached.Boundary)
		coordinatorReached := privateC18AwaitReached(t, reached, privateC18FixtureContenderReadback)
		if coordinatorReached.Boundary != physicalReached.Boundary {
			t.Fatalf("ASSERT_C18_PREDECISION_REACHED_IDENTITY physical=%+v coordinator=%+v", physicalReached, coordinatorReached)
		}
		cancel()
		releaseDecision()
		got := privateC18Receive(t, result, "PAIR9_PREDECISION")
		if !errors.Is(got.Err, context.Canceled) || got.ResponseLease.owner != nil {
			t.Fatalf("ASSERT_C18_PREDECISION_TYPED_CANCEL got=%+v", got)
		}
		if !group.Acknowledge(got.Err) {
			t.Fatal("ASSERT_C18_EXPECTED_CANCEL_ERROR_NOT_ACKNOWLEDGED")
		}
		select {
		case report := <-callbackReports:
			t.Fatal(report)
		default:
		}
		select {
		case decision := <-decisions:
			t.Fatalf("ASSERT_C18_PREDECISION_DECISION got=%+v", decision)
		default:
		}
		releaseEffect()
		privateC18AssertOnlyBarrierReached(t, beforeC15, beforeBarrier, physicalReached, f.observer, f.barrier)
		cancelledC15 := f.observer.snapshot()
		fresh := newPrivateC18Pair9Fixture(t)
		freshSeam, freshCoordinatorReached, freshDecisions, freshDecisionRelease, freshEffectRelease := privateC18NewSplitPhaseSeam()
		if err := fresh.host.installPrivateC18Pair9SplitPhaseSeamForTest(freshSeam); err != nil {
			t.Fatal(err)
		}
		if fresh.account == f.account || fresh.owner == f.owner || fresh.host.value == f.host.value || fresh.observer == f.observer || fresh.barrier == f.barrier {
			t.Fatal("ASSERT_C18_FRESH_PAIR9_FIXTURE_IDENTITY_REUSED")
		}
		freshGroup := privateC18OwnTaskGroup(t, fresh.barrier.Abort)
		freshRetentionCall := privateC18PreparePair9RetentionCall(t, fresh.host)
		freshReadbackCall := privateC18PreparePair9ReadbackCall(t, fresh.host)
		freshResult := make(chan privateC18Result, 2)
		freshCtx := privateC18LifecycleContext(t)
		retentionTask := freshGroup.Start(func() error {
			operation, err := freshRetentionCall(freshCtx)
			freshResult <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
			return err
		})
		readbackTask := freshGroup.Start(func() error {
			called := false
			operation, err := freshReadbackCall(freshCtx, func([]lspwire.Message) error { called = true; return nil })
			if called {
				err = errors.New("ASSERT_C18_SEPARATE_LOSER_CALLBACK_EFFECT")
			} else if err == nil {
				err = operation.Err
			}
			freshResult <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
			return err
		})
		freshReached := fresh.barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
		freshTransactionID, freshObjectID := freshReached[0].TransactionID, freshReached[0].ObjectID
		for _, event := range freshReached {
			if freshTransactionID == "" || freshObjectID == "" || event.TransactionID != freshTransactionID || event.ObjectID != freshObjectID {
				t.Fatalf("ASSERT_C18_FRESH_PAIR9_PHYSICAL_IDENTITY_SCOPE first=%+v got=%+v", freshReached[0], event)
			}
			fresh.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, freshCoordinatorReached, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		close(freshDecisionRelease)
		privateC18AwaitDecision(t, freshDecisions, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		close(freshEffectRelease)
		results := privateC18ResultsByContender(t, freshResult, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		winner, loser := results[privateC18FixtureContenderRetention], results[privateC18FixtureContenderReadback]
		if winner.Failure != "" || winner.ResponseLease.owner == nil || !errors.Is(loser.Err, ErrRetainedResponseLeaseClosed) || loser.ResponseLease.owner != nil {
			t.Fatalf("ASSERT_C18_SEPARATE_EXACT_WINNER_LOSER results=%+v", results)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR9_PREDECISION_FRESH_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, readbackTask, "PAIR9_PREDECISION_FRESH_LOSER_TASK"); !errors.Is(err, ErrRetainedResponseLeaseClosed) || !freshGroup.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_SEPARATE_LOSER_TASK err=%v", err)
		}
		privateC18FixtureReleaseResponseAndAssertClosed(t, winner.ResponseLease, fresh.observer, fresh.barrier)
		if got := f.observer.snapshot(); !reflect.DeepEqual(got, cancelledC15) {
			t.Fatalf("ASSERT_C18_CANCELLED_PAIR9_OBSERVER_SAW_FRESH_EVENTS before=%+v after=%+v", cancelledC15, got)
		}
	})
}

func TestADR0011C18PostDecisionCancellationUsesExternalDecisionAndEffects(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair8SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(privateC18LifecycleContext(t))
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, cancel, releaseDecision, releaseEffect, f.barrier.Abort)
		objectEventCall := privateC18PreparePair8ObjectEventCall(t, f.host)
		retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
		result := make(chan privateC18Result, 2)
		objectTask := group.Start(func() error {
			operation, err := objectEventCall(ctx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
			result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		retentionTask := group.Start(func() error {
			operation, err := retentionCall(ctx)
			result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		physical := f.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
		for _, event := range physical {
			f.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		releaseDecision()
		privateC18AwaitDecision(t, decisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		cancel()
		releaseEffect()
		results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		winner, loser := results[privateC18FixtureContenderObjectEvent], results[privateC18FixtureContenderRetention]
		if winner.Failure != "" || winner.NotificationLease.owner == nil || loser.Failure != session.ResourceExhausted || loser.NotificationLease.owner != nil {
			t.Fatalf("ASSERT_C18_POSTDECISION_EXACT_WINNER_LOSER results=%+v", results)
		}
		if err := privateC18AwaitResult(t, objectTask, "PAIR8_POSTDECISION_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR8_POSTDECISION_LOSER_TASK"); err != nil {
			t.Fatal(err)
		}
		privateC18FixtureReleaseNotificationAndAssertClosed(t, winner.NotificationLease, f.observer, f.barrier)
	})
	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair9SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(privateC18LifecycleContext(t))
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, cancel, releaseDecision, releaseEffect, f.barrier.Abort)
		retentionCall := privateC18PreparePair9RetentionCall(t, f.host)
		readbackCall := privateC18PreparePair9ReadbackCall(t, f.host)
		result := make(chan privateC18Result, 2)
		retentionTask := group.Start(func() error {
			operation, err := retentionCall(ctx)
			result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
			return err
		})
		readbackTask := group.Start(func() error {
			called := false
			operation, err := readbackCall(ctx, func([]lspwire.Message) error { called = true; return nil })
			if called {
				err = errors.New("ASSERT_C18_POSTDECISION_LOSER_CALLBACK_EFFECT")
			} else if err == nil {
				err = operation.Err
			}
			result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
			return err
		})
		physical := f.barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
		for _, event := range physical {
			f.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		releaseDecision()
		privateC18AwaitDecision(t, decisions, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		cancel()
		releaseEffect()
		results := privateC18ResultsByContender(t, result, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		winner, loser := results[privateC18FixtureContenderRetention], results[privateC18FixtureContenderReadback]
		if winner.Failure != "" || winner.ResponseLease.owner == nil || !errors.Is(loser.Err, ErrRetainedResponseLeaseClosed) || loser.ResponseLease.owner != nil {
			t.Fatalf("ASSERT_C18_POSTDECISION_EXACT_WINNER_LOSER results=%+v", results)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR9_POSTDECISION_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, readbackTask, "PAIR9_POSTDECISION_LOSER_TASK"); !errors.Is(err, ErrRetainedResponseLeaseClosed) || !group.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_POSTDECISION_LOSER_TASK err=%v", err)
		}
		privateC18FixtureReleaseResponseAndAssertClosed(t, winner.ResponseLease, f.observer, f.barrier)
	})
}

func TestADR0011C18ReleasedLeasesRejectWithoutObservation(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair8SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, releaseDecision, releaseEffect, f.barrier.Abort)
		objectEventCall := privateC18PreparePair8ObjectEventCall(t, f.host)
		retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
		results := make(chan privateC18Result, 2)
		ctx := privateC18LifecycleContext(t)
		objectTask := group.Start(func() error {
			operation, err := objectEventCall(ctx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
			results <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		retentionTask := group.Start(func() error {
			operation, err := retentionCall(ctx)
			results <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		physical := f.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit)
		for _, event := range physical {
			f.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		releaseDecision()
		privateC18AwaitDecision(t, decisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		releaseEffect()
		byContender := privateC18ResultsByContender(t, results, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
		winner, loser := byContender[privateC18FixtureContenderObjectEvent], byContender[privateC18FixtureContenderRetention]
		if winner.Failure != "" || winner.NotificationLease.owner == nil || loser.Failure != session.ResourceExhausted || loser.NotificationLease.owner != nil {
			t.Fatalf("ASSERT_C18_RELEASED_PAIR8_EXACT_WINNER_LOSER results=%+v", byContender)
		}
		if err := privateC18AwaitResult(t, objectTask, "PAIR8_RELEASED_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR8_RELEASED_LOSER_TASK"); err != nil {
			t.Fatal(err)
		}
		privateC18FixtureReleaseNotificationAndAssertClosed(t, winner.NotificationLease, f.observer, f.barrier)
		beforeC15, beforeBarrier := f.observer.snapshot(), f.barrier.snapshot()
		closed := make(chan error, 1)
		called := 0
		group.Start(func() error {
			err := winner.NotificationLease.WithNotifications(func([]lspwire.Message) error { called++; return nil })
			closed <- err
			return err
		})
		err := privateC18AwaitNoBarrierResult(t, closed, f.barrier, "CLOSED_NOTIFICATION")
		if !errors.Is(err, ErrRetainedNotificationLeaseClosed) || called != 0 {
			t.Fatalf("ASSERT_C18_CLOSED_NOTIFICATION err=%v called=%d", err, called)
		}
		if !group.Acknowledge(err) {
			t.Fatal("ASSERT_C18_EXPECTED_CLOSED_NOTIFICATION_ERROR_NOT_ACKNOWLEDGED")
		}
		privateC18AssertNoExternalMutation(t, beforeC15, beforeBarrier, f.observer, f.barrier)
	})

	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
		if err := f.host.installPrivateC18Pair9SplitPhaseSeamForTest(seam); err != nil {
			t.Fatal(err)
		}
		var decisionOnce, effectOnce sync.Once
		releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
		releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
		group := privateC18OwnTaskGroup(t, releaseDecision, releaseEffect, f.barrier.Abort)
		retentionCall := privateC18PreparePair9RetentionCall(t, f.host)
		readbackCall := privateC18PreparePair9ReadbackCall(t, f.host)
		results := make(chan privateC18Result, 2)
		ctx := privateC18LifecycleContext(t)
		retentionTask := group.Start(func() error {
			operation, err := retentionCall(ctx)
			results <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
			return err
		})
		readbackTask := group.Start(func() error {
			called := false
			operation, err := readbackCall(ctx, func([]lspwire.Message) error { called = true; return nil })
			if called {
				err = errors.New("ASSERT_C18_RELEASED_PAIR9_LOSER_CALLBACK_EFFECT")
			} else if err == nil {
				err = operation.Err
			}
			results <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
			return err
		})
		physical := f.barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
		for _, event := range physical {
			f.barrier.release(event.Boundary)
		}
		privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		releaseDecision()
		privateC18AwaitDecision(t, decisions, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		releaseEffect()
		byContender := privateC18ResultsByContender(t, results, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
		winner, loser := byContender[privateC18FixtureContenderRetention], byContender[privateC18FixtureContenderReadback]
		if winner.Failure != "" || winner.ResponseLease.owner == nil || !errors.Is(loser.Err, ErrRetainedResponseLeaseClosed) || loser.ResponseLease.owner != nil {
			t.Fatalf("ASSERT_C18_RELEASED_PAIR9_EXACT_WINNER_LOSER results=%+v", byContender)
		}
		if err := privateC18AwaitResult(t, retentionTask, "PAIR9_RELEASED_WINNER_TASK"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, readbackTask, "PAIR9_RELEASED_LOSER_TASK"); !errors.Is(err, ErrRetainedResponseLeaseClosed) || !group.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_RELEASED_PAIR9_LOSER_TASK err=%v", err)
		}
		privateC18FixtureReleaseResponseAndAssertClosed(t, winner.ResponseLease, f.observer, f.barrier)
		beforeC15, beforeBarrier := f.observer.snapshot(), f.barrier.snapshot()
		closed := make(chan error, 1)
		called := 0
		postReleaseReadback := privateC18PrepareLeaseWithResponses(winner.ResponseLease)
		group.Start(func() error {
			err := postReleaseReadback(func([]lspwire.Message) error { called++; return nil })
			closed <- err
			return err
		})
		err := privateC18AwaitNoBarrierResult(t, closed, f.barrier, "CLOSED_RESPONSE")
		if !errors.Is(err, ErrRetainedResponseLeaseClosed) || called != 0 {
			t.Fatalf("ASSERT_C18_CLOSED_RESPONSE err=%v called=%d", err, called)
		}
		if !group.Acknowledge(err) {
			t.Fatal("ASSERT_C18_EXPECTED_CLOSED_RESPONSE_ERROR_NOT_ACKNOWLEDGED")
		}
		privateC18AssertNoExternalMutation(t, beforeC15, beforeBarrier, f.observer, f.barrier)
	})
}

func TestADR0011C18StopRestartUsesActualAuthorityOperations(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		ctx := privateC18LifecycleContext(t)
		const oldPair8Marker = `{"value":{"c18":"old-pair8"}}`
		const freshPair8Marker = `{"value":{"c18":"fresh-pair8"}}`
		type taggedBarrierSnapshot struct {
			generation uint64
			events     []privateC18FixtureOwnerBarrierEvent
		}
		barrierSnapshots := map[string]taggedBarrierSnapshot{}
		recordBarrierSnapshot := func(tag string, generation uint64, barrier *privateC18ReachedBarrier) {
			barrierSnapshots[tag] = taggedBarrierSnapshot{generation: generation, events: barrier.snapshot()}
		}
		assertBarrierSnapshot := func(tag string, wantGeneration uint64, wantEvents int) {
			got, ok := barrierSnapshots[tag]
			if !ok || got.generation != wantGeneration || len(got.events) != wantEvents {
				t.Fatalf("ASSERT_C18_GENERATION_TAGGED_BARRIER_SNAPSHOT tag=%q got=%+v present=%t want_generation=%d want_events=%d", tag, got, ok, wantGeneration, wantEvents)
			}
		}
		runActualOperation := func(operationFixture privateC18Pair8Fixture, tag string, wantGeneration uint64, freshPayloadMarker string) (privateC18TestAuthority, RetainedNotificationLease) {
			t.Helper()
			barrier := newPrivateC18ReachedBarrier()
			installPrivateC18Barrier(t, operationFixture.owner, barrier.observe)
			beforeC15 := operationFixture.observer.snapshot()
			seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
			if err := operationFixture.host.installPrivateC18Pair8SplitPhaseSeamForTest(seam); err != nil {
				t.Fatal(err)
			}
			var decisionOnce, effectOnce sync.Once
			releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
			releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
			group := privateC18OwnTaskGroup(t, releaseDecision, releaseEffect, barrier.Abort)
			objectEventCall := privateC18PreparePair8ObjectEventCall(t, operationFixture.host)
			retentionCall := privateC18PreparePair8RetentionCall(t, operationFixture.host)
			result := make(chan privateC18Result, 2)
			objectTask := group.Start(func() error {
				operation, err := objectEventCall(ctx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
				result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
				return err
			})
			retentionTask := group.Start(func() error {
				operation, err := retentionCall(ctx)
				result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
				return err
			})
			reachedPair := barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit, fmt.Sprintf("PAIR8_%s_CONTENDER_PAIR", tag))
			for _, event := range reachedPair {
				barrier.release(event.Boundary)
			}
			privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
			releaseDecision()
			decision := privateC18AwaitDecision(t, decisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
			if decision.Authority.Generation != wantGeneration {
				t.Fatalf("ASSERT_C18_EXACT_ISSUED_GENERATION got=%+v want=%d", decision, wantGeneration)
			}
			for _, event := range reachedPair {
				if event.TransactionID != decision.Authority.TransactionID || event.ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_DECISION_BARRIER_AUTHORITY_MISMATCH decision=%+v barrier=%+v", decision, event)
				}
			}
			releaseEffect()
			results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
			got, loser := results[privateC18FixtureContenderObjectEvent], results[privateC18FixtureContenderRetention]
			if got.Failure != "" || got.NotificationLease.owner == nil || loser.Failure != session.ResourceExhausted || loser.NotificationLease.owner != nil {
				t.Fatalf("ASSERT_C18_ACTUAL_AUTHORITY_OPERATION results=%+v", results)
			}
			if err := privateC18AwaitResult(t, objectTask, "PAIR8_AUTHORITY_WINNER_TASK"); err != nil {
				t.Fatal(err)
			}
			if err := privateC18AwaitResult(t, retentionTask, "PAIR8_AUTHORITY_LOSER_TASK"); err != nil {
				t.Fatal(err)
			}
			if freshPayloadMarker != "" {
				callbackCount, oldPayloadCount := 0, 0
				var readbackPayload []lspwire.Message
				readback := group.Start(func() error {
					return got.NotificationLease.WithNotifications(func(messages []lspwire.Message) error {
						callbackCount++
						readbackPayload = append([]lspwire.Message(nil), messages...)
						for _, message := range messages {
							if string(message.Params) == oldPair8Marker {
								oldPayloadCount++
							}
						}
						return nil
					})
				})
				readbackEvent := barrier.await(t, privateC18FixtureReadbackBorrow, fmt.Sprintf("PAIR8_%s_FRESH_PAYLOAD_READBACK", tag))
				if readbackEvent.TransactionID != decision.Authority.TransactionID || readbackEvent.ObjectID != decision.Authority.ObjectID || decision.Authority.Generation != wantGeneration {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_AUTHORITY_MISMATCH decision=%+v readback=%+v want_generation=%d", decision, readbackEvent, wantGeneration)
				}
				barrier.release(privateC18FixtureReadbackBorrow)
				expectedPayload := []lspwire.Message{{JSONRPC: lspwire.Version, Method: "$/progress", Params: []byte(freshPayloadMarker)}}
				if err := privateC18AwaitResult(t, readback, "PAIR8_FRESH_READBACK"); err != nil || callbackCount != 1 || len(readbackPayload) != 1 || !reflect.DeepEqual(readbackPayload, expectedPayload) || oldPayloadCount != 0 {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_PAYLOAD err=%v callbacks=%d payload=%+v want=%+v old=%d", err, callbackCount, readbackPayload, expectedPayload, oldPayloadCount)
				}
				readbackC15 := operationFixture.observer.snapshot()[len(beforeC15):]
				if len(readbackC15) == 0 || readbackC15[len(readbackC15)-1].TransactionID != decision.Authority.TransactionID || readbackC15[len(readbackC15)-1].ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_C15_AUTHORITY_MISMATCH decision=%+v events=%+v", decision, readbackC15)
				}
			}
			released := privateC18StartValue(group, privateC18PrepareNotificationRelease(got.NotificationLease))
			release := barrier.await(t, privateC18FixtureRelease, fmt.Sprintf("PAIR8_%s_RELEASE", tag))
			if release.TransactionID != decision.Authority.TransactionID || release.ObjectID != decision.Authority.ObjectID {
				t.Fatalf("ASSERT_C18_DECISION_RELEASE_AUTHORITY_MISMATCH decision=%+v release=%+v", decision, release)
			}
			barrier.release(privateC18FixtureRelease)
			releaseResult := privateC18Receive(t, released, "PAIR8_AUTHORITY_RELEASE")
			if releaseResult.Err != nil || !releaseResult.Value || got.NotificationLease.Release() {
				t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
			}
			for _, event := range operationFixture.observer.snapshot()[len(beforeC15):] {
				if event.TransactionID != decision.Authority.TransactionID || event.ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_DECISION_C15_AUTHORITY_MISMATCH decision=%+v event=%+v", decision, event)
				}
			}
			delta := barrier.snapshot()
			wantBoundaries := 3
			if freshPayloadMarker != "" {
				wantBoundaries++
			}
			if len(delta) != wantBoundaries {
				t.Fatalf("ASSERT_C18_ACTUAL_AUTHORITY_BOUNDARY_CARDINALITY delta=%+v want=%d", delta, wantBoundaries)
			}
			recordBarrierSnapshot(tag, decision.Authority.Generation, barrier)
			return decision.Authority, got.NotificationLease
		}

		f.owner.notifications = []lspwire.Message{{JSONRPC: lspwire.Version, Method: "$/progress", Params: []byte(oldPair8Marker)}}
		initial := f.host.privateC18Pair8CoordinatorSnapshotForTest()
		oldAuthority, oldLease := runActualOperation(f, "initial", initial.Generation, "")
		assertBarrierSnapshot("initial", initial.Generation, 3)
		if failure := f.host.stopPrivateC18Pair8CoordinatorForTest(ctx); failure != "" {
			t.Fatal(failure)
		}
		stopped := f.host.awaitPrivateC18Pair8StoppedForTest(ctx)
		if !stopped.Stopped || stopped.Generation != oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_STOP_NOT_OBSERVED old=%+v stopped=%+v", oldAuthority, stopped)
		}
		beforeC15 := f.observer.snapshot()
		stoppedBarrier := newPrivateC18ReachedBarrier()
		installPrivateC18Barrier(t, f.owner, stoppedBarrier.observe)
		authorityCall := privateC18PreparePair8AuthorityCall(t, f.host)
		stoppedGroup := privateC18OwnTaskGroup(t, stoppedBarrier.Abort)
		stoppedResult := make(chan privateC18Result, 1)
		stoppedCtx := privateC18LifecycleContext(t)
		stoppedTask := stoppedGroup.Start(func() error {
			operation, err := authorityCall(stoppedCtx, privateC18FixtureContenderObjectEvent, oldAuthority)
			stoppedResult <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		stoppedAttempt := privateC18AwaitNoBarrierResult(t, stoppedResult, stoppedBarrier, "PAIR8_STOPPED_AUTHORITY")
		lease, failure := stoppedAttempt.NotificationLease, stoppedAttempt.Failure
		if failure != session.LifecycleConflict || lease.owner != nil {
			t.Fatalf("ASSERT_C18_STOPPED_AUTHORITY_NOT_EXACTLY_REFUSED authority=%+v failure=%q lease=%+v", oldAuthority, failure, lease)
		}
		if err := privateC18AwaitResult(t, stoppedTask, "PAIR8_STOPPED_AUTHORITY_TASK"); err != stoppedAttempt.Err || !stoppedGroup.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_STOPPED_AUTHORITY_TASK err=%v result_err=%v", err, stoppedAttempt.Err)
		}
		privateC18AssertNoExternalMutation(t, beforeC15, nil, f.observer, stoppedBarrier)
		recordBarrierSnapshot("stopped-old-authority", oldAuthority.Generation, stoppedBarrier)
		assertBarrierSnapshot("stopped-old-authority", oldAuthority.Generation, 0)
		restarted := f.host.restartPrivateC18Pair8CoordinatorForTest(ctx)
		if restarted.Stopped || restarted.Generation == oldAuthority.Generation || restarted.Decisions != 0 || len(restarted.ReachedHistory) != 0 || restarted.Winner != "" || restarted.Loser != "" {
			t.Fatalf("ASSERT_C18_RESTART_DROPS_PRIOR_GENERATION_AND_SEAM old=%+v restarted=%+v", oldAuthority, restarted)
		}
		beforeC15 = f.observer.snapshot()
		staleBarrier := newPrivateC18ReachedBarrier()
		installPrivateC18Barrier(t, f.owner, staleBarrier.observe)
		staleGroup := privateC18OwnTaskGroup(t, staleBarrier.Abort)
		staleResult := make(chan privateC18Result, 1)
		staleCtx := privateC18LifecycleContext(t)
		staleTask := staleGroup.Start(func() error {
			operation, err := authorityCall(staleCtx, privateC18FixtureContenderObjectEvent, oldAuthority)
			staleResult <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: operation.Failure, Err: err, NotificationLease: operation.Lease}
			return err
		})
		staleAttempt := privateC18AwaitNoBarrierResult(t, staleResult, staleBarrier, "PAIR8_STALE_AUTHORITY")
		lease, failure = staleAttempt.NotificationLease, staleAttempt.Failure
		if failure != session.LifecycleConflict || lease.owner != nil {
			t.Fatalf("ASSERT_C18_STALE_AUTHORITY_NOT_EXACTLY_REFUSED authority=%+v failure=%q lease=%+v", oldAuthority, failure, lease)
		}
		if err := privateC18AwaitResult(t, staleTask, "PAIR8_STALE_AUTHORITY_TASK"); err != staleAttempt.Err || !staleGroup.Acknowledge(err) {
			t.Fatalf("ASSERT_C18_STALE_AUTHORITY_TASK err=%v result_err=%v", err, staleAttempt.Err)
		}
		privateC18AssertNoExternalMutation(t, beforeC15, nil, f.observer, staleBarrier)
		recordBarrierSnapshot("restarted-old-authority", restarted.Generation, staleBarrier)
		assertBarrierSnapshot("restarted-old-authority", restarted.Generation, 0)
		oldPayloadObserved := 0
		if err := oldLease.WithNotifications(func([]lspwire.Message) error { oldPayloadObserved++; return nil }); !errors.Is(err, ErrRetainedNotificationLeaseClosed) || oldPayloadObserved != 0 || oldLease.Release() {
			t.Fatalf("ASSERT_C18_OLD_OWNER_REOPENED_OR_PAYLOAD_OBSERVED err=%v observed=%d", err, oldPayloadObserved)
		}
		freshFixture := newPrivateC18Pair8Fixture(t)
		freshFixture.owner.notifications = []lspwire.Message{{JSONRPC: lspwire.Version, Method: "$/progress", Params: []byte(freshPair8Marker)}}
		if freshFixture.account == f.account {
			t.Fatal("ASSERT_C18_FRESH_ACCOUNT_REUSED")
		}
		if freshFixture.host.value == f.host.value {
			t.Fatal("ASSERT_C18_FRESH_HOST_REUSED")
		}
		if freshFixture.owner == f.owner || freshFixture.observer == f.observer || freshFixture.barrier == f.barrier {
			t.Fatal("ASSERT_C18_FRESH_OWNER_FIXTURE_SCOPE_REUSED")
		}
		if failure := freshFixture.host.stopPrivateC18Pair8CoordinatorForTest(ctx); failure != "" {
			t.Fatal(failure)
		}
		freshStopped := freshFixture.host.awaitPrivateC18Pair8StoppedForTest(ctx)
		freshRestarted := freshFixture.host.restartPrivateC18Pair8CoordinatorForTest(ctx)
		if !freshStopped.Stopped || freshRestarted.Stopped || freshRestarted.Generation == oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_FRESH_OWNER_RESTART_GENERATION old=%+v stopped=%+v restarted=%+v", oldAuthority, freshStopped, freshRestarted)
		}
		freshAuthority, freshLease := runActualOperation(freshFixture, "fresh", freshRestarted.Generation, freshPair8Marker)
		assertBarrierSnapshot("fresh", freshRestarted.Generation, 4)
		if freshLease.owner != freshFixture.owner || freshLease.owner == oldLease.owner || freshAuthority.Generation == oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_FRESH_AUTHORITY_NOT_DISTINCT old=%+v fresh=%+v old_owner=%p fresh_owner=%p", oldAuthority, freshAuthority, oldLease.owner, freshLease.owner)
		}
	})
	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		ctx := privateC18LifecycleContext(t)
		const oldPair9Marker = `{"c18":"old-pair9"}`
		const freshPair9Marker = `{"c18":"fresh-pair9"}`
		type taggedBarrierSnapshot struct {
			generation uint64
			events     []privateC18FixtureOwnerBarrierEvent
		}
		barrierSnapshots := map[string]taggedBarrierSnapshot{}
		recordBarrierSnapshot := func(tag string, generation uint64, barrier *privateC18ReachedBarrier) {
			barrierSnapshots[tag] = taggedBarrierSnapshot{generation: generation, events: barrier.snapshot()}
		}
		assertBarrierSnapshot := func(tag string, wantGeneration uint64, wantEvents int) {
			got, ok := barrierSnapshots[tag]
			if !ok || got.generation != wantGeneration || len(got.events) != wantEvents {
				t.Fatalf("ASSERT_C18_GENERATION_TAGGED_BARRIER_SNAPSHOT tag=%q got=%+v present=%t want_generation=%d want_events=%d", tag, got, ok, wantGeneration, wantEvents)
			}
		}
		runActualOperation := func(operationFixture privateC18Pair9Fixture, tag string, wantGeneration uint64, freshPayloadMarker string) (privateC18TestAuthority, RetainedResponseLease) {
			t.Helper()
			barrier := newPrivateC18ReachedBarrier()
			installPrivateC18Barrier(t, operationFixture.owner, barrier.observe)
			beforeC15 := operationFixture.observer.snapshot()
			seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
			if err := operationFixture.host.installPrivateC18Pair9SplitPhaseSeamForTest(seam); err != nil {
				t.Fatal(err)
			}
			var decisionOnce, effectOnce sync.Once
			releaseDecision := func() { decisionOnce.Do(func() { close(decisionRelease) }) }
			releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
			group := privateC18OwnTaskGroup(t, releaseDecision, releaseEffect, barrier.Abort)
			retentionCall := privateC18PreparePair9RetentionCall(t, operationFixture.host)
			readbackCall := privateC18PreparePair9ReadbackCall(t, operationFixture.host)
			result := make(chan privateC18Result, 2)
			retentionTask := group.Start(func() error {
				operation, err := retentionCall(ctx)
				result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
				return err
			})
			readbackTask := group.Start(func() error {
				called := false
				operation, err := readbackCall(ctx, func([]lspwire.Message) error { called = true; return nil })
				if called {
					err = errors.New("ASSERT_C18_AUTHORITY_LOSER_CALLBACK_EFFECT")
				} else if err == nil {
					err = operation.Err
				}
				result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
				return err
			})
			reachedPair := barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow, fmt.Sprintf("PAIR9_%s_CONTENDER_PAIR", tag))
			for _, event := range reachedPair {
				barrier.release(event.Boundary)
			}
			privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
			releaseDecision()
			decision := privateC18AwaitDecision(t, decisions, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
			if decision.Authority.Generation != wantGeneration {
				t.Fatalf("ASSERT_C18_EXACT_ISSUED_GENERATION got=%+v want=%d", decision, wantGeneration)
			}
			for _, event := range reachedPair {
				if event.TransactionID != decision.Authority.TransactionID || event.ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_DECISION_BARRIER_AUTHORITY_MISMATCH decision=%+v barrier=%+v", decision, event)
				}
			}
			releaseEffect()
			results := privateC18ResultsByContender(t, result, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
			got, loser := results[privateC18FixtureContenderRetention], results[privateC18FixtureContenderReadback]
			if got.Failure != "" || got.ResponseLease.owner == nil || !errors.Is(loser.Err, ErrRetainedResponseLeaseClosed) || loser.ResponseLease.owner != nil {
				t.Fatalf("ASSERT_C18_ACTUAL_AUTHORITY_OPERATION results=%+v", results)
			}
			if err := privateC18AwaitResult(t, retentionTask, "PAIR9_AUTHORITY_WINNER_TASK"); err != nil {
				t.Fatal(err)
			}
			if err := privateC18AwaitResult(t, readbackTask, "PAIR9_AUTHORITY_LOSER_TASK"); !errors.Is(err, ErrRetainedResponseLeaseClosed) || !group.Acknowledge(err) {
				t.Fatalf("ASSERT_C18_AUTHORITY_LOSER_TASK err=%v", err)
			}
			if freshPayloadMarker != "" {
				callbackCount, oldPayloadCount := 0, 0
				var readbackPayload []lspwire.Message
				readback := group.Start(func() error {
					return got.ResponseLease.WithResponses(func(messages []lspwire.Message) error {
						callbackCount++
						readbackPayload = append([]lspwire.Message(nil), messages...)
						for _, message := range messages {
							if string(message.Result) == oldPair9Marker {
								oldPayloadCount++
							}
						}
						return nil
					})
				})
				readbackEvent := barrier.await(t, privateC18FixtureReadbackBorrow, fmt.Sprintf("PAIR9_%s_FRESH_PAYLOAD_READBACK", tag))
				if readbackEvent.TransactionID != decision.Authority.TransactionID || readbackEvent.ObjectID != decision.Authority.ObjectID || decision.Authority.Generation != wantGeneration {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_AUTHORITY_MISMATCH decision=%+v readback=%+v want_generation=%d", decision, readbackEvent, wantGeneration)
				}
				barrier.release(privateC18FixtureReadbackBorrow)
				expectedPayload := []lspwire.Message{{JSONRPC: lspwire.Version, ID: []byte("109"), Result: []byte(freshPayloadMarker)}}
				if err := privateC18AwaitResult(t, readback, "PAIR9_FRESH_READBACK"); err != nil || callbackCount != 1 || len(readbackPayload) != 1 || !reflect.DeepEqual(readbackPayload, expectedPayload) || oldPayloadCount != 0 {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_PAYLOAD err=%v callbacks=%d payload=%+v want=%+v old=%d", err, callbackCount, readbackPayload, expectedPayload, oldPayloadCount)
				}
				readbackC15 := operationFixture.observer.snapshot()[len(beforeC15):]
				if len(readbackC15) == 0 || readbackC15[len(readbackC15)-1].TransactionID != decision.Authority.TransactionID || readbackC15[len(readbackC15)-1].ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_FRESH_READBACK_C15_AUTHORITY_MISMATCH decision=%+v events=%+v", decision, readbackC15)
				}
			}
			released := privateC18StartValue(group, privateC18PrepareResponseRelease(got.ResponseLease))
			release := barrier.await(t, privateC18FixtureRelease, fmt.Sprintf("PAIR9_%s_RELEASE", tag))
			if release.TransactionID != decision.Authority.TransactionID || release.ObjectID != decision.Authority.ObjectID {
				t.Fatalf("ASSERT_C18_DECISION_RELEASE_AUTHORITY_MISMATCH decision=%+v release=%+v", decision, release)
			}
			barrier.release(privateC18FixtureRelease)
			releaseResult := privateC18Receive(t, released, "PAIR9_AUTHORITY_RELEASE")
			if releaseResult.Err != nil || !releaseResult.Value || got.ResponseLease.Release() {
				t.Fatal("ASSERT_C18_EXACT_RELEASE_AND_ZERO_DUPLICATE_RELEASE")
			}
			for _, event := range operationFixture.observer.snapshot()[len(beforeC15):] {
				if event.TransactionID != decision.Authority.TransactionID || event.ObjectID != decision.Authority.ObjectID {
					t.Fatalf("ASSERT_C18_DECISION_C15_AUTHORITY_MISMATCH decision=%+v event=%+v", decision, event)
				}
			}
			delta := barrier.snapshot()
			wantBoundaries := 3
			if freshPayloadMarker != "" {
				wantBoundaries++
			}
			if len(delta) != wantBoundaries {
				t.Fatalf("ASSERT_C18_ACTUAL_AUTHORITY_BOUNDARY_CARDINALITY delta=%+v want=%d", delta, wantBoundaries)
			}
			recordBarrierSnapshot(tag, decision.Authority.Generation, barrier)
			return decision.Authority, got.ResponseLease
		}

		f.owner.responses = []lspwire.Message{{JSONRPC: lspwire.Version, ID: []byte("108"), Result: []byte(oldPair9Marker)}}
		initial := f.host.privateC18Pair9CoordinatorSnapshotForTest()
		oldAuthority, oldLease := runActualOperation(f, "initial", initial.Generation, "")
		assertBarrierSnapshot("initial", initial.Generation, 3)
		if failure := f.host.stopPrivateC18Pair9CoordinatorForTest(ctx); failure != "" {
			t.Fatal(failure)
		}
		stopped := f.host.awaitPrivateC18Pair9StoppedForTest(ctx)
		if !stopped.Stopped || stopped.Generation != oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_STOP_NOT_OBSERVED old=%+v stopped=%+v", oldAuthority, stopped)
		}
		beforeC15 := f.observer.snapshot()
		stoppedBarrier := newPrivateC18ReachedBarrier()
		installPrivateC18Barrier(t, f.owner, stoppedBarrier.observe)
		authorityCall := privateC18PreparePair9AuthorityCall(t, f.host)
		stoppedGroup := privateC18OwnTaskGroup(t, stoppedBarrier.Abort)
		stoppedResult := make(chan privateC18Result, 1)
		stoppedCtx := privateC18LifecycleContext(t)
		stoppedTask := stoppedGroup.Start(func() error {
			operation, err := authorityCall(stoppedCtx, privateC18FixtureContenderRetention, oldAuthority, nil)
			if err == nil {
				err = operation.Err
			}
			stoppedResult <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Err: err, ResponseLease: operation.Lease}
			return err
		})
		stoppedAttempt := privateC18AwaitNoBarrierResult(t, stoppedResult, stoppedBarrier, "PAIR9_STOPPED_AUTHORITY")
		lease, err := stoppedAttempt.ResponseLease, stoppedAttempt.Err
		if !errors.Is(err, ErrRetainedResponseLeaseClosed) || lease.owner != nil {
			t.Fatalf("ASSERT_C18_STOPPED_AUTHORITY_NOT_EXACTLY_REFUSED authority=%+v err=%v lease=%+v", oldAuthority, err, lease)
		}
		if taskErr := privateC18AwaitResult(t, stoppedTask, "PAIR9_STOPPED_AUTHORITY_TASK"); !errors.Is(taskErr, err) || !stoppedGroup.Acknowledge(taskErr) {
			t.Fatalf("ASSERT_C18_STOPPED_AUTHORITY_TASK err=%v result_err=%v", taskErr, err)
		}
		privateC18AssertNoExternalMutation(t, beforeC15, nil, f.observer, stoppedBarrier)
		recordBarrierSnapshot("stopped-old-authority", oldAuthority.Generation, stoppedBarrier)
		assertBarrierSnapshot("stopped-old-authority", oldAuthority.Generation, 0)
		restarted := f.host.restartPrivateC18Pair9CoordinatorForTest(ctx)
		if restarted.Stopped || restarted.Generation == oldAuthority.Generation || restarted.Decisions != 0 || len(restarted.ReachedHistory) != 0 || restarted.Winner != "" || restarted.Loser != "" {
			t.Fatalf("ASSERT_C18_RESTART_DROPS_PRIOR_GENERATION_AND_SEAM old=%+v restarted=%+v", oldAuthority, restarted)
		}
		beforeC15 = f.observer.snapshot()
		staleBarrier := newPrivateC18ReachedBarrier()
		installPrivateC18Barrier(t, f.owner, staleBarrier.observe)
		staleGroup := privateC18OwnTaskGroup(t, staleBarrier.Abort)
		staleResult := make(chan privateC18Result, 1)
		staleCtx := privateC18LifecycleContext(t)
		staleTask := staleGroup.Start(func() error {
			operation, err := authorityCall(staleCtx, privateC18FixtureContenderRetention, oldAuthority, nil)
			if err == nil {
				err = operation.Err
			}
			staleResult <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Err: err, ResponseLease: operation.Lease}
			return err
		})
		staleAttempt := privateC18AwaitNoBarrierResult(t, staleResult, staleBarrier, "PAIR9_STALE_AUTHORITY")
		lease, err = staleAttempt.ResponseLease, staleAttempt.Err
		if !errors.Is(err, ErrRetainedResponseLeaseClosed) || lease.owner != nil {
			t.Fatalf("ASSERT_C18_STALE_AUTHORITY_NOT_EXACTLY_REFUSED authority=%+v err=%v lease=%+v", oldAuthority, err, lease)
		}
		if taskErr := privateC18AwaitResult(t, staleTask, "PAIR9_STALE_AUTHORITY_TASK"); !errors.Is(taskErr, err) || !staleGroup.Acknowledge(taskErr) {
			t.Fatalf("ASSERT_C18_STALE_AUTHORITY_TASK err=%v result_err=%v", taskErr, err)
		}
		privateC18AssertNoExternalMutation(t, beforeC15, nil, f.observer, staleBarrier)
		recordBarrierSnapshot("restarted-old-authority", restarted.Generation, staleBarrier)
		assertBarrierSnapshot("restarted-old-authority", restarted.Generation, 0)
		oldPayloadObserved := 0
		if err := oldLease.WithResponses(func([]lspwire.Message) error { oldPayloadObserved++; return nil }); !errors.Is(err, ErrRetainedResponseLeaseClosed) || oldPayloadObserved != 0 || oldLease.Release() {
			t.Fatalf("ASSERT_C18_OLD_OWNER_REOPENED_OR_PAYLOAD_OBSERVED err=%v observed=%d", err, oldPayloadObserved)
		}
		freshFixture := newPrivateC18Pair9Fixture(t)
		freshFixture.owner.responses = []lspwire.Message{{JSONRPC: lspwire.Version, ID: []byte("109"), Result: []byte(freshPair9Marker)}}
		if freshFixture.account == f.account {
			t.Fatal("ASSERT_C18_FRESH_ACCOUNT_REUSED")
		}
		if freshFixture.host.value == f.host.value {
			t.Fatal("ASSERT_C18_FRESH_HOST_REUSED")
		}
		if freshFixture.owner == f.owner || freshFixture.observer == f.observer || freshFixture.barrier == f.barrier {
			t.Fatal("ASSERT_C18_FRESH_OWNER_FIXTURE_SCOPE_REUSED")
		}
		if failure := freshFixture.host.stopPrivateC18Pair9CoordinatorForTest(ctx); failure != "" {
			t.Fatal(failure)
		}
		freshStopped := freshFixture.host.awaitPrivateC18Pair9StoppedForTest(ctx)
		freshRestarted := freshFixture.host.restartPrivateC18Pair9CoordinatorForTest(ctx)
		if !freshStopped.Stopped || freshRestarted.Stopped || freshRestarted.Generation == oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_FRESH_OWNER_RESTART_GENERATION old=%+v stopped=%+v restarted=%+v", oldAuthority, freshStopped, freshRestarted)
		}
		freshAuthority, freshLease := runActualOperation(freshFixture, "fresh", freshRestarted.Generation, freshPair9Marker)
		assertBarrierSnapshot("fresh", freshRestarted.Generation, 4)
		if freshLease.owner != freshFixture.owner || freshLease.owner == oldLease.owner || freshAuthority.Generation == oldAuthority.Generation {
			t.Fatalf("ASSERT_C18_FRESH_AUTHORITY_NOT_DISTINCT old=%+v fresh=%+v old_owner=%p fresh_owner=%p", oldAuthority, freshAuthority, oldLease.owner, freshLease.owner)
		}
	})
}

// Runtime reentry is authoritative for the supported lock surfaces. The
// installed owner barrier synchronously reenters coordinator, account, and
// owner lock state before returning; the mapped With operation then consumes
// only its callback effect. The AST oracle above remains a conservative guard.
func TestADR0011C18CoordinatorBarrierCallbacksOutsideAllRelevantLocks(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		ctx := privateC18LifecycleContext(t)
		probe := privateC18PreparePair8LockProbe(t, f.host)
		snapshot := privateC18PrepareSnapshotCall(t, f.host.privateC18ReflectAdapter, "PrivateC18Pair8CoordinatorSnapshotForTest")
		barrierCalls, effects := 0, 0
		installPrivateC18Barrier(t, f.owner, func(privateC18FixtureOwnerBarrierEvent) error {
			barrierCalls++
			if err := snapshot(); err != nil {
				return err
			}
			_ = f.account.snapshot()
			f.owner.mu.Lock()
			f.owner.mu.Unlock()
			return nil
		})
		type result struct {
			failure session.Failure
			err     error
		}
		done := make(chan result, 1)
		go func() {
			var operationErr error
			failure, err := probe(ctx, func() {
				operationErr = (RetainedNotificationLease{owner: f.owner}).WithNotifications(func([]lspwire.Message) error {
					effects++
					return nil
				})
			})
			if err == nil {
				err = operationErr
			}
			done <- result{failure: failure, err: err}
		}()
		select {
		case got := <-done:
			if got.failure != "" || got.err != nil || barrierCalls != 1 || effects != 1 {
				t.Fatalf("ASSERT_C18_PAIR8_REENTRANT_LOCK_PROBE failure=%q err=%v barriers=%d effects=%d", got.failure, got.err, barrierCalls, effects)
			}
		case <-time.After(time.Second):
			t.Fatal("ASSERT_C18_PAIR8_REENTRANT_CALLBACK_DEADLOCK")
		}
	})
	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		ctx := privateC18LifecycleContext(t)
		probe := privateC18PreparePair9LockProbe(t, f.host)
		snapshot := privateC18PrepareSnapshotCall(t, f.host.privateC18ReflectAdapter, "PrivateC18Pair9CoordinatorSnapshotForTest")
		barrierCalls, effects := 0, 0
		installPrivateC18Barrier(t, f.owner, func(privateC18FixtureOwnerBarrierEvent) error {
			barrierCalls++
			if err := snapshot(); err != nil {
				return err
			}
			_ = f.account.snapshot()
			f.owner.mu.Lock()
			f.owner.mu.Unlock()
			return nil
		})
		done := make(chan error, 1)
		go func() {
			var operationErr error
			err := probe(ctx, func() {
				operationErr = (RetainedResponseLease{owner: f.owner}).WithResponses(func([]lspwire.Message) error {
					effects++
					return nil
				})
			})
			if err == nil {
				err = operationErr
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil || barrierCalls != 1 || effects != 1 {
				t.Fatalf("ASSERT_C18_PAIR9_REENTRANT_LOCK_PROBE err=%v barriers=%d effects=%d", err, barrierCalls, effects)
			}
		case <-time.After(time.Second):
			t.Fatal("ASSERT_C18_PAIR9_REENTRANT_CALLBACK_DEADLOCK")
		}
	})
}

func TestADR0011C18CoordinatorBarrierSignatureCarriesNoAuthority(t *testing.T) {
	barrierType := reflect.TypeOf(privateC18FixtureOwnerBarrier(nil))
	if barrierType.NumIn() != 1 || barrierType.In(0) != reflect.TypeOf(privateC18FixtureOwnerBarrierEvent{}) || barrierType.NumOut() != 1 || barrierType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
		t.Fatalf("ASSERT_C18_BARRIER_VOID_OBSERVATION_SIGNATURE type=%v", barrierType)
	}
}

func privateC18FatalFromWorkerError(source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "c18_test.go", source, 0)
	if err != nil {
		return err
	}
	fatal := map[string]bool{"Fatal": true, "Fatalf": true, "FailNow": true}
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}
	testingT := func(expr ast.Expr) bool {
		pointer, ok := expr.(*ast.StarExpr)
		if !ok {
			return false
		}
		selector, ok := pointer.X.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		packageName, packageOK := selector.X.(*ast.Ident)
		return packageOK && packageName.Name == "testing" && selector.Sel.Name == "T"
	}
	unsafeFields := func(fields *ast.FieldList) bool {
		if fields == nil {
			return false
		}
		for _, field := range fields.List {
			if testingT(field.Type) {
				return true
			}
		}
		return false
	}
	var visitBody func(*ast.BlockStmt, map[string]bool) error
	visitFunction := func(fn *ast.FuncDecl, visiting map[string]bool) error { return nil }
	visitFunction = func(fn *ast.FuncDecl, visiting map[string]bool) error {
		if fn == nil || visiting[fn.Name.Name] {
			return nil
		}
		visiting[fn.Name.Name] = true
		defer delete(visiting, fn.Name.Name)
		if unsafeFields(fn.Type.Params) || fn.Recv != nil && unsafeFields(fn.Recv) {
			return fmt.Errorf("ASSERT_C18_TESTING_T_REACHABLE_FROM_WORKER function=%s", fn.Name.Name)
		}
		return visitBody(fn.Body, visiting)
	}
	visitBody = func(body *ast.BlockStmt, visiting map[string]bool) error {
		var found error
		ast.Inspect(body, func(node ast.Node) bool {
			if found != nil {
				return false
			}
			switch value := node.(type) {
			case *ast.FuncLit:
				if unsafeFields(value.Type.Params) {
					found = errors.New("ASSERT_C18_TESTING_T_CALLBACK_PARAMETER")
					return false
				}
			case *ast.SelectorExpr:
				if fatal[value.Sel.Name] {
					found = fmt.Errorf("ASSERT_C18_TEST_FATAL_FROM_WORKER method=%s", value.Sel.Name)
					return false
				}
			case *ast.CallExpr:
				identifier, ok := value.Fun.(*ast.Ident)
				if ok {
					if fn := functions[identifier.Name]; fn != nil {
						found = visitFunction(fn, visiting)
						return found == nil
					}
				}
			}
			return true
		})
		return found
	}
	var roots []*ast.FuncLit
	var rootFunctions []*ast.FuncDecl
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.GoStmt:
			if literal, ok := value.Call.Fun.(*ast.FuncLit); ok {
				roots = append(roots, literal)
			} else if fn := functions[privateC18CallName(value.Call)]; fn != nil {
				rootFunctions = append(rootFunctions, fn)
			}
		case *ast.CallExpr:
			// A testing.T.Run callback remains parent scope, but traversal must
			// continue through it so nested workers become roots.
			if privateC18CallName(value) != "Start" {
				return true
			}
			for _, argument := range value.Args {
				switch root := argument.(type) {
				case *ast.FuncLit:
					roots = append(roots, root)
				case *ast.Ident:
					if fn := functions[root.Name]; fn != nil {
						rootFunctions = append(rootFunctions, fn)
					}
				}
			}
		}
		return true
	})
	for _, root := range roots {
		if unsafeFields(root.Type.Params) {
			return errors.New("ASSERT_C18_TESTING_T_CALLBACK_PARAMETER")
		}
		if err := visitBody(root.Body, map[string]bool{}); err != nil {
			return err
		}
	}
	for _, root := range rootFunctions {
		if err := visitFunction(root, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func TestADR0011C18NoTestingFatalMethodsFromWorkersOrCallbacks(t *testing.T) {
	source, err := os.ReadFile("adr0011_c18_real_owner_collision_red_private_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := privateC18FatalFromWorkerError(source); err != nil {
		t.Fatal(err)
	}
	positive := []byte(`package p; import "testing"; func TestParent(t *testing.T) { t.Run("parent", func(t *testing.T){ t.Fatal("permitted") }) }`)
	if err := privateC18FatalFromWorkerError(positive); err != nil {
		t.Fatalf("ASSERT_C18_PARENT_TEST_SCOPE_REJECTED err=%v", err)
	}
	for label, negative := range map[string][]byte{
		"transitive-fatal":                []byte(`package p; import "testing"; func helper(t *testing.T){ t.Fatal("bad") }; func task(){ helper(nil) }; func f(){ go task() }`),
		"callback-parameter":              []byte(`package p; import "testing"; type group struct{}; func (group) Start(any){}; func f(g group){ g.Start(func(t *testing.T){}) }`),
		"closure-capture":                 []byte(`package p; import "testing"; func f(t *testing.T){ g.Start(func(){ _ = t; t.FailNow() }) }`),
		"closure-fatal":                   []byte(`package p; import "testing"; func f(t *testing.T){ g.Start(func(){ t.FailNow() }) }`),
		"subtest-worker-transitive-fatal": []byte(`package p; import "testing"; type group struct{}; func (group) Start(any){}; func helper(t *testing.T){ t.Fatal("bad") }; func task(){ helper(nil) }; func TestParent(t *testing.T){ t.Run("parent", func(t *testing.T){ var g group; g.Start(func(){ task() }) }) }`),
	} {
		if err := privateC18FatalFromWorkerError(negative); err == nil {
			t.Fatalf("ASSERT_C18_WORKER_STATIC_NEGATIVE_CONTROL label=%s", label)
		}
	}
}

func privateC18CoordinatorForbiddenConstructError(source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "c18_pair_coordinator_private.go", source, 0)
	if err != nil {
		return err
	}
	ownerEffects := map[string]bool{
		"CommitPrivateC18ObjectEventForTest": true, "CommitPrivateC18RetentionForTest": true,
		"BorrowPrivateC18ReadbackForPair9Test": true, "Release": true,
	}
	var found error
	ast.Inspect(file, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		switch value := node.(type) {
		case *ast.GoStmt:
			found = fmt.Errorf("ASSERT_C18_COORDINATOR_SYNTHETIC_GOROUTINE")
		case *ast.CallExpr:
			name := privateC18CallName(value)
			if name == "Sleep" {
				if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "time" {
						found = fmt.Errorf("ASSERT_C18_COORDINATOR_TIME_SLEEP")
					}
				}
			}
			if ownerEffects[name] {
				found = fmt.Errorf("ASSERT_C18_COORDINATOR_OWNER_SEMANTIC_EFFECT call=%s", name)
			}
		}
		return found == nil
	})
	return found
}

func TestADR0011C18CoordinatorCannotManufactureContendersOrEffects(t *testing.T) {
	source, err := os.ReadFile("c18_pair_coordinator_private.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := privateC18CoordinatorForbiddenConstructError(source); err != nil {
		t.Fatal(err)
	}
	for label, negative := range map[string]string{
		"goroutine": `package sessionruntime; func coordinatePrivateC18Pair(){ go func(){}() }`,
		"effect":    `package sessionruntime; func coordinatePrivateC18Pair(){ owner.CommitPrivateC18RetentionForTest() }`,
	} {
		if err := privateC18CoordinatorForbiddenConstructError([]byte(negative)); err == nil {
			t.Fatalf("ASSERT_C18_COORDINATOR_STATIC_NEGATIVE_CONTROL label=%s", label)
		}
	}
}

func privateC18AssertExactSnapshot(t *testing.T, label string, got, want privateC18TestCoordinatorSnapshot) {
	t.Helper()
	if got.Generation == 0 {
		t.Fatalf("ASSERT_C18_%s_ZERO_GENERATION snapshot=%+v", label, got)
	}
	want.Generation = got.Generation
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_C18_%s_EXACT_SNAPSHOT got=%+v want=%+v", label, got, want)
	}
}

func privateC18AssertSnapshotHasNoCompletedEffect(t *testing.T, label string, snapshot privateC18TestCoordinatorSnapshot, pair uint8, reached privateC18FixtureOwnerBoundary) {
	t.Helper()
	privateC18AssertExactSnapshot(t, label, snapshot, privateC18TestCoordinatorSnapshot{
		Pair: pair, Reached: reached, ReachedHistory: []privateC18FixtureOwnerBoundary{reached},
	})
}

func TestADR0011C18BarrierFailurePrecedesEffectAndSnapshotCompletion(t *testing.T) {
	t.Run("pair8", func(t *testing.T) {
		f := newPrivateC18Pair8Fixture(t)
		infrastructure := errors.New("c18 pair8 barrier infrastructure")
		installPrivateC18Barrier(t, f.owner, func(privateC18FixtureOwnerBarrierEvent) error { return infrastructure })
		before := f.observer.snapshot()
		lease, failure := f.host.commitPrivateC18ObjectEventForPair8Test(privateC18LifecycleContext(t), privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
		if failure != session.InitializationFailure || lease.owner != nil {
			t.Fatalf("ASSERT_C18_PAIR8_BARRIER_FAILURE_IDENTITY failure=%q want=%q lease=%+v", failure, session.InitializationFailure, lease)
		}
		if !reflect.DeepEqual(before, f.observer.snapshot()) {
			t.Fatalf("ASSERT_C18_PAIR8_BARRIER_FAILURE_MUTATED_C15 before=%+v after=%+v", before, f.observer.snapshot())
		}
		privateC18AssertSnapshotHasNoCompletedEffect(t, "PAIR8_CALLBACK_ERROR", f.host.privateC18Pair8CoordinatorSnapshotForTest(), 8, privateC18FixtureObjectEventCommit)
	})
	t.Run("pair9", func(t *testing.T) {
		f := newPrivateC18Pair9Fixture(t)
		infrastructure := errors.New("c18 pair9 barrier infrastructure")
		installPrivateC18Barrier(t, f.owner, func(privateC18FixtureOwnerBarrierEvent) error { return infrastructure })
		before := f.observer.snapshot()
		lease, failure := f.host.commitPrivateC18RetentionForPair9Test(privateC18LifecycleContext(t))
		if failure != session.InitializationFailure || lease.owner != nil {
			t.Fatalf("ASSERT_C18_PAIR9_BARRIER_FAILURE_IDENTITY failure=%q want=%q lease=%+v", failure, session.InitializationFailure, lease)
		}
		if !reflect.DeepEqual(before, f.observer.snapshot()) {
			t.Fatalf("ASSERT_C18_PAIR9_BARRIER_FAILURE_MUTATED_C15 before=%+v after=%+v", before, f.observer.snapshot())
		}
		privateC18AssertSnapshotHasNoCompletedEffect(t, "PAIR9_CALLBACK_ERROR", f.host.privateC18Pair9CoordinatorSnapshotForTest(), 9, privateC18FixtureRetentionCommit)
	})
}

func TestADR0011C18BarrierPanicIsTypedBeforeEffect(t *testing.T) {
	f := newPrivateC18Pair9Fixture(t)
	installPrivateC18Barrier(t, f.owner, func(privateC18FixtureOwnerBarrierEvent) error { panic("c18 barrier panic") })
	before := f.observer.snapshot()
	call := privateC18PreparePair9RetentionCall(t, f.host)
	operation, err := call(privateC18LifecycleContext(t))
	if operation.Failure != session.SessionCrashed || err != nil || operation.Err != nil {
		t.Fatalf("ASSERT_C18_BARRIER_PANIC_IDENTITY operation=%+v err=%v want=%q", operation, err, session.SessionCrashed)
	}
	if operation.Lease.owner != nil || !reflect.DeepEqual(before, f.observer.snapshot()) {
		t.Fatalf("ASSERT_C18_BARRIER_PANIC_MUTATED_EFFECT operation=%+v", operation)
	}
	privateC18AssertSnapshotHasNoCompletedEffect(t, "PAIR9_CALLBACK_PANIC", f.host.privateC18Pair9CoordinatorSnapshotForTest(), 9, privateC18FixtureRetentionCommit)
}

func TestADR0011C18ReleaseBarrierFailurePreservesCustodyForExactRetry(t *testing.T) {
	f := newPrivateC18Pair9Fixture(t)
	group := privateC18OwnTaskGroup(t, f.barrier.Abort)
	retentionCall := privateC18PreparePair9RetentionCall(t, f.host)
	readbackCall := privateC18PreparePair9ReadbackCall(t, f.host)
	ctx := privateC18LifecycleContext(t)
	created := make(chan privateC18Result, 2)
	retentionTask := group.Start(func() error {
		operation, err := retentionCall(ctx)
		created <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: operation.Failure, Err: err, ResponseLease: operation.Lease}
		return err
	})
	readbackTask := group.Start(func() error {
		called := false
		operation, err := readbackCall(ctx, func([]lspwire.Message) error { called = true; return nil })
		if called {
			err = errors.New("ASSERT_C18_RELEASE_RETRY_READBACK_LOSER_CALLBACK_EFFECT")
		} else if err == nil {
			err = operation.Err
		}
		created <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err, ResponseLease: operation.Lease}
		return err
	})
	arrived := f.barrier.awaitPairAnyOrder(t, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
	for _, event := range arrived {
		f.barrier.release(event.Boundary)
	}
	privateC18AssertPhysicalArrival(t, f.barrier, privateC18Simultaneous, arrived, privateC18FixtureRetentionCommit, privateC18FixtureReadbackBorrow)
	results := privateC18ResultsByContender(t, created, privateC18FixtureContenderRetention, privateC18FixtureContenderReadback)
	creation := results[privateC18FixtureContenderRetention]
	loser := results[privateC18FixtureContenderReadback]
	if creation.Failure != "" || creation.Err != nil || creation.ResponseLease.owner == nil || !errors.Is(loser.Err, ErrRetainedResponseLeaseClosed) || loser.ResponseLease.owner != nil {
		t.Fatalf("ASSERT_C18_RELEASE_RETRY_EXACT_POLICY results=%+v", results)
	}
	if err := privateC18AwaitResult(t, retentionTask, "RELEASE_RETRY_RETENTION_TASK"); err != nil {
		t.Fatalf("ASSERT_C18_RELEASE_RETRY_RETENTION_TASK err=%v", err)
	}
	if err := privateC18AwaitResult(t, readbackTask, "RELEASE_RETRY_READBACK_TASK"); !errors.Is(err, ErrRetainedResponseLeaseClosed) || !group.Acknowledge(err) {
		t.Fatalf("ASSERT_C18_RELEASE_RETRY_READBACK_TASK err=%v", err)
	}
	before := f.observer.snapshot()
	infrastructure := errors.New("c18 release barrier infrastructure")
	var failedReleaseCalls int
	restoreFailure := installPrivateC18Barrier(t, f.owner, func(event privateC18FixtureOwnerBarrierEvent) error {
		if event.Boundary == privateC18FixtureRelease {
			failedReleaseCalls++
			return infrastructure
		}
		return nil
	})
	if creation.ResponseLease.Release() {
		t.Fatal("ASSERT_C18_RELEASE_BARRIER_FAILURE_CLOSED_CUSTODY")
	}
	if failedReleaseCalls != 1 || !reflect.DeepEqual(before, f.observer.snapshot()) {
		t.Fatalf("ASSERT_C18_RELEASE_BARRIER_FAILURE_MUTATED calls=%d before=%+v after=%+v", failedReleaseCalls, before, f.observer.snapshot())
	}
	restoreFailure()
	var successfulReleaseCalls int
	installPrivateC18Barrier(t, f.owner, func(event privateC18FixtureOwnerBarrierEvent) error {
		if event.Boundary == privateC18FixtureRelease {
			successfulReleaseCalls++
		}
		return nil
	})
	if !creation.ResponseLease.Release() || creation.ResponseLease.Release() || successfulReleaseCalls != 1 {
		t.Fatalf("ASSERT_C18_RELEASE_RETRY_NOT_EXACTLY_ONCE successful_calls=%d", successfulReleaseCalls)
	}
}

func TestADR0011C18OwnerBarrierInstallInvokeRestoreRace(t *testing.T) {
	run := func(t *testing.T, owner any, group *privateC18TaskGroup, invoke func() error, wantBoundary privateC18FixtureOwnerBoundary) {
		t.Helper()
		var mu sync.Mutex
		oldEvents := make([]privateC18FixtureOwnerBarrierEvent, 0, 2)
		newEvents := make([]privateC18FixtureOwnerBarrierEvent, 0, 2)
		record := func(events *[]privateC18FixtureOwnerBarrierEvent) privateC18FixtureOwnerBarrier {
			return func(event privateC18FixtureOwnerBarrierEvent) error {
				mu.Lock()
				*events = append(*events, event)
				mu.Unlock()
				return nil
			}
		}
		oldRestore := installPrivateC18Barrier(t, owner, record(&oldEvents))
		newRestore := installPrivateC18Barrier(t, owner, record(&newEvents))
		start := make(chan struct{})
		invoked := group.Start(func() error {
			<-start
			return invoke()
		})
		restored := group.Start(func() error {
			<-start
			oldRestore()
			return nil
		})
		close(start)
		if err := privateC18AwaitResult(t, invoked, "BARRIER_RACE_INVOKE"); err != nil {
			t.Fatal(err)
		}
		if err := privateC18AwaitResult(t, restored, "BARRIER_RACE_STALE_RESTORE"); err != nil {
			t.Fatal(err)
		}
		if err := invoke(); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		gotOld := append([]privateC18FixtureOwnerBarrierEvent(nil), oldEvents...)
		gotNew := append([]privateC18FixtureOwnerBarrierEvent(nil), newEvents...)
		mu.Unlock()
		if len(gotOld) != 0 || len(gotNew) != 2 {
			t.Fatalf("ASSERT_C18_BARRIER_RACE_CURRENT_RESTORE_LIFO old=%+v new=%+v", gotOld, gotNew)
		}
		for i, event := range gotNew {
			if event.TransactionID == "" || event.ObjectID == "" || event.Sequence == 0 || event.Boundary != wantBoundary {
				t.Fatalf("ASSERT_C18_BARRIER_RACE_EXACT_EVENT index=%d got=%+v want_boundary=%d", i, event, wantBoundary)
			}
		}
		newRestore()
	}

	t.Run("pair8", func(t *testing.T) {
		owner, failure := newPrivateB4RetainedNotificationOwner(mustPrivateB4ByteAccountV2(t), notificationFixture())
		if failure != "" || owner == nil {
			t.Fatalf("BLOCKED_NOT_RED pair8 owner: failure=%q owner=%v", failure, owner)
		}
		f := privateC18Pair8Fixture{owner: owner, barrier: privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())}
		group := privateC18OwnTaskGroup(t, f.barrier.Abort)
		lease := RetainedNotificationLease{owner: f.owner}
		run(t, f.owner, group, func() error {
			return lease.WithNotifications(func([]lspwire.Message) error { return nil })
		}, privateC18FixtureReadbackBorrow)
	})

	t.Run("pair9", func(t *testing.T) {
		owner, failure := newPrivateB4RetainedResponseOwner(mustPrivateB4ByteAccountV2(t), retainedResponseFixture()[:1])
		if failure != "" || owner == nil {
			t.Fatalf("BLOCKED_NOT_RED pair9 owner: failure=%q owner=%v", failure, owner)
		}
		f := privateC18Pair9Fixture{owner: owner, barrier: privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())}
		group := privateC18OwnTaskGroup(t, f.barrier.Abort)
		lease := RetainedResponseLease{owner: f.owner}
		run(t, f.owner, group, func() error {
			return lease.WithResponses(func([]lspwire.Message) error { return nil })
		}, privateC18FixtureReadbackBorrow)
	})
}

func TestADR0011C18OwnerBarrierRestoreUsesGenerationToken(t *testing.T) {
	owner, failure := newPrivateB4RetainedNotificationOwner(mustPrivateB4ByteAccountV2(t), notificationFixture())
	if failure != "" || owner == nil {
		t.Fatalf("BLOCKED_NOT_RED retained notification owner: failure=%q owner=%v", failure, owner)
	}
	lease := RetainedNotificationLease{owner: owner}
	var mu sync.Mutex
	events := map[string][]privateC18FixtureOwnerBarrierEvent{}
	callback := func(label string) privateC18FixtureOwnerBarrier {
		return func(event privateC18FixtureOwnerBarrierEvent) error {
			mu.Lock()
			events[label] = append(events[label], event)
			mu.Unlock()
			return nil
		}
	}
	restoreOld := installPrivateC18Barrier(t, owner, callback("old"))
	restoreNew := installPrivateC18Barrier(t, owner, callback("new"))
	restoreOld() // stale restore must not replace the newer installation.

	invoke := func(label string) {
		t.Helper()
		if err := lease.WithNotifications(func([]lspwire.Message) error { return nil }); err != nil {
			t.Fatalf("ASSERT_C18_BARRIER_GENERATION_%s_WITH_NOTIFICATIONS err=%v", label, err)
		}
	}
	invoke("NEW_FIRST")
	invoke("NEW_SECOND")

	mu.Lock()
	oldEvents := append([]privateC18FixtureOwnerBarrierEvent(nil), events["old"]...)
	newEvents := append([]privateC18FixtureOwnerBarrierEvent(nil), events["new"]...)
	mu.Unlock()
	if len(oldEvents) != 0 || len(newEvents) != 2 {
		t.Fatalf("ASSERT_C18_OLD_BARRIER_RESTORE_OVERWROTE_NEW old=%+v new=%+v", oldEvents, newEvents)
	}
	for i, event := range newEvents {
		if event.TransactionID == "" || event.ObjectID == "" || event.Sequence == 0 || event.Boundary != privateC18FixtureReadbackBorrow {
			t.Fatalf("ASSERT_C18_BARRIER_GENERATION_EXACT_EVENT index=%d got=%+v want_boundary=%d", i, event, privateC18FixtureReadbackBorrow)
		}
		if i > 0 && (event.TransactionID != newEvents[0].TransactionID || event.ObjectID != newEvents[0].ObjectID || event.Sequence != newEvents[0].Sequence || event.Boundary != privateC18FixtureReadbackBorrow) {
			t.Fatalf("ASSERT_C18_BARRIER_GENERATION_STABLE_CUSTODY_TOKEN index=%d first=%+v got=%+v want_boundary=%d", i, newEvents[0], event, privateC18FixtureReadbackBorrow)
		}
	}

	restoreNew() // current restore must reveal the prior nil callback, not stale old.
	invoke("RESTORED_NIL")
	mu.Lock()
	oldEvents = append([]privateC18FixtureOwnerBarrierEvent(nil), events["old"]...)
	newEvents = append([]privateC18FixtureOwnerBarrierEvent(nil), events["new"]...)
	mu.Unlock()
	if len(oldEvents) != 0 || len(newEvents) != 2 {
		t.Fatalf("ASSERT_C18_CURRENT_BARRIER_RESTORE_NOT_PRIOR_NIL old=%+v new=%+v", oldEvents, newEvents)
	}
}

func TestADR0011C18SplitSeamRestoreGenerationSurvivesRestartAndStop(t *testing.T) {
	f := newPrivateC18Pair8Fixture(t)
	oldSeam, oldReached, oldDecisions, _, _ := privateC18NewSplitPhaseSeam()
	preRestartSeam, preRestartReached, preRestartDecisions, _, _ := privateC18NewSplitPhaseSeam()
	restoreOld, err := f.host.installPrivateC18Pair8SplitPhaseSeamRestoreForTest(oldSeam)
	if err != nil {
		t.Fatal(err)
	}
	restorePreRestart, err := f.host.installPrivateC18Pair8SplitPhaseSeamRestoreForTest(preRestartSeam)
	if err != nil {
		t.Fatal(err)
	}
	restoreOld() // stale restore must not remove the newer pre-restart seam.
	before := f.host.privateC18Pair8CoordinatorSnapshotForTest()
	ctx := privateC18LifecycleContext(t)
	if failure := f.host.stopPrivateC18Pair8CoordinatorForTest(ctx); failure != "" {
		t.Fatalf("ASSERT_C18_SEAM_STOP_FAILURE got=%q", failure)
	}
	stopped := f.host.awaitPrivateC18Pair8StoppedForTest(ctx)
	if !stopped.Stopped {
		t.Fatalf("ASSERT_C18_SEAM_STOP_NOT_TERMINAL snapshot=%+v", stopped)
	}
	restarted := f.host.restartPrivateC18Pair8CoordinatorForTest(ctx)
	if restarted.Generation == before.Generation || restarted.Stopped {
		t.Fatalf("ASSERT_C18_SEAM_RESTART_GENERATION old=%+v restarted=%+v", before, restarted)
	}

	freshSeam, freshReached, freshDecisions, freshDecisionRelease, freshEffectRelease := privateC18NewSplitPhaseSeam()
	restoreFresh, err := f.host.installPrivateC18Pair8SplitPhaseSeamRestoreForTest(freshSeam)
	if err != nil {
		t.Fatal(err)
	}
	defer restoreFresh()
	restorePreRestart() // stale across restart: must not remove fresh generation.
	restoreOld()
	group := privateC18OwnTaskGroup(t, f.barrier.Abort)
	objectCall := privateC18PreparePair8ObjectEventCall(t, f.host)
	retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
	result := make(chan privateC18Result, 2)
	objectTask := group.Start(func() error {
		op, err := objectCall(ctx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
		result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: op.Failure, Err: err, NotificationLease: op.Lease}
		return err
	})
	retentionTask := group.Start(func() error {
		op, err := retentionCall(ctx)
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: op.Failure, Err: err, NotificationLease: op.Lease}
		return err
	})
	for _, event := range f.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit) {
		f.barrier.release(event.Boundary)
	}
	privateC18AwaitReachedPairAnyOrder(t, freshReached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	close(freshDecisionRelease)
	decision := privateC18AwaitDecision(t, freshDecisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	if decision.Authority.Generation != restarted.Generation {
		t.Fatalf("ASSERT_C18_FRESH_DECISION_GENERATION got=%+v restarted=%+v", decision, restarted)
	}
	close(freshEffectRelease)
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	got, loser := results[privateC18FixtureContenderObjectEvent], results[privateC18FixtureContenderRetention]
	if got.Failure != "" || got.Err != nil || got.NotificationLease.owner == nil || loser.Failure != session.ResourceExhausted || loser.NotificationLease.owner != nil {
		t.Fatalf("ASSERT_C18_FRESH_GENERATION_RESULT results=%+v", results)
	}
	if err := privateC18AwaitResult(t, objectTask, "FRESH_GENERATION_WINNER_TASK"); err != nil {
		t.Fatalf("ASSERT_C18_FRESH_GENERATION_WINNER_TASK err=%v", err)
	}
	if err := privateC18AwaitResult(t, retentionTask, "FRESH_GENERATION_LOSER_TASK"); err != nil {
		t.Fatalf("ASSERT_C18_FRESH_GENERATION_LOSER_TASK err=%v", err)
	}
	for label, channel := range map[string]<-chan any{
		"old-reached": oldReached, "old-decision": oldDecisions,
		"pre-restart-reached": preRestartReached, "pre-restart-decision": preRestartDecisions,
	} {
		select {
		case value := <-channel:
			t.Fatalf("ASSERT_C18_STALE_SEAM_CHANNEL_NOT_SILENT label=%s value=%v", label, value)
		default:
		}
	}
	privateC18FixtureReleaseNotificationAndAssertClosed(t, got.NotificationLease, f.observer, f.barrier)
}

func TestADR0011C18PostDecisionStopAbortsBlockedEffectGate(t *testing.T) {
	f := newPrivateC18Pair8Fixture(t)
	baseline := f.observer.snapshot()
	seam, reached, decisions, decisionRelease, effectRelease := privateC18NewSplitPhaseSeam()
	if err := f.host.installPrivateC18Pair8SplitPhaseSeamForTest(seam); err != nil {
		t.Fatal(err)
	}
	ctx := privateC18LifecycleContext(t)
	var effectOnce sync.Once
	releaseEffect := func() { effectOnce.Do(func() { close(effectRelease) }) }
	group := privateC18OwnTaskGroup(t, releaseEffect, f.barrier.Abort)
	result := make(chan privateC18Result, 2)
	objectCall := privateC18PreparePair8ObjectEventCall(t, f.host)
	retentionCall := privateC18PreparePair8RetentionCall(t, f.host)
	objectTask := group.Start(func() error {
		op, err := objectCall(ctx, privateB4EventIdentityC17{Family: "TARGET", Kind: "TARGET", Ordinal: 0})
		result <- privateC18Result{Contender: privateC18FixtureContenderObjectEvent, Stage: privateC18FixtureObjectEventCommit, Failure: op.Failure, Err: err, NotificationLease: op.Lease}
		return err
	})
	retentionTask := group.Start(func() error {
		op, err := retentionCall(ctx)
		result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: op.Failure, Err: err, NotificationLease: op.Lease}
		return err
	})
	for _, event := range f.barrier.awaitPairAnyOrder(t, privateC18FixtureObjectEventCommit, privateC18FixtureRetentionCommit) {
		f.barrier.release(event.Boundary)
	}
	privateC18AwaitReachedPairAnyOrder(t, reached, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	select {
	case <-ctx.Done():
		t.Fatalf("ASSERT_C18_STOP_CAUSE_CONTEXT_NOT_LIVE_BEFORE_TRIGGER err=%v", ctx.Err())
	default:
	}
	close(decisionRelease)
	privateC18AwaitDecision(t, decisions, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	committed := f.host.privateC18Pair8CoordinatorSnapshotForTest()
	if committed.Winner != privateC18FixtureContenderObjectEvent || committed.Decisions != 1 || committed.AuthorizedTransitions != 1 || !committed.ProvisionalAuthority || committed.TerminalDispositions != 0 || committed.Stopped {
		t.Fatalf("ASSERT_C18_STOP_PRECONDITION_EXACT_AUTHORIZED_BLOCKED_SNAPSHOT snapshot=%+v", committed)
	}
	if len(result) > 1 {
		t.Fatalf("ASSERT_C18_WINNER_RESULT_BEFORE_STOP buffered_results=%d", len(result))
	}
	if got := f.observer.snapshot(); !reflect.DeepEqual(got, baseline) {
		t.Fatalf("ASSERT_C18_EFFECT_BEFORE_STOP baseline=%+v events=%+v", baseline, got)
	}
	stop := make(chan session.Failure, 1)
	stopCtx := privateC18LifecycleContext(t)
	group.Start(func() error {
		failure := f.host.stopPrivateC18Pair8CoordinatorForTest(stopCtx)
		stop <- failure
		return nil
	})
	if failure := privateC18AwaitResult(t, stop, "POSTDECISION_STOP"); failure != "" {
		t.Fatalf("ASSERT_C18_STOP_TRIGGER_FAILURE got=%q want_success", failure)
	}
	select {
	case <-ctx.Done():
		t.Fatalf("ASSERT_C18_STOP_CAUSE_CONTEXT_NOT_LIVE_AFTER_TRIGGER err=%v", ctx.Err())
	default:
	}
	results := privateC18ResultsByContender(t, result, privateC18FixtureContenderObjectEvent, privateC18FixtureContenderRetention)
	winner, loser := results[privateC18FixtureContenderObjectEvent], results[privateC18FixtureContenderRetention]
	if winner.NotificationLease.owner != nil || winner.Failure != session.SessionStopping || winner.Err != nil || loser.NotificationLease.owner != nil || loser.Failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C18_STOP_BLOCKED_EFFECT_EXACT_RESULTS results=%+v", results)
	}
	if err := privateC18AwaitResult(t, objectTask, "POSTDECISION_STOP_WINNER_TASK"); err != nil {
		t.Fatalf("ASSERT_C18_STOP_WINNER_TASK err=%v", err)
	}
	if err := privateC18AwaitResult(t, retentionTask, "POSTDECISION_STOP_LOSER_TASK"); err != nil {
		t.Fatalf("ASSERT_C18_STOP_LOSER_TASK err=%v", err)
	}
	stopped := f.host.awaitPrivateC18Pair8StoppedForTest(privateC18LifecycleContext(t))
	privateC18AssertExactSnapshot(t, "STOP_EXACT_TERMINAL", stopped, privateC18TestCoordinatorSnapshot{
		Pair: 8, Reached: privateC18FixtureObjectEventCommit,
		Winner: privateC18FixtureContenderObjectEvent, Loser: privateC18FixtureContenderRetention,
		Decisions: 1, AuthorizedTransitions: 1, TerminalDispositions: 1, Stopped: true,
		ReachedHistory: []privateC18FixtureOwnerBoundary{privateC18FixtureObjectEventCommit},
	})
}

func TestADR0011C18RestartPayloadIsolationUsesFreshOwnerAcquisition(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	acquire := func(label, marker string) (*privateB4RetainedResponseOwner, privateC18TestPair9CoordinatorAdapter, RetainedResponseLease, *privateC18ReachedBarrier, *privateC15AtomicBatchObserver, privateC18FixtureOwnerBarrierEvent) {
		observer := &privateC15AtomicBatchObserver{}
		privateC15InstallAtomicBatchObserver(t, account, observer)
		owner, failure := newPrivateB4RetainedResponseOwner(account, []lspwire.Message{{JSONRPC: lspwire.Version, ID: []byte(`"` + label + `"`), Result: []byte(marker)}})
		if failure != "" || owner == nil {
			t.Fatalf("ASSERT_C18_%s_OWNER_CONSTRUCTION failure=%q owner=%v", label, failure, owner)
		}
		host := privateC18TestPair9CoordinatorAdapter{newPrivateC18ReflectAdapter(t, label, owner)}
		host.require(t, map[string][2]int{
			"CommitPrivateC18RetentionForPair9Test": {1, 2}, "PrivateC18Pair9CoordinatorSnapshotForTest": {0, 1},
			"StopPrivateC18Pair9CoordinatorForTest": {1, 1}, "AwaitPrivateC18Pair9StoppedForTest": {1, 1},
			"RestartPrivateC18Pair9CoordinatorForTest": {1, 1},
		})
		barrier := privateC18OwnBarrier(t, newPrivateC18ReachedBarrier())
		installPrivateC18Barrier(t, owner, barrier.observe)
		call := privateC18PreparePair9RetentionCall(t, host)
		ctx := privateC18LifecycleContext(t)
		result := make(chan privateC18Result, 1)
		group := privateC18OwnTaskGroup(t, barrier.Abort)
		task := group.Start(func() error {
			op, err := call(ctx)
			result <- privateC18Result{Contender: privateC18FixtureContenderRetention, Stage: privateC18FixtureRetentionCommit, Failure: op.Failure, Err: err, ResponseLease: op.Lease}
			return err
		})
		retention := barrier.await(t, privateC18FixtureRetentionCommit, label+"_RETENTION_COMMIT")
		barrier.release(privateC18FixtureRetentionCommit)
		got := privateC18AwaitResult(t, result, label+"_ACQUIRE")
		if err := privateC18AwaitResult(t, task, label+"_TASK"); err != nil || got.Contender != privateC18FixtureContenderRetention || got.Stage != privateC18FixtureRetentionCommit || got.Failure != "" || got.Err != nil || got.ResponseLease.owner == nil {
			t.Fatalf("ASSERT_C18_%s_ACQUISITION task_err=%v result=%+v", label, err, got)
		}
		return owner, host, got.ResponseLease, barrier, observer, retention
	}
	readResponses := func(label string, lease RetainedResponseLease, barrier *privateC18ReachedBarrier, retention privateC18FixtureOwnerBarrierEvent) []lspwire.Message {
		var messages []lspwire.Message
		result := make(chan privateC18Result, 1)
		group := privateC18OwnTaskGroup(t, barrier.Abort)
		task := group.Start(func() error {
			err := lease.WithResponses(func(got []lspwire.Message) error {
				messages = append([]lspwire.Message(nil), got...)
				return nil
			})
			result <- privateC18Result{Contender: privateC18FixtureContenderReadback, Stage: privateC18FixtureReadbackBorrow, Err: err}
			return err
		})
		readback := barrier.await(t, privateC18FixtureReadbackBorrow, label+"_READBACK_BORROW")
		if readback.TransactionID != retention.TransactionID || readback.ObjectID != retention.ObjectID || readback.Sequence == 0 || readback.Sequence == retention.Sequence {
			t.Fatalf("ASSERT_C18_%s_EXACT_READBACK_AUTHORITY retention=%+v readback=%+v", label, retention, readback)
		}
		barrier.release(privateC18FixtureReadbackBorrow)
		got := privateC18AwaitResult(t, result, label+"_READBACK_RESULT")
		if err := privateC18AwaitResult(t, task, label+"_READBACK_TASK"); err != nil || got.Contender != privateC18FixtureContenderReadback || got.Stage != privateC18FixtureReadbackBorrow || got.Err != nil {
			t.Fatalf("ASSERT_C18_%s_READBACK task_err=%v result=%+v", label, err, got)
		}
		return messages
	}

	const oldMarker = `{"c18_payload":"OLD_GENERATION_UNIQUE"}`
	const freshMarker = `{"c18_payload":"FRESH_GENERATION_UNIQUE"}`
	_, oldHost, oldLease, oldBarrier, oldObserver, oldRetention := acquire("OLD_PAYLOAD", oldMarker)
	oldReadback := readResponses("OLD_PAYLOAD", oldLease, oldBarrier, oldRetention)
	if len(oldReadback) != 1 || string(oldReadback[0].Result) != oldMarker {
		t.Fatalf("ASSERT_C18_OLD_PAYLOAD_MARKER got=%+v", oldReadback)
	}
	privateC18FixtureReleaseResponseAndAssertClosedWithBorrowKind(t, oldLease, oldObserver, oldBarrier, "alias")
	ctx := privateC18LifecycleContext(t)
	if failure := oldHost.stopPrivateC18Pair9CoordinatorForTest(ctx); failure != "" {
		t.Fatalf("ASSERT_C18_OLD_PAYLOAD_STOP failure=%q", failure)
	}
	if stopped := oldHost.awaitPrivateC18Pair9StoppedForTest(ctx); !stopped.Stopped {
		t.Fatalf("ASSERT_C18_OLD_PAYLOAD_NOT_STOPPED snapshot=%+v", stopped)
	}
	if restarted := oldHost.restartPrivateC18Pair9CoordinatorForTest(ctx); restarted.Stopped {
		t.Fatalf("ASSERT_C18_OLD_PAYLOAD_RESTART snapshot=%+v", restarted)
	}

	_, _, freshLease, freshBarrier, freshObserver, freshRetention := acquire("FRESH_PAYLOAD", freshMarker)
	if oldBarrier == freshBarrier {
		t.Fatal("ASSERT_C18_RESTART_REUSED_OWNER_SCOPE")
	}
	freshReadback := readResponses("FRESH_PAYLOAD", freshLease, freshBarrier, freshRetention)
	if len(freshReadback) != 1 || string(freshReadback[0].Result) != freshMarker || strings.Contains(string(freshReadback[0].Result), "OLD_GENERATION_UNIQUE") {
		t.Fatalf("ASSERT_C18_FRESH_PAYLOAD_ISOLATION got=%+v", freshReadback)
	}
	oldCallbacks := 0
	if err := oldLease.WithResponses(func([]lspwire.Message) error { oldCallbacks++; return nil }); !errors.Is(err, ErrRetainedResponseLeaseClosed) {
		t.Fatalf("ASSERT_C18_OLD_OWNER_REOPENED err=%v", err)
	}
	if oldCallbacks != 0 {
		t.Fatalf("ASSERT_C18_OLD_OWNER_CALLBACK_AFTER_RELEASE_RESTART callbacks=%d", oldCallbacks)
	}
	privateC18FixtureReleaseResponseAndAssertClosedWithBorrowKind(t, freshLease, freshObserver, freshBarrier, "alias")
}

func privateC18OperationCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

func privateC18OperationScopeHasCall(scope *ast.BlockStmt, want string) bool {
	found := false
	ast.Inspect(scope, func(node ast.Node) bool {
		if nested, ok := node.(*ast.FuncLit); ok && nested.Body != scope {
			return false
		}
		if call, ok := node.(*ast.CallExpr); ok && privateC18OperationCallName(call) == want {
			found = true
			return false
		}
		return true
	})
	return found
}

func privateC18OperationBranchHasRelevantCall(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(candidate ast.Node) bool {
		if _, ok := candidate.(*ast.FuncLit); ok {
			return false
		}
		call, ok := candidate.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch privateC18OperationCallName(call) {
		case "Start", "awaitPairAnyOrder", "release", "privateC18AwaitReachedPairAnyOrder":
			found = true
			return false
		default:
			return true
		}
	})
	return found
}

func privateC18OperationScopeBranchError(scope *ast.BlockStmt) error {
	var branchErr error
	ast.Inspect(scope, func(node ast.Node) bool {
		if branchErr != nil {
			return false
		}
		if nested, ok := node.(*ast.FuncLit); ok && nested.Body != scope {
			return false
		}
		var branches []ast.Node
		switch statement := node.(type) {
		case *ast.IfStmt:
			branches = append(branches, statement.Body)
			if statement.Else != nil {
				branches = append(branches, statement.Else)
			}
		case *ast.SwitchStmt:
			for _, clause := range statement.Body.List {
				branches = append(branches, clause)
			}
		case *ast.TypeSwitchStmt:
			for _, clause := range statement.Body.List {
				branches = append(branches, clause)
			}
		case *ast.SelectStmt:
			for _, clause := range statement.Body.List {
				branches = append(branches, clause)
			}
		}
		relevantBranches := 0
		for _, branch := range branches {
			if privateC18OperationBranchHasRelevantCall(branch) {
				relevantBranches++
			}
		}
		if relevantBranches > 1 {
			branchErr = fmt.Errorf("ASSERT_C18_OPERATION_SCOPE_SPLIT_MUTUALLY_EXCLUSIVE_BRANCHES relevant_branches=%d", relevantBranches)
			return false
		}
		return true
	})
	return branchErr
}

func privateC18OperationScopeError(scope *ast.BlockStmt) error {
	if err := privateC18OperationScopeBranchError(scope); err != nil {
		return err
	}
	releases := 0
	var launchPositions []token.Pos
	physical, seam := token.NoPos, token.NoPos
	ast.Inspect(scope, func(node ast.Node) bool {
		if nested, ok := node.(*ast.FuncLit); ok && nested.Body != scope {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch privateC18OperationCallName(call) {
		case "Start":
			launchPositions = append(launchPositions, call.Pos())
		case "awaitPairAnyOrder":
			physical = call.Pos()
		case "release":
			if physical.IsValid() && call.Pos() > physical && (!seam.IsValid() || call.Pos() < seam) {
				releases++
			}
		case "privateC18AwaitReachedPairAnyOrder":
			seam = call.Pos()
		}
		return true
	})
	launches := 0
	for _, position := range launchPositions {
		if seam.IsValid() && position < seam {
			launches++
		}
	}
	if launches != 2 || !physical.IsValid() || releases == 0 || !seam.IsValid() || physical >= seam {
		return fmt.Errorf("ASSERT_C18_OPERATION_SCOPE_STRUCTURE launches=%d physical=%d releases=%d seam=%d", launches, physical, releases, seam)
	}
	return nil
}

func privateC18OperationScopesError(source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "c18_test.go", source, 0)
	if err != nil {
		return err
	}
	want := map[string]int{
		"TestADR0011C18PostDecisionCancellationUsesExternalDecisionAndEffects": 2,
		"TestADR0011C18StopRestartUsesActualAuthorityOperations":               2,
		"TestADR0011C18SplitSeamRestoreGenerationSurvivesRestartAndStop":       1,
		"TestADR0011C18PostDecisionStopAbortsBlockedEffectGate":                1,
	}
	seen := map[string]int{}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Body == nil || want[fn.Name.Name] == 0 {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || privateC18OperationCallName(call) != "Run" || len(call.Args) != 2 {
				return true
			}
			closure, ok := call.Args[1].(*ast.FuncLit)
			if !ok || !privateC18OperationScopeHasCall(closure.Body, "privateC18AwaitReachedPairAnyOrder") {
				return true
			}
			seen[fn.Name.Name]++
			if err == nil {
				err = privateC18OperationScopeError(closure.Body)
			}
			return false
		})
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range assign.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name != "runActualOperation" || i >= len(assign.Rhs) {
					continue
				}
				if closure, ok := assign.Rhs[i].(*ast.FuncLit); ok {
					seen[fn.Name.Name]++
					if err == nil {
						err = privateC18OperationScopeError(closure.Body)
					}
				}
			}
			return true
		})
		if seen[fn.Name.Name] == 0 && privateC18OperationScopeHasCall(fn.Body, "privateC18AwaitReachedPairAnyOrder") {
			seen[fn.Name.Name]++
			if err == nil {
				err = privateC18OperationScopeError(fn.Body)
			}
		}
	}
	if err != nil {
		return err
	}
	for name, count := range want {
		if seen[name] != count {
			return fmt.Errorf("ASSERT_C18_OPERATION_SCOPE_CARDINALITY function=%s got=%d want=%d", name, seen[name], count)
		}
	}
	return nil
}

func TestADR0011C18OperationRunningSplitSeamsUsePhysicalPairBeforeSeamPair(t *testing.T) {
	source, err := os.ReadFile("adr0011_c18_real_owner_collision_red_private_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := privateC18OperationScopesError(source); err != nil {
		t.Fatal(err)
	}
	negatives := map[string][]byte{
		"old-whole-function-index":        []byte(`package p; func TestADR0011C18PostDecisionCancellationUsesExternalDecisionAndEffects(t T){ t.Run("pair8", func(t T){ group.Start(func() error{return nil}); group.Start(func() error{return nil}); privateC18AwaitReachedPairAnyOrder(t,reached,1,2); barrier.awaitPairAnyOrder(t,1,2); barrier.release(1) }) }`),
		"pair8-correct-pair9-wrong":       []byte(`package p; func TestADR0011C18PostDecisionCancellationUsesExternalDecisionAndEffects(t T){ t.Run("pair8", func(t T){ group.Start(func() error{return nil}); group.Start(func() error{return nil}); barrier.awaitPairAnyOrder(t,1,2); barrier.release(1); privateC18AwaitReachedPairAnyOrder(t,reached,1,2) }); t.Run("pair9", func(t T){ group.Start(func() error{return nil}); group.Start(func() error{return nil}); privateC18AwaitReachedPairAnyOrder(t,reached,1,2); barrier.awaitPairAnyOrder(t,1,2); barrier.release(1) }) }`),
		"nested-runActualOperation-wrong": []byte(`package p; func TestADR0011C18StopRestartUsesActualAuthorityOperations(t T){ runActualOperation := func(){ group.Start(func() error{return nil}); group.Start(func() error{return nil}); privateC18AwaitReachedPairAnyOrder(t,reached,1,2); barrier.awaitPairAnyOrder(t,1,2); barrier.release(1) }; _ = runActualOperation }`),
		"physical-if-seam-else":           []byte(`package p; func TestADR0011C18PostDecisionStopAbortsBlockedEffectGate(t T){ group.Start(func() error{return nil}); group.Start(func() error{return nil}); if condition { barrier.awaitPairAnyOrder(t,1,2); barrier.release(1) } else { privateC18AwaitReachedPairAnyOrder(t,reached,1,2) } }`),
	}
	for label, negative := range negatives {
		if err := privateC18OperationScopesError(negative); err == nil {
			t.Fatalf("ASSERT_C18_OPERATION_SCOPE_NEGATIVE_CONTROL label=%s", label)
		}
	}
	pair8 := map[privateC18FixtureContender]privateC18FixtureOwnerBoundary{privateC18FixtureContenderObjectEvent: privateC18FixtureObjectEventCommit, privateC18FixtureContenderRetention: privateC18FixtureRetentionCommit}
	pair9 := map[privateC18FixtureContender]privateC18FixtureOwnerBoundary{privateC18FixtureContenderRetention: privateC18FixtureRetentionCommit, privateC18FixtureContenderReadback: privateC18FixtureReadbackBorrow}
	for label, control := range map[string]struct {
		got  [2]privateC18TestReachedEvent
		want map[privateC18FixtureContender]privateC18FixtureOwnerBoundary
	}{
		"pair8": {[2]privateC18TestReachedEvent{{Contender: privateC18FixtureContenderObjectEvent, Boundary: privateC18FixtureObjectEventCommit}, {Contender: privateC18FixtureContenderRetention, Boundary: privateC18FixtureRetentionCommit}}, pair8},
		"pair9": {[2]privateC18TestReachedEvent{{Contender: privateC18FixtureContenderReadback, Boundary: privateC18FixtureReadbackBorrow}, {Contender: privateC18FixtureContenderRetention, Boundary: privateC18FixtureRetentionCommit}}, pair9},
	} {
		if err := privateC18ReachedPairMappingError(control.got, control.want); err != nil {
			t.Fatalf("ASSERT_C18_REACHED_MAPPING_POSITIVE_CONTROL label=%s err=%v", label, err)
		}
	}
	for label, got := range map[string][2]privateC18TestReachedEvent{
		"swapped":   {{Contender: privateC18FixtureContenderObjectEvent, Boundary: privateC18FixtureRetentionCommit}, {Contender: privateC18FixtureContenderRetention, Boundary: privateC18FixtureObjectEventCommit}},
		"duplicate": {{Contender: privateC18FixtureContenderObjectEvent, Boundary: privateC18FixtureObjectEventCommit}, {Contender: privateC18FixtureContenderObjectEvent, Boundary: privateC18FixtureObjectEventCommit}},
		"synthetic": {{Contender: privateC18FixtureContenderObjectEvent, Boundary: privateC18FixtureRelease}, {Contender: privateC18FixtureContenderRetention, Boundary: privateC18FixtureRetentionCommit}},
	} {
		if err := privateC18ReachedPairMappingError(got, pair8); err == nil {
			t.Fatalf("ASSERT_C18_REACHED_MAPPING_NEGATIVE_CONTROL label=%s", label)
		}
	}
}

func TestADR0011C18TargetContainsNoTimeSleep(t *testing.T) {
	source, err := os.ReadFile("adr0011_c18_real_owner_collision_red_private_test.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "c18_test.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, pkgOK := selector.X.(*ast.Ident)
		if pkgOK && pkg.Name == "time" && selector.Sel.Name == "Sleep" {
			t.Fatalf("ASSERT_C18_ZERO_TIME_SLEEP")
		}
		return true
	})
}

func TestADR0011C18RequiredMethodOrderIsStable(t *testing.T) {
	adapter := newPrivateC18ReflectAdapter(t, "ORDER_CONTROL", &struct{}{})
	contracts := map[string][2]int{"Zulu": {0, 0}, "Alpha": {0, 0}, "Middle": {0, 0}}
	want := []string{"Alpha", "Middle", "Zulu"}
	for iteration := 0; iteration < 100; iteration++ {
		names, missing := adapter.requiredMethodNames(contracts)
		if !reflect.DeepEqual(names, want) || !reflect.DeepEqual(missing, want) {
			t.Fatalf("ASSERT_C18_REQUIRED_METHOD_ORDER iteration=%d names=%v missing=%v want=%v", iteration, names, missing, want)
		}
	}
}

func TestADR0011C18HarnessCleanupContracts(t *testing.T) {
	t.Run("abort-releases-and-joins-waiter", func(t *testing.T) {
		barrier := newPrivateC18ReachedBarrier()
		done := make(chan error, 1)
		go func() {
			done <- barrier.observe(privateC18FixtureOwnerBarrierEvent{TransactionID: "tx", ObjectID: "object", Sequence: 1, Boundary: privateC18FixtureRetentionCommit})
		}()
		barrier.await(t, privateC18FixtureRetentionCommit)
		barrier.Abort()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("ASSERT_C18_ABORT_DID_NOT_JOIN_WAITER")
		}
		barrier.CloseAll()
	})
	t.Run("cleanup-restore-lifo", func(t *testing.T) {
		var order []int
		inner := func(t *testing.T) {
			t.Cleanup(func() { order = append(order, 1) })
			t.Cleanup(func() { order = append(order, 2) })
		}
		t.Run("scope", inner)
		if !reflect.DeepEqual(order, []int{2, 1}) {
			t.Fatalf("ASSERT_C18_RESTORE_NOT_LIFO got=%v", order)
		}
	})
	t.Run("consumed-expected-error-does-not-fail-close", func(t *testing.T) {
		expected := errors.New("worker-report-expected")
		group := newPrivateC18TaskGroup()
		result := group.Start(func() error { return expected })
		got := privateC18AwaitResult(t, result, "TASKGROUP_EXPECTED_ERROR")
		if !errors.Is(got, expected) || !group.Acknowledge(got) {
			t.Fatalf("ASSERT_C18_WORKER_EXPECTED_ERROR_NOT_ACKNOWLEDGED got=%v", got)
		}
		if err := group.Close(); err != nil {
			t.Fatalf("ASSERT_C18_CONSUMED_ERROR_FAILED_CLOSE err=%v", err)
		}
	})
	t.Run("unconsumed-unexpected-error-fails-close", func(t *testing.T) {
		unexpected := errors.New("worker-report-unexpected")
		group := newPrivateC18TaskGroup()
		result := group.Start(func() error { return unexpected })
		if got := privateC18AwaitResult(t, result, "TASKGROUP_UNEXPECTED_ERROR"); !errors.Is(got, unexpected) {
			t.Fatalf("ASSERT_C18_WORKER_UNEXPECTED_RESULT_LOST got=%v", got)
		}
		if err := group.Close(); !errors.Is(err, unexpected) {
			t.Fatalf("ASSERT_C18_UNCONSUMED_ERROR_DID_NOT_FAIL_CLOSE err=%v", err)
		}
	})
}
