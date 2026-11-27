//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// One fixed inode is both flock owner and checksummed persistent reservation
// record. A torn update is unavailable, never interpreted as zero debt. Neither
// process death nor an empty directory scan authorizes refund of prior debt.
type storageReservationState struct {
	mu       sync.Mutex
	root     *Root
	file     *os.File
	record   storageRecord
	upper    uint64
	finished bool
}

func storageLock(ctx context.Context, f *os.File) error {
	if ctx == nil {
		return ErrStorageIntegrity
	}
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrStorageIntegrity, err)
		}
		e := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if e == nil {
			return nil
		}
		if e != unix.EWOULDBLOCK && e != unix.EAGAIN {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}
func (s *storageReservationState) close() error {
	return errors.Join(unix.Flock(int(s.file.Fd()), unix.LOCK_UN), s.file.Close(), s.root.Close())
}
func storageDecode(f *os.File) (storageRecord, error) {
	var r storageRecord
	info, e := f.Stat()
	if e != nil || info.Size() != storageLedgerBytes {
		return r, ErrStorageIntegrity
	}
	b := make([]byte, storageLedgerBytes)
	if _, e = f.ReadAt(b, 0); e != nil {
		return r, e
	}
	payload := bytes.TrimRight(b[64:], "\x00")
	sum := sha256.Sum256(payload)
	if string(b[:64]) != hex.EncodeToString(sum[:]) {
		return r, ErrStorageIntegrity
	}
	d := json.NewDecoder(bytes.NewReader(payload))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || r.Schema != "lsp-trace.storage-content-quota.v1" || r.Limit < 128<<10 || r.Limit > 1<<50 || r.Debt > r.Limit || r.Retired > r.Debt {
		return r, ErrStorageIntegrity
	}
	return r, nil
}
func storageEncode(s *storageReservationState) error {
	payload, e := json.Marshal(s.record)
	if e != nil || len(payload) > storageLedgerBytes-64 {
		return ErrStorageIntegrity
	}
	b := make([]byte, storageLedgerBytes)
	sum := sha256.Sum256(payload)
	copy(b, hex.EncodeToString(sum[:]))
	copy(b[64:], payload)
	n, e := s.file.WriteAt(b, 0)
	if n != len(b) {
		e = errors.Join(e, io.ErrShortWrite)
	}
	return errors.Join(e, s.file.Sync())
}
func storageOpen(ctx context.Context, path string) (*storageReservationState, error) {
	root, e := OpenRoot(path)
	if e != nil {
		return nil, e
	}
	if e = root.ValidatePrivate(); e != nil {
		root.Close()
		return nil, e
	}
	fd, e := unix.Openat(int(root.file.Fd()), storageLedgerName, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if e != nil {
		root.Close()
		return nil, e
	}
	s := &storageReservationState{root: root, file: os.NewFile(uintptr(fd), storageLedgerName)}
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Nlink != 1 || st.Uid != uint32(os.Geteuid()) {
		s.close()
		return nil, ErrStorageIntegrity
	}
	if e = storageLock(ctx, s.file); e != nil {
		s.close()
		return nil, e
	}
	s.record, e = storageDecode(s.file)
	if e != nil {
		s.close()
		return nil, e
	}
	return s, nil
}

// Discovery checks canonical ancestry before legacy fallback. Stable ancestor
// aliases share the configured ledger; configured writes require the current
// names to match the pinned directory. Genuinely unconfigured roots retain the
// legacy descriptor-only contract, including pathname replacement.
func storageFind(root *Root) (string, error) {
	if root == nil {
		return "", ErrStorageIntegrity
	}
	p, e := filepath.EvalSymlinks(root.path)
	if e != nil {
		return "", errors.Join(ErrStorageIntegrity, e)
	}
	if !root.storageExpected && !storageNamespaceExpected(root.path) && !storageNamespaceExpected(p) {
		return "", nil
	}
	if root.ValidatePrivate() != nil {
		return "", ErrStorageIntegrity
	}
	info, e := os.Lstat(root.path)
	if e != nil || !os.SameFile(info, root.info) {
		return "", ErrStorageIntegrity
	}
	canonicalInfo, e := os.Lstat(p)
	if e != nil || !os.SameFile(canonicalInfo, root.info) {
		return "", ErrStorageIntegrity
	}
	for n := 0; n < 128; n++ {
		_, e = os.Lstat(filepath.Join(p, storageLedgerName))
		if e == nil {
			return p, nil
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		if canonicalFinalComponent(filepath.Base(p)) == StorageDirectoryName {
			return "", ErrStorageIntegrity
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", ErrStorageIntegrity
		}
		p = parent
	}
	return "", ErrStorageIntegrity
}

// scanStorage uses descriptor-relative no-follow traversal and bounded batches.
// It reads metadata only, not arbitrary orphan bodies; content custody remains
// the responsibility of existing exact readers. No evidence is deleted.
func scanStorage(ctx context.Context, root *Root) (StorageStatus, error) {
	var out StorageStatus
	var walk func(int, int) error
	walk = func(parent, depth int) (resultErr error) {
		if depth > 32 || ctx == nil || ctx.Err() != nil {
			return ErrStorageIntegrity
		}
		fd, e := unix.Openat(parent, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return e
		}
		f := os.NewFile(uintptr(fd), "quota-scan")
		defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
		for {
			entries, e := f.ReadDir(128)
			if e != nil && e != io.EOF {
				return e
			}
			for _, entry := range entries {
				out.Entries++
				if out.Entries > 65536 || ctx.Err() != nil {
					return ErrStorageIntegrity
				}
				var st unix.Stat_t
				if e := unix.Fstatat(fd, entry.Name(), &st, unix.AT_SYMLINK_NOFOLLOW); e != nil {
					return e
				}
				if st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0 {
					return ErrStorageIntegrity
				}
				switch st.Mode & unix.S_IFMT {
				case unix.S_IFDIR:
					child, e := unix.Openat(fd, entry.Name(), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
					if e != nil {
						return e
					}
					var actual unix.Stat_t
					if unix.Fstat(child, &actual) != nil || st.Dev != actual.Dev || st.Ino != actual.Ino {
						unix.Close(child)
						return ErrStorageIntegrity
					}
					e = walk(child, depth+1)
					e = errors.Join(e, unix.Close(child))
					if e != nil {
						return e
					}
				case unix.S_IFREG:
					if st.Nlink != 1 || st.Size < 0 || st.Blocks < 0 || uint64(st.Size) > 1<<50 || uint64(st.Blocks) > 1<<41 {
						return ErrStorageIntegrity
					}
					out.Files++
					out.FileBytes += uint64(st.Size)
					out.AllocatedBytes += uint64(st.Blocks) * 512
					if out.FileBytes > 1<<50 || out.AllocatedBytes > 1<<50 {
						return ErrStorageIntegrity
					}
				default:
					return ErrStorageIntegrity
				}
			}
			if e == io.EOF {
				return nil
			}
		}
	}
	e := walk(int(root.file.Fd()), 0)
	return out, e
}
func storageView(r storageRecord, v StorageStatus) StorageStatus {
	v.Schema = r.Schema
	v.Limit = r.Limit
	v.ControlReserve = StorageControlBytes
	v.ReservedBytes = r.Debt
	v.RetirementReservedBytes = r.Retired
	v.UncertainReservedBytes = r.Debt - r.Retired
	v.PeakReservedBytes = r.PeakReserved
	v.SampledPeakFileBytes = r.PeakFile
	v.SampledPeakAllocatedBytes = r.PeakAllocated
	if v.FileBytes > v.SampledPeakFileBytes {
		v.SampledPeakFileBytes = v.FileBytes
	}
	if v.AllocatedBytes > v.SampledPeakAllocatedBytes {
		v.SampledPeakAllocatedBytes = v.AllocatedBytes
	}
	v.Reservations = r.Reservations
	if v.FileBytes+r.Debt+StorageControlBytes <= r.Limit {
		v.AvailableBytes = r.Limit - v.FileBytes - r.Debt - StorageControlBytes
	}
	return v
}
func ReadStorageStatus(ctx context.Context, root *Root) (*StorageStatus, error) {
	p, e := storageFind(root)
	if e != nil || p == "" {
		return nil, e
	}
	s, e := storageOpen(ctx, p)
	if e != nil {
		return nil, e
	}
	v, e := scanStorage(ctx, s.root)
	e = errors.Join(e, s.close())
	if e != nil {
		return nil, e
	}
	v = storageView(s.record, v)
	return &v, nil
}

// ConfigureStorage accepts only a genuinely empty new store or an identical
// existing policy. Historical populated stores cannot be silently upgraded.
func ConfigureStorage(ctx context.Context, root *Root, limit uint64) error {
	if root == nil || root.ValidatePrivate() != nil || limit < 128<<10 || limit > 1<<50 || storageNamespaceExpected(filepath.Dir(root.path)) {
		return ErrStorageIntegrity
	}
	if _, e := os.Lstat(filepath.Join(root.path, storageLedgerName)); e == nil {
		s, e := storageOpen(ctx, root.path)
		if e != nil {
			return e
		}
		same := s.record.Limit == limit
		_, se := scanStorage(ctx, s.root)
		ce := s.close()
		if !same {
			return errors.Join(ErrStorageIntegrity, se, ce)
		}
		return errors.Join(se, ce)
	} else if !os.IsNotExist(e) {
		return e
	}
	v, e := scanStorage(ctx, root)
	if e != nil || v.Entries != 0 {
		return errors.Join(ErrStorageIntegrity, e)
	}
	fd, e := unix.Openat(int(root.file.Fd()), storageLedgerName, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), storageLedgerName)
	defer f.Close()
	if e = storageLock(ctx, f); e != nil {
		return e
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	// A concurrent initializer can observe only incomplete (refused) or complete
	// configuration. Never repair/delete an incomplete exclusive creation.
	v, e = scanStorage(ctx, root)
	if e != nil || v.Entries != 1 {
		return errors.Join(ErrStorageIntegrity, e)
	}
	s := &storageReservationState{root: root, file: f, record: storageRecord{Schema: "lsp-trace.storage-content-quota.v1", Limit: limit, PeakFile: storageLedgerBytes}}
	e = storageEncode(s)
	_, se := root.SyncDirectory()
	root.storageExpected = true
	return errors.Join(e, se)
}
func ReserveStorage(ctx context.Context, root *Root, upper uint64) (*StorageReservation, error) {
	p, e := storageFind(root)
	if e != nil {
		return nil, e
	}
	if p == "" {
		return &StorageReservation{}, nil
	}
	s, e := storageOpen(ctx, p)
	if e != nil {
		return nil, e
	}
	v, e := scanStorage(ctx, s.root)
	if e != nil {
		s.close()
		return nil, e
	}
	available := storageView(s.record, v).AvailableBytes
	if upper > available || v.FileBytes+s.record.Debt+StorageControlBytes > s.record.Limit {
		s.close()
		return nil, ErrStorageCapacity
	}
	s.upper = upper
	s.record.Debt += upper
	s.record.Reservations++
	if s.record.Reservations == 0 {
		s.close()
		return nil, ErrStorageIntegrity
	}
	peak := v.FileBytes + s.record.Debt
	if peak > s.record.PeakReserved {
		s.record.PeakReserved = peak
	}
	if v.FileBytes > s.record.PeakFile {
		s.record.PeakFile = v.FileBytes
	}
	if v.AllocatedBytes > s.record.PeakAllocated {
		s.record.PeakAllocated = v.AllocatedBytes
	}
	if e = storageEncode(s); e != nil {
		s.close()
		return nil, e
	}
	return &StorageReservation{state: s}, nil
}
func observeStorage(r *StorageReservation, f *os.File) error {
	if r == nil || r.state == nil {
		return nil
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return ErrStorageIntegrity
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	v, e := scanStorage(ctx, s.root)
	if e != nil {
		return e
	}
	if f != nil {
		var st unix.Stat_t
		if unix.Fstat(int(f.Fd()), &st) != nil || st.Size < 0 || st.Blocks < 0 {
			return ErrStorageIntegrity
		}
		if st.Nlink == 0 {
			v.FileBytes += uint64(st.Size)
			v.AllocatedBytes += uint64(st.Blocks) * 512
		}
	}
	if v.FileBytes > s.record.PeakFile {
		s.record.PeakFile = v.FileBytes
	}
	if v.AllocatedBytes > s.record.PeakAllocated {
		s.record.PeakAllocated = v.AllocatedBytes
	}
	if v.FileBytes+StorageControlBytes > s.record.Limit {
		return ErrStorageCapacity
	}
	return nil
}

// Retired inode lifetimes cannot be proved across processes. Reserve before
// unlink/replacement and keep the charge; a namespace scan is not close proof.
func retainStorageRetirement(ctx context.Context, root *Root, n uint64) error {
	if n == 0 {
		return nil
	}
	r, e := ReserveStorage(ctx, root, n)
	if e != nil {
		return e
	}
	if r.state == nil {
		return nil
	}
	r.state.mu.Lock()
	r.state.record.Retired += n
	r.state.mu.Unlock()
	return r.Complete(false)
}
func completeStorage(r *StorageReservation, success bool) error {
	if r == nil || r.state == nil {
		return nil
	}
	s := r.state
	e := observeStorage(r, nil)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return ErrStorageIntegrity
	}
	s.finished = true
	if success && e == nil {
		s.record.Debt -= s.upper
	}
	e = errors.Join(e, storageEncode(s))
	return errors.Join(e, s.close())
}
