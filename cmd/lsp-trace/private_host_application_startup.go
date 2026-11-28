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

// Descriptor metadata is kept separate from acquisition so both platform rules
// can be tested without opening descriptors or substituting production syscalls.
type applicationStartupDescriptorMetadata struct {
	flags                    int
	mode                     uint32
	nlink, dev, fsType       uint64
	flagsErr, statErr, fsErr error
}

// No pathname is opened. Linux pipefs has one link; Darwin anonymous pipes
// have zero links and no vnode/filesystem. Filesystem identity also excludes
// unlinked named FIFOs; exact write-only mode excludes read ends and O_RDWR.
func classifyApplicationStartupDescriptor(goos string, fd int, m applicationStartupDescriptorMetadata) error {
	if fd < 3 {
		return product.ErrInvalid
	}
	if m.flagsErr != nil {
		return m.flagsErr
	}
	if m.statErr != nil || m.mode&unix.S_IFMT != unix.S_IFIFO || m.flags&unix.O_ACCMODE != unix.O_WRONLY {
		return product.ErrInvalid
	}
	switch goos {
	case "linux":
		if m.nlink == 1 && m.fsErr == nil && m.fsType == 0x50495045 {
			return nil
		}
	case "darwin":
		if m.nlink == 0 && m.dev == 0 && m.fsErr == unix.EINVAL {
			return nil
		}
	}
	return product.ErrInvalid
}

func readApplicationStartupDescriptor(fd int) applicationStartupDescriptorMetadata {
	var m applicationStartupDescriptorMetadata
	m.flags, m.flagsErr = unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if m.flagsErr != nil {
		return m
	}
	var st unix.Stat_t
	m.statErr = unix.Fstat(fd, &st)
	if m.statErr != nil {
		return m
	}
	m.mode, m.nlink, m.dev = uint32(st.Mode), uint64(st.Nlink), uint64(st.Dev)
	var fs unix.Statfs_t
	m.fsErr = unix.Fstatfs(fd, &fs)
	m.fsType = uint64(fs.Type)
	return m
}

// Malformed/standard descriptors are never owned. Classification failures leave
// the caller's descriptor open; setup failures consume it (the existing contract).
func openApplicationStartupNotifier(value string) (*applicationStartupNotifier, error) {
	fd, e := strconv.Atoi(value)
	if e != nil || fd < 3 || strconv.Itoa(fd) != value {
		return nil, product.ErrInvalid
	}
	return openApplicationStartupNotifierFD(fd, false, nativeApplicationStartupDescriptorOps())
}

// The caller transfers an exclusively owned raw descriptor, not an os.File's
// borrowed Fd(). Every fd >= 3 is consumed: closed on rejection or transferred
// to the notifier on success. Invalid/standard descriptors are never acquired.
func openOwnedApplicationStartupNotifier(fd int) (*applicationStartupNotifier, error) {
	return openApplicationStartupNotifierFD(fd, true, nativeApplicationStartupDescriptorOps())
}

// Per-call operations permit pure failure-stage custody tests. Both production
// entry points always supply real operations; no mutable global syscall hooks.
type applicationStartupDescriptorOps struct {
	metadata func(int) applicationStartupDescriptorMetadata
	cloexec  func(int) error
	nonblock func(int) error
	newFile  func(int) *os.File
	close    func(int) error
}

func nativeApplicationStartupDescriptorOps() applicationStartupDescriptorOps {
	return applicationStartupDescriptorOps{
		metadata: readApplicationStartupDescriptor,
		cloexec: func(fd int) error {
			_, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.FD_CLOEXEC)
			return err
		},
		nonblock: func(fd int) error { return unix.SetNonblock(fd, true) },
		newFile:  func(fd int) *os.File { return os.NewFile(uintptr(fd), "application-startup-notify") },
		close:    unix.Close,
	}
}

func openApplicationStartupNotifierFD(fd int, owned bool, ops applicationStartupDescriptorOps) (*applicationStartupNotifier, error) {
	if fd < 3 {
		return nil, product.ErrInvalid
	}
	closeOnReturn := owned
	defer func() {
		if closeOnReturn {
			_ = ops.close(fd) // exactly one attempt; never retry a possibly reused FD
		}
	}()
	if err := classifyApplicationStartupDescriptor(runtime.GOOS, fd, ops.metadata(fd)); err != nil {
		return nil, err
	}
	// Preserve public late-failure custody while the owned entry also consumes
	// early failures. Prevent grandchildren from holding or writing this pipe.
	closeOnReturn = true
	if err := ops.cloexec(fd); err != nil {
		return nil, err
	}
	if err := ops.nonblock(fd); err != nil {
		return nil, err
	}
	f := ops.newFile(fd)
	if f == nil {
		return nil, product.ErrInvalid
	}
	closeOnReturn = false // the notifier is now the only owner
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
