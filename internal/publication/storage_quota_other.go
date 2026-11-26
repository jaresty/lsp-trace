//go:build !darwin && !linux

package publication

import (
	"context"
	"os"
)

type storageReservationState struct{}

func retainStorageRetirement(ctx context.Context, r *Root, n uint64) error {
	q, e := ReserveStorage(ctx, r, n)
	if e != nil {
		return e
	}
	return q.Complete(false)
}

func ConfigureStorage(context.Context, *Root, uint64) error { return ErrStorageIntegrity }
func ReadStorageStatus(_ context.Context, r *Root) (*StorageStatus, error) {
	if r != nil && !storageNamespaceExpected(r.Path()) {
		return nil, nil
	}
	return nil, ErrStorageIntegrity
}
func ReserveStorage(_ context.Context, r *Root, _ uint64) (*StorageReservation, error) {
	if r != nil && !storageNamespaceExpected(r.Path()) {
		return &StorageReservation{}, nil
	}
	return nil, ErrStorageIntegrity
}
func completeStorage(*StorageReservation, bool) error    { return nil }
func observeStorage(*StorageReservation, *os.File) error { return nil }
