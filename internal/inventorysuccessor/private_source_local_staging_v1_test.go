//go:build darwin || linux

package inventorysuccessor

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The inspected Go 1.26.5 os.File.Stat -> poll.Fstat -> ignoringEINTR
// chain retries indefinitely. This guard observes use of that known wrapper in
// the new namespace only; it does not pretend to deliver kernel EINTR signals.
func TestLocalStageMetadataNoRetryWrapper(t *testing.T) {
	file, e := parser.ParseFile(token.NewFileSet(), "private_source_local_staging_v1.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	sites := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Stat" && len(call.Args) == 0 {
			sites++
		}
		return true
	})
	if sites != 0 {
		t.Fatalf("ASSERT_NAMESPACE_METADATA_SINGLE_ATTEMPT: hidden os.File.Stat retry sites=%d", sites)
	}
	t.Log("ASSERT_NAMESPACE_METADATA_SINGLE_ATTEMPT: PASS; no hidden os.File.Stat retries in namespace")
}

// Controlled errors precede the actual operation. Success always delegates the
// SAME FD operation; neither this seam nor its fault fixture invents metadata.
func (c *controlledLocalIOV1) namespaceFstat(fd uintptr, stat *syscall.Stat_t) error {
	if e := c.observe("namespace-stat"); e != nil {
		return e
	}
	return singleLocalDirectoryFstatV1(fd, stat)
}

type localNamespaceStatFaultV1 struct{ calls int }

func (*localNamespaceStatFaultV1) open(path string) (privateLocalFileV1, error) {
	return openLocalPinV1(path)
}
func (x *localNamespaceStatFaultV1) namespaceFstat(fd uintptr, stat *syscall.Stat_t) error {
	x.calls++
	if x.calls == 1 {
		return syscall.EINTR
	}
	return singleLocalDirectoryFstatV1(fd, stat)
}
func TestLocalStageMetadataInterruptedAttempt(t *testing.T) {
	f, e := os.Open(".")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	fault := &localNamespaceStatFaultV1{}
	metadata, e := statLocalNamespaceDirectoryV1(f, fault)
	if !errors.Is(e, syscall.EINTR) || metadata != nil || fault.calls != 1 {
		t.Fatal("ASSERT_NAMESPACE_EINTR_ONCE", fault.calls, metadata, e)
	}
	t.Log("ASSERT_NAMESPACE_EINTR_ONCE: PASS; one controlled attempt, EINTR surfaced, no metadata")
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	fault.calls = 0
	if _, e = statLocalNamespaceDirectoryV1(f, fault); e == nil || fault.calls != 0 {
		t.Fatal("ASSERT_NAMESPACE_CLOSED_FD", fault.calls, e)
	}
	t.Log("ASSERT_NAMESPACE_CLOSED_FD: PASS; no attempt on closed descriptor")
}

func TestLocalStageDirectoryAdmissionMetadata(t *testing.T) {
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(cwd, ".stage-metadata-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	f, e := os.Open(base)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	info, e := statLocalNamespaceDirectoryV1(f, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !protectedLocalDirectoryInfoV1(info, true) {
		t.Fatal("ASSERT_STAGE_METADATA_VALID")
	}
	if protectedLocalDirectoryInfoV1(nil, false) {
		t.Fatal("ASSERT_STAGE_METADATA_UNKNOWN")
	}
	st := *info
	st.Uid = uint32(os.Geteuid()) + 1
	if st.Uid == 0 {
		st.Uid++
	}
	if protectedLocalDirectoryInfoV1(&st, false) {
		t.Fatal("ASSERT_STAGE_METADATA_OWNER")
	}
	if e = os.Chmod(base, 0777); e != nil {
		t.Fatal(e)
	}
	after, e := statLocalNamespaceDirectoryV1(f, nil)
	if e != nil {
		t.Fatal(e)
	}
	if protectedLocalDirectoryInfoV1(after, false) || sameLocalDirectoryV1(info, after, true) {
		t.Fatal("ASSERT_STAGE_METADATA_WRITABLE")
	}
	var scratch [4096]byte
	if _, _, e = openProtectedLocalDirectoryV1(base, &scratch, nil); e == nil {
		t.Fatal("ASSERT_STAGE_METADATA_WRITABLE_OPEN")
	}
}

func TestLocalStageCleanupIdentityMismatch(t *testing.T) {
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(cwd, ".stage-identity-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	var scratch [4096]byte
	parent, pi, e := openProtectedLocalDirectoryV1(base, &scratch, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"owned", "different"} {
		if e = os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	dir, _, e := openOwnedStageDirectoryV1(parent, "owned", &scratch, nil)
	if e != nil {
		t.Fatal(e)
	}
	otherFD, e := os.Open(filepath.Join(base, "different"))
	if e != nil {
		t.Fatal(e)
	}
	other, e := statLocalNamespaceDirectoryV1(otherFD, nil)
	closeOther := otherFD.Close()
	if closeOther != nil {
		t.Fatal(closeOther)
	}
	if e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(base, "owned", "worker")
	if e = os.WriteFile(marker, []byte("non-executable metadata fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	s := privateSourceRPreparationStateV1{localStage: &localStageNamespaceV1{parentPath: base, name: "owned", parent: parent, parentInfo: pi, dir: dir, dirInfo: other, created: true}}
	if e = s.cleanupLocalStageV1(); e == nil {
		t.Fatal("ASSERT_STAGE_CLEANUP_IDENTITY")
	}
	if _, e = os.Stat(marker); e != nil {
		t.Fatal("ASSERT_STAGE_CLEANUP_NO_UNCERTAIN_DELETE", e)
	}
	if _, e = parent.Stat(); e == nil {
		t.Fatal("ASSERT_STAGE_CLEANUP_PARENT_CLOSED")
	}
	if _, e = dir.Stat(); e == nil {
		t.Fatal("ASSERT_STAGE_CLEANUP_DIR_CLOSED")
	}
	t.Log("ASSERT_STAGE_CLEANUP_IDENTITY: PASS; non-executable mismatched metadata refused deletion and closed original FDs")
}
