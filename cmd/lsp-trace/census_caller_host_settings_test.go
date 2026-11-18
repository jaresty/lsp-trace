package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/sessionruntime"
)

func callerHostSettingsFixture(t *testing.T) censusInventoryBridgeConfig {
	t.Helper()
	_, _, h, _ := connectedCursorFixture(t, 16)
	c := custodyFundedConfig46(t, h)
	// Synthetic policy only, deliberately not an executable application request.
	c.Request = json.RawMessage(`{"Schema":"policy-test","ApprovalSHA256":"spent-approval","Configuration":{"Schema":"policy-test","SourceRoot":"/synthetic/source","Store":"spent-store","Grant":{"ExpiresUnixNano":1,"Revision":1,"PreviousGrantSHA256":"spent-grant","Ceiling":{"LogicalBytes":9007199254740993},"BootstrapAttempts":64,"BootstrapEach":{"Calls":1}}},"Inventory":{"BudgetBytes":67108864,"NativeRetainedUpper":16777216,"MaxNativeAttempts":1,"HostWindowMS":30000,"Manifest":{"Provenance":"spent-manifest"},"Targets":[{"URI":"spent-uri"}],"Pins":{"model_sha256":"model-policy","worker_sha256":"worker-policy","grammar_sha256":"grammar-policy","prompt_sha256":"spent-prompt"},"NativeOwners":[{"RunID":"spent-native-run","AccountingRoot":"spent-accounting","CanonicalJob":"spent-job","ExpectedHead":"spent-head","Deadline":"2001-01-01T00:00:00Z","Attempts":["spent-M","spent-P","spent-S"],"Contract":{"ContinuationID":"spent-continuation","CaptureLimits":{"MaxWork":10000}},"SourceLimits":{"MaxWork":10000}}]}}`)
	return c
}

func writeCallerHostConfig(t *testing.T, c censusInventoryBridgeConfig) string {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func exportCallerHostSettings(t *testing.T, c censusInventoryBridgeConfig) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	args := append(publicCallerHostArgs(writeCallerHostConfig(t, c)), "--save-caller-host-settings", path)
	var out, diagnostic bytes.Buffer
	if code := runCensusWithDependencies(args, &out, &diagnostic, censusRunnerDependencies{}); code != 0 {
		t.Fatalf("ASSERT_SETTINGS_EXPORT: code=%d diagnostic=%s", code, &diagnostic)
	}
	return path
}

func TestCensusCallerHostSettingsExport(t *testing.T) {
	c := callerHostSettingsFixture(t)
	c.Action, c.RunID, c.Deadline = "run", "spent-run", time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	path := exportCallerHostSettings(t, c)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"spent-", `"Deadline"`, `"ExpiresUnixNano"`, `"CanonicalJob"`, `"ExpectedHead"`, `"ApprovalSHA256"`, `"Attempt"`, `"RunID"`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_SETTINGS_NO_ATTEMPT_STATE: found %s", forbidden)
		}
	}
	if !bytes.Contains(raw, []byte("9007199254740993")) {
		t.Fatal("ASSERT_SETTINGS_EXACT_NUMBERS")
	}
	saved, err := decodeCensusCallerHostSettings(raw)
	if err != nil || saved.Schema != censusCallerHostSettingsSchema {
		t.Fatal("ASSERT_SETTINGS_SCHEMA", err)
	}
	original, err := censusCallerHostStable(c)
	if err != nil || !censusCallerHostSame(original, saved) {
		t.Fatal("ASSERT_SETTINGS_POLICY_ROUNDTRIP", err)
	}
	// The full header, not a one-target denominator, is retained unchanged.
	got, _ := json.Marshal(saved.Bridge["Header"])
	want, _ := json.Marshal(c.Header)
	wantMap, _ := censusCallerHostObject(want)
	want, _ = json.Marshal(wantMap)
	if !bytes.Equal(got, want) {
		t.Fatal("ASSERT_SETTINGS_FULL_HEADER")
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("ASSERT_SETTINGS_PRIVATE_MODE", err)
	}
	args := append(publicCallerHostArgs(writeCallerHostConfig(t, c)), "--save-caller-host-settings", path)
	var out, diagnostic bytes.Buffer
	if runCensusWithDependencies(args, &out, &diagnostic, censusRunnerDependencies{}) == 0 {
		t.Fatal("ASSERT_SETTINGS_CREATE_ONLY")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("ASSERT_SETTINGS_OVERWRITE")
	}
	t.Log("ASSERT_SETTINGS_NO_ATTEMPT_STATE PASS")
	t.Log("ASSERT_SETTINGS_CREATE_ONLY PASS")
}

func TestCensusCallerHostSettingsAdmission(t *testing.T) {
	for _, kind := range []string{"absent", "expired", "mismatched", "matching"} {
		t.Run(kind, func(t *testing.T) {
			c := callerHostSettingsFixture(t)
			path := exportCallerHostSettings(t, c)
			args := publicCallerHostArgs(path)
			if kind != "absent" {
				c.Request = nil // Discovery precedes construction of the application request.
				if kind == "expired" {
					c.Deadline = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
				}
				if kind == "mismatched" {
					c.Mode = 1
				}
				args = append(args, "--caller-host-attempt-config", writeCallerHostConfig(t, c))
			}
			starts := 0
			deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
				starts++
				return nil, errors.New("controlled refusal before provider construction")
			}}
			var out, diagnostic bytes.Buffer
			code := runCensusWithDependencies(args, &out, &diagnostic, deps)
			want := 0
			if kind == "matching" {
				want = 1
			}
			if code == 0 || starts != want {
				t.Fatalf("ASSERT_SETTINGS_ADMISSION_%s: code=%d starts=%d want=%d diagnostic=%s", kind, code, starts, want, &diagnostic)
			}
			t.Logf("ASSERT_SETTINGS_ADMISSION_%s PASS", kind)
		})
	}
}

func TestCensusCallerHostSettingsApplicationPolicy(t *testing.T) {
	c := callerHostSettingsFixture(t)
	path := exportCallerHostSettings(t, c)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeCensusCallerHostSettings(raw)
	if err != nil {
		t.Fatal(err)
	}
	c.Action = "setup"
	if !censusCallerHostSettingsMatch(saved, c) {
		t.Fatal("ASSERT_SETTINGS_APPLICATION_POLICY_MATCH")
	}
	c.Request = bytes.Replace(c.Request, []byte(`"BudgetBytes":67108864`), []byte(`"BudgetBytes":1`), 1)
	if censusCallerHostSettingsMatch(saved, c) {
		t.Fatal("ASSERT_SETTINGS_APPLICATION_POLICY_MISMATCH")
	}
	args := append(publicCallerHostArgs(path), "--caller-host-attempt-config", writeCallerHostConfig(t, c))
	var out, diagnostic bytes.Buffer
	if code := runCensusWithDependencies(args, &out, &diagnostic, censusRunnerDependencies{}); code == 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "CONFIG") {
		t.Fatal("ASSERT_SETTINGS_APPLICATION_POLICY", code, diagnostic.String())
	}
}

func TestCensusCallerHostSettingsSyntax(t *testing.T) {
	for _, args := range [][]string{
		{"--save-caller-host-settings", "/synthetic/settings.json", "--machine"},
		{"--caller-host-attempt-config", "/synthetic/attempt.json", "--machine"},
		append(publicCallerHostArgs("/synthetic/settings.json"), "--caller-host-attempt-config", "relative.json"),
		append(publicCallerHostArgs("/synthetic/settings.json"), "--save-caller-host-settings", "relative.json"),
		append(publicCallerHostArgs("/synthetic/settings.json"), "--save-caller-host-settings", "/synthetic/new.json", "--caller-host-attempt-config", "/synthetic/attempt.json"),
	} {
		if _, err := parseCensusCLIOptions(args); err == nil {
			t.Fatalf("ASSERT_SETTINGS_SYNTAX: accepted %q", args)
		}
	}
}
