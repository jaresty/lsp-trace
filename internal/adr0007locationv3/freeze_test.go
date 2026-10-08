//go:build linux || darwin

package adr0007locationv3

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func fixture(t *testing.T) (string, Freeze) {
	t.Helper()
	r := t.TempDir()
	must(t, os.WriteFile(filepath.Join(r, "a"), []byte("a"), 0644))
	must(t, os.Mkdir(filepath.Join(r, "d"), 0755))
	must(t, os.WriteFile(filepath.Join(r, "d", "b"), []byte("bb"), 0644))
	e, er := Census(r)
	must(t, er)
	f := Freeze{Schema: FreezeSchema, State: "DESIGN_FROZEN_NON_DISPATCHING", FileCount: len(e), Files: e}
	if er = Verify(r, f); er != nil {
		t.Fatal(er)
	}
	return r, f
}
func TestFreezeCensusMutationsFailClosed(t *testing.T) {
	muts := []struct {
		name string
		mut  func(*testing.T, string, *Freeze)
	}{
		{"deleted-entry", func(t *testing.T, r string, f *Freeze) { f.Files = f.Files[1:]; f.FileCount-- }},
		{"extra-unlisted-file", func(t *testing.T, r string, f *Freeze) {
			must(t, os.WriteFile(filepath.Join(r, "extra"), []byte("x"), 0644))
		}},
		{"duplicate", func(t *testing.T, r string, f *Freeze) { f.Files = append(f.Files, f.Files[0]); f.FileCount++ }},
		{"reorder", func(t *testing.T, r string, f *Freeze) { f.Files[0], f.Files[1] = f.Files[1], f.Files[0] }},
		{"substituted-path", func(t *testing.T, r string, f *Freeze) { f.Files[0].Path = "d/b" }},
		{"substituted-digest", func(t *testing.T, r string, f *Freeze) {
			f.Files[0].SHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"substituted-length", func(t *testing.T, r string, f *Freeze) { f.Files[0].Bytes++ }},
		{"traversal", func(t *testing.T, r string, f *Freeze) { f.Files[0].Path = "../escape" }},
		{"malformed-digest", func(t *testing.T, r string, f *Freeze) { f.Files[0].SHA256 = "sha256:no" }},
	}
	for _, m := range muts {
		t.Run(m.name, func(t *testing.T) {
			r, f := fixture(t)
			m.mut(t, r, &f)
			if Verify(r, f) == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
}
func TestFreezeCensusRejectsSymlinkAndSpecial(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		r := t.TempDir()
		must(t, os.WriteFile(filepath.Join(r, "target"), []byte("x"), 0644))
		if e := os.Symlink("target", filepath.Join(r, "link")); e != nil {
			t.Skip(e)
		}
		if _, e := Census(r); e == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("special", func(t *testing.T) {
		r := t.TempDir()
		if e := syscall.Mkfifo(filepath.Join(r, "fifo"), 0600); e != nil {
			t.Skip(e)
		}
		if _, e := Census(r); e == nil {
			t.Fatal("special accepted")
		}
	})
}
func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
