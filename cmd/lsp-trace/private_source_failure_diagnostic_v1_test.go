//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/inventorysuccessor/sourceoperator"
)

func TestPrivateSourceFailureDiagnosticModeled(t *testing.T) {
	now := time.Unix(100, 0)
	out := sourceoperator.Outcome{Schema: "SOURCE_NATIVE_V1", Disposition: "SOURCE_PROVISIONAL_DELIVERED_PAIR"}
	secret := errors.New("SECRET_SENTINEL file:///private providerstderr grant nonce")
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var errs [5]error
			deadline := now.Add(time.Second)
			if i < 5 {
				errs[i] = secret
			}
			if i == 5 {
				deadline = now
			}
			if i == 6 {
				errs[1] = context.DeadlineExceeded
				errs[2] = context.Canceled
			}
			d := privateSourceDiagnosticV1(out, errs[0], errs[1], errs[2], errs[3], errs[4], false, false, now, deadline)
			expected := i != 7
			flags := []bool{d.ExecutionError, d.ShutdownError, d.CloseError, d.TotalContextError, d.CleanupContextError, d.TerminalDeadlineExpired}
			if d.failed() != expected {
				t.Fatal("ASSERT_TERMINAL_PREDICATE")
			}
			for j, flag := range flags {
				want := i == j || (i == 6 && (j == 1 || j == 2))
				if flag != want {
					t.Fatal("ASSERT_DISTINCT_TERMINAL_FLAGS", j)
				}
			}
			var stdout, stderr bytes.Buffer
			code := finishPrivateSourceV1(out, d, &stdout, &stderr)
			if (code == 1) != expected {
				t.Fatal("ASSERT_TERMINAL_EXIT")
			}
			wanted := out
			if expected {
				wanted.Disposition = "FAILED_RETAINED"
			}
			raw, _ := json.Marshal(wanted)
			if stdout.String() != string(raw)+"\n" {
				t.Fatal("ASSERT_STDOUT_COMPATIBLE")
			}
			if expected {
				lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
				if len(lines) != 2 || lines[0] != "private source application: EXECUTION_REFUSED_RETAINED" || !json.Valid([]byte(lines[1])) || len(lines[1])+1 > 2048 {
					t.Fatal("ASSERT_BOUNDED_FAILURE_RECORD")
				}
			} else if stderr.Len() != 0 {
				t.Fatal("ASSERT_SUCCESS_SILENT")
			}
			if strings.Contains(stdout.String()+stderr.String(), "SECRET_SENTINEL") {
				t.Fatal("ASSERT_NO_SECRET")
			}
			if i == 6 && (d.ShutdownClass != "deadline_exceeded" || d.CloseClass != "canceled") {
				t.Fatal("ASSERT_MIXED_CLEANUP_CLASSES")
			}
		})
	}
	t.Log("ASSERT_TERMINAL_PREDICATE: PASS; ASSERT_DISTINCT_TERMINAL_FLAGS: PASS; ASSERT_TERMINAL_EXIT: PASS; ASSERT_STDOUT_COMPATIBLE: PASS; ASSERT_BOUNDED_FAILURE_RECORD: PASS; ASSERT_SUCCESS_SILENT: PASS; ASSERT_NO_SECRET: PASS; ASSERT_MIXED_CLEANUP_CLASSES: PASS (MODELED)")
}

type diagnosticFailWriterV1 struct{}

func (diagnosticFailWriterV1) Write([]byte) (int, error) { return 0, errors.New("SECRET_SENTINEL") }
func TestPrivateSourceFailureDiagnosticOutputModeled(t *testing.T) {
	for _, failed := range []bool{false, true} {
		d := privateSourceFailureDiagnosticV1{ExecutionError: failed}
		var stderr bytes.Buffer
		if finishPrivateSourceV1(sourceoperator.Outcome{}, d, diagnosticFailWriterV1{}, &stderr) != 1 {
			t.Fatal("ASSERT_OUTPUT_EXIT")
		}
		if !strings.HasPrefix(stderr.String(), "private source application: OUTPUT_REFUSED\n") || strings.Contains(stderr.String(), "SECRET_SENTINEL") || strings.Count(stderr.String(), "\n") != 1+map[bool]int{true: 1, false: 0}[failed] {
			t.Fatal("ASSERT_OUTPUT_GENERIC_PRESERVED")
		}
	}
	t.Log("ASSERT_OUTPUT_EXIT: PASS; ASSERT_OUTPUT_GENERIC_PRESERVED: PASS (MODELED failing writer)")
}
func TestPrivateSourceFailureDiagnosticMaliciousModeled(t *testing.T) {
	secret := strings.Repeat("SECRET_SENTINEL", 1024)
	d := privateSourceFailureDiagnosticV1{Schema: secret, Stage: secret, OperatorClass: secret, ExecutionClass: secret, ShutdownClass: secret, CloseClass: secret, TotalContextClass: secret, CleanupContextClass: secret, ExecutionError: true}
	var stdout, stderr bytes.Buffer
	finishPrivateSourceV1(sourceoperator.Outcome{}, d, &stdout, &stderr)
	if strings.Contains(stderr.String(), "SECRET_SENTINEL") || stderr.Len() > 2048 {
		t.Fatal("ASSERT_WRITER_ENUM_SANITIZED")
	}
	var record privateSourceFailureDiagnosticV1
	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	if len(lines) != 2 || json.Unmarshal([]byte(lines[1]), &record) != nil || record.Schema != "SOURCE_FAILURE_DIAGNOSTIC_V1" || record.Stage != "unknown" || record.OperatorClass != "other" {
		t.Fatal("ASSERT_WRITER_FIXED_SCHEMA")
	}
	t.Log("ASSERT_WRITER_ENUM_SANITIZED: PASS; ASSERT_WRITER_FIXED_SCHEMA: PASS (MODELED malicious metadata)")
}
func TestPrivateSourceFailureDiagnosticTailModeled(t *testing.T) {
	now := time.Unix(100, 0)
	d := privateSourceDiagnosticV1(sourceoperator.Outcome{}, nil, nil, nil, context.DeadlineExceeded, context.Canceled, true, true, now, now.Add(time.Second))
	if d.ExecutionError || !d.TailInvalid || !d.failed() || d.Stage != "contract" || d.TotalContextClass != "deadline_exceeded" || d.CleanupContextClass != "canceled" {
		t.Fatal("ASSERT_TAIL_AND_CONTEXT_FACTS")
	}
	var stdout, stderr bytes.Buffer
	finishPrivateSourceV1(sourceoperator.Outcome{}, d, &stdout, &stderr)
	if !strings.HasPrefix(stderr.String(), "private source application: EXECUTION_REFUSED_RETAINED\n") {
		t.Fatal("ASSERT_GENERIC_FIRST")
	}
	t.Log("ASSERT_TAIL_AND_CONTEXT_FACTS: PASS; ASSERT_GENERIC_FIRST: PASS (MODELED)")
}
