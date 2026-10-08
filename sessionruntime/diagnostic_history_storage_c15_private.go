package sessionruntime

import (
	"runtime"
	"unsafe"

	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

const managerPrivateDiagnosticHistorySlots = 1024

type privateDiagnosticHistoryRef struct {
	generation uint64
	slot       uint16
	qualified  bool
}

type privateDiagnosticHistorySlot struct {
	generation           uint64
	sequence             uint64
	active               bool
	closed               bool
	described            bool
	collector            *manageddiagnostic.EventCollector
	identity             managedprocess.Identity
	attemptID            manageddiagnostic.StartupAttemptID
	sessionID            string
	diagnosticGeneration uint64
	capability           *diagnosticCapability
	method               string
	documentURI          string
	initialize           SessionMetadata
	account              *privateB4ByteAccountV2
	metadataSource       privateB4ByteLeaseV2
	retainedBacking      managerDiagnosticLease
}

type privateDiagnosticHistory struct {
	slots           [managerPrivateDiagnosticHistorySlots]privateDiagnosticHistorySlot
	order           [managerPrivateDiagnosticHistorySlots]uint16
	head            uint16
	count           uint16
	nextGeneration  uint64
	backingReleased bool
}

func privateDiagnosticHistoryRepresentationSupported() bool {
	return runtime.Version() == privateB4QualifiedGoVersion &&
		unsafe.Sizeof(privateDiagnosticHistorySlot{}) == 528 && unsafe.Alignof(privateDiagnosticHistorySlot{}) == 8 &&
		unsafe.Sizeof(privateDiagnosticHistory{}) == 542744 && unsafe.Alignof(privateDiagnosticHistory{}) == 8 &&
		unsafe.Sizeof(privateDiagnosticHistoryRef{}) == 16 && unsafe.Alignof(privateDiagnosticHistoryRef{}) == 8 &&
		unsafe.Sizeof(DiagnosticOperationHandle{}) == 72 && unsafe.Alignof(DiagnosticOperationHandle{}) == 8
}

func managerPrivateDiagnosticHistoryTableBytes() uint64 {
	return uint64(unsafe.Sizeof(privateDiagnosticHistory{}))
}

func (h *privateDiagnosticHistory) reserveSlot(sequence uint64) (privateDiagnosticHistoryRef, session.Failure) {
	if h == nil || h.backingReleased || h.count >= managerPrivateDiagnosticHistorySlots || h.nextGeneration == ^uint64(0) || sequence == 0 {
		return privateDiagnosticHistoryRef{}, session.ResourceExhausted
	}
	slot := -1
	for i := range h.slots {
		if !h.slots[i].active {
			slot = i
			break
		}
	}
	if slot < 0 {
		return privateDiagnosticHistoryRef{}, session.ResourceExhausted
	}
	h.nextGeneration++
	h.slots[slot] = privateDiagnosticHistorySlot{generation: h.nextGeneration, sequence: sequence, active: true}
	tail := (int(h.head) + int(h.count)) % managerPrivateDiagnosticHistorySlots
	h.order[tail] = uint16(slot)
	h.count++
	return privateDiagnosticHistoryRef{generation: h.nextGeneration, slot: uint16(slot), qualified: true}, ""
}

func (h *privateDiagnosticHistory) resolve(ref privateDiagnosticHistoryRef, sequence uint64) *privateDiagnosticHistorySlot {
	if h == nil || !ref.qualified || int(ref.slot) >= len(h.slots) {
		return nil
	}
	s := &h.slots[ref.slot]
	if !s.active || s.generation != ref.generation || s.sequence != sequence {
		return nil
	}
	return s
}

func (h *privateDiagnosticHistory) oldestClosed(exclude privateDiagnosticHistoryRef) (privateDiagnosticHistoryRef, bool) {
	if h == nil {
		return privateDiagnosticHistoryRef{}, false
	}
	for i := 0; i < int(h.count); i++ {
		idx := (int(h.head) + i) % managerPrivateDiagnosticHistorySlots
		slot := h.order[idx]
		s := &h.slots[slot]
		if s.active && s.closed && (!exclude.qualified || slot != exclude.slot || s.generation != exclude.generation) {
			return privateDiagnosticHistoryRef{generation: s.generation, slot: slot, qualified: true}, true
		}
	}
	return privateDiagnosticHistoryRef{}, false
}

func (h *privateDiagnosticHistory) releaseSlot(ref privateDiagnosticHistoryRef) bool {
	if h == nil || !ref.qualified || int(ref.slot) >= len(h.slots) {
		return false
	}
	s := h.resolve(ref, h.slots[ref.slot].sequence)
	if s == nil {
		return false
	}
	position := -1
	for i := 0; i < int(h.count); i++ {
		idx := (int(h.head) + i) % managerPrivateDiagnosticHistorySlots
		if h.order[idx] == ref.slot {
			position = i
			break
		}
	}
	if position < 0 {
		return false
	}
	s.metadataSource.release()
	s.retainedBacking.release()
	*s = privateDiagnosticHistorySlot{}
	if position == 0 {
		h.order[h.head] = 0
		h.head = uint16((int(h.head) + 1) % managerPrivateDiagnosticHistorySlots)
	} else {
		for i := position; i < int(h.count)-1; i++ {
			from := (int(h.head) + i + 1) % managerPrivateDiagnosticHistorySlots
			to := (int(h.head) + i) % managerPrivateDiagnosticHistorySlots
			h.order[to] = h.order[from]
		}
		last := (int(h.head) + int(h.count) - 1) % managerPrivateDiagnosticHistorySlots
		h.order[last] = 0
	}
	h.count--
	if h.count == 0 {
		h.head = 0
	}
	return true
}
