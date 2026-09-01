//go:build windows

package main

import "os"

const platformDirectoryDurability = directoryDurabilityUnavailable

func platformSyncPublicationDirectory(_ *os.File) error {
	return errDirectorySyncUnavailable
}
