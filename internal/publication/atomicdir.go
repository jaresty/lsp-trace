package publication

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
)

const DirectoryGenerationMechanism = "atomic_no_replace_directory_generation"

// Package-private transaction hooks are nil outside deterministic tests.
var (
	testHookGenerationRootValidated func()
	testHookGenerationStageOpened   func()
	testHookGenerationBeforeCommit  func()
)

// GenerationFile is one exact regular file in a private staged generation.
// Name is slash-separated and relative to the generation directory.
type GenerationFile struct {
	Name  string
	Bytes []byte
}

// GenerationRequest installs every file with one no-replace directory rename.
type GenerationRequest struct {
	Root          *Root
	FinalSelector string
	Files         []GenerationFile
}

// GenerationReceipt distinguishes atomic namespace visibility from durability.
type GenerationReceipt struct {
	FinalSelector   string
	Mechanism       string
	NamespaceAtomic bool
	CrashDurability string
}

// PublishGeneration stages, syncs, and rereads a complete private directory,
// then makes it visible with one descriptor-relative atomic no-replace rename.
// Root validation pins a capability, not a pathname lease: later renaming or
// replacing any pathname ancestor neither redirects nor invalidates publication.
// Errors after the final rename are not returned: the namespace transaction committed.
func PublishGeneration(req GenerationRequest) (*GenerationReceipt, error) {
	if req.Root == nil || len(req.Files) == 0 {
		return nil, errors.New("invalid generation request")
	}
	if err := req.Root.ValidatePrivate(); err != nil {
		return nil, err
	}
	if testHookGenerationRootValidated != nil {
		testHookGenerationRootValidated()
	}
	t, err := capabilityTarget(req.Root, req.FinalSelector)
	if err != nil {
		return nil, err
	}
	defer t.close()
	parentInfo, err := t.parent.Stat(".")
	if err != nil || !parentInfo.IsDir() || enforcePOSIXOwnerOnlyMode(runtime.GOOS) && parentInfo.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("generation parent is not private directory")
	}
	if err := validateRootOwner(parentInfo); err != nil {
		return nil, err
	}
	lock := targetLock(t.key)
	lock.Lock()
	defer lock.Unlock()
	if _, err := t.parent.Lstat(t.name); err == nil {
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	files := append([]GenerationFile(nil), req.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	seen := map[string]bool{}
	for _, f := range files {
		if err := validateGenerationName(f.Name); err != nil || f.Bytes == nil || seen[f.Name] {
			if err == nil {
				err = errors.New("nil or duplicate generation file")
			}
			return nil, err
		}
		seen[f.Name] = true
	}

	stage, err := randomStageName(t.parent)
	if err != nil {
		return nil, err
	}
	committed := false
	var stageRoot *os.Root
	var stagedInfo os.FileInfo
	defer func() {
		if !committed {
			removeBoundStage(t.parent, stage, stageRoot, stagedInfo)
		}
		if stageRoot != nil {
			_ = stageRoot.Close()
		}
	}()
	if err := t.parent.Mkdir(stage, 0o700); err != nil {
		return nil, err
	}
	createdInfo, err := t.parent.Lstat(stage)
	if err != nil || !createdInfo.IsDir() || createdInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("created staging directory identity unavailable")
	}
	stageRoot, err = t.parent.OpenRoot(stage)
	if err != nil {
		return nil, err
	}
	openedInfo, err := stageRoot.Stat(".")
	if err != nil || !os.SameFile(createdInfo, openedInfo) {
		return nil, errors.New("created staging directory identity changed")
	}
	stagedInfo = openedInfo
	if testHookGenerationStageOpened != nil {
		testHookGenerationStageOpened()
	}
	for _, f := range files {
		if err := writeGenerationFile(stageRoot, f); err != nil {
			return nil, err
		}
	}
	if err := verifyGeneration(stageRoot, files); err != nil {
		return nil, err
	}
	if err := syncGenerationDirectories(stageRoot, files); err != nil {
		return nil, err
	}
	stageDir, err := stageRoot.Open(".")
	if err != nil {
		return nil, err
	}
	if err = stageDir.Sync(); err != nil {
		_ = stageDir.Close()
		return nil, err
	}
	if err = stageDir.Close(); err != nil {
		return nil, err
	}
	stagedInfo, err = stageRoot.Stat(".")
	if err != nil {
		return nil, err
	}
	parentDir, err := t.parent.Open(".")
	if err != nil {
		return nil, err
	}
	if testHookGenerationBeforeCommit != nil {
		testHookGenerationBeforeCommit()
	}
	if err := validateBoundStage(t.parent, stage, stageRoot, stagedInfo); err != nil {
		_ = parentDir.Close()
		return nil, err
	}
	if err = renameNoReplace(int(parentDir.Fd()), stage, t.name); err != nil {
		_ = parentDir.Close()
		return nil, err
	}
	committed = true
	// Namespace visibility is now irrevocably successful. Directory sync and
	// close are best-effort post-commit durability work and cannot reverse it.
	durability := "NOT_CHECKED_POST_COMMIT"
	if runtime.GOOS == "windows" {
		durability = "UNAVAILABLE"
	} else if parentDir.Sync() == nil {
		// This confirms the final parent entry was sync-requested. It does not
		// promise whole-filesystem, storage-device, or crash-recovery durability.
		durability = "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE"
	}
	_ = parentDir.Close()
	return &GenerationReceipt{FinalSelector: req.FinalSelector, Mechanism: DirectoryGenerationMechanism, NamespaceAtomic: true, CrashDurability: durability}, nil
}

func validateBoundStage(parent *os.Root, name string, stage *os.Root, expected os.FileInfo) error {
	if parent == nil || stage == nil || expected == nil {
		return errors.New("staging directory is unavailable")
	}
	named, err := parent.Lstat(name)
	if err != nil {
		return errors.New("staging directory entry changed")
	}
	bound, err := stage.Stat(".")
	if err != nil || !os.SameFile(named, bound) || !os.SameFile(expected, bound) {
		return errors.New("staging directory identity changed")
	}
	if !named.IsDir() || named.Mode()&os.ModeSymlink != 0 || named.Mode().Perm() != 0o700 || nlink(named) != nlink(bound) || nlink(bound) != nlink(expected) {
		return errors.New("staging directory metadata changed")
	}
	if err := validateRootOwner(named); err != nil {
		return errors.New("staging directory ownership changed")
	}
	return nil
}

func removeBoundStage(parent *os.Root, name string, stage *os.Root, expected os.FileInfo) {
	if validateBoundStage(parent, name, stage, expected) == nil {
		_ = parent.RemoveAll(name)
	}
}

func validateGenerationName(name string) error {
	if name == "" || strings.IndexByte(name, 0) >= 0 || path.Clean(name) != name || !filepathLocal(name) {
		return errors.New("unsafe generation file name")
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." {
			return errors.New("unsafe generation file component")
		}
	}
	return nil
}

func filepathLocal(name string) bool {
	return !strings.HasPrefix(name, "/") && name != ".." && !strings.HasPrefix(name, "../")
}

func randomStageName(parent *os.Root) (string, error) {
	for i := 0; i < 32; i++ {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		name := ".lsp-trace-generation-" + hex.EncodeToString(b[:])
		if _, err := parent.Lstat(name); os.IsNotExist(err) {
			return name, nil
		}
	}
	return "", errors.New("generation staging collision limit reached")
}

func writeGenerationFile(root *os.Root, f GenerationFile) error {
	parts := strings.Split(f.Name, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if err := root.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := root.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || enforcePOSIXOwnerOnlyMode(runtime.GOOS) && info.Mode().Perm()&0o077 != 0 {
			return errors.New("unsafe staged directory")
		}
	}
	file, err := root.OpenFile(f.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	writeErr := writeAll(file, f.Bytes)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func syncGenerationDirectories(root *os.Root, files []GenerationFile) error {
	dirs := map[string]bool{".": true}
	for _, f := range files {
		parts := strings.Split(f.Name, "/")
		for i := 1; i < len(parts); i++ {
			dirs[strings.Join(parts[:i], "/")] = true
		}
	}
	names := make([]string, 0, len(dirs))
	for name := range dirs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.Count(names[i], "/") > strings.Count(names[j], "/") })
	for _, name := range names {
		d, err := root.Open(name)
		if err != nil {
			return err
		}
		err = d.Sync()
		closeErr := d.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func verifyGeneration(root *os.Root, files []GenerationFile) error {
	for _, want := range files {
		before, err := root.Lstat(want.Name)
		if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm()&0o077 != 0 || before.Sys() == nil {
			return errors.New("unsafe staged regular file")
		}
		if nlink(before) != 1 {
			return errors.New("staged file link ambiguity")
		}
		f, err := root.Open(want.Name)
		if err != nil {
			return err
		}
		got, readErr := io.ReadAll(io.LimitReader(f, int64(len(want.Bytes))+1))
		after, statErr := f.Stat()
		closeErr := f.Close()
		if readErr != nil || statErr != nil || closeErr != nil {
			return errors.Join(readErr, statErr, closeErr)
		}
		if !os.SameFile(before, after) || !bytes.Equal(got, want.Bytes) || after.Size() != int64(len(want.Bytes)) {
			return fmt.Errorf("staged exact bytes mismatch: %s", want.Name)
		}
	}
	return nil
}
