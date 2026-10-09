package describeworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describerequest"
)

func writeFixture(t *testing.T, name, body string, mode os.FileMode) (string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	sum := sha256.Sum256(raw)
	return p, "sha256:" + hex.EncodeToString(sum[:])
}
func requireDarwinPreflight(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("describe worker preflight is qualified only on Darwin")
	}
}

func fixtureConfig(t *testing.T) Config {
	t.Helper()
	worker, wd := writeFixture(t, "worker", "#!/bin/sh\nexit 0\n", 0700)
	model, md := writeFixture(t, "model", "model", 0600)
	lib, ld := writeFixture(t, "lib", "lib", 0600)
	sandbox, sd := writeFixture(t, "sandbox", "#!/bin/sh\nexit 0\n", 0700)
	profile, pd := writeFixture(t, "profile", "(version 1)\n(deny network*)\n", 0600)
	grammar, gd := writeFixture(t, "grammar", "root ::= object\n", 0600)
	return Config{Worker: FilePin{worker, wd}, Model: FilePin{model, md}, Library: FilePin{lib, ld}, SandboxExecutable: FilePin{sandbox, sd}, SandboxProfile: FilePin{profile, pd}, Grammar: FilePin{grammar, gd}, RuntimeIdentity: "runtime-v1", AdapterIdentity: "adapter-v1", ModelIdentity: "model-v1", Limits: Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 4096, StderrBytes: 1024, WorkBytes: 8192, TempBytes: 8192}}
}
func TestPreflightPinsBoundsAndPolicy(t *testing.T) {
	requireDarwinPreflight(t)
	c := fixtureConfig(t)
	if _, err := Preflight(c); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Config){"relative": func(c *Config) { c.Worker.Path = "relative" }, "pin": func(c *Config) { c.Model.SHA256 = digestA }, "bound": func(c *Config) { c.Limits.StdoutBytes = 0 }, "policy": func(c *Config) {
		p, _ := writeFixture(t, "bad-profile", "(allow network*)", 0600)
		c.SandboxProfile.Path = p
		raw, _ := os.ReadFile(p)
		s := sha256.Sum256(raw)
		c.SandboxProfile.SHA256 = "sha256:" + hex.EncodeToString(s[:])
	}}
	for n, mutate := range cases {
		t.Run(n, func(t *testing.T) {
			bad := c
			mutate(&bad)
			_, err := Preflight(bad)
			if err == nil {
				t.Fatal("ASSERT_PREFLIGHT_REJECTION")
			}
			var f *Failure
			if !AsFailure(err, &f) || strings.Contains(err.Error(), bad.Worker.Path) {
				t.Fatal("ASSERT_TYPED_SOURCE_SAFE_FAILURE")
			}
		})
	}
}
func TestPreflightPinFailureSubcodesAreSourceSafe(t *testing.T) {
	requireDarwinPreflight(t)
	base := fixtureConfig(t)
	cases := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"path":    {func(c *Config) { c.Model.Path = "relative" }, "MODEL_PIN_PATH_INVALID"},
		"missing": {func(c *Config) { c.Model.Path = filepath.Join(t.TempDir(), "missing") }, "MODEL_PIN_FILE_MISSING"},
		"regular": {func(c *Config) { c.Model.Path = t.TempDir() }, "MODEL_PIN_NOT_REGULAR"},
		"symlink": {func(c *Config) {
			target, digest := writeFixture(t, "model-target", "model", 0600)
			link := filepath.Join(t.TempDir(), "model-link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			c.Model = FilePin{Path: link, SHA256: digest}
		}, "MODEL_PIN_SYMLINK"},
		"digest": {func(c *Config) { c.Model.SHA256 = digestA }, "MODEL_PIN_DIGEST_MISMATCH"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := base
			tc.mutate(&c)
			_, err := Preflight(c)
			var f *Failure
			if !AsFailure(err, &f) || f.Code() != CodeModelUnavailable || f.Subcode() != tc.want {
				t.Fatalf("ASSERT_SOURCE_SAFE_PIN_SUBCODE: got=%v want=%s", err, tc.want)
			}
			if strings.Contains(err.Error(), c.Model.Path) || strings.Contains(f.Subcode(), c.Model.Path) {
				t.Fatal("ASSERT_SOURCE_SAFE_PIN_SUBCODE_LEAK")
			}
		})
	}
}

func TestStrictWorkerResult(t *testing.T) {
	semantic := `{"verdict":"SUPPORTED","target_role":"role","nearest_outward_consumer":"consumer","consumer_need":"need","provided_behavior":"behavior","boundary_contribution":"boundary","limitations":["bounded"],"citations":["C1"]}`
	valid := []byte(`{"status":"COMPLETE","text":` + quoteJSON(semantic) + `,"tokens":1,"load_ms":2.5,"run_ms":3.5,"cancelled":false,"decode_status":0,"error":"","grammar":"json","context_tokens":64}` + "\n")
	if _, err := parseWorkerResult(valid); err != nil {
		t.Fatal(err)
	}
	withoutError := bytes.Replace(valid, []byte(`,"error":""`), nil, 1)
	if _, err := parseWorkerResult(withoutError); err == nil {
		t.Fatal("ASSERT_V1_OMITTED_EMPTY_ERROR_REMAINS_NONCANONICAL")
	}
	for n, bad := range map[string][]byte{"no-lf": bytes.TrimSuffix(valid, []byte("\n")), "trailing": append(append([]byte(nil), valid...), []byte("{}\n")...), "duplicate": bytes.Replace(valid, []byte(`{"status":`), []byte(`{"status":"COMPLETE","status":`), 1), "unknown": bytes.Replace(valid, []byte(`{"status":`), []byte(`{"unknown":0,"status":`), 1), "nested": bytes.Replace(valid, []byte(`\"citations\":`), []byte(`\"unknown\":0,\"citations\":`), 1)} {
		t.Run(n, func(t *testing.T) {
			if _, err := parseWorkerResult(bad); err == nil {
				t.Fatal("ASSERT_OUTPUT_INVALID")
			}
		})
	}
}
func TestPolicyMismatchOffDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-Darwin contract")
	}
	r, err := NewRunner(fixtureConfig(t))
	if err == nil || r != nil {
		t.Fatal("ASSERT_POLICY_MISMATCH")
	}
}
func TestRunRejectsInvalidRequestWithoutStarting(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin supervisor")
	}
	r, err := NewRunner(fixtureConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = r.Run(ctx, describerequestZero(), "attempt")
	if err == nil {
		t.Fatal("ASSERT_REQUEST_VALIDATION")
	}
}
func quoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
func describerequestZero() (r describerequest.Record) { return }
