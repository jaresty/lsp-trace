//go:build darwin || linux

package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type hostProductRetiredInput struct{ reads int }

func (r *hostProductRetiredInput) Read([]byte) (int, error) {
	r.reads++
	return 0, fs.ErrPermission
}

// No provider, subprocess, native descriptor or retained-store fixture is used.
// Snapshots observe this owned fixture only; they are not a global syscall audit.
func TestHostProductRetiredRun(t *testing.T) {
	root := t.TempDir()
	malformed := filepath.Join(root, "malformed.json")
	if err := os.WriteFile(malformed, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"claim", "result", "auxiliary"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "sentinel"), []byte(dir), 0600); err != nil {
			t.Fatal(err)
		}
	}
	type entry struct {
		Mode fs.FileMode
		Body string
	}
	snapshot := func() map[string]entry {
		t.Helper()
		entries := map[string]entry{}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			e := entry{Mode: info.Mode()}
			if !d.IsDir() {
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				e.Body = string(b)
			}
			entries[path] = e
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no-request", []string{"run"}},
		{"missing-request", []string{"run", filepath.Join(root, "absent", "request.json")}},
		{"malformed-request", []string{"run", malformed}},
		{"directory-request", []string{"run", root}},
		{"empty-request", []string{"run", ""}},
		{"extra-argument", []string{"run", malformed, "extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshot()
			input := &hostProductRetiredInput{}
			var out, diagnostic bytes.Buffer
			code := runPrivateHostProductV3(tc.args, input, &out, &diagnostic)
			if code != 1 || diagnostic.String() != "private host product: RETIRED_OPERATION_RUN: use ordinary census; historical V3 guarantees are not a drop-in contract\n" {
				t.Fatalf("retirement must precede invocation/config admission: code=%d diagnostic=%q", code, diagnostic.String())
			}
			if out.Len() != 0 || input.reads != 0 {
				t.Fatalf("unexpected data IO: stdout=%q stdin reads=%d", out.String(), input.reads)
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				t.Fatalf("fixture state changed: before=%v after=%v", before, after)
			}
		})
	}
}

func TestHostProductRetirementDoesNotReplaceOtherDispatch(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "INVALID_INVOCATION"},
		{[]string{"ready"}, "INVALID_READY"},
		{[]string{"wait"}, "INVALID_WAIT"},
		{[]string{"respond"}, "INVALID_RESPONSE"},
		{[]string{"acquire", ""}, "CONFIG_ADMISSION_REFUSED"},
		{[]string{"resume", ""}, "CONFIG_ADMISSION_REFUSED"},
		{[]string{"preview", ""}, "CONFIG_ADMISSION_REFUSED"},
		{[]string{"setup", ""}, "CONFIG_ADMISSION_REFUSED"},
		{[]string{"fail", ""}, "CONFIG_ADMISSION_REFUSED"},
	} {
		var out, diagnostic bytes.Buffer
		if code := runPrivateHostProductV3(tc.args, &hostProductRetiredInput{}, &out, &diagnostic); code != 1 || diagnostic.String() != "private host product: "+tc.want+"\n" || out.Len() != 0 {
			t.Fatalf("args=%q code=%d diagnostic=%q stdout=%q", tc.args, code, diagnostic.String(), out.String())
		}
	}
}
