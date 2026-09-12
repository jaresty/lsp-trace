//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openSkillPayloadDirectory(container *os.File, name, _ string) (*os.File, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(container.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var (
		handle windows.Handle
		status windows.IO_STATUS_BLOCK
	)
	err = windows.NtCreateFile(
		&handle,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		attributes,
		&status,
		nil,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), name), nil
}

type fileRenameInfoEx struct {
	Flags          uint32
	RootDirectory  windows.Handle
	FileNameLength uint32
	FileName       [1]uint16
}

func renameDirectoryNoReplace(parent, _, payload *os.File, _, _, newName string) error {
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
		windows.Handle(payload.Fd()),
		windows.FileRenameInfoEx,
		&buffer[0],
		uint32(len(buffer)),
	)
}
