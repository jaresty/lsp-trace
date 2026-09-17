package sourceposition

import "testing"

func TestExactPositionEncodingsAndLineTerminators(t *testing.T) {
	const assertion = "ASSERT_C02_UTF8_UTF16_UTF32_EXACT_RANGES"
	for _, tc := range []struct {
		encoding string
		end      uint32
	}{
		{encoding: "utf-8", end: 5},
		{encoding: "utf-16", end: 3},
		{encoding: "utf-32", end: 2},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			raw := []byte("a😀b")
			start, end, err := Offsets(raw, tc.encoding, Range{Start: Position{Character: 1}, End: Position{Character: tc.end}})
			if err != nil || string(raw[start:end]) != "😀" {
				t.Fatalf("%s: encoding=%s start=%d end=%d body=%q err=%v", assertion, tc.encoding, start, end, raw[start:end], err)
			}
			position, err := PositionAtOffset(raw, tc.encoding, end)
			if err != nil || position != (Position{Character: tc.end}) {
				t.Fatalf("%s_ROUND_TRIP: encoding=%s position=%+v err=%v", assertion, tc.encoding, position, err)
			}
		})
	}

	for name, raw := range map[string][]byte{
		"lf":   []byte("a\nb"),
		"cr":   []byte("a\rb"),
		"crlf": []byte("a\r\nb"),
	} {
		t.Run(name, func(t *testing.T) {
			offset, err := Offset(raw, "utf-16", Position{Line: 1})
			if err != nil || raw[offset] != 'b' {
				t.Fatalf("ASSERT_C02_BOM_NEWLINES_NONBMP_EMPTY_CROSSLINE: offset=%d err=%v", offset, err)
			}
		})
	}
}

func TestBOMEmptyAndCrossLineRanges(t *testing.T) {
	const assertion = "ASSERT_C02_BOM_NEWLINES_NONBMP_EMPTY_CROSSLINE"
	raw := []byte("\xef\xbb\xbf😀\r\nxy")
	for _, encoding := range []string{"utf-8", "utf-16", "utf-32"} {
		bomUnits := uint32(1)
		emojiUnits := uint32(1)
		if encoding == "utf-8" {
			bomUnits, emojiUnits = 3, 4
		} else if encoding == "utf-16" {
			emojiUnits = 2
		}
		empty := Range{Start: Position{Character: bomUnits}, End: Position{Character: bomUnits}}
		start, end, err := Offsets(raw, encoding, empty)
		if err != nil || start != end {
			t.Fatalf("%s_EMPTY: encoding=%s start=%d end=%d err=%v", assertion, encoding, start, end, err)
		}
		cross := Range{Start: Position{Character: bomUnits}, End: Position{Line: 1, Character: 2}}
		start, end, err = Offsets(raw, encoding, cross)
		if err != nil || string(raw[start:end]) != "😀\r\nxy" {
			t.Fatalf("%s_CROSS_LINE: encoding=%s emoji_units=%d body=%q err=%v", assertion, encoding, emojiUnits, raw[start:end], err)
		}
	}
}

func TestInvalidEncodingAndBoundariesFailClosed(t *testing.T) {
	const assertion = "ASSERT_C06_INVALID_ID_RANGE_ENCODING_PRIVACY_STATUS_LIMIT_FAILS"
	raw := []byte("😀")
	for _, tc := range []struct {
		name     string
		encoding string
		position Position
	}{
		{name: "unknown", encoding: "UTF-16", position: Position{}},
		{name: "utf8-mid-rune", encoding: "utf-8", position: Position{Character: 1}},
		{name: "utf16-mid-surrogate", encoding: "utf-16", position: Position{Character: 1}},
		{name: "utf32-out-of-line", encoding: "utf-32", position: Position{Character: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if offset, err := Offset(raw, tc.encoding, tc.position); err == nil {
				t.Fatalf("%s: offset=%d", assertion, offset)
			}
		})
	}
}
