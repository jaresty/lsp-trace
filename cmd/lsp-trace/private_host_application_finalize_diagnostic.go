//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// An observation of compared operands, never authentication or a cause inference.
// Eight fixed reasons times two roles bound records to sixteen per run, each
// below 4096 bytes. Repetition cannot append records or overwrite first evidence.
func applicationReturnReason(ret product.CohortRoleReturn, ready product.Ready, role, terminal string, duplicate bool) string {
	switch {
	case ret.Role != role:
		return "ROLE_ORDER"
	case ret.LaunchID != ready.LaunchID:
		return "READY_IDENTITY"
	case duplicate:
		return "DUPLICATE_READY_ID"
	case ret.TerminalSHA256 != terminal:
		return "TERMINAL_BINDING"
	case ret.Status != "RETURNED":
		return "STATUS_BINDING"
	case !validApplicationDigest(ret.ReturnSHA256):
		return "RETURN_DIGEST"
	case ret.Sequence > 16:
		return "SEQUENCE_BOUND"
	case ret.Kind != "HOST_OBSERVED" && ret.Kind != "SCRIPTED_CONTRACT":
		return "KIND_BINDING"
	}
	return ""
}

func applicationRefusedReturn(root *os.Root, ticket string, index int, ret product.CohortRoleReturn, ready product.Ready, role, terminal, reason string, duplicate bool, out, errout io.Writer, lanes ...*applicationFinalizeLane) int {
	if applicationFinalizeCheck(lanes...) != nil {
		fmt.Fprintln(errout, "private application HOST: FINALIZE_SLICE_EXPIRED_CHARGED")
		return 1
	}
	// Hash every string, including invalid digest-looking values. No raw external
	// field (even a role/status/launch ID) enters the private diagnostic or stdout.
	expected := [6]string{role, ready.LaunchID, terminal, "RETURNED", "VALID_SHA256", "HOST_OBSERVED_OR_SCRIPTED_CONTRACT"}
	actual := [6]string{ret.Role, ret.LaunchID, ret.TerminalSHA256, ret.Status, ret.ReturnSHA256, ret.Kind}
	for i := range expected {
		if applicationFinalizeCheck(lanes...) != nil {
			fmt.Fprintln(errout, "private application HOST: FINALIZE_SLICE_EXPIRED_CHARGED")
			return 1
		}
		expected[i] = pilot.Digest([]byte(expected[i]))
		if applicationFinalizeCheck(lanes...) != nil {
			fmt.Fprintln(errout, "private application HOST: FINALIZE_SLICE_EXPIRED_CHARGED")
			return 1
		}
		actual[i] = pilot.Digest([]byte(actual[i]))
	}
	record := struct {
		Schema, Class, Reason, RunID                                   string
		RoleIndex                                                      int
		Expected, Actual                                               [6]string
		ActualSequence, MaximumSequence                                uint64
		ReturnDigestValid                                              bool
		RoleMatches, ReadyIdentityMatches, DuplicateReadyID            bool
		TerminalMatches, StatusMatches, SequenceWithinBound, KindValid bool
		Qualification                                                  string
	}{"private-application-finalize-diagnostic.v1", "HOST_RETURN_REFUSED", reason, ticket, index, expected, actual, ret.Sequence, 16, validApplicationDigest(ret.ReturnSHA256), ret.Role == role, ret.LaunchID == ready.LaunchID, duplicate, ret.TerminalSHA256 == terminal, ret.Status == "RETURNED", ret.Sequence <= 16, ret.Kind == "HOST_OBSERVED" || ret.Kind == "SCRIPTED_CONTRACT", "OBSERVABLE_HASH_IDENTITY_NOT_AUTHENTICATION_CAUSE_UNKNOWN"}
	if applicationFinalizeCheck(lanes...) != nil {
		fmt.Fprintln(errout, "private application HOST: FINALIZE_SLICE_EXPIRED_CHARGED")
		return 1
	}
	raw, e := json.Marshal(record)
	relative := fmt.Sprintf("host-refusal-%s-%d-%s.json", ticket[7:], index, reason)
	if e == nil && len(raw) <= 4096 {
		e = applicationFinalizePublish(root, relative, raw, true, lanes...)
	}
	status := "PUBLISHED"
	if e != nil || len(raw) > 4096 || applicationFinalizeCheck(lanes...) != nil {
		status = "PUBLICATION_FAILED"
		relative = ""
	}
	response := struct{ Schema, Class, Reason, Path, DiagnosticStatus string }{"private-application-finalize-refusal.v1", "HOST_RETURN_REFUSED", reason, relative, status}
	if encodeHostProductV3(out, response) != 0 {
		fmt.Fprintln(errout, "private application HOST: HOST_RETURN_REFUSED_DIAGNOSTIC_OUTPUT_FAILED")
		return 1
	}
	if status != "PUBLISHED" {
		fmt.Fprintln(errout, "private application HOST: HOST_RETURN_REFUSED_DIAGNOSTIC_PUBLICATION_FAILED")
		return 1
	}
	fmt.Fprintln(errout, "private application HOST: HOST_RETURN_REFUSED")
	return 1
}
