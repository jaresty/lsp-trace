// Package censuslineagepilot owns the first private durable-custody pilot wire.
package censuslineagepilot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"

	"lsp-trace/internal/censussourcecontinuation"
)

const (
	ManifestSchemaVersion = "lsp-trace.census-lineage-pilot-manifest.private.v1"
	HeadSchemaVersion     = "lsp-trace.census-lineage-pilot-head.private.v1"
)

var (
	ErrInvalid    = errors.New("invalid private census lineage pilot")
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Artifact struct {
	Kind       string `json:"kind"`
	Selector   string `json:"selector"`
	SHA256     string `json:"sha256"`
	ByteLength uint64 `json:"byte_length"`
}

type Manifest struct {
	SchemaVersion             string     `json:"schema_version"`
	Authority                 uint8      `json:"authority"`
	Accepted                  bool       `json:"accepted"`
	LineageID                 string     `json:"lineage_id"`
	SourceSelectionSHA256     string     `json:"source_selection_sha256"`
	SourceSelectionByteLength uint64     `json:"source_selection_byte_length"`
	PartitionPlanSHA256       string     `json:"partition_plan_sha256"`
	PartitionPlanByteLength   uint64     `json:"partition_plan_byte_length"`
	Artifacts                 []Artifact `json:"artifacts"`
	Effects                   string     `json:"effects"`
	FeatureIdentity           string     `json:"feature_identity"`
	Completeness              string     `json:"completeness"`
	SourceGraphComplete       string     `json:"source_graph_complete"`
	AbsenceInference          string     `json:"absence_inference"`
}

type ManifestInput struct {
	LineageID                 string
	SourceSelectionSHA256     string
	SourceSelectionByteLength uint64
	PartitionPlanSHA256       string
	PartitionPlanByteLength   uint64
	Artifacts                 []Artifact
}

func NewManifest(in ManifestInput) (Manifest, error) {
	m := Manifest{SchemaVersion: ManifestSchemaVersion, LineageID: in.LineageID,
		SourceSelectionSHA256: in.SourceSelectionSHA256, SourceSelectionByteLength: in.SourceSelectionByteLength,
		PartitionPlanSHA256: in.PartitionPlanSHA256, PartitionPlanByteLength: in.PartitionPlanByteLength,
		Artifacts: append([]Artifact{}, in.Artifacts...), Effects: "NONE", FeatureIdentity: "UNRESOLVED",
		Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	if validateManifest(m) != nil {
		return Manifest{}, ErrInvalid
	}
	return m, nil
}

func (m Manifest) ArtifactsCopy() []Artifact { return append([]Artifact{}, m.Artifacts...) }

func EncodeManifest(m Manifest) ([]byte, error) {
	m.Artifacts = append([]Artifact{}, m.Artifacts...)
	if validateManifest(m) != nil {
		return nil, ErrInvalid
	}
	return json.Marshal(m)
}

func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if strict(raw, &m) != nil || validateManifest(m) != nil {
		return Manifest{}, ErrInvalid
	}
	canonical, err := EncodeManifest(m)
	if err != nil || !bytes.Equal(raw, canonical) {
		return Manifest{}, ErrInvalid
	}
	m.Artifacts = append([]Artifact{}, m.Artifacts...)
	return m, nil
}

func validateManifest(m Manifest) error {
	if m.SchemaVersion != ManifestSchemaVersion || m.Authority != 0 || m.Accepted || m.LineageID == "" ||
		!validDigest(m.SourceSelectionSHA256) || m.SourceSelectionByteLength == 0 ||
		!validDigest(m.PartitionPlanSHA256) || m.PartitionPlanByteLength == 0 || m.Artifacts == nil || len(m.Artifacts) == 0 ||
		m.Effects != "NONE" || m.FeatureIdentity != "UNRESOLVED" || m.Completeness != "UNKNOWN" ||
		m.SourceGraphComplete != "UNKNOWN" || m.AbsenceInference != "PROHIBITED" {
		return ErrInvalid
	}
	kinds := make(map[string]struct{}, len(m.Artifacts))
	selectors := make(map[string]struct{}, len(m.Artifacts))
	for i, a := range m.Artifacts {
		_, duplicateKind := kinds[a.Kind]
		_, duplicateSelector := selectors[a.Selector]
		if a.Kind == "" || !validDigest(a.Selector) || a.Selector != a.SHA256 || a.ByteLength == 0 || duplicateKind || duplicateSelector {
			return ErrInvalid
		}
		if i > 0 && !artifactLess(m.Artifacts[i-1], a) {
			return ErrInvalid
		}
		kinds[a.Kind] = struct{}{}
		selectors[a.Selector] = struct{}{}
	}
	return nil
}

func artifactLess(a, b Artifact) bool {
	return a.Kind < b.Kind || a.Kind == b.Kind && a.Selector < b.Selector
}
func validDigest(s string) bool { return digestPattern.MatchString(s) }
func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func strict(raw []byte, dst any) error {
	if len(raw) == 0 || duplicateJSONKey(raw) != nil {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	return nil
}

func duplicateJSONKey(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return ErrInvalid
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('['):
			for d.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('}'), json.Delim(']'):
			return ErrInvalid
		default:
			return nil
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	return nil
}

type head struct {
	SchemaVersion    string `json:"schema_version"`
	Generation       uint64 `json:"generation"`
	Selector         string `json:"selector"`
	WorkflowKey      string `json:"workflow_key"`
	LineageID        string `json:"lineage_id"`
	ManifestSelector string `json:"manifest_selector"`
	Authority        uint8  `json:"authority"`
	Accepted         bool   `json:"accepted"`
	State            string `json:"state"`
}

func encodeHead(h head) ([]byte, error) {
	h.Selector = ""
	unsigned, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	h.Selector = Digest(unsigned)
	return json.Marshal(h)
}

func newHead(workflowKey, lineageID, manifestSelector string, generation uint64) (head, []byte, error) {
	if workflowKey == "" || lineageID == "" || !validDigest(manifestSelector) || generation == 0 {
		return head{}, nil, ErrInvalid
	}
	h := head{SchemaVersion: HeadSchemaVersion, Generation: generation, WorkflowKey: workflowKey, LineageID: lineageID, ManifestSelector: manifestSelector, State: "COMMITTED"}
	raw, err := encodeHead(h)
	if err != nil {
		return head{}, nil, err
	}
	var parsed head
	if strict(raw, &parsed) != nil {
		return head{}, nil, ErrInvalid
	}
	return parsed, raw, nil
}

func ParseHeadRevision(raw []byte) (censussourcecontinuation.HeadRevision, error) {
	if len(raw) == 0 {
		return censussourcecontinuation.HeadRevision{}, ErrInvalid
	}
	h, err := parseHead(raw)
	if err != nil {
		return censussourcecontinuation.HeadRevision{}, err
	}
	return censussourcecontinuation.HeadRevision{Generation: h.Generation, Selector: h.Selector}, nil
}

func parseHead(raw []byte) (head, error) {
	var h head
	if strict(raw, &h) != nil || h.SchemaVersion != HeadSchemaVersion || h.Generation == 0 || !validDigest(h.Selector) || h.WorkflowKey == "" || h.LineageID == "" || !validDigest(h.ManifestSelector) || h.Authority != 0 || h.Accepted || h.State != "COMMITTED" {
		return head{}, ErrInvalid
	}
	canonical, err := encodeHead(h)
	if err != nil || !bytes.Equal(canonical, raw) {
		return head{}, ErrInvalid
	}
	return h, nil
}

func SortArtifactsForConstruction(a []Artifact) []Artifact {
	out := append([]Artifact{}, a...)
	sort.Slice(out, func(i, j int) bool { return artifactLess(out[i], out[j]) })
	return out
}
