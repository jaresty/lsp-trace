//go:build darwin || linux

package sessionruntime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"lsp-trace/internal/session"
)

// Canonicalize only the owned fixture base, never a path under test.
func preparedContractRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func preparedContractWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func preparedContractRefused(t *testing.T, root, path string, limits ...int) {
	t.Helper()
	got, err := readPreparedNoFollow(root, path, limits...)
	if err == nil || got != nil {
		t.Fatalf("ASSERT_PREPARED_REFUSAL: bytes=%d err=%v", len(got), err)
	}
}

func TestPreparedNoFollowBoundsAndPaths(t *testing.T) {
	root := preparedContractRoot(t)
	file := filepath.Join(root, "source.go")
	for _, tc := range []struct {
		name    string
		size    int
		limits  []int
		success bool
	}{
		{"default", 8, nil, true},
		{"exact-private", 8, []int{8}, true},
		{"private-plus-one", 9, []int{8}, false},
		{"exact-default", MaxDocumentSupplyBytes, nil, true},
		{"default-plus-one", MaxDocumentSupplyBytes + 1, nil, false},
		{"exact-explicit-max", MaxDocumentSupplyBytes, []int{MaxDocumentSupplyBytes}, true},
		{"empty", 0, nil, false},
		{"zero-limit", 8, []int{0}, false},
		{"negative-limit", 8, []int{-1}, false},
		{"excess-limit", 8, []int{MaxDocumentSupplyBytes + 1}, false},
		{"multiple-limits", 8, []int{8, 8}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := bytes.Repeat([]byte{'x'}, tc.size)
			preparedContractWrite(t, file, want)
			if !tc.success {
				preparedContractRefused(t, root, file, tc.limits...)
				return
			}
			got, err := readPreparedNoFollow(root, file, tc.limits...)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("ASSERT_PREPARED_BYTES: bytes=%d want=%d err=%v", len(got), len(want), err)
			}
		})
	}
	preparedContractWrite(t, file, []byte("valid"))
	outside := filepath.Join(preparedContractRoot(t), "outside.go")
	preparedContractWrite(t, outside, []byte("outside"))
	nested := filepath.Join(root, "work")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, "work-other")
	if err := os.Mkdir(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	siblingFile := filepath.Join(sibling, "source.go")
	preparedContractWrite(t, siblingFile, []byte("sibling"))
	for _, tc := range []struct{ name, workspace, document string }{
		{"relative-root", "relative", file},
		{"relative-file", root, "source.go"},
		{"unclean-root", root + "/.", file},
		{"unclean-file", root, root + "/./source.go"},
		{"parent-component", root, root + "/work/../source.go"},
		{"root-workspace", "/", file},
		{"same-path", file, file},
		{"outside", root, outside},
		{"prefix-collision", nested, siblingFile},
		{"ancestor", nested, file},
	} {
		t.Run(tc.name, func(t *testing.T) { preparedContractRefused(t, tc.workspace, tc.document) })
	}
}

func TestPreparedNoFollowSymlinksAndNonregular(t *testing.T) {
	base := preparedContractRoot(t)
	root := filepath.Join(base, "parent", "workspace")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(sub, "source.go")
	preparedContractWrite(t, leaf, []byte("valid"))
	links := []struct{ name, target, link, workspace, document string }{
		{"leaf", leaf, filepath.Join(sub, "leaf-link"), root, filepath.Join(sub, "leaf-link")},
		{"descendant", sub, filepath.Join(root, "sub-link"), root, filepath.Join(root, "sub-link", "source.go")},
		{"workspace", root, filepath.Join(base, "workspace-link"), filepath.Join(base, "workspace-link"), filepath.Join(base, "workspace-link", "sub", "source.go")},
		{"above-workspace", filepath.Dir(root), filepath.Join(base, "parent-link"), filepath.Join(base, "parent-link", "workspace"), filepath.Join(base, "parent-link", "workspace", "sub", "source.go")},
	}
	for _, tc := range links {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Symlink(tc.target, tc.link); err != nil {
				t.Fatal(err)
			}
			// Targets exist and contain valid bytes, so missing-file refusal cannot pass.
			got, err := os.ReadFile(tc.document)
			if err != nil || string(got) != "valid" {
				t.Fatalf("fixture target: %q %v", got, err)
			}
			preparedContractRefused(t, tc.workspace, tc.document)
		})
	}
	t.Run("directory", func(t *testing.T) { preparedContractRefused(t, root, sub) })
	t.Run("fifo", func(t *testing.T) {
		fifo := filepath.Join(root, "fifo")
		if err := unix.Mkfifo(fifo, 0600); err != nil {
			t.Fatal(err)
		}
		type result struct {
			data []byte
			err  error
		}
		done := make(chan result, 1)
		go func() { b, e := readPreparedNoFollow(root, fifo); done <- result{b, e} }()
		select {
		case got := <-done:
			if got.err == nil || got.data != nil {
				t.Fatalf("ASSERT_PREPARED_FIFO_REFUSED: %+v", got)
			}
		case <-time.After(2 * time.Second):
			// Recover a blocking-open regression using only the owned FIFO, no helper process.
			fd, err := unix.Open(fifo, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err == nil {
				_ = unix.Close(fd)
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("FIFO reader did not join after recovery")
			}
			t.Fatal("ASSERT_PREPARED_FIFO_NONBLOCKING")
		}
	})
}

func TestPreparedNoFollowRepeatedDescriptorClosure(t *testing.T) {
	root := preparedContractRoot(t)
	good, empty, dir := filepath.Join(root, "good"), filepath.Join(root, "empty"), filepath.Join(root, "dir")
	preparedContractWrite(t, good, []byte("valid"))
	preparedContractWrite(t, empty, nil)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	fdDir := "/dev/fd"
	if runtime.GOOS == "linux" {
		fdDir = "/proc/self/fd"
	}
	count := func() int {
		directory, err := os.Open(fdDir)
		if err != nil {
			t.Fatal(err)
		}
		// Names avoid statting transient /dev/fd entries on Darwin.
		entries, readErr := directory.Readdirnames(-1)
		closeErr := directory.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("descriptor enumeration: read=%v close=%v", readErr, closeErr)
		}
		return len(entries)
	}
	cycle := func() {
		if b, e := readPreparedNoFollow(root, good); e != nil || string(b) != "valid" {
			t.Fatalf("good read: %q %v", b, e)
		}
		for _, p := range []string{empty, dir, filepath.Join(root, "missing"), filepath.Join(root, "missing-parent", "file")} {
			preparedContractRefused(t, root, p)
		}
	}
	cycle()
	before := count()
	for i := 0; i < 64; i++ {
		cycle()
	}
	if after := count(); after != before {
		t.Fatalf("ASSERT_PREPARED_DESCRIPTOR_CLOSURE: before=%d after=%d", before, after)
	}
}

func TestPreparedNoFollowManagerContract(t *testing.T) {
	// supplyFixture uses t.TempDir; give it a canonical owned base on Darwin too.
	t.Setenv("TMPDIR", preparedContractRoot(t))
	for _, tc := range []struct {
		name      string
		content   []byte
		failure   session.Failure
		uriSuffix string
		failWrite bool
	}{
		{"utf8-owned-copy", []byte("package p\n// café 😀\n"), "", "", false},
		{"invalid-utf8", []byte{0xff}, DocumentSupplyUnavailable, "", false},
		{"empty", nil, DocumentSupplyUnavailable, "", false},
		{"query", []byte("valid"), DocumentSupplyUnavailable, "?alias=1", false},
		{"fragment", []byte("valid"), DocumentSupplyUnavailable, "#alias", false},
		{"failed-write", []byte("valid"), session.SessionPoisoned, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, req, _, writer := supplyFixture(t, tc.content)
			t.Cleanup(func() { m.Shutdown(context.Background()) })
			req.PreparedNoFollow = true
			req.URI += tc.uriSuffix
			writer.fail = tc.failWrite
			got := m.PrepareDocument(context.Background(), req)
			if tc.failure != "" {
				if got.Failure != tc.failure || got.Supply != nil || writer.Len() != 0 {
					t.Fatalf("ASSERT_PREPARED_NO_FALSE_SUPPLY: %+v writes=%d", got, writer.Len())
				}
				return
			}
			if got.Failure != "" || got.Supply == nil || !bytes.Equal(got.Supply.Content, tc.content) {
				t.Fatalf("ASSERT_PREPARED_MANAGER_SUPPLY: %+v", got)
			}
			params := append([]byte(nil), got.Supply.Params...)
			got.Supply.Content[0] = '!'
			got.Supply.Params[0] = '!'
			before := writer.Len()
			again := m.PrepareDocument(context.Background(), req)
			if again.Failure != "" || again.Supply == nil || !bytes.Equal(again.Supply.Content, tc.content) || !bytes.Equal(again.Supply.Params, params) || again.Version != got.Version || writer.Len() != before {
				t.Fatalf("ASSERT_PREPARED_OWNED_COPY: %+v", again)
			}
		})
	}
}
