//go:build !windows

package main

import "os"

const platformDirectoryDurability = directoryDurabilityChecked

func platformSyncPublicationDirectory(dir *os.File) error {
	return dir.Sync()
}
