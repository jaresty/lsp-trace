//go:build darwin || linux

package applicationinventory

// CapacityRefusal contains only bounded accounting scalars, never a head, run
// locator, source content or identity. Preparation effects precede this refusal;
// no durable head has been published. It grants no permission to retry.
type CapacityRefusal struct {
	Reason  string
	Metrics *CapacityMetrics
}

type CapacityMetrics struct {
	Unit                                        string
	Spent, Protected, Limit, Available, Deficit uint64
}

const (
	CapacityInvalidQuote = "CAPACITY_INVALID_QUOTE"
	CapacityBytes        = "CAPACITY_BYTES"
	CapacityWork         = "CAPACITY_WORK"
	CapacityPackets      = "CAPACITY_PACKETS"
)

func (e *CapacityRefusal) Error() string { return e.Reason }
func (e *CapacityRefusal) Unwrap() error { return ErrDurableInventory }

func capacityRefusal(reason, unit string, spent, protected, limit uint64) error {
	// Callers establish spent <= limit and protected > available. Subtraction
	// rather than addition avoids overflow, including on hostile scalar states.
	available := limit - spent
	return &CapacityRefusal{reason, &CapacityMetrics{unit, spent, protected, limit, available, protected - available}}
}

// startCapacityRefusal is the actual Start preadmission guard, not a second
// policy. The existing quote/byte/work/packet precedence and equality admission
// are preserved. Malformed counters cannot turn subtraction into underflow.
func (h durableInventoryHead) startCapacityRefusal() error {
	protected, packets, ok := h.protectedCapacity()
	if !ok || h.Used > h.Limit || h.Work > 1<<32 {
		return &CapacityRefusal{Reason: CapacityInvalidQuote}
	}
	if protected > h.Limit-h.Used {
		return capacityRefusal(CapacityBytes, "bytes", h.Used, protected, h.Limit)
	}
	if protected > ((1<<32)-h.Work)/32 {
		// A valid protectedCapacity quote is <=64 MiB, so *32 is bounded.
		return capacityRefusal(CapacityWork, "work_units", h.Work, protected*32, 1<<32)
	}
	if packets > 64 {
		// Start's original guard compares the protected packet count with 64,
		// not with 64-h.Packets. No packet has been spent at this boundary.
		return capacityRefusal(CapacityPackets, "packets", 0, packets, 64)
	}
	return nil
}
