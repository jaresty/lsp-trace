package scopejob

import (
	"errors"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// SelectionVersion freezes declared eligibility, never server-confirmed support.
const SelectionVersion = "lsp-trace.declared-source-selection.v1"
const MaxAssociations = 64
const MaxLanguageIDs = 32
const MaxSelectionString = 128

var ErrAssociationConflict = errors.New("FILE_ASSOCIATION_CONFLICT")

type Association struct {
	Pattern  string `json:"pattern"`
	Language string `json:"language"`
}
type SelectionPolicy struct {
	Version         string        `json:"version"`
	DefaultLanguage string        `json:"default_language"`
	LanguageIDs     []string      `json:"language_ids"`
	Associations    []Association `json:"associations"`
}

// Language identifiers are declarations, not an enum of qualified providers.
func ValidLanguageID(s string) bool {
	if len(s) == 0 || len(s) > MaxSelectionString {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '+' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}
func validAssociationPattern(s string) bool {
	if !utf8.ValidString(s) || s == "" || len(s) > MaxSelectionString || s != strings.TrimSpace(s) || strings.ContainsAny(s, "\\\x00") || strings.Contains(s, "**") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "!") || strings.HasSuffix(s, "/") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	_, err := path.Match(s, "x")
	return err == nil
}

// FreezeSelection merges exact keys before resolving overlap. New defaults never
// participate in readback: the complete effective mapping is stored in the job.
func FreezeSelection(ids []string, project map[string]string) (*SelectionPolicy, error) {
	if len(ids) == 0 || len(ids) > MaxLanguageIDs || len(project) > MaxAssociations {
		return nil, ErrInvalid
	}
	p := &SelectionPolicy{Version: SelectionVersion, DefaultLanguage: ids[0], LanguageIDs: slices.Clone(ids)}
	slices.Sort(p.LanguageIDs)
	for i, id := range p.LanguageIDs {
		if !ValidLanguageID(id) || i > 0 && id == p.LanguageIDs[i-1] {
			return nil, ErrInvalid
		}
	}
	rules := map[string]string{"*.go": "go", "*.ts": "typescript", "*.tsx": "typescriptreact", "*.js": "javascript", "*.jsx": "javascriptreact", "*.cs": "csharp", "*.ex": "elixir", "*.exs": "elixir", "*.py": "python", "*.rs": "rust"}
	for pattern, id := range project {
		rules[pattern] = id
	}
	for pattern, id := range rules {
		p.Associations = append(p.Associations, Association{pattern, id})
	}
	slices.SortFunc(p.Associations, func(a, b Association) int { return strings.Compare(a.Pattern, b.Pattern) })
	if !p.valid() {
		return nil, ErrInvalid
	}
	return p, nil
}
func (p *SelectionPolicy) valid() bool {
	if p == nil || p.Version != SelectionVersion || !ValidLanguageID(p.DefaultLanguage) || len(p.LanguageIDs) == 0 || len(p.LanguageIDs) > MaxLanguageIDs || len(p.Associations) == 0 || len(p.Associations) > MaxAssociations {
		return false
	}
	found := false
	for i, id := range p.LanguageIDs {
		if !ValidLanguageID(id) || i > 0 && id <= p.LanguageIDs[i-1] {
			return false
		}
		found = found || id == p.DefaultLanguage
	}
	if !found {
		return false
	}
	for i, a := range p.Associations {
		if !validAssociationPattern(a.Pattern) || !ValidLanguageID(a.Language) || i > 0 && a.Pattern <= p.Associations[i-1].Pattern {
			return false
		}
	}
	return true
}
func (p *SelectionPolicy) clone() *SelectionPolicy {
	out := *p
	out.LanguageIDs = slices.Clone(p.LanguageIDs)
	out.Associations = slices.Clone(p.Associations)
	return &out
}

// Resolve requires a validated frozen policy. Matching is case-sensitive. A
// slashless pattern matches the basename at any depth; other patterns match the
// entire workspace-relative path using path.Match (no recursive **).
func (p *SelectionPolicy) Resolve(file string) (string, error) {
	language := ""
	for _, a := range p.Associations {
		candidate := file
		if !strings.Contains(a.Pattern, "/") {
			candidate = path.Base(file)
		}
		match, _ := path.Match(a.Pattern, candidate)
		if match {
			if language != "" && language != a.Language {
				return "", ErrAssociationConflict
			}
			language = a.Language
		}
	}
	return language, nil
}
func (p *SelectionPolicy) eligible(language string) bool {
	return slices.Contains(p.LanguageIDs, language)
}

// Prepaid upper bound for both classification and replay: byte comparisons in
// bounded path.Match plus membership scans. No validation scan occurs per file.
func (p *SelectionPolicy) work(pathBytes int) uint64 {
	var bytes uint64
	for _, a := range p.Associations {
		bytes += uint64(len(a.Pattern) + 1)
	}
	return 2 * (bytes*uint64(pathBytes+1) + uint64(len(p.LanguageIDs)*MaxSelectionString))
}
