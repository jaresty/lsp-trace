//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/runtimeprofile"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPrivateSourcePrepareDescribeConfig(t *testing.T) {
	c := privateSourcePrepareDescribeConfig{Schema: "private-source-prepare-describe.v1", AccountingRoot: "/tmp/private-accounting", CanonicalJob: []byte("unissued declaration"), ExpectedHead: "sha256:" + strings.Repeat("1", 64), Profile: runtimeprofile.Selector{Workspace: "/tmp/source"}, Workspace: "/tmp/source", ProcessPath: "/not-executed", PacketLimit: 1 << 20}
	c.Persist = inv.PreparedDescribePersistRequestV1{RootPath: c.AccountingRoot, InitialHead: c.ExpectedHead, Deadline: time.Now().Add(time.Second)}
	for _, fault := range []string{"valid", "HOST", "OutputRoots", "Timing", "borrowedgrant", "duplicate", "oversize", "root-mismatch", "trailing"} {
		t.Run(fault, func(t *testing.T) {
			q := c
			if fault == "root-mismatch" {
				q.Persist.RootPath = "/foreign"
			}
			raw, _ := json.Marshal(q)
			switch fault {
			case "HOST", "OutputRoots", "Timing", "borrowedgrant":
				raw = append([]byte("{\""+fault+"\":true,"), raw[1:]...)
			case "duplicate":
				raw = append([]byte("{\"Schema\":\"private-source-prepare-describe.v1\","), raw[1:]...)
			case "oversize":
				raw = bytes.Repeat([]byte(" "), 65537)
			case "trailing":
				raw = append(raw, []byte("{}")...)
			}
			_, e := decodePrivateSourcePrepareDescribeConfig(raw)
			if (e == nil) != (fault == "valid") {
				t.Fatal("ASSERT_PREPARE_CONFIG_STRICT", fault, e)
			}
		})
	}
}
func TestPrivateSourcePrepareDescribeDispatchRefused(t *testing.T) {
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"source-prepare-describe"}, nil, &out, &diag) == 0 || out.Len() != 0 || !strings.Contains(diag.String(), "CONFIG_REFUSED") {
		t.Fatal("ASSERT_PREPARE_DISPATCH", diag.String())
	}
}
func TestPrivateSourcePrepareDescribeConfigFileRefused(t *testing.T) {
	for _, fault := range []string{"mode", "symlink", "fifo", "oversize", "unknown-control"} {
		t.Run(fault, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config")
			switch fault {
			case "fifo":
				if e := syscall.Mkfifo(p, 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				target := p + ".target"
				if e := os.WriteFile(target, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(target, p); e != nil {
					t.Fatal(e)
				}
			default:
				raw := []byte(`{"HOST":{"OutputRoots":["not-opened"]}}`)
				mode := os.FileMode(0600)
				if fault == "mode" {
					mode = 0644
				}
				if fault == "oversize" {
					raw = bytes.Repeat([]byte("x"), 65537)
				}
				if e := os.WriteFile(p, raw, mode); e != nil {
					t.Fatal(e)
				}
			}
			var out, diag bytes.Buffer
			if runPrivateSourcePrepareDescribe([]string{"source-prepare-describe", p}, &out, &diag) == 0 || out.Len() != 0 || !strings.Contains(diag.String(), "CONFIG_REFUSED") {
				t.Fatal("ASSERT_PREPARE_CONFIG_FILE_ZERO", fault, diag.String())
			}
		})
	}
}
func TestPrivateSourcePrepareDescribeOutputFailure(t *testing.T) {
	// Writer-only unit check, NOT positive native pipeline qualification.
	for _, writer := range []interface{ Write([]byte) (int, error) }{readyShortWriter{}, readyErrorWriter{}} {
		var diag bytes.Buffer
		if finishPrivateSourcePrepareDescribe(context.Background(), time.Now().Add(time.Second), inv.StagedPrepareDescribeV1{Packet: []byte("writer test")}, nil, writer, &diag) == 0 || !strings.Contains(diag.String(), "POSSIBLY_PARTIAL") {
			t.Fatal("ASSERT_PREPARE_PARTIAL_OUTPUT_FAILED")
		}
	}
}
