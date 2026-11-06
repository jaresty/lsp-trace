package inventorysuccessor_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
	"lsp-trace/sessionruntime"
)

func nativeParentFromActualV2(v sessionruntime.PrivateDirectionalEvidenceViewV2) acquisition.PrivateDirectionalParentV2 {
	supply, _ := json.Marshal(v.Supply)
	p := acquisition.PrivateDirectionalParentV2{OperationID: v.OperationID, SessionID: v.Session.SessionID, Generation: v.Session.Generation, URI: v.Request.URI, LanguageID: v.Request.LanguageID, Line: v.Request.Line, Character: v.Request.Character, MaxMessages: v.Request.MaxMessages, PositionEncoding: v.Metadata.PositionEncoding, SupplyURI: v.Supply.URI, SupplyObservation: supply}
	for _, r := range v.Requests {
		p.Requests = append(p.Requests, acquisition.PrivateDirectionalObservationV2{Ordinal: r.Ordinal, Method: r.Method, RequestID: r.RequestID, Generation: r.Generation, Params: r.Params, Result: r.Result})
	}
	return p
}
func TestPrivateNativeConstructorDiscriminatorFalseViewBinding(t *testing.T) {
	m, q, _ := nativeConstructorFixtureV2(t, false)
	err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
		h, e := m.CaptureDirectionalEvidencePrivateV2(ctx, q.Session, q.Directional)
		if e != nil {
			return e
		}
		v, e := m.TakeDirectionalEvidencePrivateV2(ctx, h)
		if e != nil {
			return e
		}
		return func(v sessionruntime.PrivateDirectionalEvidenceViewV2) error {
			p := nativeParentFromActualV2(v)
			p.URI = strings.Replace(p.URI, "root.go", "foreign.go", 1)
			p.SupplyURI = p.URI // pass source-event equality; fail actual prepare binding
			_, e = acquisition.BuildDirectionalViewsPrivateV2(p)
			if !errors.Is(e, acquisition.ErrPrivateDirectionalViewBindingV2) {
				t.Fatalf("ASSERT_FALSE_PARENT_VIEW_BINDING: got=%v", e)
			}
			t.Log("ASSERT_FALSE_PARENT_VIEW_BINDING: PASS; private actual-record normalization")
			return nil
		}(v)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrivateNativeConstructorDiscriminatorOpaqueControls(t *testing.T) {
	for _, mode := range []string{"zero", "foreign-manager", "released", "foreign-lease", "wrong-generation", "changed-source", "expired", "double-consume", "concurrent"} {
		t.Run(mode, func(t *testing.T) {
			m, q, _ := nativeConstructorFixtureV2(t, false)
			var h sessionruntime.PrivateDirectionalEvidenceV2
			var retired context.Context
			callbacks := 0
			var mu sync.Mutex
			take := func(ctx context.Context, manager *sessionruntime.Manager, h sessionruntime.PrivateDirectionalEvidenceV2) error {
				_, err := manager.TakeDirectionalEvidencePrivateV2(ctx, h)
				if err != nil {
					return err
				}
				mu.Lock()
				callbacks++
				mu.Unlock()
				return nil
			}
			err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				retired = ctx
				if mode == "wrong-generation" {
					bad := q.Directional
					bad.Generation++
					_, e := m.CaptureDirectionalEvidencePrivateV2(ctx, q.Session, bad)
					if !errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
						t.Fatalf("ASSERT_OPAQUE_wrong-generation: %v", e)
					}
					return nil
				}
				if mode == "zero" {
					e := take(ctx, m, h)
					if !errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
						t.Fatalf("ASSERT_OPAQUE_zero: %v", e)
					}
					return nil
				}
				if mode == "expired" {
					q.Directional.Deadline = time.Now().Add(150 * time.Millisecond)
				}
				var e error
				h, e = m.CaptureDirectionalEvidencePrivateV2(ctx, q.Session, q.Directional)
				if e != nil {
					return e
				}
				switch mode {
				case "released", "foreign-lease":
					return nil
				case "changed-source":
					u, parseErr := url.Parse(q.Directional.URI)
					if parseErr != nil {
						return parseErr
					}
					if writeErr := os.WriteFile(u.Path, []byte("package fixture\nfunc Root(){ }\n"), 0644); writeErr != nil {
						return writeErr
					}
					d := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, URI: q.Directional.URI, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
					if d.Failure != "" {
						t.Fatal(d.Failure)
					}
					e = take(ctx, m, h)
				case "foreign-manager":
					other, _, _ := nativeConstructorFixtureV2(t, false)
					e = take(ctx, other, h)
				case "expired":
					timer := time.NewTimer(time.Until(q.Directional.Deadline) + time.Millisecond)
					defer timer.Stop()
					<-timer.C
					e = take(ctx, m, h)
				case "double-consume":
					copied := h
					if e = take(ctx, m, h); e != nil {
						return e
					}
					e = take(ctx, m, copied)
				case "concurrent":
					var wg sync.WaitGroup
					errorsSeen := make(chan error, 2)
					for i := 0; i < 2; i++ {
						wg.Add(1)
						go func() { defer wg.Done(); errorsSeen <- take(ctx, m, h) }()
					}
					wg.Wait()
					close(errorsSeen)
					successes := 0
					refusals := 0
					for e := range errorsSeen {
						if e == nil {
							successes++
						} else if errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
							refusals++
						}
					}
					if successes != 1 || refusals != 1 {
						t.Fatalf("ASSERT_OPAQUE_concurrent: success=%d refusal=%d", successes, refusals)
					}
					return nil
				}
				if !errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
					t.Fatalf("ASSERT_OPAQUE_%s: %v", mode, e)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "foreign-lease" {
				if e := m.WithReadySessionLease(context.Background(), q.Session, func(current context.Context) error {
					e := take(current, m, h)
					if !errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
						t.Fatalf("ASSERT_OPAQUE_foreign-lease: %v", e)
					}
					return nil
				}); e != nil {
					t.Fatal(e)
				}
			}
			if mode == "released" {
				e := take(retired, m, h)
				if !errors.Is(e, sessionruntime.ErrPrivateDirectionalEvidenceV2) {
					t.Fatalf("ASSERT_OPAQUE_released: %v", e)
				}
			}
			want := 0
			if mode == "double-consume" || mode == "concurrent" {
				want = 1
			}
			if callbacks != want {
				t.Fatalf("ASSERT_OPAQUE_%s_CALLBACKS: %d != %d", mode, callbacks, want)
			}
			t.Logf("ASSERT_OPAQUE_%s: PASS callbacks=%d", mode, callbacks)
		})
	}
}

func TestPrivateNativeConstructorDiscriminatorOrdinaryCaptureAndConflict(t *testing.T) {
	m, q, _ := nativeConstructorFixtureV2(t, false)
	var got inventorysuccessor.PrivateNativeCallsResultV2
	err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
		var e error
		got, e = inventorysuccessor.ConstructNativeCallsPrivateV2(ctx, m, q)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("missing-required-capture", func(t *testing.T) {
		source := got.Sources[0]
		source.Captures = source.Captures[:len(source.Captures)-1]
		e := graphprovenance.ValidateNativeConstructorSourcePrivateV2(source)
		if e == nil || e.Error() != "V2 missing/extra capture" {
			t.Fatalf("ASSERT_MISSING_ORDINARY_CAPTURE: %v", e)
		}
		t.Log("ASSERT_MISSING_ORDINARY_CAPTURE: PASS; V2 missing/extra capture")
	})
	t.Run("unequal-same-ID-edge", func(t *testing.T) {
		g := got.Views[1].Result.Graph
		g.SchemaVersion = graph.SchemaVersionV5
		g.Invocation = got.ParentInvocation
		g.Edges = append([]graph.Edge(nil), g.Edges...)
		g.Edges[0].CallSites = append([]graph.Range(nil), g.Edges[0].CallSites...)
		g.Edges[0].CallSites[0].Start.Character++
		raw, e := json.Marshal(g)
		if e != nil {
			t.Fatal(e)
		}
		envelope, e := graphprovenance.CaptureV5(raw, q.Session.SessionID, q.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, &got.Sources[1])
		if e != nil {
			t.Fatal(e)
		}
		inputs := append([]programccompose.Input(nil), got.Inputs...)
		inputs[1].Bytes = envelope
		inputs[1].SHA256 = fmt.Sprintf("sha256:%x", sha256.Sum256(envelope))
		inputs[1].ByteLength = len(envelope)
		_, e = programccompose.Compose(inputs)
		if programccompose.ErrorBranch(e) != programccompose.BranchEdgeConflict {
			t.Fatalf("ASSERT_UNEQUAL_SAME_ID_EDGE: branch=%s err=%v", programccompose.ErrorBranch(e), e)
		}
		t.Log("ASSERT_UNEQUAL_SAME_ID_EDGE: PASS; EDGE_CONFLICT")
	})
}
