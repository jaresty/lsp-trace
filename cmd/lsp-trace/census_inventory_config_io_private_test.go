//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
)

// Worker-only fixture: the blocked baseline is confined to this process. The
// parent always kills and Waits it before reporting an assertion result.
func TestCensusPrivateConfigIOWorker(t *testing.T) {
	root := os.Getenv("LSP_TRACE_CONFIG_IO_WORKER_ROOT")
	if root == "" {
		t.Skip("owned subprocess only")
	}
	mode := os.Getenv("LSP_TRACE_CONFIG_IO_WORKER_MODE")
	path := filepath.Join(root, "config.json")
	c, err := loadCensusInventoryConfigPrivate(path)
	if err != nil {
		t.Fatal("fixture initial regular admission", err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	} // only this fixture's own file
	if err = syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(mode, "HELD_WRITER") {
		fd, e := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if e != nil {
			t.Fatal(e)
		}
		keeper := os.NewFile(uintptr(fd), "owned-fifo-keeper")
		defer keeper.Close()
		if _, e = keeper.Write([]byte("{")); e != nil {
			t.Fatal(e)
		}
	}
	if strings.HasPrefix(mode, "INITIAL") {
		_, _, _, args := inventoryFixturePrivate(t)
		for i, a := range args {
			if a == "--private-inventory-config" {
				args[i+1] = path
			}
		}
		deps, starts := inventoryDepsPrivate(t)
		if err = os.WriteFile(filepath.Join(root, "entered"), []byte(mode), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errout bytes.Buffer
		code := runCensusWithDependencies(args, &out, &errout, deps)
		if code != 1 || starts.Load() != 0 || out.Len() != 0 {
			t.Fatalf("wrong initial refusal code=%d starts=%d", code, starts.Load())
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()
		if err = os.WriteFile(filepath.Join(root, "entered"), []byte(mode), 0600); err != nil {
			t.Fatal(err)
		}
		enumerated := false
		err = revalidateCensusInventoryDisclosurePrivate(ctx, censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			enumerated = true
			return nil, errors.New("owned unexpected enumeration")
		})
		if err == nil || enumerated {
			t.Fatalf("wrong disclosure refusal err=%v enumerated=%v", err, enumerated)
		}
	}
	fmt.Println("CONFIG_IO_REFUSED", mode)
}

func TestCensusPrivateConfigIOFIFOBounded(t *testing.T) {
	for _, mode := range []string{"DISCLOSURE_NO_WRITER", "DISCLOSURE_HELD_WRITER", "INITIAL_NO_WRITER", "INITIAL_HELD_WRITER"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			saveInventoryConfigPrivate(t, filepath.Join(root, "config.json"), inventoryConfigPrivate())
			cmd := exec.Command(os.Args[0], "-test.run=^TestCensusPrivateConfigIOWorker$", "-test.v")
			cmd.Env = append(os.Environ(), "LSP_TRACE_CONFIG_IO_WORKER_ROOT="+root, "LSP_TRACE_CONFIG_IO_WORKER_MODE="+mode, "GORACE=atexit_sleep_ms=0")
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				t.Fatal("subprocess setup", err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = cmd.Process.Kill()
					<-done
				}
			}()
			entered, completedEarly := false, false
			var earlyErr error
			setupDeadline := time.NewTimer(5 * time.Second)
			defer setupDeadline.Stop()
			for !entered {
				select {
				case err := <-done:
					joined = true
					if _, markerErr := os.Stat(filepath.Join(root, "entered")); markerErr != nil {
						t.Fatalf("fixture did not reach governed call: %v %s", err, output.String())
					}
					entered, completedEarly, earlyErr = true, true, err
				case <-setupDeadline.C:
					t.Fatal("fixture entry setup timed out")
				case <-time.After(2 * time.Millisecond):
					_, err := os.Stat(filepath.Join(root, "entered"))
					entered = err == nil
				}
			}
			timer := time.NewTimer(500 * time.Millisecond)
			defer timer.Stop()
			returned, killed := completedEarly, false
			waitErr := earlyErr
			if !completedEarly {
				select {
				case waitErr = <-done:
					joined = true
					returned = true
				case <-timer.C:
					_ = cmd.Process.Kill()
					killed = true
					waitErr = <-done
					joined = true
				}
			}
			t.Logf("owned FIFO observation mode=%s entered=%v returned=%v killed=%v joined=%v wait=%v", mode, entered, returned, killed, joined, waitErr)
			if returned && waitErr != nil {
				t.Fatalf("fixture procedure failure, not assertion RED: %v %s", waitErr, output.String())
			}
			assertInventoryPrivate(t, "ASSERT_CONFIG_IO_"+mode+"_BOUNDED", entered && returned && !killed && joined && strings.Contains(output.String(), "CONFIG_IO_REFUSED "+mode))
		})
	}
}

func TestCensusPrivateConfigIOPreCancelledCallsite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := inventoryConfigPrivate()
	saveInventoryConfigPrivate(t, path, c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := revalidateCensusInventoryDisclosurePrivate(ctx, censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
		calls++
		return nil, errors.New("owned enumeration sentinel")
	})
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_CALLSITE_PRE_CANCELLED", errors.Is(err, context.Canceled) && calls == 0)
}

func TestCensusPrivateConfigIONoFollow(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "regular.json")
	saveInventoryConfigPrivate(t, path, inventoryConfigPrivate())
	alias := filepath.Join(root, "alias.json")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	_, err := loadCensusInventoryConfigPrivate(alias)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_SYMLINK_REFUSED", err != nil)
	_, err = loadCensusInventoryConfigPrivate(root)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_DIRECTORY_REFUSED", err != nil)
}

// Cancellation is injected when the actual configuration descriptor is visible,
// not after a guessed number of implementation callbacks. No blocking I/O worker
// or timeout-abandoned goroutine is used.
type configDescriptorCancelContextPrivate struct {
	context.Context
	cancel                  context.CancelFunc
	identity                os.FileInfo
	acquired                bool
	afterRead, readObserved bool
}

func (c *configDescriptorCancelContextPrivate) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	entries, err := configDescriptorNamesPrivate()
	if err != nil {
		return err
	}
	identity := c.identity.Sys().(*syscall.Stat_t)
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}
		var observed syscall.Stat_t
		err = syscall.Fstat(fd, &observed)
		if err == nil && identity.Dev == observed.Dev && identity.Ino == observed.Ino {
			c.acquired = true
			if c.afterRead {
				// Query this owned regular descriptor's offset without changing it.
				position, seekErr := syscall.Seek(fd, 0, io.SeekCurrent)
				if seekErr != nil || position == 0 {
					continue
				}
				c.readObserved = true
			}
			c.cancel()
			break
		}
	}
	return c.Context.Err()
}
func configDescriptorNamesPrivate() ([]string, error) {
	f, err := os.Open("/dev/fd")
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	return names, errors.Join(err, f.Close())
}
func configFDCountPrivate(t *testing.T) int {
	t.Helper()
	entries, err := configDescriptorNamesPrivate()
	if err != nil {
		t.Fatal("fd fixture setup", err)
	}
	if len(entries) < 3 {
		t.Fatal("fd fixture did not enumerate standard descriptors")
	}
	return len(entries)
}

func TestCensusPrivateConfigIOAcquiredCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := inventoryConfigPrivate()
	saveInventoryConfigPrivate(t, path, c)
	identity, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Warm the ordinary descriptor/poller path before measuring owned descriptor cleanup.
	if _, err = loadCensusInventoryConfigPrivate(path); err != nil {
		t.Fatal(err)
	}
	before := configFDCountPrivate(t)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &configDescriptorCancelContextPrivate{Context: base, cancel: cancel, identity: identity}
	calls := 0
	err = revalidateCensusInventoryDisclosurePrivate(ctx, censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
		calls++
		return nil, errors.New("owned enumeration sentinel")
	})
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_ACQUIRED_CANCELLED", ctx.acquired && errors.Is(err, context.Canceled) && calls == 0)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_CANCELLED_FDS_CLOSED", configFDCountPrivate(t) == before)
}

func TestCensusPrivateConfigIOReadCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c := inventoryConfigPrivate()
	saveInventoryConfigPrivate(t, path, c)
	identity, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = loadCensusInventoryConfigPrivate(path); err != nil {
		t.Fatal(err)
	}
	before := configFDCountPrivate(t)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &configDescriptorCancelContextPrivate{Context: base, cancel: cancel, identity: identity, afterRead: true}
	calls := 0
	err = revalidateCensusInventoryDisclosurePrivate(ctx, censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
		calls++
		return nil, errors.New("owned enumeration sentinel")
	})
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_READ_CONTEXT_CANCELLED", ctx.acquired && ctx.readObserved && errors.Is(err, context.Canceled) && calls == 0)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_READ_CANCEL_FDS_CLOSED", configFDCountPrivate(t) == before)
}

func TestCensusPrivateConfigIORegularCompatibility(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	c := inventoryConfigPrivate()
	saveInventoryConfigPrivate(t, path, c)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadCensusInventoryConfigPrivate(path)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_REGULAR_CANONICAL", err == nil)
	calls := 0
	enumerate := func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
		calls++
		return nil, nil
	}
	err = revalidateCensusInventoryDisclosurePrivate(context.Background(), censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, enumerate)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_REGULAR_REVALIDATION", err == nil && calls == 1)
	drift := c
	drift.MaxOrigins = 2
	saveInventoryConfigPrivate(t, path, drift)
	calls = 0
	err = revalidateCensusInventoryDisclosurePrivate(context.Background(), censusCLIOptions{PrivateInventoryConfigPath: path}, c, nil, nil, nil, enumerate)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_REGULAR_DRIFT", err != nil && calls == 0)
	// Padding this existing string makes canonical JSON exactly at/over the byte
	// boundary without adding unknown fields or changing loader-level validity.
	c.RevisionAssertion = ""
	saveInventoryConfigPrivate(t, path, c)
	base, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c.RevisionAssertion = strings.Repeat("a", (64<<10)-len(base))
	saveInventoryConfigPrivate(t, path, c)
	_, err = loadCensusInventoryConfigPrivate(path)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_EXACT_BYTE_CEILING", err == nil)
	c.RevisionAssertion += "a"
	saveInventoryConfigPrivate(t, path, c)
	_, err = loadCensusInventoryConfigPrivate(path)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_OVERSIZE_REFUSED", err != nil)
	saveInventoryConfigPrivate(t, path, inventoryConfigPrivate())
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("\n"))
	ce := f.Close()
	if err != nil || ce != nil {
		t.Fatal(err, ce)
	}
	_, err = loadCensusInventoryConfigPrivate(path)
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_NONCANONICAL_REFUSED", err != nil)
}

func TestCensusPrivateConfigIOFDClosure(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid.json")
	saveInventoryConfigPrivate(t, valid, inventoryConfigPrivate())
	alias := filepath.Join(root, "alias.json")
	if err := os.Symlink(valid, alias); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	over := filepath.Join(root, "oversized.json")
	if err := os.WriteFile(over, bytes.Repeat([]byte("x"), (64<<10)+1), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(root, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	driftPath := filepath.Join(root, "drift.json")
	expected := inventoryConfigPrivate()
	drift := expected
	drift.MaxOrigins = 2
	saveInventoryConfigPrivate(t, driftPath, drift)
	if _, err := loadCensusInventoryConfigPrivate(valid); err != nil {
		t.Fatal(err)
	}
	before := configFDCountPrivate(t)
	for i := 0; i < 32; i++ {
		for _, path := range []string{valid, alias, fifo, over, root, invalid} {
			_, _ = loadCensusInventoryConfigPrivate(path)
		}
		err := revalidateCensusInventoryDisclosurePrivate(context.Background(), censusCLIOptions{PrivateInventoryConfigPath: driftPath}, expected, nil, nil, nil, func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			return nil, errors.New("unexpected drift enumeration")
		})
		if err == nil {
			t.Error("regular configuration drift accepted")
		}
	}
	assertInventoryPrivate(t, "ASSERT_CONFIG_IO_ALL_FDS_CLOSED", configFDCountPrivate(t) == before)
}
