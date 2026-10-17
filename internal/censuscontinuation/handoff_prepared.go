package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"lsp-trace/internal/describerequest"
)

const privateHandoffPreparedSchema = "lsp-trace.census-handoff-prepared.private.v1"

const (
	privateHandoffPreparedStatus = "HANDOFF_PREPARED"
	privateHandoffCustody        = "CALLER_CONTROLLED_INFERENCE"
)

type privateHandoffPreparedWire struct {
	SchemaVersion      string `json:"schema_version"`
	PreparedID         string `json:"prepared_id"`
	Status             string `json:"status"`
	Authority          int    `json:"authority"`
	Accepted           bool   `json:"accepted"`
	Completeness       string `json:"completeness"`
	CheckpointSelector string `json:"checkpoint_selector"`
	CheckpointID       string `json:"checkpoint_id"`
	ChainIdentity      string `json:"chain_identity"`
	RequestsSelector   string `json:"requests_selector"`
	RequestsDigest     string `json:"requests_digest"`
	RequestsByteLength uint64 `json:"requests_byte_length"`
	RequestsCount      int    `json:"requests_count"`
	Custody            string `json:"custody"`
}

type privateHandoffPrepared struct{ wire privateHandoffPreparedWire }

func (p privateHandoffPrepared) ID() string { return p.wire.PreparedID }

func (p privateHandoffPrepared) Bytes() ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(p.wire)
}

func parsePrivateHandoffPrepared(raw []byte) (privateHandoffPrepared, error) {
	var wire privateHandoffPreparedWire
	if err := strictCanonical(raw, &wire); err != nil {
		return privateHandoffPrepared{}, err
	}
	prepared := privateHandoffPrepared{wire: wire}
	return prepared, prepared.validate()
}

func (p privateHandoffPrepared) validate() error {
	w := p.wire
	if w.SchemaVersion != privateHandoffPreparedSchema ||
		w.Status != privateHandoffPreparedStatus ||
		w.Authority != 0 || w.Accepted || w.Completeness != ContinuationCompleteness ||
		!validDigest(w.PreparedID) || !validDigest(w.CheckpointSelector) ||
		!validDigest(w.CheckpointID) || !validDigest(w.ChainIdentity) ||
		!validDigest(w.RequestsSelector) || !validDigest(w.RequestsDigest) ||
		w.RequestsByteLength == 0 || w.RequestsCount < 0 || w.Custody != privateHandoffCustody {
		return errors.New("private handoff prepared integrity mismatch")
	}
	want := identityJSON("lsp-trace:census-handoff-prepared:private:v1", w, func(v *privateHandoffPreparedWire) {
		v.PreparedID = ""
	})
	if want != w.PreparedID {
		return errors.New("private handoff prepared identity mismatch")
	}
	return nil
}

// PreparePrivateHandoffSelector validates an exact retained checkpoint and returns
// only the private selector produced by the reviewed preparation primitive.
func PreparePrivateHandoffSelector(ctx context.Context, store Store, checkpointSelector string) (string, error) {
	_, selector, err := preparePrivateHandoff(ctx, store, checkpointSelector)
	return selector, err
}

func preparePrivateHandoff(ctx context.Context, store Store, checkpointSelector string) (privateHandoffPrepared, string, error) {
	if store == nil || !validDigest(checkpointSelector) {
		return privateHandoffPrepared{}, "", errors.New("private handoff preparation input invalid")
	}
	chain, err := VerifyChain(ctx, store, checkpointSelector)
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}
	if len(chain) == 0 {
		return privateHandoffPrepared{}, "", errors.New("private handoff checkpoint chain empty")
	}
	checkpoint := chain[len(chain)-1]
	if checkpoint.Stage() != StageRequestsRendered || checkpoint.Status() != StatusRunning {
		return privateHandoffPrepared{}, "", errors.New("private handoff requires requests rendered running")
	}
	if err := VerifyCheckpoint(ctx, store, checkpoint, checkpoint.PriorID()); err != nil {
		return privateHandoffPrepared{}, "", err
	}

	var requestsRef ArtifactRef
	for _, ref := range checkpoint.Artifacts() {
		if ref.Kind == "requests" {
			if requestsRef.Kind != "" {
				return privateHandoffPrepared{}, "", errors.New("private handoff requests artifact ambiguous")
			}
			requestsRef = ref
		}
	}
	if requestsRef.Kind == "" {
		return privateHandoffPrepared{}, "", errors.New("private handoff requests artifact missing")
	}
	requestsRaw, err := store.Get(ctx, requestsRef.ID)
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}
	if digestOf(requestsRaw) != requestsRef.Digest || uint64(len(requestsRaw)) != requestsRef.ByteLength || requestsRef.ID != requestsRef.Digest {
		return privateHandoffPrepared{}, "", errors.New("checkpoint artifact mismatch")
	}
	requests, err := describerequest.Parse(requestsRaw)
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}

	checkpointIDs := make([]string, len(chain))
	for i := range chain {
		checkpointIDs[i] = chain[i].ID()
	}
	wire := privateHandoffPreparedWire{
		SchemaVersion:      privateHandoffPreparedSchema,
		Status:             privateHandoffPreparedStatus,
		Completeness:       ContinuationCompleteness,
		CheckpointSelector: checkpointSelector,
		CheckpointID:       checkpoint.ID(),
		ChainIdentity: identityJSON("lsp-trace:census-handoff-prepared-chain:private:v1", checkpointIDs, func(*[]string) {
		}),
		RequestsSelector:   requestsRef.ID,
		RequestsDigest:     requestsRef.Digest,
		RequestsByteLength: requestsRef.ByteLength,
		RequestsCount:      len(requests),
		Custody:            privateHandoffCustody,
	}
	wire.PreparedID = identityJSON("lsp-trace:census-handoff-prepared:private:v1", wire, func(v *privateHandoffPreparedWire) {
		v.PreparedID = ""
	})
	prepared := privateHandoffPrepared{wire: wire}
	preparedRaw, err := prepared.Bytes()
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}
	selector, err := store.Put(ctx, preparedRaw)
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}
	if selector != digestOf(preparedRaw) {
		return privateHandoffPrepared{}, "", errors.New("private handoff store selector mismatch")
	}
	readback, err := store.Get(ctx, selector)
	if err != nil {
		return privateHandoffPrepared{}, "", err
	}
	parsed, err := parsePrivateHandoffPrepared(readback)
	if err != nil || !bytes.Equal(readback, preparedRaw) || parsed.ID() != prepared.ID() {
		return privateHandoffPrepared{}, "", errors.New("private handoff readback mismatch")
	}
	return prepared, selector, nil
}
