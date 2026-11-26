package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultStorageBytes uint64 = 2 << 30
const StorageControlBytes uint64 = 64 << 10
const storageLedgerBytes = 4096
const storageLedgerName = ".storage-quota-v1"
const StorageDirectoryName = "durable-quota-v1"

var ErrStorageCapacity = errors.New("owned-store content-length capacity exhausted")
var ErrStorageIntegrity = errors.New("owned-store quota integrity/configuration unavailable")

// StorageStatus is file-content accounting, NOT allocated-block enforcement.
// AllocatedBytes is a separate sum of st_blocks*512, not exclusive physical use
// on clone/snapshot filesystems. Debt is never inferred free after interruption.
type StorageStatus struct {
	ReservedBytes, RetirementReservedBytes                                          uint64
	Schema                                                                          string
	Limit, ControlReserve, FileBytes, AllocatedBytes, Files, Entries                uint64
	UncertainReservedBytes, AvailableBytes, PeakReservedBytes, SampledPeakFileBytes uint64
	SampledPeakAllocatedBytes, Reservations                                         uint64
}
type storageRecord struct {
	Retired                                                          uint64
	Schema                                                           string
	Limit, Debt, PeakReserved, PeakFile, PeakAllocated, Reservations uint64
}
type StorageReservation struct{ state *storageReservationState }

func (r *StorageReservation) Complete(success bool) error       { return completeStorage(r, success) }
func (r *StorageReservation) ObserveAnonymous(f *os.File) error { return observeStorage(r, f) }

// RefuseUnaccountedStorage is for write owners not qualified for quota stores.
func RefuseUnaccountedStorage(path string) error {
	if storageNamespaceExpected(path) {
		return ErrStorageIntegrity
	}
	return nil
}

// StorageReservedSelector protects the stable lock/ledger namespace from the
// generic publisher, CAS-temporary and deletion APIs.
func StorageReservedSelector(selector string) bool {
	for _, part := range strings.Split(filepath.ToSlash(selector), "/") {
		part = canonicalFinalComponent(part)
		if part == storageLedgerName || strings.HasPrefix(part, storageLedgerName+".") {
			return true
		}
	}
	return false
}
func storageNamespaceExpected(path string) bool {
	for n := 0; n < 128; n++ {
		if _, e := os.Lstat(filepath.Join(path, storageLedgerName)); e == nil || !os.IsNotExist(e) {
			return true
		}
		if canonicalFinalComponent(filepath.Base(path)) == StorageDirectoryName {
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
	return true
}
func reserveRootStorage(root *Root, upper uint64) (*StorageReservation, error) {
	if root == nil {
		return nil, ErrStorageIntegrity
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return ReserveStorage(ctx, root, upper)
}
