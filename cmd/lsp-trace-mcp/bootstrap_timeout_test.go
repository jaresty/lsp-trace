package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrapTimeoutPrecedenceAndDefault(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_TIMEOUT_PRECEDENCE"
	for _, tc := range []struct {
		name    string
		process string
		host    time.Duration
		want    time.Duration
	}{
		{name: "omitted historical default", want: 10 * time.Second},
		{name: "host override", host: 15 * time.Minute, want: 15 * time.Minute},
		{name: "process override", process: "20m", host: 15 * time.Minute, want: 20 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := effectiveBootstrapTimeout(tc.process, tc.host)
			if err != nil || got != tc.want {
				t.Fatalf("%s: got=%s want=%s err=%v", assertion, got, tc.want, err)
			}
		})
	}
}

func TestBootstrapTimeoutConfigAndCLIRejectInvalidValues(t *testing.T) {
	const bodyPrefix = `{"version":1,"processes":[{"bootstrap_timeout":`
	const bodySuffix = `,"profile":{"trust_domain":"test","workspace":"/workspace","profile":"go","environment_reference":"local"},"execution":{"path":"/server","directory":"/workspace"}}]}`
	for _, value := range []string{`"0s"`, `"-1s"`, `"bad"`, `"2562048h"`, `"1h1ns"`, `1000`} {
		path := filepath.Join(t.TempDir(), "bootstrap.json")
		if err := os.WriteFile(path, []byte(bodyPrefix+value+bodySuffix), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadBootstrapConfig(path); err == nil {
			t.Fatalf("ASSERT_BOOTSTRAP_TIMEOUT_CONFIG_REJECTED: value=%s", value)
		}
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--bootstrap-timeout", "0s"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "bootstrap timeout") {
		t.Fatalf("ASSERT_BOOTSTRAP_TIMEOUT_CLI_REJECTED: code=%d stderr=%q", code, stderr.String())
	}
}

func TestBootstrapTimeoutDeadlinePropagation(t *testing.T) {
	now := time.Unix(123, 456)
	if got, want := bootstrapReadinessDeadline(now, 15*time.Minute), now.Add(15*time.Minute); !got.Equal(want) {
		t.Fatalf("ASSERT_BOOTSTRAP_TIMEOUT_EXACT_DEADLINE: got=%s want=%s", got, want)
	}
}

func TestBootstrapTimeoutValidationBoundaries(t *testing.T) {
	const assertion = "ASSERT_BOOTSTRAP_TIMEOUT_STRICT_BOUNDS"
	if got, err := parseBootstrapTimeout("1h"); err != nil || got != time.Hour {
		t.Fatalf("%s: exact maximum got=%s err=%v", assertion, got, err)
	}
	for _, value := range []string{"0", "0s", "-1s", "not-a-duration", "2562048h", "1h1ns"} {
		t.Run(value, func(t *testing.T) {
			if _, err := parseBootstrapTimeout(value); err == nil || !strings.Contains(err.Error(), "bootstrap timeout") {
				t.Fatalf("%s: value=%q err=%v", assertion, value, err)
			}
		})
	}
}
