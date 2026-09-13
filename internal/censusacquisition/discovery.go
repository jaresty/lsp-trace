package censusacquisition

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/discoveryfilter"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/source"
)

// SourceFile is one canonical workspace-relative source-file census member.
// Disposition is empty for a readable candidate; enumerators use FileUnreadable
// or FileUnsupported when a file cannot be supplied to the initialized session.
type SourceFile struct {
	Path        string
	URI         string
	LanguageID  string
	Disposition census.FileDisposition
}

// FileEnumerator returns the complete bounded source-file denominator. The
// adapter copies, validates, deduplicates, and deterministically orders it.
type FileEnumerator interface {
	Enumerate(context.Context) ([]SourceFile, error)
}

// DocumentSupplier makes one readable file available to the already initialized
// LSP session. It must not initialize, install, restart, or own the session.
type DocumentSupplier interface {
	Supply(context.Context, SourceFile) error
}

// DiscoveryClient is the initialized-session subset used by census discovery.
type DiscoveryClient interface {
	SupportsDocumentSymbols() bool
	SupportsCallHierarchy() bool
	DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error)
	PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error)
}

// DiscoveryLimits bound work without changing the denominator. Zero means no
// additional bound; members beyond a positive bound are retained as incomplete.
type DiscoveryLimits struct {
	MaxFiles       int
	MaxSymbols     int
	MaxDepth       int
	MaxStringBytes int
}

const (
	// FileProcessed and SymbolPrepared are the successful terminal vocabulary
	// written to capture-set ledgers. Other terminals retain census disposition values.
	FileProcessed  = "processed"
	SymbolPrepared = "prepared"

	defaultMaxSymbolNodes = captureset.MaxTargets
	defaultMaxSymbolDepth = 64
	defaultMaxStringBytes = 1 << 20
)

// DiscoveryAdapter performs only private census discovery in one existing
// session. Inputs are copied before use and are never mutated.
type DiscoveryAdapter struct {
	Workspace string
	Filters   Filters
	Files     FileEnumerator
	Supplier  DocumentSupplier
	Client    DiscoveryClient
	Limits    DiscoveryLimits
}

func (a DiscoveryAdapter) Discover(ctx context.Context, session SessionIdentity) (Discovery, error) {
	out := Discovery{Session: session, Workspace: a.Workspace}
	if err := session.Validate(); err != nil {
		return out, err
	}
	if a.Workspace == "" || !filepath.IsAbs(a.Workspace) || filepath.Clean(a.Workspace) != a.Workspace {
		return out, errors.New("canonical absolute workspace required")
	}
	if a.Files == nil || a.Supplier == nil || a.Client == nil {
		return out, errors.New("file enumerator, document supplier, and initialized client required")
	}
	if a.Limits.MaxFiles < 0 || a.Limits.MaxSymbols < 0 || a.Limits.MaxDepth < 0 || a.Limits.MaxStringBytes < 0 {
		return out, errors.New("discovery limits must be non-negative")
	}
	includes := append([]string(nil), a.Filters.Includes...)
	excludes := append([]string(nil), a.Filters.Excludes...)
	if err := discoveryfilter.ValidatePatterns(includes, excludes); err != nil {
		return out, err
	}
	files, err := a.Files.Enumerate(ctx)
	if err != nil {
		return out, fmt.Errorf("enumerate source files: %w", err)
	}
	files = append([]SourceFile(nil), files...)
	for i := range files {
		if err := validateSourceFile(files[i], a.Workspace); err != nil {
			return out, fmt.Errorf("source file %d: %w", i, err)
		}
	}
	sort.Slice(files, func(i, j int) bool { return sourceFileKey(files[i]) < sourceFileKey(files[j]) })
	for i := 1; i < len(files); i++ {
		if platformPathEqual(runtimeGOOS(), files[i-1].Path, files[i].Path) {
			return out, errors.New("duplicate canonical source-file identity")
		}
	}

	out.Accounting.FileDenominator = len(files)
	out.FileLedger.Denominator = len(files)
	complete := true
	symbolOrdinal := 0
	filesAttempted := 0
	for fileOrdinal, file := range files {
		fileDisposition := file.Disposition
		if fileDisposition == "" && ctx.Err() != nil {
			fileDisposition = census.FileIncomplete
		}
		if fileDisposition == "" && !selectPath(includes, excludes, file.Path) {
			fileDisposition = census.FileExcluded
		}
		if fileDisposition == "" && a.Limits.MaxFiles > 0 && filesAttempted >= a.Limits.MaxFiles {
			fileDisposition = census.FileIncomplete
		}
		if fileDisposition != "" {
			appendFile(&out, fileOrdinal, file.Path, fileDisposition)
			if fileDisposition != census.FileExcluded {
				complete = false
			}
			continue
		}
		filesAttempted++
		if err := a.Supplier.Supply(ctx, file); err != nil {
			appendFile(&out, fileOrdinal, file.Path, census.FileUnreadable)
			complete = false
			continue
		}
		if !a.Client.SupportsDocumentSymbols() {
			appendFile(&out, fileOrdinal, file.Path, census.FileUnsupported)
			complete = false
			continue
		}
		symbols, err := a.Client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: file.URI}})
		if err != nil {
			appendFile(&out, fileOrdinal, file.Path, census.FileDocumentSymbolFailed)
			complete = false
			continue
		}
		maxNodes := defaultMaxSymbolNodes
		maxDepth := defaultMaxSymbolDepth
		if a.Limits.MaxDepth > 0 {
			maxDepth = a.Limits.MaxDepth
		}
		maxStrings := defaultMaxStringBytes
		if a.Limits.MaxStringBytes > 0 {
			maxStrings = a.Limits.MaxStringBytes
		}
		flat, bounded := flattenDocumentSymbols(symbols, maxNodes, maxDepth, maxStrings)
		if !bounded {
			appendFile(&out, fileOrdinal, file.Path, census.FileIncomplete)
			complete = false
			continue
		}
		sort.SliceStable(flat, func(i, j int) bool { return documentSymbolKey(flat[i]) < documentSymbolKey(flat[j]) })
		fileComplete := true
		for _, symbol := range flat {
			ordinal := symbolOrdinal
			symbolOrdinal++
			identity := symbolIdentity(file, symbol, ordinal)
			disposition := census.SymbolIncomplete
			if ctx.Err() != nil || ordinal >= captureset.MaxTargets || (a.Limits.MaxSymbols > 0 && ordinal >= a.Limits.MaxSymbols) {
				fileComplete, complete = false, false
			} else if !callableSymbolKind(symbol.Kind) {
				disposition = census.SymbolNonCallable
				fileComplete, complete = false, false
			} else if !a.Client.SupportsCallHierarchy() {
				disposition = census.SymbolUnsupported
				fileComplete, complete = false, false
			} else {
				items, prepareErr := a.Client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: file.URI}, Position: symbol.SelectionRange.Start})
				switch {
				case prepareErr != nil:
					disposition = census.SymbolPreparationFailed
					fileComplete, complete = false, false
				case len(items) == 0:
					disposition = census.SymbolPrepareMissing
					fileComplete, complete = false, false
				case len(items) != 1 || !preparedItemMatches(items[0], file.URI, symbol):
					disposition = census.SymbolIncomplete
					fileComplete, complete = false, false
				default:
					seed, seedErr := canonicalPositionSeed(a.Workspace, file.Path, ordinal, symbol.SelectionRange.Start)
					if seedErr == nil {
						disposition = census.SymbolSelected
						out.Targets = append(out.Targets, PreparedTarget{CensusOrdinal: ordinal, CanonicalSeedV2: seed, URI: file.URI, Name: symbol.Name, Kind: symbol.Kind, Range: symbol.Range, SelectionRange: symbol.SelectionRange, SymbolIdentity: identity})
					} else {
						disposition = census.SymbolIncomplete
						fileComplete, complete = false, false
					}
				}
			}
			out.Accounting.Symbols = append(out.Accounting.Symbols, census.SymbolEntry{Ordinal: ordinal, Disposition: disposition})
			ledgerDisposition := string(disposition)
			if disposition == census.SymbolSelected {
				ledgerDisposition = SymbolPrepared
			}
			out.SymbolLedger.Entries = append(out.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: ordinal, Identity: identity, Disposition: ledgerDisposition})
		}
		if fileComplete {
			appendFile(&out, fileOrdinal, file.Path, census.FileSelected)
		} else {
			appendFile(&out, fileOrdinal, file.Path, census.FileIncomplete)
		}
	}
	out.Accounting.SymbolDenominator = symbolOrdinal
	out.SymbolLedger.Denominator = symbolOrdinal
	if ctx.Err() != nil {
		complete = false
	}
	accountingErr := out.Accounting.Validate()
	discoveryErr := validateDiscovery(out)
	_, _, canonicalErr := canonicalTargets(out)
	out.Complete = complete && len(out.Targets) > 0 && accountingErr == nil && discoveryErr == nil && canonicalErr == nil
	return cloneDiscovery(out), nil
}

func appendFile(out *Discovery, ordinal int, identity string, disposition census.FileDisposition) {
	out.Accounting.Files = append(out.Accounting.Files, census.FileEntry{Ordinal: ordinal, Disposition: disposition})
	ledgerDisposition := string(disposition)
	if disposition == census.FileSelected {
		ledgerDisposition = FileProcessed
	}
	out.FileLedger.Entries = append(out.FileLedger.Entries, captureset.LedgerEntry{Ordinal: ordinal, Identity: identity, Disposition: ledgerDisposition})
}

func validateSourceFile(f SourceFile, workspace string) error {
	if f.Path == "" || path.IsAbs(f.Path) || path.Clean(f.Path) != f.Path || strings.Contains(f.Path, "\\") || f.Path == "." || f.Path == ".." || strings.HasPrefix(f.Path, "../") {
		return errors.New("canonical workspace-relative slash path required")
	}
	if _, err := canonicalWorkspaceRelativePath(workspace, f.URI); err != nil {
		return err
	} else if rel, _ := canonicalWorkspaceRelativePath(workspace, f.URI); !platformPathEqual(runtimeGOOS(), rel, f.Path) {
		return errors.New("path and URI identity mismatch")
	}
	switch f.Disposition {
	case "", census.FileUnreadable, census.FileUnsupported:
		return nil
	default:
		return errors.New("enumerator supplied invalid pre-disposition")
	}
}

func sourceFileKey(f SourceFile) string {
	return normalizedPlatformPath(runtimeGOOS(), f.Path) + "\x00" + f.URI + "\x00" + f.LanguageID
}
func normalizedPlatformPath(goos, value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	if goos == "windows" {
		return strings.ToLower(value)
	}
	return value
}

func selectPath(includes, excludes []string, candidate string) bool {
	for _, p := range excludes {
		if matchPattern(p, candidate) {
			return false
		}
	}
	if len(includes) == 0 {
		return true
	}
	for _, p := range includes {
		if matchPattern(p, candidate) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, candidate string) bool {
	pattern = strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
	candidate = strings.TrimPrefix(strings.ReplaceAll(candidate, "\\", "/"), "./")
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if !strings.Contains(pattern, "/") && !anchored {
		ok, _ := path.Match(pattern, path.Base(candidate))
		return ok
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(candidate, "/"))
}
func matchSegments(patterns, candidates []string) bool {
	if len(patterns) == 0 {
		return len(candidates) == 0
	}
	if patterns[0] == "**" {
		return matchSegments(patterns[1:], candidates) || (len(candidates) > 0 && matchSegments(patterns, candidates[1:]))
	}
	if len(candidates) == 0 {
		return false
	}
	ok, _ := path.Match(patterns[0], candidates[0])
	return ok && matchSegments(patterns[1:], candidates[1:])
}

func flattenDocumentSymbols(in []lsp.DocumentSymbol, maxNodes, maxDepth, maxStringBytes int) ([]lsp.DocumentSymbol, bool) {
	type frame struct {
		symbols []lsp.DocumentSymbol
		depth   int
	}
	if len(in) > maxNodes {
		return nil, false
	}
	stack := []frame{{symbols: in, depth: 1}}
	out := make([]lsp.DocumentSymbol, 0, len(in))
	stringsSeen := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		f := stack[last]
		stack = stack[:last]
		if f.depth > maxDepth || len(f.symbols) > maxNodes-len(out) {
			return nil, false
		}
		for i := len(f.symbols) - 1; i >= 0; i-- {
			s := f.symbols[i]
			if len(s.Name) > maxStringBytes-stringsSeen {
				return nil, false
			}
			stringsSeen += len(s.Name)
			if len(s.Detail) > maxStringBytes-stringsSeen || len(out) >= maxNodes {
				return nil, false
			}
			stringsSeen += len(s.Detail)
			children := s.Children
			s.Children = nil
			out = append(out, s)
			if len(children) > 0 {
				stack = append(stack, frame{symbols: children, depth: f.depth + 1})
			}
		}
	}
	return out, true
}
func documentSymbolKey(s lsp.DocumentSymbol) string {
	return fmt.Sprintf("%010d:%010d:%05d:%s:%s", s.SelectionRange.Start.Line, s.SelectionRange.Start.Character, s.Kind, s.Name, s.Detail)
}
func callableSymbolKind(kind int) bool { return kind == 6 || kind == 9 || kind == 12 }
func preparedItemMatches(item lsp.CallHierarchyItem, sourceURI string, symbol lsp.DocumentSymbol) bool {
	return item.URI == sourceURI && item.Name == symbol.Name && item.Kind == symbol.Kind && item.Range == symbol.Range && item.SelectionRange.Start == symbol.SelectionRange.Start && validRange(item.Range) && validRange(item.SelectionRange)
}
func validRange(r lsp.Range) bool {
	return r.Start.Line < r.End.Line || (r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
}
func symbolIdentity(file SourceFile, symbol lsp.DocumentSymbol, ordinal int) string {
	return fmt.Sprintf("%s#%d:%d:%d:%s:%d", file.Path, symbol.SelectionRange.Start.Line, symbol.SelectionRange.Start.Character, symbol.Kind, symbol.Name, ordinal)
}
func canonicalPositionSeed(workspace, filePath string, ordinal int, position lsp.Position) ([]byte, error) {
	if position.Line == math.MaxUint32 || position.Character == math.MaxUint32 {
		return nil, errors.New("zero-based coordinate cannot convert to canonical one-based uint32")
	}
	return seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", ordinal), Path: filePath, Line: uint64(position.Line) + 1, Column: uint64(position.Character) + 1}}}}, workspace)
}
func cloneDiscovery(d Discovery) Discovery {
	d.Accounting.Files = append([]census.FileEntry(nil), d.Accounting.Files...)
	d.Accounting.Symbols = append([]census.SymbolEntry(nil), d.Accounting.Symbols...)
	d.FileLedger.Entries = append([]captureset.LedgerEntry(nil), d.FileLedger.Entries...)
	d.SymbolLedger.Entries = append([]captureset.LedgerEntry(nil), d.SymbolLedger.Entries...)
	d.Targets = append([]PreparedTarget(nil), d.Targets...)
	for i := range d.Targets {
		d.Targets[i] = cloneTarget(d.Targets[i])
	}
	return d
}

// StaticFiles is a convenient immutable FileEnumerator for callers that already
// hold a bounded canonical denominator.
type StaticFiles []SourceFile

func (s StaticFiles) Enumerate(context.Context) ([]SourceFile, error) {
	return append([]SourceFile(nil), s...), nil
}

// EnumerateWorkspace converts the shared root-confined source discovery records
// into canonical source-file entries. Directories are not denominator members.
func EnumerateWorkspace(workspace string, roots []string, language func(string) string) ([]SourceFile, error) {
	records, err := source.Discover(source.Request{Base: workspace, Inputs: append([]string(nil), roots...)})
	if err != nil {
		return nil, err
	}
	return workspaceSourceFiles(workspace, records, language), nil
}

// EnumerateWorkspaceContext performs context-aware, traversal-bounded discovery.
// MaxAccepted counts only readable regular files surviving exclusion-first filters.
func EnumerateWorkspaceContext(ctx context.Context, workspace string, roots []string, language func(string) string, filters Filters, limits source.Limits) ([]SourceFile, error) {
	request := source.Request{Base: workspace, Inputs: append([]string(nil), roots...), AllowDot: true}
	request.Accept = func(record source.Record) bool {
		return record.Kind == source.SourceRegularFile && record.Inclusion == source.Included && selectPath(filters.Includes, filters.Excludes, record.Name)
	}
	records, err := source.DiscoverContext(ctx, request, limits)
	if err != nil {
		return nil, err
	}
	return workspaceSourceFiles(workspace, records, language), nil
}

func workspaceSourceFiles(workspace string, records []source.Record, language func(string) string) []SourceFile {
	out := make([]SourceFile, 0, len(records))
	for _, record := range records {
		if record.Kind == source.SourceDirectory {
			continue
		}
		entry := SourceFile{Path: record.Name}
		absolute := filepath.Join(workspace, filepath.FromSlash(record.Name))
		entry.URI = (&url.URL{Scheme: "file", Path: absolute}).String()
		if language != nil {
			entry.LanguageID = language(record.Name)
		}
		if record.Inclusion == source.Unavailable {
			entry.Disposition = census.FileUnreadable
		}
		if record.Inclusion != source.Included && record.Inclusion != source.Unavailable {
			entry.Disposition = census.FileUnsupported
		}
		out = append(out, entry)
	}
	return out
}

var runtimeGOOS = func() string { return runtime.GOOS }
