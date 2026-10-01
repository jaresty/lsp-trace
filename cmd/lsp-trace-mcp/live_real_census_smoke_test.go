package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"lsp-trace/internal/censusrequest"
	"time"
)

const liveRealCensusFingerprint = "sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9"

func TestLiveRealCensusSmoke(t *testing.T) {
	if os.Getenv("LSP_TRACE_LIVE_REAL_SMOKE") != "1" {
		t.Skip("set LSP_TRACE_LIVE_REAL_SMOKE=1 to run")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("real managed gopls smoke requires darwin")
	}
	root := canonicalRepoRoot(t)
	gopls := os.Getenv("LSP_TRACE_GOPLS_PATH")
	if !filepath.IsAbs(gopls) {
		t.Fatal("ASSERT_LIVE_REAL_GOPLS_ABSOLUTE_PATH")
	}
	if info, err := os.Stat(gopls); err != nil || info.Mode()&0111 == 0 {
		t.Fatalf("ASSERT_LIVE_REAL_GOPLS_EXECUTABLE: %v", err)
	}
	evidence := requireEvidenceRoot(t)
	candidate := buildMCPBinary(t)
	candidateDigest := fileDigest(t, candidate)
	publication, continuation := liveRealRetainedStores(t, evidence)
	diagnostic := freshFile(t, "acquisition.ndjson")
	managed := freshFile(t, "managed.ndjson")
	trace := freshFile(t, "runtime.trace")
	bootstrap := freshFile(t, "bootstrap.json")
	config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "live-real-gopls", "language_id": "go", "profile": map[string]any{"trust_domain": "live-real-smoke", "workspace": root, "profile": "gopls", "environment_reference": "diagnostic-rehearsal"}, "execution": map[string]any{"path": gopls, "directory": root}}}, "continuation": map[string]any{"publication_root": continuation, "max_object_bytes": 64 << 20, "capabilities": []string{"STOP_AFTER_DESCRIBE_REQUESTS"}, "managed_preparation_diagnostic_path": managed}}
	write0600(t, bootstrap, mustMarshal(t, config))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, candidate, liveRealCandidateArgs(bootstrap, publication, diagnostic)...)
	cmd.Env = append(os.Environ(), "LSP_TRACE_GOPLS_PATH="+gopls, "LSP_TRACE_GO_RUNTIME_TRACE_PATH="+trace)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("ASSERT_LIVE_REAL_STDIN: %v", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("ASSERT_LIVE_REAL_STDOUT: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("ASSERT_LIVE_REAL_START: %v", err)
	}
	reader := bufio.NewReader(out)
	result := runLiveRealAfterStart(cmd, ctx, in, reader, &stderr, func(send liveRealSend) (liveRealPhaseResult, error) {
		list, err := send(1, "lsp_session_v1_list", map[string]any{"detail": "full"})
		if err != nil {
			return liveRealPhaseResult{}, liveRealPhaseError{"before-session-list", err}
		}
		session, generation, err := exactReadySessionValue(list, root)
		if err != nil {
			return liveRealPhaseResult{}, liveRealPhaseError{"session-list", err}
		}

		templatePath := filepath.Join(root, ".pi", "evidence", "acquisition-real-code-qualification-20260922T044000Z", "REQUEST.template.json")
		template, qualificationFingerprint, err := loadLiveRealTemplate(templatePath, session, generation)
		if err != nil {
			return liveRealPhaseResult{}, liveRealPhaseError{"template", err}
		}
		if qualificationFingerprint != liveRealPinnedSemantic {
			return liveRealPhaseResult{}, liveRealPhaseError{"qualification", errors.New("semantic fingerprint mismatch")}
		}
		staged := liveRealStagedRequest(template)
		staged["session_id"], staged["generation"] = session, generation
		response, err := send(2, "lsp_trace_v1_census", staged)
		decoded, decodeErr := decodeDirectEnvelope(response)
		pr := liveRealPhaseResult{response: decoded, semantic: qualificationFingerprint}
		if err != nil {
			return pr, liveRealPhaseError{"before-census", err}
		}
		if decodeErr != nil {
			return pr, liveRealPhaseError{"census-terminal", decodeErr}
		}
		if status, stage, ok := liveRealCensusAdmission(decoded); !ok {
			phase := "census-terminal"
			if liveRealCensusFailureCode(decoded) == "ACQUISITION_FAILED" {
				phase = "census-acquisition"
			}
			return pr, liveRealPhaseError{phase, fmt.Errorf("census terminal %s/%s/%s", status, stage, liveRealCensusAdmissionInvariant(decoded))}
		}
		return pr, nil
	}, liveRealRunInput{evidenceDir: evidence, tracePath: trace, diagnosticPath: diagnostic, candidateDigest: candidateDigest, semanticFingerprint: liveRealPinnedSemantic})
	if result.err != nil {
		t.Error(result.err)
	}
	if result.failureClass != "SUCCESS" || result.terminal != "SUCCEEDED/PAUSED/DESCRIBE_REQUESTS" || result.failedField != "" || result.failedInvariant != "" || result.response == nil {
		t.Fatalf("ASSERT_LIVE_REAL_RETAINED_SUCCESS: class=%q terminal=%q field=%q invariant=%q response=%v", result.failureClass, result.terminal, result.failedField, result.failedInvariant, result.response != nil)
	}
	t.Logf("LIVE_REAL_SUMMARY candidate_sha256=%s qualification_semantic_fingerprint=%s runtime_request_fingerprint=%s terminal=%s failure_class=%s failed_field=%s failed_invariant=%s evidence=%s", candidateDigest, result.semantic, result.runtimeFingerprint, result.terminal, result.failureClass, result.failedField, result.failedInvariant, evidence)
}

func TestLiveRealSmokeGateAndFingerprint(t *testing.T) {
	if liveRealCensusFingerprint != "sha256:246a63cddcc552144d60578a4f127e34eddb970f496f863cd6a3509011783cb9" {
		t.Fatal("ASSERT_LIVE_REAL_FINGERPRINT_VECTOR")
	}
	if censusrequest.DefaultBatchTargets != 63 || censusrequest.DefaultTimeoutMS != 60000 {
		t.Fatal("ASSERT_LIVE_REAL_VALIDATOR_DEFAULTS")
	}
}

func canonicalRepoRoot(t *testing.T) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_LIVE_REAL_REPO_ROOT_CALLER")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil || filepath.Clean(root) != "/Users/schwa/dev/lsp-trace" {
		t.Fatalf("ASSERT_LIVE_REAL_CANONICAL_WORKSPACE: %q", root)
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ASSERT_LIVE_REAL_WORKSPACE_CUSTODY")
	}
	return root
}
func requireEvidenceRoot(t *testing.T) string {
	p := os.Getenv("LSP_TRACE_LIVE_REAL_SMOKE_EVIDENCE_ROOT")
	if !filepath.IsAbs(p) {
		t.Fatal("ASSERT_LIVE_REAL_EVIDENCE_ROOT_ABSOLUTE")
	}
	i, e := os.Lstat(p)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm() != 0700 {
		t.Fatalf("ASSERT_LIVE_REAL_EVIDENCE_ROOT_CUSTODY: %v", e)
	}
	run := filepath.Join(p, fmt.Sprintf("run-%d", time.Now().UnixNano()))
	if e = os.Mkdir(run, 0700); e != nil {
		t.Fatal(e)
	}
	return run
}
func liveRealCandidateArgs(bootstrap, publication, acquisitionDiagnostic string) []string {
	return []string{"--enable-live-lsp", "--tool-profile", "advanced", "--bootstrap-config", bootstrap, "--publication-root", publication, "--acquisition-diagnostic-path", acquisitionDiagnostic}
}

func liveRealRetainedStores(t *testing.T, evidence string) (string, string) {
	publication, continuation, err := allocateLiveRealRetainedStores(evidence, "publication", "continuation")
	if err != nil {
		t.Fatalf("ASSERT_LIVE_REAL_RETAINED_STORES: %v", err)
	}
	return publication, continuation
}

func allocateLiveRealRetainedStores(evidence, publicationName, continuationName string) (string, string, error) {
	info, err := os.Lstat(evidence)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return "", "", fmt.Errorf("evidence root is not a real 0700 directory")
	}
	publication := filepath.Join(evidence, publicationName)
	continuation := filepath.Join(evidence, continuationName)
	if err := os.Mkdir(publication, 0700); err != nil {
		return "", "", fmt.Errorf("publication: %w", err)
	}
	if err := os.Mkdir(continuation, 0700); err != nil {
		return "", "", fmt.Errorf("continuation: %w", err)
	}
	return publication, continuation, nil
}

func freshDir(t *testing.T, n string) string {
	p := filepath.Join(t.TempDir(), n)
	if e := os.Mkdir(p, 0700); e != nil {
		t.Fatal(e)
	}
	return p
}
func freshFile(t *testing.T, n string) string { return filepath.Join(freshDir(t, n), n) }
func write0600(t *testing.T, p string, b []byte) {
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func mustMarshal(t *testing.T, v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func fileDigest(t *testing.T, p string) string {
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	d := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(d[:])
}
func requestReceiptFingerprint(response map[string]any) string {
	receipt, _ := response["request_receipt"].(map[string]any)
	fingerprint, _ := receipt["fingerprint"].(string)
	return fingerprint
}

func liveRealCensusFailureCode(response map[string]any) string {
	err, _ := response["error"].(map[string]any)
	code, _ := err["code"].(string)
	return code
}

const liveRealResumeGuidance = "Resume with this selector and omit stop_after to continue exactly the remaining work once."

var liveRealSelectorPattern = regexp.MustCompile(`^g-[0-9a-f]{64}\.selector\.json$`)
var liveRealDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func liveRealCensusAdmission(response map[string]any) (string, string, bool) {
	status, stage, ok, _ := liveRealCensusAdmissionDetails(response)
	return status, stage, ok
}

func liveRealCensusAdmissionInvariant(response map[string]any) string {
	_, _, _, invariant := liveRealCensusAdmissionDetails(response)
	return invariant
}

func liveRealCensusInvariantField(invariant string) string {
	switch invariant {
	case "PAUSED_ENVELOPE", "ENVELOPE_SCHEMA":
		return "envelope"
	case "RESULT_OBJECT", "RESULT_SCHEMA", "REQUEST_COUNT":
		return "result"
	case "CATALOG_CONTRACT", "SELECTOR_GRAMMAR", "SELECTOR_EQUALITY", "PREPARATION_COUNT_BOUNDED":
		return "catalog"
	case "CENSUS_XOR_IDENTITY", "IDENTITY_BINDING":
		return "census_identity"
	default:
		return "result"
	}
}

func liveRealCensusAdmissionDetails(response map[string]any) (string, string, bool, string) {
	status, _ := response["operation_status"].(string)
	outcome, _ := response["outcome"].(string)
	isError, _ := response["isError"].(bool)
	err, _ := response["error"].(map[string]any)
	stage, _ := err["stage"].(string)
	if stage == "" {
		stage, _ = response["stage"].(string)
	}
	if liveRealCensusFailureCode(response) == "ACQUISITION_FAILED" && stage == "acquisition" && isError {
		return status, stage, false, "ACQUISITION_COMPLETES"
	}
	if isError || status != "SUCCEEDED" || outcome != "PAUSED" {
		return status, stage, false, "PAUSED_ENVELOPE"
	}
	if response["envelope_schema_id"] != "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-feature-catalog-result.v2.schema.json" {
		return status, stage, false, "ENVELOPE_SCHEMA"
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		return status, stage, false, "RESULT_OBJECT"
	}
	if result["schema_version"] != "lsp-trace.census-feature-catalog-result.v2" {
		return status, stage, false, "RESULT_SCHEMA"
	}
	catalog, ok := result["catalog"].(map[string]any)
	if !ok {
		return status, stage, false, "CATALOG_CONTRACT"
	}
	if catalog["kind"] != "ADR_0007_FEATURE_CATALOG" || catalog["status"] != "PAUSED" || catalog["authority"] != float64(0) || catalog["accepted"] != false || catalog["completeness"] != "UNKNOWN" || catalog["resume_guidance"] != liveRealResumeGuidance {
		return status, stage, false, "CATALOG_CONTRACT"
	}
	requestCount, ok := catalog["request_count"].(float64)
	if !ok || requestCount < 1 || requestCount != float64(int(requestCount)) {
		return status, stage, false, "REQUEST_COUNT"
	}
	var catalogSelector string
	for _, key := range []string{"checkpoint_selector", "composite_selector", "catalog_selector"} {
		selector, ok := catalog[key].(string)
		if !ok || !liveRealSelectorPattern.MatchString(selector) {
			return status, stage, false, "SELECTOR_GRAMMAR"
		}
		if catalogSelector == "" {
			catalogSelector = selector
		} else if selector != catalogSelector {
			return status, stage, false, "SELECTOR_EQUALITY"
		}
	}
	preparationCount, ok := catalog["preparation_count"].(float64)
	if !ok || preparationCount < 0 || preparationCount > requestCount || preparationCount != float64(int(preparationCount)) {
		return status, stage, false, "PREPARATION_COUNT_BOUNDED"
	}
	census, censusPresent := result["census"]
	identity, identityPresent := result["census_identity"]
	censusNonNil := censusPresent && census != nil
	identityNonNil := identityPresent && identity != nil
	if censusNonNil == identityNonNil {
		return status, stage, false, "CENSUS_XOR_IDENTITY"
	}
	if identityNonNil {
		identityMap, ok := identity.(map[string]any)
		if !ok {
			return status, stage, false, "IDENTITY_BINDING"
		}
		selector, selectorOK := identityMap["selector"].(string)
		digest, digestOK := identityMap["digest"].(string)
		byteLength, byteLengthOK := identityMap["byte_length"].(float64)
		if !selectorOK || selector != catalogSelector || !liveRealSelectorPattern.MatchString(selector) || !digestOK || !liveRealDigestPattern.MatchString(digest) || !byteLengthOK || byteLength <= 0 || byteLength != float64(int(byteLength)) {
			return status, stage, false, "IDENTITY_BINDING"
		}
	}
	return status, "describe-requests", true, ""
}

func terminalStatus(raw []byte) string {
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return "INVALID"
	}
	if _, _, ok := liveRealCensusAdmission(v); ok {
		return "SUCCEEDED/PAUSED/DESCRIBE_REQUESTS"
	}
	stage, _ := v["stage"].(string)
	if stage == "" {
		if diagnostic, ok := v["diagnostic"].(map[string]any); ok {
			stage, _ = diagnostic["stage"].(string)
		}
	}
	if stage == "" {
		if publicError, ok := v["error"].(map[string]any); ok {
			stage, _ = publicError["stage"].(string)
		}
	}
	return fmt.Sprintf("%v/%v/%s", v["operation_status"], v["outcome"], stage)
}
func writeEvidence(t *testing.T, root string, summary map[string]any, trace string) {
	b := mustMarshal(t, summary)
	write0600(t, filepath.Join(root, "summary.json"), b)
	tb, _ := os.ReadFile(trace)
	write0600(t, filepath.Join(root, "runtime.trace"), tb)
	manifest := map[string]string{"summary.json": fileDigest(t, filepath.Join(root, "summary.json")), "runtime.trace": fileDigest(t, filepath.Join(root, "runtime.trace"))}
	write0600(t, filepath.Join(root, "manifest.json"), mustMarshal(t, manifest))
}
func exactReadySession(t *testing.T, response map[string]any) (string, uint64) {
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatal("ASSERT_LIVE_REAL_SESSION_RESULT")
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) < 1 {
		t.Fatal("ASSERT_LIVE_REAL_SESSION_CONTENT")
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatal("ASSERT_LIVE_REAL_SESSION_ITEM")
	}
	text, ok := item["text"].(string)
	if !ok {
		t.Fatal("ASSERT_LIVE_REAL_SESSION_TEXT")
	}
	var p struct {
		Result struct {
			Sessions []struct {
				SessionID  string `json:"SessionID"`
				Generation uint64 `json:"Generation"`
				State      string `json:"State"`
				Workers    int    `json:"Workers"`
			} `json:"Sessions"`
		} `json:"result"`
	}
	if e := json.Unmarshal([]byte(text), &p); e != nil {
		t.Fatalf("ASSERT_LIVE_REAL_SESSION_JSON: %v", e)
	}
	if len(p.Result.Sessions) != 1 || p.Result.Sessions[0].State != "READY" || p.Result.Sessions[0].SessionID == "" || p.Result.Sessions[0].Workers != 0 {
		t.Fatalf("ASSERT_LIVE_REAL_EXACT_READY: %s", text)
	}
	return p.Result.Sessions[0].SessionID, p.Result.Sessions[0].Generation
}
