package hydratedevidence

import (
	"errors"
	"unicode/utf8"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Interval struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ByteInterval interprets positions in their declared units over UTF-8 source.
// CR, LF and CRLF terminate lines; positions never address their interiors.
// BOM is retained and counts as an ordinary code point, not discarded metadata.
func ByteInterval(content []byte, encoding string, r Range) (Interval, error) {
	if encoding != "utf-8" && encoding != "utf-16" && encoding != "utf-32" {
		return Interval{}, errors.New("unsupported position encoding")
	}
	if !utf8.Valid(content) {
		return Interval{}, errors.New("source is not UTF-8")
	}
	offset := func(p Position) (int, error) {
		if p.Line < 0 || p.Character < 0 {
			return 0, errors.New("negative coordinate")
		}
		line, start := 0, 0
		for line < p.Line && start < len(content) {
			if content[start] == '\r' {
				start++
				if start < len(content) && content[start] == '\n' {
					start++
				}
				line++
			} else if content[start] == '\n' {
				start++
				line++
			} else {
				_, n := utf8.DecodeRune(content[start:])
				start += n
			}
		}
		if line != p.Line {
			return 0, errors.New("line out of bounds")
		}
		units := 0
		for i := start; ; {
			if units == p.Character {
				return i, nil
			}
			if i == len(content) || content[i] == '\r' || content[i] == '\n' {
				return 0, errors.New("character out of bounds")
			}
			ch, n := utf8.DecodeRune(content[i:])
			width := 1
			if encoding == "utf-8" {
				width = n
			}
			if encoding == "utf-16" && ch > 0xffff {
				width = 2
			}
			units += width
			i += n
			if units > p.Character {
				return 0, errors.New("coordinate splits code point or surrogate pair")
			}
		}
	}
	a, err := offset(r.Start)
	if err != nil {
		return Interval{}, err
	}
	b, err := offset(r.End)
	if err != nil {
		return Interval{}, err
	}
	if b < a {
		return Interval{}, errors.New("reversed interval")
	}
	return Interval{a, b}, nil
}
