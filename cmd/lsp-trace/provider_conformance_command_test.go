package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderConformanceCommandRejectsNonAbsoluteExecutableAsJSON(t *testing.T) {
	input := `{"schema_version":"lsp-trace.provider-conformance-request.v1","request_id":"r","provider":{"name":"p","version":"1"},"protocol":{"name":"proto","version":"1"},"capabilities":{"operations":["observe"]},"limits":{"request_bytes":1024,"response_bytes":1024,"messages":1,"observations":1,"diagnostics":1,"stderr_bytes":0},"custody":{"original_uri":"file:///x","original_revision":"r1","original_digest":"sha256:x"},"payload":{}}`
	var out, errout bytes.Buffer
	code := runProviderConformance([]string{"--executable", "relative", "--input", "-"}, strings.NewReader(input), &out, &errout)
	var report map[string]any
	if code != 1 || json.Unmarshal(out.Bytes(), &report) != nil || report["outcome"] != "failed" || errout.Len() != 0 {
		t.Fatalf("ASSERT_GENERIC_CONFORMANCE_COMMAND_REPORT: code=%d out=%q err=%q", code, out.String(), errout.String())
	}
}

func TestProviderConformanceCommandUsageRequiresExplicitPathAndInput(t *testing.T) {
	var out, errout bytes.Buffer
	if code := runProviderConformance(nil, strings.NewReader(""), &out, &errout); code != 1 || out.Len() != 0 || !strings.Contains(errout.String(), "--executable ABSOLUTE_PATH") {
		t.Fatalf("ASSERT_GENERIC_CONFORMANCE_COMMAND_EXPLICIT_AUTHORITY: code=%d out=%q err=%q", code, out.String(), errout.String())
	}
	if !filepath.IsAbs("/tmp/provider") {
		t.Fatal("test requires absolute path semantics")
	}
}
