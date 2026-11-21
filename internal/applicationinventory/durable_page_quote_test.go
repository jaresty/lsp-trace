//go:build darwin || linux

package applicationinventory

import "testing"

func TestDurablePageQuote(t *testing.T) {
	for _, n := range []int{32767, 32768, 32769, 8 << 20} {
		for _, offset := range []int{0, n - 1} {
			quote, count, err := durablePageQuote(n, offset, 32768)
			want := min(32768, n-offset)
			if err != nil || count != want || quote != 6*uint64(n)+2*uint64(want)+16384 {
				t.Fatalf("quote n=%d offset=%d: %d %d %v", n, offset, quote, count, err)
			}
		}
	}
	for _, tc := range [][3]int{{0, 0, 1}, {-1, 0, 1}, {1, -1, 1}, {1, 1, 1}, {1, 0, 0}, {1, 0, 32769}, {(8 << 20) + 1, 0, 1}, {int(^uint(0) >> 1), 0, 32768}} {
		if _, _, e := durablePageQuote(tc[0], tc[1], tc[2]); e == nil {
			t.Fatalf("invalid or overflowing quote accepted: %v", tc)
		}
	}
}
