//go:build !linux && !darwin

package publication

import "os"

func removeEmptyChildDirectory(*os.File, string) error { return ErrObservedRootUnsupported }
