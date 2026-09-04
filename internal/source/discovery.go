package source

import (
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

var ErrAmbiguousRequest = errors.New("ambiguous source discovery request")

type Request struct {
	Base   string
	Inputs []string
}

type Record struct {
	Name      string
	Kind      SourceKind
	Inclusion Inclusion
	Reason    string
}

func Discover(request Request) ([]Record, error) {
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
	if len(request.Inputs) == 0 {
		return nil, ErrAmbiguousRequest
	}

	records := make(map[string]Record)
	for _, input := range request.Inputs {
		if input == "" || filepath.Clean(input) == "." {
			return nil, ErrAmbiguousRequest
		}
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve source input %q: %w", input, err)
		}
		path = filepath.Clean(path)
		rel, inside := relativeToBase(base, path)
		if !inside {
			name := filepath.ToSlash(filepath.Join("_outside", filepath.Base(path)))
			records[path] = Record{Name: name, Kind: SourceOther, Inclusion: OutsideBase, Reason: "outside bounded base"}
			continue
		}
		if _, exists := records[path]; exists {
			continue
		}
		entry, statErr := os.Lstat(path)
		if statErr != nil {
			records[path] = Record{Name: rel, Kind: SourceOther, Inclusion: Unavailable, Reason: unavailableReason(statErr)}
			continue
		}
		if entry.IsDir() {
			err = filepath.WalkDir(path, func(child string, dirEntry fs.DirEntry, walkErr error) error {
				childRel, childInside := relativeToBase(base, child)
				if !childInside {
					return filepath.SkipDir
				}
				if walkErr != nil {
					records[child] = Record{Name: childRel, Kind: SourceOther, Inclusion: Unavailable, Reason: unavailableReason(walkErr)}
					if dirEntry != nil && dirEntry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				records[child] = classify(childRel, dirEntry.Type())
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("discover source directory %q: %w", input, err)
			}
			continue
		}
		records[path] = classify(rel, entry.Mode())
	}

	result := make([]Record, 0, len(records))
	for _, record := range records {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return recordSortKey(result[i]) < recordSortKey(result[j]) })
	return result, nil
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
