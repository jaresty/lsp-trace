package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"unicode/utf8"
	"unsafe"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// Private and deliberately unwired. These values carry custody, not authority.
const restrictedProvenanceRecordVersion uint32 = 1

const (
	restrictedProvenanceUnreserved uint8 = iota
	restrictedProvenanceReserved
	restrictedProvenanceValidated
	restrictedProvenanceHandedOff
)

const (
	restrictedRecordControl      = 0
	restrictedRecordClaimant     = 256
	restrictedRecordPredecessor  = 768
	restrictedRecordFrame        = 1408
	restrictedRecordEvent        = 2432
	restrictedRecordRegistration = 2944
	restrictedRecordFilter       = 6016
	restrictedRecordExchange     = 10112
	restrictedRecordReserved     = 11136
)

type restrictedDefinitionLengths struct {
	Provenance uint64
	Records    uint64
}

// Provenance is one complete exact definition result. It is copied into BID30
// before parsing. No caller-derived candidate, chronology, count, offset, or
// status is accepted. Source bytes are exact retained target-source originals,
// positionally corresponding to result members after the result has parsed.
type restrictedDefinitionInput struct {
	Owner, Session, Generation uint64
	Attempt, Version           uint64
	Provenance                 []byte
	TargetSources              [restrictedCandidateMax][]byte
}

// restrictedDefinitionB4Input is typed, private, and deliberately has no wire
// representation. The manager lease is the only source of response originals.
type restrictedDefinitionB4Input struct {
	Manager           *sessionruntime.Manager
	Lease             sessionruntime.B4DefinitionLease
	Selection         sessionruntime.B4DefinitionSelectionKey
	Replay            v5.B4bFullCandidateInput
	QueryOccurrenceID string
	TargetSources     map[string][]byte
}

type restrictedProvenanceReservation struct {
	manager                                      *restrictedOwnerManager
	slot                                         uint8
	state                                        uint8
	reserved0                                    uint16
	recordVersion                                uint32
	Owner, Session, Generation, Attempt, Version uint64
	rawCapacity, recordCapacity, decodedCapacity uint64
	tokenCapacity, stackCapacity                 uint32
	borrowerCount, flags                         uint16
	reserved1                                    uint32
}

type restrictedValidatedDefinition struct {
	manager                                          *restrictedOwnerManager
	slot, chronologyTerminal, state, flags           uint8
	recordVersion                                    uint32
	Owner, Session, Generation, Attempt, Version     uint64
	requestOff, requestLen, responseOff, responseLen uint32
	resultOff, resultLen                             uint32
	candidateCount, borrowerCount                    uint16
	obligationBits                                   uint32
	reserved                                         uint64
}

type restrictedDefinitionProducer struct{ manager *restrictedOwnerManager }

var (
	_ [96 - unsafe.Sizeof(restrictedProvenanceReservation{})]byte
	_ [unsafe.Sizeof(restrictedProvenanceReservation{}) - 96]byte
	_ [96 - unsafe.Sizeof(restrictedValidatedDefinition{})]byte
	_ [unsafe.Sizeof(restrictedValidatedDefinition{}) - 96]byte
	_ [8 - unsafe.Offsetof(restrictedProvenanceReservation{}.slot)]byte
	_ [unsafe.Offsetof(restrictedProvenanceReservation{}.slot) - 8]byte
	_ [12 - unsafe.Offsetof(restrictedProvenanceReservation{}.recordVersion)]byte
	_ [unsafe.Offsetof(restrictedProvenanceReservation{}.recordVersion) - 12]byte
	_ [56 - unsafe.Offsetof(restrictedProvenanceReservation{}.rawCapacity)]byte
	_ [unsafe.Offsetof(restrictedProvenanceReservation{}.rawCapacity) - 56]byte
	_ [88 - unsafe.Offsetof(restrictedProvenanceReservation{}.borrowerCount)]byte
	_ [unsafe.Offsetof(restrictedProvenanceReservation{}.borrowerCount) - 88]byte
	_ [8 - unsafe.Offsetof(restrictedValidatedDefinition{}.slot)]byte
	_ [unsafe.Offsetof(restrictedValidatedDefinition{}.slot) - 8]byte
	_ [56 - unsafe.Offsetof(restrictedValidatedDefinition{}.requestOff)]byte
	_ [unsafe.Offsetof(restrictedValidatedDefinition{}.requestOff) - 56]byte
	_ [80 - unsafe.Offsetof(restrictedValidatedDefinition{}.candidateCount)]byte
	_ [unsafe.Offsetof(restrictedValidatedDefinition{}.candidateCount) - 80]byte
	_ [88 - unsafe.Offsetof(restrictedValidatedDefinition{}.reserved)]byte
	_ [unsafe.Offsetof(restrictedValidatedDefinition{}.reserved) - 88]byte
)

func (m *restrictedOwnerManager) reserveDefinitionProvenance(p restrictedProcessingReservation, lengths restrictedDefinitionLengths) (restrictedProvenanceReservation, error) {
	if lengths.Provenance > restrictedProvenanceRawBytes || lengths.Records > restrictedProvenanceRecordBytes {
		return restrictedProvenanceReservation{}, errRestrictedBound
	}
	if !m.tryBoth() {
		return restrictedProvenanceReservation{}, errRestrictedBusy
	}
	defer m.unlockBoth()
	a, err := p.exact(m)
	if err != nil {
		return restrictedProvenanceReservation{}, err
	}
	if a.retired || a.quarantined || a.terminal || a.retentionDisposed || a.forgotten || a.provenanceState != restrictedProvenanceUnreserved {
		return restrictedProvenanceReservation{}, errRestrictedState
	}
	var genUsed uint64
	for i := range m.attempts {
		x := &m.attempts[i]
		if x.used && x.owner == a.owner && x.session == a.session && x.generation == a.generation {
			genUsed += x.processingCharge + x.outputCharge
		}
	}
	if !restrictedAddFits(a.processingCharge, restrictedProvenanceCharge, restrictedGenerationBytes) ||
		!restrictedAddFits(genUsed, restrictedProvenanceCharge, restrictedGenerationBytes) ||
		!restrictedAddFits(m.root.globalUsed, restrictedProvenanceCharge, restrictedGlobalBytes) {
		return restrictedProvenanceReservation{}, errRestrictedBusy
	}
	a.processingCharge += restrictedProvenanceCharge
	m.root.globalUsed += restrictedProvenanceCharge
	a.provenanceState = restrictedProvenanceReserved
	a.provenanceRecordVersion = restrictedProvenanceRecordVersion
	a.provenanceEpoch++
	m.allocations++
	a.provenanceRaw = make([]byte, restrictedProvenanceRawBytes)
	m.allocations++
	a.provenanceRecords = make([]byte, restrictedProvenanceRecordBytes)
	m.root.provenanceRaw[p.slot] = a.provenanceRaw
	m.root.provenanceRecords[p.slot] = a.provenanceRecords
	return restrictedProvenanceReservation{manager: m, slot: p.slot, state: restrictedProvenanceReserved, recordVersion: a.provenanceRecordVersion,
		Owner: a.owner, Session: a.session, Generation: a.generation, Attempt: a.attempt, Version: a.version,
		rawCapacity: restrictedProvenanceRawBytes, recordCapacity: restrictedProvenanceRecordBytes, decodedCapacity: 131072,
		tokenCapacity: 131072, stackCapacity: 512, borrowerCount: a.provenanceBorrowers, flags: uint16(a.provenanceFlags)}, nil
}

func (r restrictedProvenanceReservation) exact(m *restrictedOwnerManager) (*restrictedAttempt, error) {
	if m == nil || r.manager != m || int(r.slot) >= len(m.attempts) || r.state != restrictedProvenanceReserved || r.recordVersion != restrictedProvenanceRecordVersion ||
		r.rawCapacity != restrictedProvenanceRawBytes || r.recordCapacity != restrictedProvenanceRecordBytes || r.decodedCapacity != 131072 || r.tokenCapacity != 131072 || r.stackCapacity != 512 || r.reserved0 != 0 || r.reserved1 != 0 {
		return nil, errRestrictedIdentity
	}
	a := &m.attempts[r.slot]
	if !a.used || a.provenanceState != restrictedProvenanceReserved || a.retired || a.quarantined || a.terminal || a.retentionDisposed || a.forgotten ||
		a.owner != r.Owner || a.session != r.Session || a.generation != r.Generation || a.attempt != r.Attempt || a.version != r.Version ||
		a.provenanceRecordVersion != r.recordVersion || a.provenanceBorrowers != r.borrowerCount || uint16(a.provenanceFlags) != r.flags ||
		len(a.provenanceRaw) != int(restrictedProvenanceRawBytes) || len(a.provenanceRecords) != int(restrictedProvenanceRecordBytes) ||
		len(m.root.provenanceRaw[r.slot]) != len(a.provenanceRaw) || len(m.root.provenanceRecords[r.slot]) != len(a.provenanceRecords) ||
		&m.root.provenanceRaw[r.slot][0] != &a.provenanceRaw[0] || &m.root.provenanceRecords[r.slot][0] != &a.provenanceRecords[0] {
		return nil, errRestrictedIdentity
	}
	return a, nil
}

type restrictedParsedCandidate struct {
	uriStart, uriEnd   uint32
	selection, target  Range
	hasTarget          bool
	itemStart, itemEnd uint32
}

type restrictedJSONCursor struct {
	b                         []byte
	i, tokens, depth, members int
}

func (c *restrictedJSONCursor) ws() {
	for c.i < len(c.b) && (c.b[c.i] == ' ' || c.b[c.i] == '\n' || c.b[c.i] == '\r' || c.b[c.i] == '\t') {
		c.i++
	}
}
func (c *restrictedJSONCursor) take(ch byte) bool {
	c.ws()
	if c.i >= len(c.b) || c.b[c.i] != ch {
		return false
	}
	c.i++
	c.tokens++
	return c.tokens <= 4096
}
func (c *restrictedJSONCursor) literal(s string) bool {
	c.ws()
	if len(c.b)-c.i < len(s) || string(c.b[c.i:c.i+len(s)]) != s {
		return false
	}
	c.i += len(s)
	c.tokens++
	return c.tokens <= 4096
}
func (c *restrictedJSONCursor) str() (uint32, uint32, bool) {
	c.ws()
	if c.i >= len(c.b) || c.b[c.i] != '"' {
		return 0, 0, false
	}
	c.i++
	start := c.i
	for c.i < len(c.b) {
		ch := c.b[c.i]
		if ch == '"' {
			end := c.i
			c.i++
			c.tokens++
			return uint32(start), uint32(end), c.tokens <= 4096 && end-start <= 4096 && utf8.Valid(c.b[start:end])
		}
		if ch == '\\' || ch < 0x20 {
			return 0, 0, false
		}
		c.i++
	}
	return 0, 0, false
}
func (c *restrictedJSONCursor) uint32() (uint32, bool) {
	c.ws()
	start := c.i
	if start >= len(c.b) || c.b[start] < '0' || c.b[start] > '9' {
		return 0, false
	}
	var n uint64
	for c.i < len(c.b) && c.b[c.i] >= '0' && c.b[c.i] <= '9' {
		n = n*10 + uint64(c.b[c.i]-'0')
		if n > ^uint64(uint32(0)) {
			return 0, false
		}
		c.i++
	}
	if c.i-start > 1 && c.b[start] == '0' {
		return 0, false
	}
	c.tokens++
	return uint32(n), c.tokens <= 4096
}
func rawEq(b []byte, s, e uint32, want string) bool {
	return int(e) <= len(b) && string(b[s:e]) == want
}
func (c *restrictedJSONCursor) position() (Position, bool) {
	var p Position
	if !c.take('{') {
		return p, false
	}
	seen := uint8(0)
	for {
		c.ws()
		if c.take('}') {
			return p, seen == 3
		}
		ks, ke, ok := c.str()
		if !ok || !c.take(':') {
			return p, false
		}
		n, ok := c.uint32()
		if !ok {
			return p, false
		}
		switch {
		case rawEq(c.b, ks, ke, "line") && seen&1 == 0:
			p.Line = n
			seen |= 1
		case rawEq(c.b, ks, ke, "character") && seen&2 == 0:
			p.Character = n
			seen |= 2
		default:
			return p, false
		}
		c.ws()
		if c.take(',') {
			continue
		}
		if c.take('}') {
			return p, seen == 3
		}
		return p, false
	}
}
func (c *restrictedJSONCursor) rng() (Range, bool) {
	var r Range
	if !c.take('{') {
		return r, false
	}
	seen := uint8(0)
	for {
		c.ws()
		if c.take('}') {
			return r, seen == 3 && (r.Start.Line < r.End.Line || r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
		}
		ks, ke, ok := c.str()
		if !ok || !c.take(':') {
			return r, false
		}
		p, ok := c.position()
		if !ok {
			return r, false
		}
		switch {
		case rawEq(c.b, ks, ke, "start") && seen&1 == 0:
			r.Start = p
			seen |= 1
		case rawEq(c.b, ks, ke, "end") && seen&2 == 0:
			r.End = p
			seen |= 2
		default:
			return r, false
		}
		c.ws()
		if c.take(',') {
			continue
		}
		if c.take('}') {
			return r, seen == 3 && (r.Start.Line < r.End.Line || r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
		}
		return r, false
	}
}
func (c *restrictedJSONCursor) item() (restrictedParsedCandidate, bool) {
	var out restrictedParsedCandidate
	c.ws()
	out.itemStart = uint32(c.i)
	if !c.take('{') {
		return out, false
	}
	seen := uint8(0)
	for {
		c.ws()
		if c.take('}') {
			out.itemEnd = uint32(c.i)
			return out, seen == 3
		}
		ks, ke, ok := c.str()
		if !ok || !c.take(':') {
			return out, false
		}
		switch {
		case rawEq(c.b, ks, ke, "uri") && seen&1 == 0:
			out.uriStart, out.uriEnd, ok = c.str()
			seen |= 1
		case rawEq(c.b, ks, ke, "range") && seen&2 == 0:
			out.selection, ok = c.rng()
			seen |= 2
		case rawEq(c.b, ks, ke, "targetRange") && seen&4 == 0:
			out.target, ok = c.rng()
			out.hasTarget = ok
			seen |= 4
		default:
			return out, false
		}
		if !ok {
			return out, false
		}
		c.ws()
		if c.take(',') {
			continue
		}
		if c.take('}') {
			out.itemEnd = uint32(c.i)
			return out, seen&3 == 3
		}
		return out, false
	}
}
func restrictedParseResult(raw []byte, out *[restrictedCandidateMax]restrictedParsedCandidate) (int, bool, error) {
	c := restrictedJSONCursor{b: raw}
	c.ws()
	if c.literal("null") {
		c.ws()
		if c.i == len(raw) {
			return 0, true, nil
		}
		return 0, false, errRestrictedState
	}
	if !c.take('[') {
		return 0, false, errRestrictedState
	}
	n := 0
	c.ws()
	if c.take(']') {
		c.ws()
		if c.i == len(raw) {
			return 0, false, nil
		}
		return 0, false, errRestrictedState
	}
	for {
		if n == restrictedCandidateMax {
			return 0, false, errRestrictedBound
		}
		item, ok := c.item()
		if !ok {
			return 0, false, errRestrictedState
		}
		out[n] = item
		n++
		c.ws()
		if c.take(',') {
			continue
		}
		if !c.take(']') {
			return 0, false, errRestrictedState
		}
		c.ws()
		if c.i != len(raw) {
			return 0, false, errRestrictedState
		}
		return n, false, nil
	}
}

func restrictedRangeWithinSource(source []byte, r Range) bool {
	if !utf8.Valid(source) || r.Start.Line > r.End.Line || r.Start.Line == r.End.Line && r.Start.Character > r.End.Character {
		return false
	}
	check := func(p Position) bool {
		line := uint32(0)
		units := uint32(0)
		for i := 0; i <= len(source); {
			if line == p.Line {
				for i < len(source) && source[i] != '\n' {
					rr, n := utf8.DecodeRune(source[i:])
					if rr == '\r' && i+n == len(source) {
						break
					}
					if rr > 0xffff {
						units += 2
					} else {
						units++
					}
					i += n
				}
				return p.Character <= units
			}
			if i >= len(source) {
				return false
			}
			if source[i] == '\n' {
				line++
			}
			i++
		}
		return false
	}
	return check(r.Start) && check(r.End)
}
func restrictedHex(dst []byte, sum [32]byte) {
	const h = "0123456789abcdef"
	for i, b := range sum {
		dst[i*2] = h[b>>4]
		dst[i*2+1] = h[b&15]
	}
}
func restrictedCandidateID(raw []byte, domain string, ordinal int, dst []byte) string {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write(raw)
	var x [8]byte
	binary.BigEndian.PutUint64(x[:], uint64(ordinal))
	h.Write(x[:])
	sum := h.Sum(nil)
	copy(dst, "sha256:")
	var a [32]byte
	copy(a[:], sum)
	restrictedHex(dst[7:], a)
	return unsafe.String(&dst[0], 71)
}

func (m *restrictedOwnerManager) validateDefinitionProvenance(r restrictedProvenanceReservation, in restrictedDefinitionInput) (restrictedValidatedDefinition, error) {
	if len(in.Provenance) == 0 || uint64(len(in.Provenance)) > restrictedProvenanceRawBytes {
		return restrictedValidatedDefinition{}, errRestrictedBound
	}
	// Unit 2 fails closed before owner state or BID30/BID31 backing changes.
	// The parser below recognizes only a bare result array or null; neither is
	// complete definition provenance.
	var resultOnly [restrictedCandidateMax]restrictedParsedCandidate
	if _, _, err := restrictedParseResult(in.Provenance, &resultOnly); err == nil {
		return restrictedValidatedDefinition{}, errRestrictedIncomplete
	}
	if !m.tryBoth() {
		return restrictedValidatedDefinition{}, errRestrictedBusy
	}
	defer m.unlockBoth()
	a, err := r.exact(m)
	if err != nil {
		return restrictedValidatedDefinition{}, err
	}
	if in.Owner != a.owner || in.Session != a.session || in.Generation != a.generation || in.Attempt != a.attempt || in.Version != a.version {
		return restrictedValidatedDefinition{}, errRestrictedIdentity
	}
	copy(a.provenanceRaw, in.Provenance)
	raw := a.provenanceRaw[:len(in.Provenance)]
	var parsed [restrictedCandidateMax]restrictedParsedCandidate
	count, isNull, err := restrictedParseResult(raw, &parsed)
	if err != nil {
		return restrictedValidatedDefinition{}, err
	}
	for i := 0; i < count; i++ {
		if len(in.TargetSources[i]) == 0 || !restrictedRangeWithinSource(in.TargetSources[i], parsed[i].selection) || (parsed[i].hasTarget && !restrictedRangeWithinSource(in.TargetSources[i], parsed[i].target)) {
			return restrictedValidatedDefinition{}, errRestrictedState
		}
	}
	// IDs are derived into otherwise-unused BID30 tail space, then copied by the accepted constructor.
	idBytes := count * 3 * 71
	if len(raw)+idBytes > len(a.provenanceRaw) {
		return restrictedValidatedDefinition{}, errRestrictedBound
	}
	ids := a.provenanceRaw[len(raw) : len(raw)+idBytes]
	var owned [restrictedCandidateMax]DefinitionCandidate
	for i := 0; i < count; i++ {
		p := parsed[i]
		uri := unsafe.String(&a.provenanceRaw[p.uriStart], int(p.uriEnd-p.uriStart))
		owned[i] = DefinitionCandidate{OccurrenceID: restrictedCandidateID(raw[p.itemStart:p.itemEnd], "lsp-trace:adr0011:restricted-definition-occurrence:v1", i, ids[(i*3)*71:]), QueryOccurrenceID: restrictedCandidateID(raw, "lsp-trace:adr0011:restricted-query-occurrence:v1", 0, ids[(i*3+1)*71:]), TargetID: restrictedCandidateID(raw[p.itemStart:p.itemEnd], "lsp-trace:adr0011:restricted-definition-target:v1", 0, ids[(i*3+2)*71:]), Ordinal: i, QueryURI: "restricted://owned", TargetURI: uri, TargetKind: Location, TargetSelectionRange: p.selection}
		if p.hasTarget {
			q := p.target
			owned[i].TargetRange = &q
		}
	}
	copy(a.provenanceCandidates[:], owned[:count])
	a.provenanceCandidateCount = uint16(count)
	rec := a.provenanceRecords
	copy(rec[:16], []byte("ADR0011-PROV-V2"))
	binary.LittleEndian.PutUint32(rec[16:20], restrictedProvenanceRecordVersion)
	binary.LittleEndian.PutUint16(rec[20:22], uint16(restrictedProvenanceValidated))
	binary.LittleEndian.PutUint32(rec[24:28], uint32(len(raw)))
	binary.LittleEndian.PutUint16(rec[52:54], 1)
	binary.LittleEndian.PutUint16(rec[54:56], uint16(count))
	binary.LittleEndian.PutUint64(rec[64:72], a.owner)
	binary.LittleEndian.PutUint64(rec[72:80], a.session)
	binary.LittleEndian.PutUint64(rec[80:88], a.generation)
	binary.LittleEndian.PutUint64(rec[88:96], a.attempt)
	binary.LittleEndian.PutUint64(rec[96:104], a.version)
	binary.LittleEndian.PutUint32(rec[104:108], a.provenanceRecordVersion)
	binary.LittleEndian.PutUint32(rec[112:116], 0)
	binary.LittleEndian.PutUint32(rec[116:120], uint32(len(raw)))
	d := sha256.Sum256(raw)
	copy(rec[176:208], d[:])
	a.provenanceState = restrictedProvenanceValidated
	a.provenanceChronology = 1
	a.provenanceRawUsed = uint32(len(raw))
	a.provenanceResultLen = uint32(len(raw))
	a.provenanceEpoch++
	flags := uint8(0)
	if isNull {
		flags = 1
	}
	a.provenanceFlags = flags
	return a.validatedHandle(m, r.slot), nil
}

type restrictedDefinitionB4Prepared struct {
	capture      sessionruntime.PrivateB4DefinitionCapture
	ownedSources map[string][]byte
	candidates   [restrictedCandidateMax]DefinitionCandidate
	resultLen    int
	resultSHA256 [sha256.Size]byte
	candidateLen int
}

func (m *restrictedOwnerManager) validateDefinitionProvenanceB4(r restrictedProvenanceReservation, p restrictedProcessingReservation, in restrictedDefinitionB4Input) (restrictedValidatedDefinition, error) {
	if in.Manager == nil {
		return restrictedValidatedDefinition{}, errRestrictedIdentity
	}
	if !m.tryBoth() {
		return restrictedValidatedDefinition{}, errRestrictedBusy
	}
	a, err := r.exact(m)
	pa, perr := p.exact(m)
	if err != nil || perr != nil || a != pa || p.slot != r.slot {
		m.unlockBoth()
		return restrictedValidatedDefinition{}, errRestrictedIdentity
	}
	m.unlockBoth()

	var prepared restrictedDefinitionB4Prepared
	var prepareErr error
	prepare := func(borrow sessionruntime.PrivateB4DefinitionBorrow) bool {
		capture := borrow.Capture
		result := borrow.Result
		w := in.Replay.Write

		if capture.SessionID != in.Selection.SessionID || capture.Key != in.Selection.Key ||
			capture.Transaction != in.Selection.Transaction || capture.CompletedOwnerKey != in.Selection.CompletedOwnerKey ||
			capture.Method != "textDocument/definition" || w.Method != capture.Method || w.Session != capture.SessionID ||
			w.Generation != capture.Key.Generation || w.Transaction != capture.Transaction || w.CompletedKey != capture.CompletedOwnerKey ||
			!w.WriteCompleted || !bytes.Equal(w.RequestFrame, capture.RequestFrame) || !bytes.Equal(w.RequestParams, capture.RequestParams) ||
			!privateCaptureHash(capture.RequestFrame, capture.RequestFrameSHA256) ||
			!privateCaptureHash(capture.ResponseFrame, capture.ResponseFrameSHA256) ||
			!privateCaptureHash(result, capture.ResultSHA256) {
			prepareErr = errRestrictedIdentity
			return false
		}

		id, idErr := strconv.ParseUint(string(w.RequestID), 10, 64)
		if idErr != nil || id == 0 || id > math.MaxInt64 || id != capture.Key.ID ||
			!bytes.Equal(w.RequestID, []byte(strconv.FormatUint(id, 10))) {
			prepareErr = errRestrictedIdentity
			return false
		}

		ownedSources := make(map[string][]byte, len(capture.TargetSources))
		for _, source := range capture.TargetSources {
			if source.URI == "" || source.AcquisitionID == "" ||
				source.SessionID != capture.SessionID || source.Generation != capture.Key.Generation ||
				len(source.Bytes) == 0 || !privateCaptureHash(source.Bytes, source.SHA256) {
				prepareErr = errRestrictedIdentity
				return false
			}
			ownedSources[source.URI] = source.Bytes
		}

		request, requestBody, ok := privateB4Frame(capture.RequestFrame)
		if !ok || request.Kind() != lspwire.KindRequest || request.Method != capture.Method ||
			!bytes.Equal(request.ID, w.RequestID) || !bytes.Equal(request.Params, capture.RequestParams) ||
			len(requestBody) == 0 {
			prepareErr = errRestrictedState
			return false
		}

		response, responseBody, ok := privateB4Frame(capture.ResponseFrame)
		if !ok || response.Kind() != lspwire.KindSuccessResponse ||
			!bytes.Equal(response.ID, w.RequestID) || !bytes.Equal(response.Result, result) ||
			len(responseBody) == 0 {
			prepareErr = errRestrictedState
			return false
		}

		checked := CheckB4DefinitionBridge(DefinitionBridgeInput{
			Replay:            in.Replay,
			ResponseFrame:     responseBody,
			QueryOccurrenceID: capture.QueryOccurrenceID,
			TargetSources:     ownedSources,
		})
		if checked.Status != DefinitionBridgeCandidateItems ||
			checked.ChronologyTerminal != "SUPPORTED" ||
			len(checked.Candidates) > restrictedCandidateMax {
			prepareErr = errRestrictedState
			return false
		}

		total := len(capture.RequestFrame) + len(capture.ResponseFrame) + len(result)
		if total > int(restrictedProvenanceRawBytes) {
			prepareErr = errRestrictedBound
			return false
		}
		for _, candidate := range checked.Candidates {
			source := ownedSources[candidate.TargetURI]
			if len(source) == 0 || total > int(restrictedProvenanceRawBytes)-len(source) {
				prepareErr = errRestrictedBound
				return false
			}
			total += len(source)
		}

		cloneString := func(value string) string {
			return string(append([]byte(nil), value...))
		}
		for i, candidate := range checked.Candidates {
			owned := candidate
			owned.OccurrenceID = cloneString(candidate.OccurrenceID)
			owned.QueryOccurrenceID = cloneString(candidate.QueryOccurrenceID)
			owned.TargetID = cloneString(candidate.TargetID)
			owned.QueryURI = cloneString(candidate.QueryURI)
			owned.TargetURI = cloneString(candidate.TargetURI)
			if candidate.TargetRange != nil {
				targetRange := *candidate.TargetRange
				owned.TargetRange = &targetRange
			}
			prepared.candidates[i] = owned
		}

		prepared.capture = capture
		prepared.capture.Result = nil
		prepared.capture.TargetSources = make([]sessionruntime.B4DefinitionTargetSource, len(capture.TargetSources))
		for i, source := range capture.TargetSources {
			source.Bytes = nil
			prepared.capture.TargetSources[i] = source
		}
		prepared.ownedSources = ownedSources
		prepared.candidateLen = len(checked.Candidates)
		prepared.resultLen = len(result)
		prepared.resultSHA256 = sha256.Sum256(result)
		return true
	}

	_, status := in.Manager.PreparePrivateB4DefinitionBorrowed(in.Lease, in.Selection, prepare)
	if status != sessionruntime.PrivateB4Selected {
		if prepareErr != nil {
			return restrictedValidatedDefinition{}, prepareErr
		}
		return restrictedValidatedDefinition{}, errRestrictedBusy
	}

	var committed restrictedValidatedDefinition
	prepareCommit := func(commitBorrow sessionruntime.PrivateB4DefinitionBorrow) bool {
		commitCapture := commitBorrow.Capture
		savedCapture := prepared.capture
		if commitCapture.SessionID != savedCapture.SessionID ||
			commitCapture.Key != savedCapture.Key ||
			commitCapture.Transaction != savedCapture.Transaction ||
			commitCapture.CompletedOwnerKey != savedCapture.CompletedOwnerKey ||
			commitCapture.Method != savedCapture.Method ||
			commitCapture.RequestFrameSHA256 != savedCapture.RequestFrameSHA256 ||
			commitCapture.ResponseFrameSHA256 != savedCapture.ResponseFrameSHA256 ||
			commitCapture.ResultSHA256 != savedCapture.ResultSHA256 ||
			commitCapture.QueryOccurrenceID != savedCapture.QueryOccurrenceID ||
			!bytes.Equal(commitCapture.RequestFrame, savedCapture.RequestFrame) ||
			!bytes.Equal(commitCapture.RequestParams, savedCapture.RequestParams) ||
			!bytes.Equal(commitCapture.ResponseFrame, savedCapture.ResponseFrame) ||
			len(commitCapture.Result) != 0 ||
			len(commitCapture.TargetSources) != len(savedCapture.TargetSources) ||
			len(commitBorrow.Result) != prepared.resultLen ||
			sha256.Sum256(commitBorrow.Result) != prepared.resultSHA256 {
			return false
		}
		for i, source := range commitCapture.TargetSources {
			savedSource := savedCapture.TargetSources[i]
			if source.URI != savedSource.URI ||
				source.AcquisitionID != savedSource.AcquisitionID ||
				source.SHA256 != savedSource.SHA256 ||
				source.SessionID != savedSource.SessionID ||
				source.Generation != savedSource.Generation ||
				source.DocumentVersion != savedSource.DocumentVersion ||
				!bytes.Equal(source.Bytes, prepared.ownedSources[source.URI]) {
				return false
			}
		}
		if !m.tryBoth() {
			return false
		}
		a, err = r.exact(m)
		pa, perr = p.exact(m)
		if err != nil || perr != nil || a != pa || p.slot != r.slot {
			m.unlockBoth()
			return false
		}
		return true
	}
	publish := func(commitBorrow sessionruntime.PrivateB4DefinitionBorrow) error {
		defer m.unlockBoth()
		commitCapture := commitBorrow.Capture
		off := 0
		a.provenanceRequestOff, a.provenanceRequestLen = uint32(off), uint32(len(commitCapture.RequestFrame))
		copy(a.provenanceRaw[off:], commitCapture.RequestFrame)
		off += len(commitCapture.RequestFrame)
		a.provenanceResponseOff, a.provenanceResponseLen = uint32(off), uint32(len(commitCapture.ResponseFrame))
		copy(a.provenanceRaw[off:], commitCapture.ResponseFrame)
		off += len(commitCapture.ResponseFrame)
		a.provenanceResultOff, a.provenanceResultLen = uint32(off), uint32(len(commitBorrow.Result))
		copy(a.provenanceRaw[off:], commitBorrow.Result)
		off += len(commitBorrow.Result)
		for i := 0; i < prepared.candidateLen; i++ {
			candidate := prepared.candidates[i]
			appendCanonical := func() error {
				source := prepared.ownedSources[candidate.TargetURI]
				sourceOff, sourceLen := uint32(off), uint32(len(source))
				copy(a.provenanceRaw[off:], source)
				off += len(source)
				base := restrictedRecordReserved + i*8
				binary.LittleEndian.PutUint32(a.provenanceRecords[base:base+4], sourceOff)
				binary.LittleEndian.PutUint32(a.provenanceRecords[base+4:base+8], sourceLen)
				a.provenanceCandidates[i] = candidate
				return nil
			}
			if err := commitBorrow.WithTargetAppendAdmission(uint64(candidate.Ordinal), appendCanonical); err != nil {
				if !errors.Is(err, sessionruntime.ErrPrivateB4C17NotEnabled) {
					return err
				}
				if err := appendCanonical(); err != nil {
					return err
				}
			}
		}
		a.provenanceCandidateCount = uint16(prepared.candidateLen)
		rec := a.provenanceRecords
		copy(rec[:16], []byte("ADR0011-PROV-V2"))
		binary.LittleEndian.PutUint32(rec[16:20], restrictedProvenanceRecordVersion)
		binary.LittleEndian.PutUint16(rec[20:22], uint16(restrictedProvenanceValidated))
		binary.LittleEndian.PutUint32(rec[24:28], uint32(off))
		binary.LittleEndian.PutUint16(rec[52:54], 3)
		binary.LittleEndian.PutUint16(rec[54:56], uint16(prepared.candidateLen))
		binary.LittleEndian.PutUint64(rec[64:72], a.owner)
		binary.LittleEndian.PutUint64(rec[72:80], a.session)
		binary.LittleEndian.PutUint64(rec[80:88], a.generation)
		binary.LittleEndian.PutUint64(rec[88:96], a.attempt)
		binary.LittleEndian.PutUint64(rec[96:104], a.version)
		binary.LittleEndian.PutUint32(rec[104:108], a.provenanceRecordVersion)
		binary.LittleEndian.PutUint32(rec[112:116], a.provenanceRequestOff)
		binary.LittleEndian.PutUint32(rec[116:120], a.provenanceRequestLen)
		binary.LittleEndian.PutUint32(rec[120:124], a.provenanceResponseOff)
		binary.LittleEndian.PutUint32(rec[124:128], a.provenanceResponseLen)
		binary.LittleEndian.PutUint32(rec[128:132], a.provenanceResultOff)
		binary.LittleEndian.PutUint32(rec[132:136], a.provenanceResultLen)
		d := sha256.Sum256(a.provenanceRaw[:off])
		copy(rec[176:208], d[:])
		a.provenanceState = restrictedProvenanceValidated
		a.provenanceChronology = 3
		a.provenanceRawUsed = uint32(off)
		a.provenanceEpoch++
		a.provenanceFlags = 0
		committed = a.validatedHandle(m, r.slot)
		return nil
	}
	_, status, publishErr := in.Manager.CommitPrivateB4DefinitionBorrowedC17(in.Lease, in.Selection, prepareCommit, publish)
	if publishErr != nil {
		return restrictedValidatedDefinition{}, publishErr
	}
	if status != sessionruntime.PrivateB4Selected {
		return restrictedValidatedDefinition{}, errRestrictedBusy
	}
	return committed, nil
}

func (a *restrictedAttempt) validatedHandle(m *restrictedOwnerManager, slot uint8) restrictedValidatedDefinition {
	return restrictedValidatedDefinition{manager: m, slot: slot, chronologyTerminal: a.provenanceChronology, state: restrictedProvenanceValidated, flags: a.provenanceFlags, recordVersion: a.provenanceRecordVersion, Owner: a.owner, Session: a.session, Generation: a.generation, Attempt: a.attempt, Version: a.version, requestOff: a.provenanceRequestOff, requestLen: a.provenanceRequestLen, responseOff: a.provenanceResponseOff, responseLen: a.provenanceResponseLen, resultOff: a.provenanceResultOff, resultLen: a.provenanceResultLen, candidateCount: a.provenanceCandidateCount, borrowerCount: a.provenanceBorrowers, obligationBits: uint32(a.provenanceObligations)}
}
func (v restrictedValidatedDefinition) exact(m *restrictedOwnerManager) (*restrictedAttempt, error) {
	if m == nil || v.manager != m || int(v.slot) >= len(m.attempts) || v.state != restrictedProvenanceValidated || v.recordVersion != restrictedProvenanceRecordVersion || v.reserved != 0 {
		return nil, errRestrictedIdentity
	}
	a := &m.attempts[v.slot]
	if !a.used || a.provenanceState != restrictedProvenanceValidated || a.retired || a.quarantined || a.terminal || a.retentionDisposed || a.forgotten || a.owner != v.Owner || a.session != v.Session || a.generation != v.Generation || a.attempt != v.Attempt || a.version != v.Version || a.provenanceRecordVersion != v.recordVersion || a.provenanceChronology != v.chronologyTerminal || a.provenanceFlags != v.flags || a.provenanceRequestOff != v.requestOff || a.provenanceRequestLen != v.requestLen || a.provenanceResponseOff != v.responseOff || a.provenanceResponseLen != v.responseLen || a.provenanceResultOff != v.resultOff || a.provenanceResultLen != v.resultLen || a.provenanceCandidateCount != v.candidateCount || a.provenanceBorrowers != v.borrowerCount || uint32(a.provenanceObligations) != v.obligationBits {
		return nil, errRestrictedIdentity
	}
	for _, x := range [][2]uint32{{v.requestOff, v.requestLen}, {v.responseOff, v.responseLen}, {v.resultOff, v.resultLen}} {
		if uint64(x[0])+uint64(x[1]) > uint64(a.provenanceRawUsed) {
			return nil, errRestrictedIdentity
		}
	}
	return a, nil
}

// Production deliberately performs identity validation and construction from the
// manager-owned candidate set. The accepted owner methods still provide custody;
// a committed lease is returned on every later error so recovery is never lost.
func (m *restrictedOwnerManager) produceDefinition(p restrictedProcessingReservation, v restrictedValidatedDefinition, recipient restrictedRecipient) (restrictedLease, restrictedView, error) {
	if recipient.ID == 0 {
		return restrictedLease{}, restrictedView{}, errRestrictedIdentity
	}
	if !m.tryBoth() {
		return restrictedLease{}, restrictedView{}, errRestrictedBusy
	}
	a1, e1 := p.exact(m)
	a2, e2 := v.exact(m)
	same := e1 == nil && e2 == nil && a1 == a2 && p.slot == v.slot && p.Owner == v.Owner && p.Session == v.Session && p.Generation == v.Generation && p.Attempt == v.Attempt && p.Version == v.Version
	m.unlockBoth()
	if e1 != nil {
		return restrictedLease{}, restrictedView{}, e1
	}
	if e2 != nil {
		return restrictedLease{}, restrictedView{}, e2
	}
	if !same {
		return restrictedLease{}, restrictedView{}, errRestrictedIdentity
	}
	l, err := m.construct(p, a1.provenanceCandidates[:a1.provenanceCandidateCount])
	if err != nil {
		return restrictedLease{}, restrictedView{}, err
	}
	if err = m.accept(l, l.Transfer, recipient); err != nil {
		return l, restrictedView{}, err
	}
	view, err := m.view(l, recipient)
	if err != nil {
		return l, restrictedView{}, err
	}
	return l, view, nil
}
