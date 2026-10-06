package sessionruntime

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"lsp-trace/internal/lspwire"
)

const (
	privateHistoryMaxEntries = lspwire.MaxImmutableHistoryEntries
	privateHistoryMaxBytes   = uint64(8 << 20)
	privateHistoryChunkSize  = 127
	privateHistoryChunks     = 130
)

type privateHistoryDirection uint8

const (
	privateHistoryOutbound privateHistoryDirection = iota + 1
	privateHistoryInbound
)

type privateHistoryEntry struct {
	ordinal   uint64
	direction privateHistoryDirection
	frame     []byte
	digest    [32]byte
}

type privateHistoryChunk [privateHistoryChunkSize]privateHistoryEntry

type privateReadinessHistory struct {
	identity      string
	chunks        [privateHistoryChunks]*privateHistoryChunk
	entries       uint64
	acquired      uint64
	outstanding   uint64
	borrowers     uint64
	receiptOwners uint64
	retired       bool
}

type privateHistoryReceipt struct {
	metadata lspwire.ImmutableHistoryMetadata
	history  *privateReadinessHistory
}

func (m *Manager) issuePrivateReadinessHistoryLocked(sessionID string, generation uint64) (*privateReadinessHistory, error) {
	var raw [32]byte
	if _, err := io.ReadFull(m.readinessHistoryRandom, raw[:]); err != nil || raw == ([32]byte{}) || sessionID == "" || generation == 0 {
		return nil, errors.New("readiness history identity unavailable")
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("lsp-trace/readiness-history-identity/v1\x00"))
	putHistoryField(hash, raw[:])
	putHistoryField(hash, []byte(sessionID))
	var encodedGeneration [8]byte
	binary.BigEndian.PutUint64(encodedGeneration[:], generation)
	_, _ = hash.Write(encodedGeneration[:])
	var identity [32]byte
	copy(identity[:], hash.Sum(nil))
	return &privateReadinessHistory{identity: "sha256:" + fmtDigest(identity)}, nil
}

func (h *privateReadinessHistory) appendOwned(direction privateHistoryDirection, frame []byte) error {
	if h == nil || h.retired || direction == 0 || len(frame) == 0 || h.entries >= privateHistoryMaxEntries {
		return errors.New("readiness history append refused")
	}
	capacity := uint64(len(frame))
	if capacity > privateHistoryMaxBytes-h.acquired || capacity > privateHistoryMaxBytes-h.outstanding {
		return errors.New("readiness history byte budget refused")
	}
	index := h.entries
	chunkIndex, entryIndex := index/privateHistoryChunkSize, index%privateHistoryChunkSize
	if chunkIndex >= privateHistoryChunks {
		return errors.New("readiness history table exhausted")
	}
	chunk := h.chunks[chunkIndex]
	if chunk == nil {
		chunk = &privateHistoryChunk{}
		h.chunks[chunkIndex] = chunk
	}
	owned := append([]byte(nil), frame...)
	chunk[entryIndex] = privateHistoryEntry{ordinal: index + 1, direction: direction, frame: owned, digest: sha256.Sum256(owned)}
	h.entries++
	h.acquired += capacity
	h.outstanding += uint64(cap(owned))
	if h.outstanding > privateHistoryMaxBytes {
		chunk[entryIndex] = privateHistoryEntry{}
		h.entries--
		h.acquired -= capacity
		h.outstanding -= uint64(cap(owned))
		return errors.New("readiness history backing budget refused")
	}
	return nil
}

func (h *privateReadinessHistory) cutDigest() string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("lsp-trace/readiness-history-cut/v1\x00"))
	putHistoryField(hash, []byte(h.identity))
	for i := uint64(0); i < h.entries; i++ {
		e := h.chunks[i/privateHistoryChunkSize][i%privateHistoryChunkSize]
		var scalar [8]byte
		binary.BigEndian.PutUint64(scalar[:], e.ordinal)
		_, _ = hash.Write(scalar[:])
		_, _ = hash.Write([]byte{byte(e.direction)})
		binary.BigEndian.PutUint64(scalar[:], uint64(len(e.frame)))
		_, _ = hash.Write(scalar[:])
		_, _ = hash.Write(e.digest[:])
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return "sha256:" + fmtDigest(sum)
}

func putHistoryField(w io.Writer, value []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(value)))
	_, _ = w.Write(n[:])
	_, _ = w.Write(value)
}

func (h *privateReadinessHistory) freezeReceiptLocked() (privateHistoryReceipt, error) {
	if h == nil || h.entries == 0 || h.entries > privateHistoryMaxEntries || h.acquired > privateHistoryMaxBytes || h.outstanding > privateHistoryMaxBytes {
		return privateHistoryReceipt{}, errors.New("readiness history unavailable")
	}
	h.receiptOwners++
	return privateHistoryReceipt{history: h, metadata: lspwire.ImmutableHistoryMetadata{
		Identity: h.identity, Cut: h.cutDigest(), Entries: h.entries, CutOrdinal: h.entries,
		Acquired: h.acquired, Outstanding: h.outstanding,
	}}, nil
}

func (h *privateReadinessHistory) releaseReceiptOwnerLocked() {
	if h != nil && h.receiptOwners > 0 {
		h.receiptOwners--
	}
}

func (h *privateReadinessHistory) acquireBorrowerLocked() bool {
	if h == nil || h.receiptOwners == 0 {
		return false
	}
	h.receiptOwners--
	h.borrowers++
	return true
}

func (h *privateReadinessHistory) releaseBorrowerLocked() {
	if h != nil && h.borrowers > 0 {
		h.borrowers--
	}
}

func (h *privateReadinessHistory) retireLocked() {
	if h != nil {
		h.retired = true
	}
}

func (m *Manager) appendReadinessHistoryFrame(opID string, direction privateHistoryDirection, frame []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	op := m.readiness[opID]
	if op == nil {
		return errors.New("readiness operation unavailable")
	}
	r := m.sessions[op.snapshot.SessionID]
	if r == nil || r.record.Generation != op.snapshot.Generation || r.readinessHistory == nil {
		return errors.New("readiness generation unavailable")
	}
	return r.readinessHistory.appendOwned(direction, frame)
}
