//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Only bounded content-free metadata crosses the fixture diagnostic boundary.
// The production CLI remains authoritative for wait events; no terminal is
// inferred from a pause, child exit, or the coordinator's own terminal output.
func applicationFixtureEventAction(e applicationHostEvent, app, run, role string, after uint64) (string, string) {
	kind := e.Event
	switch kind {
	case "NEEDS_HOST", "TERMINAL", "PAUSED_BUDGET", "APPLICATION_EXPIRED", "DEADLINE", "PUBLICATION_ERROR":
	default:
		kind = "UNKNOWN"
	}
	d := struct {
		Schema, Event, Phase, ApplicationSHA256, RunID, Role, Status string
		GlobalSequence                                               uint64
		BundlePresent                                                bool
	}{"scripted-application-wait-observation.v1", kind, "WAIT", e.ApplicationSHA256, e.RunID, role, "UNKNOWN", e.GlobalSequence, e.Bundle != nil}
	// Never echo arbitrary subprocess strings or bundle paths/source into logs.
	if !validApplicationDigest(d.ApplicationSHA256) {
		d.ApplicationSHA256 = "UNKNOWN"
	}
	if !validApplicationDigest(d.RunID) {
		d.RunID = "UNKNOWN"
	}
	if role != "interpreter" && role != "reviewer" {
		d.Role = "UNKNOWN"
	}
	action := "STOP"
	if e.ApplicationSHA256 == app && e.RunID == run && validApplicationDigest(app) && validApplicationDigest(run) {
		switch e.Event {
		case "TERMINAL":
			if validApplicationDigest(e.TerminalSHA256) && e.Bundle == nil {
				action = "TERMINAL"
				d.Status = "OBSERVED_TERMINAL"
			}
		case "NEEDS_HOST":
			b := e.Bundle
			if b != nil && validApplicationDigest(e.UnitID) && e.GlobalSequence >= 1 && e.GlobalSequence <= 16 && e.GlobalSequence > after && b.Protocol == product.CohortProtocol && b.Role == role && b.Sequence >= 1 && b.Sequence <= 16 && b.Sequence <= e.GlobalSequence && validApplicationDigest(b.RequestDigest) && validApplicationDigest(b.ReceiptSHA256) && b.Bytes > 0 && b.Bytes <= 1<<20 && len(b.Parts) > 0 {
				action = "NEEDS_HOST"
				d.Status = "BOUND_REQUEST"
			}
		}
	}
	raw, _ := json.Marshal(d)
	return action, string(raw)
}

// No Run goroutine is released until both actual public waits return. This
// owned barrier deterministically models exhausted startup before a child
// window exists; it does not claim to reproduce the lost historical sample.
func TestHostApplicationFixtureWindowBeforeRolesTwenty(t *testing.T) {
	t.Setenv("TEST_FIXTURE_WINDOW_BARRIER", "1")
	r, path, base := readinessLiteralFixture(t)
	readinessCombinedTwenty(t, r, path, base, "chosen/deeper/leaf/twenty.go")
}

func TestHostApplicationFixturePublicStartupPoolPause(t *testing.T) {
	r, path, _ := readinessLiteralFixture(t)
	var out, diag bytes.Buffer
	if code := runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diag); code != 0 {
		t.Fatalf("setup code=%d %s", code, diag.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil || !validApplicationDigest(setup.RunID) {
		t.Fatal("setup ticket")
	}
	r, err := readHostApplication(path)
	if err != nil {
		t.Fatal(err)
	}
	app := product.ApplicationConfigDigest(r.Configuration)
	before, err := applicationWaitChildDeadline(r.Configuration.Store, app, setup.RunID)
	if err != nil || !before.IsZero() {
		t.Fatalf("expected unpublished window %v", err)
	}
	for n := 0; n < 100; n++ {
		if err := applicationClaimWaitSlice(r.Configuration.Store, setup.RunID, []byte(`{"schedule":"BEFORE_CHILD_WINDOW"}`)); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for _, role := range []string{"interpreter", "reviewer"} {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			var out, diag bytes.Buffer
			start := time.Now()
			code := runPrivateHostApplication([]string{"wait", path, setup.RunID, role, "0", "60000"}, bytes.NewReader(nil), &out, &diag)
			var event applicationHostEvent
			if code != 0 || json.Unmarshal(out.Bytes(), &event) != nil {
				t.Errorf("public wait code=%d diagnostic_bytes=%d", code, diag.Len())
				return
			}
			action, observation := applicationFixtureEventAction(event, app, setup.RunID, role, 0)
			if event.Event != "PAUSED_BUDGET" || event.Bundle != nil || event.GlobalSequence != 0 || action != "STOP" || !strings.Contains(observation, `"Event":"PAUSED_BUDGET"`) || time.Since(start) > 5*time.Second {
				t.Errorf("ASSERT_PUBLIC_STARTUP_POOL_PAUSE A-fail %s", observation)
				return
			}
			t.Logf("ASSERT_PUBLIC_STARTUP_POOL_PAUSE A-pass slots=100 window=UNPUBLISHED schedule=BEFORE_RUN %s", observation)
		}(role)
	}
	wg.Wait()
	if err := applicationClaimWaitSlice(r.Configuration.Store, setup.RunID, []byte(`{}`)); err != product.ErrPaused {
		t.Fatalf("pool reset: %v", err)
	}
	after, err := applicationWaitChildDeadline(r.Configuration.Store, app, setup.RunID)
	if err != nil || !after.IsZero() {
		t.Fatal("wait invented a child window")
	}
}

func TestHostApplicationFixturePublicInitialDeadlineAndExpiry(t *testing.T) {
	for _, kind := range []string{"DEADLINE", "APPLICATION_EXPIRED"} {
		t.Run(kind, func(t *testing.T) {
			r, path, _ := readinessLiteralFixture(t)
			if kind == "APPLICATION_EXPIRED" {
				r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(2 * time.Second).UnixNano()
				writeApplicationFixture(t, r, path)
			}
			var out, diag bytes.Buffer
			if code := runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diag); code != 0 {
				t.Fatalf("setup code=%d %s", code, diag.String())
			}
			var setup struct{ RunID string }
			if json.Unmarshal(out.Bytes(), &setup) != nil {
				t.Fatal("setup JSON")
			}
			r, err := readHostApplication(path)
			if err != nil {
				t.Fatal(err)
			}
			ms := "1"
			if kind == "APPLICATION_EXPIRED" {
				ms = "60000"
			}
			out.Reset()
			diag.Reset()
			code := runPrivateHostApplication([]string{"wait", path, setup.RunID, "interpreter", "0", ms}, bytes.NewReader(nil), &out, &diag)
			var event applicationHostEvent
			if code != 0 || json.Unmarshal(out.Bytes(), &event) != nil {
				t.Fatalf("wait code=%d diagnostic=%s", code, diag.String())
			}
			action, observation := applicationFixtureEventAction(event, product.ApplicationConfigDigest(r.Configuration), setup.RunID, "interpreter", 0)
			if event.Event != kind || action != "STOP" || event.Bundle != nil || !strings.Contains(observation, `"Event":"`+kind+`"`) {
				t.Fatalf("ASSERT_PUBLIC_INITIAL_WAIT_EVENT A-fail expected=%s %s", kind, observation)
			}
			t.Logf("ASSERT_PUBLIC_INITIAL_WAIT_EVENT A-pass %s", observation)
		})
	}
}

func applicationFixtureCompleted(observed [2]bool) bool { return observed[0] && observed[1] }

func TestHostApplicationFixtureNoPretendReturns(t *testing.T) {
	for _, observed := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
		if got := applicationFixtureCompleted(observed); got != (observed == [2]bool{true, true}) {
			t.Fatalf("ASSERT_NO_PRETEND_RETURNS A-fail observed=%v", observed)
		}
	}
	t.Log("ASSERT_NO_PRETEND_RETURNS A-pass")
}

func TestHostApplicationFixtureRequestBinding(t *testing.T) {
	id := pilot.Digest([]byte("bound fixture"))
	valid := applicationHostEvent{Event: "NEEDS_HOST", ApplicationSHA256: id, RunID: id, UnitID: id, GlobalSequence: 1, Bundle: &product.Bundle{Protocol: product.CohortProtocol, Role: "interpreter", Sequence: 1, RequestDigest: id, ReceiptSHA256: id, Bytes: 1, Parts: []product.Part{{Path: "unused", SHA256: id, Bytes: 1}}}}
	for _, kind := range []string{"valid", "nil", "sequence-zero", "stale", "foreign-app", "foreign-run", "foreign-role", "terminal-without-hash"} {
		e := valid
		b := *valid.Bundle
		e.Bundle = &b
		after := uint64(0)
		switch kind {
		case "nil":
			e.Bundle = nil
		case "sequence-zero":
			e.GlobalSequence = 0
		case "stale":
			after = 1
		case "foreign-app":
			e.ApplicationSHA256 = pilot.Digest([]byte("foreign"))
		case "foreign-run":
			e.RunID = pilot.Digest([]byte("foreign"))
		case "foreign-role":
			e.Bundle.Role = "reviewer"
		case "terminal-without-hash":
			e.Event = "TERMINAL"
			e.Bundle = nil
		}
		action, _ := applicationFixtureEventAction(e, id, id, "interpreter", after)
		if (action == "NEEDS_HOST") != (kind == "valid") || action == "TERMINAL" {
			t.Errorf("ASSERT_REQUEST_BINDING A-fail case=%s action=%s", kind, action)
		}
	}
	if !t.Failed() {
		t.Log("ASSERT_REQUEST_BINDING A-pass")
	}
}

func TestHostApplicationFixtureTypedWaitEvents(t *testing.T) {
	for _, kind := range []string{"PAUSED_BUDGET", "APPLICATION_EXPIRED", "DEADLINE", "PUBLICATION_ERROR", "NEEDS_HOST"} {
		t.Run(kind, func(t *testing.T) {
			action, diagnostic := applicationFixtureEventAction(applicationHostEvent{Event: kind}, "", "", "interpreter", 0)
			var d struct {
				Schema, Event, Role string
				GlobalSequence      uint64
			}
			if action != "STOP" || json.Unmarshal([]byte(diagnostic), &d) != nil || d.Schema != "scripted-application-wait-observation.v1" || d.Event != kind || d.Role != "interpreter" || strings.Contains(diagnostic, "global sequence binding") {
				t.Fatalf("ASSERT_TYPED_WAIT_EVENT A-fail event=%s action=%s diagnostic=%s", kind, action, diagnostic)
			}
			t.Logf("ASSERT_TYPED_WAIT_EVENT A-pass event=%s diagnostic=%s", kind, diagnostic)
		})
	}
}
