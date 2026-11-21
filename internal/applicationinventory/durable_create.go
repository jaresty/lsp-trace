//go:build darwin || linux

package applicationinventory

import (
	"bytes"
	"context"
	"encoding/json"
	is "lsp-trace/internal/inventorysuccessor"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/scopejob"
	"path/filepath"
	"reflect"
	"time"
)

// This is the existing inventory head's bootstrap phase, not a second journal
// or an executable source authority. Config remains self-contained if its later
// immutable object publication fails. Only this in-flight Create can finish.
type ownedCreationIntent struct {
	Model  string
	Config OwnedInventoryConfig
}

const ownedCreateMinimum uint64 = 32*(ownedBootstrapMaxBytes+16384) + 32768

// Logical byte/work model, NOT physical RSS or disk quota. Config pays eight
// traversals for canonical bytes, copying, hashing, write and verified readback.
// Each of two head publications pays 32 traversals of its bounded wire size:
// canonical/envelope encodings, hashes/copies, CAS predecessor + temporary write
// verification + committed readback, and bounded failure/status inspection.
// 32768 covers fixed IDs, descriptors and scalar bookkeeping. All multiplication
// is range-checked before use; no future admission or semantic work is included.
func ownedCreationReserve(c OwnedInventoryConfig) uint64 {
	if c.Schema == is.InventoryResumeConfigVersionV1 {
		return enumerationFixed + scopejob.BatchBytes + 2*(cat.InventoryNativeControlBytesV2+is.InventorySelectedControlForHeadV1(is.InventoryResumeHeadVersionV1))
	}
	reserve := c.AdmissionBytes + ownedSealCost
	if c.Enumeration != nil {
		reserve = max(reserve, enumerationFixed+scopejob.BatchBytes)
	}
	if c.NativeProfile != nil {
		reserve += is.InventoryNativeControlBytesV1
	}
	if c.SelectedProfile != nil {
		reserve += cat.InventorySelectedControlBytesV1 + cat.InventoryAdmissionBytesV4
	}
	return reserve
}
func ownedCreationQuote(configBytes int) (uint64, error) {
	if configBytes < 1 || configBytes > ownedConfigMaxBytes {
		return 0, ErrDurableInventory
	}
	n := uint64(configBytes)
	if ownedCreateMinimum > durableRunLimit || n > (durableRunLimit-ownedCreateMinimum)/8 {
		return 0, ErrDurableInventory
	}
	return ownedCreateMinimum + 8*n, nil
}
func inventoryHeadLimit(h durableInventoryHead) int {
	if (h.Schema == ownedHeadVersion || h.Schema == is.InventoryNativeHeadVersionV1 || h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1) && h.Phase == "Creating" {
		return ownedBootstrapMaxBytes
	}
	return 16384
}
func ownedConfigReference(raw []byte) publication.StoredObject {
	return publication.StoredObject{ID: cat.NativeDigestV3(raw)[7:], Bytes: len(raw)}
}
func ownedCreationValid(h durableInventoryHead) bool {
	if (h.Schema != ownedHeadVersion && h.Schema != is.InventoryNativeHeadVersionV1 && h.Schema != is.InventorySelectedHeadVersionV1 && h.Schema != is.InventoryResumeHeadVersionV1) || (h.Schema == is.InventoryNativeHeadVersionV1 || h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1) != (h.Creation != nil && h.Creation.Config.NativeProfile != nil) || (h.Schema == is.InventorySelectedHeadVersionV1 || h.Schema == is.InventoryResumeHeadVersionV1) != (h.Creation != nil && h.Creation.Config.SelectedProfile != nil) || h.Phase != "Creating" || h.Status != "CLAIMED" || h.Creation == nil || h.Creation.Model != ownedCreateModel || !validDurableID(h.RunID) || !ownedConfigValid(h.Creation.Config) || h.Issued <= 0 || h.Deadline <= h.Issued || h.Deadline-h.Issued > int64(h.Creation.Config.OperationWindow) {
		return false
	}
	raw, e := json.Marshal(h.Creation.Config)
	if e != nil {
		return false
	}
	quote, e := ownedCreationQuote(len(raw))
	if e != nil || quote != h.Used || h.Limit != h.Creation.Config.BudgetBytes || quote > h.Limit || ownedCreationReserve(h.Creation.Config) > h.Limit-quote || h.State != ownedConfigReference(raw) {
		return false
	}
	expected := durableInventoryHead{Schema: h.Schema, RunID: h.RunID, TaskID: h.RunID, Phase: "Creating", Status: "CLAIMED", State: h.State, Issued: h.Issued, Deadline: h.Deadline, Used: quote, Work: quote * 32, Packets: 1, Limit: h.Limit, Creation: h.Creation}
	a, _ := json.Marshal(h)
	b, _ := json.Marshal(expected)
	return bytes.Equal(a, b)
}

// fault/run are private deterministic test seams only. Public Create supplies
// neither. There is no retry, rebase, refund, global hook or recovery framework.
func (s DurableStore) createOwned(ctx context.Context, c OwnedInventoryConfig, run string, fault func(string) error) (InventoryJobStatus, error) {
	if c.Schema == "" {
		c.Schema = ownedConfigVersion
	}
	if c.BudgetBytes == 0 {
		c.BudgetBytes = durableRunLimit
	}
	if c.MaxAdmittedEvidenceBytes == 0 {
		c.MaxAdmittedEvidenceBytes = 2 << 30
	}
	if ctx == nil || ctx.Err() != nil || s.Root == nil || !filepath.IsAbs(s.Directory) || len(s.Directory) > 4096 || filepath.Clean(s.Directory) != s.Directory || s.Root.Path() != s.Directory || !ownedConfigValid(c) {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	canonical, e := filepath.EvalSymlinks(s.Directory)
	if e != nil || canonical != s.Directory || s.Root.ValidatePrivate() != nil {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	ctx, cancel := context.WithTimeout(ctx, c.OperationWindow)
	defer cancel()
	raw, e := json.Marshal(c)
	if e != nil {
		return InventoryJobStatus{}, e
	}
	quote, e := ownedCreationQuote(len(raw))
	if e != nil || quote > c.BudgetBytes || ownedCreationReserve(c) > c.BudgetBytes-quote {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	// The self-contained wire config must preserve the validated values exactly;
	// JSON replacement of invalid UTF-8 must not commit an unreadable intent.
	var roundtrip OwnedInventoryConfig
	if strictInventory(raw, &roundtrip) != nil || !reflect.DeepEqual(roundtrip, c) {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	c = roundtrip // freeze caller-owned enumeration slices before any effects
	if run == "" {
		run, e = newDurableID()
		if e != nil {
			return InventoryJobStatus{}, e
		}
	}
	if !validDurableID(run) {
		return InventoryJobStatus{}, ErrDurableInventory
	}
	deadline, _ := ctx.Deadline()
	schema := ownedHeadVersion
	if c.NativeProfile != nil {
		schema = is.InventoryNativeHeadVersionV1
	}
	if c.SelectedProfile != nil {
		schema = is.InventorySelectedHeadVersionV1
	}
	if c.Schema == is.InventoryResumeConfigVersionV1 {
		schema = is.InventoryResumeHeadVersionV1
	}
	h := durableInventoryHead{Schema: schema, RunID: run, TaskID: run, Phase: "Creating", Status: "CLAIMED", State: ownedConfigReference(raw), Issued: time.Now().UnixNano(), Deadline: deadline.UnixNano(), Used: quote, Work: quote * 32, Packets: 1, Limit: c.BudgetBytes, Creation: &ownedCreationIntent{Model: ownedCreateModel, Config: c}}
	fail := func(err error) (InventoryJobStatus, error) {
		// Bounded administrative readback, not execution or a renewed lease. A failed
		// initial CAS may have committed; never discard its discoverable RunID.
		status, readErr := s.JobStatus(context.Background(), run)
		if readErr == nil {
			return status, err
		}
		return InventoryJobStatus{Schema: "lsp-trace.inventory-job-status.v1", RunID: run, Phase: "Creating", Status: "CREATION_UNCONFIRMED", JobExpiry: "NONE"}, err
	}
	check := func(stage string) error {
		if fault != nil {
			if e := fault(stage); e != nil {
				return e
			}
		}
		return ctx.Err()
	}
	// No config/object effects precede this context-checked, self-contained claim.
	if e = check("before-claim"); e != nil {
		return InventoryJobStatus{}, e
	}
	claimed, e := s.change(ctx, nil, h)
	if e != nil {
		return fail(e)
	}
	if e = check("after-claim"); e != nil {
		return fail(e)
	}
	if e = check("before-config"); e != nil {
		return fail(e)
	}
	ref, e := publication.StoreObject(s.Root, "inventory-owned-config-", raw)
	if e != nil {
		return fail(e)
	}
	if ref != h.State {
		return fail(ErrDurableInventory)
	}
	if e = check("after-config"); e != nil {
		return fail(e)
	}
	h.Phase = "Collecting"
	if c.Enumeration != nil {
		h.Phase = "Enumerating"
		h.Enumeration = &ownedEnumeration{Policy: c.ResourcePolicy, Config: h.State}
	}
	h.Status = "READY"
	h.TaskID = ""
	h.Deadline = 0
	h.Creation = nil
	if e = check("before-ready"); e != nil {
		return fail(e)
	}
	if _, e = s.change(ctx, claimed, h); e != nil {
		return fail(e)
	}
	status, e := s.JobStatus(ctx, run)
	if e != nil {
		return fail(e)
	}
	return status, nil
}
