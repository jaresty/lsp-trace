//go:build darwin || linux

package sessionruntime

import (
	"context"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func privateCaptureOpenFile(ctx context.Context, root *os.File, slot uint8) (io.WriteCloser, error) {
	names := [2]string{"error-01.private.json", "error-02.private.json"}
	if ctx == nil || ctx.Err() != nil || root == nil || slot >= 2 {
		return nil, ErrPrivateDiscovery
	}
	fd, e := unix.Openat(int(root.Fd()), names[slot], unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return nil, ErrPrivateDiscovery
	}
	if ctx.Err() != nil || unix.Fchmod(fd, 0600) != nil {
		if unix.Close(fd) != nil {
			return nil, errPrivateCaptureCloseUncertain
		}
		return nil, ErrPrivateDiscovery
	}
	return os.NewFile(uintptr(fd), "private-error-object"), nil
}

// Preparation has no capture owner yet. Keep functional refusal identity and
// descriptor-close uncertainty independently discoverable, without native text.
// The empty value carries no descriptor and cannot retry or close anything.
type privateCapturePreparationCloseError struct{}

func (privateCapturePreparationCloseError) Error() string {
	return "private discovery enclosure refused"
}
func (privateCapturePreparationCloseError) Unwrap() error { return ErrPrivateDiscovery }
func (privateCapturePreparationCloseError) Is(target error) bool {
	return target == errPrivateCaptureCloseUncertain
}

func privateCaptureDirectoryClose(fd int) bool { return unix.Close(fd) != nil }

func openPrivateCaptureDirectory(ctx context.Context, path string) (*os.File, error) {
	if ctx == nil || ctx.Err() != nil || !privateCapturePathSyntax(path) {
		return nil, ErrPrivateDiscovery
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if e != nil {
		return nil, ErrPrivateDiscovery
	}
	for at := 1; at < len(path); {
		end := at
		for end < len(path) && path[end] != '/' {
			end++
		}
		if ctx.Err() != nil {
			if privateCaptureDirectoryClose(fd) {
				return nil, privateCapturePreparationCloseError{}
			}
			return nil, ErrPrivateDiscovery
		}
		next, err := unix.Openat(fd, path[at:end], unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		parentUncertain := privateCaptureDirectoryClose(fd)
		if err != nil || parentUncertain {
			childUncertain := false
			if err == nil {
				// next is owned even when closing its parent was uncertain. Each
				// acquisition gets exactly one attempt, never an EINTR retry.
				childUncertain = privateCaptureDirectoryClose(next)
			}
			if parentUncertain || childUncertain {
				return nil, privateCapturePreparationCloseError{}
			}
			return nil, ErrPrivateDiscovery
		}
		fd = next
		var st unix.Stat_t
		if ctx.Err() != nil || unix.Fstat(fd, &st) != nil || (st.Uid != uint32(os.Getuid()) && st.Uid != 0) || st.Mode&0022 != 0 {
			if privateCaptureDirectoryClose(fd) {
				return nil, privateCapturePreparationCloseError{}
			}
			return nil, ErrPrivateDiscovery
		}
		if end == len(path) && (st.Uid != uint32(os.Getuid()) || st.Mode&07777 != 0700) {
			if privateCaptureDirectoryClose(fd) {
				return nil, privateCapturePreparationCloseError{}
			}
			return nil, ErrPrivateDiscovery
		}
		at = end + 1
	}
	if ctx.Err() != nil {
		if privateCaptureDirectoryClose(fd) {
			return nil, privateCapturePreparationCloseError{}
		}
		return nil, ErrPrivateDiscovery
	}
	return os.NewFile(uintptr(fd), "private-capture-directory"), nil
}
