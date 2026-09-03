package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPublishExactBytesAndReceipt(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	data := []byte("exact\x00bytes")
	r := NewPublisher().Publish(Request{Root: root, Selector: "out.bin", Bytes: data, ArtifactSchemaID: "schema-id"})
	if r.Failure != nil {
		t.Fatalf("publish: %+v", r.Failure)
	}
	got, err := os.ReadFile(filepath.Join(root.Path(), "out.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("bytes = %q", got)
	}
	sum := sha256.Sum256(data)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if r.Receipt == nil || r.Receipt.ByteLength != uint64(len(data)) || r.Receipt.Digest != wantDigest || r.Receipt.ArtifactSchemaID != "schema-id" || r.Receipt.PublicationMechanism != PublicationMechanism {
		t.Fatalf("receipt = %#v", r.Receipt)
	}
	info, err := os.Stat(filepath.Join(root.Path(), "out.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("published mode = %o", info.Mode().Perm())
	}
}

func TestUnsafeSelectorsFailClosed(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	for _, selector := range []string{"", ".", "../escape", "/absolute", "a/../../escape", "a//b", "a/./b", "nul\x00byte"} {
		r := NewPublisher().Publish(Request{Root: root, Selector: selector, Bytes: []byte("x")})
		if r.Failure == nil || r.Failure.Code != CodeOutputSelectorUnsafe {
			t.Errorf("selector %q: %#v", selector, r)
		}
	}
}

func TestNestedParentsAreOwnerOnly(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	result := NewPublisher().Publish(Request{Root: root, Selector: "nested/deep/out", Bytes: []byte("x")})
	if result.Failure != nil {
		t.Fatalf("publish: %#v", result.Failure)
	}
	for _, name := range []string{"nested", "nested/deep"} {
		info, err := os.Stat(filepath.Join(root.Path(), name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 {
			t.Fatalf("%s mode = %v", name, info.Mode())
		}
	}
}

func TestSymlinkComponentsFailClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "alias")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, selector := range []string{"alias/out", "alias"} {
		result := NewPublisher().Publish(Request{Root: root, Selector: selector, Bytes: []byte("x")})
		if result.Failure == nil || result.Failure.Code != CodeOutputSelectorUnsafe {
			t.Errorf("selector %q: %#v", selector, result)
		}
	}
}

func TestPinnedRootSubstitutionFailsClosed(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "root")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	r := NewPublisher().Publish(Request{Root: root, Selector: "out", Bytes: []byte("x")})
	if r.Failure == nil || r.Failure.Code != CodeOutputSelectorUnsafe {
		t.Fatalf("result = %#v", r)
	}
}

func TestConcurrentCollisionHasOneSuccess(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	p := NewPublisher()
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for _, data := range [][]byte{[]byte("a"), []byte("b")} {
		wg.Add(1)
		go func(data []byte) {
			defer wg.Done()
			results <- p.Publish(Request{Root: root, Selector: "same", Bytes: data})
		}(data)
	}
	wg.Wait()
	close(results)
	success := 0
	for r := range results {
		if r.Receipt != nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d", success)
	}
}

func TestConcurrentEquivalentRootsHaveOneSuccess(t *testing.T) {
	dir := t.TempDir()
	first, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	publisher := NewPublisher()
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for _, root := range []*Root{first, second} {
		wg.Add(1)
		go func(root *Root) {
			defer wg.Done()
			results <- publisher.Publish(Request{Root: root, Selector: "same", Bytes: []byte("x")})
		}(root)
	}
	wg.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result.Receipt != nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d", successes)
	}
}

func TestOperationIsTerminalOnce(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	op := NewOperation(NewPublisher(), Request{Root: root, Selector: "once", Bytes: []byte("x")})
	first, second := op.Publish(), op.Publish()
	if first.Receipt == nil {
		t.Fatalf("first = %#v", first)
	}
	if !errors.Is(second.Err(), ErrOperationTerminal) {
		t.Fatalf("second = %#v", second)
	}
}

func TestOperationConcurrentPublishIsTerminalOnce(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	op := NewOperation(NewPublisher(), Request{Root: root, Selector: "once-concurrent", Bytes: []byte("x")})
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- op.Publish()
		}()
	}
	wg.Wait()
	close(results)
	receipts, terminals := 0, 0
	for result := range results {
		if result.Receipt != nil {
			receipts++
		}
		if errors.Is(result.Err(), ErrOperationTerminal) {
			terminals++
		}
	}
	if receipts != 1 || terminals != 1 {
		t.Fatalf("receipts=%d terminal_failures=%d", receipts, terminals)
	}
}

func TestFailureHasNoReceiptOrPathEcho(t *testing.T) {
	root, _ := OpenRoot(t.TempDir())
	defer root.Close()
	secret := "secret-selector"
	r := NewPublisher().Publish(Request{Root: root, Selector: "../" + secret, Bytes: []byte("x")})
	if r.Receipt != nil || r.Failure == nil {
		t.Fatalf("result = %#v", r)
	}
	if strings.Contains(r.Failure.Error(), secret) {
		t.Fatalf("failure leaked selector: %v", r.Failure)
	}
	if r.Failure.Stage == "" || r.Failure.Code == "" {
		t.Fatalf("unbounded failure = %#v", r.Failure)
	}
}
