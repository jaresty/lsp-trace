package utf16position

import (
	"errors"
	"testing"
)

func TestOffsetStrictUTF16Contract(t *testing.T) {
	t.Run("astral-two-code-units", func(t *testing.T) {
		raw := []byte("a😀b")
		got, err := Offset(raw, Position{Line: 0, Character: 3})
		if err != nil || got != len("a😀") {
			t.Fatalf("ASSERT_UTF16_ASTRAL_TWO_CODE_UNITS: offset=%d err=%v", got, err)
		}
	})
	t.Run("crlf-terminator", func(t *testing.T) {
		if got, err := Offset([]byte("a\r\nb"), Position{Line: 0, Character: 2}); !errors.Is(err, ErrCharacterOutOfLine) {
			t.Fatalf("ASSERT_UTF16_CRLF_TERMINATOR_NOT_ADDRESSABLE: offset=%d err=%v", got, err)
		}
		if got, err := Offset([]byte("a\r\nb"), Position{Line: 1, Character: 0}); err != nil || got != 3 {
			t.Fatalf("ASSERT_UTF16_CRLF_NEXT_LINE_BOUNDARY: offset=%d err=%v", got, err)
		}
	})
	t.Run("mid-surrogate", func(t *testing.T) {
		if got, err := Offset([]byte("😀"), Position{Line: 0, Character: 1}); !errors.Is(err, ErrMidSurrogate) {
			t.Fatalf("ASSERT_UTF16_MID_SURROGATE_REJECTED: offset=%d err=%v", got, err)
		}
	})
	t.Run("line-out-of-document", func(t *testing.T) {
		if got, err := Offset([]byte("a"), Position{Line: 1, Character: 0}); !errors.Is(err, ErrLineOutOfDocument) {
			t.Fatalf("ASSERT_UTF16_LINE_OUT_OF_DOCUMENT_REJECTED: offset=%d err=%v", got, err)
		}
	})
	t.Run("character-out-of-line", func(t *testing.T) {
		if got, err := Offset([]byte("a"), Position{Line: 0, Character: 2}); !errors.Is(err, ErrCharacterOutOfLine) {
			t.Fatalf("ASSERT_UTF16_CHARACTER_OUT_OF_LINE_REJECTED: offset=%d err=%v", got, err)
		}
	})
}

func TestOffsetsPreservesHalfOpenRange(t *testing.T) {
	raw := []byte("a😀b")
	start, end, err := Offsets(raw, Range{
		Start: Position{Line: 0, Character: 1},
		End:   Position{Line: 0, Character: 3},
	})
	if err != nil || string(raw[start:end]) != "😀" {
		t.Fatalf("ASSERT_UTF16_HALF_OPEN_RANGE_OFFSETS: start=%d end=%d body=%q err=%v", start, end, raw[start:end], err)
	}

	_, _, err = Offsets(raw, Range{
		Start: Position{Line: 0, Character: 3},
		End:   Position{Line: 0, Character: 1},
	})
	if !errors.Is(err, ErrInvertedRange) {
		t.Fatalf("ASSERT_UTF16_INVERTED_RANGE_REJECTED: err=%v", err)
	}
}
