package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	q "lsp-trace/qualification/program-c/gonum-louvain"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type identity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type run struct {
	Index         int                `json:"index"`
	FixtureID     string             `json:"fixture_id"`
	Seed          uint64             `json:"seed"`
	Repeat        int                `json:"repeat"`
	Kind          string             `json:"kind"`
	Request       q.Request          `json:"request"`
	RequestSHA256 string             `json:"request_sha256"`
	Result        q.SupervisedResult `json:"result"`
	Outcome       string             `json:"outcome"`
	Failure       *q.Failure         `json:"failure"`
}

type canonicalOutput struct {
	FixtureID string   `json:"fixture_id"`
	Seed      uint64   `json:"seed"`
	Output    q.Output `json:"output"`
}

type receipt struct {
	Schema             string            `json:"schema"`
	Result             string            `json:"result"`
	Recommendation     string            `json:"recommendation"`
	Repository         map[string]any    `json:"repository"`
	Command            []string          `json:"command"`
	Inputs             map[string]string `json:"inputs"`
	FixtureInventory   map[string]any    `json:"fixture_inventory"`
	Fixtures           []q.FixtureRecord `json:"fixtures"`
	Candidate          map[string]string `json:"candidate"`
	Build              map[string]any    `json:"build"`
	Supervisor         map[string]any    `json:"supervisor"`
	Runtime            map[string]string `json:"runtime"`
	Schedule           map[string]any    `json:"schedule"`
	Limits             map[string]any    `json:"limits"`
	Counts             map[string]int    `json:"counts"`
	Runs               []run             `json:"runs"`
	CanonicalOutputs   []canonicalOutput `json:"canonical_outputs"`
	TypedFailurePolicy []string          `json:"typed_failure_policy"`
	NoFallbackPolicy   map[string]bool   `json:"no_fallback_policy"`
	ClaimCeiling       string            `json:"claim_ceiling"`
	ReceiptSHA256      string            `json:"receipt_sha256"`
}

func fileID(path string) (identity, error) {
	b, err := os.ReadFile(path)
	return identity{Path: path, SHA256: q.Digest(b)}, err
}

func git(root string, args ...string) (string, error) {
	all := append([]string{"-C", root}, args...)
	b, err := exec.Command("/usr/bin/git", all...).Output()
	return strings.TrimSpace(string(b)), err
}

func repositoryDigests(root string, paths []string) (map[string]string, error) {
	sort.Strings(paths)
	out := make(map[string]string, len(paths))
	for _, path := range paths {
		if _, exists := out[path]; exists {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		out[path] = q.Digest(b)
	}
	return out, nil
}

func typedFailure(err error) *q.Failure {
	var failure *q.Failure
	if errors.As(err, &failure) {
		copy := *failure
		return &copy
	}
	return &q.Failure{Kind: "NONZERO_EXIT", Detail: err.Error()}
}

func writeReceipt(path string, value *receipt) error {
	value.ReceiptSHA256 = ""
	canonical, err := json.Marshal(value)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(canonical)
	value.ReceiptSHA256 = "sha256:" + hex.EncodeToString(sum[:])
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if err != nil {
		return err
	}
	if _, err = file.Write(encoded); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func main() {
	var root, inventoryPath, candidate, out, goTool, candidateBuild, qualifierBuild string
	flag.StringVar(&root, "repo-root", "", "repository root")
	flag.StringVar(&inventoryPath, "fixture-inventory", "", "repository-relative exact fixture inventory")
	flag.StringVar(&candidate, "candidate", "", "built candidate executable")
	flag.StringVar(&out, "receipt", "", "new immutable receipt path")
	flag.StringVar(&goTool, "go-tool", "", "absolute Go executable used to build both binaries")
	flag.StringVar(&candidateBuild, "candidate-build-command", "", "exact candidate build command")
	flag.StringVar(&qualifierBuild, "qualifier-build-command", "", "exact qualifier build command")
	flag.Parse()
	die := func(kind string, value any) { fmt.Fprintf(os.Stderr, "%s: %v\n", kind, value); os.Exit(1) }
	if root == "" || inventoryPath == "" || candidate == "" || out == "" || goTool == "" || candidateBuild == "" || qualifierBuild == "" {
		die("INVALID_COMMAND", "required flags")
	}
	inventory, fixtures, err := q.LoadFixtureInventory(root, inventoryPath)
	if err != nil {
		die(q.FailureKind(err), err)
	}
	revision, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	dirtyText, err := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	candidateID, err := fileID(candidate)
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	goID, err := fileID(goTool)
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	psID, err := fileID(q.PSPath)
	if err != nil {
		die("MEMORY_UNSUPPORTED", err)
	}
	self, err := os.Executable()
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	supervisorID, err := fileID(self)
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	policyPaths := []string{
		inventoryPath,
		"qualification/program-c/projection-profiles.v1.json",
		"qualification/program-c/profile-qualification.tsv",
		"qualification/program-c/i-03-candidate-inventory.v2.json",
		"qualification/program-c/i-04-license-inputs.v2.json",
		"qualification/program-c/gonum-leiden-69ca49f.LICENSE",
		"qualification/program-c/i-06-determinism-policy.v1.json",
		"qualification/program-c/i-07-resource-envelope.v1.json",
		"qualification/program-c/gate-i-receipts.tsv",
	}
	for _, fixture := range fixtures {
		policyPaths = append(policyPaths, fixture.Record.Graph.Path, fixture.Record.Provenance.SourcePath)
	}
	inputs, err := repositoryDigests(root, policyPaths)
	if err != nil {
		die("IDENTITY_FAILURE", err)
	}
	value := receipt{
		Schema:         "lsp-trace.private.program-c.gonum-louvain.gate-ii-qualification.v1",
		Result:         "FAIL",
		Recommendation: "REJECTED",
		Repository: map[string]any{
			"revision": revision, "dirty": dirtyText != "", "status_porcelain_sha256": q.Digest([]byte(dirtyText)),
		},
		Command: append([]string{self}, os.Args[1:]...),
		Inputs:  inputs,
		FixtureInventory: map[string]any{
			"path": inventoryPath, "sha256": inputs[inventoryPath], "inventory_id": inventory.InventoryID,
		},
		Fixtures: inventory.Fixtures,
		Candidate: map[string]string{
			"name": "Gonum Louvain", "algorithm": "community.Modularize", "module": "gonum.org/v1/gonum", "version": q.CandidateVersion, "module_sum": q.GonumModuleSum, "executable": candidateID.Path, "executable_sha256": candidateID.SHA256,
		},
		Build: map[string]any{
			"go_tool": goID, "candidate_command": candidateBuild, "qualifier_command": qualifierBuild, "required_flags": []string{"-trimpath", "-buildvcs=false", "-ldflags=-buildid="},
		},
		Supervisor: map[string]any{
			"implementation": supervisorID, "ps": psID, "process_group": true, "canonicalization_boundary": "external parent supervisor process", "rss_metric": "aggregate descendant RSS from /bin/ps -axo pid=,ppid=,rss=", "poll_interval_milliseconds": 20, "fail_closed": true,
		},
		Runtime: map[string]string{"go": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH},
		Schedule: map[string]any{
			"seed_inventory": inventory.Seeds, "base_repeats_per_fixture_seed": q.Runs, "deterministic_input_permutations_per_fixture_seed": q.Permutations,
		},
		Limits:             map[string]any{"wall_seconds": 60, "aggregate_process_tree_rss_bytes": int64(512 << 20), "nodes": q.MaxNodes, "directed_weighted_edges": q.MaxEdges},
		TypedFailurePolicy: []string{"CAP_BREACH", "DERIVATION_REJECTED", "IDENTITY_FAILURE", "INPUT_DIGEST_MISMATCH", "MEMORY_EXCEEDED", "MEMORY_UNSUPPORTED", "NONCANONICAL_OUTPUT", "NONDETERMINISM", "NONZERO_EXIT", "PANIC", "RECEIPT_INVALID", "RECEIPT_WRITE", "SCHEDULE_INVALID", "SUPERVISOR_UNSUPPORTED", "TIMEOUT"},
		NoFallbackPolicy:   map[string]bool{"fallback": false, "substitution": false, "sampling": false, "truncation": false, "approximation": false, "silent_omission": false},
		ClaimCeiling:       "This receipt qualifies only exact bounded execution of Gonum Louvain v0.17.0 over the declared private Program C fixture and seed inventory. It does not authorize production implementation, dependency adoption, public surfaces, deployment, other candidates, revisions, licenses, or distribution modes.",
	}
	value.Counts = map[string]int{"fixtures": len(fixtures), "seeds": len(inventory.Seeds), "expected_runs": len(fixtures) * len(inventory.Seeds) * (q.Runs + q.Permutations), "completed_runs": 0, "failed_runs": 0}
	supervisor := q.DarwinSupervisor()
	globalIndex := 0
	for _, fixture := range fixtures {
		for _, seed := range inventory.Seeds {
			var cellDigest string
			for scheduleIndex := 0; scheduleIndex < q.Runs+q.Permutations; scheduleIndex++ {
				kind := "base_repeat"
				repeat := scheduleIndex + 1
				reverse := false
				if scheduleIndex == q.Runs {
					kind = "deterministic_input_permutation"
					repeat = 0
					reverse = true
				}
				request := q.Request{Fixture: fixture.Request, Seed: seed, ReverseInsertion: reverse}
				requestBytes, marshalErr := json.Marshal(request)
				if marshalErr != nil {
					die("RECEIPT_INVALID", marshalErr)
				}
				entry := run{Index: globalIndex, FixtureID: fixture.Record.ID, Seed: seed, Repeat: repeat, Kind: kind, Request: request, RequestSHA256: q.Digest(requestBytes), Outcome: "FAIL"}
				entry.Result, err = supervisor.Run(candidate, request)
				if err != nil {
					entry.Failure = typedFailure(err)
					value.Runs = append(value.Runs, entry)
					value.Counts["failed_runs"]++
					if writeErr := writeReceipt(out, &value); writeErr != nil {
						die("RECEIPT_WRITE", writeErr)
					}
					die(entry.Failure.Kind, entry.Failure.Detail)
				}
				if len(entry.Result.Observation.ResourceSamples) == 0 {
					die("MEMORY_UNSUPPORTED", "missing exact process-tree resource samples")
				}
				if scheduleIndex == 0 {
					cellDigest = entry.Result.Output.Digest
				} else if entry.Result.Output.Digest != cellDigest {
					err = q.Fail("NONDETERMINISM", fmt.Sprintf("fixture=%s seed=%d schedule_index=%d", fixture.Record.ID, seed, scheduleIndex))
					entry.Failure = typedFailure(err)
					value.Runs = append(value.Runs, entry)
					value.Counts["failed_runs"]++
					if writeErr := writeReceipt(out, &value); writeErr != nil {
						die("RECEIPT_WRITE", writeErr)
					}
					die(entry.Failure.Kind, entry.Failure.Detail)
				}
				entry.Outcome = "PASS"
				value.Runs = append(value.Runs, entry)
				value.Counts["completed_runs"]++
				globalIndex++
			}
			value.CanonicalOutputs = append(value.CanonicalOutputs, canonicalOutput{FixtureID: fixture.Record.ID, Seed: seed, Output: value.Runs[len(value.Runs)-1].Result.Output})
		}
	}
	if value.Counts["completed_runs"] != value.Counts["expected_runs"] || value.Counts["failed_runs"] != 0 || len(value.Runs) != value.Counts["expected_runs"] {
		die("SCHEDULE_INVALID", value.Counts)
	}
	value.Result = "PASS"
	value.Recommendation = "IMPLEMENTATION_CANDIDATE"
	if err = writeReceipt(out, &value); err != nil {
		die("RECEIPT_WRITE", err)
	}
	fmt.Println(value.ReceiptSHA256)
}
