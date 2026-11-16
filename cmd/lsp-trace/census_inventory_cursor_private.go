package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/seedformat"
	"net/url"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"time"

	"lsp-trace/internal/censusacquisition"
	continuation "lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourceplan"
)

// V1 had no partition custody; fail closed rather than migrating prefix-zero
// state that may represent an already executed but unreceipted discovery.
const censusInventoryCursorSchema = "private-connected-inventory-cursor.v2"

// Preserve the original selected producer's seven-slot schedule order.
var censusInventoryModes = [7]string{"RD", "C", "CRD", "D", "CD", "R", "CR"}

type censusInventoryHeader struct {
	Schema, SourceRevision, Workspace, PlanIdentity, Workflow, Scope, Provenance string
	Plan                                                                         censussourceplan.Plan
	UntouchedGaps                                                                []string
	Exclusions                                                                   []string
	QualifiedAdmission                                                           *censusInventoryQualifiedAdmission `json:",omitempty"`
}
type censusInventoryModeState struct {
	Mode, State, QDigest, QSelector, RunID, Attempt, Predecessor, Publication string
}
type censusInventoryTargetRecord struct {
	Schema, Header, Receipt, SeedDigest string
	Ordinal                             int
	Target                              censusacquisition.PreparedTarget
}
type censusInventoryTargetState struct {
	Record string
	Modes  [7]censusInventoryModeState
}
type censusInventoryIndexEntry struct {
	Ordinal int
	Receipt string
	Targets []string
}
type censusInventoryDenominatorIndex struct {
	Schema, Header, Previous string
	Entries                  []censusInventoryIndexEntry
}
type censusInventoryCursor struct {
	Schema, Selector, Predecessor, Header, IndexRecord string
	Generation                                         uint64
	IndexedPrefix                                      int
	Index                                              []censusInventoryIndexEntry
	Targets                                            []censusInventoryTargetState
	Partitions                                         []censusInventoryPartitionState
}
type censusInventoryCursorStore struct {
	objects continuation.ImmutableStore
	heads   continuation.HeadStore
	key     string
	root    string
}

func sealCensusInventoryCursor(c censusInventoryCursor) ([]byte, error) {
	c.Selector = ""
	raw, e := censusInventoryMarshal(c, 4<<20)
	if e != nil {
		return nil, e
	}
	c.Selector = censusInventoryDigest(raw)
	return censusInventoryMarshal(c, 4<<20)
}
func parseCensusInventoryCursor(raw []byte) (censusInventoryCursor, error) {
	var c censusInventoryCursor
	if censusInventoryDecode(raw, &c, 4<<20) != nil || c.Schema != censusInventoryCursorSchema || c.Generation == 0 || c.Generation == ^uint64(0) || !censusInventoryHash(c.Header) || c.Generation == 1 && c.Predecessor != "" || c.Generation > 1 && !censusInventoryHash(c.Predecessor) || c.IndexedPrefix != len(c.Index) {
		return c, errCensusInventoryPrivate
	}
	if len(c.Partitions) < c.IndexedPrefix || len(c.Partitions) > c.IndexedPrefix+1 {
		return c, errCensusInventoryPrivate
	}
	for i, p := range c.Partitions {
		if !censusInventoryHash(p.Claim) || i < c.IndexedPrefix && (p.State != "INDEXED" || p.Receipt != c.Index[i].Receipt) || i == c.IndexedPrefix && p.State != "CLAIMED" || p.Receipt != "" && !censusInventoryHash(p.Receipt) {
			return c, errCensusInventoryPrivate
		}
	}
	canonical, e := sealCensusInventoryCursor(c)
	if e != nil || !bytes.Equal(canonical, raw) {
		return c, errCensusInventoryPrivate
	}
	seen := map[string]bool{}
	var ordered []string
	for i, x := range c.Index {
		if x.Ordinal != i || !censusInventoryHash(x.Receipt) {
			return c, errCensusInventoryPrivate
		}
		for _, id := range x.Targets {
			if !censusInventoryHash(id) || seen[id] {
				return c, errCensusInventoryPrivate
			}
			seen[id] = true
			ordered = append(ordered, id)
		}
	}
	if len(seen) != len(c.Targets) || c.IndexedPrefix > 0 && !censusInventoryHash(c.IndexRecord) || c.IndexedPrefix == 0 && c.IndexRecord != "" {
		return c, errCensusInventoryPrivate
	}
	for position, t := range c.Targets {
		if t.Record != ordered[position] || !seen[t.Record] {
			return c, errCensusInventoryPrivate
		}
		delete(seen, t.Record)
		for i, m := range t.Modes {
			if m.Mode != censusInventoryModes[i] {
				return c, errCensusInventoryPrivate
			}
			switch m.State {
			case "PENDING", "AVAILABLE":
				if m.QDigest != "" || m.QSelector != "" || m.RunID != "" || m.Attempt != "" || m.Predecessor != "" || m.Publication != "" {
					return c, errCensusInventoryPrivate
				}
			case "CONFIG_FROZEN", "CLAIMED", "COMPLETED", "INDETERMINATE":
				if !censusInventoryHash(m.QDigest) || !censusInventoryHash(m.QSelector) || !censusInventoryHash(m.Attempt) || !censusInventoryHash(m.Predecessor) {
					return c, errCensusInventoryPrivate
				}
				if m.State == "CONFIG_FROZEN" && m.RunID != "" && !censusInventoryHash(m.RunID) || m.State != "CONFIG_FROZEN" && m.State != "INDETERMINATE" && !censusInventoryHash(m.RunID) || m.State == "INDETERMINATE" && m.RunID != "" && !censusInventoryHash(m.RunID) {
					return c, errCensusInventoryPrivate
				}
				if m.State == "COMPLETED" && !censusInventoryHash(m.Publication) || m.State != "COMPLETED" && m.Publication != "" {
					return c, errCensusInventoryPrivate
				}
			default:
				return c, errCensusInventoryPrivate
			}
		}
	}
	return c, nil
}
func censusInventoryCursorRevision(raw []byte) (continuation.HeadRevision, error) {
	c, e := parseCensusInventoryCursor(raw)
	return continuation.HeadRevision{Generation: c.Generation, Selector: c.Selector}, e
}
func (s censusInventoryCursorStore) load(ctx context.Context) (censusInventoryCursor, error) {
	if censusInventoryCheckContext(ctx) != nil || s.objects == nil || s.heads == nil || s.key == "" {
		return censusInventoryCursor{}, errCensusInventoryPrivate
	}
	raw, e := s.heads.Load(ctx, s.key)
	if e != nil {
		return censusInventoryCursor{}, e
	}
	if len(raw) == 0 {
		return censusInventoryCursor{}, nil
	}
	c, e := parseCensusInventoryCursor(raw)
	if e != nil {
		return c, e
	}
	header, e := s.objects.Get(ctx, c.Header)
	if e != nil || len(header) > 4<<20 || censusInventoryDigest(header) != c.Header {
		return c, errCensusInventoryPrivate
	}
	var h censusInventoryHeader
	if censusInventoryDecode(header, &h, 4<<20) != nil {
		return c, errCensusInventoryPrivate
	}
	_, canonical, headerErr := canonicalCensusInventoryHeader(h)
	if headerErr != nil || !bytes.Equal(header, canonical) || h.Workflow != s.key || c.IndexedPrefix > len(h.Plan.Partitions) {
		return c, errCensusInventoryPrivate
	}
	for i, p := range c.Partitions {
		var claim censusInventoryPartitionClaim
		raw, e := s.objects.Get(ctx, p.Claim)
		if !censusInventoryPartitionAvailable(h, i) || e != nil || censusInventoryDigest(raw) != p.Claim || censusInventoryDecode(raw, &claim, 64<<10) != nil || claim.Schema != "private-connected-partition-claim.v1" || claim.Header != c.Header || claim.Root != s.root || claim.Workflow != s.key || claim.Plan != h.Plan.Identity || claim.Ordinal != i || i >= len(h.Plan.Partitions) || !censusInventoryHash(claim.Members) || claim.Members != censusInventoryMembersDigest(h.Plan.Partitions[i].Members) || !censusInventoryHash(claim.FundingJob) || !censusInventoryHash(claim.FundingHead) || !censusInventoryHash(claim.Attempt) || !censusInventoryHash(claim.CursorPredecessor) || claim.Deadline.IsZero() || i == 0 && claim.SourceHead != (continuation.HeadRevision{}) || i > 0 && (claim.SourceHead.Generation == 0 || !censusInventoryHash(claim.SourceHead.Selector)) {
			return c, errCensusInventoryPrivate
		}
	}
	if c.IndexedPrefix > 0 {
		raw, e := s.objects.Get(ctx, c.IndexRecord)
		var index censusInventoryDenominatorIndex
		if e != nil || censusInventoryDigest(raw) != c.IndexRecord || censusInventoryDecode(raw, &index, 4<<20) != nil || index.Schema != "private-connected-inventory-index.v1" || index.Header != c.Header || !reflect.DeepEqual(index.Entries, c.Index) {
			return c, errCensusInventoryPrivate
		}
	}
	for _, t := range c.Targets {
		raw, e := s.objects.Get(ctx, t.Record)
		if e != nil || len(raw) > 4<<20 || censusInventoryDigest(raw) != t.Record {
			return c, errCensusInventoryPrivate
		}
		var r censusInventoryTargetRecord
		if censusInventoryDecode(raw, &r, 4<<20) != nil || r.Schema != "private-connected-inventory-target.v1" || r.Header != c.Header || r.Ordinal < 0 || r.Ordinal >= c.IndexedPrefix || r.Receipt != c.Index[r.Ordinal].Receipt || validateCensusInventoryTarget(r.Target, h, r.Ordinal) != nil {
			return c, errCensusInventoryPrivate
		}
		seed, e := censusInventoryMarshal(r.Target, 1<<20)
		if e != nil || len(seed) == 0 || censusInventoryDigest(r.Target.CanonicalSeedV2) != r.SeedDigest {
			return c, errCensusInventoryPrivate
		}
	}
	return c, nil
}
func (s censusInventoryCursorStore) replace(ctx context.Context, old, next censusInventoryCursor) (censusInventoryCursor, error) {
	expected := continuation.HeadRevision{Generation: old.Generation, Selector: old.Selector}
	if old.Generation == ^uint64(0)-1 {
		return old, errCensusInventoryPrivate
	}
	next.Generation = old.Generation + 1
	next.Predecessor = old.Selector
	next.Schema = censusInventoryCursorSchema
	raw, e := sealCensusInventoryCursor(next)
	if e != nil {
		return old, e
	}
	if _, e = parseCensusInventoryCursor(raw); e != nil {
		return old, e
	}
	result := s.heads.CompareAndSwap(ctx, s.key, expected, raw)
	if result.Err != nil || result.Outcome != continuation.CASCommitted {
		return old, errCensusInventoryPrivate
	}
	actual, e := s.heads.Load(ctx, s.key)
	if e != nil || !bytes.Equal(actual, raw) {
		return old, errCensusInventoryPrivate
	}
	return parseCensusInventoryCursor(actual)
}
func (s censusInventoryCursorStore) put(ctx context.Context, v any) (string, error) {
	raw, e := censusInventoryMarshal(v, 4<<20)
	if e != nil {
		return "", e
	}
	id, e := s.objects.Put(ctx, raw)
	if e != nil || id != censusInventoryDigest(raw) {
		return "", errCensusInventoryPrivate
	}
	read, e := s.objects.Get(ctx, id)
	if e != nil || !bytes.Equal(raw, read) {
		return "", errCensusInventoryPrivate
	}
	return id, nil
}
func (s censusInventoryCursorStore) initialize(ctx context.Context, h censusInventoryHeader) (censusInventoryCursor, error) {
	h, _, e := canonicalCensusInventoryHeader(h)
	if e != nil || h.Workflow != s.key {
		return censusInventoryCursor{}, errCensusInventoryPrivate
	}
	id, e := s.put(ctx, h)
	if e != nil {
		return censusInventoryCursor{}, e
	}
	old, e := s.load(ctx)
	if e != nil {
		return old, e
	}
	if old.Generation != 0 {
		if old.Header != id {
			return old, errCensusInventoryPrivate
		}
		return old, nil
	}
	return s.replace(ctx, old, censusInventoryCursor{Header: id})
}
func (s censusInventoryCursorStore) indexTerminal(ctx context.Context, c censusInventoryCursor, r continuation.Result, t continuation.Terminal) (censusInventoryCursor, error) {
	if t.Authority != 0 || t.Ordinal != c.IndexedPrefix || r.Ordinal != t.Ordinal || r.Status != continuation.StatusCommitted || !censusInventoryHash(r.ReceiptSelector) {
		return c, errCensusInventoryPrivate
	}
	entry := censusInventoryIndexEntry{Ordinal: t.Ordinal, Receipt: r.ReceiptSelector}
	if t.Discovery.Targets != nil {
		entry.Targets = make([]string, 0, len(t.Discovery.Targets))
		if c.Targets == nil {
			c.Targets = make([]censusInventoryTargetState, 0)
		}
	}
	headerRaw, e := s.objects.Get(ctx, c.Header)
	var header censusInventoryHeader
	if e != nil || censusInventoryDecode(headerRaw, &header, 4<<20) != nil {
		return c, errCensusInventoryPrivate
	}
	_, canonical, headerErr := canonicalCensusInventoryHeader(header)
	if headerErr != nil || !bytes.Equal(headerRaw, canonical) || censusInventoryDigest(canonical) != c.Header || !censusInventoryPartitionAvailable(header, t.Ordinal) {
		return c, errCensusInventoryPrivate
	}
	for _, target := range t.Discovery.Targets {
		if validateCensusInventoryTarget(target, header, t.Ordinal) != nil {
			return c, errCensusInventoryPrivate
		}
		raw, e := censusInventoryMarshal(target, 1<<20)
		if e != nil || len(raw) == 0 {
			return c, e
		}
		record := censusInventoryTargetRecord{Schema: "private-connected-inventory-target.v1", Header: c.Header, Receipt: r.ReceiptSelector, Ordinal: t.Ordinal, Target: target, SeedDigest: censusInventoryDigest(target.CanonicalSeedV2)}
		id, e := s.put(ctx, record)
		if e != nil {
			return c, e
		}
		entry.Targets = append(entry.Targets, id)
		state := censusInventoryTargetState{Record: id}
		for i, mode := range censusInventoryModes {
			state.Modes[i] = censusInventoryModeState{Mode: mode, State: "PENDING"}
		}
		c.Targets = append(c.Targets, state)
	}
	old, e := s.load(ctx)
	if e != nil || old.Selector != c.Selector {
		return c, errCensusInventoryPrivate
	}
	if len(c.Partitions) != c.IndexedPrefix+1 || c.Partitions[c.IndexedPrefix].State != "CLAIMED" || c.Partitions[c.IndexedPrefix].Receipt != r.ReceiptSelector {
		return c, errCensusInventoryPrivate
	}
	c.Partitions = append([]censusInventoryPartitionState(nil), c.Partitions...)
	c.Partitions[c.IndexedPrefix].State = "INDEXED"
	c.Index = append(c.Index, entry)
	c.IndexedPrefix++
	c.IndexRecord, e = s.put(ctx, censusInventoryDenominatorIndex{Schema: "private-connected-inventory-index.v1", Header: c.Header, Previous: old.IndexRecord, Entries: c.Index})
	if e != nil {
		return c, e
	}
	return s.replace(ctx, old, c)
}
func (s censusInventoryCursorStore) transition(ctx context.Context, c censusInventoryCursor, target, mode int, from string, next censusInventoryModeState) (censusInventoryCursor, error) {
	if target < 0 || target >= len(c.Targets) || mode < 0 || mode >= 7 || c.Targets[target].Modes[mode].State != from || next.Mode != censusInventoryModes[mode] {
		return c, errCensusInventoryPrivate
	}
	// Deep-copy before editing: a failed/CAS-uncertain transition must not mutate
	// caller snapshots or the predecessor supplied to another coordinator.
	raw, e := json.Marshal(c)
	if e != nil || len(raw) > 4<<20 {
		return c, errCensusInventoryPrivate
	}
	var n censusInventoryCursor
	if json.Unmarshal(raw, &n) != nil {
		return c, errCensusInventoryPrivate
	}
	old := c.Targets[target].Modes[mode]
	switch from + "/" + next.State {
	case "PENDING/AVAILABLE":
	case "AVAILABLE/CONFIG_FROZEN":
		if next.Predecessor != c.Selector {
			return c, errCensusInventoryPrivate
		}
	case "CONFIG_FROZEN/CONFIG_FROZEN":
		copy := next
		copy.RunID = old.RunID
		if old.RunID != "" || !censusInventoryHash(next.RunID) || !reflect.DeepEqual(copy, old) {
			return c, errCensusInventoryPrivate
		}
	case "CONFIG_FROZEN/INDETERMINATE":
		copy := next
		copy.State = old.State
		if old.RunID != "" || !reflect.DeepEqual(copy, old) {
			return c, errCensusInventoryPrivate
		}
	case "CONFIG_FROZEN/CLAIMED":
		if old.RunID == "" || old.RunID != next.RunID || old.QDigest != next.QDigest || old.QSelector != next.QSelector || old.Attempt != next.Attempt || old.Predecessor != next.Predecessor {
			return c, errCensusInventoryPrivate
		}
	case "CLAIMED/COMPLETED", "CLAIMED/INDETERMINATE":
		copy := next
		copy.State = old.State
		copy.Publication = old.Publication
		if !reflect.DeepEqual(copy, old) {
			return c, errCensusInventoryPrivate
		}
	default:
		return c, errCensusInventoryPrivate
	}
	n.Targets[target].Modes[mode] = next
	return s.replace(ctx, c, n)
}

// Recovery reads the exact latest linked terminal before another discovery step.
// It does not advance continuation custody, and never rediscovers an indexed
// target. A claimed receipt-less head remains indeterminate with no retry.
func (s censusInventoryCursorStore) recoverTerminal(ctx context.Context, c censusInventoryCursor, request continuation.Request) (censusInventoryCursor, error) {
	raw, e := request.Heads.Load(ctx, request.HeadKey)
	if e != nil {
		return c, e
	}
	inspected, e := continuation.InspectHead(raw, request.WorkflowKey, request.Plan.Identity, len(request.Plan.Partitions))
	if e != nil {
		return c, e
	}
	if inspected.State == continuation.HeadStateAbsent {
		if c.IndexedPrefix != 0 || len(c.Partitions) != 0 {
			return c, errCensusInventoryPartitionBlocked
		}
		return c, nil
	}
	request.Expected = inspected.Revision
	result, t, e := continuation.ReadHeadTerminal(ctx, request)
	if e != nil || result.Status == continuation.StatusIndeterminateExecution || result.Status == continuation.StatusRetained {
		return c, errCensusInventoryPrivate
	}
	if inspected.CommittedPrefix == c.IndexedPrefix {
		if c.IndexedPrefix > 0 && c.Index[c.IndexedPrefix-1].Receipt != result.ReceiptSelector {
			return c, errCensusInventoryPrivate
		}
		if len(c.Partitions) > c.IndexedPrefix {
			return c, errCensusInventoryPartitionBlocked
		}
		return c, nil
	}
	if t == nil || inspected.CommittedPrefix != c.IndexedPrefix+1 {
		return c, errCensusInventoryPrivate
	}
	return s.indexTerminal(ctx, c, result, *t)
}

var errCensusInventoryPartitionBlocked = errors.New("SOURCE_PARTITION_INDETERMINATE_NO_REPLAY")

type censusInventoryPartitionState struct{ Claim, State, Receipt string }
type censusInventoryPartitionClaim struct {
	Schema, Header, Root, Workflow, Plan, Members, FundingJob, FundingHead, Attempt, CursorPredecessor string
	Ordinal                                                                                            int
	SourceHead                                                                                         continuation.HeadRevision
	Deadline                                                                                           time.Time
}

// A parsed claim is never a permit. Only a successful CAS and exact readback
// constructs this private, invocation-local shared-on-copy one-shot state.
type censusInventoryPartitionPermit struct {
	store           censusInventoryCursorStore
	claim, selector string
	ordinal         int
	members         string
	deadline        time.Time
	used            *atomic.Bool
}

func censusInventoryMembersDigest(files []censusacquisition.SourceFile) string {
	raw, e := censusInventoryMarshal(files, 4<<20)
	if e != nil {
		return ""
	}
	return censusInventoryDigest(raw)
}
func (s censusInventoryCursorStore) claimPartition(ctx context.Context, c censusInventoryCursor, cfg censusInventoryBridgeConfig, r continuation.Request) (censusInventoryCursor, *censusInventoryPartitionPermit, error) {
	if censusInventoryCheckContext(ctx) != nil || !time.Now().Before(cfg.Deadline) || time.Until(cfg.Deadline) > 15*time.Second || len(c.Partitions) != c.IndexedPrefix || c.IndexedPrefix >= len(r.Plan.Partitions) || cfg.Root != s.root || r.WorkflowKey != s.key || r.HeadKey != s.key || r.Plan.Identity != cfg.Header.Plan.Identity || !censusInventoryHash(cfg.ExpectedHead) || !censusInventoryHash(cfg.Attempt) {
		return c, nil, errCensusInventoryPartitionBlocked
	}
	_, header, e := canonicalCensusInventoryHeader(cfg.Header)
	if e != nil || censusInventoryDigest(header) != c.Header || !censusInventoryPartitionAvailable(cfg.Header, c.IndexedPrefix) || !reflect.DeepEqual(r.Plan, cfg.Header.Plan) {
		return c, nil, errCensusInventoryPrivate
	}
	raw, e := r.Heads.Load(ctx, r.HeadKey)
	if e != nil {
		return c, nil, e
	}
	head, e := continuation.InspectHead(raw, r.WorkflowKey, r.Plan.Identity, len(r.Plan.Partitions))
	if e != nil || head.CommittedPrefix != c.IndexedPrefix || head.State != continuation.HeadStateAbsent && head.State != continuation.HeadStateCommitted {
		return c, nil, errCensusInventoryPartitionBlocked
	}
	members := censusInventoryMembersDigest(r.Plan.Partitions[c.IndexedPrefix].Members)
	if !censusInventoryHash(members) {
		return c, nil, errCensusInventoryPrivate
	}
	claim := censusInventoryPartitionClaim{Schema: "private-connected-partition-claim.v1", Header: c.Header, Root: s.root, Workflow: s.key, Plan: r.Plan.Identity, Members: members, FundingJob: censusInventoryDigest(cfg.CanonicalJob), FundingHead: cfg.ExpectedHead, Attempt: cfg.Attempt, CursorPredecessor: c.Selector, Ordinal: c.IndexedPrefix, SourceHead: head.Revision, Deadline: cfg.Deadline}
	id, e := s.put(ctx, claim)
	if e != nil {
		return c, nil, e
	}
	next := c
	next.Partitions = append(append([]censusInventoryPartitionState(nil), c.Partitions...), censusInventoryPartitionState{Claim: id, State: "CLAIMED"})
	next, e = s.replace(ctx, c, next)
	if e != nil {
		return c, nil, e
	}
	p := &censusInventoryPartitionPermit{store: s, claim: id, selector: next.Selector, ordinal: c.IndexedPrefix, members: members, deadline: cfg.Deadline, used: &atomic.Bool{}}
	return next, p, nil
}
func (p *censusInventoryPartitionPermit) take(ctx context.Context, o int, files []censusacquisition.SourceFile) error {
	if p == nil || p.used == nil || censusInventoryCheckContext(ctx) != nil || !time.Now().Before(p.deadline) || o != p.ordinal || !censusInventoryHash(p.members) || censusInventoryMembersDigest(files) != p.members || !p.used.CompareAndSwap(false, true) {
		return errCensusInventoryPartitionBlocked
	}
	c, e := p.store.load(ctx)
	if e != nil || c.Selector != p.selector || len(c.Partitions) != o+1 || c.IndexedPrefix != o || c.Partitions[o].Claim != p.claim || c.Partitions[o].State != "CLAIMED" {
		return errCensusInventoryPartitionBlocked
	}
	return nil
}

// Receipt ownership is established by this invocation's guarded publication,
// not by equating the funding attempt with coordinator claim identifiers.
type censusInventoryPartitionHeads struct {
	continuation.HeadStore
	permit   *censusInventoryPartitionPermit
	request  continuation.Request
	expected continuation.HeadRevision
}

func (h *censusInventoryPartitionHeads) CompareAndSwap(ctx context.Context, key string, expected continuation.HeadRevision, raw []byte) continuation.CASResult {
	refuse := func() continuation.CASResult {
		return continuation.CASResult{Outcome: continuation.CASNotCommitted, Err: errCensusInventoryPartitionBlocked}
	}
	p := h.permit
	if p == nil || p.used == nil || key != h.request.HeadKey || expected != h.expected || censusInventoryCheckContext(ctx) != nil || !time.Now().Before(p.deadline) {
		return refuse()
	}
	candidate, e := continuation.InspectHead(raw, h.request.WorkflowKey, h.request.Plan.Identity, len(h.request.Plan.Partitions))
	if e != nil || candidate.Revision.Generation != expected.Generation+1 {
		return refuse()
	}
	c, e := p.store.load(ctx)
	if e != nil || c.IndexedPrefix != p.ordinal || len(c.Partitions) != p.ordinal+1 || c.Partitions[p.ordinal].Claim != p.claim || c.Partitions[p.ordinal].State != "CLAIMED" {
		return refuse()
	}
	if candidate.ReceiptSelector != "" {
		if !p.used.Load() || candidate.ClaimOrdinal != p.ordinal || c.Partitions[p.ordinal].Receipt != "" && c.Partitions[p.ordinal].Receipt != candidate.ReceiptSelector {
			return refuse()
		}
		if c.Partitions[p.ordinal].Receipt == "" {
			next := c
			next.Partitions = append([]censusInventoryPartitionState(nil), c.Partitions...)
			next.Partitions[p.ordinal].Receipt = candidate.ReceiptSelector
			if _, e = p.store.replace(ctx, c, next); e != nil {
				return refuse()
			}
		}
	}
	result := h.HeadStore.CompareAndSwap(ctx, key, expected, raw)
	if result.Err != nil || result.Outcome != continuation.CASCommitted {
		return result
	}
	read, e := h.HeadStore.Load(ctx, key)
	if e != nil || !bytes.Equal(read, raw) {
		return continuation.CASResult{Outcome: continuation.CASCommitUncertain, Err: errCensusInventoryPartitionBlocked}
	}
	h.expected = candidate.Revision
	return result
}
func validateCensusInventoryTarget(t censusacquisition.PreparedTarget, h censusInventoryHeader, ordinal int) error {
	if ordinal < 0 || ordinal >= len(h.Plan.Partitions) || t.CensusOrdinal < 0 || len(t.CanonicalSeedV2) == 0 || len(t.CanonicalSeedV2) > 64<<10 || len(t.URI) > 4096 || len(t.Name) > 4096 || len(t.SymbolIdentity) == 0 || len(t.SymbolIdentity) > 4096 || t.Kind <= 0 {
		return errCensusInventoryPrivate
	}
	seed, e := seedformat.Decode(t.CanonicalSeedV2, h.Workspace)
	if e != nil || len(seed.Seeds) != 1 {
		return errCensusInventoryPrivate
	}
	canonical, e := seedformat.EncodeCanonical(seed, h.Workspace)
	if e != nil || !bytes.Equal(canonical, t.CanonicalSeedV2) {
		return errCensusInventoryPrivate
	}
	value := seed.Seeds[0]
	p := value.Position
	if value.Type != seedformat.PositionType || p == nil || p.Line != uint64(t.SelectionRange.Start.Line)+1 || p.Column != uint64(t.SelectionRange.Start.Character)+1 || p.Label != fmt.Sprintf("census-%06d", t.CensusOrdinal) {
		return errCensusInventoryPrivate
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(h.Workspace, filepath.FromSlash(p.Path)))}).String()
	if uri != t.URI {
		return errCensusInventoryPrivate
	}
	for _, f := range h.Plan.Partitions[ordinal].Members {
		if f.URI == t.URI && f.LanguageID == t.LanguageID {
			return nil
		}
	}
	return errCensusInventoryPrivate
}
