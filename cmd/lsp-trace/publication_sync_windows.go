//go:build windows

package main

import "os"

const platformDirectoryDurability = directoryDurabilityUnavailable

func platformSyncPublicationDirectory(_ *os.File) error {
	return errDirectorySyncUnavailable
}

func platformInstallNoReplace(_ *os.Root, _, _ string) error {
	return errDirectorySyncUnavailable
}
