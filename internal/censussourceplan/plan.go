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

// pathIdentityPolicy keeps path comparison and URI-relative derivation together.
// Qualified preparation supplies a lexical policy; legacy Build keeps its host
// behavior. Partitioning, duplicate detection and hashing remain shared.
type pathIdentityPolicy struct {
	key      func(string) string
	relative func(string, string) (string, error)
}

func legacyPathIdentityPolicy() pathIdentityPolicy {
	return pathIdentityPolicy{key: platformPathKey, relative: canonicalWorkspaceRelativePath}
}

func Build(files []censusacquisition.SourceFile, workspace string, partitionSize int) (Plan, error) {
	return buildWithPathIdentity(files, workspace, partitionSize, legacyPathIdentityPolicy())
}

func buildWithPathIdentity(files []censusacquisition.SourceFile, workspace string, partitionSize int, policy pathIdentityPolicy) (Plan, error) {
	if partitionSize <= 0 || partitionSize > MaxPartitionSize {
		return Plan{}, ErrInvalidPartitionSize
	}
	if len(files) == 0 {
		return Plan{}, ErrInvalidIdentity
	}
	planned := append([]censusacquisition.SourceFile(nil), files...)
	for i := range planned {
		if err := validateWithPathIdentity(planned[i], workspace, policy); err != nil {
			return Plan{}, err
		}
	}
	sort.Slice(planned, func(i, j int) bool {
		left, right := policy.key(planned[i].Path), policy.key(planned[j].Path)
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
		pathKey := policy.key(file.Path)
		if _, exists := paths[pathKey]; exists {
			return Plan{}, ErrDuplicateIdentity
		}
		if _, exists := uris[file.URI]; exists {
			return Plan{}, ErrDuplicateIdentity
		}
		paths[pathKey] = struct{}{}
		uris[file.URI] = struct{}{}
	}
	identity, err := canonicalIdentity(planned, partitionSize)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Identity: identity, Authority: 0, Files: append([]censusacquisition.SourceFile(nil), planned...)}
	for start := 0; start < len(planned); start += partitionSize {
		end := start + partitionSize
		if end > len(planned) {
			end = len(planned)
		}
		plan.Partitions = append(plan.Partitions, Partition{Ordinal: len(plan.Partitions), Members: append([]censusacquisition.SourceFile(nil), planned[start:end]...)})
	}
	return plan, nil
}

func Validate(plan Plan, partitionSize int) error {
	if plan.Authority != 0 || len(plan.Files) == 0 || len(plan.Partitions) == 0 {
		return ErrInvalidIdentity
	}
	identity, err := canonicalIdentity(plan.Files, partitionSize)
	if err != nil || plan.Identity != identity {
		return ErrInvalidIdentity
	}
	file := 0
	for ordinal, partition := range plan.Partitions {
		if partition.Ordinal != ordinal || len(partition.Members) == 0 || file+len(partition.Members) > len(plan.Files) {
			return ErrInvalidIdentity
		}
		for i := range partition.Members {
			if partition.Members[i] != plan.Files[file+i] {
				return ErrInvalidIdentity
			}
		}
		file += len(partition.Members)
	}
	if file != len(plan.Files) {
		return ErrInvalidIdentity
	}
	return nil
}

func canonicalIdentity(files []censusacquisition.SourceFile, partitionSize int) (string, error) {
	if partitionSize <= 0 || partitionSize > MaxPartitionSize {
		return "", ErrInvalidPartitionSize
	}
	canonical, err := json.Marshal(identityEnvelope{Policy: policyVersion, Authority: 0, PartitionSize: partitionSize, Files: files})
	if err != nil {
		return "", ErrInvalidIdentity
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validate(file censusacquisition.SourceFile, workspace string) error {
	return validateWithPathIdentity(file, workspace, legacyPathIdentityPolicy())
}

func validateWithPathIdentity(file censusacquisition.SourceFile, workspace string, policy pathIdentityPolicy) error {
	if !validRelativePath(file.Path) {
		return ErrInvalidIdentity
	}
	relative, err := policy.relative(workspace, file.URI)
	if err != nil || policy.key(relative) != policy.key(file.Path) {
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
	uriPath, err := canonicalFileURIPath(rawURI)
	if err != nil {
		return "", err
	}
	target := filepath.FromSlash(uriPath)
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

// canonicalFileURIPath is shared by both policies. It rejects URI aliases,
// hosts, queries and fragments without touching the filesystem.
func canonicalFileURIPath(rawURI string) (string, error) {
	u, err := url.Parse(rawURI)
	if err != nil || u.Scheme != "file" || u.Opaque != "" || u.User != nil || u.Host != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", ErrInvalidIdentity
	}
	if rawURI != (&url.URL{Scheme: "file", Path: u.Path}).String() {
		return "", ErrInvalidIdentity
	}
	return u.Path, nil
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
