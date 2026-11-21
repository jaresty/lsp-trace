//go:build darwin || linux

package sourcecontinuation

import (
	"context"
	dw "lsp-trace/internal/describeworker"
	"unicode/utf8"
)

// inventoryRecordUpper sizes the existing response-record grammar, not a new
// response limit. The strictly canonical adapter escapes quotes/backslashes
// and HTML/line separators at least as much as the inner semantic encoder.
// Missing optional nested fields can add closed default keys/values during
// canonicalization. The extra2048 covers those defaults plus record keys,
// identities, basis and attributed suggestions (four closed handles, unique
// within each of five categories).
// HOST is counted separately, including all original opaque strings.
func inventoryRecordUpper(ctx context.Context, h dw.ResponseHostV2, adapter int, b *CallerBudgetV3) (uint64, error) {
	strings := [...]string{h.RequestRecordID, h.MessageID, h.AttemptID, h.Consumer.Resolution, h.Consumer.AlternativeID, h.Pins.WorkerSHA256, h.Pins.ModelSHA256, h.Pins.GrammarSHA256, h.Pins.PromptSHA256, h.Provenance.PacketID, h.Provenance.RequestLineageIdentity, h.Custody.GraphDigest, h.Custody.CaptureID}
	var length uint64
	for _, s := range strings {
		var ok bool
		length, ok = sourceResponseAdd(length, uint64(len(s)))
		if !ok {
			return 0, ErrDurableDescribeV1
		}
	}
	scan, ok := sourceResponseMul(length, 6)
	if !ok || adapter < 1 || adapter > 512<<10 || scan > 64<<20-4096 || b.Reserve(ctx, scan+4096) != nil {
		return 0, ErrDurableDescribeV1
	}
	// HOST's closed keys, braces, quotes and maximum signed decimal ordinal fit
	// 512 bytes. String content is sized without allocating or marshaling a clone.
	upper := uint64(adapter) + 2048 + 512
	for _, s := range strings {
		for offset := 0; offset < len(s); {
			r, width := utf8.DecodeRuneInString(s[offset:])
			offset += width
			if r == utf8.RuneError && width == 1 {
				upper += 6 // encoding/json emits the ASCII escape \\ufffd
				continue
			}
			switch {
			case r < 0x20 || r == '<' || r == '>' || r == '&' || r == 0x2028 || r == 0x2029:
				upper += 6
			case r == '"' || r == '\\':
				upper += 2
			default:
				upper += uint64(width)
			}
		}
	}
	if upper > 8<<20 {
		return 0, ErrDurableDescribeV1
	}
	return upper, nil
}
