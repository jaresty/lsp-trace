package hydratedevidence

import "testing"

func TestCoordinateOracle(t *testing.T) {
	// Offsets are independently enumerated from UTF-8 bytes, not producer conversion.
	content := []byte("\ufeffA😀Z\r\nβ\n")
	for _, tc := range []struct {
		name, enc string
		r         Range
		want      Interval
	}{
		{"utf8", "utf-8", Range{Position{0, 4}, Position{0, 8}}, Interval{4, 8}},
		{"utf16", "utf-16", Range{Position{0, 2}, Position{0, 4}}, Interval{4, 8}},
		{"utf32", "utf-32", Range{Position{0, 2}, Position{0, 3}}, Interval{4, 8}},
		{"bom-crlf", "utf-16", Range{Position{0, 0}, Position{1, 0}}, Interval{0, 11}},
		{"last-line", "utf-32", Range{Position{1, 0}, Position{2, 0}}, Interval{11, 14}},
		{"endpoint", "utf-16", Range{Position{2, 0}, Position{2, 0}}, Interval{14, 14}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ByteInterval(content, tc.enc, tc.r)
			if err != nil || got != tc.want {
				t.Fatalf("coordinate fidelity: got %v %v want %v", got, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, enc string
		r         Range
	}{
		{"surrogate", "utf-16", Range{Position{0, 3}, Position{0, 4}}},
		{"split-utf8", "utf-8", Range{Position{0, 5}, Position{0, 8}}},
		{"line-break-interior", "utf-8", Range{Position{0, 10}, Position{1, 0}}},
		{"unknown-encoding", "", Range{}},
		{"unsupported-source-encoding", "utf-7", Range{}},
		{"negative", "utf-8", Range{Position{-1, 0}, Position{0, 0}}},
		{"reversed", "utf-8", Range{Position{1, 0}, Position{0, 0}}},
		{"beyond-end", "utf-8", Range{Position{3, 0}, Position{3, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ByteInterval(content, tc.enc, tc.r); err == nil {
				t.Fatal("invalid coordinate accepted")
			}
		})
	}
	if got, err := ByteInterval([]byte{}, "utf-8", Range{}); err != nil || got != (Interval{}) {
		t.Fatalf("empty readable: %v %v", got, err)
	}
	if _, err := ByteInterval([]byte{0xff}, "utf-8", Range{}); err == nil {
		t.Fatal("invalid source UTF-8 accepted")
	}
}
