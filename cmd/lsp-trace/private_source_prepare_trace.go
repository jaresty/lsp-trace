//go:build darwin || linux

package main

import (
	"context"
	"io"
	"os"
	"runtime/trace"
	"strconv"
	"sync"
	"syscall"
)

const privatePrepareTraceFDEnv = "LSP_TRACE_PRIVATE_PREPARE_TRACE_FD"
const privatePrepareTraceBytes = 8 << 20

// This opt-in allowance is caller-funded, separate from native source and
// diagnostic budgets. It caps file bytes, not runtime buffers or physical IO
// duration. The original deadline and caller's process guard still apply.
type privatePrepareTraceWriter struct {
	mu        sync.Mutex
	dst       io.Writer
	remaining int
	overflow  bool
	err       error
}

func (w *privatePrepareTraceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	requested := len(p)
	if requested > w.remaining {
		w.overflow = true
		p = p[:w.remaining]
	}
	n := 0
	var err error
	if len(p) != 0 {
		n, err = w.dst.Write(p)
	}
	// Charge the attempted prefix, not just delivered bytes: short/error writes
	// are terminal and can never buy another attempt at the remaining quota.
	w.remaining -= len(p)
	if err == nil && (n != len(p) || w.overflow) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
	}
	return n, err
}

// Only the fixed application's known writer wrapper is unwrapped. No generic
// descriptor-observer interface, pathname or arbitrary writer method is used.
func privatePrepareTraceOutputFile(w io.Writer) *os.File {
	if full, ok := w.(applicationFullWriter); ok {
		w = full.Writer
	}
	f, _ := w.(*os.File)
	return f
}

func privatePrepareTraceFile(raw string, stdout, stderr io.Writer) *os.File {
	fd, err := strconv.Atoi(raw)
	if err != nil || fd < 3 || strconv.Itoa(fd) != raw {
		return nil
	}
	// Ownership starts here. Even a subsequently refused descriptor is closed
	// once; descriptors 0..2 and noncanonical spellings are never taken.
	syscall.CloseOnExec(fd)
	f := os.NewFile(uintptr(fd), "private-prepare-trace")
	if f == nil {
		return nil
	}
	valid := false
	defer func() {
		if !valid {
			_ = f.Close()
		}
	}()
	fdflags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if errno != 0 || fdflags&syscall.FD_CLOEXEC == 0 {
		return nil
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
	if errno != 0 || flags&syscall.O_ACCMODE == syscall.O_RDONLY || flags&syscall.O_APPEND != 0 {
		return nil
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Size() != 0 {
		return nil
	}
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil || offset != 0 {
		return nil
	}
	for _, other := range []*os.File{os.Stdin, os.Stdout, os.Stderr, privatePrepareTraceOutputFile(stdout), privatePrepareTraceOutputFile(stderr)} {
		if other == nil {
			continue
		}
		otherInfo, err := other.Stat()
		if err != nil || os.SameFile(info, otherInfo) {
			return nil
		}
	}
	valid = true
	return f
}

func runPrivateSourcePrepareDescribeTraced(args []string, stdout, stderr io.Writer) (code int) {
	raw, enabled := os.LookupEnv(privatePrepareTraceFDEnv)
	if !enabled {
		return runPrivateSourcePrepareDescribe(args, stdout, stderr)
	}
	f := privatePrepareTraceFile(raw, stdout, stderr)
	if f == nil {
		return 1
	}
	defer f.Close()
	writer := &privatePrepareTraceWriter{dst: f, remaining: privatePrepareTraceBytes}
	if trace.Start(writer) != nil {
		return 1
	}
	// Stop must finish before owned Close and before main's os.Exit. Never stop
	// an existing trace after Start refuses. Flush failure cannot rewrite the
	// native result; the caller must parse the trace and reject incomplete output.
	defer trace.Stop()
	trace.Log(context.Background(), "private.prepare.cli", "TRACE_START")
	code = runPrivateSourcePrepareDescribe(args, stdout, stderr)
	if code == 0 {
		trace.Log(context.Background(), "private.prepare.cli", "TRACE_RETURN_ZERO")
	} else {
		trace.Log(context.Background(), "private.prepare.cli", "TRACE_RETURN_NONZERO")
	}
	return code
}
