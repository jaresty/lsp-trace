//go:build !darwin && !linux

package adr0011lifecycle

import (
	"errors"
	"os"
)

const supportedLock = false

func directoryNumbers(os.FileInfo) (uint64, uint64, error) { return 0, 0, errUnsupported }
func directoryIdentity(os.FileInfo) (string, error)        { return "", errUnsupported }

var errUnsupported = errors.New("unsupported OS advisory lock platform")

func openLockNoFollow(string) (*os.File, error) { return nil, errUnsupported }
func singleLink(os.FileInfo) bool               { return false }
func tryExclusive(*os.File) error               { return errUnsupported }
func unlockExclusive(*os.File) error            { return errUnsupported }
func lockBusy(error) bool                       { return false }
