//go:build darwin || linux

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"strconv"
	"sync"
	"syscall"
	"testing"
)

func privateSourcePrepareTraceFD(t *testing.T, mode int, permission os.FileMode) (int, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trace")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, permission)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Chmod(permission); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil || info.Mode().Perm() != permission {
		_ = f.Close()
		t.Fatal("ASSERT_TRACE_FIXTURE_MODE: FAIL")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	fd, err := syscall.Open(path, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	return fd, path
}

func privateSourcePrepareTraceClosed(t *testing.T, fd int) {
	t.Helper()
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if errno != syscall.EBADF {
		t.Fatal("ASSERT_TRACE_FD_CLOSED: FAIL")
	}
}

func TestPrivateSourcePrepareTraceDefaultAndNativeRefusal(t *testing.T) {
	t.Log("PROCEDURE: original and wrapped missing-config refusal; no runtime/provider entry")
	t.Setenv(privatePrepareTraceFDEnv, "")
	if err := os.Unsetenv(privatePrepareTraceFDEnv); err != nil {
		t.Fatal(err)
	}
	args := []string{"source-prepare-describe"}
	var originalOut, originalErr, offOut, offErr bytes.Buffer
	want := runPrivateSourcePrepareDescribe(args, &originalOut, &originalErr)
	if got := runPrivateSourcePrepareDescribeTraced(args, &offOut, &offErr); got != want || !bytes.Equal(offOut.Bytes(), originalOut.Bytes()) || !bytes.Equal(offErr.Bytes(), originalErr.Bytes()) || trace.IsEnabled() {
		t.Fatal("ASSERT_TRACE_DEFAULT_IDENTITY: FAIL")
	}
	t.Log("ASSERT_TRACE_DEFAULT_IDENTITY: PASS")
	fd, path := privateSourcePrepareTraceFD(t, syscall.O_RDWR, 0600)
	t.Setenv(privatePrepareTraceFDEnv, strconv.Itoa(fd))
	var onOut, onErr bytes.Buffer
	if got := runPrivateSourcePrepareDescribeTraced(args, &onOut, &onErr); got != want || !bytes.Equal(onOut.Bytes(), originalOut.Bytes()) || !bytes.Equal(onErr.Bytes(), originalErr.Bytes()) || trace.IsEnabled() {
		t.Fatal("ASSERT_TRACE_NATIVE_IDENTITY: FAIL")
	}
	privateSourcePrepareTraceClosed(t, fd)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || len(raw) >= privatePrepareTraceBytes || !bytes.HasPrefix(raw, []byte("go 1.")) || !bytes.Contains(raw, []byte("TRACE_START")) || !bytes.Contains(raw, []byte("TRACE_RETURN_NONZERO")) {
		t.Fatal("ASSERT_TRACE_FLUSH: FAIL")
	}
	t.Log("ASSERT_TRACE_NATIVE_IDENTITY: PASS")
	t.Log("ASSERT_TRACE_FLUSH: PASS")
}

func TestPrivateSourcePrepareTraceCanonicalFD(t *testing.T) {
	for _, raw := range []string{"", "-1", "0", "1", "2", "+3", "03", " 3", "3 ", "3.0", "/tmp/trace", "9999999999999999999999999"} {
		t.Setenv(privatePrepareTraceFDEnv, raw)
		var out, errOut bytes.Buffer
		if runPrivateSourcePrepareDescribeTraced([]string{"source-prepare-describe"}, &out, &errOut) == 0 || out.Len() != 0 || errOut.Len() != 0 {
			t.Fatal("ASSERT_TRACE_FD_REFUSAL: FAIL", raw)
		}
	}
	t.Log("ASSERT_TRACE_FD_REFUSAL: PASS canonical spellings")
}

func TestPrivateSourcePrepareTraceAdmission(t *testing.T) {
	for _, mode := range []string{"valid", "permission", "size", "offset", "readonly", "append", "pipe", "output-alias"} {
		t.Run(mode, func(t *testing.T) {
			permission := os.FileMode(0600)
			flags := syscall.O_RDWR
			if mode == "permission" {
				permission = 0644
			}
			if mode == "readonly" {
				flags = syscall.O_RDONLY
			}
			if mode == "append" {
				flags |= syscall.O_APPEND
			}
			fd, _ := privateSourcePrepareTraceFD(t, flags, permission)
			if mode == "size" {
				if _, err := syscall.Write(fd, []byte("x")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "offset" {
				if _, err := syscall.Seek(fd, 1, io.SeekStart); err != nil {
					t.Fatal(err)
				}
			}
			var output io.Writer = io.Discard
			var alias *os.File
			if mode == "pipe" {
				_ = syscall.Close(fd)
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				fd, err = syscall.Dup(int(w.Fd()))
				if err != nil {
					t.Fatal(err)
				}
				_ = w.Close()
			}
			if mode == "output-alias" {
				dup, err := syscall.Dup(fd)
				if err != nil {
					t.Fatal(err)
				}
				alias = os.NewFile(uintptr(dup), "caller-output")
				defer alias.Close()
				output = applicationFullWriter{alias}
			}
			// Model a descriptor inherited without close-on-exec; admission must fix it.
			if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, 0); errno != 0 {
				t.Fatal(errno)
			}
			f := privatePrepareTraceFile(strconv.Itoa(fd), output, io.Discard)
			if mode == "valid" {
				if f == nil {
					t.Fatal("ASSERT_TRACE_FD_ADMISSION: FAIL valid")
				}
				got, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
				if errno != 0 || got&syscall.FD_CLOEXEC == 0 {
					t.Fatal("ASSERT_TRACE_CLOEXEC: FAIL")
				}
				t.Log("ASSERT_TRACE_CLOEXEC: PASS")
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			} else if f != nil {
				_ = f.Close()
				t.Fatal("ASSERT_TRACE_FD_ADMISSION: FAIL", mode)
			}
			privateSourcePrepareTraceClosed(t, fd)
			t.Log("ASSERT_TRACE_FD_ADMISSION: PASS", mode)
		})
	}
}

type privateSourcePrepareTraceShortWriter struct {
	calls, attempted int
	fail             bool
}

func (w *privateSourcePrepareTraceShortWriter) Write(p []byte) (int, error) {
	w.calls++
	w.attempted += len(p)
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return len(p) - 1, nil
}

func TestPrivateSourcePrepareTraceWriter(t *testing.T) {
	t.Log("PROCEDURE: exact cap, cap+1, terminal short/error and concurrent quota")
	for _, size := range []int{7, 8, 9} {
		var dst bytes.Buffer
		w := &privatePrepareTraceWriter{dst: &dst, remaining: 8}
		n, err := w.Write(make([]byte, size))
		if n != min(size, 8) || dst.Len() != min(size, 8) || (size > 8) != (err != nil) || w.overflow != (size > 8) {
			t.Fatal("ASSERT_TRACE_CAP: FAIL boundary")
		}
		_, _ = w.Write(make([]byte, 8))
		if dst.Len() > 8 {
			t.Fatal("ASSERT_TRACE_CAP: FAIL retry")
		}
	}
	for _, fail := range []bool{false, true} {
		dst := &privateSourcePrepareTraceShortWriter{fail: fail}
		w := &privatePrepareTraceWriter{dst: dst, remaining: 8}
		_, err := w.Write([]byte("1234"))
		if err == nil {
			t.Fatal("ASSERT_TRACE_CAP: FAIL short")
		}
		_, _ = w.Write([]byte("5678"))
		if dst.calls != 1 || dst.attempted != 4 || w.err == nil {
			t.Fatal("ASSERT_TRACE_CAP: FAIL retry error")
		}
	}
	var dst bytes.Buffer
	w := &privatePrepareTraceWriter{dst: &dst, remaining: 1024}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = w.Write(make([]byte, 65)) }()
	}
	wg.Wait()
	if dst.Len() != 1024 || !w.overflow || w.err == nil {
		t.Fatal("ASSERT_TRACE_CAP: FAIL concurrent")
	}
	t.Log("ASSERT_TRACE_CAP: PASS")
}

func TestPrivateSourcePrepareTraceOccupied(t *testing.T) {
	var borrowed bytes.Buffer
	if err := trace.Start(&borrowed); err != nil {
		t.Fatal(err)
	}
	defer trace.Stop()
	fd, path := privateSourcePrepareTraceFD(t, syscall.O_WRONLY, 0600)
	t.Setenv(privatePrepareTraceFDEnv, strconv.Itoa(fd))
	var out, errOut bytes.Buffer
	if runPrivateSourcePrepareDescribeTraced([]string{"source-prepare-describe"}, &out, &errOut) == 0 || out.Len() != 0 || errOut.Len() != 0 || !trace.IsEnabled() {
		t.Fatal("ASSERT_TRACE_BORROWED: FAIL")
	}
	privateSourcePrepareTraceClosed(t, fd)
	info, err := os.Stat(path)
	if err != nil || info.Size() != 0 {
		t.Fatal("ASSERT_TRACE_BORROWED: FAIL output")
	}
	t.Log("ASSERT_TRACE_BORROWED: PASS")
}
