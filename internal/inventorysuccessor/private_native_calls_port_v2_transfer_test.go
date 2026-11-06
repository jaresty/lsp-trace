package inventorysuccessor_test

import (
	"context"
	"fmt"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/sessionruntime"
	"strings"
	"testing"
	"time"
)

func TestPrivateNativeConstructorWindowJoinOrdinaryExclusion(t *testing.T) {
	m, q, _ := nativeConstructorFixtureV2(t, false)
	entered, release := make(chan struct{}), make(chan struct{})
	var base context.Context
	done := make(chan error, 1)
	go func() {
		done <- m.WithReadySessionLease(context.Background(), q.Session, func(lease context.Context) error {
			bounded, cancel := context.WithDeadline(lease, q.Directional.Deadline)
			defer cancel()
			base = bounded
			got, e := inventorysuccessor.ConstructNativeCallsPrivateV2(&nativeSourcePauseV2{Context: bounded, entered: entered, release: release}, m, q)
			if e == nil && !got.Admission.Admission.Valid() {
				return fmt.Errorf("invalid ordinary admission")
			}
			return e
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("ASSERT_ORDINARY_SOURCE_ENTERED")
	}
	d := m.PrepareDocument(base, sessionruntime.DocumentRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, URI: q.Directional.URI, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	r := m.RoundTrip(base, sessionruntime.RoundTripRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, Method: "textDocument/documentSymbol", Params: []byte(`{}`), Deadline: q.Directional.Deadline, MaxMessages: 4, MaxBytes: 65536})
	close(release)
	e := <-done
	if d.Failure == "" || r.Failure == "" {
		t.Fatalf("ASSERT_ORDINARY_SAME_LEASE_EXCLUSION: prepare=%s wire=%s", d.Failure, r.Failure)
	}
	t.Log("ASSERT_ORDINARY_SAME_LEASE_EXCLUSION: PASS")
	if e != nil {
		t.Fatalf("ASSERT_ORDINARY_POST_EXCLUSION_SUCCESS: %v", e)
	}
	t.Log("ASSERT_ORDINARY_POST_EXCLUSION_SUCCESS: PASS")
	// No caller completion method exists; returning the existing lease releases
	// the transfer exclusion and the next ordinary lease is compatible again.
	e = m.WithReadySessionLease(context.Background(), q.Session, func(next context.Context) error {
		if f := m.CheckActiveSessionLeasePrivateV2(next, q.Session); f != "" {
			return fmt.Errorf("%s", f)
		}
		return nil
	})
	if e != nil {
		t.Fatalf("ASSERT_TRANSFER_RELEASE_ON_LEASE_RETURN: %v", e)
	}
	t.Log("ASSERT_TRANSFER_RELEASE_ON_LEASE_RETURN: PASS")
}

type nativeStripWindowV2 struct{ context.Context }

func (c nativeStripWindowV2) Value(key any) any {
	if strings.Contains(fmt.Sprintf("%T", key), "privateProviderWindowKey") {
		return nil
	}
	return c.Context.Value(key)
}
func TestPrivateNativeConstructorWindowJoinBinding(t *testing.T) {
	for _, mode := range []string{"stripped-window", "fresh-deadline", "subset-deadline"} {
		t.Run(mode, func(t *testing.T) {
			m, q, ctx, _ := nativeWindowFixtureV2(t, false, 15*time.Second)
			e := m.WithReadySessionLease(ctx, q.Session, func(base context.Context) error {
				c := base
				bad := q
				var cancel context.CancelFunc
				switch mode {
				case "stripped-window":
					c = nativeStripWindowV2{base}
				case "fresh-deadline":
					bad.Directional.Deadline = bad.Directional.Deadline.Add(time.Second)
				case "subset-deadline":
					c, cancel = context.WithDeadline(base, q.Directional.Deadline.Add(-time.Second))
				}
				got, e := inventorysuccessor.ConstructNativeCallsPrivateV2(c, m, bad)
				if cancel != nil {
					cancel()
				}
				if e == nil || got.Admission.Admission.Valid() {
					return fmt.Errorf("ASSERT_WINDOW_ORIGINAL_BINDING_%s: %v", mode, e)
				}
				t.Logf("ASSERT_WINDOW_ORIGINAL_BINDING_%s: PASS", mode)
				got, e = inventorysuccessor.ConstructNativeCallsPrivateV2(base, m, q)
				if e != nil || !got.Admission.Admission.Valid() {
					return fmt.Errorf("original BASE after refusal: %v", e)
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			c, e := m.FinishPrivateProviderWindow(ctx)
			if e != nil || !c.Completed {
				t.Fatalf("ASSERT_WINDOW_BINDING_NO_EFFECT: %+v %v", c, e)
			}
			t.Log("ASSERT_WINDOW_BINDING_NO_EFFECT: PASS")
		})
	}
}
