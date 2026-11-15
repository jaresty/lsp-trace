package lspwire

// Discovery's codec is deliberately separate from the ordinary JSON decoder.
// Its logical ledger covers requested backing/string payloads and byte visits,
// not allocator rounding, physical heap, goroutine stacks, or typed results.
import (
	"context"
	"errors"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var ErrPrivateDiscoveryAdmission = errors.New("private discovery admission refused")

// PrivateDiscoveryCharge is a request, never a funding receipt. The runtime
// binds the callback to its concrete child and remaining invocation balances.
type PrivateDiscoveryCharge struct{ Messages, Retained, Work uint64 }
type PrivateDiscoveryAdmission func(PrivateDiscoveryCharge) error

func discoveryPay(ctx context.Context, admit PrivateDiscoveryAdmission, q PrivateDiscoveryCharge) (err error) {
	if ctx == nil || admit == nil || ctx.Err() != nil {
		return ErrPrivateDiscoveryAdmission
	}
	defer func() {
		if recover() != nil {
			err = ErrPrivateDiscoveryAdmission
		}
	}()
	if err = admit(q); err != nil {
		return ErrPrivateDiscoveryAdmission
	}
	if ctx.Err() != nil {
		return ErrPrivateDiscoveryAdmission
	}
	return nil
}

const discoveryDepth = 10000

// Slots: version, ID, method, params, result, error code, message, data.
type discoverySpan struct{ start, end int }
type discoveryPlan struct {
	slots                   [8]discoverySpan
	errorPresent, typeError bool
	code                    int
	// Key/scalar scratch is fixed and included in compiler-derived P even though
	// matching itself streams. No occurrence-count-dependent state exists.
	key    [7]rune
	scalar [4]byte
}

const discoveryPlanBytes = uint64(unsafe.Sizeof(discoveryPlan{}))

type discoveryScanner struct {
	b      []byte
	states []byte
}

func discoverySpace(c byte) bool { return c == ' ' || c == '\n' || c == '\r' || c == '\t' }
func (s discoveryScanner) ws(i int) int {
	for i < len(s.b) && discoverySpace(s.b[i]) {
		i++
	}
	return i
}
func discoveryHex(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c-'a') + 10
	}
	if c >= 'A' && c <= 'F' {
		return int(c-'A') + 10
	}
	return -1
}
func (s discoveryScanner) str(i int) (int, bool) {
	if i >= len(s.b) || s.b[i] != '"' {
		return i, false
	}
	i++
	for i < len(s.b) {
		c := s.b[i]
		i++
		if c == '"' {
			return i, true
		}
		if c < 0x20 {
			return i, false
		}
		if c == '\\' {
			if i >= len(s.b) {
				return i, false
			}
			c = s.b[i]
			i++
			switch c {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				if len(s.b)-i < 4 {
					return i, false
				}
				for j := 0; j < 4; j++ {
					if discoveryHex(s.b[i+j]) < 0 {
						return i, false
					}
				}
				i += 4
			default:
				return i, false
			}
		}
	}
	return i, false
}
func (s discoveryScanner) number(i int) (int, bool) {
	n := len(s.b)
	if i < n && s.b[i] == '-' {
		i++
	}
	if i >= n {
		return i, false
	}
	if s.b[i] == '0' {
		i++
	} else {
		if s.b[i] < '1' || s.b[i] > '9' {
			return i, false
		}
		for i < n && s.b[i] >= '0' && s.b[i] <= '9' {
			i++
		}
	}
	if i < n && s.b[i] == '.' {
		i++
		start := i
		for i < n && s.b[i] >= '0' && s.b[i] <= '9' {
			i++
		}
		if start == i {
			return i, false
		}
	}
	if i < n && (s.b[i] == 'e' || s.b[i] == 'E') {
		i++
		if i < n && (s.b[i] == '+' || s.b[i] == '-') {
			i++
		}
		start := i
		for i < n && s.b[i] >= '0' && s.b[i] <= '9' {
			i++
		}
		if start == i {
			return i, false
		}
	}
	return i, true
}

// value validates exactly one JSON value nonrecursively. States encode object
// key/colon/value/comma and array value/comma; depth includes every container.
func (s discoveryScanner) value(start int) (int, bool) {
	i := s.ws(start)
	depth := 0
	want := true
	for {
		if want {
			i = s.ws(i)
			if i >= len(s.b) {
				return i, false
			}
			c := s.b[i]
			switch c {
			case '{', '[':
				if depth == len(s.states) {
					return i, false
				}
				if c == '{' {
					s.states[depth] = 1
				} else {
					s.states[depth] = 5
				}
				depth++
				i++
				want = false
				continue
			case '"':
				var ok bool
				i, ok = s.str(i)
				if !ok {
					return i, false
				}
			case 't', 'f', 'n':
				word := "null"
				if c == 't' {
					word = "true"
				} else if c == 'f' {
					word = "false"
				}
				if len(s.b)-i < len(word) {
					return i, false
				}
				for j := 0; j < len(word); j++ {
					if s.b[i+j] != word[j] {
						return i, false
					}
				}
				i += len(word)
			default:
				var ok bool
				i, ok = s.number(i)
				if !ok {
					return i, false
				}
			}
			want = false
			if depth == 0 {
				return i, true
			}
			if s.states[depth-1] == 3 {
				s.states[depth-1] = 4
			} else if s.states[depth-1] == 6 {
				s.states[depth-1] = 7
			} else {
				return i, false
			}
		}
		if depth == 0 {
			return i, true
		}
		i = s.ws(i)
		if i >= len(s.b) {
			return i, false
		}
		switch s.states[depth-1] {
		case 1, 2:
			if s.states[depth-1] == 1 && s.b[i] == '}' {
				depth--
				i++
				break
			}
			var ok bool
			i, ok = s.str(i)
			if !ok {
				return i, false
			}
			s.states[depth-1] = 8
			continue
		case 8:
			if s.b[i] != ':' {
				return i, false
			}
			i++
			s.states[depth-1] = 3
			want = true
			continue
		case 4:
			if s.b[i] == ',' {
				i++
				s.states[depth-1] = 2
				continue
			}
			if s.b[i] != '}' {
				return i, false
			}
			depth--
			i++
		case 5:
			if s.b[i] == ']' {
				depth--
				i++
				break
			}
			s.states[depth-1] = 6
			want = true
			continue
		case 9:
			s.states[depth-1] = 6
			want = true
			continue
		case 7:
			if s.b[i] == ',' {
				i++
				s.states[depth-1] = 9
				continue
			}
			if s.b[i] != ']' {
				return i, false
			}
			depth--
			i++
		default:
			return i, false
		}
		// Completed a container; mark its parent's pending value complete.
		if depth == 0 {
			return i, true
		}
		if s.states[depth-1] == 3 {
			s.states[depth-1] = 4
		} else if s.states[depth-1] == 6 {
			s.states[depth-1] = 7
		} else {
			return i, false
		}
	}
}

// discoveryRune streams one decoded JSON string rune. Syntax was validated.
// Invalid UTF-8 and lone surrogates become RuneError exactly as JSON v1 does.
func discoveryRune(b []byte, i int) (rune, int) {
	c := b[i]
	if c != '\\' {
		r, n := utf8.DecodeRune(b[i:])
		return r, i + n
	}
	i++
	c = b[i]
	i++
	switch c {
	case '"', '\\', '/':
		return rune(c), i
	case 'b':
		return '\b', i
	case 'f':
		return '\f', i
	case 'n':
		return '\n', i
	case 'r':
		return '\r', i
	case 't':
		return '\t', i
	}
	r := rune(0)
	for j := 0; j < 4; j++ {
		r = r*16 + rune(discoveryHex(b[i+j]))
	}
	i += 4
	if r >= 0xD800 && r <= 0xDBFF && len(b)-i >= 6 && b[i] == '\\' && b[i+1] == 'u' {
		lo := rune(0)
		valid := true
		for j := 0; j < 4; j++ {
			h := discoveryHex(b[i+2+j])
			if h < 0 {
				valid = false
			}
			lo = lo*16 + rune(h)
		}
		if valid && lo >= 0xDC00 && lo <= 0xDFFF {
			return utf16.DecodeRune(r, lo), i + 6
		}
	}
	if r >= 0xD800 && r <= 0xDFFF {
		r = utf8.RuneError
	}
	return r, i
}
func discoveryFold(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	if r < utf8.RuneSelf {
		return r
	}
	for {
		next := unicode.SimpleFold(r)
		if next <= r {
			return next
		}
		r = next
	}
}
func discoveryKey(b []byte, sp discoverySpan, key string) bool {
	i, j := sp.start+1, 0
	for i < sp.end-1 && j < len(key) {
		r, next := discoveryRune(b, i)
		if discoveryFold(r) != discoveryFold(rune(key[j])) {
			return false
		}
		i = next
		j++
	}
	return i == sp.end-1 && j == len(key)
}
func discoverySlot(b []byte, k discoverySpan, nested bool) int {
	if nested {
		if discoveryKey(b, k, "code") {
			return 5
		}
		if discoveryKey(b, k, "message") {
			return 6
		}
		if discoveryKey(b, k, "data") {
			return 7
		}
		return -1
	}
	if discoveryKey(b, k, "jsonrpc") {
		return 0
	}
	if discoveryKey(b, k, "id") {
		return 1
	}
	if discoveryKey(b, k, "method") {
		return 2
	}
	if discoveryKey(b, k, "params") {
		return 3
	}
	if discoveryKey(b, k, "result") {
		return 4
	}
	if discoveryKey(b, k, "error") {
		return 8
	}
	return -1
}
func discoveryInt(b []byte) (int, bool) {
	i := 0
	neg := false
	if len(b) > 0 && b[0] == '-' {
		neg = true
		i++
	}
	limit := uint64(^uint(0) >> 1)
	if neg {
		limit++
	}
	var n uint64
	if i == len(b) {
		return 0, false
	}
	for ; i < len(b); i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, false
		}
		d := uint64(b[i] - '0')
		if n > (limit-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	if neg {
		return -int(n), true
	}
	return int(n), true
}
func (s discoveryScanner) object(p *discoveryPlan, start int, nested bool) {
	i := s.ws(start + 1)
	for i < len(s.b) && s.b[i] != '}' {
		end, _ := s.str(i)
		key := discoverySpan{i, end}
		i = s.ws(end)
		i = s.ws(i + 1)
		end, _ = s.value(i)
		v := discoverySpan{i, end}
		slot := discoverySlot(s.b, key, nested)
		if slot >= 0 {
			c := s.b[i]
			switch slot {
			case 0, 2, 6:
				if c == '"' {
					p.slots[slot] = v
				} else if c != 'n' {
					p.typeError = true
				}
			case 5:
				if c != 'n' {
					n, ok := discoveryInt(s.b[i:end])
					if !ok {
						p.typeError = true
					} else {
						p.code = n
					}
				}
			case 8:
				if c == 'n' {
					p.errorPresent = false
					p.code = 0
					p.slots[6] = discoverySpan{}
					p.slots[7] = discoverySpan{}
				} else if c == '{' {
					p.errorPresent = true
					s.objectError(p, i)
				} else {
					p.typeError = true
				}
			default:
				p.slots[slot] = v
			}
		}
		i = s.ws(end)
		if i < len(s.b) && s.b[i] == ',' {
			i = s.ws(i + 1)
		} else {
			break
		}
	}
}

// Explicit separate entry: schema depth is fixed at two; grammar traversal is
// nonrecursive even for unknown nested data. There is no schema recursion.
func (s discoveryScanner) objectError(p *discoveryPlan, start int) {
	i := s.ws(start + 1)
	for i < len(s.b) && s.b[i] != '}' {
		end, _ := s.str(i)
		key := discoverySpan{i, end}
		i = s.ws(end)
		i = s.ws(i + 1)
		end, _ = s.value(i)
		switch discoverySlot(s.b, key, true) {
		case 5:
			if s.b[i] != 'n' {
				v, ok := discoveryInt(s.b[i:end])
				if !ok {
					p.typeError = true
				} else {
					p.code = v
				}
			}
		case 6:
			if s.b[i] == '"' {
				p.slots[6] = discoverySpan{i, end}
			} else if s.b[i] != 'n' {
				p.typeError = true
			}
		case 7:
			p.slots[7] = discoverySpan{i, end}
		}
		i = s.ws(end)
		if i < len(s.b) && s.b[i] == ',' {
			i = s.ws(i + 1)
		} else {
			break
		}
	}
}
func discoveryStringLen(b []byte, sp discoverySpan) int {
	n := 0
	for i := sp.start + 1; i < sp.end-1; {
		r, next := discoveryRune(b, i)
		n += utf8.RuneLen(r)
		i = next
	}
	return n
}
func discoveryString(b []byte, sp discoverySpan) string {
	if sp.end == 0 {
		return ""
	}
	n := discoveryStringLen(b, sp)
	out := make([]byte, n)
	at := 0
	for i := sp.start + 1; i < sp.end-1; {
		r, next := discoveryRune(b, i)
		at += utf8.EncodeRune(out[at:], r)
		i = next
	}
	return string(out)
}
func discoveryRaw(b []byte, sp discoverySpan) []byte {
	if sp.end == 0 {
		return nil
	}
	out := make([]byte, sp.end-sp.start)
	copy(out, b[sp.start:sp.end])
	return out
}

// Count-only encoder. Raw spans are already valid: strip only JSON whitespace
// outside strings and apply appendCompact's HTML/line-separator escaping while
// preserving raw invalid UTF-8. Typed strings use encoding/json's UTF-8 rules.
// Saturating checked accumulation: the sentinel is rejected before returning a
// record. Real bounded frames cannot reach it; helpers still fail closed.
func discoveryCountAdd(n, delta uint64) uint64 {
	if delta > ^uint64(0)-n {
		return ^uint64(0)
	}
	return n + delta
}
func discoveryCountString(s string) uint64 {
	n := uint64(2)
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			switch c {
			case '"', '\\', '\n', '\r', '\t', '\b', '\f':
				n = discoveryCountAdd(n, 2)
			case '<', '>', '&':
				n = discoveryCountAdd(n, 6)
			default:
				if c < 32 {
					n = discoveryCountAdd(n, 6)
				} else {
					n = discoveryCountAdd(n, 1)
				}
			}
			continue
		}
		r, k := utf8.DecodeRuneInString(s[i:])
		i += k
		if r == utf8.RuneError && k == 1 {
			n = discoveryCountAdd(n, 6)
		} else if r == 0x2028 || r == 0x2029 {
			n = discoveryCountAdd(n, 6)
		} else {
			n = discoveryCountAdd(n, uint64(k))
		}
	}
	return n
}
func discoveryCountRaw(b []byte) uint64 {
	var n uint64
	inside, escape := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if !inside && discoverySpace(c) {
			continue
		}
		if c == '<' || c == '>' || c == '&' {
			n = discoveryCountAdd(n, 6)
		} else if c == 0xe2 && len(b)-i >= 3 && b[i+1] == 0x80 && (b[i+2] == 0xa8 || b[i+2] == 0xa9) {
			n = discoveryCountAdd(n, 6)
			i += 2
		} else {
			n = discoveryCountAdd(n, 1)
		}
		if inside {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inside = false
			}
		} else if c == '"' {
			inside = true
		}
	}
	return n
}
func discoveryIntDigits(v int) uint64 {
	n := uint64(1)
	var u uint64
	if v < 0 {
		n++
		u = uint64(-(v + 1)) + 1
	} else {
		u = uint64(v)
	}
	for u >= 10 {
		n++
		u /= 10
	}
	return n
}
func discoveryCanonicalCount(m Message) uint64 {
	n := discoveryCountAdd(uint64(len(`{"jsonrpc":`)+1), discoveryCountString(m.JSONRPC))
	if len(m.ID) > 0 {
		n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"id":`)), discoveryCountRaw(m.ID)))
	}
	if m.Method != "" {
		n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"method":`)), discoveryCountString(m.Method)))
	}
	if len(m.Params) > 0 {
		n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"params":`)), discoveryCountRaw(m.Params)))
	}
	if len(m.Result) > 0 {
		n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"result":`)), discoveryCountRaw(m.Result)))
	}
	if m.Error != nil {
		n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"error":{"code":`)+len(`,"message":`)+1), discoveryCountAdd(discoveryIntDigits(m.Error.Code), discoveryCountString(m.Error.Message))))
		if len(m.Error.Data) > 0 {
			n = discoveryCountAdd(n, discoveryCountAdd(uint64(len(`,"data":`)), discoveryCountRaw(m.Error.Data)))
		}
	}
	return n
}

func decodePrivateDiscovery(ctx context.Context, b []byte, admit PrivateDiscoveryAdmission) (Message, uint64, error) {
	// P and D are charged BEFORE either fixed store is allocated/zeroed. Work:
	// initialization D+P; grammar <=2B; schema <=3B plus <=9 key comparisons,
	// each at most 7 rune-fold steps and 6-byte escapes; 128B covers these visits.
	if uint64(len(b)) > (^uint64(0)-discoveryDepth-discoveryPlanBytes)/128 {
		return Message{}, 0, ErrPrivateDiscoveryAdmission
	}
	if err := discoveryPay(ctx, admit, PrivateDiscoveryCharge{Retained: discoveryDepth + discoveryPlanBytes, Work: discoveryDepth + discoveryPlanBytes + 128*uint64(len(b))}); err != nil {
		return Message{}, 0, err
	}
	states := make([]byte, discoveryDepth)
	p := new(discoveryPlan)
	s := discoveryScanner{b, states}
	end, ok := s.value(0)
	if !ok || s.ws(end) != len(b) {
		return Message{}, 0, ErrMalformedJSON
	}
	start := s.ws(0)
	if b[start] == '{' {
		s.object(p, start, false)
	} else if b[start] != 'n' {
		return Message{}, 0, ErrMalformedJSON
	}
	if p.typeError {
		return Message{}, 0, ErrMalformedJSON
	}
	var raw, decoded uint64
	for i, sp := range p.slots {
		if sp.end == 0 {
			continue
		}
		if i == 0 || i == 2 || i == 6 {
			decoded += uint64(discoveryStringLen(b, sp))
		} else if i != 5 {
			raw += uint64(sp.end - sp.start)
		}
	}
	retained := raw + 2*decoded
	if p.errorPresent {
		retained += uint64(unsafe.Sizeof(RPCError{}))
	}
	// String length traversal + exact decode/copy/conversion + canonical visits.
	// Raw and strings are disjoint surviving spans; count output <=6B+constant.
	if err := discoveryPay(ctx, admit, PrivateDiscoveryCharge{Retained: retained, Work: 16*uint64(len(b)) + 2*decoded + raw + 256}); err != nil {
		return Message{}, 0, err
	}
	m := Message{JSONRPC: discoveryString(b, p.slots[0]), ID: discoveryRaw(b, p.slots[1]), Method: discoveryString(b, p.slots[2]), Params: discoveryRaw(b, p.slots[3]), Result: discoveryRaw(b, p.slots[4])}
	if p.errorPresent {
		m.Error = &RPCError{Code: p.code, Message: discoveryString(b, p.slots[6]), Data: discoveryRaw(b, p.slots[7])}
	}
	if m.JSONRPC != Version {
		return Message{}, 0, ErrWrongVersion
	}
	if m.Kind() == KindInvalid {
		return Message{}, 0, ErrInvalidMessage
	}
	count := discoveryCanonicalCount(m)
	if count == ^uint64(0) {
		return Message{}, 0, ErrPrivateDiscoveryAdmission
	}
	return m, count, nil
}
