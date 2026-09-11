package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	q "lsp-trace/qualification/program-c/gonum-louvain"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type identity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type run struct {
	Index         int                `json:"index"`
	Kind          string             `json:"kind"`
	RequestSHA256 string             `json:"request_sha256"`
	Result        q.SupervisedResult `json:"result"`
}
type receipt struct {
	Schema          string            `json:"schema"`
	Recommendation  string            `json:"recommendation"`
	Repository      map[string]any    `json:"repository"`
	Command         []string          `json:"command"`
	Inputs          map[string]string `json:"inputs"`
	Candidate       map[string]string `json:"candidate"`
	Build           map[string]any    `json:"build"`
	Supervisor      map[string]any    `json:"supervisor"`
	Runtime         map[string]string `json:"runtime"`
	Parameters      map[string]any    `json:"parameters"`
	Limits          map[string]any    `json:"limits"`
	Runs            []run             `json:"runs"`
	CanonicalOutput q.Output          `json:"canonical_output"`
	ReceiptSHA256   string            `json:"receipt_sha256"`
}

func fileID(path string) (identity, error) {
	b, e := os.ReadFile(path)
	return identity{Path: path, SHA256: q.Digest(b)}, e
}
func git(root string, args ...string) (string, error) {
	a := append([]string{"-C", root}, args...)
	b, e := exec.Command("/usr/bin/git", a...).Output()
	return strings.TrimSpace(string(b)), e
}
func repositoryDigests(root string, paths []string) (map[string]string, error) {
	out := make(map[string]string, len(paths))
	for _, path := range paths {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}
		out[path] = q.Digest(b)
	}
	return out, nil
}
func main() {
	var root, input, candidate, out, goTool, candidateBuild, qualifierBuild string
	flag.StringVar(&root, "repo-root", "", "repository root")
	flag.StringVar(&input, "input", "", "authoritative retained export")
	flag.StringVar(&candidate, "candidate", "", "built candidate executable")
	flag.StringVar(&out, "receipt", "", "new receipt path")
	flag.StringVar(&goTool, "go-tool", "", "absolute Go executable used to build both binaries")
	flag.StringVar(&candidateBuild, "candidate-build-command", "", "exact candidate build command")
	flag.StringVar(&qualifierBuild, "qualifier-build-command", "", "exact qualifier build command")
	flag.Parse()
	die := func(kind string, e any) { fmt.Fprintf(os.Stderr, "%s: %v\n", kind, e); os.Exit(1) }
	if root == "" || input == "" || candidate == "" || out == "" || goTool == "" || candidateBuild == "" || qualifierBuild == "" {
		die("INVALID_COMMAND", "required flags")
	}
	raw, e := os.ReadFile(input)
	if e != nil {
		die("DERIVATION_REJECTED", e)
	}
	fixture, e := q.DeriveFixture(raw)
	if e != nil {
		die(q.FailureKind(e), e)
	}
	rev, e := git(root, "rev-parse", "HEAD")
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	dirtyText, e := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	cid, e := fileID(candidate)
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	goid, e := fileID(goTool)
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	psid, e := fileID(q.PSPath)
	if e != nil {
		die("MEMORY_UNSUPPORTED", e)
	}
	self, e := os.Executable()
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	sid, e := fileID(self)
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	policyPaths := []string{
		"qualification/program-c/projection-profiles.v1.json",
		"qualification/program-c/profile-qualification.tsv",
		"qualification/program-c/i-03-candidate-inventory.v1.json",
		"qualification/program-c/i-04-license-inputs.v1.json",
		"qualification/program-c/i-06-determinism-policy.v1.json",
		"qualification/program-c/i-07-resource-envelope.v1.json",
		"qualification/program-c/gate-i-i-01-all-profiles.receipt.txt",
	}
	boundInputs, e := repositoryDigests(root, policyPaths)
	if e != nil {
		die("IDENTITY_FAILURE", e)
	}
	boundInputs["retained_export_path"] = input
	boundInputs["retained_export_sha256"] = q.Digest(raw)
	r := receipt{Schema: "lsp-trace.private.program-c.gonum-louvain.qualification.v2", Recommendation: "REJECTED", Repository: map[string]any{"revision": rev, "dirty": dirtyText != "", "status_porcelain_sha256": q.Digest([]byte(dirtyText))}, Command: append([]string{self}, os.Args[1:]...), Inputs: boundInputs, Candidate: map[string]string{"name": "gonum Louvain Modularize", "version": q.CandidateVersion, "module": "gonum.org/v1/gonum", "module_sum": q.GonumModuleSum, "executable": cid.Path, "executable_sha256": cid.SHA256}, Build: map[string]any{"go_tool": goid, "candidate_command": candidateBuild, "qualifier_command": qualifierBuild, "required_flags": []string{"-trimpath", "-buildvcs=false", "-ldflags=-buildid="}}, Supervisor: map[string]any{"implementation": sid, "ps": psid, "process_group": true, "rss_metric": "aggregate descendant RSS from /bin/ps -axo pid=,ppid=,rss=", "poll_interval_milliseconds": 20, "fail_closed": true}, Runtime: map[string]string{"go": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH}, Parameters: map[string]any{"seed": uint64(1), "resolution": 1.0, "directed": true, "base_runs": q.Runs, "reverse_insertion_permutations": q.Permutations}, Limits: map[string]any{"wall_seconds": 60, "aggregate_process_tree_rss_bytes": int64(512 << 20), "nodes": q.MaxNodes, "edges": q.MaxEdges}}
	sup := q.DarwinSupervisor()
	for i := 0; i < q.Runs+q.Permutations; i++ {
		kind := "base"
		reverse := false
		if i == q.Runs {
			kind = "reverse_insertion"
			reverse = true
		}
		request := q.Request{Fixture: fixture, Seed: 1, ReverseInsertion: reverse}
		requestBytes, e := json.Marshal(request)
		if e != nil {
			die("RECEIPT_INVALID", e)
		}
		res, e := sup.Run(candidate, request)
		if e != nil {
			die(q.FailureKind(e), e)
		}
		if i > 0 && res.Output.Digest != r.Runs[0].Result.Output.Digest {
			die("NONDETERMINISM", fmt.Sprintf("run %d", i))
		}
		r.Runs = append(r.Runs, run{Index: i, Kind: kind, RequestSHA256: q.Digest(requestBytes), Result: res})
	}
	if len(r.Runs) != 4 {
		die("SCHEDULE_INVALID", len(r.Runs))
	}
	r.CanonicalOutput = r.Runs[0].Result.Output
	r.Recommendation = "IMPLEMENTATION_CANDIDATE"
	r.ReceiptSHA256 = ""
	canonical, e := json.Marshal(r)
	if e != nil {
		die("RECEIPT_INVALID", e)
	}
	sum := sha256.Sum256(canonical)
	r.ReceiptSHA256 = "sha256:" + hex.EncodeToString(sum[:])
	encoded, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		die("RECEIPT_INVALID", e)
	}
	encoded = append(encoded, '\n')
	if e = os.MkdirAll(filepath.Dir(out), 0755); e != nil {
		die("RECEIPT_WRITE", e)
	}
	f, e := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if e != nil {
		die("RECEIPT_WRITE", e)
	}
	if _, e = f.Write(encoded); e != nil {
		f.Close()
		die("RECEIPT_WRITE", e)
	}
	if e = f.Sync(); e != nil {
		f.Close()
		die("RECEIPT_WRITE", e)
	}
	if e = f.Close(); e != nil {
		die("RECEIPT_WRITE", e)
	}
	fmt.Println(r.ReceiptSHA256)
}
