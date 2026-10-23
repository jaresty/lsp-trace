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
	"internal/adr0011work":            true,
	"internal/publication":            true,
	"internal/strictjson":             true,
	"internal/lspwire":                true,
	"sessionruntime":                  true,
}

const syntheticSourceHistoricalAdditionalGoCount = 5

var syntheticSourceAdditionalGoPaths = []string{
	"cmd/lsp-trace/census_assembly.go",
	"cmd/lsp-trace/census_core.go",
	"cmd/lsp-trace/census_private_discovery_causes.go",
	"cmd/lsp-trace/census_runner.go",
	"cmd/lsp-trace/census_chunking.go",
	"internal/censuschunking/chunking.go",
	"cmd/lsp-trace-mcp/census_chunking.go",
	"cmd/lsp-trace-mcp/census_completion.go",
	"cmd/lsp-trace-mcp/census_executor.go",
	"cmd/lsp-trace-mcp/census_mcp_binding.go",
	"cmd/lsp-trace-mcp/census_runtime.go",
	"cmd/lsp-trace-mcp/main.go",
	"cmd/lsp-trace/census_chunking_adapters_private.go",
	"internal/censuschunkingruntime/adapters.go",
	"internal/censuschunkingruntime/translate.go",
	"internal/mcpcontract/census_future.go",
	"internal/operation/types.go",
	"cmd/lsp-trace/acquisition_v2.go",
	"cmd/lsp-trace/census_discovery_adapter.go",
	"internal/censusacquisition/discovery.go",
	"internal/censusacquisition/request_budget.go",
	"cmd/lsp-trace/census_command.go",
	"internal/censuspreflight/result.go",
	"internal/censusrequest/receipt_v2.go",
	"internal/schema/schema.go",
	"internal/source/discovery.go",
	"cmd/lsp-trace/private_census_terminal_observation.go",
	"cmd/lsp-trace/private_provider_probe.go",
	"internal/managedprocess/process.go",
	"internal/describeworker/config.go",
	"internal/describeworker/runner.go",
	"internal/describeworker/runner_v2.go",
	"internal/censusenumerationprojection/projection.go",
	"internal/censussourceplan/plan.go",
}

var syntheticSourceAdditionalSchemaPaths = []string{
	"internal/mcpcontract/testdata/schemas/envelope-census-chunk-terminal-result.v1.schema.json",
	"internal/schema/schemas/lsp-trace.census-enumeration-diagnostic.v1.schema.json",
	"internal/schema/schemas/lsp-trace.census-enumeration-preflight-result.v1.schema.json",
	"internal/schema/schemas/lsp-trace.census-request-receipt.v2.schema.json",
}

var syntheticSourceAdditionalGoPathSet = func() map[string]bool {
	paths := make(map[string]bool, len(syntheticSourceAdditionalGoPaths))
	for _, name := range syntheticSourceAdditionalGoPaths {
		paths[name] = true
	}
	return paths
}()

var syntheticSourceAdditionalSchemaPathSet = func() map[string]bool {
	paths := make(map[string]bool, len(syntheticSourceAdditionalSchemaPaths))
	for _, name := range syntheticSourceAdditionalSchemaPaths {
		paths[name] = true
	}
	return paths
}()

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
	additionalGoSelected := 0
	additionalSchemaSelected := 0
	var word [8]byte
	for _, entry := range entries {
		name := entry.Path
		inDirectory := syntheticSourceDirectories[path.Dir(name)]
		isAdditionalGo := syntheticSourceAdditionalGoPathSet[name]
		isAdditionalSchema := syntheticSourceAdditionalSchemaPathSet[name]
		validDirectoryGo := inDirectory && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
		validAdditionalGo := isAdditionalGo && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
		validAdditionalSchema := isAdditionalSchema && strings.HasSuffix(name, ".json")
		if !utf8.ValidString(name) || strings.ContainsRune(name, 0) || strings.ContainsRune(name, '\\') ||
			path.Clean(name) != name || strings.HasPrefix(name, "/") ||
			(!validDirectoryGo && !validAdditionalGo && !validAdditionalSchema) ||
			(validDirectoryGo && (additionalGoSelected != 0 || additionalSchemaSelected != 0 || name <= previous)) ||
			(validAdditionalGo && (additionalSchemaSelected != 0 || additionalGoSelected >= len(syntheticSourceAdditionalGoPaths) || name != syntheticSourceAdditionalGoPaths[additionalGoSelected])) ||
			(validAdditionalSchema && (additionalGoSelected != len(syntheticSourceAdditionalGoPaths) || additionalSchemaSelected >= len(syntheticSourceAdditionalSchemaPaths) || name != syntheticSourceAdditionalSchemaPaths[additionalSchemaSelected])) ||
			entry.ByteLength > syntheticSourceFileLimit || total+entry.ByteLength > syntheticSourceTotalLimit {
			return errSyntheticSourcePin
		}
		if validAdditionalGo {
			additionalGoSelected++
		} else if validAdditionalSchema {
			additionalSchemaSelected++
		} else {
			previous = name
		}
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
	historicalProfile := additionalGoSelected == syntheticSourceHistoricalAdditionalGoCount && additionalSchemaSelected == 0
	successorProfile := additionalGoSelected == len(syntheticSourceAdditionalGoPaths) && additionalSchemaSelected == len(syntheticSourceAdditionalSchemaPaths)
	if additionalGoSelected != 0 && !historicalProfile && !successorProfile {
		return errSyntheticSourcePin
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
