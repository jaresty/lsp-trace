package utf16position

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrCharacterOutOfLine = errors.New("UTF-16 character out of line")
	ErrInvalidUTF8        = errors.New("invalid UTF-8 source")
	ErrInvertedRange      = errors.New("inverted range")
	ErrLineOutOfDocument  = errors.New("line out of document")
	ErrMidSurrogate       = errors.New("UTF-16 character splits surrogate pair")
)

type Position struct {
	Line      uint32
	Character uint32
}

type Range struct {
	Start Position
	End   Position
}

func Offset(raw []byte, position Position) (int, error) {
	lineStart, lineEnd, err := lineBounds(raw, position.Line)
	if err != nil {
		return 0, err
	}

	units := uint32(0)
	for offset := lineStart; offset < lineEnd; {
		if units == position.Character {
			return offset, nil
		}
		r, size := utf8.DecodeRune(raw[offset:lineEnd])
		if r == utf8.RuneError && size == 1 {
			return 0, ErrInvalidUTF8
		}
		width := uint32(1)
		if r > 0xffff {
			width = 2
		}
		if units+width > position.Character {
			return 0, fmt.Errorf("%w: %d", ErrMidSurrogate, position.Character)
		}
		units += width
		offset += size
	}
	if units == position.Character {
		return lineEnd, nil
	}
	return 0, fmt.Errorf("%w: %d", ErrCharacterOutOfLine, position.Character)
}

func Offsets(raw []byte, r Range) (int, int, error) {
	start, err := Offset(raw, r.Start)
	if err != nil {
		return 0, 0, fmt.Errorf("start: %w", err)
	}
	end, err := Offset(raw, r.End)
	if err != nil {
		return 0, 0, fmt.Errorf("end: %w", err)
	}
	if end < start {
		return 0, 0, ErrInvertedRange
	}
	return start, end, nil
}

func lineBounds(raw []byte, target uint32) (int, int, error) {
	lineStart := 0
	line := uint32(0)
	for line < target {
		newline := -1
		for i := lineStart; i < len(raw); i++ {
			if raw[i] == '\n' {
				newline = i
				break
			}
		}
		if newline < 0 {
			return 0, 0, fmt.Errorf("%w: %d", ErrLineOutOfDocument, target)
		}
		lineStart = newline + 1
		line++
	}

	lineEnd := lineStart
	for lineEnd < len(raw) && raw[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd > lineStart && raw[lineEnd-1] == '\r' {
		lineEnd--
	}
	return lineStart, lineEnd, nil
}
