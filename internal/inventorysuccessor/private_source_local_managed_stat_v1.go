//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"time"
)

// Exactly the before/after operations required by the existing streaming
// verifier. Each managed pin prepays these two additional logical metadata calls
// in its lifecycle quote; no attempt, including EINTR/error, creates retry credit.
const managedAdmissionMetadataCallsV1 uint64 = 2

type managedAdmissionFileV1 struct {
	privateLocalFileV1
	ctx        context.Context
	live       func() bool
	io         privateLocalIOV1
	kind, name string
	remaining  uint64
}

func newManagedAdmissionFileV1(ctx context.Context, live func() bool, owner privateLocalIOV1, kind, name string, f privateLocalFileV1) privateLocalFileV1 {
	return &managedAdmissionFileV1{privateLocalFileV1: f, ctx: ctx, live: live, io: owner, kind: kind, name: filepath.Base(name), remaining: managedAdmissionMetadataCallsV1}
}
func (f *managedAdmissionFileV1) Stat() (os.FileInfo, error) {
	if f.ctx == nil || f.ctx.Err() != nil || !f.live() || f.remaining == 0 {
		return nil, ErrPrivateSourceLocalV1
	}
	f.remaining-- // debit BEFORE Control/the underlying attempt, even on errors
	if observer, ok := f.io.(interface{ observeEffect(string) error }); ok {
		if e := observer.observeEffect("managed-metadata-debit:" + f.kind); e != nil {
			return nil, e
		}
	}
	raw, ok := f.privateLocalFileV1.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil, ErrPrivateSourceLocalV1
	}
	conn, e := raw.SyscallConn()
	if e != nil {
		return nil, e
	}
	var st syscall.Stat_t
	var statErr error
	e = conn.Control(func(fd uintptr) {
		if controlled, ok := f.io.(interface {
			managedPinFstat(string, uintptr, *syscall.Stat_t) error
		}); ok {
			statErr = controlled.managedPinFstat(f.kind, fd, &st)
		} else {
			statErr = singleLocalDirectoryFstatV1(fd, &st)
		}
	}) // holds the actual FD lifetime across ONE raw Fstat; never os.File.Stat
	if e != nil || statErr != nil {
		return nil, errors.Join(e, statErr)
	}
	return managedAdmissionInfoV1{f.name, &st}, nil
}

// os.SameFile accepts the standard library's private FileInfo implementation,
// not this raw-stat view. Compare actual Stat_t identity AND custody metadata.
func (*managedAdmissionFileV1) sameManagedPinMetadata(before, after os.FileInfo) bool {
	a, aok := before.(managedAdmissionInfoV1)
	b, bok := after.(managedAdmissionInfoV1)
	return aok && bok && sameManagedFileV1(a.stat, b.stat)
}

type managedAdmissionInfoV1 struct {
	name string
	stat *syscall.Stat_t
}

func (i managedAdmissionInfoV1) Name() string { return i.name }
func (i managedAdmissionInfoV1) Size() int64  { return i.stat.Size }
func (i managedAdmissionInfoV1) Mode() os.FileMode {
	mode := os.FileMode(i.stat.Mode & 0777)
	if i.stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		mode |= os.ModeIrregular
	}
	return mode
}
func (i managedAdmissionInfoV1) ModTime() time.Time {
	v := reflect.ValueOf(i.stat).Elem()
	m := v.FieldByName("Mtimespec")
	if !m.IsValid() {
		m = v.FieldByName("Mtim")
	}
	return time.Unix(m.FieldByName("Sec").Int(), m.FieldByName("Nsec").Int())
}
func (i managedAdmissionInfoV1) IsDir() bool { return i.stat.Mode&syscall.S_IFMT == syscall.S_IFDIR }
func (i managedAdmissionInfoV1) Sys() any    { return i.stat }
