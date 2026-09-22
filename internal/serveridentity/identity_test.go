package serveridentity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMeasureVerifiedCanonicalRegularExecutable(t *testing.T) {
	d := t.TempDir()
	regular := filepath.Join(d, "server")
	if err := os.WriteFile(regular, []byte("one"), 0o700); err != nil {
		t.Fatal(err)
	}
	id1 := New("lsp-trace-mcp devel", "abc", true, func() (string, error) { return regular, nil })
	if id1.Custody != "SELF_MEASURED" || id1.ExecutableSHA256 == "" || !id1.Dirty {
		t.Fatalf("ASSERT_SELF_MEASURED_REGULAR: %#v", id1)
	}
	link := filepath.Join(d, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if got := New("v", "abc", false, func() (string, error) { return link, nil }); got.ExecutableSHA256 != "" {
		t.Fatalf("ASSERT_SYMLINK_UNAVAILABLE: %#v", got)
	}
	dir := filepath.Join(d, "dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := New("v", "abc", false, func() (string, error) { return dir, nil }); got.ExecutableSHA256 != "" {
		t.Fatalf("ASSERT_NONREGULAR_UNAVAILABLE: %#v", got)
	}
	if got := New("v", "abc", false, func() (string, error) { return "", os.ErrNotExist }); got.ExecutableSHA256 != "" {
		t.Fatalf("ASSERT_HASH_UNAVAILABLE: %#v", got)
	}
}

func TestCoordinatedSubstitutionFailsClosedAndInstancesDiffer(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "server")
	if err := os.WriteFile(p, []byte("one"), 0o700); err != nil {
		t.Fatal(err)
	}
	called := false
	got := New("v", "abc", false, func() (string, error) {
		if !called {
			called = true
			return p, nil
		}
		_ = os.WriteFile(p, []byte("two"), 0o700)
		return p, nil
	})
	if got.ExecutableSHA256 != "" {
		t.Fatalf("ASSERT_SUBSTITUTION_UNAVAILABLE: %#v", got)
	}
	a := New("v", "abc", false, func() (string, error) { return "", os.ErrNotExist })
	b := New("v", "abc", false, func() (string, error) { return "", os.ErrNotExist })
	if a.InstanceID == "" || a.InstanceID == b.InstanceID {
		t.Fatalf("ASSERT_DISTINCT_PROCESS_INSTANCES: %q %q", a.InstanceID, b.InstanceID)
	}
}
