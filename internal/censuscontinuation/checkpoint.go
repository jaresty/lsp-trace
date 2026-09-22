package censuscontinuation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"sync"

	"lsp-trace/internal/strictjson"
)

const (
	CheckpointSchema         = "lsp-trace.census-continuation-checkpoint.v1"
	CompositeSchema          = "lsp-trace.census-continuation-composite.v1"
	ContinuationCompleteness = "UNKNOWN"
)

type Stage string

const (
	StageCensusCommitted   Stage = "CENSUS_COMMITTED"
	StageProgramCComputed  Stage = "PROGRAM_C_COMPUTED"
	StageSnapshotsCaptured Stage = "SNAPSHOTS_CAPTURED"
	StagePacketsPrepared   Stage = "PACKETS_PREPARED"
	StageRequestsRendered  Stage = "REQUESTS_RENDERED"
	StageDescribeAttempts  Stage = "DESCRIBE_ATTEMPTS"
	StageDescribeComplete  Stage = "DESCRIBE_COMPLETE"
	StageCatalogAssembled  Stage = "CATALOG_ASSEMBLED"
	StageCatalogCommitted  Stage = "CATALOG_COMMITTED"
)

var stageOrder = []Stage{StageCensusCommitted, StageProgramCComputed, StageSnapshotsCaptured, StagePacketsPrepared, StageRequestsRendered, StageDescribeAttempts, StageDescribeComplete, StageCatalogAssembled, StageCatalogCommitted}

type ContinuationStatus string

const (
	StatusRunning              ContinuationStatus = "RUNNING"
	StatusComplete             ContinuationStatus = "COMPLETE"
	StatusFailedCapture        ContinuationStatus = "FAILED_CAPTURE"
	StatusFailedPacket         ContinuationStatus = "FAILED_PACKET"
	StatusFailedRender         ContinuationStatus = "FAILED_RENDER"
	StatusFailedWorker         ContinuationStatus = "FAILED_WORKER"
	StatusFailedCatalog        ContinuationStatus = "FAILED_CATALOG"
	StatusCancelledAfterCommit ContinuationStatus = "CANCELLED_AFTER_CENSUS_COMMIT"
)

type ArtifactRef struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Digest     string `json:"digest"`
	ByteLength uint64 `json:"byte_length"`
}
type Diagnostic struct {
	Code         string `json:"code"`
	Stage        Stage  `json:"stage"`
	Category     string `json:"category,omitempty"`
	Observed     int64  `json:"observed,omitempty"`
	Limit        int64  `json:"limit,omitempty"`
	FailedField  string `json:"failed_field,omitempty"`
	Invariant    string `json:"invariant,omitempty"`
	CallerAction string `json:"caller_action,omitempty"`
	Recovery     string `json:"recovery,omitempty"`
}
type checkpointWire struct {
	SchemaVersion     string             `json:"schema_version"`
	CheckpointID      string             `json:"checkpoint_id"`
	PriorCheckpointID string             `json:"prior_checkpoint_id"`
	Stage             Stage              `json:"stage"`
	CensusID          string             `json:"census_id"`
	HandoffID         string             `json:"handoff_id"`
	ContractID        string             `json:"contract_id"`
	ProfileID         string             `json:"profile_id"`
	Artifacts         []ArtifactRef      `json:"artifacts"`
	Authority         int                `json:"authority"`
	Accepted          bool               `json:"accepted"`
	Completeness      string             `json:"completeness"`
	Status            ContinuationStatus `json:"status"`
	Diagnostics       []Diagnostic       `json:"diagnostics"`
}
type Checkpoint struct {
	wire   checkpointWire
	legacy bool
}
type CheckpointInput struct {
	PriorCheckpointID                          string
	Stage                                      Stage
	CensusID, HandoffID, ContractID, ProfileID string
	Artifacts                                  []ArtifactRef
	Status                                     ContinuationStatus
	Diagnostics                                []Diagnostic
}

func NewCheckpoint(in CheckpointInput) (Checkpoint, error) {
	w := checkpointWire{SchemaVersion: CheckpointSchema, PriorCheckpointID: in.PriorCheckpointID, Stage: in.Stage, CensusID: in.CensusID, HandoffID: in.HandoffID, ContractID: in.ContractID, ProfileID: in.ProfileID, Artifacts: append([]ArtifactRef(nil), in.Artifacts...), Completeness: ContinuationCompleteness, Status: in.Status, Diagnostics: append([]Diagnostic(nil), in.Diagnostics...)}
	w.CheckpointID = identityJSON("lsp-trace:census-continuation-checkpoint:v1", w, func(v *checkpointWire) { v.CheckpointID = "" })
	c := Checkpoint{wire: w}
	return c, c.Validate()
}
func (c Checkpoint) ID() string                 { return c.wire.CheckpointID }
func (c Checkpoint) Stage() Stage               { return c.wire.Stage }
func (c Checkpoint) PriorID() string            { return c.wire.PriorCheckpointID }
func (c Checkpoint) Status() ContinuationStatus { return c.wire.Status }
func (c Checkpoint) Artifacts() []ArtifactRef   { return append([]ArtifactRef(nil), c.wire.Artifacts...) }
func (c Checkpoint) CensusID() string           { return c.wire.CensusID }
func (c Checkpoint) HandoffID() string          { return c.wire.HandoffID }
func (c Checkpoint) ContractID() string         { return c.wire.ContractID }
func (c Checkpoint) ProfileID() string          { return c.wire.ProfileID }
func (c Checkpoint) CaptureFailureDiagnostic() (Diagnostic, bool) {
	if c.wire.Status != StatusFailedCapture {
		return Diagnostic{}, false
	}
	for _, d := range c.wire.Diagnostics {
		if d.Code == "CAPTURE" && validCaptureDiagnostic(d) {
			return d, true
		}
	}
	return Diagnostic{}, false
}
func (c Checkpoint) Bytes() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c.wire)
}
func ParseCheckpoint(raw []byte) (Checkpoint, error) {
	var w checkpointWire
	if err := strictCanonical(raw, &w); err != nil {
		return Checkpoint{}, err
	}
	c := Checkpoint{wire: w, legacy: w.Status == StatusFailedCapture && legacyCaptureDiagnostics(w.Diagnostics)}
	return c, c.Validate()
}
func (c Checkpoint) Validate() error {
	w := c.wire
	if w.SchemaVersion != CheckpointSchema || !validDigest(w.CheckpointID) || w.CensusID == "" || !validDigest(w.HandoffID) || !validDigest(w.ContractID) || !validDigest(w.ProfileID) || w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness || stageIndex(w.Stage) < 0 || !validStatus(w.Status) {
		return errors.New("checkpoint integrity mismatch")
	}
	if w.PriorCheckpointID != "" && !validDigest(w.PriorCheckpointID) {
		return errors.New("checkpoint prior identity invalid")
	}
	expected := expectedArtifactKinds(w.Stage)
	if len(w.Artifacts) != len(expected) {
		return errors.New("checkpoint artifact membership mismatch")
	}
	last := ""
	for i, a := range w.Artifacts {
		if a.Kind == "" || a.Kind <= last || a.Kind != expected[i] || a.ID != a.Digest || !validDigest(a.ID) || a.ByteLength == 0 {
			return errors.New("checkpoint artifacts noncanonical")
		}
		last = a.Kind
	}
	for _, d := range w.Diagnostics {
		if d.Code == "" || stageIndex(d.Stage) < 0 {
			return errors.New("checkpoint diagnostic invalid")
		}
	}
	if w.Status == StatusFailedCapture && !c.legacy {
		for _, d := range w.Diagnostics {
			if d.Code == "CAPTURE" && !validCaptureDiagnostic(d) {
				return errors.New("failed capture diagnostic required")
			}
		}
	}
	want := identityJSON("lsp-trace:census-continuation-checkpoint:v1", w, func(v *checkpointWire) { v.CheckpointID = "" })
	if want != w.CheckpointID {
		return errors.New("checkpoint identity mismatch")
	}
	return nil
}
func legacyCaptureDiagnostics(d []Diagnostic) bool {
	return len(d) == 1 && d[0].Code == "CAPTURE" && d[0].Category == "" && d[0].Observed == 0 && d[0].Limit == 0 && d[0].FailedField == "" && d[0].Invariant == "" && d[0].CallerAction == "" && d[0].Recovery == ""
}

func validCaptureDiagnostic(d Diagnostic) bool {
	if d.Code != "CAPTURE" || d.Stage != StageProgramCComputed || d.Recovery != "RESTART_FROM_PRESERVED_CENSUS_COMMIT" {
		return false
	}
	limitCategory := d.Category == "input" || d.Category == "output" || d.Category == "unique_source"
	limitField := d.FailedField == "graph_bytes" || d.FailedField == "artifact_bytes" || d.FailedField == "source_bytes" || d.FailedField == "total_source_bytes" || d.FailedField == "receipts" || d.FailedField == "bindings" || d.FailedField == "outcomes" || d.FailedField == "work"
	if limitCategory && limitField && d.Observed > d.Limit && d.Limit > 0 && d.Invariant == "OBSERVED_MUST_NOT_EXCEED_LIMIT" && d.CallerAction == "INCREASE_BOUNDED_CAPTURE_LIMIT" {
		return true
	}
	if d.Observed != 0 || d.Limit != 0 || d.Invariant != "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT" {
		return false
	}
	switch {
	case d.Category == "availability" && d.FailedField == "source_preparation" && (d.CallerAction == "RETRY_WITH_MANAGED_SOURCE_SUPPLY" || d.CallerAction == "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME"):
		return true
	case d.Category == "internal" && d.FailedField == "resolver" && d.CallerAction == "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION":
		return true
	case d.Category == "internal" && d.FailedField == "serialization" && d.CallerAction == "REPORT_CAPTURE_IMPLEMENTATION":
		return true
	case d.Category == "internal" && d.FailedField == "runtime_injection" && d.CallerAction == "FIX_CAPTURE_RUNTIME_INJECTION":
		return true
	case d.Category == "internal" && d.FailedField == "capture" && d.CallerAction == "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION":
		return true
	}
	return false
}

func validStatus(s ContinuationStatus) bool {
	switch s {
	case StatusRunning, StatusComplete, StatusFailedCapture, StatusFailedPacket, StatusFailedRender, StatusFailedWorker, StatusFailedCatalog, StatusCancelledAfterCommit:
		return true
	}
	return false
}
func expectedArtifactKinds(s Stage) []string {
	kinds := []string{"continuation_contract", "handoff"}
	if stageIndex(s) >= stageIndex(StageProgramCComputed) {
		kinds = append(kinds, "program_c")
	}
	if stageIndex(s) >= stageIndex(StageSnapshotsCaptured) {
		kinds = append(kinds, "snapshots")
	}
	if stageIndex(s) >= stageIndex(StagePacketsPrepared) {
		kinds = append(kinds, "packets")
	}
	if stageIndex(s) >= stageIndex(StageRequestsRendered) {
		kinds = append(kinds, "requests")
	}
	if stageIndex(s) >= stageIndex(StageDescribeAttempts) {
		kinds = append(kinds, "describe_records")
	}
	if stageIndex(s) >= stageIndex(StageCatalogAssembled) {
		kinds = append(kinds, "catalog")
	}
	if stageIndex(s) >= stageIndex(StageCatalogCommitted) {
		kinds = append(kinds, "composite")
	}
	sort.Strings(kinds)
	return kinds
}

func stageIndex(s Stage) int {
	for i, v := range stageOrder {
		if v == s {
			return i
		}
	}
	return -1
}

// Store is an immutable content-addressed byte store. Implementations must clone inputs and outputs.
type Store interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
}
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{objects: map[string][]byte{}} }
func (s *MemoryStore) Put(ctx context.Context, b []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	id := digestOf(b)
	return id, s.PutAt(ctx, id, b)
}
func (s *MemoryStore) PutAt(ctx context.Context, id string, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id != digestOf(b) {
		return errors.New("store digest mismatch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.objects[id]; ok {
		if !bytes.Equal(prior, b) {
			return errors.New("store replacement rejected")
		}
		return nil
	}
	s.objects[id] = append([]byte(nil), b...)
	return nil
}
func (s *MemoryStore) Get(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.objects[id]
	if !ok {
		return nil, errors.New("store object missing")
	}
	return append([]byte(nil), b...), nil
}
func VerifyCheckpoint(ctx context.Context, s Store, c Checkpoint, expectedPrior string) error {
	if s == nil {
		return errors.New("store required")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if c.PriorID() != expectedPrior {
		return errors.New("checkpoint chain mismatch")
	}
	for _, a := range c.Artifacts() {
		b, err := s.Get(ctx, a.ID)
		if err != nil {
			return errors.New("checkpoint artifact missing")
		}
		if digestOf(b) != a.Digest || uint64(len(b)) != a.ByteLength {
			return errors.New("checkpoint artifact mismatch")
		}
	}
	return nil
}

type verificationObjectKey struct {
	selector   string
	digest     string
	byteLength uint64
	kind       string
}

type verificationStore struct {
	store       Store
	objects     map[verificationObjectKey][]byte
	parsed      map[verificationObjectKey]any
	parseCounts map[verificationObjectKey]int
	derived     map[string]any
	selectors   map[string]verificationObjectKey
}

func newVerificationStore(store Store) *verificationStore {
	return &verificationStore{
		store: store, objects: make(map[verificationObjectKey][]byte), parsed: make(map[verificationObjectKey]any),
		parseCounts: make(map[verificationObjectKey]int), derived: make(map[string]any), selectors: make(map[string]verificationObjectKey),
	}
}

func (s *verificationStore) Put(context.Context, []byte) (string, error) {
	return "", errors.New("verification store is read only")
}

func (s *verificationStore) bind(ref ArtifactRef) error {
	key := verificationObjectKey{selector: ref.ID, digest: ref.Digest, byteLength: ref.ByteLength, kind: ref.Kind}
	if prior, ok := s.selectors[ref.ID]; ok && prior != key {
		return errors.New("checkpoint artifact reference conflict")
	}
	s.selectors[ref.ID] = key
	return nil
}

func (s *verificationStore) key(ref ArtifactRef) (verificationObjectKey, error) {
	key, ok := s.selectors[ref.ID]
	if !ok || key != (verificationObjectKey{selector: ref.ID, digest: ref.Digest, byteLength: ref.ByteLength, kind: ref.Kind}) {
		return verificationObjectKey{}, errors.New("checkpoint artifact reference unbound")
	}
	return key, nil
}

func (s *verificationStore) Get(ctx context.Context, selector string) ([]byte, error) {
	key, ok := s.selectors[selector]
	if !ok {
		key = verificationObjectKey{selector: selector, digest: selector, kind: "checkpoint"}
	}
	if raw, ok := s.objects[key]; ok {
		return append([]byte(nil), raw...), nil
	}
	raw, err := s.store.Get(ctx, selector)
	if err != nil {
		return nil, err
	}
	if key.kind == "checkpoint" {
		key.byteLength = uint64(len(raw))
		s.selectors[selector] = key
	}
	if digestOf(raw) != key.digest || uint64(len(raw)) != key.byteLength {
		return nil, errors.New("store object identity mismatch")
	}
	s.objects[key] = append([]byte(nil), raw...)
	return append([]byte(nil), raw...), nil
}

type verificationStats struct {
	ParseCounts map[verificationObjectKey]int
}

func VerifyChain(ctx context.Context, s Store, selector string) ([]Checkpoint, error) {
	return verifyChain(ctx, s, selector, nil)
}

func verifyChain(ctx context.Context, s Store, selector string, stats *verificationStats) ([]Checkpoint, error) {
	operationStore := newVerificationStore(s)
	if stats != nil {
		defer func() {
			stats.ParseCounts = make(map[verificationObjectKey]int, len(operationStore.parseCounts))
			for key, count := range operationStore.parseCounts {
				stats.ParseCounts[key] = count
			}
		}()
	}
	return verifyChainOperation(ctx, operationStore, selector)
}

func verifyChainOperation(ctx context.Context, operationStore *verificationStore, selector string) ([]Checkpoint, error) {
	seen := map[string]bool{}
	var reverse []Checkpoint
	var reverseSelectors []string
	id := selector
	for id != "" {
		if seen[id] {
			return nil, errors.New("checkpoint cycle")
		}
		seen[id] = true
		b, err := operationStore.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if digestOf(b) != id {
			return nil, errors.New("checkpoint selector digest mismatch")
		}
		c, err := ParseCheckpoint(b)
		if err != nil {
			return nil, err
		}
		reverse = append(reverse, c)
		reverseSelectors = append(reverseSelectors, id)
		id = c.PriorID()
	}
	out := make([]Checkpoint, len(reverse))
	selectors := make([]string, len(reverseSelectors))
	for i := range reverse {
		out[len(reverse)-1-i] = reverse[i]
		selectors[len(reverse)-1-i] = reverseSelectors[i]
	}
	for i, c := range out {
		for _, artifact := range c.Artifacts() {
			if err := operationStore.bind(artifact); err != nil {
				return nil, err
			}
		}
		prior := ""
		if i > 0 {
			previous := out[i-1]
			prior = selectors[i-1]
			if !validStageTransition(previous.Stage(), c.Stage()) {
				return nil, errors.New("checkpoint stage transition invalid")
			}
			if c.CensusID() != previous.CensusID() || c.HandoffID() != previous.HandoffID() || c.ContractID() != previous.ContractID() || c.ProfileID() != previous.ProfileID() {
				return nil, errors.New("checkpoint chain identity mismatch")
			}
			if err := verifyAttemptHistoryExtension(ctx, operationStore, previous, c); err != nil {
				return nil, err
			}
		}
		if i == 0 && c.Stage() != StageCensusCommitted {
			return nil, errors.New("checkpoint chain root invalid")
		}
		if err := VerifyCheckpoint(ctx, operationStore, c, prior); err != nil {
			return nil, err
		}
		if err := verifyCheckpointIdentityArtifacts(ctx, operationStore, c); err != nil {
			return nil, err
		}
		if err := verifyCheckpointStageArtifacts(ctx, operationStore, c); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validStageTransition(previous, current Stage) bool {
	previousIndex, currentIndex := stageIndex(previous), stageIndex(current)
	return previousIndex >= 0 && (currentIndex == previousIndex || currentIndex == previousIndex+1)
}

func verifyCheckpointIdentityArtifacts(ctx context.Context, store Store, checkpoint Checkpoint) error {
	var contractFound, handoffFound bool
	for _, artifact := range checkpoint.Artifacts() {
		if artifact.Kind != "continuation_contract" && artifact.Kind != "handoff" {
			continue
		}
		verification, cached := store.(*verificationStore)
		if !cached {
			raw, err := store.Get(ctx, artifact.ID)
			if err != nil {
				return errors.New("checkpoint identity artifact missing")
			}
			switch artifact.Kind {
			case "continuation_contract":
				contract, parseErr := ParseContinuationContract(raw)
				if parseErr != nil || contract.ID() != checkpoint.ContractID() || contract.ProfileID() != checkpoint.ProfileID() {
					return errors.New("checkpoint continuation contract mismatch")
				}
				contractFound = true
			case "handoff":
				handoff, parseErr := Parse(raw)
				if parseErr != nil || handoff.HandoffID() != checkpoint.HandoffID() || handoff.CensusID() != checkpoint.CensusID() {
					return errors.New("checkpoint committed handoff mismatch")
				}
				handoffFound = true
			}
			continue
		}
		parsed, err := verification.parseArtifact(ctx, artifact)
		if err != nil {
			return errors.New("checkpoint identity artifact missing")
		}
		switch value := parsed.(type) {
		case ContinuationContract:
			if value.ID() != checkpoint.ContractID() || value.ProfileID() != checkpoint.ProfileID() {
				return errors.New("checkpoint continuation contract mismatch")
			}
			contractFound = true
		case CommittedHandoff:
			if value.HandoffID() != checkpoint.HandoffID() || value.CensusID() != checkpoint.CensusID() {
				return errors.New("checkpoint committed handoff mismatch")
			}
			handoffFound = true
		}
	}
	if !contractFound || !handoffFound {
		return errors.New("checkpoint identity artifact missing")
	}
	return nil
}

func verifyAttemptHistoryExtension(ctx context.Context, store Store, previous, current Checkpoint) error {
	find := func(checkpoint Checkpoint) string {
		for _, artifact := range checkpoint.Artifacts() {
			if artifact.Kind == "describe_records" {
				return artifact.ID
			}
		}
		return ""
	}
	priorID, currentID := find(previous), find(current)
	if priorID == "" {
		return nil
	}
	if currentID == "" {
		return errors.New("checkpoint attempt history removed")
	}
	load := func(checkpoint Checkpoint, id string) ([]string, []string, error) {
		var records parsedWorkerRecords
		if verification, ok := store.(*verificationStore); ok {
			for _, artifact := range checkpoint.Artifacts() {
				if artifact.ID != id {
					continue
				}
				parsed, err := verification.parseArtifact(ctx, artifact)
				if err != nil {
					return nil, nil, err
				}
				records = parsed.(parsedWorkerRecords)
			}
		} else {
			raw, err := store.Get(ctx, id)
			if err != nil {
				return nil, nil, err
			}
			var header struct {
				SchemaVersion string `json:"schema_version"`
			}
			_ = json.Unmarshal(raw, &header)
			if header.SchemaVersion == workerRecordsV2Schema {
				records.invocationsV2, records.responsesV2, err = decodeWorkerHistoryV2(raw)
			} else {
				records.invocations, records.responses, err = decodeWorkerRecords(raw)
			}
			if err != nil {
				return nil, nil, err
			}
		}
		invocationIDs, responseIDs := []string{}, []string{}
		for _, record := range records.invocations {
			invocationIDs = append(invocationIDs, record.ID())
		}
		for _, record := range records.invocationsV2 {
			invocationIDs = append(invocationIDs, record.ID())
		}
		for _, record := range records.responses {
			responseIDs = append(responseIDs, record.ID())
		}
		for _, record := range records.responsesV2 {
			responseIDs = append(responseIDs, record.ID())
		}
		return invocationIDs, responseIDs, nil
	}
	priorInvocations, priorResponses, err := load(previous, priorID)
	if err != nil {
		return errors.New("checkpoint attempt history invalid")
	}
	currentInvocations, currentResponses, err := load(current, currentID)
	if err != nil {
		return errors.New("checkpoint attempt history invalid")
	}
	invocations := make(map[string]bool, len(currentInvocations))
	for _, invocation := range currentInvocations {
		invocations[invocation] = true
	}
	for _, invocation := range priorInvocations {
		if !invocations[invocation] {
			return errors.New("checkpoint attempt history replaced")
		}
	}
	responses := make(map[string]bool, len(currentResponses))
	for _, response := range currentResponses {
		responses[response] = true
	}
	for _, response := range priorResponses {
		if !responses[response] {
			return errors.New("checkpoint attempt history replaced")
		}
	}
	return nil
}
func putArtifact(ctx context.Context, s Store, kind string, b []byte) (ArtifactRef, error) {
	id, err := s.Put(ctx, b)
	return ArtifactRef{Kind: kind, ID: id, Digest: id, ByteLength: uint64(len(b))}, err
}
func putCheckpoint(ctx context.Context, s Store, c Checkpoint) (string, error) {
	b, err := c.Bytes()
	if err != nil {
		return "", err
	}
	return s.Put(ctx, b)
}
func digestOf(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }

var canonicalDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validDigest(s string) bool { return canonicalDigest.MatchString(s) }
func identityJSON[T any](domain string, v T, clear func(*T)) string {
	clear(&v)
	b, _ := json.Marshal(v)
	return digestOf(append([]byte(domain+"\x00"), b...))
}
func strictCanonical(raw []byte, out any) error {
	if len(raw) == 0 || !bytes.Equal(raw, bytes.TrimSpace(raw)) {
		return errors.New("canonical json required")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one json value required")
	}
	b, err := json.Marshal(out)
	if err != nil || !bytes.Equal(raw, b) {
		return errors.New("noncanonical json")
	}
	return nil
}

var _ = fmt.Sprintf
var _ = sort.Strings
