package main

import (
	"errors"
	"io"
	"testing"
	"time"

	"lsp-trace/internal/censusrequest"
)

// Observe options at the first resolver boundary: no filesystem preparation,
// provider, or semantic task is needed to witness default/deadline selection.
func TestCensusDurableTimeoutBeforePreparation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		caller  string
		want    time.Duration
		durable bool
	}{
		{"normal_default", nil, "", 180 * time.Second, true},
		{"explicit_60", []string{"--timeout", "60s"}, "", 60 * time.Second, true},
		{"explicit_single_dash", []string{"-timeout=60s"}, "", 60 * time.Second, true},
		{"explicit_240_ceiling", []string{"--timeout=240s"}, "", 240 * time.Second, true},
		{"connected_default", nil, "3", 60 * time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LSP_TRACE_CALLER_FD", tc.caller)
			start := time.Now()
			called := false
			deps := censusRunnerDependencies{resolveFolder: func(o censusCLIOptions) (censusCLIOptions, error) {
				called = true
				if o.Timeout != tc.want {
					t.Errorf("timeout before preparation = %s, want %s", o.Timeout, tc.want)
				}
				if o.DurableInventory != tc.durable {
					t.Errorf("durable = %v", o.DurableInventory)
				}
				if tc.durable {
					span := min(tc.want, 180*time.Second)
					if o.durableDeadline.Before(start.Add(span)) || o.durableDeadline.After(time.Now().Add(span)) {
						t.Errorf("deadline not fixed before preparation: %v", o.durableDeadline)
					}
				} else if !o.durableDeadline.IsZero() {
					t.Errorf("legacy connected deadline changed")
				}
				return o, errors.New("stop before effects")
			}}
			if code := runCensusWithDependencies(tc.args, io.Discard, io.Discard, deps); code != 1 || !called {
				t.Fatalf("resolver not reached: code=%d called=%v", code, called)
			}
		})
	}
}

func TestCensusLegacyTimeoutDefaultsUnchanged(t *testing.T) {
	if censusrequest.DefaultTimeoutMS != 60000 {
		t.Fatal("shared receipt default changed")
	}
	for _, args := range [][]string{
		{"--workspace", "/workspace", "--server", "gopls", "--publication-root", "/private"},
		{"--durable-describe"},
	} {
		o, err := parseCensusCLIOptions(args)
		if err != nil {
			t.Fatal(err)
		}
		if o.Timeout != 60*time.Second {
			t.Fatalf("legacy timeout = %s", o.Timeout)
		}
	}
}
