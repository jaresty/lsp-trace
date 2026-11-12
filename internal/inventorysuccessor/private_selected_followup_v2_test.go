package inventorysuccessor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Four independently owned scopes. These are ZERO-AUTHORITY barrier probes:
// nil owner/kernel, empty head/input/originals, no reservation/paid phase/grant.
// For Execute/Begin/Finish an all-live control reaches the nil-owner mutex
// tripwire BEFORE any kernel access. A canceled scope must return normally first.
// No production hooks, filesystem stores, native calls or in-flight claims.
type followupScopes struct {
	m, p, s, caller context.Context
	cancel          [4]context.CancelFunc
}

func followupContexts(t *testing.T) *followupScopes {
	t.Helper()
	f := &followupScopes{}
	dst := []*context.Context{&f.m, &f.p, &f.s, &f.caller}
	for i := range dst {
		*dst[i], f.cancel[i] = context.WithCancel(context.Background())
		t.Cleanup(f.cancel[i])
	}
	return f
}
func (f *followupScopes) cancelOnly(t *testing.T, which int) {
	t.Helper()
	f.cancel[which]()
	for i, c := range []context.Context{f.m, f.p, f.s, f.caller} {
		if (c.Err() != nil) != (i == which) {
			t.Fatalf("scope independence %d canceled %d", i, which)
		}
	}
}
func (f *followupScopes) plan() *selectedPlanV2 {
	p := &selectedPlanV2{}
	p.request.Start.Deadline = time.Now().Add(10 * time.Second)
	p.operation = PrivateMixedInputOperationV2{state: &privateMixedInputOperationV2{retainedContext: f.m}}
	return p
}

// Capture a pure nil-owner tripwire. A panic in a canceled case fails the test;
// the separate all-live control requires it, so zero-state rejection alone is
// not mistaken for cancellation coverage. No kernel object even exists here.
func followupEntry(call func() error) (err error, tripwire bool) {
	defer func() {
		if v := recover(); v != nil {
			tripwire = true
		}
	}()
	err = call()
	return
}

func TestSelectedV2FollowupExecutePContexts(t *testing.T) {
	for _, which := range []int{-1, 0, 1, 3} {
		t.Run(fmt.Sprintf("cancel_%d", which), func(t *testing.T) {
			f := followupContexts(t)
			if which >= 0 {
				f.cancelOnly(t, which)
			}
			state := &selectedPreparationOperationV2{captured: &selectedCapturedV2{plan: f.plan()}, ctx: f.p}
			op := PrivateSelectedPreparationOperationV2{state: state}
			call := func(ctx context.Context) error {
				out, e := ExecuteSelectedPreparationV2(ctx, op)
				if out.state != nil {
					t.Fatal("P cancellation exposed prepared availability")
				}
				return e
			}
			e, trip := followupEntry(func() error { return call(f.caller) })
			if which < 0 {
				if !trip {
					t.Fatal("FOLLOWUP_P_BARRIER: all-live did not reach pre-kernel tripwire")
				}
				t.Log("FOLLOWUP_P_BARRIER: A-fail: all-live reaches pre-kernel tripwire")
				return
			}
			if trip || !errors.Is(e, ErrPrivateSelectedV2) || !state.used {
				t.Fatal("FOLLOWUP_P_BARRIER: canceled scope must consume and refuse before owner", e, trip)
			}
			e, trip = followupEntry(func() error { return call(context.Background()) })
			if trip || !errors.Is(e, ErrPrivateSelectedV2) {
				t.Fatal("fresh caller revived P", e, trip)
			}
			t.Log("FOLLOWUP_P_BARRIER: A-pass: isolated cancellation consumed before owner; no fresh-context revival")
		})
	}
}

func TestSelectedV2FollowupSourceEntryContexts(t *testing.T) {
	for _, finish := range []bool{false, true} {
		for which := -1; which < 4; which++ {
			t.Run(fmt.Sprintf("finish_%t_cancel_%d", finish, which), func(t *testing.T) {
				f := followupContexts(t)
				if which >= 0 {
					f.cancelOnly(t, which)
				}
				state := &selectedSourceOperationV2{prepared: &preparedSelectedV2{plan: f.plan(), scope: f.p}, ctx: f.s}
				// Finish's booleans exercise ONLY its pre-owner barrier. They do not assert
				// a successful Begin or paid native phase: owner/head/originals remain zero.
				if finish {
					state.used = true
					state.begun = true
				}
				op := PrivateSelectedSourceOperationV2{state: state}
				call := func(ctx context.Context) error {
					if finish {
						return op.Finish(ctx, nil)
					}
					h, c, _, objects, scope, e := op.Begin(ctx)
					if h.ID() != "" || c.RunID != "" || len(objects) != 0 || scope != nil {
						t.Fatal("S cancellation exposed handoff/availability")
					}
					return e
				}
				e, trip := followupEntry(func() error { return call(f.caller) })
				if which < 0 {
					if !trip {
						t.Fatal("FOLLOWUP_S_BARRIER: all-live did not reach pre-kernel tripwire")
					}
					t.Log("FOLLOWUP_S_BARRIER: A-fail: all-live reaches pre-kernel tripwire")
					return
				}
				if trip || !errors.Is(e, ErrPrivateSelectedV2) || !state.used || state.finished != finish || state.begun != finish {
					t.Fatal("FOLLOWUP_S_BARRIER: isolated cancellation did not retire before owner", e, trip)
				}
				e, trip = followupEntry(func() error { return call(context.Background()) })
				if trip || !errors.Is(e, ErrPrivateSelectedV2) {
					t.Fatal("fresh caller revived S entry", e, trip)
				}
				if !finish {
					if e := op.Finish(context.Background(), nil); !errors.Is(e, ErrPrivateSelectedV2) || state.finished {
						t.Fatal("failed Begin relabeled observed", e)
					}
				}
				t.Log("FOLLOWUP_S_BARRIER: A-pass: isolated cancellation consumed before owner; no fresh-context revival")
			})
		}
	}
}

func TestSelectedV2FollowupReservationContexts(t *testing.T) {
	// A reservation cannot have its own future scope yet. Cover M/caller for P
	// reserve and M/prepared-P/caller for S reserve; zero input/pair is the pure
	// all-live control, never a fabricated payable object.
	for _, source := range []bool{false, true} {
		scopes := []int{-1, 0, 3}
		if source {
			scopes = append(scopes, 1)
		}
		for _, which := range scopes {
			t.Run(fmt.Sprintf("source_%t_cancel_%d", source, which), func(t *testing.T) {
				f := followupContexts(t)
				if which >= 0 {
					f.cancelOnly(t, which)
				}
				owner := &PrivateManagedInputOwnerV2{}
				p := f.plan()
				p.owner = owner
				captured := &selectedCapturedV2{plan: p}
				prepared := &preparedSelectedV2{plan: p, scope: f.p}
				call := func(ctx context.Context) error {
					if source {
						out, e := owner.ReserveSelectedSourceV2(ctx, PreparedSelectedV2{state: prepared})
						if out.state != nil {
							t.Fatal("zero authority source reservation available")
						}
						return e
					}
					out, e := owner.ReserveSelectedPreparationV2(ctx, PrivateSelectedCapturedV2{state: captured})
					if out.state != nil {
						t.Fatal("zero authority preparation reservation available")
					}
					return e
				}
				e, trip := followupEntry(func() error { return call(f.caller) })
				if trip || e == nil {
					t.Fatal("reservation must stop without native call", e, trip)
				}
				if which < 0 {
					if errors.Is(e, ErrPrivateSelectedV2) {
						t.Fatal("FOLLOWUP_RESERVE_BARRIER: all-live did not reach pure metadata/pair rejection", e)
					}
					t.Logf("FOLLOWUP_RESERVE_BARRIER: A-fail: all-live reaches pure metadata/pair rejection: %v", e)
				} else if !errors.Is(e, ErrPrivateSelectedV2) {
					t.Fatal("FOLLOWUP_RESERVE_BARRIER: canceled scope did not stop at availability guard", e)
				}
				if source && !prepared.sourceReserved || !source && !captured.reserved {
					t.Fatal("reservation not permanently consumed")
				}
				e, trip = followupEntry(func() error { return call(context.Background()) })
				if trip || !errors.Is(e, ErrPrivateSelectedV2) {
					t.Fatal("fresh caller revived reservation", e, trip)
				}
				if which >= 0 {
					t.Log("FOLLOWUP_RESERVE_BARRIER: A-pass: isolated cancellation refused and consumed before metadata/pair")
				}
			})
		}
	}
}

func TestSelectedV2FollowupConcurrentRetirement(t *testing.T) {
	for _, entry := range []string{"P", "Begin", "Finish"} {
		t.Run(entry, func(t *testing.T) {
			f := followupContexts(t)
			f.cancelOnly(t, 3)
			p := f.plan()
			ps := &selectedPreparationOperationV2{captured: &selectedCapturedV2{plan: p}, ctx: f.p}
			ss := &selectedSourceOperationV2{prepared: &preparedSelectedV2{plan: p, scope: f.p}, ctx: f.s}
			if entry == "Finish" {
				ss.used = true
				ss.begun = true
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for n := 0; n < 16; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					e, trip := followupEntry(func() error {
						switch entry {
						case "P":
							out, e := ExecuteSelectedPreparationV2(f.caller, PrivateSelectedPreparationOperationV2{state: ps})
							if out.state != nil {
								t.Error("concurrent P availability")
							}
							return e
						case "Begin":
							h, _, _, objects, scope, e := (PrivateSelectedSourceOperationV2{state: ss}).Begin(f.caller)
							if h.ID() != "" || len(objects) != 0 || scope != nil {
								t.Error("concurrent S availability")
							}
							return e
						default:
							return (PrivateSelectedSourceOperationV2{state: ss}).Finish(f.caller, nil)
						}
					})
					if trip || !errors.Is(e, ErrPrivateSelectedV2) {
						t.Errorf("concurrent cancellation escaped before-owner barrier: %v %v", e, trip)
					}
				}()
			}
			close(start)
			wg.Wait()
			if entry == "P" && !ps.used || entry == "Begin" && (!ss.used || ss.begun) || entry == "Finish" && !ss.finished {
				t.Fatal("concurrent permanent retirement")
			}
		})
	}
}
