package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/verification"
)

const (
	generationArtifactName         = "artifact.json"
	generationReceiptName          = "receipt.json"
	directoryDurabilityChecked     = verification.DirectoryDurabilityChecked
	directoryDurabilityUnavailable = verification.DirectoryDurabilityUnavailable
)

var (
	errDirectorySyncUnavailable    = errors.New("directory sync unavailable on platform")
	openPublicationDirectory       = os.Open
	openPublicationRoot            = os.OpenRoot
	openChildPublicationRoot       = func(root *os.Root, name string) (*os.Root, error) { return root.OpenRoot(name) }
	syncPublicationDirectory       = platformSyncPublicationDirectory
	publicationDirectoryDurability = platformDirectoryDurability
)

func syncPublishedDirectory(path string) error {
	dir, err := openPublicationDirectory(path)
	if err != nil {
		return fmt.Errorf("open publication directory %q: %w", path, err)
	}
	if err := syncPublicationDirectory(dir); err != nil && !errors.Is(err, errDirectorySyncUnavailable) {
		_ = dir.Close()
		return fmt.Errorf("sync publication directory %q: %w", path, err)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close publication directory %q: %w", path, err)
	}
	return nil
}

type generationSelector = verification.Selector

func readGenerationSelector(path string) (generationSelector, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return generationSelector{}, err
	}
	return decodeGenerationSelector(data)
}

func decodeGenerationSelector(data []byte) (generationSelector, error) {
	return verification.DecodeSelector(data)
}

type publicationFailureRecord struct {
	Version                    string `json:"version"`
	Target                     string `json:"target"`
	Error                      string `json:"error"`
	ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	DirectoryDurability        string `json:"directory_durability"`
}

func retainPublicationFailure(path string, data []byte, publishErr error) (string, error) {
	dir := filepath.Dir(path)
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing ancestor for publication failure record")
		}
		dir = parent
	}
	record, err := json.Marshal(publicationFailureRecord{
		Version:                    "lsp-trace.publication-failure.v1",
		Target:                     path,
		Error:                      publishErr.Error(),
		ExactSerializedBytesDigest: graph.ExactBytesDigest(data),
		DirectoryDurability:        publicationDirectoryDurability,
	})
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".publication-failure-*.json")
	if err != nil {
		return "", err
	}
	name := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := f.Write(append(record, '\n')); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := syncPublishedDirectory(dir); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}

func marshalResult(result graph.Result, pretty bool) ([]byte, error) {
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(result, "", "  ")
	} else {
		b, err = json.Marshal(result)
	}
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
func receiptBytes(data []byte) ([]byte, error) {
	return verification.ReceiptBytes(data, publicationDirectoryDurability)
}
func stage(dir, pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = io.Copy(f, bytes.NewReader(data)); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}
func publishArtifact(path string, data []byte) error {
	dir := filepath.Dir(path)
	artifactTemp, err := stage(dir, ".lsp-trace-artifact-*", data)
	if err != nil {
		return err
	}
	defer os.Remove(artifactTemp)
	staged, err := os.ReadFile(artifactTemp)
	if err != nil || !bytes.Equal(staged, data) {
		return fmt.Errorf("staged artifact byte validation failed: %v", err)
	}
	if err = os.Rename(artifactTemp, path); err != nil {
		return err
	}
	if err = os.Remove(path + ".receipt.json"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncPublishedDirectory(dir)
}

func randomPublicationName(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func openPinnedPublicationParent(path string) (*os.Root, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	parentPath := filepath.Dir(absolute)
	info, err := os.Lstat(parentPath)
	if err != nil {
		return nil, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, "", fmt.Errorf("publication parent is not a no-follow directory")
	}
	root, err := openPublicationRoot(parentPath)
	if err != nil {
		return nil, "", err
	}
	opened, statErr := root.Stat(".")
	if statErr != nil || !os.SameFile(info, opened) {
		root.Close()
		return nil, "", fmt.Errorf("publication parent identity changed")
	}
	return root, filepath.Base(absolute), nil
}

func writeRootFile(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = root.Remove(name)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func syncRoot(root *os.Root) error {
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syncPublicationDirectory(f); err != nil && !errors.Is(err, errDirectorySyncUnavailable) {
		return err
	}
	return nil
}

func publishBundle(path string, data []byte) error {
	return publishValidatedBundle(path, data, graph.ValidateSemanticBundle)
}

func admitGraphProvenance(data []byte) error {
	_, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v1")
	return err
}

func publishGraphProvenance(path string, data []byte) error {
	return publishValidatedBundle(path, data, admitGraphProvenance)
}

// Admission is selected by the explicit route, never guessed from submitted JSON.
// The immutable generation and exact-byte receipt format remains unchanged.
func publishValidatedBundle(path string, data []byte, admit func([]byte) error) error {
	if err := admit(bytes.TrimSpace(data)); err != nil {
		return fmt.Errorf("staged semantic validation: %w", err)
	}
	receipt, err := receiptBytes(data)
	if err != nil {
		return err
	}
	parent, finalName, err := openPinnedPublicationParent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	generationName, err := randomPublicationName(".lsp-trace-generation-")
	if err != nil {
		return err
	}
	if err := parent.Mkdir(generationName, 0700); err != nil {
		return err
	}
	selected := false
	defer func() {
		if !selected {
			_ = parent.RemoveAll(generationName)
		}
	}()
	generation, err := openChildPublicationRoot(parent, generationName)
	if err != nil {
		return err
	}
	defer generation.Close()
	if err := writeRootFile(generation, generationArtifactName, data); err != nil {
		return err
	}
	if err := writeRootFile(generation, generationReceiptName, receipt); err != nil {
		return err
	}
	staged, err := generation.ReadFile(generationArtifactName)
	if err != nil || !bytes.Equal(staged, data) {
		return fmt.Errorf("staged artifact byte validation failed: %v", err)
	}
	if err := syncRoot(generation); err != nil {
		return err
	}
	selectorData, err := json.Marshal(generationSelector{Generation: generationName})
	if err != nil {
		return err
	}
	selectorTemp, err := randomPublicationName(".lsp-trace-selector-")
	if err != nil {
		return err
	}
	if err := writeRootFile(parent, selectorTemp, append(selectorData, '\n')); err != nil {
		return err
	}
	defer parent.Remove(selectorTemp)
	if err := platformInstallNoReplace(parent, selectorTemp, finalName); err != nil {
		return err
	}
	selected = true
	_ = parent.Remove(finalName + ".receipt.json")
	return syncRoot(parent)
}

func writePrivateSyncedFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
func readRootRegularNoFollow(root *os.Root, name string) ([]byte, error) {
	return readRootRegularNoFollowLimit(root, name, 0)
}

func readRootRegularNoFollowLimit(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("no-follow regular file required: %q", name)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("file identity changed: %q", name)
	}
	if limit > 0 {
		if opened.Size() > limit {
			return nil, fmt.Errorf("bounded publication byte LIMIT: %q", name)
		}
		raw, err := io.ReadAll(io.LimitReader(f, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(raw)) > limit {
			return nil, fmt.Errorf("bounded publication byte LIMIT: %q", name)
		}
		return raw, nil
	}
	return io.ReadAll(f)
}

func openRootDirectoryNoFollow(root *os.Root, name string) (*os.Root, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("no-follow directory required: %q", name)
	}
	child, err := openChildPublicationRoot(root, name)
	if err != nil {
		return nil, err
	}
	opened, err := child.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		child.Close()
		return nil, fmt.Errorf("directory identity changed: %q", name)
	}
	return child, nil
}

func loadCustodiedGeneration(path string) ([]byte, string, error) {
	return loadCustodiedGenerationLimit(path, 0)
}

func loadCustodiedGenerationLimit(path string, limit int64) ([]byte, string, error) {
	parent, finalName, err := openPinnedPublicationParent(path)
	if err != nil {
		return nil, "verify", err
	}
	defer parent.Close()
	selectorData, err := readRootRegularNoFollowLimit(parent, finalName, limit)
	if err != nil {
		return nil, "verify", err
	}
	selector, err := decodeGenerationSelector(selectorData)
	if err != nil {
		return nil, "verify", err
	}
	generation, err := openRootDirectoryNoFollow(parent, selector.Generation)
	if err != nil {
		return nil, "verify", fmt.Errorf("incomplete selected generation: %w", err)
	}
	defer generation.Close()
	data, err := readRootRegularNoFollowLimit(generation, generationArtifactName, limit)
	if err != nil {
		return nil, "verify", fmt.Errorf("incomplete selected generation: %w", err)
	}
	receiptData, err := readRootRegularNoFollowLimit(generation, generationReceiptName, limit)
	if err != nil {
		return nil, "verify receipt", fmt.Errorf("incomplete selected generation: %w", err)
	}
	if err = verification.VerifyReceipt(data, receiptData); err != nil {
		return nil, "verify receipt", err
	}
	return data, "", nil
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	family := flags.String("family", "graph", "explicit evidence family")
	version := flags.String("version", "", "evidence version")
	if flags.Parse(args) != nil || flags.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace verify [--family graph-provenance|retained-calls --version v1] PATH")
		return 1
	}
	admit := graph.ValidateSemanticBundle
	switch {
	case *family == "graph" && (*version == "" || *version == "v3"):
	case *family == graphprovenance.Family && (*version == "v2" || *version == "lsp-trace.graph-provenance.v2"):
		admit = admitAcquisitionV2
	case *family == graphprovenance.Family && (*version == "v3" || *version == graphprovenance.VersionV3):
		admit = admitAcquisitionV3
	case *family == graphprovenance.Family && *version == "v1":
		admit = admitGraphProvenance
	case *family == "retained-calls" && *version == "v1":
		admit = admitRetainedCalls
	case *family == "bounded-retained-analysis" && *version == "v1":
		admit = admitBoundedAnalysis
	case *family == "bounded-retained-metrics" && *version == "v1":
		admit = admitBoundedMetrics
	case *family == "bounded-retained-ranking" && *version == "v1":
		admit = admitBoundedRanking
	default:
		fmt.Fprintln(stderr, "unsupported verification family/version")
		return 1
	}
	var byteLimit int64
	if *family == "bounded-retained-analysis" || *family == "bounded-retained-metrics" || *family == "bounded-retained-ranking" {
		byteLimit = boundedAnalysisVerificationLimit
	}
	data, stage, err := loadCustodiedGenerationLimit(flags.Arg(0), byteLimit)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", stage, err)
		return 1
	}
	if err = admit(bytes.TrimSpace(data)); err != nil {
		fmt.Fprintf(stderr, "verify semantic receipt: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "verified integrity and custody")
	return 0
}
