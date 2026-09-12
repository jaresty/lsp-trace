//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInfoEx struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

func renameDirectoryNoReplace(parent, staging *os.File, _, newName string) error {
	name, err := windows.UTF16FromString(newName)
	if err != nil {
		return err
	}
	name = name[:len(name)-1]
	headerSize := unsafe.Offsetof(fileRenameInfoEx{}.FileName)
	buffer := make([]byte, headerSize+uintptr(len(name))*2)
	info := (*fileRenameInfoEx)(unsafe.Pointer(&buffer[0]))
	// FILE_RENAME_FLAG_REPLACE_IF_EXISTS is deliberately absent.
	info.Flags = 0
	info.RootDirectory = windows.Handle(parent.Fd())
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	return windows.SetFileInformationByHandle(
		windows.Handle(staging.Fd()),
		windows.FileRenameInfoEx,
		&buffer[0],
		uint32(len(buffer)),
	)
}
