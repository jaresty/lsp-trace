package adr0011acquisition

import (
	"errors"
	"strconv"
	"strings"

	"lsp-trace/internal/publication"
)

// rawScannerObservation is deliberately inert: no implementation-byte pin or
// accepted dependency refs exist yet, so it is not a schema scanner receipt.
type rawScannerObservation struct {
	selector, digest     string
	byteLength           int
	implementationDigest string
	form                 string
	knownE, workUnits    int
}

var errRawScanner = errors.New("private raw scanner input not verified")

func observeRawScanner(root *publication.Root, raw privateBodyPublication, read func(*publication.Root, string, int64) ([]byte, error)) (rawScannerObservation, error) {
	var absent rawScannerObservation
	if root == nil || raw.stage != "VERIFIED" || raw.byteCount < 1 || raw.byteCount > rawResultPayloadLimit || raw.digest == "" || raw.selector != "adr0011-references-raw-payload-v1-"+strings.TrimPrefix(raw.digest, "sha256:")+".bin" {
		return absent, errRawScanner
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	data, err := read(root, raw.selector, rawResultPayloadLimit)
	if err != nil || len(data) != raw.byteCount || privateDigest(data) != raw.digest {
		return absent, errRawScanner
	}
	observed := scanRawTopLevel(data)
	observed.selector, observed.digest, observed.byteLength = raw.selector, raw.digest, len(data)
	// MALFORMED remains an in-memory diagnostic, not a success observation.
	if observed.form == "MALFORMED" {
		return observed, errRawScanner
	}
	return observed, nil
}

type rawCursor struct {
	b            []byte
	i, pending   int
	invalid      bool
	first        byte
	firstPending bool
}

func (c *rawCursor) take() byte {
	v := c.b[c.i]
	c.i++
	if c.pending > 0 {
		if v < 0x80 || v > 0xbf || (c.firstPending && ((c.first == 0xe0 && v < 0xa0) || (c.first == 0xed && v > 0x9f) || (c.first == 0xf0 && v < 0x90) || (c.first == 0xf4 && v > 0x8f))) {
			c.invalid = true
		}
		c.pending--
		c.firstPending = false
	} else if v >= 0x80 {
		c.first = v
		c.firstPending = true
		switch {
		case v >= 0xc2 && v <= 0xdf:
			c.pending = 1
		case v >= 0xe0 && v <= 0xef:
			c.pending = 2
		case v >= 0xf0 && v <= 0xf4:
			c.pending = 3
		default:
			c.invalid = true
			c.firstPending = false
		}
	}
	return v
}
func (c *rawCursor) space() {
	for c.i < len(c.b) {
		switch c.b[c.i] {
		case ' ', '\n', '\r', '\t':
			c.take()
		default:
			return
		}
	}
}
func (c *rawCursor) literal(s string) bool {
	for i := 0; i < len(s); i++ {
		if c.i == len(c.b) || c.take() != s[i] {
			return false
		}
	}
	return true
}
func (c *rawCursor) str() (string, bool) {
	if c.i == len(c.b) || c.take() != '"' {
		return "", false
	}
	// Copy only object-key material for decoded-key duplicate detection. All
	// inspection of the original byte slice is charged by take().
	var text []byte
	for c.i < len(c.b) {
		v := c.take()
		if v == '"' {
			decoded, err := strconv.Unquote("\"" + string(text) + "\"")
			return decoded, err == nil
		}
		if v < 0x20 {
			return "", false
		}
		text = append(text, v)
		if v == '\\' {
			if c.i == len(c.b) {
				return "", false
			}
			e := c.take()
			text = append(text, e)
			if e == 'u' {
				for j := 0; j < 4; j++ {
					if c.i == len(c.b) {
						return "", false
					}
					h := c.take()
					text = append(text, h)
					if !((h >= '0' && h <= '9') || (h >= 'a' && h <= 'f') || (h >= 'A' && h <= 'F')) {
						return "", false
					}
				}
			} else if !strings.ContainsRune(`"\/bfnrt`, rune(e)) {
				return "", false
			}
		}
	}
	return "", false
}
func (c *rawCursor) number() bool {
	if c.i < len(c.b) && c.b[c.i] == '-' {
		c.take()
	}
	if c.i == len(c.b) {
		return false
	}
	if c.b[c.i] == '0' {
		c.take()
	} else {
		if c.b[c.i] < '1' || c.b[c.i] > '9' {
			return false
		}
		for c.i < len(c.b) && c.b[c.i] >= '0' && c.b[c.i] <= '9' {
			c.take()
		}
	}
	if c.i < len(c.b) && c.b[c.i] == '.' {
		c.take()
		if !c.digits() {
			return false
		}
	}
	if c.i < len(c.b) && (c.b[c.i] == 'e' || c.b[c.i] == 'E') {
		c.take()
		if c.i < len(c.b) && (c.b[c.i] == '+' || c.b[c.i] == '-') {
			c.take()
		}
		if !c.digits() {
			return false
		}
	}
	return true
}
func (c *rawCursor) digits() bool {
	start := c.i
	for c.i < len(c.b) && c.b[c.i] >= '0' && c.b[c.i] <= '9' {
		c.take()
	}
	return c.i > start
}
func (c *rawCursor) value(depth int) bool {
	if depth > 512 || c.i == len(c.b) {
		return false
	}
	switch c.b[c.i] {
	case '"':
		_, ok := c.str()
		return ok
	case 'n':
		return c.literal("null")
	case 't':
		return c.literal("true")
	case 'f':
		return c.literal("false")
	case '[':
		c.take()
		c.space()
		if c.i < len(c.b) && c.b[c.i] == ']' {
			c.take()
			return true
		}
		for {
			if !c.value(depth + 1) {
				return false
			}
			c.space()
			if c.i == len(c.b) {
				return false
			}
			sep := c.take()
			if sep == ']' {
				return true
			}
			if sep != ',' {
				return false
			}
			c.space()
		}
	case '{':
		c.take()
		c.space()
		if c.i < len(c.b) && c.b[c.i] == '}' {
			c.take()
			return true
		}
		keys := make(map[string]struct{})
		for {
			key, ok := c.str()
			if !ok {
				return false
			}
			if _, exists := keys[key]; exists {
				return false
			}
			keys[key] = struct{}{}
			c.space()
			if c.i == len(c.b) || c.take() != ':' {
				return false
			}
			c.space()
			if !c.value(depth + 1) {
				return false
			}
			c.space()
			if c.i == len(c.b) {
				return false
			}
			sep := c.take()
			if sep == '}' {
				return true
			}
			if sep != ',' {
				return false
			}
			c.space()
		}
	default:
		return c.number()
	}
}
func scanRawTopLevel(b []byte) rawScannerObservation {
	out := rawScannerObservation{form: "MALFORMED"}
	if len(b) == 0 {
		return out
	}
	if len(b) > rawResultPayloadLimit {
		out.workUnits = rawResultPayloadLimit
		return out
	}
	c := rawCursor{b: b}
	c.space()
	if c.i < len(b) && b[c.i] == 'n' {
		if c.literal("null") {
			c.space()
			if c.i == len(b) {
				out.form = "NULL"
			}
		}
	} else if c.i < len(b) && b[c.i] == '[' {
		c.take()
		c.space()
		count := 0
		valid := true
		if c.i < len(b) && b[c.i] == ']' {
			c.take()
		} else {
			for {
				if !c.value(1) {
					valid = false
					break
				}
				count++
				c.space()
				if c.i == len(b) {
					valid = false
					break
				}
				sep := c.take()
				if sep == ']' {
					break
				}
				if sep != ',' {
					valid = false
					break
				}
				c.space()
			}
		}
		if valid {
			c.space()
			if c.i == len(b) {
				out.form = "ARRAY"
				out.knownE = count
			}
		}
	}
	// Complete the single charged traversal even for malformed JSON.
	for c.i < len(b) {
		c.take()
	}
	out.workUnits = c.i
	if c.invalid || c.pending != 0 {
		out.form = "MALFORMED"
		out.knownE = 0
	}
	return out
}
