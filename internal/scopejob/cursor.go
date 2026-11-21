package scopejob

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/source"
)

type Record struct {
	LanguageID         string `json:"language_id,omitempty"`
	Path               string `json:"path"`
	Kind               string `json:"kind"`
	Role               string `json:"role"`
	Size               int64  `json:"size"`
	Mode               uint32 `json:"mode"`
	ModifiedNS         int64  `json:"modified_ns"`
	MetadataObserved   bool   `json:"metadata_observed"`
	UnknownDescendants bool   `json:"unknown_descendants"`
}
type Counts struct {
	Entries             uint64 `json:"entries"`
	Regular             uint64 `json:"regular"`
	Directories         uint64 `json:"directories"`
	Selected            uint64 `json:"supported_unacquired"`
	Excluded            uint64 `json:"excluded"`
	UnsupportedLanguage uint64 `json:"unsupported_language"`
	NotSource           uint64 `json:"unknown_extension_not_source"`
	UnsupportedKind     uint64 `json:"unsupported_kind"`
	Unavailable         uint64 `json:"unavailable"`
	Boundaries          uint64 `json:"unknown_descendant_boundaries"`
}

func (n *Counts) add(r Record) {
	n.Entries++
	if r.Kind == "REGULAR_FILE" {
		n.Regular++
	}
	if r.Kind == "DIRECTORY" {
		n.Directories++
	}
	if r.UnknownDescendants {
		n.Boundaries++
	}
	switch r.Role {
	case "SUPPORTED_UNACQUIRED":
		n.Selected++
	case "EXCLUDED":
		n.Excluded++
	case "UNSUPPORTED_LANGUAGE":
		n.UnsupportedLanguage++
	case "NOT_SOURCE":
		n.NotSource++
	case "UNSUPPORTED_KIND":
		n.UnsupportedKind++
	case "UNAVAILABLE":
		n.Unavailable++
	}
}

type frame struct {
	path string
	root *os.Root
	file *os.File
}
type cursor struct {
	selection regularSelector
	config    Config
	base      *os.Root
	roots     []string
	frames    []frame
	counts    Counts
}

func newCursor(c Config) (*cursor, error) {
	// Reject aliases rather than silently changing the frozen workspace identity.
	real, e := filepath.EvalSymlinks(c.Workspace)
	if e != nil || real != c.Workspace {
		return nil, ErrInvalid
	}
	root, e := os.OpenRoot(c.Workspace)
	if e != nil {
		return nil, e
	}
	// Own the operation's frozen inputs even if a caller later mutates its DTO.
	c.Roots = append([]string(nil), c.Roots...)
	c.Prunes = append([]string(nil), c.Prunes...)
	c.Includes = append([]string(nil), c.Includes...)
	c.Excludes = append([]string(nil), c.Excludes...)
	if c.Selection != nil {
		c.Selection = c.Selection.clone()
	}
	return &cursor{config: c, base: root, roots: append([]string(nil), c.Roots...)}, nil
}
func (c *cursor) close() error {
	var e error
	for i := len(c.frames) - 1; i >= 0; i-- {
		e = errors.Join(e, c.frames[i].file.Close(), c.frames[i].root.Close())
	}
	c.frames = nil
	if c.base != nil {
		e = errors.Join(e, c.base.Close())
		c.base = nil
	}
	return e
}
func (c *cursor) frontier() []string {
	out := append([]string(nil), c.roots...)
	for _, f := range c.frames {
		out = append(out, f.path)
	}
	return out
}
func (c *cursor) done() bool { return len(c.roots) == 0 && len(c.frames) == 0 }

// step consumes one root, directory entry, or directory EOF. It never reads
// source contents, materializes a directory, or reopens an offset on restart.
func (c *cursor) step() (*Record, error) {
	if c.done() {
		return nil, nil
	}
	var parent *os.Root
	var name, path string
	if len(c.frames) == 0 {
		parent = c.base
		path = c.roots[0]
		name = path
		// Check before the first prefix Lstat, even for an internally constructed
		// cursor bypassing Canonical. A rejected root leaves the frontier intact.
		if !local(path, c.config.Budget.PathBytes) || pathDepth(path) > c.config.Budget.Depth {
			return nil, ErrInvalid
		}
		// An explicit root may contain intermediate components. Reject every symlink
		// before opening it; OpenRoot separately confines racing resolution.
		components := strings.Split(path, "/")
		prefix := ""
		for _, part := range components {
			prefix = filepath.Join(prefix, part)
			info, e := parent.Lstat(prefix)
			if e != nil {
				break
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, ErrInvalid
			}
		}
		c.roots = c.roots[1:]
	} else {
		f := &c.frames[len(c.frames)-1]
		entries, e := f.file.ReadDir(1)
		if len(entries) == 0 {
			if e != nil && !errors.Is(e, io.EOF) {
				return nil, e
			}
			if e == nil {
				return nil, ErrInvalid
			}
			e = errors.Join(f.file.Close(), f.root.Close())
			c.frames = c.frames[:len(c.frames)-1]
			return nil, e
		}
		if e != nil && !errors.Is(e, io.EOF) {
			return nil, e
		}
		parent = f.root
		name = entries[0].Name()
		path = filepath.ToSlash(filepath.Join(f.path, name))
	}
	if !local(path, c.config.Budget.PathBytes) || pathDepth(path) > c.config.Budget.Depth {
		return nil, ErrInvalid
	}
	if c.counts.Entries >= c.config.Budget.Entries {
		return nil, errEntries
	}
	r := Record{Path: path, Kind: "OTHER", Role: "UNAVAILABLE"}
	info, e := parent.Lstat(name)
	if e != nil {
		r.UnknownDescendants = true
		c.counts.add(r)
		return &r, nil
	}
	r.MetadataObserved = true
	r.Size = info.Size()
	r.Mode = uint32(info.Mode())
	r.ModifiedNS = info.ModTime().UnixNano()
	switch {
	case info.Mode().IsRegular():
		r.Kind = "REGULAR_FILE"
		r.Role, r.LanguageID, e = c.selection.selectRegular(c.config, path)
		if e != nil {
			return nil, e
		}
	case info.IsDir():
		r.Kind = "DIRECTORY"
		r.Role = "DIRECTORY"
		if c.counts.Directories >= c.config.Budget.Directories {
			return nil, errDirectories
		}
		for _, p := range c.config.Prunes {
			if beneath(p, path) {
				r.Role = "EXCLUDED"
				r.UnknownDescendants = true
				c.counts.add(r)
				return &r, nil
			}
		}
		if !frameAvailable(c.config, len(c.frames)) {
			return nil, ErrInvalid
		}
		child, e := parent.OpenRoot(name)
		if e != nil {
			r.Role = "UNAVAILABLE"
			r.UnknownDescendants = true
			break
		}
		f, e := child.Open(".")
		if e != nil {
			_ = child.Close()
			r.Role = "UNAVAILABLE"
			r.UnknownDescendants = true
			break
		}
		opened, e := f.Stat()
		if e != nil || !os.SameFile(info, opened) {
			_ = f.Close()
			_ = child.Close()
			return nil, ErrInvalid
		}
		c.frames = append(c.frames, frame{path, child, f})
	default:
		r.Role = "UNSUPPORTED_KIND"
	}
	c.counts.add(r)
	return &r, nil
}

// Both traversal and artifact verification use the same source-selection owner.
// Unsupported/unknown language takes precedence over selection filters.
func regularRole(c Config, path string) (string, error) {
	role, _, err := regularSelection(c, path)
	return role, err
}
func regularSelection(c Config, path string) (role, recordedLanguage string, err error) {
	var selection regularSelector
	return selection.selectRegular(c, path)
}

// Preparation is lazy so language eligibility and FileURI errors retain their
// historical precedence. A selector belongs to one frozen operation, not a DTO.
type regularSelector struct {
	prepared    *censusacquisition.PreparedSourceSelection
	err         error
	initialized bool
}

// Process-local seam for counting preparation in bounded tests, not authority.
var prepareSourceSelection = censusacquisition.PrepareSourceSelection

func (s *regularSelector) selectRegular(c Config, path string) (role, recordedLanguage string, err error) {
	language := source.LanguageID(path)
	if c.Selection != nil {
		language, err = c.Selection.Resolve(path)
		if err != nil {
			return "", "", err
		}
		recordedLanguage = language
	}
	if language == "" {
		return "NOT_SOURCE", recordedLanguage, nil
	}
	if c.Selection == nil && language != c.Language || c.Selection != nil && !c.Selection.eligible(language) {
		return "UNSUPPORTED_LANGUAGE", recordedLanguage, nil
	}
	uri, e := source.FileURI(filepath.Join(c.Workspace, path))
	if e != nil {
		return "", recordedLanguage, e
	}
	if !s.initialized {
		s.prepared, s.err = prepareSourceSelection(c.Workspace, censusacquisition.Filters{Includes: c.Includes, Excludes: c.Excludes})
		s.initialized = true
	}
	if s.err != nil {
		return "", recordedLanguage, s.err
	}
	selected, e := s.prepared.Select(censusacquisition.SourceFile{Path: path, URI: uri, LanguageID: language})
	if e != nil {
		return "", recordedLanguage, e
	}
	if selected {
		return "SUPPORTED_UNACQUIRED", recordedLanguage, nil
	}
	return "EXCLUDED", recordedLanguage, nil
}

var errEntries = errors.New("ENTRY_LIMIT")
var errDirectories = errors.New("DIRECTORY_LIMIT")
