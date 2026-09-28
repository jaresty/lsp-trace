package adr0011acquisition

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

const syntheticSourceHeader = "ADR0011_SYNTHETIC_SOURCE_SET_V1\x00"
const syntheticSourceFileLimit = 2 << 20
const syntheticSourceTotalLimit = 16 << 20
const syntheticSourceCountLimit = 256

var errSyntheticSourcePin = errors.New("synthetic implementation source pin mismatch")

// SyntheticSourceEntry is an independently selected expected source file, not a
// record obtained from the claimant. Paths are UTF-8, slash-relative and sorted.
type SyntheticSourceEntry struct {
	Path       string
	ByteLength uint64
	SHA256     [32]byte
}

var syntheticSourceDirectories = map[string]bool{
	"internal/adr0011acquisition":     true,
	"internal/adr0011methodresult":    true,
	"internal/adr0011querytarget":     true,
	"internal/adr0011requestkey":      true,
	"internal/adr0011methodtransport": true,
	"internal/adr0011sourcecontext":   true,
	"internal/adr0011closure":         true,
	"internal/publication":            true,
	"internal/strictjson":             true,
	"internal/lspwire":                true,
	"sessionruntime":                  true,
}

// VerifySyntheticSourcePin uses the source checkout containing this function.
// It verifies source bytes only: not the build, binary, runtime, dependency
// closure, provider authentication, or live qualification.
func VerifySyntheticSourcePin(entries []SyntheticSourceEntry, expected [32]byte) error {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return errSyntheticSourcePin
	}
	return verifySyntheticSourcePinAt(filepath.Clean(filepath.Join(filepath.Dir(source), "../..")), entries, expected)
}

// verifySyntheticSourcePinAt supplies a test root without changing production
// root selection. Neither this root nor the expected values come from a proposal.
func verifySyntheticSourcePinAt(root string, entries []SyntheticSourceEntry, expected [32]byte) error {
	if !filepath.IsAbs(root) || len(entries) == 0 || len(entries) > syntheticSourceCountLimit {
		return errSyntheticSourcePin
	}
	h := sha256.New()
	_, _ = h.Write([]byte(syntheticSourceHeader))
	var total uint64
	previous := ""
	selected := make(map[string]bool, len(entries))
	var word [8]byte
	for _, entry := range entries {
		name := entry.Path
		if !utf8.ValidString(name) || strings.ContainsRune(name, 0) || strings.ContainsRune(name, '\\') ||
			path.Clean(name) != name || strings.HasPrefix(name, "/") ||
			!strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			!syntheticSourceDirectories[path.Dir(name)] || name <= previous ||
			entry.ByteLength > syntheticSourceFileLimit || total+entry.ByteLength > syntheticSourceTotalLimit {
			return errSyntheticSourcePin
		}
		previous = name
		selected[name] = true
		// Reject symlink ancestors as well as symlink leaves; lexical path checks
		// alone do not constrain filesystem traversal.
		current := root
		for _, component := range strings.Split(name, "/") {
			current = filepath.Join(current, component)
			info, err := os.Lstat(current)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return errSyntheticSourcePin
			}
		}
		file, err := os.Open(current)
		if err != nil {
			return errSyntheticSourcePin
		}
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() < 0 || uint64(info.Size()) != entry.ByteLength {
			_ = file.Close()
			return errSyntheticSourcePin
		}
		data, readErr := io.ReadAll(io.LimitReader(file, syntheticSourceFileLimit+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(data) > syntheticSourceFileLimit || uint64(len(data)) != entry.ByteLength || sha256.Sum256(data) != entry.SHA256 {
			return errSyntheticSourcePin
		}
		total += uint64(len(data))
		binary.BigEndian.PutUint64(word[:], uint64(len(name)))
		_, _ = h.Write(word[:])
		_, _ = h.Write([]byte(name))
		binary.BigEndian.PutUint64(word[:], uint64(len(data)))
		_, _ = h.Write(word[:])
		_, _ = h.Write(entry.SHA256[:])
	}
	// Reject omitted implementation files, not just mismatches among selected
	// entries. Directory enumeration is independent of claimant proposals.
	for dir := range syntheticSourceDirectories {
		items, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			return errSyntheticSourcePin
		}
		for _, item := range items {
			name := item.Name()
			if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && !selected[dir+"/"+name] {
				return errSyntheticSourcePin
			}
		}
	}
	var actual [32]byte
	copy(actual[:], h.Sum(nil))
	if actual != expected {
		return errSyntheticSourcePin
	}
	return nil
}
