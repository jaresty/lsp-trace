package publication

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVisitNamesBoundedPinnedLimitAndRepeat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "store")
	if e := os.Mkdir(p, 0700); e != nil {
		t.Fatal(e)
	}
	r, e := OpenRoot(p)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	for i := 0; i < 3; i++ {
		if e := os.WriteFile(filepath.Join(p, fmt.Sprint(i)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	// Rename and replace the ambient path: the visitor must still read the pinned root.
	if e := os.Rename(p, p+"-original"); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(p, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(p, "replacement"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	for _, limit := range []uint64{2, 3, 3, 1 << 25} {
		seen := 0
		e := r.VisitNamesBounded(limit, func(n string) error {
			seen++
			if n == "replacement" {
				t.Fatal("ambient path followed")
			}
			return nil
		})
		if limit < 3 {
			if e == nil || seen != 2 {
				t.Fatalf("cap: %d %v", seen, e)
			}
		} else if e != nil || seen != 3 {
			t.Fatalf("visit: %d %v", seen, e)
		}
	}
	sentinel := errors.New("stop")
	if e := r.VisitNamesBounded(3, func(string) error { return sentinel }); !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
	if e := r.Close(); e != nil {
		t.Fatal(e)
	}
	if e := r.VisitNamesBounded(3, func(string) error { return nil }); e == nil {
		t.Fatal("closed root accepted")
	}
}
