//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Hosted-only execution: no environment skip is needed because ordinary CI
// runs these witnesses; bounded local verification selects only the pure tests.
func TestApplicationStartupDescriptorHostedMetadata(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "live"
		if cancelled {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			fd := duplicateApplicationStartupWriter(t, w)
			m := readApplicationStartupDescriptor(fd)
			// Log only descriptor fields, never source/readiness payload or paths.
			t.Logf("nlink=%d mode=%#o flags=%#x fstatfs_return=%v fstatfs_type=%#x", m.nlink, m.mode, m.flags, m.fsErr, m.fsType)
			n, err := openOwnedApplicationStartupNotifier(fd)
			if err != nil {
				t.Fatal(err)
			} // consumed even on rejection
			defer n.close()
			if m.flagsErr != nil || m.statErr != nil || classifyApplicationStartupDescriptor(runtime.GOOS, fd, m) != nil {
				t.Fatal("owned pipe metadata was not classified")
			}
			// Use the held raw number: os.File.Fd can restore blocking mode.
			flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
			if err != nil || flags&unix.O_NONBLOCK == 0 {
				t.Fatal("NONBLOCK not set", err)
			}
			fdFlags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
			if err != nil || fdFlags&unix.FD_CLOEXEC == 0 {
				t.Fatal("CLOEXEC not set", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			witness := []byte("controlled-descriptor-witness\n")
			err = notifyPrivateSourceReady(ctx, n, witness)
			if deadlineErr := r.SetReadDeadline(time.Now().Add(2 * time.Second)); deadlineErr != nil {
				t.Fatal(deadlineErr)
			}
			got, readErr := io.ReadAll(r)
			if readErr != nil {
				t.Fatal("expected EOF", readErr)
			}
			if cancelled {
				if !errors.Is(err, context.Canceled) || len(got) != 0 {
					t.Fatal("cancelled pipe must carry zero bytes", err)
				}
			} else if err != nil || !bytes.Equal(got, witness) {
				t.Fatal("controlled pipe witness mismatch", err)
			}
			if n.file != nil {
				t.Fatal("notifier retained its file after emission")
			}
			if err := n.close(); err != nil {
				t.Fatal("idempotent notifier close", err)
			}
			if notifyPrivateSourceReady(context.Background(), n, witness) == nil {
				t.Fatal("repeat notification admitted")
			}
		})
	}
}

func TestApplicationStartupDescriptorHostedFIFORejection(t *testing.T) {
	for _, unlinked := range []bool{false, true} {
		for _, access := range []int{unix.O_WRONLY, unix.O_RDWR} {
			name := "linked-writeonly"
			if unlinked {
				name = "unlinked-writeonly"
			}
			if access == unix.O_RDWR {
				name += "-rdwr"
			}
			t.Run(name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "fifo")
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
				reader, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer unix.Close(reader)
				fd, err := unix.Open(path, access|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
				if err != nil {
					t.Fatal(err)
				}
				if unlinked {
					if err := os.Remove(path); err != nil {
						_ = unix.Close(fd) // not yet transferred
						t.Fatal(err)
					}
				}
				assertApplicationStartupOwnedRejection(t, fd)
			})
		}
	}
}

func assertApplicationStartupOwnedRejection(t *testing.T, fd int) {
	t.Helper()
	n, err := openOwnedApplicationStartupNotifier(fd)
	// No raw close here: the owned opener consumed fd on every return.
	if n != nil {
		defer n.close()
	}
	if err == nil || n != nil {
		t.Fatal("invalid native descriptor admitted")
	}
}

func TestApplicationStartupDescriptorHostedOtherRejection(t *testing.T) {
	t.Run("read-end", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()
		// The duplicate helper's ownership rule also applies to this read end.
		assertApplicationStartupOwnedRejection(t, duplicateApplicationStartupWriter(t, r))
	})
	t.Run("regular-file", func(t *testing.T) {
		f, err := os.OpenFile(filepath.Join(t.TempDir(), "regular"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		assertApplicationStartupOwnedRejection(t, duplicateApplicationStartupWriter(t, f))
	})
	t.Run("invalid-fd", func(t *testing.T) {
		// Outside the native descriptor range on supported Darwin/Linux hosts.
		// Public early rejection retains custody: never close this numeric value.
		n, err := openApplicationStartupNotifier("2147483647")
		if n != nil {
			defer n.close()
		}
		if !errors.Is(err, unix.EBADF) || n != nil {
			t.Fatal("bad descriptor not refused", err)
		}
	})
}

// These two test families perform no descriptor syscalls. Native witnesses above
// are compiled locally but executed only by the ordinary hosted test suite.
func TestApplicationStartupDescriptorOwnership(t *testing.T) {
	for _, owned := range []bool{false, true} {
		name := "public"
		if owned {
			name = "owned"
		}
		for _, stage := range []string{"getfl", "fstat", "classification", "fstatfs", "cloexec", "nonblock", "newfile", "success"} {
			t.Run(name+"-"+stage, func(t *testing.T) {
				var calls []string
				mark := func(fd int, op string) {
					if fd != 19 {
						t.Fatalf("wrong synthetic descriptor: %d", fd)
					}
					calls = append(calls, op)
				}
				// Inert identity token only: no native descriptor, Fd, or Close.
				file := new(os.File)
				ops := applicationStartupDescriptorOps{
					metadata: func(fd int) applicationStartupDescriptorMetadata {
						mark(fd, "metadata")
						m := applicationStartupDescriptorMetadata{flags: unix.O_WRONLY, mode: unix.S_IFIFO, nlink: 1, fsType: 0x50495045}
						if runtime.GOOS == "darwin" {
							m.nlink, m.fsErr = 0, unix.EINVAL
						}
						switch stage {
						case "getfl":
							m.flagsErr = unix.EBADF
						case "fstat":
							m.statErr = unix.EIO
						case "classification":
							m.mode = unix.S_IFREG
						case "fstatfs":
							m.fsErr = unix.EIO
						}
						return m
					},
					cloexec: func(fd int) error {
						mark(fd, "cloexec")
						if stage == "cloexec" {
							return unix.EIO
						}
						return nil
					},
					nonblock: func(fd int) error {
						mark(fd, "nonblock")
						if stage == "nonblock" {
							return unix.EIO
						}
						return nil
					},
					newFile: func(fd int) *os.File {
						mark(fd, "newfile")
						if stage == "newfile" {
							return nil
						}
						return file
					},
					close: func(fd int) error {
						mark(fd, "close")
						return unix.EIO // close error must not cause a second attempt
					},
				}
				n, err := openApplicationStartupNotifierFD(19, owned, ops)
				want := []string{"metadata"}
				early := stage == "getfl" || stage == "fstat" || stage == "classification" || stage == "fstatfs"
				if !early {
					want = append(want, "cloexec")
					if stage != "cloexec" {
						want = append(want, "nonblock")
						if stage != "nonblock" {
							want = append(want, "newfile")
						}
					}
				}
				if stage != "success" && (owned || !early) {
					want = append(want, "close")
				}
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("custody/stage calls: got %v want %v", calls, want)
				}
				if stage == "success" {
					if err != nil || n == nil || n.file != file {
						t.Fatal("ownership not transferred", err)
					}
				} else if err == nil || n != nil {
					t.Fatal("failure must not return a notifier")
				}
				if stage == "getfl" && err != unix.EBADF {
					t.Fatal("F_GETFL error contract changed", err)
				}
			})
		}
		for _, fd := range []int{-1, 0, 1, 2} {
			// Nil operations panic if any query/setup/close occurs.
			if n, err := openApplicationStartupNotifierFD(fd, owned, applicationStartupDescriptorOps{}); n != nil || err == nil {
				t.Fatalf("standard/invalid FD acquired: %d", fd)
			}
		}
	}
	for _, value := range []string{"", "-1", "0", "1", "2", "03", "+3", " 3", "3 ", "3x", "999999999999999999999999999999999"} {
		if n, err := openApplicationStartupNotifier(value); n != nil || err == nil {
			t.Fatalf("noncanonical/standard FD admitted: %q", value)
		}
	}
}

func TestApplicationStartupDescriptorClassification(t *testing.T) {
	linux := applicationStartupDescriptorMetadata{flags: unix.O_WRONLY, mode: unix.S_IFIFO | 0600, nlink: 1, fsType: 0x50495045}
	darwin := applicationStartupDescriptorMetadata{flags: unix.O_WRONLY, mode: unix.S_IFIFO | 0600, fsErr: unix.EINVAL}
	for _, tc := range []struct {
		name, goos string
		fd         int
		base       applicationStartupDescriptorMetadata
		change     func(*applicationStartupDescriptorMetadata)
		valid      bool
	}{
		{"linux-one-link", "linux", 3, linux, nil, true},
		{"linux-zero-links", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.nlink = 0 }, false},
		{"linux-two-links", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.nlink = 2 }, false},
		{"linux-max-links", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.nlink = ^uint64(0) }, false},
		{"linux-wrong-fs", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.fsType = 0x1234 }, false},
		{"linux-fs-einval", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.fsErr = unix.EINVAL }, false},
		{"linux-fs-eio", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.fsErr = unix.EIO }, false},
		{"linux-nonzero-dev", "linux", 3, linux, func(m *applicationStartupDescriptorMetadata) { m.dev = 42 }, true},
		{"darwin-zero-links", "darwin", 3, darwin, nil, true},
		{"darwin-one-link", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.nlink = 1 }, false},
		{"darwin-two-links", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.nlink = 2 }, false},
		{"darwin-device", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.dev = 1 }, false},
		{"darwin-fs-success", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.fsErr = nil }, false},
		{"darwin-fs-eio", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.fsErr = unix.EIO }, false},
		{"darwin-fs-type-ignored-on-einval", "darwin", 3, darwin, func(m *applicationStartupDescriptorMetadata) { m.fsType = 42 }, true},
		{"unsupported-os", "freebsd", 3, linux, nil, false},
		{"negative-fd", "linux", -1, linux, nil, false},
		{"stdin", "linux", 0, linux, nil, false},
		{"stdout", "linux", 1, linux, nil, false},
		{"stderr", "linux", 2, linux, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.base
			if tc.change != nil {
				tc.change(&m)
			}
			if err := classifyApplicationStartupDescriptor(tc.goos, tc.fd, m); (err == nil) != tc.valid {
				t.Fatalf("descriptor admission: got %v, want valid=%v", err, tc.valid)
			}
		})
	}
	for _, platform := range []struct {
		goos string
		base applicationStartupDescriptorMetadata
	}{{"linux", linux}, {"darwin", darwin}} {
		for _, tc := range []struct {
			name   string
			change func(*applicationStartupDescriptorMetadata)
		}{
			{"getfl-error", func(m *applicationStartupDescriptorMetadata) { m.flagsErr = unix.EBADF }},
			{"fstat-error", func(m *applicationStartupDescriptorMetadata) { m.statErr = unix.EIO }},
			{"read-end", func(m *applicationStartupDescriptorMetadata) { m.flags = unix.O_RDONLY }},
			{"read-write", func(m *applicationStartupDescriptorMetadata) { m.flags = unix.O_RDWR }},
			{"regular-file", func(m *applicationStartupDescriptorMetadata) { m.mode = unix.S_IFREG | 0600 }},
			{"socket", func(m *applicationStartupDescriptorMetadata) { m.mode = unix.S_IFSOCK | 0600 }},
		} {
			t.Run(platform.goos+"-"+tc.name, func(t *testing.T) {
				m := platform.base
				tc.change(&m)
				if err := classifyApplicationStartupDescriptor(platform.goos, 3, m); err == nil {
					t.Fatal("invalid metadata admitted")
				}
			})
		}
	}
}
