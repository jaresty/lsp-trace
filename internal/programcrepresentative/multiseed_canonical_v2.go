package programcrepresentative

import (
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

const MaxArtifactCanonicalBytesV2 uint64 = 67108864

// canonicalStringSizeV2 counts the exact Go JSON string representation.
func canonicalStringSizeV2(s string) (uint64, error) {
	if !utf8.ValidString(s) {
		return 0, fmt.Errorf("invalid multiseed UTF-8")
	}
	n := uint64(2)
	for _, r := range s {
		width := uint64(utf8.RuneLen(r))
		switch {
		case r == '"' || r == '\\' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t':
			width = 2
		case r < 0x20 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029':
			width = 6
		}
		if math.MaxUint64-n < width {
			return 0, fmt.Errorf("multiseed size overflow")
		}
		n += width
	}
	return n, nil
}

// canonicalSizerV2 is only a byte counter for the explicitly declared DTO
// walks. It never serializes, reflects, or clones the value being admitted.
type canonicalSizerV2 struct {
	n   uint64
	err error
}

func (z *canonicalSizerV2) add(n uint64) {
	if z.err != nil {
		return
	}
	if math.MaxUint64-z.n < n {
		z.err = fmt.Errorf("multiseed size overflow")
		return
	}
	z.n += n
	if z.n > MaxArtifactCanonicalBytesV2 {
		z.err = fmt.Errorf("multiseed canonical artifact exceeds 64 MiB")
	}
}
func (z *canonicalSizerV2) str(s string) {
	if len(s) > MaxStringBytesV2 {
		z.err = fmt.Errorf("multiseed string byte cap")
		return
	}
	n, err := canonicalStringSizeV2(s)
	if err != nil {
		z.err = err
		return
	}
	z.add(n)
}
func (z *canonicalSizerV2) num(n uint64) { z.add(uint64(len(strconv.FormatUint(n, 10)))) }
func (z *canonicalSizerV2) integer(n int) {
	if n < 0 {
		z.err = fmt.Errorf("negative multiseed integer")
		return
	}
	z.num(uint64(n))
}
func (z *canonicalSizerV2) boolean(b bool) {
	if b {
		z.add(4)
	} else {
		z.add(5)
	}
}
func (z *canonicalSizerV2) key(k string, first bool) {
	if !first {
		z.add(1)
	}
	z.str(k)
	z.add(1)
}
func (z *canonicalSizerV2) strings(v []string) {
	if v == nil {
		z.err = fmt.Errorf("null multiseed array")
		return
	}
	z.add(2)
	for i, s := range v {
		if i > 0 {
			z.add(1)
		}
		z.str(s)
	}
}
func (z *canonicalSizerV2) binding(v ArtifactBindingDTO) {
	z.add(2)
	z.key("schema_version", true)
	z.str(v.SchemaVersion)
	z.key("identity", false)
	z.str(v.Identity)
	z.key("sha256", false)
	z.str(v.SHA256)
	z.key("byte_length", false)
	z.num(v.ByteLength)
}
