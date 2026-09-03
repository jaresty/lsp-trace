package publication

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

const (
	CodeOutputSelectorUnsafe = "OUTPUT_SELECTOR_UNSAFE"
	CodePublicationFailed    = "PUBLICATION_FAILED"
	CodeTargetExists         = "TARGET_EXISTS"
	CodeOperationTerminal    = "OPERATION_TERMINAL"
	PublicationMechanism     = "atomic_no_replace"
)

var ErrOperationTerminal = errors.New("operation already terminal")

type Root struct {
	path   string
	file   *os.File
	handle *os.Root
	info   os.FileInfo
}

// OpenRoot opens one cleaned absolute owner-only directory as a process-lifetime
// publication capability. Relative and symlink roots are rejected rather than
// converted into ambient filesystem authority.
func OpenRoot(path string) (*Root, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("publication root must be a cleaned absolute path")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("publication root is not a no-follow directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(before, info) {
		file.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("publication root identity changed")
	}
	handle, err := os.OpenRoot(path)
	if err != nil {
		file.Close()
		return nil, err
	}
	handleInfo, err := handle.Stat(".")
	if err != nil || !os.SameFile(info, handleInfo) {
		handle.Close()
		file.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("publication root identity changed")
	}
	return &Root{path: path, file: file, handle: handle, info: info}, nil
}

func (r *Root) Close() error {
	if r == nil {
		return nil
	}
	var first error
	if r.handle != nil {
		first = r.handle.Close()
		r.handle = nil
	}
	if r.file != nil {
		if err := r.file.Close(); first == nil {
			first = err
		}
		r.file = nil
	}
	return first
}

func (r *Root) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

type Request struct {
	Root             *Root
	Selector         string
	Bytes            []byte
	ArtifactSchemaID string
}

type Receipt struct {
	Digest               string `json:"artifact_digest"`
	ByteLength           uint64 `json:"artifact_byte_length"`
	ArtifactSchemaID     string `json:"artifact_schema_id"`
	PublicationMechanism string `json:"publication_mechanism"`
}

type Failure struct {
	Stage        string `json:"stage"`
	Code         string `json:"code"`
	Cleanup      bool   `json:"cleanup"`
	AtomicRename bool   `json:"atomic_rename"`
	Truncated    bool   `json:"truncated"`
	cause        error
}

func (f *Failure) Error() string {
	if f == nil {
		return ""
	}
	return f.Code
}

func (f *Failure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.cause
}

type Result struct {
	Receipt *Receipt
	Failure *Failure
}

func (r Result) Err() error {
	if r.Failure != nil {
		return r.Failure
	}
	return nil
}

type Publisher struct{}

func NewPublisher() *Publisher { return &Publisher{} }

var targetLocks sync.Map

func targetLock(key string) *sync.Mutex {
	value, _ := targetLocks.LoadOrStore(key, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func failure(stage, code string, cleanup, renamed bool, cause error) Result {
	return Result{Failure: &Failure{Stage: stage, Code: code, Cleanup: cleanup, AtomicRename: renamed, cause: cause}}
}

type target struct {
	parent *os.Root
	owned  bool
	name   string
	key    string
}

func (t *target) close() {
	if t != nil && t.owned && t.parent != nil {
		_ = t.parent.Close()
	}
}

func safeTarget(root *Root, selector string) (*target, error) {
	if root == nil || root.file == nil || root.handle == nil || selector == "" || strings.IndexByte(selector, 0) >= 0 || filepath.IsAbs(selector) || !filepath.IsLocal(selector) {
		return nil, errors.New("unsafe selector")
	}
	parts, err := selectorPathParts(runtime.GOOS, selector)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, filepath.Separator) {
			return nil, errors.New("unsafe selector component")
		}
	}
	currentPathInfo, err := os.Lstat(root.path)
	if err != nil || !os.SameFile(root.info, currentPathInfo) {
		return nil, errors.New("root identity changed")
	}
	handleInfo, err := root.handle.Stat(".")
	if err != nil || !os.SameFile(root.info, handleInfo) {
		return nil, errors.New("root identity changed")
	}

	current := root.handle
	owned := false
	for _, part := range parts[:len(parts)-1] {
		before, err := current.Lstat(part)
		if os.IsNotExist(err) {
			if err := current.Mkdir(part, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				if owned {
					current.Close()
				}
				return nil, err
			}
			before, err = current.Lstat(part)
		}
		if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() || enforcePOSIXOwnerOnlyMode(runtime.GOOS) && before.Mode().Perm()&0077 != 0 {
			if owned {
				current.Close()
			}
			return nil, errors.New("unsafe selector component")
		}
		child, err := current.OpenRoot(part)
		if err != nil {
			if owned {
				current.Close()
			}
			return nil, errors.New("unsafe selector component")
		}
		after, err := child.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			child.Close()
			if owned {
				current.Close()
			}
			return nil, errors.New("selector component identity changed")
		}
		if owned {
			current.Close()
		}
		current, owned = child, true
	}

	name := parts[len(parts)-1]
	if info, err := current.Lstat(name); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if owned {
			current.Close()
		}
		return nil, errors.New("unsafe final target")
	} else if err != nil && !os.IsNotExist(err) {
		if owned {
			current.Close()
		}
		return nil, err
	}
	parentInfo, err := current.Stat(".")
	if err != nil {
		if owned {
			current.Close()
		}
		return nil, err
	}
	key := fmt.Sprintf("%T:%v:%s", parentInfo.Sys(), parentInfo.Sys(), canonicalFinalComponent(name))
	return &target{parent: current, owned: owned, name: name, key: key}, nil
}

func canonicalFinalComponent(name string) string {
	name = norm.NFC.String(name)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(name)
	}
	return name
}

func enforcePOSIXOwnerOnlyMode(goos string) bool {
	return goos != "windows"
}

func selectorPathParts(goos, selector string) ([]string, error) {
	if goos == "windows" && strings.Contains(selector, `\`) {
		return nil, errors.New("unsafe selector component")
	}
	if path.Clean(selector) != selector {
		return nil, errors.New("selector is not normalized")
	}
	return strings.Split(selector, "/"), nil
}

func createTemporarySibling(parent *os.Root) (*os.File, string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".lsp-trace-publication-" + hex.EncodeToString(random[:])
		file, err := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			return file, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("temporary sibling collision limit reached")
}

func (p *Publisher) Publish(req Request) Result {
	target, err := safeTarget(req.Root, req.Selector)
	if err != nil {
		return failure("selector", CodeOutputSelectorUnsafe, true, false, err)
	}
	defer target.close()
	lock := targetLock(target.key)
	lock.Lock()
	defer lock.Unlock()

	if info, err := target.parent.Lstat(target.name); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return failure("selector", CodeOutputSelectorUnsafe, true, false, errors.New("unsafe final target"))
		}
		return failure("install", CodeTargetExists, true, false, os.ErrExist)
	} else if !os.IsNotExist(err) {
		return failure("install", CodePublicationFailed, true, false, err)
	}

	tmp, tmpName, err := createTemporarySibling(target.parent)
	if err != nil {
		return failure("stage", CodePublicationFailed, false, false, err)
	}
	cleanup := false
	defer func() { _ = target.parent.Remove(tmpName) }()
	if _, err = tmp.Write(req.Bytes); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup = target.parent.Remove(tmpName) == nil
		return failure("stage", CodePublicationFailed, cleanup, false, err)
	}

	// A component-relative hard-link install is the available atomic no-replace
	// namespace primitive. Removing the private sibling after success does not
	// change the published inode or imply directory-entry crash durability.
	if err = target.parent.Link(tmpName, target.name); err != nil {
		cleanup = target.parent.Remove(tmpName) == nil
		if errors.Is(err, os.ErrExist) {
			return failure("install", CodeTargetExists, cleanup, false, err)
		}
		return failure("install", CodePublicationFailed, cleanup, false, err)
	}
	cleanup = target.parent.Remove(tmpName) == nil
	_ = cleanup // post-install cleanup cannot change the committed outcome
	sum := sha256.Sum256(req.Bytes)
	return Result{Receipt: &Receipt{
		Digest: "sha256:" + hex.EncodeToString(sum[:]), ByteLength: uint64(len(req.Bytes)),
		ArtifactSchemaID: req.ArtifactSchemaID, PublicationMechanism: PublicationMechanism,
	}}
}

type Operation struct {
	mu       sync.Mutex
	p        *Publisher
	request  Request
	terminal bool
}

func NewOperation(p *Publisher, request Request) *Operation {
	return &Operation{p: p, request: request}
}

func (o *Operation) Publish() Result {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.terminal {
		return failure("arbitration", CodeOperationTerminal, true, false, ErrOperationTerminal)
	}
	o.terminal = true
	return o.p.Publish(o.request)
}
