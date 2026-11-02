//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func TestHostApplicationCompatibilityFinalizeDiagnostics(t *testing.T) {
	cases := []struct {
		reason string
		mutate func(*product.CohortRoleReturn)
	}{
		{"ROLE_ORDER", func(r *product.CohortRoleReturn) { r.Role = "secret-role" }},
		{"ROLE_ORDER_PUBLICATION_FAILED", func(r *product.CohortRoleReturn) { r.Role = "secret-role" }},
		{"DUPLICATE_READY_ID", func(r *product.CohortRoleReturn) {}},
		{"READY_IDENTITY", func(r *product.CohortRoleReturn) { r.LaunchID = "secret-id" }},
		{"TERMINAL_BINDING", func(r *product.CohortRoleReturn) { r.TerminalSHA256 = pilot.Digest([]byte("other")) }},
		{"STATUS_BINDING", func(r *product.CohortRoleReturn) { r.Status = "secret-status" }},
		{"RETURN_DIGEST", func(r *product.CohortRoleReturn) { r.ReturnSHA256 = "secret-return" }},
		{"SEQUENCE_BOUND", func(r *product.CohortRoleReturn) { r.Sequence = 17 }},
		{"KIND_BINDING", func(r *product.CohortRoleReturn) { r.Kind = "secret-kind" }},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			r, path, _ := hostApplicationCLIFixture(t)
			var out, errout bytes.Buffer
			if runPrivateHostApplication([]string{"setup", path}, strings.NewReader(""), &out, &errout) != 0 {
				t.Fatal(errout.String())
			}
			var setup struct{ RunID string }
			if json.Unmarshal(out.Bytes(), &setup) != nil {
				t.Fatal("setup")
			}
			for i, role := range []string{"interpreter", "reviewer"} {
				id := role
				if tc.reason == "DUPLICATE_READY_ID" {
					id = "interpreter"
				}
				if runPrivateHostApplication([]string{"register", path, setup.RunID, role, id, r.Configuration.RolePromptSHA256[i]}, strings.NewReader(""), &out, &errout) != 0 {
					t.Fatal(errout.String())
				}
			}
			terminal, _ := json.Marshal(applicationTerminal{Schema: "private-host-application-terminal.v1", Status: "CONTROLLED_DIAGNOSTIC_FIXTURE"})
			if e := applicationWriteOnce(applicationTerminalPath(r.Configuration.Store, setup.RunID), terminal); e != nil {
				t.Fatal(e)
			}
			returns := [2]product.CohortRoleReturn{}
			for i, role := range []string{"interpreter", "reviewer"} {
				returns[i] = product.CohortRoleReturn{Kind: "SCRIPTED_CONTRACT", Role: role, LaunchID: role, TerminalSHA256: pilot.Digest(terminal), Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("controlled-return")), Sequence: 0}
			}
			if tc.reason == "DUPLICATE_READY_ID" {
				returns[1].LaunchID = "interpreter"
			}
			tc.mutate(&returns[0])
			expectedReason := strings.TrimSuffix(tc.reason, "_PUBLICATION_FAILED")
			if tc.reason == "ROLE_ORDER_PUBLICATION_FAILED" {
				if e := os.Mkdir(filepath.Join(r.Configuration.Store, fmt.Sprintf("host-refusal-%s-0-ROLE_ORDER.json", setup.RunID[7:])), 0700); e != nil {
					t.Fatal(e)
				}
			}
			raw, _ := json.Marshal(returns)
			out.Reset()
			errout.Reset()
			code := runPrivateHostApplication([]string{"finalize", path, setup.RunID}, bytes.NewReader(raw), &out, &errout)
			var diagnostic struct{ Schema, Class, Reason, Path, DiagnosticStatus string }
			if tc.reason == "ROLE_ORDER_PUBLICATION_FAILED" {
				if code == 0 || json.Unmarshal(out.Bytes(), &diagnostic) != nil || diagnostic.Class != "HOST_RETURN_REFUSED" || diagnostic.DiagnosticStatus != "PUBLICATION_FAILED" || diagnostic.Path != "" {
					t.Fatalf("ASSERT_DIAGNOSTIC_PUBLICATION_FAILURE_SURFACED code=%d output=%s", code, out.String())
				}
				t.Log("ASSERT_DIAGNOSTIC_PUBLICATION_FAILURE_SURFACED A-pass")
				return
			}
			if code == 0 || json.Unmarshal(out.Bytes(), &diagnostic) != nil || diagnostic.Class != "HOST_RETURN_REFUSED" || diagnostic.Reason != expectedReason || diagnostic.Path == "" {
				t.Fatalf("ASSERT_SAFE_FINALIZE_DIAGNOSTIC_%s code=%d output=%s error=%s", tc.reason, code, out.String(), errout.String())
			}
			b, e := os.ReadFile(filepath.Join(r.Configuration.Store, diagnostic.Path))
			if e != nil || len(b) > 4096 || bytes.Contains(b, []byte("secret-")) {
				t.Fatalf("ASSERT_SAFE_FINALIZE_DIAGNOSTIC_%s unsafe record %v", tc.reason, e)
			}
			var record struct {
				Schema, Class, Reason, RunID, Qualification                    string
				RoleIndex                                                      int
				Expected, Actual                                               [6]string
				ActualSequence, MaximumSequence                                uint64
				ReturnDigestValid                                              bool
				RoleMatches, ReadyIdentityMatches, DuplicateReadyID            bool
				TerminalMatches, StatusMatches, SequenceWithinBound, KindValid bool
			}
			index := 0
			if tc.reason == "DUPLICATE_READY_ID" {
				index = 1
			}
			ret := returns[index]
			role := []string{"interpreter", "reviewer"}[index]
			readyID := role
			if tc.reason == "DUPLICATE_READY_ID" {
				readyID = "interpreter"
			}
			expected := [6]string{role, readyID, pilot.Digest(terminal), "RETURNED", "VALID_SHA256", "HOST_OBSERVED_OR_SCRIPTED_CONTRACT"}
			actual := [6]string{ret.Role, ret.LaunchID, ret.TerminalSHA256, ret.Status, ret.ReturnSHA256, ret.Kind}
			for n := range expected {
				expected[n] = pilot.Digest([]byte(expected[n]))
				actual[n] = pilot.Digest([]byte(actual[n]))
			}
			if json.Unmarshal(b, &record) != nil || record.Schema != "private-application-finalize-diagnostic.v1" || record.Class != "HOST_RETURN_REFUSED" || record.Reason != expectedReason || record.RunID != setup.RunID || record.RoleIndex != index || record.Expected != expected || record.Actual != actual || record.ActualSequence != ret.Sequence || record.MaximumSequence != 16 || record.ReturnDigestValid != validApplicationDigest(ret.ReturnSHA256) || record.RoleMatches != (ret.Role == role) || record.ReadyIdentityMatches != (ret.LaunchID == readyID) || record.DuplicateReadyID != (tc.reason == "DUPLICATE_READY_ID") || record.TerminalMatches != (ret.TerminalSHA256 == pilot.Digest(terminal)) || record.StatusMatches != (ret.Status == "RETURNED") || record.SequenceWithinBound != (ret.Sequence <= 16) || record.KindValid != (ret.Kind == "HOST_OBSERVED" || ret.Kind == "SCRIPTED_CONTRACT") || record.Qualification != "OBSERVABLE_HASH_IDENTITY_NOT_AUTHENTICATION_CAUSE_UNKNOWN" {
				t.Fatalf("ASSERT_FULL_DIAGNOSTIC_OPERANDS_%s invalid typed record", tc.reason)
			}
			st, e := os.Stat(filepath.Join(r.Configuration.Store, diagnostic.Path))
			if e != nil || st.Mode().Perm() != 0600 {
				t.Fatal("private diagnostic")
			}
			t.Logf("ASSERT_SAFE_FINALIZE_DIAGNOSTIC_%s A-pass", tc.reason)
		})
	}
}
