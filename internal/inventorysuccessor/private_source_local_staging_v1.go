//go:build darwin || linux

package inventorysuccessor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// This unexported method can be implemented only inside this package. Its sole
// implementation is in _test.go; no exported config/JSON/runtime flag enables it.
type controlledLocalWorkerFixtureV1 interface{ controlledWorkerFixture() bool }

func localWorkerFixtureV1(c PrivateSourceLocalConfigV1) bool {
	x, ok := c.io.(controlledLocalWorkerFixtureV1)
	return ok && x.controlledWorkerFixture()
}
func trustedSystemSandboxInfoV1(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Mode().Perm()&0111 == 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0
}
func (s *privateSourceRPreparationStateV1) prepareLocalStageV1() error {
	c := *s.local
	fixture := localWorkerFixtureV1(c)
	if fixture && c.Config.SandboxExecutable.Path == localSystemSandboxV1 {
		return ErrPrivateSourceLocalV1
	}
	if !fixture && (runtime.GOOS != "darwin" || c.Config.SandboxExecutable.Path != localSystemSandboxV1 || filepath.Base(c.Config.Library.Path) != "libllama.dylib") {
		return fmt.Errorf("%w: requires Darwin OS-protected sandbox-exec and isolated pinned libllama.dylib; no passthrough", ErrPrivateSourceLocalV1)
	}
	if _, e := localStageBytesV1(c); e != nil {
		return e
	}
	parentPath := filepath.Dir(c.Config.Worker.Path)
	ns := &localStageNamespaceV1{parentPath: parentPath, name: ".source-worker-" + s.request.Attempt[7:]}
	parent, parentInfo, e := openProtectedLocalDirectoryV1(parentPath, &ns.aclScratch, c.io)
	if e != nil {
		return e
	}
	ns.parent, ns.parentInfo = parent, parentInfo
	s.localStage = ns // every acquired descriptor closes inside original Finalize
	dir := filepath.Join(parentPath, ns.name)
	if e = unix.Mkdirat(int(parent.Fd()), ns.name, 0700); e != nil {
		return e
	}
	ns.created = true
	ns.dir, ns.dirInfo, e = openOwnedStageDirectoryV1(parent, ns.name, &ns.aclScratch, c.io)
	if e != nil {
		return e
	}
	s.localStageDir = dir
	if e = unix.Mkdirat(int(ns.dir.Fd()), "lib", 0700); e != nil {
		return e
	}
	ns.libCreated = true
	ns.lib, ns.libInfo, e = openOwnedStageDirectoryV1(ns.dir, "lib", &ns.aclScratch, c.io)
	if e != nil {
		return e
	}
	staged := c.Config
	staged.Worker.Path = filepath.Join(dir, "worker")
	staged.Model.Path = filepath.Join(dir, "model")
	staged.Library.Path = filepath.Join(dir, "lib", filepath.Base(c.Config.Library.Path))
	staged.SandboxProfile.Path = filepath.Join(dir, "profile.sb")
	staged.Grammar.Path = filepath.Join(dir, "grammar.gbnf")
	if fixture {
		staged.SandboxExecutable.Path = filepath.Join(dir, "sandbox")
	}
	s.localStagedConfig = staged
	s.localExecutorContract = "DARWIN_OS_SANDBOX_CLOSED_READONLY_V1"
	if fixture {
		s.localExecutorContract = "CONTROLLED_TEST_EXECUTOR_NOT_OS_QUALIFIED"
	}
	return nil
}

// The trusted controller UID and root may maintain their own namespace. No
// other UID, writable ancestor, symlink, or unclassified ACL is admitted.
// Paths are used by exec only inside this protected chain; descriptors retain
// the exact parent/stage/lib objects until original pending Finalize closes them.
type localStageNamespaceV1 struct {
	parentPath, name             string
	parent, dir, lib             *os.File
	parentInfo, dirInfo, libInfo *syscall.Stat_t
	created, libCreated          bool
	aclScratch                   [4096]byte
}

// Only new namespace metadata uses this primitive; A's pin/EOF path is unchanged.
func singleLocalDirectoryFstatV1(fd uintptr, stat *syscall.Stat_t) error {
	// Inspected generated syscall.Fstat: one libc Fstat/Fstat64 on Darwin,
	// one SYS_FSTAT on Linux/amd64; no Go EINTR retry as in os.File.Stat.
	return syscall.Fstat(int(fd), stat)
}
func statLocalNamespaceDirectoryV1(f *os.File, ioOwner privateLocalIOV1) (*syscall.Stat_t, error) {
	conn, e := f.SyscallConn()
	if e != nil {
		return nil, e
	}
	attempt := singleLocalDirectoryFstatV1
	// Sole alternate implementation is the package-private test IO observer:
	// inject an error or delegate the SAME FD operation, never fabricate metadata.
	if fixture, ok := ioOwner.(interface {
		namespaceFstat(uintptr, *syscall.Stat_t) error
	}); ok {
		attempt = fixture.namespaceFstat
	}
	var stat syscall.Stat_t
	var statErr error
	// Control holds an FD reference across exactly one callback. EINTR is terminal;
	// no poll.Read, os.File.Stat, ignoringEINTR, or retry loop is used here.
	e = conn.Control(func(fd uintptr) { statErr = attempt(fd, &stat) })
	if e != nil || statErr != nil {
		return nil, errors.Join(e, statErr)
	}
	return &stat, nil
}
func protectedLocalDirectoryInfoV1(info *syscall.Stat_t, owned bool) bool {
	if info == nil || info.Mode&syscall.S_IFMT != syscall.S_IFDIR || info.Mode&0022 != 0 {
		return false
	}
	uid := uint32(os.Geteuid())
	if owned {
		return info.Uid == uid && info.Mode&0777 == 0700
	}
	return info.Uid == 0 || info.Uid == uid
}
func sameLocalDirectoryV1(before, after *syscall.Stat_t, owned bool) bool {
	return protectedLocalDirectoryInfoV1(before, owned) && protectedLocalDirectoryInfoV1(after, owned) &&
		before.Dev == after.Dev && before.Ino == after.Ino && before.Mode == after.Mode &&
		before.Uid == after.Uid && before.Gid == after.Gid
}
func openProtectedLocalDirectoryV1(path string, scratch *[4096]byte, ioOwner privateLocalIOV1) (*os.File, *syscall.Stat_t, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 {
		return nil, nil, ErrPrivateSourceLocalV1
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if path == "/" {
		parts = nil
	}
	if len(parts) > 128 {
		return nil, nil, ErrPrivateSourceLocalV1
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if e != nil {
		return nil, nil, e
	}
	f := os.NewFile(uintptr(fd), "/")
	info, e := statLocalNamespaceDirectoryV1(f, ioOwner)
	admit := func() error {
		if e != nil || !protectedLocalDirectoryInfoV1(info, false) {
			return errors.Join(ErrPrivateSourceLocalV1, e)
		}
		return localDirectoryACLAdmissionV1(f, scratch)
	}
	if e = admit(); e != nil {
		return nil, nil, errors.Join(e, f.Close())
	}
	for _, part := range parts {
		fd, oe := unix.Openat(int(f.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		ce := f.Close()
		if oe != nil {
			return nil, nil, errors.Join(oe, ce)
		}
		f = os.NewFile(uintptr(fd), part)
		if ce != nil {
			return nil, nil, errors.Join(ce, f.Close())
		}
		info, e = statLocalNamespaceDirectoryV1(f, ioOwner)
		if e = admit(); e != nil {
			return nil, nil, errors.Join(e, f.Close())
		}
	}
	return f, info, nil
}
func openOwnedStageDirectoryV1(parent *os.File, name string, scratch *[4096]byte, ioOwner privateLocalIOV1) (*os.File, *syscall.Stat_t, error) {
	fd, e := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if e != nil {
		return nil, nil, e
	}
	f := os.NewFile(uintptr(fd), name)
	info, e := statLocalNamespaceDirectoryV1(f, ioOwner)
	if e != nil || !protectedLocalDirectoryInfoV1(info, true) {
		return nil, nil, errors.Join(ErrPrivateSourceLocalV1, e, f.Close())
	}
	if e = localDirectoryACLAdmissionV1(f, scratch); e != nil {
		return nil, nil, errors.Join(e, f.Close())
	}
	return f, info, nil
}
func (s *privateSourceRPreparationStateV1) localIOForNamespaceV1() privateLocalIOV1 {
	if s.local == nil {
		return nil
	}
	return s.local.io
}
func (s *privateSourceRPreparationStateV1) validateLocalStageV1() error {
	ns := s.localStage
	if ns == nil {
		return ErrPrivateSourceLocalV1
	}
	parent, info, e := openProtectedLocalDirectoryV1(ns.parentPath, &ns.aclScratch, s.localIOForNamespaceV1())
	if e != nil {
		return e
	}
	ce := parent.Close()
	if !sameLocalDirectoryV1(ns.parentInfo, info, false) || ce != nil {
		return errors.Join(ErrPrivateSourceLocalV1, ce)
	}
	for _, item := range []struct {
		parent *os.File
		name   string
		f      *os.File
		info   *syscall.Stat_t
		owned  bool
	}{
		{nil, "", ns.parent, ns.parentInfo, false},
		{ns.parent, ns.name, ns.dir, ns.dirInfo, true},
		{ns.dir, "lib", ns.lib, ns.libInfo, true},
	} {
		if item.f == nil {
			continue
		}
		current, se := statLocalNamespaceDirectoryV1(item.f, s.localIOForNamespaceV1())
		if se != nil || !sameLocalDirectoryV1(item.info, current, item.owned) {
			return errors.Join(ErrPrivateSourceLocalV1, se)
		}
		if e = localDirectoryACLAdmissionV1(item.f, &ns.aclScratch); e != nil {
			return e
		}
		if item.parent != nil {
			f, current, oe := openOwnedStageDirectoryV1(item.parent, item.name, &ns.aclScratch, s.localIOForNamespaceV1())
			if oe != nil {
				return oe
			}
			ce := f.Close()
			if !sameLocalDirectoryV1(item.info, current, true) || ce != nil {
				return errors.Join(ErrPrivateSourceLocalV1, ce)
			}
		}
	}
	return nil
}
func openLocalStageFileV1(parent *os.File, name string) (*os.File, error) {
	fd, e := unix.Openat(int(parent.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), name), nil
}
func (s *privateSourceRPreparationStateV1) removeLocalPromptV1() error {
	if e := s.validateLocalStageV1(); e != nil {
		return e
	}
	e := unix.Unlinkat(int(s.localStage.dir.Fd()), "prompt.txt", 0)
	if errors.Is(e, unix.ENOENT) {
		return nil
	}
	return e
}

type localStageWriterV1 struct {
	f       *os.File
	profile []byte
	collect bool
	managed bool
}

func (w *localStageWriterV1) Write(b []byte) (int, error) {
	n, e := w.f.Write(b)
	if w.collect {
		if len(w.profile)+n > 16384 {
			return n, ErrPrivateSourceLocalV1
		}
		w.profile = append(w.profile, b[:n]...)
	}
	return n, e
}
func (s *privateSourceRPreparationStateV1) stageWriterV1(i int) (*localStageWriterV1, error) {
	if i == 3 && !localWorkerFixtureV1(*s.local) {
		return nil, nil
	}
	if e := s.validateLocalStageV1(); e != nil {
		return nil, e
	}
	paths := localPinsV1(s.localStagedConfig)
	parent := s.localStage.dir
	if i == 2 {
		parent = s.localStage.lib
	}
	f, e := openLocalStageFileV1(parent, filepath.Base(paths[i].Path))
	if e != nil {
		return nil, e
	}
	return &localStageWriterV1{f: f, collect: i == 4, managed: s.local.managedAdmission}, nil
}
func finishLocalStageWriterV1(w *localStageWriterV1, i int) error {
	if w == nil {
		return nil
	}
	mode := os.FileMode(0400)
	if i == 0 || i == 3 {
		mode = 0500
	}
	var policy error
	expectedProfile := privateLocalSandboxProfileV1
	if w.managed {
		expectedProfile = privateManagedSandboxProfileV1
	}
	if i == 4 && !bytes.Equal(w.profile, []byte(expectedProfile)) {
		policy = fmt.Errorf("%w: closed network/file-write/read policy required", ErrPrivateSourceLocalV1)
	}
	return errors.Join(policy, w.f.Sync(), w.f.Chmod(mode), w.f.Close())
}
func (s *privateSourceRPreparationStateV1) cleanupLocalStageV1() (result error) {
	ns := s.localStage
	if ns == nil {
		return nil
	}
	defer func() {
		for _, f := range []*os.File{ns.lib, ns.dir, ns.parent} {
			if f != nil {
				result = errors.Join(result, f.Close())
			}
		}
		s.localStage = nil
	}()
	// Same package-private observer as worker start/join, for controlled metadata
	// interruption at Finalize. Production IO has no implementation of this seam.
	if observer, ok := s.localIOForNamespaceV1().(interface{ observeEffect(string) error }); ok {
		if e := observer.observeEffect("namespace-cleanup"); e != nil {
			return e
		}
	}
	// No deletion if any acquired identity or its protected ancestry is uncertain.
	if e := s.validateLocalStageV1(); e != nil {
		return e
	}
	if ns.created && ns.dir == nil || ns.libCreated && ns.lib == nil {
		return ErrPrivateSourceLocalV1
	}
	if ns.dir == nil {
		return nil
	} // exclusive mkdir failed: no owned entry
	remove := func(parent *os.File, name string, flags int) {
		e := unix.Unlinkat(int(parent.Fd()), name, flags)
		if !errors.Is(e, unix.ENOENT) {
			result = errors.Join(result, e)
		}
	}
	for _, name := range []string{"worker", "model", "profile.sb", "grammar.gbnf", "prompt.txt", "sandbox"} {
		remove(ns.dir, name, 0)
	}
	if ns.lib != nil {
		remove(ns.lib, filepath.Base(s.local.Config.Library.Path), 0)
		remove(ns.dir, "lib", unix.AT_REMOVEDIR)
	}
	remove(ns.parent, ns.name, unix.AT_REMOVEDIR)
	return result
}
func (s *privateSourceRPreparationStateV1) checkLocalStageV1() error {
	if e := s.validateLocalStageV1(); e != nil {
		return e
	}
	expected := map[string]bool{"worker": true, "model": true, "profile.sb": true, "grammar.gbnf": true, "prompt.txt": true, "lib": true}
	if s.localManagedUse != nil {
		expected = map[string]bool{"prompt.txt": true, "lib": true}
	}
	if localWorkerFixtureV1(*s.local) && s.localManagedUse == nil {
		expected["sandbox"] = true
	}
	for _, item := range []struct {
		dir   *os.File
		names map[string]bool
	}{{s.localStage.dir, expected}, {s.localStage.lib, func() map[string]bool {
		if s.localManagedUse != nil {
			return map[string]bool{}
		}
		return map[string]bool{filepath.Base(s.localStagedConfig.Library.Path): true}
	}()}} {
		fd, e := unix.Openat(int(item.dir.Fd()), ".", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_DIRECTORY, 0)
		if e != nil {
			return e
		}
		f := os.NewFile(uintptr(fd), "stage-list")
		entries, re := f.ReadDir(len(item.names) + 1)
		if re != nil && re != io.EOF {
			return errors.Join(re, f.Close())
		}
		valid := len(entries) == len(item.names)
		for _, entry := range entries {
			if !item.names[entry.Name()] {
				valid = false
			}
		}
		ce := f.Close()
		if !valid || ce != nil {
			return errors.Join(ErrPrivateSourceLocalV1, ce)
		}
	}
	return nil
}

func (p PendingLocalSourceRV1) WorkerOutputObservation() (PrivateSourceLocalWorkerObservationV1, error) {
	if p.state == nil {
		return PrivateSourceLocalWorkerObservationV1{}, ErrPrivateSourceLocalV1
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.localLive() || s.localFailure != nil || s.localWorkerObservation.RawStdoutBytes == 0 {
		return PrivateSourceLocalWorkerObservationV1{}, ErrPrivateSourceLocalV1
	}
	return s.localWorkerObservation, nil
}

// This is only the documented SetEscapeHTML(false)->strict outer transport
// escaping difference. It neither parses nor re-marshals model semantic content;
// all numeric lexemes and decoded text bytes stay unchanged. The existing strict
// parser still rejects every other noncanonical/invalid form.
func frameLocalWorkerStdoutV1(raw []byte, cap int) ([]byte, error) {
	n := len(raw)
	for _, b := range raw {
		if b == '&' || b == '<' || b == '>' {
			n += 5
			if n > cap {
				return nil, ErrPrivateSourceLocalV1
			}
		}
	}
	if n > cap {
		return nil, ErrPrivateSourceLocalV1
	}
	out := make([]byte, 0, n)
	for _, b := range raw {
		switch b {
		case '&':
			out = append(out, `\u0026`...)
		case '<':
			out = append(out, `\u003c`...)
		case '>':
			out = append(out, `\u003e`...)
		default:
			out = append(out, b)
		}
	}
	return out, nil
}
