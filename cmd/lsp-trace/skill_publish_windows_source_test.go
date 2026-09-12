package main

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsPayloadOpenIsCapabilityRelativeAndFailClosed(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	windowsSource, err := os.ReadFile("skill_publish_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	mainText := string(mainSource)
	windowsText := strings.Join(strings.Fields(string(windowsSource)), " ")
	if strings.Contains(mainText, `root.Open(staging + "/" + payload)`) {
		t.Fatal("ASSERT_WINDOWS_PAYLOAD_OPEN_CAPABILITY_RELATIVE: payload is reopened through the parent pathname")
	}
	for _, required := range []string{
		"windows.NtCreateFile", "RootDirectory:",
		"windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE",
		"windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE",
		"windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE",
		"windows.FILE_OPEN",
		"windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT",
	} {
		if !strings.Contains(windowsText, required) {
			t.Fatalf("ASSERT_WINDOWS_PAYLOAD_OPEN_FAILS_CLOSED: missing %q", required)
		}
	}
	pin := strings.Index(mainText, "afterSkillContainerPinned")
	open := strings.Index(mainText, "openSkillPayloadDirectory")
	if pin < 0 || open < 0 || pin >= open {
		t.Fatalf("ASSERT_SKILL_CONTAINER_PIN_HOOK_PRECEDES_PAYLOAD_OPEN: hook=%d open=%d", pin, open)
	}
}
