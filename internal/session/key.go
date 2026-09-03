package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type Value struct {
	tag   byte
	value any
}
type Entry struct {
	Name  string
	Value Value
}

func Null() Value { return Value{tag: 0} }
func Bool(v bool) Value {
	if v {
		return Value{tag: 2}
	}
	return Value{tag: 1}
}
func Uint(v uint64) Value   { return Value{tag: 3, value: v} }
func Int(v int64) Value     { return Value{tag: 4, value: v} }
func String(v string) Value { return Value{tag: 6, value: v} }
func Bytes(v []byte) Value  { return Value{tag: 5, value: append([]byte(nil), v...)} }
func Path(v string) Value   { return Value{tag: 7, value: v} }
func List(v ...Value) Value { return Value{tag: 8, value: append([]Value(nil), v...)} }
func Map(v ...Entry) Value  { return Value{tag: 9, value: append([]Entry(nil), v...)} }
func LanguageConfiguration(v ...Entry) Value {
	return Value{tag: 10, value: append([]Entry(nil), v...)}
}
func ServerAffectingOptions(v ...Entry) Value {
	return Value{tag: 11, value: append([]Entry(nil), v...)}
}

func frame(tag byte, p []byte) []byte {
	b := make([]byte, 9, len(p)+9)
	b[0] = tag
	binary.BigEndian.PutUint64(b[1:], uint64(len(p)))
	return append(b, p...)
}
func unsigned(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	i := 0
	for i < 7 && b[i] == 0 {
		i++
	}
	return append([]byte(nil), b[i:]...)
}
func signed(v int64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	i := 0
	if v >= 0 {
		for i < 7 && b[i] == 0 && b[i+1]&0x80 == 0 {
			i++
		}
	} else {
		for i < 7 && b[i] == 0xff && b[i+1]&0x80 != 0 {
			i++
		}
	}
	return append([]byte(nil), b[i:]...)
}
func canonicalString(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("invalid UTF-8")
	}
	return norm.NFC.String(s), nil
}
func canonicalPath(s string) (string, error) {
	n, e := canonicalString(strings.ReplaceAll(s, "\\", "/"))
	if e != nil {
		return "", e
	}
	if strings.HasPrefix(n, "//?/") || strings.HasPrefix(n, "//./") {
		return "", fmt.Errorf("device path")
	}
	cleanComponents := func(root string, components []string) (string, error) {
		stack := make([]string, 0, len(components))
		for _, component := range components {
			switch component {
			case "", ".":
				continue
			case "..":
				if len(stack) == 0 {
					return "", fmt.Errorf("path crosses root")
				}
				stack = stack[:len(stack)-1]
			default:
				if strings.Contains(component, ":") {
					return "", fmt.Errorf("alternate data stream")
				}
				stack = append(stack, component)
			}
		}
		if len(stack) == 0 {
			return root, nil
		}
		return root + strings.Join(stack, "/"), nil
	}
	if strings.HasPrefix(n, "//") {
		parts := strings.Split(strings.TrimPrefix(n, "//"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" || parts[0] == "." || parts[1] == "." || parts[0] == ".." || parts[1] == ".." {
			return "", fmt.Errorf("invalid UNC path")
		}
		root := "//" + parts[0] + "/" + parts[1]
		if len(parts) == 2 {
			return root, nil
		}
		return cleanComponents(root+"/", parts[2:])
	}
	if len(n) >= 2 && n[1] == ':' {
		if !((n[0] >= 'a' && n[0] <= 'z') || (n[0] >= 'A' && n[0] <= 'Z')) || len(n) < 3 || n[2] != '/' {
			return "", fmt.Errorf("drive-relative path")
		}
		if n[0] >= 'a' && n[0] <= 'z' {
			n = string(n[0]-32) + n[1:]
		}
		return cleanComponents(n[:3], strings.Split(n[3:], "/"))
	}
	if !strings.HasPrefix(n, "/") {
		return "", fmt.Errorf("path is not absolute")
	}
	return cleanComponents("/", strings.Split(strings.TrimPrefix(n, "/"), "/"))
}
func encodeMap(tag byte, entries []Entry) ([]byte, error) {
	if tag == 10 {
		if len(entries) != 3 {
			return nil, fmt.Errorf("language configuration requires exactly three entries")
		}
		names := map[string]bool{}
		for _, entry := range entries {
			names[entry.Name] = true
		}
		for _, required := range []string{"language_id", "initialization_options", "workspace_configuration"} {
			if !names[required] {
				return nil, fmt.Errorf("language configuration missing %q", required)
			}
		}
	}
	type pair struct{ k, v []byte }
	ps := make([]pair, 0, len(entries))
	for _, e := range entries {
		ks, err := canonicalString(e.Name)
		if err != nil {
			return nil, err
		}
		k, _ := String(ks).Encode()
		v, err := e.Value.Encode()
		if err != nil {
			return nil, err
		}
		ps = append(ps, pair{k, v})
	}
	sort.Slice(ps, func(i, j int) bool { return bytes.Compare(ps[i].k, ps[j].k) < 0 })
	p := make([]byte, 8)
	binary.BigEndian.PutUint64(p, uint64(len(ps)))
	for i, x := range ps {
		if i > 0 && bytes.Equal(ps[i-1].k, x.k) {
			return nil, fmt.Errorf("duplicate encoded map key")
		}
		p = append(p, x.k...)
		p = append(p, x.v...)
	}
	return frame(tag, p), nil
}
func (v Value) Encode() ([]byte, error) {
	switch v.tag {
	case 0, 1, 2:
		if v.value != nil {
			return nil, fmt.Errorf("tag %02x has payload", v.tag)
		}
		return frame(v.tag, nil), nil
	case 3:
		x, ok := v.value.(uint64)
		if !ok {
			return nil, fmt.Errorf("invalid unsigned payload")
		}
		return frame(3, unsigned(x)), nil
	case 4:
		x, ok := v.value.(int64)
		if !ok {
			return nil, fmt.Errorf("invalid signed payload")
		}
		return frame(4, signed(x)), nil
	case 5:
		x, ok := v.value.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid bytes payload")
		}
		return frame(5, x), nil
	case 6:
		x, ok := v.value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid string payload")
		}
		s, e := canonicalString(x)
		if e != nil {
			return nil, e
		}
		return frame(6, []byte(s)), nil
	case 7:
		x, ok := v.value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid path payload")
		}
		s, e := canonicalPath(x)
		if e != nil {
			return nil, e
		}
		return frame(7, []byte(s)), nil
	case 8:
		vals, ok := v.value.([]Value)
		if !ok {
			return nil, fmt.Errorf("invalid list payload")
		}
		p := make([]byte, 8)
		binary.BigEndian.PutUint64(p, uint64(len(vals)))
		for _, x := range vals {
			b, e := x.Encode()
			if e != nil {
				return nil, e
			}
			p = append(p, b...)
		}
		return frame(8, p), nil
	case 9, 10, 11:
		entries, ok := v.value.([]Entry)
		if !ok {
			return nil, fmt.Errorf("invalid map payload")
		}
		return encodeMap(v.tag, entries)
	default:
		return nil, fmt.Errorf("unknown tag %02x", v.tag)
	}
}

var sessionKeyV1Prefix = append([]byte("lsp-trace/session-key\x00"), 0, 0, 0, 0, 0, 0, 0, 1)
var sessionKeyV1Tags = [8]byte{6, 7, 6, 6, 8, 5, 10, 11}

func EncodeSessionKeyV1(values [8]Value) ([]byte, error) {
	b := append([]byte(nil), sessionKeyV1Prefix...)
	for i, v := range values {
		if v.tag != sessionKeyV1Tags[i] {
			return nil, fmt.Errorf("session key component %d: got tag %02x want %02x", i, v.tag, sessionKeyV1Tags[i])
		}
		x, e := v.Encode()
		if e != nil {
			return nil, fmt.Errorf("session key component %d: %w", i, e)
		}
		b = append(b, x...)
	}
	return b, nil
}

func DecodeSessionKeyV1(encoded []byte) ([8]Value, error) {
	var values [8]Value
	if len(encoded) < len(sessionKeyV1Prefix) || !bytes.Equal(encoded[:len(sessionKeyV1Prefix)], sessionKeyV1Prefix) {
		return values, fmt.Errorf("unknown session key encoding version")
	}
	offset := len(sessionKeyV1Prefix)
	for i := range values {
		value, next, err := decodeValue(encoded, offset)
		if err != nil {
			return values, fmt.Errorf("session key component %d: %w", i, err)
		}
		if value.tag != sessionKeyV1Tags[i] {
			return values, fmt.Errorf("session key component %d: got tag %02x want %02x", i, value.tag, sessionKeyV1Tags[i])
		}
		values[i], offset = value, next
	}
	if offset != len(encoded) {
		return values, fmt.Errorf("trailing session key bytes")
	}
	reencoded, err := EncodeSessionKeyV1(values)
	if err != nil {
		return values, err
	}
	if !bytes.Equal(reencoded, encoded) {
		return values, fmt.Errorf("non-canonical session key encoding")
	}
	return values, nil
}

func decodeValue(encoded []byte, offset int) (Value, int, error) {
	if offset > len(encoded)-9 {
		return Value{}, offset, fmt.Errorf("truncated value")
	}
	tag := encoded[offset]
	length := binary.BigEndian.Uint64(encoded[offset+1 : offset+9])
	offset += 9
	if length > uint64(len(encoded)-offset) {
		return Value{}, offset, fmt.Errorf("truncated payload")
	}
	end := offset + int(length)
	payload := encoded[offset:end]
	switch tag {
	case 0, 1, 2:
		if len(payload) != 0 {
			return Value{}, offset, fmt.Errorf("boolean/null payload must be empty")
		}
		return Value{tag: tag}, end, nil
	case 3:
		if len(payload) == 0 || len(payload) > 8 {
			return Value{}, offset, fmt.Errorf("invalid unsigned length")
		}
		var b [8]byte
		copy(b[8-len(payload):], payload)
		return Uint(binary.BigEndian.Uint64(b[:])), end, nil
	case 4:
		if len(payload) == 0 || len(payload) > 8 {
			return Value{}, offset, fmt.Errorf("invalid signed length")
		}
		var b [8]byte
		fill := byte(0)
		if payload[0]&0x80 != 0 {
			fill = 0xff
		}
		for i := 0; i < 8-len(payload); i++ {
			b[i] = fill
		}
		copy(b[8-len(payload):], payload)
		return Int(int64(binary.BigEndian.Uint64(b[:]))), end, nil
	case 5:
		return Bytes(payload), end, nil
	case 6, 7:
		if !utf8.Valid(payload) || !norm.NFC.IsNormal(payload) {
			return Value{}, offset, fmt.Errorf("invalid UTF-8 or non-NFC")
		}
		if tag == 6 {
			return String(string(payload)), end, nil
		}
		canonical, err := canonicalPath(string(payload))
		if err != nil || canonical != string(payload) {
			return Value{}, offset, fmt.Errorf("non-canonical path")
		}
		return Path(string(payload)), end, nil
	case 8:
		if len(payload) < 8 {
			return Value{}, offset, fmt.Errorf("truncated list count")
		}
		count := binary.BigEndian.Uint64(payload)
		if count > uint64((len(payload)-8)/9) {
			return Value{}, offset, fmt.Errorf("invalid list count")
		}
		p := offset + 8
		vals := make([]Value, 0, int(count))
		for j := uint64(0); j < count; j++ {
			x, next, err := decodeValue(encoded, p)
			if err != nil || next > end {
				return Value{}, offset, fmt.Errorf("invalid list item")
			}
			vals = append(vals, x)
			p = next
		}
		if p != end {
			return Value{}, offset, fmt.Errorf("trailing list payload")
		}
		return List(vals...), end, nil
	case 9, 10, 11:
		if len(payload) < 8 {
			return Value{}, offset, fmt.Errorf("truncated map count")
		}
		count := binary.BigEndian.Uint64(payload)
		if count > uint64((len(payload)-8)/18) {
			return Value{}, offset, fmt.Errorf("invalid map count")
		}
		p := offset + 8
		entries := make([]Entry, 0, int(count))
		for j := uint64(0); j < count; j++ {
			k, next, err := decodeValue(encoded, p)
			if err != nil || k.tag != 6 {
				return Value{}, offset, fmt.Errorf("invalid map key")
			}
			val, next2, err := decodeValue(encoded, next)
			if err != nil || next2 > end {
				return Value{}, offset, fmt.Errorf("invalid map value")
			}
			entries = append(entries, Entry{Name: k.value.(string), Value: val})
			p = next2
		}
		if p != end {
			return Value{}, offset, fmt.Errorf("trailing map payload")
		}
		if tag == 9 {
			return Map(entries...), end, nil
		}
		if tag == 10 {
			return LanguageConfiguration(entries...), end, nil
		}
		return ServerAffectingOptions(entries...), end, nil
	default:
		return Value{}, offset, fmt.Errorf("unknown tag %02x", tag)
	}
}
func EnvironmentValue(processKey []byte, entries map[string][]byte) (Value, error) {
	type environmentEntry struct {
		name  string
		value []byte
	}
	canonical := make([]environmentEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for rawName, value := range entries {
		name, err := canonicalString(rawName)
		if err != nil {
			return Value{}, fmt.Errorf("environment name: %w", err)
		}
		if _, duplicate := seen[name]; duplicate {
			return Value{}, fmt.Errorf("duplicate canonical environment name %q", name)
		}
		seen[name] = struct{}{}
		canonical = append(canonical, environmentEntry{name: name, value: value})
	}
	sort.Slice(canonical, func(i, j int) bool {
		return bytes.Compare([]byte(canonical[i].name), []byte(canonical[j].name)) < 0
	})
	items := make([]Value, 0, len(canonical))
	for _, entry := range canonical {
		name := String(entry.name)
		runtime := Bytes(entry.value)
		nb, err := name.Encode()
		if err != nil {
			return Value{}, fmt.Errorf("environment name: %w", err)
		}
		rb, err := runtime.Encode()
		if err != nil {
			return Value{}, fmt.Errorf("environment value: %w", err)
		}
		domain := append([]byte("lsp-trace/environment-entry/v1\x00"), nb...)
		domain = append(domain, rb...)
		h := hmacSHA256(processKey, domain)
		items = append(items, List(name, Bytes(h)))
	}
	encoded, err := List(items...).Encode()
	if err != nil {
		return Value{}, fmt.Errorf("environment entries: %w", err)
	}
	domain := append([]byte("lsp-trace/process-context/v1\x00"), encoded...)
	return Bytes(hmacSHA256(processKey, domain)), nil
}
func hmacSHA256(key, msg []byte) []byte {
	const block = 64
	k := append([]byte(nil), key...)
	if len(k) > block {
		s := sha256.Sum256(k)
		k = s[:]
	}
	k = append(k, make([]byte, block-len(k))...)
	inner, outer := make([]byte, block), make([]byte, block)
	for i := range k {
		inner[i] = k[i] ^ 0x36
		outer[i] = k[i] ^ 0x5c
	}
	ih := sha256.Sum256(append(inner, msg...))
	oh := sha256.Sum256(append(outer, ih[:]...))
	return oh[:]
}
func DecodeBase64(s string) (Value, error) {
	b, e := base64.StdEncoding.Strict().DecodeString(s)
	if e != nil {
		return Value{}, e
	}
	if base64.StdEncoding.EncodeToString(b) != s {
		return Value{}, fmt.Errorf("non-canonical base64")
	}
	return Bytes(b), nil
}
func DigestSessionKey(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
func Reusable(ap string, a []byte, bp string, b []byte) bool { return ap == bp && bytes.Equal(a, b) }
