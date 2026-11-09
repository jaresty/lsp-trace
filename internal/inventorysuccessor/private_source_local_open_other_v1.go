//go:build !darwin && !linux

package inventorysuccessor

import "os"

func openLocalPinV1(string) (*os.File, error) { return nil, ErrPrivateSourceLocalV1 }
