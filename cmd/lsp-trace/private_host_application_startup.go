//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// This is metadata transport, not a role frame, caller approval or grant.
// POSIX guarantees PIPE_BUF >=512: a single nonblocking write of <=512 bytes
// either fits atomically or fails. We never poll, retry or wait for the reader.
const applicationStartupPacketLimit = 512

type applicationStartupPacket struct {
	Schema, Event, ApplicationSHA256, RunID, UnitID, ActivationSHA256 string
	Activation                                                        int
}
type applicationStartupNotifier struct {
	file      *os.File
	attempted bool
}

// No pathname is opened. Zero links and the held descriptor's filesystem
// distinguish an anonymous pipe even from an unlinked named FIFO; exact
// write-only mode excludes read ends and sockets.
// Ownership transfers only for this explicit flag, never standard descriptors.
func openApplicationStartupNotifier(value string) (*applicationStartupNotifier, error) {
	fd, e := strconv.Atoi(value)
	if e != nil || fd < 3 || strconv.Itoa(fd) != value {
		return nil, product.ErrInvalid
	}
	flags, e := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if e != nil {
		return nil, e
	}
	var st unix.Stat_t
	if e = unix.Fstat(fd, &st); e != nil || st.Mode&unix.S_IFMT != unix.S_IFIFO || st.Nlink != 0 || flags&unix.O_ACCMODE != unix.O_WRONLY {
		return nil, product.ErrInvalid
	}
	var fs unix.Statfs_t
	fsErr := unix.Fstatfs(fd, &fs)
	// Darwin pipes have no vnode/filesystem (EINVAL from fstatfs despite a
	// successful fstat, device0). Linux identifies anonymous pipefs directly.
	if runtime.GOOS == "darwin" && (fsErr != unix.EINVAL || st.Dev != 0) || runtime.GOOS == "linux" && (fsErr != nil || uint64(fs.Type) != 0x50495045) {
		return nil, product.ErrInvalid
	}
	// Prevent acquisition grandchildren from writing or holding this writer open.
	if _, e = unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.FD_CLOEXEC); e != nil {
		_ = unix.Close(fd)
		return nil, e
	}
	if e = unix.SetNonblock(fd, true); e != nil {
		_ = unix.Close(fd)
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "application-startup-notify")
	if f == nil {
		_ = unix.Close(fd)
		return nil, product.ErrInvalid
	}
	return &applicationStartupNotifier{file: f}, nil
}
func (n *applicationStartupNotifier) close() error {
	if n == nil || n.file == nil {
		return nil
	}
	f := n.file
	n.file = nil
	return f.Close()
}
func (n *applicationStartupNotifier) emit(ctx context.Context, p applicationStartupPacket) (err error) {
	if n == nil {
		return nil
	}
	if n.attempted {
		return nil
	} // only the first activation in this run notifies
	n.attempted = true
	defer func() { err = errors.Join(err, n.close()) }()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if n.file == nil || p.Schema != "private-application-startup.v1" || p.Event != "ACTIVATION_READY" || !validApplicationDigest(p.ApplicationSHA256) || !validApplicationDigest(p.RunID) || !validApplicationDigest(p.UnitID) || !validApplicationDigest(p.ActivationSHA256) || p.Activation < 1 || p.Activation > 16 {
		return product.ErrInvalid
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > applicationStartupPacketLimit {
		return product.ErrInvalid
	}
	// Fd() on an os.File can restore blocking mode; reassert NONBLOCK after it.
	fd := int(n.file.Fd())
	if e = unix.SetNonblock(fd, true); e != nil {
		return e
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	wrote, e := unix.Write(fd, raw)
	if e != nil {
		return e
	}
	if wrote != len(raw) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}
func applicationActivationPacket(app, run, unit string, ordinal int, activation applicationActivation) applicationStartupPacket {
	raw, _ := json.Marshal(activation)
	return applicationStartupPacket{"private-application-startup.v1", "ACTIVATION_READY", app, run, unit, pilot.Digest(raw), ordinal}
}
