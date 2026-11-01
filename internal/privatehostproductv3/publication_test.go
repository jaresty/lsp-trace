//go:build darwin || linux

package privatehostproductv3

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostProductInProgressPublicationIsNotReadable(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(dir, 0700)
	s, e := mailboxStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	visible, release := make(chan struct{}), make(chan struct{})
	s.visibleObserver = func() { close(visible); <-release }
	done := make(chan error, 1)
	go func() { done <- s.write("request-00000001.json", []byte(`{"complete":true}`)) }()
	<-visible
	_, readErr := s.read("request-00000001.json", 1024)
	close(release)
	writeErr := <-done
	if !os.IsNotExist(readErr) {
		t.Fatalf("ASSERT_IN_PROGRESS_NOT_ADMITTED: readErr=%v", readErr)
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	raw, e := s.read("request-00000001.json", 1024)
	if e != nil || string(raw) != `{"complete":true}` {
		t.Fatal("ASSERT_COMPLETE_PUBLICATION_READABLE", e)
	}
	t.Log("ASSERT_IN_PROGRESS_NOT_ADMITTED: PASS")
}
