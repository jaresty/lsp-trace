//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Only known nonblocking memory readers or owned OS descriptors are admitted.
// An arbitrary io.Reader cannot promise that Close interrupts Read. In particular,
// a goroutine around ReadAll is not cancellation or a bounded lifecycle.
func readCensusDurableInput(ctx context.Context, input io.Reader) (data []byte, returnErr error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("durable input context unavailable")
	}
	switch r := input.(type) {
	case *bytes.Reader, *bytes.Buffer, *strings.Reader:
		raw, err := io.ReadAll(io.LimitReader(r, (64<<10)+1))
		return raw, errors.Join(err, ctx.Err())
	case *os.File:
		fd, err := unix.Dup(int(r.Fd()))
		if err != nil {
			return nil, err
		}
		unix.CloseOnExec(fd)
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
		if err != nil {
			_ = unix.Close(fd)
			return nil, err
		}
		if err = unix.SetNonblock(fd, true); err != nil {
			_ = unix.Close(fd)
			return nil, err
		}
		owned := os.NewFile(uintptr(fd), "durable-submit-input")
		if owned == nil {
			_ = unix.Close(fd)
			return nil, errors.New("cannot own durable input")
		}
		// Dup shares status flags with stdin; restore them only after the reader
		// and cancellation callback have both finished, then close our descriptor.
		defer func() {
			returnErr = errors.Join(returnErr, unix.SetNonblock(fd, flags&unix.O_NONBLOCK != 0), owned.Close())
		}()
		// Regular files cannot wait for a remote writer and need no poll deadline.
		info, err := owned.Stat()
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			raw, err := io.ReadAll(io.LimitReader(owned, (64<<10)+1))
			return raw, errors.Join(err, ctx.Err())
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("durable input requires deadline")
		}
		if err = owned.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		done := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { defer close(done); _ = owned.SetReadDeadline(time.Now()) })
		raw, readErr := io.ReadAll(io.LimitReader(owned, (64<<10)+1))
		// The poller's deadline can fire before the context timer is scheduled.
		// This descriptor has only our context deadline (or its cancellation
		// callback's immediate deadline), so join that notification before
		// classifying the transport failure. Never turn arbitrary IO errors into
		// cancellation, and never accept partial bytes on either failure.
		if errors.Is(readErr, os.ErrDeadlineExceeded) {
			<-ctx.Done()
		}
		if !stop() {
			<-done
		}
		return raw, errors.Join(readErr, ctx.Err())
	default:
		return nil, errors.New("durable input requires an owned file or bounded memory reader")
	}
}
