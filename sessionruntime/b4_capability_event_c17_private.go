package sessionruntime

import (
	"unicode/utf16"
	"unicode/utf8"

	"lsp-trace/internal/session"
)

const (
	privateB4RegisterCapabilityMethod   = "client/registerCapability"
	privateB4UnregisterCapabilityMethod = "client/unregisterCapability"
	privateB4RegistrationsMember        = "registrations"
	privateB4UnregisterationsMember     = "unregisterations"
	privateB4CapabilityMaximumJSONDepth = 64
)

type privateB4CapabilityBatchC17 struct {
	Count uint64
}

type privateB4CapabilityStringC17 struct {
	start int
	end   int
}

type privateB4CapabilityEntryC17 struct {
	id     privateB4CapabilityStringC17
	method privateB4CapabilityStringC17
}

type privateB4CapabilityScannerC17 struct {
	src   []byte
	pos   int
	depth int
}

func parsePrivateB4CapabilityBatchC17(method string, params []byte) (privateB4CapabilityBatchC17, bool) {
	member := ""
	switch method {
	case privateB4RegisterCapabilityMethod:
		member = privateB4RegistrationsMember
	case privateB4UnregisterCapabilityMethod:
		member = privateB4UnregisterationsMember
	default:
		return privateB4CapabilityBatchC17{}, false
	}
	s := privateB4CapabilityScannerC17{src: params}
	s.space()
	objectStart := s.pos
	if !s.take('{') {
		return privateB4CapabilityBatchC17{}, false
	}
	found := false
	var count uint64
	s.space()
	if !s.peek('}') {
		for {
			keyStart := s.pos
			key, ok := s.string()
			if !ok || privateB4CapabilityPriorObjectKey(s.src, objectStart, keyStart, key) {
				return privateB4CapabilityBatchC17{}, false
			}
			s.space()
			if !s.take(':') {
				return privateB4CapabilityBatchC17{}, false
			}
			s.space()
			if privateB4CapabilityStringLiteralEqual(s.src, key, member) {
				found = true
				arrayStart := s.pos
				if !s.take('[') {
					return privateB4CapabilityBatchC17{}, false
				}
				s.space()
				if !s.peek(']') {
					for {
						entryStart := s.pos
						entry, valid := s.capabilityEntry()
						if !valid || privateB4CapabilityPriorIdentity(s.src, arrayStart, entryStart, entry) {
							return privateB4CapabilityBatchC17{}, false
						}
						count++
						s.space()
						if s.take(']') {
							break
						}
						if !s.take(',') {
							return privateB4CapabilityBatchC17{}, false
						}
						s.space()
					}
				} else {
					s.pos++
				}
			} else if !s.value() {
				return privateB4CapabilityBatchC17{}, false
			}
			s.space()
			if s.take('}') {
				break
			}
			if !s.take(',') {
				return privateB4CapabilityBatchC17{}, false
			}
			s.space()
		}
	} else {
		s.pos++
	}
	s.space()
	return privateB4CapabilityBatchC17{Count: count}, found && s.pos == len(s.src)
}

func (s *privateB4CapabilityScannerC17) capabilityEntry() (privateB4CapabilityEntryC17, bool) {
	start := s.pos
	if !s.take('{') {
		return privateB4CapabilityEntryC17{}, false
	}
	var entry privateB4CapabilityEntryC17
	hasID := false
	hasMethod := false
	s.space()
	if s.peek('}') {
		return privateB4CapabilityEntryC17{}, false
	}
	for {
		keyStart := s.pos
		key, ok := s.string()
		if !ok || privateB4CapabilityPriorObjectKey(s.src, start, keyStart, key) {
			return privateB4CapabilityEntryC17{}, false
		}
		s.space()
		if !s.take(':') {
			return privateB4CapabilityEntryC17{}, false
		}
		s.space()
		switch {
		case privateB4CapabilityStringLiteralEqual(s.src, key, "id"):
			entry.id, ok = s.string()
			hasID = ok && !privateB4CapabilityStringEmpty(s.src, entry.id)
			if !hasID {
				return privateB4CapabilityEntryC17{}, false
			}
		case privateB4CapabilityStringLiteralEqual(s.src, key, "method"):
			entry.method, ok = s.string()
			hasMethod = ok && !privateB4CapabilityStringEmpty(s.src, entry.method)
			if !hasMethod {
				return privateB4CapabilityEntryC17{}, false
			}
		default:
			if !s.value() {
				return privateB4CapabilityEntryC17{}, false
			}
		}
		s.space()
		if s.take('}') {
			break
		}
		if !s.take(',') {
			return privateB4CapabilityEntryC17{}, false
		}
		s.space()
	}
	return entry, hasID && hasMethod
}

func privateB4CapabilityPriorObjectKey(src []byte, objectStart, currentKeyStart int, current privateB4CapabilityStringC17) bool {
	s := privateB4CapabilityScannerC17{src: src, pos: objectStart}
	s.space()
	if !s.take('{') {
		return true
	}
	s.space()
	for s.pos < currentKeyStart {
		key, ok := s.string()
		if !ok {
			return true
		}
		if privateB4CapabilityStringsEqual(src, key, current) {
			return true
		}
		s.space()
		if !s.take(':') {
			return true
		}
		s.space()
		if !s.value() {
			return true
		}
		s.space()
		if s.pos >= currentKeyStart {
			break
		}
		if !s.take(',') {
			return true
		}
		s.space()
	}
	return false
}

func privateB4CapabilityPriorIdentity(src []byte, arrayStart, currentEntryStart int, current privateB4CapabilityEntryC17) bool {
	s := privateB4CapabilityScannerC17{src: src, pos: arrayStart}
	s.space()
	if !s.take('[') {
		return true
	}
	s.space()
	for s.pos < currentEntryStart {
		prior, ok := s.capabilityEntry()
		if !ok {
			return true
		}
		if privateB4CapabilityStringsEqual(src, prior.id, current.id) && privateB4CapabilityStringsEqual(src, prior.method, current.method) {
			return true
		}
		s.space()
		if s.pos >= currentEntryStart {
			break
		}
		if !s.take(',') {
			return true
		}
		s.space()
	}
	return false
}

func (s *privateB4CapabilityScannerC17) value() bool {
	if s.depth >= privateB4CapabilityMaximumJSONDepth || s.pos >= len(s.src) {
		return false
	}
	s.depth++
	defer func() { s.depth-- }()
	s.space()
	if s.pos >= len(s.src) {
		return false
	}
	switch s.src[s.pos] {
	case '"':
		_, ok := s.string()
		return ok
	case '{':
		s.pos++
		s.space()
		if s.take('}') {
			return true
		}
		for {
			if _, ok := s.string(); !ok {
				return false
			}
			s.space()
			if !s.take(':') {
				return false
			}
			s.space()
			if !s.value() {
				return false
			}
			s.space()
			if s.take('}') {
				return true
			}
			if !s.take(',') {
				return false
			}
			s.space()
		}
	case '[':
		s.pos++
		s.space()
		if s.take(']') {
			return true
		}
		for {
			if !s.value() {
				return false
			}
			s.space()
			if s.take(']') {
				return true
			}
			if !s.take(',') {
				return false
			}
			s.space()
		}
	case 't':
		return s.literal("true")
	case 'f':
		return s.literal("false")
	case 'n':
		return s.literal("null")
	default:
		return s.number()
	}
}

func (s *privateB4CapabilityScannerC17) string() (privateB4CapabilityStringC17, bool) {
	if !s.take('"') {
		return privateB4CapabilityStringC17{}, false
	}
	span := privateB4CapabilityStringC17{start: s.pos - 1}
	for s.pos < len(s.src) {
		b := s.src[s.pos]
		if b == '"' {
			s.pos++
			span.end = s.pos
			return span, true
		}
		if b < 0x20 {
			return privateB4CapabilityStringC17{}, false
		}
		if b == '\\' {
			s.pos++
			if s.pos >= len(s.src) {
				return privateB4CapabilityStringC17{}, false
			}
			switch s.src[s.pos] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				s.pos++
			case 'u':
				if !s.hex4() {
					return privateB4CapabilityStringC17{}, false
				}
			default:
				return privateB4CapabilityStringC17{}, false
			}
			continue
		}
		if b < utf8.RuneSelf {
			s.pos++
			continue
		}
		_, size := utf8.DecodeRune(s.src[s.pos:])
		if size == 1 {
			return privateB4CapabilityStringC17{}, false
		}
		s.pos += size
	}
	return privateB4CapabilityStringC17{}, false
}

func (s *privateB4CapabilityScannerC17) hex4() bool {
	if s.pos+5 > len(s.src) || s.src[s.pos] != 'u' {
		return false
	}
	for i := s.pos + 1; i < s.pos+5; i++ {
		if !privateB4CapabilityHex(s.src[i]) {
			return false
		}
	}
	s.pos += 5
	return true
}

func (s *privateB4CapabilityScannerC17) number() bool {
	start := s.pos
	if s.take('-') && s.pos >= len(s.src) {
		return false
	}
	if s.take('0') {
		if s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
			return false
		}
	} else {
		if s.pos >= len(s.src) || s.src[s.pos] < '1' || s.src[s.pos] > '9' {
			return false
		}
		for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
			s.pos++
		}
	}
	if s.take('.') {
		if s.pos >= len(s.src) || s.src[s.pos] < '0' || s.src[s.pos] > '9' {
			return false
		}
		for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
			s.pos++
		}
	}
	if s.pos < len(s.src) && (s.src[s.pos] == 'e' || s.src[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.src) && (s.src[s.pos] == '+' || s.src[s.pos] == '-') {
			s.pos++
		}
		if s.pos >= len(s.src) || s.src[s.pos] < '0' || s.src[s.pos] > '9' {
			return false
		}
		for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
			s.pos++
		}
	}
	return s.pos > start
}

func (s *privateB4CapabilityScannerC17) literal(value string) bool {
	if len(s.src)-s.pos < len(value) {
		return false
	}
	for i := range value {
		if s.src[s.pos+i] != value[i] {
			return false
		}
	}
	s.pos += len(value)
	return true
}

func (s *privateB4CapabilityScannerC17) space() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\r', '\n':
			s.pos++
		default:
			return
		}
	}
}

func (s *privateB4CapabilityScannerC17) take(b byte) bool {
	if s.pos >= len(s.src) || s.src[s.pos] != b {
		return false
	}
	s.pos++
	return true
}

func (s *privateB4CapabilityScannerC17) peek(b byte) bool {
	return s.pos < len(s.src) && s.src[s.pos] == b
}

func privateB4CapabilityStringsEqual(src []byte, a, b privateB4CapabilityStringC17) bool {
	ai, bi := a.start+1, b.start+1
	for {
		ar, an, aok := privateB4CapabilityStringRune(src, ai, a.end-1)
		br, bn, bok := privateB4CapabilityStringRune(src, bi, b.end-1)
		if aok != bok || (aok && ar != br) {
			return false
		}
		if !aok {
			return true
		}
		ai, bi = an, bn
	}
}

func privateB4CapabilityStringLiteralEqual(src []byte, span privateB4CapabilityStringC17, literal string) bool {
	i, j := span.start+1, 0
	for {
		r, next, ok := privateB4CapabilityStringRune(src, i, span.end-1)
		if !ok {
			return j == len(literal)
		}
		if j >= len(literal) {
			return false
		}
		lr, size := utf8.DecodeRuneInString(literal[j:])
		if r != lr {
			return false
		}
		i, j = next, j+size
	}
}

func privateB4CapabilityStringEmpty(src []byte, span privateB4CapabilityStringC17) bool {
	_, _, ok := privateB4CapabilityStringRune(src, span.start+1, span.end-1)
	return !ok
}

func privateB4CapabilityStringRune(src []byte, pos, end int) (rune, int, bool) {
	if pos >= end {
		return 0, pos, false
	}
	if src[pos] != '\\' {
		r, size := utf8.DecodeRune(src[pos:end])
		return r, pos + size, true
	}
	pos++
	switch src[pos] {
	case '"', '\\', '/':
		return rune(src[pos]), pos + 1, true
	case 'b':
		return '\b', pos + 1, true
	case 'f':
		return '\f', pos + 1, true
	case 'n':
		return '\n', pos + 1, true
	case 'r':
		return '\r', pos + 1, true
	case 't':
		return '\t', pos + 1, true
	case 'u':
		first := privateB4CapabilityHex4Value(src[pos+1 : pos+5])
		next := pos + 5
		if utf16.IsSurrogate(rune(first)) && next+6 <= end && src[next] == '\\' && src[next+1] == 'u' {
			second := privateB4CapabilityHex4Value(src[next+2 : next+6])
			decoded := utf16.DecodeRune(rune(first), rune(second))
			if decoded != utf8.RuneError {
				return decoded, next + 6, true
			}
		}
		return rune(first), next, true
	default:
		return utf8.RuneError, end, true
	}
}

func privateB4CapabilityHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func privateB4CapabilityHex4Value(src []byte) uint16 {
	var value uint16
	for _, b := range src {
		value <<= 4
		switch {
		case b >= '0' && b <= '9':
			value += uint16(b - '0')
		case b >= 'a' && b <= 'f':
			value += uint16(b-'a') + 10
		default:
			value += uint16(b-'A') + 10
		}
	}
	return value
}

func writePrivateB4CapabilityResponseC17(profile *privateB4EventAccountC17, method string, params []byte, responder func() (session.Failure, bool)) (session.Failure, bool) {
	if responder == nil {
		return session.SessionPoisoned, true
	}
	if profile == nil {
		return responder()
	}
	batch, valid := parsePrivateB4CapabilityBatchC17(method, params)
	if !valid || batch.Count == 0 {
		return responder()
	}
	if batch.Count > uint64(^uint(0)>>1) {
		return session.ResourceExhausted, false
	}
	return profile.withCapabilityBatchAdmission(int(batch.Count), responder)
}
