package sessionruntime

import (
	"encoding/json"
	"errors"
)

// PrivateDiscoveryRefusal is a bounded value, not authority or a retry permit.
// AmountsAvailable distinguishes an observed zero balance from unknown amounts.
type PrivateDiscoveryRefusal struct {
	Stage            PrivateDiscoveryRefusalStage `json:"stage"`
	Currency         PrivateDiscoveryCurrency     `json:"currency"`
	Requested        uint64                       `json:"requested"`
	Remaining        uint64                       `json:"remaining"`
	AmountsAvailable bool                         `json:"amounts_available"`
	CleanupUncertain bool                         `json:"cleanup_uncertain"`
}
type PrivateDiscoveryRefusalStage uint8
type PrivateDiscoveryCurrency uint8

const (
	PrivateDiscoveryAccounting PrivateDiscoveryRefusalStage = iota + 1
	PrivateDiscoverySource
	PrivateDiscoveryFrame
	PrivateDiscoveryBackend
	PrivateDiscoveryWrite
	PrivateDiscoveryRead
	PrivateDiscoveryAllSupplied
	PrivateDiscoveryConsume
	PrivateDiscoveryCleanup
	PrivateDiscoveryUnknownStage
)
const (
	PrivateDiscoveryUnknownCurrency PrivateDiscoveryCurrency = iota
	PrivateDiscoverySourceBytes
	PrivateDiscoveryRequestBytes
	PrivateDiscoveryResponseBytes
	PrivateDiscoveryWireBytes
	PrivateDiscoveryRetainedBytes
	PrivateDiscoveryWork
	PrivateDiscoveryMessages
	PrivateDiscoveryRequests
)

func (s PrivateDiscoveryRefusalStage) label() string {
	switch s {
	case PrivateDiscoveryAccounting:
		return "accounting"
	case PrivateDiscoverySource:
		return "source_quote"
	case PrivateDiscoveryFrame:
		return "frame_preflight"
	case PrivateDiscoveryBackend:
		return "backend_preflight"
	case PrivateDiscoveryWrite:
		return "write_reservation"
	case PrivateDiscoveryRead:
		return "read_reservation"
	case PrivateDiscoveryAllSupplied:
		return "all_supplied"
	case PrivateDiscoveryConsume:
		return "consume"
	case PrivateDiscoveryCleanup:
		return "cleanup"
	case PrivateDiscoveryUnknownStage:
		return "unknown"
	}
	return ""
}
func (c PrivateDiscoveryCurrency) label() string {
	switch c {
	case PrivateDiscoveryUnknownCurrency:
		return "unknown"
	case PrivateDiscoverySourceBytes:
		return "source_bytes"
	case PrivateDiscoveryRequestBytes:
		return "request_bytes"
	case PrivateDiscoveryResponseBytes:
		return "response_bytes"
	case PrivateDiscoveryWireBytes:
		return "wire_bytes"
	case PrivateDiscoveryRetainedBytes:
		return "retained_bytes"
	case PrivateDiscoveryWork:
		return "work"
	case PrivateDiscoveryMessages:
		return "messages"
	case PrivateDiscoveryRequests:
		return "requests"
	}
	return ""
}
func (s PrivateDiscoveryRefusalStage) MarshalJSON() ([]byte, error) {
	if s.label() == "" {
		return nil, errors.New("invalid private discovery diagnostic")
	}
	return json.Marshal(s.label())
}
func (c PrivateDiscoveryCurrency) MarshalJSON() ([]byte, error) {
	if c.label() == "" {
		return nil, errors.New("invalid private discovery diagnostic")
	}
	return json.Marshal(c.label())
}
func (s *PrivateDiscoveryRefusalStage) UnmarshalJSON(raw []byte) error {
	var label string
	if json.Unmarshal(raw, &label) != nil {
		return errors.New("invalid private discovery diagnostic")
	}
	for value := PrivateDiscoveryAccounting; value <= PrivateDiscoveryUnknownStage; value++ {
		if value.label() == label {
			*s = value
			return nil
		}
	}
	return errors.New("invalid private discovery diagnostic")
}
func (c *PrivateDiscoveryCurrency) UnmarshalJSON(raw []byte) error {
	var label string
	if json.Unmarshal(raw, &label) != nil {
		return errors.New("invalid private discovery diagnostic")
	}
	for value := PrivateDiscoveryUnknownCurrency; value <= PrivateDiscoveryRequests; value++ {
		if value.label() == label {
			*c = value
			return nil
		}
	}
	return errors.New("invalid private discovery diagnostic")
}
func (d PrivateDiscoveryRefusal) Valid() bool {
	if d.Stage < PrivateDiscoveryAccounting || d.Stage > PrivateDiscoveryUnknownStage {
		return false
	}
	if !d.AmountsAvailable {
		return d.Currency == PrivateDiscoveryUnknownCurrency && d.Requested == 0 && d.Remaining == 0
	}
	return d.Currency >= PrivateDiscoverySourceBytes && d.Currency <= PrivateDiscoveryRequests && d.Requested > d.Remaining && d.Stage <= PrivateDiscoveryRead
}

// Validate the complete value even outside the connected census DTO. The alias
// avoids recursively calling this method; enum marshalers still enforce labels.
func (d PrivateDiscoveryRefusal) MarshalJSON() ([]byte, error) {
	if !d.Valid() {
		return nil, errors.New("invalid private discovery diagnostic")
	}
	type value PrivateDiscoveryRefusal
	return json.Marshal(value(d))
}

type privateDiscoveryRefusalError struct{ diagnostic PrivateDiscoveryRefusal }

func (e privateDiscoveryRefusalError) Error() string { return "private discovery enclosure refused" }
func (e privateDiscoveryRefusalError) Unwrap() error { return ErrPrivateDiscovery }

// PrivateDiscoveryRefusalFromError returns a copy; it exposes no mutable recorder.
func PrivateDiscoveryRefusalFromError(err error) (PrivateDiscoveryRefusal, bool) {
	var e privateDiscoveryRefusalError
	if errors.As(err, &e) && e.diagnostic.Valid() {
		return e.diagnostic, true
	}
	return PrivateDiscoveryRefusal{}, false
}
func (w *privateDiscoveryWindow) refusalError(fallback PrivateDiscoveryRefusalStage, cleanupUncertain bool) error {
	return w.refusalErrorFrom(nil, fallback, cleanupUncertain)
}
func (w *privateDiscoveryWindow) refusalErrorFrom(err error, fallback PrivateDiscoveryRefusalStage, cleanupUncertain bool) error {
	prior, priorKnown := PrivateDiscoveryRefusalFromError(err)
	candidate := prior
	if !priorKnown {
		candidate = PrivateDiscoveryRefusal{Stage: fallback}
	}
	w.mu.Lock()
	// Every generated refusal, including finalization's non-budget fallback,
	// is recorded before returning it. The first recorder entry therefore wins
	// over a later supplied error, without making prior arguments always win.
	w.recordRefusalLocked(candidate)
	d := w.refusal
	w.mu.Unlock()
	d.CleanupUncertain = d.CleanupUncertain || cleanupUncertain || priorKnown && prior.CleanupUncertain
	return privateDiscoveryRefusalError{d}
}

type privateDiscoveryCharge struct {
	field  *uint64
	amount uint64
}

func (w *privateDiscoveryWindow) currencyLocked(p *uint64) PrivateDiscoveryCurrency {
	switch p {
	case &w.remaining.SourceBytes:
		return PrivateDiscoverySourceBytes
	case &w.remaining.RequestBytes:
		return PrivateDiscoveryRequestBytes
	case &w.remaining.ResponseBytes:
		return PrivateDiscoveryResponseBytes
	case &w.remaining.WireBytes:
		return PrivateDiscoveryWireBytes
	case &w.remaining.RetainedBytes:
		return PrivateDiscoveryRetainedBytes
	case &w.remaining.Work:
		return PrivateDiscoveryWork
	case &w.remaining.Messages:
		return PrivateDiscoveryMessages
	case &w.remaining.Requests:
		return PrivateDiscoveryRequests
	}
	return PrivateDiscoveryUnknownCurrency
}

// Caller holds mu. Validate every currency before mutating any counter. Checks
// use subtraction/comparison, never potentially wrapping cumulative additions.
func (w *privateDiscoveryWindow) reserveLocked(stage PrivateDiscoveryRefusalStage, charges ...privateDiscoveryCharge) bool {
	if w.failed || w.ctx.Err() != nil {
		w.failLocked(PrivateDiscoveryRefusal{Stage: stage})
		return false
	}
	for _, c := range charges {
		currency := w.currencyLocked(c.field)
		if currency == PrivateDiscoveryUnknownCurrency {
			w.failLocked(PrivateDiscoveryRefusal{Stage: stage})
			return false
		}
		if c.amount > *c.field {
			w.failLocked(PrivateDiscoveryRefusal{Stage: stage, Currency: currency, Requested: c.amount, Remaining: *c.field, AmountsAvailable: true})
			return false
		}
	}
	for _, c := range charges {
		*c.field -= c.amount
	}
	return true
}

// Caller holds mu. Diagnostic identity and accounting poison are independent:
// recording an incomplete/consume refusal must not block bounded Stop cleanup.
func (w *privateDiscoveryWindow) recordRefusalLocked(d PrivateDiscoveryRefusal) {
	if !w.refusal.Valid() && d.Valid() {
		w.refusal = d
	}
}
func (w *privateDiscoveryWindow) failLocked(d PrivateDiscoveryRefusal) {
	w.recordRefusalLocked(d)
	w.failed = true
}

// finalize is the production post-consume decision. It has no starter, Manager,
// transport, source read, funding, publication or native authority side effect.
func (s *PrivateDiscoverySession) finalize(err error) error {
	s.mu.Lock()
	s.closed = true
	allSupplied := len(s.supplied) == len(s.members)
	s.mu.Unlock()
	if !allSupplied {
		return s.client.w.refusalErrorFrom(err, PrivateDiscoveryAllSupplied, false)
	}
	if err != nil {
		return s.client.w.refusalErrorFrom(err, PrivateDiscoveryConsume, false)
	}
	return nil
}
