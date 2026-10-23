package censussourceplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
)

const (
	MaxPartitionSize = 1024
	policyVersion    = "lsp-trace:census-source-plan:v1"
)

var (
	ErrInvalidIdentity      = errors.New("invalid census source identity")
	ErrDuplicateIdentity    = errors.New("duplicate census source identity")
	ErrInvalidPartitionSize = errors.New("invalid census source partition size")
)

type Partition struct {
	Ordinal int
	Members []censusacquisition.SourceFile
}

type Plan struct {
	Identity   string
	Authority  uint8
	Files      []censusacquisition.SourceFile
	Partitions []Partition
}

type identityEnvelope struct {
	Policy        string                         `json:"policy"`
	Authority     uint8                          `json:"authority"`
	PartitionSize int                            `json:"partition_size"`
	Files         []censusacquisition.SourceFile `json:"files"`
}

func Build(files []censusacquisition.SourceFile, workspace string, partitionSize int) (Plan, error) {
	if partitionSize <= 0 || partitionSize > MaxPartitionSize {
		return Plan{}, ErrInvalidPartitionSize
	}
	if len(files) == 0 {
		return Plan{}, ErrInvalidIdentity
	}
	planned := append([]censusacquisition.SourceFile(nil), files...)
	for i := range planned {
		if err := validate(planned[i], workspace); err != nil {
			return Plan{}, err
		}
	}
	sort.Slice(planned, func(i, j int) bool {
		left, right := platformPathKey(planned[i].Path), platformPathKey(planned[j].Path)
		if left != right {
			return left < right
		}
		if planned[i].URI != planned[j].URI {
			return planned[i].URI < planned[j].URI
		}
		if planned[i].LanguageID != planned[j].LanguageID {
			return planned[i].LanguageID < planned[j].LanguageID
		}
		return planned[i].Disposition < planned[j].Disposition
	})
	paths, uris := make(map[string]struct{}, len(planned)), make(map[string]struct{}, len(planned))
	for _, file := range planned {
		pathKey := platformPathKey(file.Path)
		if _, exists := paths[pathKey]; exists {
			return Plan{}, ErrDuplicateIdentity
		}
		if _, exists := uris[file.URI]; exists {
			return Plan{}, ErrDuplicateIdentity
		}
		paths[pathKey] = struct{}{}
		uris[file.URI] = struct{}{}
	}
	canonical, err := json.Marshal(identityEnvelope{Policy: policyVersion, Authority: 0, PartitionSize: partitionSize, Files: planned})
	if err != nil {
		return Plan{}, ErrInvalidIdentity
	}
	sum := sha256.Sum256(canonical)
	plan := Plan{Identity: "sha256:" + hex.EncodeToString(sum[:]), Authority: 0, Files: append([]censusacquisition.SourceFile(nil), planned...)}
	for start := 0; start < len(planned); start += partitionSize {
		end := start + partitionSize
		if end > len(planned) {
			end = len(planned)
		}
		plan.Partitions = append(plan.Partitions, Partition{Ordinal: len(plan.Partitions), Members: append([]censusacquisition.SourceFile(nil), planned[start:end]...)})
	}
	return plan, nil
}

func validate(file censusacquisition.SourceFile, workspace string) error {
	if !validRelativePath(file.Path) {
		return ErrInvalidIdentity
	}
	relative, err := canonicalWorkspaceRelativePath(workspace, file.URI)
	if err != nil || !platformPathEqual(relative, file.Path) {
		return ErrInvalidIdentity
	}
	switch file.Disposition {
	case "", census.FileUnreadable, census.FileUnsupported:
		return nil
	default:
		return ErrInvalidIdentity
	}
}

func validRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, 0) || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || path.IsAbs(value) || path.Clean(value) != value {
		return false
	}
	for i, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." || i == 0 && strings.Contains(component, ":") {
			return false
		}
	}
	return true
}

func canonicalWorkspaceRelativePath(workspace, rawURI string) (string, error) {
	if workspace == "" || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return "", ErrInvalidIdentity
	}
	u, err := url.Parse(rawURI)
	if err != nil || u.Scheme != "file" || u.Opaque != "" || u.User != nil || u.Host != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", ErrInvalidIdentity
	}
	if rawURI != (&url.URL{Scheme: "file", Path: u.Path}).String() {
		return "", ErrInvalidIdentity
	}
	target := filepath.FromSlash(u.Path)
	if runtime.GOOS == "windows" && len(target) >= 3 && target[0] == filepath.Separator && target[2] == ':' {
		target = target[1:]
	}
	if !filepath.IsAbs(target) || filepath.Clean(target) != target {
		return "", ErrInvalidIdentity
	}
	relative, err := filepath.Rel(workspace, target)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidIdentity
	}
	relative = filepath.ToSlash(relative)
	if !validRelativePath(relative) {
		return "", ErrInvalidIdentity
	}
	return relative, nil
}

func platformPathEqual(left, right string) bool {
	return platformPathKey(left) == platformPathKey(right)
}

func platformPathKey(value string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(value)
	}
	return value
}
