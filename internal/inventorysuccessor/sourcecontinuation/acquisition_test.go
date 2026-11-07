package sourcecontinuation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
)

func TestSuppliedEarlyBounds(t *testing.T) {
	for _, test := range []struct {
		name          string
		source, lower uint64
	}{{"source", 400, 200}, {"lower", 200, 400}} {
		t.Run(test.name, func(t *testing.T) {
			peak, ok := sourceAcquisitionEnvelopePeak(100, test.source, test.lower, 50, 10, 560)
			if !ok || peak != 560 {
				t.Fatal("ASSERT_EARLY_JUST_FIT", peak, ok)
			}
			source, lower := test.source, test.lower
			if test.name == "source" {
				source++
			} else {
				lower++
			}
			if _, ok := sourceAcquisitionEnvelopePeak(100, source, lower, 50, 10, 560); ok {
				t.Fatal("ASSERT_EARLY_PHASE_ONE_OVER")
			}
			t.Log("ASSERT_EARLY_JUST_FIT: PASS; ASSERT_EARLY_PHASE_ONE_OVER: PASS")
		})
	}
	t.Run("invalid", func(t *testing.T) {
		for _, values := range [][6]uint64{{0, 400, 200, 50, 10, 560}, {100, 0, 200, 50, 10, 560}, {100, 400, 0, 50, 10, 560}, {100, 400, 200, 0, 10, 560}, {100, 400, 200, 50, 10, 0}, {^uint64(0), 400, 200, 50, 10, ^uint64(0)}} {
			if _, ok := sourceAcquisitionEnvelopePeak(values[0], values[1], values[2], values[3], values[4], values[5]); ok {
				t.Fatal("ASSERT_EARLY_ZERO_OVERFLOW")
			}
		}
		t.Log("ASSERT_EARLY_ZERO_OVERFLOW: PASS")
	})
	t.Run("handoff", func(t *testing.T) {
		if _, ok := sourceAcquisitionEnvelopePeak(101, 400, 200, 50, 10, 560); ok {
			t.Fatal("ASSERT_EARLY_HANDOFF_ONE_OVER")
		}
		t.Log("ASSERT_EARLY_HANDOFF_ONE_OVER: PASS")
	})
	t.Run("other", func(t *testing.T) {
		if _, ok := sourceAcquisitionEnvelopePeak(100, 400, 200, 51, 10, 560); ok {
			t.Fatal("ASSERT_EARLY_RUNNER_ONE_OVER")
		}
		if _, ok := sourceAcquisitionEnvelopePeak(100, 400, 200, 50, 11, 560); ok {
			t.Fatal("ASSERT_EARLY_RAW_ONE_OVER")
		}
		// No public/scalar-only expected cache can reconstruct a real acquisition.
		s := &expected{}
		if _, e := sourceAcquisitionEarlyEnvelope(s, 8192, 0); e == nil {
			t.Fatal("ASSERT_EARLY_MISSING_CACHE")
		}
		s.regeneration = sourceRegenerationEnvelope{response: 1, source: 2, lower: 3}
		if _, e := sourceAcquisitionEarlyEnvelope(s, 8192, 0); e == nil {
			t.Fatal("ASSERT_EARLY_FAKE_CACHE")
		}
		t.Log("ASSERT_EARLY_RUNNER_ONE_OVER: PASS; ASSERT_EARLY_RAW_ONE_OVER: PASS; ASSERT_EARLY_MISSING_CACHE: PASS; ASSERT_EARLY_FAKE_CACHE: PASS")
	})
}

func TestSuppliedAcquireZero(t *testing.T) {
	store := &responseCountingStore{}
	q := SourceAcquisitionRequest{Resume: ResumeRequest{Store: store, PrivateSources: &PrivateAllModeSourcesResume{}}}
	var runner describeworker.Runner
	for _, r := range []*describeworker.Runner{nil, &runner} {
		h, e := PrepareSourceAcquisition(context.Background(), r, q)
		if e == nil || h.ID() != "" || store.gets != 0 {
			t.Fatal("ASSERT_ACQUIRE_ZERO_NO_IO", e)
		}
	}
	var h SourceAcquisitionHandoff
	if json.Unmarshal([]byte(`{"state":{},"plan":{},"id":"public"}`), &h) != nil || h.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_PUBLIC")
	}
	if b, e := CompleteSuppliedSourceAcquisition(context.Background(), &runner, h, SuppliedSourceResponseRequest{}); e == nil || b.ID() != "" || store.gets != 0 {
		t.Fatal("ASSERT_ACQUIRE_PUBLIC")
	}
	// A lower plan cannot be converted to an upper handoff: its only exported
	// information is a nonadmitting ID; decoding its public representation is zero.
	p := describeworker.SourceAcquisitionPlan{}
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(raw, &h) != nil || h.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_LOWER_NONADMITTING")
	}
	t.Log("ASSERT_ACQUIRE_ZERO_NO_IO: PASS; ASSERT_ACQUIRE_PUBLIC: PASS; ASSERT_ACQUIRE_LOWER_NONADMITTING: PASS")
}

func TestSuppliedAcquireBounds(t *testing.T) {
	if _, ok := sourceResponseAdd(^uint64(0), 8192); ok {
		t.Fatal("ASSERT_ACQUIRE_EXTRA_OVERFLOW")
	}
	b, e := sourceResponseAggregateBound(4096, 8192, 128, 1024, 1024, 1<<20, 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	extra := uint64(8192)
	with, e := sourceResponseAggregateBound(4096+extra, 8192, 128, 1024, 1024, 1<<20, 8<<20)
	if e != nil || with.retained != b.retained+extra || with.peak != b.peak+extra {
		t.Fatal("ASSERT_ACQUIRE_EXTRA_EXACT", e)
	}
	deadline := time.Now().Add(time.Second)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if sourceAcquisitionPublicationLive(cancelled, deadline) || sourceAcquisitionPublicationLive(context.Background(), time.Now().Add(-time.Second)) || sourceAcquisitionPublicationLive(nil, deadline) || !sourceAcquisitionPublicationLive(context.Background(), deadline) {
		t.Fatal("ASSERT_ACQUIRE_FINAL_LIVENESS")
	}
	t.Log("ASSERT_ACQUIRE_EXTRA_OVERFLOW: PASS; ASSERT_ACQUIRE_EXTRA_EXACT: PASS; ASSERT_ACQUIRE_FINAL_LIVENESS: PASS")
}
