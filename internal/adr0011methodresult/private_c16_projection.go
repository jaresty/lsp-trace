package adr0011methodresult

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"strconv"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/sessionruntime"
)

const privateC16ProjectionVersion = "ADR0011_PRIVATE_C16_PROJECTION_V1"
const privateC16ObjectLimit = 4096

type privateC16Value struct {
	Ordinal int
	Kind    Kind
	URI     string
	Range   Range
	Target  *Range
	Origin  *Range
}
type privateC16View struct {
	items  []Item
	active *bool
}

func (v privateC16View) count() (int, bool) {
	if v.active == nil || !*v.active {
		return 0, false
	}
	return len(v.items), true
}
func (v privateC16View) at(i int) (privateC16Value, bool) {
	if v.active == nil || !*v.active || i < 0 || i >= len(v.items) {
		return privateC16Value{}, false
	}
	x := v.items[i]
	return privateC16Value{x.Ordinal, x.Kind, x.URI, x.Range, x.TargetRange, x.OriginSelectionRange}, true
}

type privateC16Decision struct {
	Version    string
	Status     DefinitionBridgeStatus
	Chronology string
	Count      int
	Digest     [32]byte
}
type privateC16Admitter struct {
	b sessionruntime.PrivateB4DefinitionBorrow
}

func (a privateC16Admitter) WithObjectAdmission(fn func() error) error {
	return a.b.WithObjectAdmission(fn)
}
func hashN(h hash.Hash, n uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	_, _ = h.Write(b[:])
}
func hashR(h hash.Hash, r Range) {
	hashN(h, uint64(r.Start.Line))
	hashN(h, uint64(r.Start.Character))
	hashN(h, uint64(r.End.Line))
	hashN(h, uint64(r.End.Character))
}
func evaluatePrivateC16(v privateC16View, sources map[string][]byte) (privateC16Decision, bool) {
	n, ok := v.count()
	if !ok {
		return privateC16Decision{}, false
	}
	h := sha256.New()
	_, _ = h.Write([]byte(privateC16ProjectionVersion))
	for i := 0; i < n; i++ {
		x, ok := v.at(i)
		if !ok || x.Ordinal != i {
			return privateC16Decision{}, false
		}
		src, ok := sources[x.URI]
		if !ok || !bridgeRangeWithinSource(src, x.Range) || x.Target != nil && !bridgeRangeWithinSource(src, *x.Target) {
			return privateC16Decision{}, false
		}
		hashN(h, uint64(i))
		_, _ = h.Write([]byte(x.Kind))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(x.URI))
		hashR(h, x.Range)
		if x.Target != nil {
			hashR(h, *x.Target)
		}
		if x.Origin != nil {
			hashR(h, *x.Origin)
		}
	}
	var d [32]byte
	copy(d[:], h.Sum(nil))
	return privateC16Decision{privateC16ProjectionVersion, DefinitionBridgeCandidateItems, "", n, d}, true
}

func checkPrivateComposedB4DefinitionC16(m *sessionruntime.Manager, lease sessionruntime.B4DefinitionLease, selection sessionruntime.B4DefinitionSelectionKey, replay v5.B4bFullCandidateInput, sources map[string][]byte) privateC16Decision {
	var staged, published privateC16Decision
	_, status := m.CommitPrivateB4DefinitionBorrowed(lease, selection, func(b sessionruntime.PrivateB4DefinitionBorrow) bool {
		c, r := b.Capture, b.Result
		if c.Method != "textDocument/definition" || c.SessionID != selection.SessionID || c.Key != selection.Key || c.Transaction != selection.Transaction || c.CompletedOwnerKey != selection.CompletedOwnerKey || replay.Write.Method != c.Method || replay.Write.Session != c.SessionID || replay.Write.Generation != c.Key.Generation || replay.Write.Transaction != c.Transaction || replay.Write.CompletedKey != c.CompletedOwnerKey || !replay.Write.WriteCompleted || !privateCaptureHash(c.RequestFrame, c.RequestFrameSHA256) || !privateCaptureHash(c.ResponseFrame, c.ResponseFrameSHA256) || !privateCaptureHash(r, c.ResultSHA256) {
			return false
		}
		id, e := strconv.ParseUint(string(replay.Write.RequestID), 10, 64)
		if e != nil || id != c.Key.ID {
			return false
		}
		response, _, ok := privateB4Frame(c.ResponseFrame)
		if !ok || string(response.ID) != string(replay.Write.RequestID) || string(response.Result) != string(r) {
			return false
		}
		chron := v5.CheckB4bDefinitionSuccessor(replay)
		if chron != "SUPPORTED" {
			return false
		}
		parsed, f := parseRawUntrustedWithAdmission(c.Method, r, privateC16ObjectLimit+1, privateC16Admitter{b})
		if f != nil || len(parsed.Items) > privateC16ObjectLimit {
			return false
		}
		active := true
		d, ok := evaluatePrivateC16(privateC16View{parsed.Items, &active}, sources)
		active = false
		if !ok {
			return false
		}
		d.Chronology = chron
		staged = d
		return true
	}, func(sessionruntime.PrivateB4DefinitionBorrow) { published = staged })
	if status != sessionruntime.PrivateB4Selected {
		return privateC16Decision{}
	}
	return published
}
