package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

const (
	workerArgument             = "__lsp_trace_private_program_c_worker_v1"
	workerVersion              = "lsp-trace.private-program-c-worker.v1"
	MaxWorkerInputBytes  int64 = 64 << 20
	MaxWorkerOutputBytes int64 = 64 << 20
	MaxWorkerStderrBytes int64 = 64 << 10
	SupervisorWall             = 60 * time.Second
	SupervisorRSSBytes   int64 = 512 << 20
	SupervisorPoll             = 20 * time.Millisecond
)

// SupervisionObservation describes the qualified external observation mechanism.
// RSS is a 20ms sampled aggregate of /bin/ps snapshots of the descendant tree.
// It is not continuous kernel-backed aggregate process-tree memory enforcement;
// an allocation entirely between snapshots need not be observed.
type SupervisionObservation struct {
	ElapsedNanos     int64  `json:"elapsed_nanos"`
	PeakTreeRSSBytes int64  `json:"peak_tree_rss_bytes"`
	Samples          int    `json:"samples"`
	Ceiling          string `json:"ceiling"`
}

const ObservationCeiling = "Darwin arm64 external supervisor samples aggregate descendant process-tree RSS from /bin/ps every 20ms; this is not continuous kernel-backed aggregate process-tree memory enforcement and cannot detect an allocation wholly between snapshots."

type SupervisionCode string

const (
	CodePlatformUnsupported SupervisionCode = "PLATFORM_UNSUPPORTED"
	CodeTimeout             SupervisionCode = "TIMEOUT"
	CodeMemoryLimit         SupervisionCode = "MEMORY_LIMIT"
	CodeMemoryUnsupported   SupervisionCode = "MEMORY_UNSUPPORTED"
	CodeCancelled           SupervisionCode = "CANCELLED"
	CodeNonzeroExit         SupervisionCode = "NONZERO_EXIT"
	CodePanic               SupervisionCode = "PANIC"
	CodeMalformedOutput     SupervisionCode = "MALFORMED_OUTPUT"
	CodeOutputLimit         SupervisionCode = "OUTPUT_LIMIT"
	CodeInternalAdmission   SupervisionCode = "INTERNAL_ADMISSION"
	CodeInternalComputation SupervisionCode = "INTERNAL_COMPUTATION"
)

type SupervisionFailure struct {
	Code    SupervisionCode `json:"code"`
	Message string          `json:"message"`
}

func (f *SupervisionFailure) Error() string { return string(f.Code) + ": " + f.Message }

type workerRequest struct {
	Version string `json:"version"`
	Input   []byte `json:"input"`
	Seed    uint64 `json:"seed"`
}
type workerOutcome struct {
	Outcome       string      `json:"outcome"`
	ProfileID     string      `json:"profile_id"`
	ProfileDigest string      `json:"profile_digest"`
	Algorithm     string      `json:"algorithm"`
	LogicalDigest string      `json:"logical_digest"`
	ClaimCeiling  string      `json:"claim_ceiling"`
	Resolution    float64     `json:"resolution"`
	Seed          uint64      `json:"seed"`
	Communities   []Community `json:"communities"`
}
type workerResponse struct {
	Version string              `json:"version"`
	Outcome *workerOutcome      `json:"outcome,omitempty"`
	Failure *SupervisionFailure `json:"failure,omitempty"`
}

func failure(code SupervisionCode, err error) *SupervisionFailure {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	return &SupervisionFailure{Code: code, Message: message}
}

func strictDecode(r io.Reader, limit int64, v any) error {
	limited := &io.LimitedReader{R: r, N: limit + 1}
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	if limited.N <= 0 {
		return errors.New("input limit exceeded")
	}
	return nil
}

// RunPrivateWorker handles the unadvertised worker argument before public parsing.
func RunPrivateWorker(args []string, stdin io.Reader, stdout, stderr io.Writer) (bool, int) {
	if len(args) != 1 || args[0] != workerArgument {
		return false, 0
	}
	var request workerRequest
	if err := strictDecode(stdin, MaxWorkerInputBytes, &request); err != nil || request.Version != workerVersion {
		return true, writeWorkerResponse(stdout, workerResponse{Version: workerVersion, Failure: failure(CodeInternalAdmission, fmt.Errorf("invalid worker request"))})
	}
	out, failed := Compute(request.Input, request.Seed)
	if failed != nil {
		code := CodeInternalComputation
		if failed.Code != CodeComputation {
			code = CodeInternalAdmission
		}
		return true, writeWorkerResponse(stdout, workerResponse{Version: workerVersion, Failure: failure(code, failed)})
	}
	wire := workerOutcome{Outcome: out.Outcome, ProfileID: out.ProfileID, ProfileDigest: out.ProfileDigest, Algorithm: out.Algorithm, LogicalDigest: out.LogicalDigest, ClaimCeiling: out.ClaimCeiling, Resolution: out.Resolution, Seed: out.Seed, Communities: out.Communities}
	return true, writeWorkerResponse(stdout, workerResponse{Version: workerVersion, Outcome: &wire})
}

func writeWorkerResponse(w io.Writer, response workerResponse) int {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(response); err != nil || int64(b.Len()) > MaxWorkerOutputBytes {
		return 120
	}
	if _, err := w.Write(b.Bytes()); err != nil {
		return 121
	}
	return 0
}

// ComputeSupervised executes Compute only in an externally supervised self-reexec worker.
func ComputeSupervised(ctx context.Context, input []byte, seed uint64) (Outcome, SupervisionObservation, *SupervisionFailure) {
	if ctx == nil {
		ctx = context.Background()
	}
	return computeSupervised(ctx, input, seed)
}

func reconstruct(input []byte, expectedSeed uint64, wire *workerOutcome) (Outcome, *SupervisionFailure) {
	if wire == nil {
		return Outcome{}, failure(CodeMalformedOutput, errors.New("missing outcome"))
	}
	p, failed := Project(input)
	if failed != nil {
		return Outcome{}, failure(CodeInternalAdmission, failed)
	}
	out := Outcome{Outcome: wire.Outcome, ProfileID: wire.ProfileID, ProfileDigest: wire.ProfileDigest, Algorithm: wire.Algorithm, LogicalDigest: wire.LogicalDigest, ClaimCeiling: wire.ClaimCeiling, Resolution: wire.Resolution, Seed: wire.Seed, Communities: wire.Communities, Projection: p, Source: p.Source}
	validOutcome := (len(p.Occurrences) == 0 && out.Outcome == "EMPTY") || (len(p.Occurrences) > 0 && out.Outcome == "COMPLETE")
	if !validOutcome || out.ProfileID != ProfileID || out.ProfileDigest != ProfileDigest || out.Algorithm != algorithm || out.Resolution != 1 || out.Seed != expectedSeed || out.ClaimCeiling != ClaimCeiling || out.LogicalDigest != logicalDigest(out.Communities) || !validCanonicalCommunities(out.Communities, p.NodeIdentities) {
		return Outcome{}, failure(CodeMalformedOutput, errors.New("worker output violates Program C invariants"))
	}
	return out, nil
}

func validCanonicalCommunities(communities []Community, nodes []string) bool {
	seen := make(map[string]bool, len(nodes))
	for i, community := range communities {
		if len(community.Members) == 0 || !sort.StringsAreSorted(community.Members) || (i > 0 && compareStrings(communities[i-1].Members, community.Members) >= 0) {
			return false
		}
		for _, member := range community.Members {
			if seen[member] {
				return false
			}
			seen[member] = true
		}
	}
	if len(seen) != len(nodes) {
		return false
	}
	for _, node := range nodes {
		if !seen[node] {
			return false
		}
	}
	return true
}

func executablePath() (string, *SupervisionFailure) {
	path, err := os.Executable()
	if err != nil {
		return "", failure(CodeNonzeroExit, err)
	}
	return path, nil
}
