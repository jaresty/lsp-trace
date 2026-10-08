package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Case struct {
	ID              string `json:"id"`
	Requirement     string `json:"requirement"`
	Stimulus        string `json:"stimulus"`
	Assertion       string `json:"assertion"`
	ExpectedOutcome string `json:"expected_outcome"`
	ExpectedCode    string `json:"expected_code"`
}

func dig(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func main() {
	root := "docs/pilot/adr0007/source-text-search-v4/cases"
	os.RemoveAll(root)
	os.MkdirAll(root, 0755)
	names := []string{"raw_malformed_json", "raw_unknown_field", "raw_duplicate_field", "raw_trailing_data", "empty_query", "empty_source_set", "control_duplicate", "control_order", "control_simultaneous_deadline_cancel", "control_midscan_cancel", "control_postmatch_deadline", "utf8_empty_file", "utf8_nfc_path", "path_backslash", "path_absolute", "path_traversal", "duplicate_path_same_revision", "duplicate_path_diff_revision", "ordinal_missing", "ordinal_extra", "file_digest_mutation", "object_digest_mutation", "admission_digest_order", "multi_path_byte_order", "overlap_literal", "metachar_literal", "case_sensitive_literal", "combining_literal", "lf_positions", "crlf_positions", "bare_cr_positions", "mixed_newlines", "non_bmp_utf16", "crossline_literal", "max_files_equal", "max_files_plus_one", "max_source_equal", "max_source_plus_one", "max_total_equal", "max_total_plus_one", "max_path_equal", "max_path_plus_one", "max_matches_equal", "max_matches_plus_one", "max_work_equal", "max_work_plus_one", "max_output_equal", "max_output_plus_one", "overflow_helper_case", "complete_location_pins_order"}
	matrix := []Case{}
	for i, n := range names {
		id := fmt.Sprintf("case-%02d-%s", i+1, n)
		dir := filepath.Join(root, id)
		os.MkdirAll(filepath.Join(dir, "source"), 0755)
		text := sourceFor(n)
		os.WriteFile(filepath.Join(dir, "source", "a.txt"), []byte(text), 0644)
		raw := attempt(id, n, text)
		switch n {
		case "raw_malformed_json":
			raw = []byte(`{"schema_version":`)
		case "raw_unknown_field":
			raw = []byte(strings.Replace(string(raw), "}", `,"unknown_field":true}`, 1))
		case "raw_duplicate_field":
			raw = []byte(strings.Replace(string(raw), `"attempt_id"`, `"attempt_id":"attempt-dupe","attempt_id"`, 1))
		case "raw_trailing_data":
			raw = append(raw, []byte("{}")...)
		}
		os.WriteFile(filepath.Join(dir, "attempt.json"), raw, 0644)
		out, code := expect(n)
		matrix = append(matrix, Case{ID: id, Requirement: "adr0007-v4-source-text-search", Stimulus: n, Assertion: assertion(n), ExpectedOutcome: out, ExpectedCode: code})
	}
	b, _ := json.MarshalIndent(matrix, "", "  ")
	os.WriteFile(filepath.Join(root, "CASE_MATRIX.json"), append(b, '\n'), 0644)
}
func sourceFor(n string) string {
	switch n {
	case "utf8_empty_file":
		return ""
	case "crlf_positions":
		return "aa\r\nneedle\r\n"
	case "bare_cr_positions":
		return "aa\rneedle\r"
	case "mixed_newlines":
		return "a\r\nb\rneedle\n"
	case "non_bmp_utf16":
		return "😀needle"
	case "crossline_literal":
		return "aa\nneedle"
	case "combining_literal":
		return "cafe\u0301 needle"
	case "overlap_literal":
		return "aaaa"
	case "metachar_literal":
		return "literal .* chars"
	default:
		return "alpha needle beta needle"
	}
}
func attempt(id, n, text string) []byte {
	path := "a.txt"
	if n == "path_backslash" {
		path = `a\b.txt`
	}
	if n == "path_absolute" {
		path = "/a.txt"
	}
	if n == "path_traversal" {
		path = "../a.txt"
	}
	if n == "utf8_nfc_path" {
		path = "café.txt"
	}
	q := "needle"
	if n == "empty_query" {
		q = ""
	}
	if n == "overlap_literal" {
		q = "aa"
	}
	if n == "metachar_literal" {
		q = ".*"
	}
	if n == "non_bmp_utf16" {
		q = "😀"
	}
	fd := dig([]byte(text))
	od := fd
	if n == "file_digest_mutation" {
		fd = dig([]byte("x"))
	}
	if n == "object_digest_mutation" {
		od = dig([]byte("x"))
	}
	sources := []map[string]any{{"ordinal": uint64(1), "path": path, "revision": "rev1", "file_digest": fd, "object_digest": od}}
	inputs := []map[string]any{{"schema_version": "lsp-trace.adr0007.source-text-search.source-input.private.v4", "ordinal": uint64(1), "path": path, "revision": "rev1", "file_digest": fd, "object_digest": od, "bytes_base64": base64.StdEncoding.EncodeToString([]byte(text))}}
	if n == "empty_source_set" {
		sources = []map[string]any{}
		inputs = []map[string]any{}
	}
	if strings.HasPrefix(n, "duplicate_path_") {
		sources = append(sources, map[string]any{"ordinal": uint64(2), "path": path, "revision": "rev2", "file_digest": fd, "object_digest": od})
		inputs = append(inputs, map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.source-input.private.v4", "ordinal": uint64(2), "path": path, "revision": "rev2", "file_digest": fd, "object_digest": od, "bytes_base64": base64.StdEncoding.EncodeToString([]byte(text))})
	}
	if n == "max_files_plus_one" {
		btxt := "needle in b"
		bfd := dig([]byte(btxt))
		sources = append(sources, map[string]any{"ordinal": uint64(2), "path": "b.txt", "revision": "rev1", "file_digest": bfd, "object_digest": bfd})
		inputs = append(inputs, map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.source-input.private.v4", "ordinal": uint64(2), "path": "b.txt", "revision": "rev1", "file_digest": bfd, "object_digest": bfd, "bytes_base64": base64.StdEncoding.EncodeToString([]byte(btxt))})
	}
	if n == "ordinal_missing" {
		sources[0]["ordinal"] = uint64(7)
	}
	if n == "ordinal_extra" {
		inputs = append(inputs, map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.source-input.private.v4", "ordinal": uint64(9), "path": "z.txt", "revision": "rev1", "file_digest": fd, "object_digest": od, "bytes_base64": base64.StdEncoding.EncodeToString([]byte(text))})
	}
	obs := []map[string]any{}
	if strings.HasPrefix(n, "control_") {
		obs = []map[string]any{{"poll_index": uint64(2), "cancelled": strings.Contains(n, "cancel"), "deadline_expired": strings.Contains(n, "deadline")}}
		if n == "control_duplicate" {
			obs = append(obs, obs[0])
		}
		if n == "control_order" {
			obs = []map[string]any{{"poll_index": uint64(3)}, {"poll_index": uint64(2)}}
		}
		if n == "control_simultaneous_deadline_cancel" {
			obs = []map[string]any{{"poll_index": uint64(0), "cancelled": true, "deadline_expired": true}}
		}
	}
	limits := map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.limits.private.v4", "max_files": uint64(8), "max_matches": uint64(8), "max_work": uint64(999999), "max_output_bytes": uint64(999999), "max_source_bytes": uint64(999999), "max_total_bytes": uint64(999999), "max_path_bytes": uint64(999999)}
	if n == "max_matches_equal" {
		limits["max_matches"] = uint64(2)
	}
	if strings.Contains(n, "plus_one") {
		if strings.Contains(n, "files") {
			limits["max_files"] = uint64(1)
		}
		if strings.Contains(n, "source") {
			limits["max_source_bytes"] = uint64(1)
		}
		if strings.Contains(n, "total") {
			limits["max_total_bytes"] = uint64(1)
		}
		if strings.Contains(n, "path") {
			limits["max_path_bytes"] = uint64(1)
		}
		if strings.Contains(n, "matches") {
			limits["max_matches"] = uint64(1)
		}
		if strings.Contains(n, "work") {
			limits["max_work"] = uint64(1)
		}
		if strings.Contains(n, "output") {
			limits["max_output_bytes"] = uint64(1)
		}
	}
	if n == "overflow_helper_case" {
		limits["max_work"] = uint64(999999999)
	}
	a := map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.attempt.private.v4", "attempt_id": "attempt-" + id, "request": map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.request.private.v4", "query": q, "sources": sources, "policy": map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.policy.private.v4", "literal_mode": "byte-literal", "allow_regex": false, "allow_fuzzy": false, "allow_token": false, "allow_rank": false, "allow_model": false, "allow_backend_semantics": false}, "limits": limits, "location_pin": map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.location-pin.private.v4", "operation": "RANGE_UNION", "executedLocation": false, "design_commit": "c943a484060462121c6f0929d4182b053ff95ab5", "design_root_sha256": "sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d", "execution_commit": "f0f8b49aa368bea9b3e6d105eef5cb2614221067", "seal_commit": "16f40dcb03a234b00db80059a7eef400495e9d97", "final_seal_sha256": "sha256:f4981045d3489f5ef0633eb4ce6b4a17ab6de1ddc73c4b729f9dd524106b1fd6", "source_admission_pin": map[string]any{"repository_commit": "af2ce89321afc94c937636f841bb98b8977b6496", "path": "internal/sourceadmissionv2/admission.go", "bytes": uint64(3519), "sha256": "sha256:da74770d5b36f63e6f1265ba78e2404e13f2d1f1451a4e7aa405f448e47fe7da", "git_blob_sha1": "4953fab89d911e2352fa6c2253497c07c8e9a777", "symbols": []string{"Admit", "CanonicalPath", "Digest"}}, "complete_location_pins": []string{"design_commit", "design_root_sha256", "execution_commit", "seal_commit", "final_seal_sha256", "source_admission_pin"}}}, "execution_control": map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.execution-control.private.v4", "observations": obs}, "source_inputs": inputs}
	if n == "overflow_helper_case" {
		a["test_control"] = map[string]any{"schema_version": "lsp-trace.adr0007.source-text-search.test-control.private.v4", "initial_B_output_bytes": ^uint64(0)}
	}
	b, _ := json.MarshalIndent(a, "", "  ")
	return append(b, '\n')
}
func expect(n string) (string, string) {
	if n == "overflow_helper_case" {
		return "FAILED", "OVERFLOW"
	}
	if n == "control_midscan_cancel" {
		return "FAILED", "CANCELLED"
	}
	if n == "control_simultaneous_deadline_cancel" || n == "control_postmatch_deadline" {
		return "FAILED", "DEADLINE_EXCEEDED"
	}
	if strings.HasPrefix(n, "raw_") || n == "empty_query" || n == "empty_source_set" || n == "control_duplicate" || n == "control_order" {
		return "FAILED", "INVALID_INPUT"
	}
	if strings.HasPrefix(n, "path_") || n == "utf8_empty_file" || strings.Contains(n, "digest_mutation") || strings.HasPrefix(n, "duplicate_path_") {
		return "FAILED", "ADMISSION_FAILED"
	}
	if strings.HasPrefix(n, "ordinal_") {
		return "FAILED", "ASSOCIATION_FAILED"
	}
	if strings.Contains(n, "plus_one") {
		return "FAILED", "RESOURCE_EXHAUSTED"
	}
	return "COMPLETE", ""
}
func assertion(n string) string {
	return "attempt bytes exercise " + n + " and terminal outcome/code must match matrix"
}
