// Package retainedresolver resolves source bytes under exact retained-manifest
// custody. It never reads a working-tree path, contacts a network, or falls
// through between storage classes.
package retainedresolver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"lsp-trace/internal/retainedmanifest"
	"lsp-trace/internal/sourceobject"
)

type Code string

const (
	CodeInvalidRequest   Code = "INVALID_REQUEST"
	CodeManifestMismatch Code = "MANIFEST_MISMATCH"
	CodeEntryMissing     Code = "ENTRY_MISSING"
	CodeUnavailable      Code = "UNAVAILABLE"
	CodeClassMismatch    Code = "STORAGE_CLASS_MISMATCH"
	CodeCustodyMismatch  Code = "CUSTODY_MISMATCH"
	CodeResolutionFailed Code = "RESOLUTION_FAILED"
	CodeIdentityMismatch Code = "IDENTITY_MISMATCH"
	CodeLimit            Code = "LIMIT"
)

type Error struct {
	Code  Code
	cause error
}

func (e *Error) Error() string          { return string(e.Code) }
func (e *Error) Unwrap() error          { return e.cause }
func fail(code Code, cause error) error { return &Error{Code: code, cause: cause} }
func IsCode(err error, code Code) bool {
	var typed *Error
	return errors.As(err, &typed) && typed.Code == code
}

type GitBinding struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Blob   string `json:"blob"`
	Path   string `json:"path"`
}

type Request struct {
	ManifestDigest string
	EntryID        string
	Git            *GitBinding
}

type ObjectLookup interface {
	Get(sourceobject.Identity) (sourceobject.Object, error)
}

type GitLookup interface {
	Get(GitBinding, sourceobject.Identity) (sourceobject.Object, error)
}

type Dependencies struct {
	Git     GitLookup
	Objects ObjectLookup
}

type Result struct {
	EntryID         string
	StorageClass    string
	CustodyIdentity string
	Object          sourceobject.Object
}

func Resolve(manifestRaw, graphBytes, captureBytes []byte, request Request, dependencies Dependencies) (Result, error) {
	zero := Result{}
	manifest, manifestID, err := retainedmanifest.Admit(manifestRaw, graphBytes, captureBytes)
	if err != nil {
		return zero, fail(CodeManifestMismatch, err)
	}
	if request.ManifestDigest == "" || request.ManifestDigest != manifestID || request.EntryID == "" {
		return zero, fail(CodeManifestMismatch, errors.New("exact manifest and entry identity required"))
	}
	var entry *retainedmanifest.Entry
	for i := range manifest.Entries {
		if manifest.Entries[i].EntryID == request.EntryID {
			entry = &manifest.Entries[i]
			break
		}
	}
	if entry == nil {
		return zero, fail(CodeEntryMissing, errors.New("entry is not present in admitted manifest"))
	}
	if entry.Availability != "AVAILABLE" || entry.StorageClass == "UNAVAILABLE" {
		return zero, fail(CodeUnavailable, errors.New("manifest records source bytes unavailable"))
	}

	var object sourceobject.Object
	switch entry.StorageClass {
	case "GIT_BLOB":
		if request.Git == nil || dependencies.Git == nil {
			return zero, fail(CodeInvalidRequest, errors.New("exact Git binding and resolver required"))
		}
		if entry.CustodyIdentity != GitCustodyIdentity(*request.Git) {
			return zero, fail(CodeCustodyMismatch, errors.New("Git binding differs from manifest custody"))
		}
		object, err = dependencies.Git.Get(*request.Git, entry.Source)
	case "CONTENT_ADDRESS", "EMBEDDED_IMMUTABLE":
		if request.Git != nil {
			return zero, fail(CodeClassMismatch, errors.New("Git binding cannot satisfy immutable-object class"))
		}
		if dependencies.Objects == nil {
			return zero, fail(CodeInvalidRequest, errors.New("immutable object resolver required"))
		}
		if entry.CustodyIdentity != ContentCustodyIdentity(entry.StorageClass, entry.Source) {
			return zero, fail(CodeCustodyMismatch, errors.New("object identity differs from manifest custody"))
		}
		object, err = dependencies.Objects.Get(entry.Source)
	default:
		return zero, fail(CodeClassMismatch, errors.New("unsupported retained storage class"))
	}
	if err != nil {
		return zero, classifyLookup(err)
	}
	if object.Identity != entry.Source || uint64(len(object.Bytes)) != entry.Source.ByteLength || digest(object.Bytes) != entry.Source.Digest {
		return zero, fail(CodeIdentityMismatch, errors.New("resolved bytes differ from exact manifest source identity"))
	}
	return Result{EntryID: entry.EntryID, StorageClass: entry.StorageClass, CustodyIdentity: entry.CustodyIdentity, Object: sourceobject.Object{Identity: object.Identity, Bytes: append([]byte(nil), object.Bytes...)}}, nil
}

func GitCustodyIdentity(binding GitBinding) string {
	raw, _ := json.Marshal(binding)
	return domainDigest("lsp-trace-retained-git-custody-v1", raw)
}

func ContentCustodyIdentity(class string, id sourceobject.Identity) string {
	raw, _ := json.Marshal(struct {
		Class  string                `json:"storage_class"`
		Source sourceobject.Identity `json:"source_identity"`
	}{class, id})
	return domainDigest("lsp-trace-retained-object-custody-v1", raw)
}

func UnavailableCustodyIdentity(id sourceobject.Identity) string {
	raw, _ := json.Marshal(id)
	return domainDigest("lsp-trace-retained-unavailable-custody-v1", raw)
}

func domainDigest(domain string, raw []byte) string {
	preimage := append(append([]byte(domain), 0), raw...)
	return digest(preimage)
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func classifyLookup(err error) error {
	if sourceobject.IsCode(err, sourceobject.CodeLimit) {
		return fail(CodeLimit, err)
	}
	var typed *Error
	if errors.As(err, &typed) {
		return err
	}
	return fail(CodeResolutionFailed, err)
}

type gitCLI struct {
	root     string
	maxBytes int64
}

func NewGitLookup(root string, maxBytes int64) (GitLookup, error) {
	if !filepath.IsAbs(root) || maxBytes < 1 {
		return nil, fail(CodeInvalidRequest, errors.New("absolute Git root and positive byte limit required"))
	}
	clean := filepath.Clean(root)
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return nil, fail(CodeInvalidRequest, errors.New("Git root must be an existing directory"))
	}
	return &gitCLI{root: clean, maxBytes: maxBytes}, nil
}

func (g *gitCLI) Get(binding GitBinding, expected sourceobject.Identity) (sourceobject.Object, error) {
	if g == nil || g.root == "" || g.maxBytes < 1 || !validOID(binding.Commit) || !validOID(binding.Tree) || !validOID(binding.Blob) || !validGitPath(binding.Path) {
		return sourceobject.Object{}, fail(CodeInvalidRequest, errors.New("canonical commit/tree/blob/path binding required"))
	}
	commitTree, err := g.text("rev-parse", binding.Commit+"^{tree}")
	if err != nil {
		return sourceobject.Object{}, fail(CodeResolutionFailed, err)
	}
	if commitTree != binding.Tree {
		return sourceobject.Object{}, fail(CodeCustodyMismatch, errors.New("commit does not bind exact tree"))
	}
	line, err := g.bytes(int64(256+len(binding.Path)), "ls-tree", "-z", binding.Tree, "--", binding.Path)
	if err != nil {
		return sourceobject.Object{}, fail(CodeResolutionFailed, err)
	}
	want := "100644 blob " + binding.Blob + "\t" + binding.Path + "\x00"
	wantExec := "100755 blob " + binding.Blob + "\t" + binding.Path + "\x00"
	if string(line) != want && string(line) != wantExec {
		return sourceobject.Object{}, fail(CodeCustodyMismatch, errors.New("tree does not bind exact path and blob"))
	}
	kind, err := g.text("cat-file", "-t", binding.Blob)
	if err != nil {
		return sourceobject.Object{}, fail(CodeResolutionFailed, err)
	}
	if kind != "blob" {
		return sourceobject.Object{}, fail(CodeCustodyMismatch, errors.New("bound object is not a blob"))
	}
	body, err := g.bytes(g.maxBytes, "cat-file", "blob", binding.Blob)
	if err != nil {
		return sourceobject.Object{}, err
	}
	object := sourceobject.Object{Identity: sourceobject.Identity{Digest: digest(body), ByteLength: uint64(len(body))}, Bytes: body}
	if object.Identity != expected {
		return sourceobject.Object{}, fail(CodeIdentityMismatch, errors.New("Git blob differs from manifest source identity"))
	}
	return object, nil
}

func (g *gitCLI) text(args ...string) (string, error) {
	raw, err := g.bytes(4096, args...)
	return strings.TrimSpace(string(raw)), err
}

func (g *gitCLI) bytes(limit int64, args ...string) ([]byte, error) {
	if limit < 1 {
		return nil, fail(CodeLimit, errors.New("positive Git read limit required"))
	}
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fail(CodeResolutionFailed, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fail(CodeResolutionFailed, err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(stdout, limit+1))
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, fail(CodeResolutionFailed, readErr)
	}
	if int64(len(raw)) > limit {
		return nil, fail(CodeLimit, errors.New("Git object exceeds configured byte limit"))
	}
	if waitErr != nil {
		return nil, fmt.Errorf("git command failed (%s): %w", strconv.Itoa(cmd.ProcessState.ExitCode()), waitErr)
	}
	return raw, nil
}

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validGitPath(value string) bool {
	return value != "" && value == filepath.ToSlash(filepath.Clean(value)) && !filepath.IsAbs(value) && value != "." && value != ".." && !strings.HasPrefix(value, "../")
}
