//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// These names are exclusively issued by applicationWriteOnce. Native journal,
// mailbox and locator records have their separate existing durability contracts.
// An older unmarked application terminal is UNKNOWN, not inferred committed.
func applicationPublicationManaged(name string) bool {
	name = filepath.Base(name)
	return strings.HasPrefix(name, "terminal-") || strings.HasPrefix(name, "role-") || strings.HasPrefix(name, "host-wait-") || name == "source.json" || name == "application-original-claim.json"
}
func applicationPublicationIntent(name string) string {
	return ".application-intent-" + pilot.Digest([]byte(name))[7:]
}
func applicationPublicationCompletion(name string, st os.FileInfo, raw []byte) string {
	return "complete-" + pilot.Digest([]byte(name + "/" + applicationFileIdentity(st) + "/" + pilot.Digest(raw)))[7:]
}
func applicationPublicationLock(f *os.File, lanes ...*applicationFinalizeLane) error {
	if e := applicationFinalizeCheck(lanes...); e != nil {
		return e
	}
	e := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	if ce := applicationFinalizeCheck(lanes...); ce != nil {
		return ce
	}
	if errors.Is(e, unix.EWOULDBLOCK) {
		return os.ErrNotExist
	}
	return e
}

// Fixed-name, bounded observations; no scans, repairs, durability recovery,
// volatile registry, cleanup, or claim refunds. Native metadata outside this
// protocol retains its existing read contract. For this protocol the intent was
// synced before rename, so a crash cannot expose a final without its intent.
// Same-UID removal/forgery of the protocol state is NOT authenticated here.
func applicationPublicationAdmitted(root *os.Root, name string, f *os.File, raw []byte, lanes ...*applicationFinalizeLane) error {
	check := func() error { return applicationFinalizeCheck(lanes...) }
	if e := check(); e != nil {
		return e
	}
	intent := filepath.Join(filepath.Dir(name), applicationPublicationIntent(filepath.Base(name)))
	st, e := root.Lstat(intent)
	if ce := check(); ce != nil {
		return ce
	}
	if os.IsNotExist(e) {
		if applicationPublicationManaged(name) {
			return product.ErrInvalid
		}
		return nil
	}
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode().Perm() != 0700 {
		return product.ErrInvalid
	}
	held, e := f.Stat()
	if ce := check(); ce != nil {
		return ce
	}
	if e != nil {
		return e
	}
	visible, e := root.Lstat(name)
	if ce := check(); ce != nil {
		return ce
	}
	if e != nil || !os.SameFile(held, visible) || applicationFileIdentity(held) != applicationFileIdentity(visible) {
		return product.ErrInvalid
	}
	completion := filepath.Join(intent, applicationPublicationCompletion(filepath.Base(name), held, raw))
	evidence, e := root.Lstat(completion)
	if ce := check(); ce != nil {
		return ce
	}
	if os.IsNotExist(e) {
		return product.ErrInvalid
	}
	if e != nil {
		return e
	}
	if !evidence.IsDir() || evidence.Mode().Perm() != 0700 {
		return product.ErrInvalid
	}
	return check()
}
