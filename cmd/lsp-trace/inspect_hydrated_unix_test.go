//go:build unix

package main

import (
	"bytes"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestHydratedCLIFIFO(t *testing.T) {
	file := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(file, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		var out, stderr bytes.Buffer
		done <- runInspect([]string{file, "--hydrated", "--json"}, &out, &stderr)
	}()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("PUBLIC_FIFO FAIL: admitted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("PUBLIC_FIFO FAIL: blocked")
	}
	t.Log("PUBLIC_FIFO PASS")
}
