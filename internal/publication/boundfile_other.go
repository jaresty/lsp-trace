//go:build !linux && !darwin

package publication

import (
	"errors"
	"os"
)

var errExactFDUnsupported = errors.New("exact-fd publication primitive unsupported")

func validPublishedMetadata(rootInfo, finalInfo os.FileInfo) bool { return false }

func publishExactFD(root *Root, selector string, raw []byte, verify func([]byte) error) (bool, string, error) {
	return false, "", errExactFDUnsupported
}
