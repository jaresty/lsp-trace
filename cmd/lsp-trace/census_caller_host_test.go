package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/sessionruntime"
)

func publicCallerHostArgs(path string) []string {
	return []string{"--catalog", "--caller-host", "--catalog-config", path, "--machine"}
}

// Exercise the real connected discovery admission/custody code. The existing
// fixture creates fresh synthetic test accounting only; the starter refuses
// before constructing any native provider or application HOST.
func TestCensusCallerHostDiscoveryCustody(t *testing.T) {
	_, _, header, _ := connectedCursorFixture(t, 16)
	first := custodyFundedConfig46(t, header)
	second := custodyFundedConfig46(t, header)
	starts := 0
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
		starts++
		return nil, errors.New("controlled refusal before provider construction")
	}}
	invoke := func(c censusInventoryBridgeConfig) int {
		t.Helper()
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "connected.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		return runCensusWithDependencies(publicCallerHostArgs(path), &out, &diagnostic, deps)
	}
	if code := invoke(first); code == 0 || starts != 1 {
		t.Fatalf("ASSERT_PUBLIC_HOST_CONNECTED_DISPATCH: code=%d starts=%d", code, starts)
	}
	t.Log("ASSERT_PUBLIC_HOST_CONNECTED_DISPATCH PASS")
	if code := invoke(second); code == 0 || starts != 1 {
		t.Fatalf("ASSERT_PUBLIC_HOST_NO_REPLAY: code=%d starts=%d", code, starts)
	}
	t.Log("ASSERT_PUBLIC_HOST_NO_REPLAY PASS")
}

func TestCensusCallerHostLegacyServerArgument(t *testing.T) {
	args := []string{"--workspace", "/synthetic", "--server", "never-started", "--publication-root", "/synthetic/publication", "--server-arg", "--caller-host", "--machine"}
	o, err := parseCensusCLIOptions(args)
	if err != nil || o.CallerHost || len(o.ServerArgs) != 1 || o.ServerArgs[0] != "--caller-host" {
		t.Fatalf("ASSERT_PUBLIC_HOST_LEGACY_SERVER_ARG: options=%+v error=%v", o, err)
	}
	t.Log("ASSERT_PUBLIC_HOST_LEGACY_SERVER_ARG PASS")
}

func TestCensusCallerHostEntry(t *testing.T) {
	var out, diagnostic bytes.Buffer
	starts := 0
	deps := censusRunnerDependencies{newStarter: func() (sessionruntime.Starter, error) {
		starts++
		return nil, errors.New("legacy provider must not be constructed")
	}}
	code := runCensusWithDependencies(publicCallerHostArgs(filepath.Join(t.TempDir(), "missing.json")), &out, &diagnostic, deps)
	if code == 0 || starts != 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "CONFIG") || strings.Contains(diagnostic.String(), "SYNTAX") {
		t.Fatalf("ASSERT_PUBLIC_HOST_ENTRY: code=%d starts=%d out=%s diagnostic=%s", code, starts, &out, &diagnostic)
	}
	t.Log("ASSERT_PUBLIC_HOST_ENTRY PASS")
}

func TestCensusCallerHostExclusiveSyntax(t *testing.T) {
	base := publicCallerHostArgs("/synthetic/connected.json")
	for _, extra := range [][]string{
		{"--workspace", "/synthetic"}, {"--publication-root", "/synthetic"},
		{"--server", "never-started"}, {"--profile", "go"}, {"--source", "."},
		{"--resume", "not-a-selector"}, {"--stop-after", "describe-requests"},
		{"--private-inventory-bridge-config", "/synthetic/other.json"},
		{"--caller-host"}, {"-caller-host"}, {"--caller-host=false"},
		{"--catalog-config", "/synthetic/other.json"}, {"-machine"},
		{"--timeout", "1s"}, {"--enumeration-preflight"},
	} {
		t.Run(strings.Join(extra, " "), func(t *testing.T) {
			args := append(append([]string(nil), base...), extra...)
			var out, diagnostic bytes.Buffer
			code := runCensusWithDependencies(args, &out, &diagnostic, censusRunnerDependencies{})
			if code == 0 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "SYNTAX") {
				t.Fatalf("ASSERT_PUBLIC_HOST_EXCLUSIVE: code=%d diagnostic=%s", code, &diagnostic)
			}
		})
	}
	for _, args := range [][]string{
		{"--caller-host", "--catalog-config", "/synthetic/connected.json", "--machine"},
		{"--catalog", "--caller-host", "--machine"},
		{"--catalog", "--caller-host", "--catalog-config", "/synthetic/connected.json"},
		publicCallerHostArgs("relative.json"),
		publicCallerHostArgs("/synthetic/../connected.json"),
		{"--catalog", "--caller-host=false", "--catalog-config", "/synthetic/connected.json", "--machine"},
	} {
		if _, err := parseCensusCLIOptions(args); err == nil {
			t.Fatalf("ASSERT_PUBLIC_HOST_REQUIRED: accepted %q", args)
		}
	}
}

func TestCensusCallerHostPrivateControlsRemainExclusive(t *testing.T) {
	for _, private := range []string{"--private-inventory-bridge-config", "--private-inventory-qualified-prepare-config"} {
		args := []string{private, "/synthetic/connected.json", "--machine", "--caller-host"}
		if _, err := parseCensusCLIOptions(args); err == nil {
			t.Fatalf("ASSERT_PUBLIC_HOST_PRIVATE_EXCLUSIVE: accepted %q", args)
		}
	}
}

func TestCensusCallerHostMalformedConfigNoFallback(t *testing.T) {
	for _, content := range []string{`{}`, `{"version":1,"continuation":{}}`, `{"Schema":"private-connected-inventory-bridge.v1"}`} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "connected.json")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			var publicOut, publicErr, privateOut, privateErr bytes.Buffer
			deps := censusRunnerDependencies{}
			publicCode := runCensusWithDependencies(publicCallerHostArgs(path), &publicOut, &publicErr, deps)
			privateCode := runCensusWithDependencies([]string{"--private-inventory-bridge-config", path, "--machine"}, &privateOut, &privateErr, deps)
			if publicCode != privateCode || !bytes.Equal(publicOut.Bytes(), privateOut.Bytes()) || !bytes.Equal(publicErr.Bytes(), privateErr.Bytes()) {
				t.Fatalf("ASSERT_PUBLIC_HOST_SAME_ADMISSION: public=%d/%s private=%d/%s", publicCode, &publicErr, privateCode, &privateErr)
			}
		})
	}
}
