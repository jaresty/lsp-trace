//go:build darwin || linux

package applicationinventory

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestStartCapacityRefusalBoundaries(t *testing.T) {
	base := func() durableInventoryHead {
		return durableInventoryHead{Limit: durableRunLimit, Completion: durableCompletionPlan{Version: durableQuoteVersion, StateUpper: 1, Stages: []uint64{1, 1, 1}}}
	}
	cases := []struct {
		name         string
		alter        func(*durableInventoryHead)
		reason, unit string
		deficit      uint64
	}{
		{"supplied_exact_scalar_observation", func(h *durableInventoryHead) {
			h.Used = 14825416
			h.Work = h.Used * 32
			h.Completion.Stages = []uint64{54463089, 1, 1}
		}, CapacityBytes, "bytes", 2179643},
		{"bytes_equal", func(h *durableInventoryHead) { h.Used = h.Limit - 3 }, "", "", 0},
		{"bytes_one_over", func(h *durableInventoryHead) { h.Used = h.Limit - 2 }, CapacityBytes, "bytes", 1},
		{"work_equal", func(h *durableInventoryHead) { h.Work = (1 << 32) - 96 }, "", "", 0},
		{"work_one_over", func(h *durableInventoryHead) { h.Work = (1 << 32) - 95 }, CapacityWork, "work_units", 1},
		{"packets_equal", func(h *durableInventoryHead) { h.SuccessorPackets = 61 }, "", "", 0},
		{"packets_one_over", func(h *durableInventoryHead) { h.SuccessorPackets = 62 }, CapacityPackets, "packets", 1},
		{"invalid_quote_first", func(h *durableInventoryHead) {
			h.Completion.Version = "invalid"
			h.Used = h.Limit
			h.Work = 1 << 32
			h.SuccessorPackets = 62
		}, CapacityInvalidQuote, "", 0},
		{"bytes_before_work_packets", func(h *durableInventoryHead) { h.Used = h.Limit; h.Work = 1 << 32; h.SuccessorPackets = 62 }, CapacityBytes, "bytes", 3},
		{"work_before_packets", func(h *durableInventoryHead) { h.Work = 1 << 32; h.SuccessorPackets = 62 }, CapacityWork, "work_units", 96},
		{"used_underflow", func(h *durableInventoryHead) { h.Used = math.MaxUint64 }, CapacityInvalidQuote, "", 0},
		{"work_underflow", func(h *durableInventoryHead) { h.Work = math.MaxUint64 }, CapacityInvalidQuote, "", 0},
		{"quote_overflow", func(h *durableInventoryHead) { h.Completion.Stages[0] = math.MaxUint64 }, CapacityInvalidQuote, "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := base()
			c.alter(&h)
			before := h
			err := h.startCapacityRefusal()
			if !reflect.DeepEqual(h, before) {
				t.Fatal("guard changed head")
			}
			if c.reason == "" {
				if err != nil {
					t.Fatal("equality refused", err)
				}
				return
			}
			var refusal *CapacityRefusal
			if !errors.As(err, &refusal) || !errors.Is(err, ErrDurableInventory) || refusal.Reason != c.reason {
				t.Fatalf("classification: %v", err)
			}
			if c.unit == "" {
				if refusal.Metrics != nil {
					t.Fatal("invalid quote invented metrics")
				}
				return
			}
			m := refusal.Metrics
			if m == nil || m.Unit != c.unit || m.Deficit != c.deficit || m.Available != m.Limit-m.Spent || m.Protected-m.Available != m.Deficit {
				t.Fatalf("metrics: %+v", m)
			}
		})
	}
}
