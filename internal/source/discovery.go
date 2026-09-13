package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SourceKind string

const (
	SourceRegularFile SourceKind = "REGULAR_FILE"
	SourceDirectory   SourceKind = "DIRECTORY"
	SourceOther       SourceKind = "OTHER"
)

type Inclusion string

const (
	Included    Inclusion = "INCLUDED"
	Unsupported Inclusion = "UNSUPPORTED"
	Unavailable Inclusion = "UNAVAILABLE"
	OutsideBase Inclusion = "OUTSIDE_BASE"
)

var (
	ErrAmbiguousRequest = errors.New("ambiguous source discovery request")
	ErrDiscoveryLimit   = errors.New("source discovery limit exceeded")
	ErrUnsafeSource     = errors.New("unsafe source traversal")
)

type Request struct {
	Base     string
	Inputs   []string
	AllowDot bool
	// Accept identifies denominator members for MaxAccepted. It is evaluated
	// only for unique records and must not retain the supplied record.
	Accept func(Record) bool
}

// Limits bounds traversal itself. Every field must be positive. MaxEntries
// counts unique discovered records; MaxWork counts walker callbacks, including
// duplicates and directory entries.
type Limits struct {
	MaxEntries   int
	MaxAccepted  int
	MaxWork      int
	MaxPathBytes int
	MaxDepth     int
}

type Record struct {
	Name      string
	Kind      SourceKind
	Inclusion Inclusion
	Reason    string
}

type walkDirFunc func(string, fs.WalkDirFunc) error

// Discover retains the historical unbounded API for existing callers.
func Discover(request Request) ([]Record, error) {
	return discoverContext(context.Background(), request, Limits{}, filepath.WalkDir)
}

// DiscoverContext enumerates with cancellation and explicit finite traversal
// limits. Limit failures intentionally carry neither roots nor paths.
func DiscoverContext(ctx context.Context, request Request, limits Limits) ([]Record, error) {
	if limits.MaxEntries < 1 || limits.MaxAccepted < 0 || limits.MaxWork < 1 || limits.MaxPathBytes < 1 || limits.MaxDepth < 1 {
		return nil, ErrDiscoveryLimit
	}
	return discoverContext(ctx, request, limits, filepath.WalkDir)
}

func discoverContext(ctx context.Context, request Request, limits Limits, walk walkDirFunc) ([]Record, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base, err := filepath.Abs(request.Base)
	if err != nil {
		return nil, fmt.Errorf("resolve source discovery base: %w", err)
	}
	base = filepath.Clean(base)
	info, err := os.Stat(base)
	if err != nil {
		return nil, fmt.Errorf("inspect source discovery base: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source discovery base is not a directory: %s", base)
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil, ErrUnsafeSource
	}
	realBase = filepath.Clean(realBase)
	if len(request.Inputs) == 0 {
		return nil, ErrAmbiguousRequest
	}
	bounded := limits.MaxEntries > 0 || limits.MaxAccepted > 0 || limits.MaxWork > 0 || limits.MaxPathBytes > 0 || limits.MaxDepth > 0
	if bounded && (limits.MaxEntries < 1 || limits.MaxAccepted < 0 || limits.MaxWork < 1 || limits.MaxPathBytes < 1 || limits.MaxDepth < 1) {
		return nil, ErrDiscoveryLimit
	}

	records := make(map[string]Record)
	work, accepted := 0, 0
	add := func(key string, record Record) error {
		if _, exists := records[key]; exists {
			return nil
		}
		if bounded && len(records) >= limits.MaxEntries {
			return ErrDiscoveryLimit
		}
		if limits.MaxAccepted > 0 && request.Accept != nil && request.Accept(record) {
			if accepted >= limits.MaxAccepted {
				return ErrDiscoveryLimit
			}
			accepted++
		}
		records[key] = record
		return nil
	}
	for _, input := range request.Inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if input == "" || filepath.Clean(input) == "." && !request.AllowDot {
			return nil, ErrAmbiguousRequest
		}
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve source input: %w", err)
		}
		path = filepath.Clean(path)
		rel, inside := relativeToBase(base, path)
		if !inside {
			name := filepath.ToSlash(filepath.Join("_outside", filepath.Base(path)))
			if err := add(path, Record{Name: name, Kind: SourceOther, Inclusion: OutsideBase, Reason: "outside bounded base"}); err != nil {
				return nil, err
			}
			continue
		}
		if bounded && (len(rel) > limits.MaxPathBytes || pathDepth(rel) > limits.MaxDepth) {
			return nil, ErrDiscoveryLimit
		}
		if _, exists := records[path]; exists {
			continue
		}
		entry, statErr := os.Lstat(path)
		if statErr != nil {
			if err := add(path, Record{Name: rel, Kind: SourceOther, Inclusion: Unavailable, Reason: unavailableReason(statErr)}); err != nil {
				return nil, err
			}
			continue
		}
		if entry.IsDir() {
			err = walk(path, func(child string, dirEntry fs.DirEntry, walkErr error) error {
				work++
				if err := ctx.Err(); err != nil {
					return err
				}
				if bounded && work > limits.MaxWork {
					return ErrDiscoveryLimit
				}
				childRel, childInside := relativeToBase(base, child)
				if !childInside {
					return filepath.SkipDir
				}
				if bounded && (len(childRel) > limits.MaxPathBytes || pathDepth(childRel) > limits.MaxDepth) {
					return ErrDiscoveryLimit
				}
				if walkErr != nil {
					if err := add(child, Record{Name: childRel, Kind: SourceOther, Inclusion: Unavailable, Reason: unavailableReason(walkErr)}); err != nil {
						return err
					}
					if dirEntry != nil && dirEntry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if dirEntry.Type()&fs.ModeSymlink == 0 {
					resolved, resolveErr := filepath.EvalSymlinks(child)
					if resolveErr != nil {
						return ErrUnsafeSource
					}
					if _, stable := relativeToBase(realBase, filepath.Clean(resolved)); !stable {
						return ErrUnsafeSource
					}
				}
				return add(child, classify(childRel, dirEntry.Type()))
			})
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrDiscoveryLimit) || errors.Is(err, ErrUnsafeSource) {
					return nil, err
				}
				return nil, fmt.Errorf("discover source directory: %w", err)
			}
			continue
		}
		if err := add(path, classify(rel, entry.Mode())); err != nil {
			return nil, err
		}
	}

	result := make([]Record, 0, len(records))
	for _, record := range records {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return recordSortKey(result[i]) < recordSortKey(result[j]) })
	return result, nil
}

func pathDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func relativeToBase(base, path string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(filepath.Clean(rel)), true
}

func classify(name string, mode fs.FileMode) Record {
	switch {
	case mode.IsRegular():
		return Record{Name: name, Kind: SourceRegularFile, Inclusion: Included}
	case mode.IsDir():
		return Record{Name: name, Kind: SourceDirectory, Inclusion: Included}
	default:
		return Record{Name: name, Kind: SourceOther, Inclusion: Unsupported, Reason: "unsupported filesystem kind"}
	}
}

func unavailableReason(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return "source input does not exist"
	}
	if errors.Is(err, fs.ErrPermission) {
		return "source input is inaccessible"
	}
	return "source input is unavailable"
}

func recordSortKey(record Record) string {
	return record.Name + "\x00" + string(record.Kind) + "\x00" + string(record.Inclusion) + "\x00" + record.Reason
}
