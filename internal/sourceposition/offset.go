package sourceposition

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrCharacterOutOfLine  = errors.New("character out of line")
	ErrInvalidUTF8         = errors.New("invalid UTF-8 source")
	ErrInvertedRange       = errors.New("inverted range")
	ErrLineOutOfDocument   = errors.New("line out of document")
	ErrMidCodeUnit         = errors.New("character splits encoded code unit")
	ErrUnsupportedEncoding = errors.New("unsupported position encoding")
)

type Position struct {
	Line      uint32
	Character uint32
}

type Range struct {
	Start Position
	End   Position
}

func Supported(encoding string) bool {
	return encoding == "utf-8" || encoding == "utf-16" || encoding == "utf-32"
}

func Offset(raw []byte, encoding string, position Position) (int, error) {
	if !Supported(encoding) {
		return 0, fmt.Errorf("%w %q", ErrUnsupportedEncoding, encoding)
	}
	lineStart, lineEnd, err := lineBounds(raw, position.Line)
	if err != nil {
		return 0, err
	}
	if !utf8.Valid(raw[lineStart:lineEnd]) {
		return 0, ErrInvalidUTF8
	}
	units := uint32(0)
	for offset := lineStart; offset < lineEnd; {
		if units == position.Character {
			return offset, nil
		}
		r, size := utf8.DecodeRune(raw[offset:lineEnd])
		width := uint32(1)
		switch encoding {
		case "utf-8":
			width = uint32(size)
		case "utf-16":
			if r > 0xffff {
				width = 2
			}
		case "utf-32":
			width = 1
		}
		if units+width > position.Character {
			return 0, fmt.Errorf("%w: %d", ErrMidCodeUnit, position.Character)
		}
		units += width
		offset += size
	}
	if units == position.Character {
		return lineEnd, nil
	}
	return 0, fmt.Errorf("%w: %d", ErrCharacterOutOfLine, position.Character)
}

func Offsets(raw []byte, encoding string, r Range) (int, int, error) {
	start, err := Offset(raw, encoding, r.Start)
	if err != nil {
		return 0, 0, fmt.Errorf("start: %w", err)
	}
	end, err := Offset(raw, encoding, r.End)
	if err != nil {
		return 0, 0, fmt.Errorf("end: %w", err)
	}
	if end < start {
		return 0, 0, ErrInvertedRange
	}
	return start, end, nil
}

func PositionAtOffset(raw []byte, encoding string, target int) (Position, error) {
	if !Supported(encoding) {
		return Position{}, fmt.Errorf("%w %q", ErrUnsupportedEncoding, encoding)
	}
	if target < 0 || target > len(raw) || !utf8.Valid(raw) {
		return Position{}, ErrInvalidUTF8
	}
	line := uint32(0)
	lineStart := 0
	for offset := 0; offset < target; {
		if raw[offset] == '\r' {
			if offset+1 < len(raw) && raw[offset+1] == '\n' {
				offset += 2
			} else {
				offset++
			}
			if offset > target {
				return Position{}, ErrMidCodeUnit
			}
			line++
			lineStart = offset
			continue
		}
		if raw[offset] == '\n' {
			offset++
			line++
			lineStart = offset
			continue
		}
		_, size := utf8.DecodeRune(raw[offset:])
		if offset+size > target {
			return Position{}, ErrMidCodeUnit
		}
		offset += size
	}
	units := uint32(0)
	for offset := lineStart; offset < target; {
		r, size := utf8.DecodeRune(raw[offset:target])
		switch encoding {
		case "utf-8":
			units += uint32(size)
		case "utf-16":
			if r > 0xffff {
				units += 2
			} else {
				units++
			}
		case "utf-32":
			units++
		}
		offset += size
	}
	return Position{Line: line, Character: units}, nil
}

func lineBounds(raw []byte, target uint32) (int, int, error) {
	line := uint32(0)
	start := 0
	for i := 0; i <= len(raw); {
		if line == target {
			end := i
			for end < len(raw) && raw[end] != '\r' && raw[end] != '\n' {
				end++
			}
			return start, end, nil
		}
		if i == len(raw) {
			break
		}
		if raw[i] == '\r' {
			i++
			if i < len(raw) && raw[i] == '\n' {
				i++
			}
			line++
			start = i
			continue
		}
		if raw[i] == '\n' {
			i++
			line++
			start = i
			continue
		}
		_, size := utf8.DecodeRune(raw[i:])
		if size == 0 {
			break
		}
		i += size
	}
	return 0, 0, fmt.Errorf("%w: %d", ErrLineOutOfDocument, target)
}
