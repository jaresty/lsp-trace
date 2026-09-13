//go:build linux || darwin

package publication

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var errExactFDUnsupported = errors.New("exact-fd publication primitive unsupported")

func syncBoundDirectory(fd int) error {
	err := unix.Fsync(fd)
	if testHookBoundFileDirectorySync != nil {
		err = errors.Join(err, testHookBoundFileDirectorySync())
	}
	return err
}

func closeBoundSource(f *os.File) error {
	err := f.Close()
	if testHookBoundFileSourceClose != nil {
		err = errors.Join(err, testHookBoundFileSourceClose())
	}
	return err
}

func closeBoundRootFD(fd int) error {
	err := unix.Close(fd)
	if testHookBoundFileRootClose != nil {
		err = errors.Join(err, testHookBoundFileRootClose())
	}
	return err
}

func boundParentFD(root *Root, selector string) (int, string, error) {
	parts, err := selectorPathParts(runtime.GOOS, selector)
	if err != nil {
		return -1, "", err
	}
	fd, err := unix.Dup(int(root.file.Fd()))
	if err != nil {
		return -1, "", err
	}
	for _, part := range parts[:len(parts)-1] {
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if openErr != nil {
			return -1, "", openErr
		}
		fd = next
	}
	name := parts[len(parts)-1]
	if name == "" || strings.ContainsRune(name, '/') {
		unix.Close(fd)
		return -1, "", errors.New("unsafe final name")
	}
	return fd, name, nil
}

func validPublishedMetadata(rootInfo, finalInfo os.FileInfo) bool {
	rootStat, rootOK := rootInfo.Sys().(*syscall.Stat_t)
	finalStat, finalOK := finalInfo.Sys().(*syscall.Stat_t)
	return rootOK && finalOK && rootStat.Uid == finalStat.Uid && finalStat.Nlink == 1
}

func prepareSource(f *os.File, raw []byte, verify func([]byte) error) error {
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if err := writeAll(f, raw); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	got, err := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
	if err != nil || len(got) != len(raw) || !equalBytes(got, raw) {
		return errors.New("source handle reread mismatch")
	}
	if err := verify(got); err != nil {
		return err
	}
	if testHookBoundFileAfterVerify != nil {
		testHookBoundFileAfterVerify()
	}
	if testHookBoundFileBeforePublish != nil {
		testHookBoundFileBeforePublish()
	}
	return nil
}
