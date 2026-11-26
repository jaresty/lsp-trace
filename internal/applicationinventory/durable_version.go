//go:build darwin || linux

package applicationinventory

import (
	"encoding/json"
	"errors"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
)

// ErrUnsupportedStoredVersion identifies a recognized but non-executable stored
// representation. It confers no permission to migrate, retry or reacquire.
var ErrUnsupportedStoredVersion = errors.New("unsupported stored inventory version")

const durableHeadVersion = "lsp-trace.durable-inventory-head.v2"
const durableQuoteVersion = "lsp-trace.inventory-completion-quote.v2"

// Recognition only: these frozen pre-delivery shapes never become a runnable
// head, selected capability or publication reader. Canonical decoding is kept
// separate so missing new fields cannot be accepted as the current format.
type inventoryQuoteV1 struct {
	Version                     string
	Stages                      []uint64
	StateUpper, PreparationUsed uint64
}
type inventoryHeadV1 struct {
	Schema, RunID, TaskID, PreviousTaskID, Phase, Status string
	Sequence                                             int
	State, Input, Report, Manifest                       publication.StoredObject
	InputPrefix, Role, Actor, Prompt                     string
	Issued, RunDeadline, Deadline, Window, SemanticNS    int64
	Used, Work, Packets, Limit, OutputBytes              uint64
	Acceptance                                           string
	AcceptanceDeadline                                   int64
	Completion                                           inventoryQuoteV1
	CompletionCursor                                     int
}

func decodeVersionedInventoryHead(raw []byte, out *durableInventoryHead) error {
	var fields map[string]json.RawMessage
	if cat.DecodeNativeV3(raw, &fields) != nil {
		return ErrDurableInventory
	}
	var version string
	if json.Unmarshal(fields["Schema"], &version) != nil {
		return ErrDurableInventory
	}
	if version != is.InventorySemanticHeadVersionV1 {
		if _, ok := fields["SemanticPolicy"]; ok {
			return ErrDurableInventory
		}
		if _, ok := fields["TaskIssuedNS"]; ok {
			return ErrDurableInventory
		}
	}
	if _, present := fields["Native"]; present && version != is.InventoryNativeHeadVersionV1 && version != is.InventorySelectedHeadVersionV1 && !is.InventoryResumableHeadV1(version) {
		return ErrDurableInventory
	}
	if _, present := fields["Checkpoint"]; present && !is.InventoryResumableHeadV1(version) {
		return ErrDurableInventory
	}
	if _, present := fields["Selected"]; present && version != is.InventorySelectedHeadVersionV1 && !is.InventoryResumableHeadV1(version) {
		return ErrDurableInventory
	}
	if _, present := fields["Closure"]; present && !is.InventoryConnectedHead(version) {
		return ErrDurableInventory
	}
	switch version {
	case "lsp-trace.durable-inventory-head.v1":
		// v1 was emitted with two incompatible shapes before explicit versioning.
		// Recognize both strictly, but never execute either.
		var old inventoryHeadV1
		if strictInventory(raw, &old) == nil && old.Completion.Version == "lsp-trace.inventory-completion-quote.v1" && validDurableID(old.RunID) && validDurableID(old.TaskID) && old.State.Valid() && old.Input.Valid() {
			return ErrUnsupportedStoredVersion
		}
		var prior durableInventoryHead
		if strictInventory(raw, &prior) == nil && prior.Completion.Version == "lsp-trace.inventory-completion-quote.v1" && validDurableID(prior.RunID) && validDurableID(prior.TaskID) && prior.State.Valid() && prior.Input.Valid() {
			return ErrUnsupportedStoredVersion
		}
		return ErrDurableInventory
	case "lsp-trace.durable-inventory-head.v3":
		// Unreleased fixed-charge shape is recognized but never migrated or
		// re-costed. Its config RetainedBytes field is not reinterpreted.
		var h durableInventoryHead
		if strictInventory(raw, &h) != nil || h.Creation != nil || h.Enumeration != nil || !validDurableID(h.RunID) || !h.State.Valid() {
			return ErrDurableInventory
		}
		return ErrUnsupportedStoredVersion
	case ownedHeadVersion, is.InventoryNativeHeadVersionV1, is.InventorySelectedHeadVersionV1, is.InventoryResumeHeadVersionV1, is.InventoryConnectedHeadVersionV1, is.InventoryReferenceHeadVersionV1, is.InventorySemanticHeadVersionV1:
		var h durableInventoryHead
		if strictInventory(raw, &h) != nil || (!ownedPhase(h) && h.Completion.Version != durableQuoteVersion) {
			return ErrDurableInventory
		}
		*out = h
		return nil
	case durableHeadVersion:
		var h durableInventoryHead
		if strictInventory(raw, &h) != nil || h.Creation != nil || h.Enumeration != nil {
			return ErrDurableInventory
		}
		switch h.Completion.Version {
		case durableQuoteVersion:
			*out = h
			return nil
		case "lsp-trace.inventory-completion-quote.v1":
			return ErrUnsupportedStoredVersion
		default:
			return ErrDurableInventory
		}
	default:
		// Unknown identifiers are not evidence of a known old representation.
		return ErrDurableInventory
	}
}
