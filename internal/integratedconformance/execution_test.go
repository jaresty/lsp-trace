package integratedconformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
	"lsp-trace/internal/verification"
)

const executionPerturbation = "LSP_TRACE_EXECUTION_CONFORMANCE_PERTURB"

const (
	assertSourceOrigin = "ASSERT_EXECUTION_SOURCE_ORIGIN"
	assertPathSuccess  = "ASSERT_EXECUTION_PATH_SUCCESS"
	assertParity       = "ASSERT_EXECUTION_LOGICAL_PARITY"
	assertOffline      = "ASSERT_EXECUTION_OFFLINE_VERIFY"
	assertReplayLogic  = "ASSERT_EXECUTION_REPLAY_LOGICAL"
	assertReplayBytes  = "ASSERT_EXECUTION_REPLAY_BYTES"
	assertFailureStage = "ASSERT_EXECUTION_FAILURE_STAGE"
	assertFailureStops = "ASSERT_EXECUTION_FAILURE_STOPS"
	assertFrozenV2     = "ASSERT_EXECUTION_FROZEN_V2"
	assertFrozenV3     = "ASSERT_EXECUTION_FROZEN_V3"
)

type executionInput struct {
	Root   string `json:"root"`
	Source string `json:"source"`
	FailAt string `json:"fail_at,omitempty"`
}

type executionOutput struct {
	Response operation.CustodyResponse `json:"response"`
	Failure  *executionFailure         `json:"failure,omitempty"`
	Artifact string                    `json:"artifact,omitempty"`
	Receipt  string                    `json:"receipt,omitempty"`
}

type executionFailure struct {
	Code  string                 `json:"code"`
	Stage operation.CustodyStage `json:"stage"`
}

type normalizedExecution struct {
	LogicalDigest string                                           `json:"logical_digest"`
	Lifecycle     []operation.StageRecord                          `json:"lifecycle"`
	Results       map[operation.CustodyStage]operation.StageResult `json:"results"`
	ArtifactHash  string                                           `json:"artifact_hash"`
	ArtifactSize  int                                              `json:"artifact_size"`
}

func TestEndToEndExecutionConformance(t *testing.T) {
	root := repositoryRoot(t)
	beforeV2 := mustRead(t, filepath.Join(root, "internal/graph/testdata/historical_identity_v2.json"))
	beforeV3 := mustRead(t, filepath.Join(root, "internal/graph/testdata/historical_identity_v3.json"))
	const sourceText = "package fixture\nfunc Hermetic() {}\n"
	paths := []string{"direct", "cli", "mcp"}
	first := make(map[string]executionOutput, len(paths))
	for _, path := range paths {
		out := runExecutionPath(t, path, executionInput{Root: t.TempDir(), Source: sourceText})
		if os.Getenv(executionPerturbation) == assertPathSuccess && path == "cli" {
			out.Failure = &executionFailure{Code: "PERTURBED", Stage: operation.StageDiscovery}
		}
		if out.Failure != nil {
			t.Fatalf("%s: path=%s failure=%+v", assertPathSuccess, path, out.Failure)
		}
		artifact := mustRead(t, out.Artifact)
		if os.Getenv(executionPerturbation) == assertSourceOrigin && path == "direct" {
			artifact = bytes.ReplaceAll(artifact, []byte(sourceDigest(sourceText)), []byte(sourceDigest("wrong source")))
		}
		if !bytes.Contains(artifact, []byte(sourceDigest(sourceText))) {
			t.Fatalf("%s: path=%s artifact does not retain source digest", assertSourceOrigin, path)
		}
		verifyOffline(t, assertOffline, path, out, artifact)
		first[path] = out
	}

	want := normalizeExecution(t, first["direct"])
	for _, path := range paths[1:] {
		got := normalizeExecution(t, first[path])
		if os.Getenv(executionPerturbation) == assertParity && path == "mcp" {
			got.LogicalDigest = "perturbed"
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: path=%s\nwant=%+v\ngot=%+v", assertParity, path, want, got)
		}
	}

	for _, path := range paths {
		second := runExecutionPath(t, path, executionInput{Root: t.TempDir(), Source: sourceText})
		firstNorm, secondNorm := normalizeExecution(t, first[path]), normalizeExecution(t, second)
		if os.Getenv(executionPerturbation) == assertReplayLogic && path == "direct" {
			secondNorm.LogicalDigest = "perturbed"
		}
		if !reflect.DeepEqual(firstNorm, secondNorm) {
			t.Fatalf("%s: path=%s first=%+v second=%+v", assertReplayLogic, path, firstNorm, secondNorm)
		}
		firstBytes, secondBytes := mustRead(t, first[path].Artifact), mustRead(t, second.Artifact)
		if os.Getenv(executionPerturbation) == assertReplayBytes && path == "cli" {
			secondBytes = append(secondBytes, '\n')
		}
		assertBytes(t, assertReplayBytes, firstBytes, secondBytes)
	}

	for _, stage := range []operation.CustodyStage{operation.StageDiscovery, operation.StageReceipt, operation.StageManifest, operation.StageSnapshot, operation.StageAdmission, operation.StagePublication} {
		failureRoot := t.TempDir()
		out := executeHermetic(context.Background(), executionInput{Root: failureRoot, Source: sourceText, FailAt: string(stage)})
		wantStage := stage
		if os.Getenv(executionPerturbation) == assertFailureStage && stage == operation.StageManifest {
			wantStage = operation.StageReceipt
		}
		if out.Failure == nil || out.Failure.Stage != wantStage {
			t.Fatalf("%s: injected=%s failure=%+v want=%s", assertFailureStage, stage, out.Failure, wantStage)
		}
		failed := false
		for _, record := range out.Response.Lifecycle {
			if record.Stage == stage {
				failed = true
				if record.State != operation.LifecycleFailed {
					t.Fatalf("%s: stage=%s state=%s", assertFailureStops, stage, record.State)
				}
				continue
			}
			if failed && record.State != operation.LifecycleSkipped {
				t.Fatalf("%s: failed=%s later=%s state=%s", assertFailureStops, stage, record.Stage, record.State)
			}
		}
		entries, err := os.ReadDir(failureRoot)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv(executionPerturbation) == assertFailureStops && stage == operation.StageAdmission {
			entries = append(entries, syntheticDirEntry("perturbed"))
		}
		if len(entries) != 0 {
			t.Fatalf("%s: stage=%s publication entries=%d", assertFailureStops, stage, len(entries))
		}
	}

	afterV2 := mustRead(t, filepath.Join(root, "internal/graph/testdata/historical_identity_v2.json"))
	afterV3 := mustRead(t, filepath.Join(root, "internal/graph/testdata/historical_identity_v3.json"))
	if os.Getenv(executionPerturbation) == assertFrozenV2 {
		afterV2 = append(afterV2, '\n')
	}
	if os.Getenv(executionPerturbation) == assertFrozenV3 {
		afterV3 = append(afterV3, '\n')
	}
	assertBytes(t, assertFrozenV2, beforeV2, afterV2)
	assertBytes(t, assertFrozenV3, beforeV3, afterV3)

	for _, assertion := range []string{assertPathSuccess, assertSourceOrigin, assertParity, assertOffline, assertReplayLogic, assertReplayBytes, assertFailureStage, assertFailureStops, assertFrozenV2, assertFrozenV3} {
		t.Log("PASS " + assertion)
	}
}

func TestEndToEndExecutionHelper(t *testing.T) {
	if os.Getenv("LSP_TRACE_EXECUTION_HELPER") != "1" {
		return
	}
	var request struct {
		Mode    string          `json:"mode"`
		Input   executionInput  `json:"input"`
		JSONRPC string          `json:"jsonrpc,omitempty"`
		ID      int             `json:"id,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if request.Mode == "mcp" {
		if err := json.Unmarshal(request.Params, &request.Input); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	out := executeHermetic(context.Background(), request.Input)
	encoder := json.NewEncoder(os.Stdout)
	if request.Mode == "mcp" {
		_ = encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": out})
	} else {
		_ = encoder.Encode(out)
	}
	os.Exit(0)
}

func runExecutionPath(t *testing.T, mode string, input executionInput) executionOutput {
	t.Helper()
	if mode == "direct" {
		return executeHermetic(context.Background(), input)
	}
	request := map[string]any{"mode": mode, "input": input}
	if mode == "mcp" {
		params, _ := json.Marshal(input)
		request = map[string]any{"mode": mode, "jsonrpc": "2.0", "id": 1, "params": json.RawMessage(params)}
	}
	encoded, _ := json.Marshal(request)
	cmd := exec.Command(os.Args[0], "-test.run=^TestEndToEndExecutionHelper$")
	cmd.Env = append(os.Environ(), "LSP_TRACE_EXECUTION_HELPER=1")
	cmd.Stdin = bytes.NewReader(encoded)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s helper: %v stderr=%s", mode, err, stderr.String())
	}
	var out executionOutput
	if mode == "mcp" {
		var envelope struct {
			Result executionOutput `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		out = envelope.Result
	} else if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func executeHermetic(ctx context.Context, input executionInput) executionOutput {
	logicalInput, _ := json.Marshal(struct {
		Source string `json:"source"`
	}{input.Source})
	var artifact, receipt []byte
	root, err := publication.OpenRoot(input.Root)
	if err != nil {
		return executionOutput{Failure: &executionFailure{Code: publication.CodePublicationFailed, Stage: operation.StagePublication}}
	}
	defer root.Close()
	fail := operation.CustodyStage(input.FailAt)
	handlers := make(map[operation.CustodyStage]operation.CustodyStageHandler)
	for _, stage := range []operation.CustodyStage{operation.StageDiscovery, operation.StageReceipt, operation.StageManifest, operation.StageSnapshot, operation.StageAdmission, operation.StagePublication} {
		stage := stage
		handlers[stage] = func(_ context.Context, _ operation.CustodyRequest, response operation.CustodyResponse) (operation.StageResult, error) {
			if fail == stage {
				return operation.StageResult{}, fmt.Errorf("injected %s failure", stage)
			}
			switch stage {
			case operation.StageDiscovery:
				return jsonStage(map[string]any{"locator": "hermetic/input.go", "source_digest": sourceDigest(input.Source)})
			case operation.StageReceipt:
				_, _, encoded, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: "source-1", Locator: "hermetic/input.go"}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "hermetic", Locator: "hermetic/input.go"}}, []byte(input.Source))
				if err != nil {
					return operation.StageResult{}, err
				}
				return operation.StageResult{Artifact: encoded}, nil
			case operation.StageManifest:
				digest := sourceDigest(input.Source)
				encoded, err := source.AssembleManifest([]source.ManifestReceipt{{ID: "source-1", Path: "input.go", Digest: digest}}, []source.ManifestDecision{{ReceiptID: "source-1", State: source.ManifestInclude}})
				if err != nil {
					return operation.StageResult{}, err
				}
				artifact = encoded
				return operation.StageResult{Artifact: encoded}, nil
			case operation.StageSnapshot:
				return jsonStage(map[string]any{"snapshot_identity": sourceDigest(string(artifact))})
			case operation.StageAdmission:
				admitted := true
				result, err := jsonStage(map[string]any{"status": "MISSING_TRUST"})
				result.Admitted = &admitted
				return result, err
			case operation.StagePublication:
				receipt, err = verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
				if err != nil {
					return operation.StageResult{}, err
				}
				pub := publication.NewPublisher()
				if result := pub.Publish(publication.Request{Root: root, Selector: "artifact.json", Bytes: artifact, ArtifactSchemaID: "lsp-trace.source-custody-manifest.v1"}); result.Err() != nil {
					return operation.StageResult{}, result.Err()
				}
				if result := pub.Publish(publication.Request{Root: root, Selector: "receipt.json", Bytes: receipt, ArtifactSchemaID: "lsp-trace.publication-receipt.v1"}); result.Err() != nil {
					return operation.StageResult{}, result.Err()
				}
				return jsonStage(map[string]any{"artifact": "artifact.json", "receipt": "receipt.json", "digest": sourceDigest(string(artifact)), "byte_length": len(artifact)})
			}
			return operation.StageResult{}, fmt.Errorf("unknown stage")
		}
	}
	response, failure := (&operation.CustodyOperation{Handlers: handlers}).ExecuteCustody(ctx, operation.CustodyRequest{OperationID: "hermetic-custody", Input: logicalInput})
	out := executionOutput{Response: response}
	if failure != nil {
		out.Failure = &executionFailure{Code: failure.Code, Stage: failure.Stage}
		return out
	}
	out.Artifact = filepath.Join(input.Root, "artifact.json")
	out.Receipt = filepath.Join(input.Root, "receipt.json")
	return out
}

func jsonStage(value any) (operation.StageResult, error) {
	encoded, err := json.Marshal(value)
	return operation.StageResult{Artifact: encoded}, err
}

func normalizeExecution(t *testing.T, out executionOutput) normalizedExecution {
	t.Helper()
	artifact := mustRead(t, out.Artifact)
	results := make(map[operation.CustodyStage]operation.StageResult, len(out.Response.Results))
	for stage, result := range out.Response.Results {
		if len(result.Artifact) > 0 {
			var compact bytes.Buffer
			if err := json.Compact(&compact, result.Artifact); err != nil {
				t.Fatalf("normalize %s: %v", stage, err)
			}
			result.Artifact = compact.Bytes()
		}
		results[stage] = result
	}
	return normalizedExecution{LogicalDigest: out.Response.LogicalDigest, Lifecycle: out.Response.Lifecycle, Results: results, ArtifactHash: sourceDigest(string(artifact)), ArtifactSize: len(artifact)}
}

func verifyOffline(t *testing.T, assertion, path string, out executionOutput, artifact []byte) {
	t.Helper()
	receipt := mustRead(t, out.Receipt)
	if os.Getenv(executionPerturbation) == assertion && path == "mcp" {
		receipt = append(receipt, 'x')
	}
	if err := verification.VerifyReceipt(artifact, receipt); err != nil {
		t.Fatalf("%s: path=%s: %v", assertion, path, err)
	}
}

func sourceDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertBytes(t *testing.T, assertion string, want, got []byte) {
	t.Helper()
	if !bytes.Equal(want, got) {
		t.Fatalf("%s: exact bytes differ", assertion)
	}
}

type namedDirEntry string

func syntheticDirEntry(name string) os.DirEntry  { return namedDirEntry(name) }
func (e namedDirEntry) Name() string             { return string(e) }
func (namedDirEntry) IsDir() bool                { return false }
func (namedDirEntry) Type() os.FileMode          { return 0 }
func (namedDirEntry) Info() (os.FileInfo, error) { return nil, fmt.Errorf("synthetic") }
