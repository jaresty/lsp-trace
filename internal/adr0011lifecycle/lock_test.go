package adr0011lifecycle

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func canonicalSyntheticTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSymlinkedParentDenied(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"private", "anchor"} {
		if err := os.Mkdir(filepath.Join(real, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	l, err := openSynthetic(filepath.Join(alias, "private"), filepath.Join(alias, "anchor"))
	if err == nil {
		_ = l.close()
		t.Fatal("ASSERT_SYMLINKED_PARENT_DENIED: accepted sibling paths through symlinked parent")
	}
}

func TestCaseFoldedDirectoryAliasDenied(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	for _, name := range []string{"Private", "Anchor"} {
		if err := os.Mkdir(filepath.Join(parent, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	if _, err := os.Lstat(root); err != nil {
		t.Skip("filesystem does not resolve case-folded alias")
	}
	l, err := openSynthetic(root, anchor)
	if err == nil {
		_ = l.close()
		t.Fatal("case-folded directory spelling admitted")
	}
}

func TestAncestryReplacementDeniedAtValidation(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	for _, when := range []string{"before", "after"} {
		t.Run(when, func(t *testing.T) {
			parent := canonicalSyntheticTempDir(t)
			host := filepath.Join(parent, "host")
			if err := os.Mkdir(host, 0700); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(host, "private")
			anchor := filepath.Join(host, "anchor")
			for _, p := range []string{root, anchor} {
				if err := os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			l, err := openSynthetic(root, anchor)
			if err != nil {
				t.Fatal(err)
			}
			defer l.close()
			replace := func() error {
				if err := os.Rename(host, host+"-moved"); err != nil {
					return err
				}
				return os.Symlink(host+"-moved", host)
			}
			if when == "before" {
				if err := replace(); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			err = l.withLock(func() error {
				called = true
				if when == "after" {
					return replace()
				}
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "ancestry") {
				t.Fatalf("changed ancestor admitted: %v", err)
			}
			if when == "before" && called {
				t.Fatal("callback ran after ancestor replacement")
			}
			if when == "after" && !called {
				t.Fatal("callback did not reach post-lock validation")
			}
		})
	}
}
func TestNonCanonicalSyntheticPathsDenied(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []string{root + string(filepath.Separator), filepath.Join(parent, ".", "private") + "/./", "private"} {
		l, err := openSynthetic(bad, anchor)
		if err == nil {
			_ = l.close()
			t.Fatalf("noncanonical root admitted: %q", bad)
		}
	}
}

func TestUnlockFailureReturned(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	l, err := openSynthetic(root, anchor)
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	unlockFailure := errors.New("injected unlock failure")
	l.unlockForTest = func(f *os.File) error { _ = unlockExclusive(f); return unlockFailure }
	if err := l.withLock(func() error { return nil }); !errors.Is(err, unlockFailure) {
		t.Fatalf("ASSERT_UNLOCK_FAILURE_DENIES_SUCCESS: %v", err)
	}
	callbackFailure := errors.New("callback failure")
	err = l.withLock(func() error { return callbackFailure })
	if !errors.Is(err, unlockFailure) || !errors.Is(err, callbackFailure) {
		t.Fatalf("ASSERT_UNLOCK_PRESERVES_CALLBACK_ERROR: %v", err)
	}
	path := filepath.Join(root, "adr0011-root.lock")
	err = l.withLock(func() error { return os.Rename(path, filepath.Join(root, "moved.lock")) })
	if !errors.Is(err, unlockFailure) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ASSERT_UNLOCK_PRESERVES_POST_VALIDATION_ERROR: %v", err)
	}
	if err := os.Rename(filepath.Join(root, "moved.lock"), path); err != nil {
		t.Fatal(err)
	}
	unlocked := false
	l.unlockForTest = func(f *os.File) error { unlocked = true; return unlockExclusive(f) }
	func() {
		defer func() {
			if r := recover(); r != "expected panic" {
				t.Errorf("panic changed: %v", r)
			}
		}()
		_ = l.withLock(func() error { panic("expected panic") })
	}()
	if !unlocked {
		t.Fatal("deferred unlock not attempted on panic")
	}
}

func TestTwoProcessRootLock(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	l, err := openSynthetic(root, anchor)
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	ready := filepath.Join(parent, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$")
	cmd.Env = append(os.Environ(), "ADR_LOCK_HELPER=1", "ADR_LOCK_ROOT="+root, "ADR_LOCK_ANCHOR="+anchor, "ADR_LOCK_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper failed to acquire lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	err = l.withLock(func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("ASSERT_B_DENIED_WHILE_A_HOLDS: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := l.withLock(func() error { return nil }); err != nil {
		t.Fatalf("OS lock not released after death: %v", err)
	}
}
func TestLockHelper(t *testing.T) {
	if os.Getenv("ADR_LOCK_HELPER") != "1" {
		return
	}
	l, err := openSynthetic(os.Getenv("ADR_LOCK_ROOT"), os.Getenv("ADR_LOCK_ANCHOR"))
	if err != nil {
		t.Fatal(err)
	}
	err = l.withLock(func() error {
		if err := os.WriteFile(os.Getenv("ADR_LOCK_READY"), []byte("ready"), 0600); err != nil {
			return err
		}
		for {
			time.Sleep(time.Second)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestHardLinkedLockDenied(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "adr0011-root.lock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if l, err := openSynthetic(root, anchor); err == nil {
		_ = l.close()
		t.Fatal("hard-linked lock admitted")
	}
}

func TestUnsupportedPlatformDenies(t *testing.T) {
	if supportedLock {
		t.Skip("supported platform")
	}
	if _, err := openSynthetic("/unused/private", "/unused/anchor"); err == nil {
		t.Fatal("unsupported platform admitted")
	}
}

func TestLockReplacementDenied(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	l, err := openSynthetic(root, anchor)
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	if err := os.Rename(filepath.Join(root, "adr0011-root.lock"), filepath.Join(root, "old.lock")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "adr0011-root.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.withLock(func() error { return nil }); err == nil {
		t.Fatal("replacement lock inode admitted")
	}
}
