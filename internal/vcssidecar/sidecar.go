package vcssidecar

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tsr "lsp-trace/internal/transientstructuralresult"
)

const SchemaVersion = "lsp-trace.vcs-churn-sidecar.v1"

type Request struct {
	Workspace    string
	FromRevision string
	ToRevision   string
}

type FileMetric struct {
	CommitCount   int    `json:"commit_count"`
	LinesAdded    int    `json:"lines_added"`
	LinesDeleted  int    `json:"lines_deleted"`
	BinaryChanges int    `json:"binary_changes"`
	LastRevision  string `json:"last_revision,omitempty"`
}

type HistoryCollection struct {
	FromRevision string
	ToRevision   string
	Metrics      map[string]FileMetric
}

type HistorySource interface {
	Collect(workspace string, paths []string, from, to string) (HistoryCollection, error)
}

type NodeMetric struct {
	NodeID string `json:"node_id"`
	Path   string `json:"path"`
	Kind   int    `json:"kind"`
	Name   string `json:"name"`
	FileMetric
}

type Result struct {
	SchemaVersion        string       `json:"schema_version"`
	Authority            int          `json:"authority"`
	SourceGraphComplete  string       `json:"source_graph_complete"`
	Attribution          string       `json:"attribution"`
	GraphArtifactDigest  string       `json:"graph_artifact_digest"`
	VCS                  string       `json:"vcs"`
	FromRevision         string       `json:"from_revision"`
	ToRevision           string       `json:"to_revision"`
	HistoryScopeComplete string       `json:"history_scope_complete"`
	NodeCount            int          `json:"node_count"`
	ChangedNodeCount     int          `json:"changed_node_count"`
	ZeroChurnNodeCount   int          `json:"zero_churn_node_count"`
	Nodes                []NodeMetric `json:"nodes"`
}

func Build(raw []byte, input tsr.LocatorResultV2, req Request, history HistorySource) (Result, error) {
	if len(raw) == 0 || input.SchemaVersion != "lsp-trace.transient-structural-result.v2" || input.Authority != 0 || input.SourceGraphComplete != "UNKNOWN" {
		return Result{}, errors.New("VCS sidecar requires conservative structural context V2 input")
	}
	if history == nil || !filepath.IsAbs(req.Workspace) || strings.TrimSpace(req.FromRevision) == "" || strings.TrimSpace(req.ToRevision) == "" {
		return Result{}, errors.New("VCS sidecar requires workspace, revision range, and history source")
	}
	pathSet := make(map[string]struct{}, len(input.Nodes))
	for _, node := range input.Nodes {
		if node.ID == "" || node.Path == "" {
			return Result{}, errors.New("VCS sidecar node identity is incomplete")
		}
		pathSet[node.Path] = struct{}{}
	}
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	collection, err := history.Collect(req.Workspace, paths, req.FromRevision, req.ToRevision)
	if err != nil {
		return Result{}, err
	}
	if collection.FromRevision == "" || collection.ToRevision == "" {
		return Result{}, errors.New("history source omitted resolved revision identity")
	}
	metrics := collection.Metrics
	for path, metric := range metrics {
		if _, ok := pathSet[path]; !ok {
			return Result{}, fmt.Errorf("history returned unrequested path %q", path)
		}
		if metric.CommitCount < 0 || metric.LinesAdded < 0 || metric.LinesDeleted < 0 || metric.BinaryChanges < 0 || (metric.CommitCount > 0) != (metric.LastRevision != "") {
			return Result{}, fmt.Errorf("history returned invalid metric for %q", path)
		}
	}
	sum := sha256.Sum256(raw)
	result := Result{
		SchemaVersion: SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", Attribution: "FILE_PATH_ONLY",
		GraphArtifactDigest: fmt.Sprintf("sha256:%x", sum), VCS: "git", FromRevision: collection.FromRevision, ToRevision: collection.ToRevision,
		HistoryScopeComplete: "COMPLETE", NodeCount: len(input.Nodes), Nodes: make([]NodeMetric, 0, len(input.Nodes)),
	}
	for _, node := range input.Nodes {
		metric := metrics[node.Path]
		result.Nodes = append(result.Nodes, NodeMetric{NodeID: node.ID, Path: node.Path, Kind: node.Kind, Name: node.Name, FileMetric: metric})
		if metric.CommitCount == 0 {
			result.ZeroChurnNodeCount++
		} else {
			result.ChangedNodeCount++
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].NodeID < result.Nodes[j].NodeID })
	if err := Validate(result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func Validate(result Result) error {
	if result.SchemaVersion != SchemaVersion || result.Authority != 0 || result.SourceGraphComplete != "UNKNOWN" || result.Attribution != "FILE_PATH_ONLY" || result.VCS != "git" || result.HistoryScopeComplete != "COMPLETE" {
		return errors.New("invalid VCS sidecar identity or claim ceiling")
	}
	if !strings.HasPrefix(result.GraphArtifactDigest, "sha256:") || len(result.GraphArtifactDigest) != 71 || !validHex(result.GraphArtifactDigest[7:]) || !validCommit(result.FromRevision) || !validCommit(result.ToRevision) {
		return errors.New("invalid VCS sidecar digest or revision identity")
	}
	if result.NodeCount != len(result.Nodes) || result.NodeCount != result.ChangedNodeCount+result.ZeroChurnNodeCount || result.NodeCount < 1 {
		return errors.New("invalid VCS sidecar accounting")
	}
	for i, node := range result.Nodes {
		if node.NodeID == "" || node.Path == "" || (i > 0 && result.Nodes[i-1].NodeID >= node.NodeID) || node.CommitCount < 0 || node.LinesAdded < 0 || node.LinesDeleted < 0 || node.BinaryChanges < 0 || (node.CommitCount > 0) != (node.LastRevision != "") || (node.LastRevision != "" && !validCommit(node.LastRevision)) {
			return errors.New("invalid VCS sidecar node row")
		}
	}
	return nil
}

func validCommit(value string) bool {
	return (len(value) == 40 || len(value) == 64) && validHex(value)
}

func validHex(value string) bool {
	for _, c := range value {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			return false
		}
	}
	return true
}
