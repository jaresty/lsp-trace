// Package seedformat defines the transport-neutral lsp-trace.seeds.v2 file
// contract and translates it into the existing acquisition operation model.
// It performs no source acquisition and infers no graph entities.
package seedformat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/strictjson"
)

const (
	Version              = "lsp-trace.seeds.v2"
	CoordinateConvention = "one-based"
	MaxInputBytes        = acquisitionops.MaxInputBytes
	MaxSeeds             = 64
	MaxDepth             = 64
	PositionType         = "position"
	SymbolType           = "symbol"
	SliceType            = "slice"
)

var labelPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,63}$`)

type Defaults struct {
	DownDepth *int `json:"down_depth,omitempty"`
	UpDepth   *int `json:"up_depth,omitempty"`
}

type Position struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Line   uint64 `json:"line"`
	Column uint64 `json:"column"`
}

type Symbol struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Symbol string `json:"symbol"`
}

type Seed struct {
	Type      string
	Position  *Position
	Symbol    *Symbol
	Target    *Seed
	DownDepth *int
	UpDepth   *int
}

func (s Seed) Label() string {
	switch s.Type {
	case PositionType:
		return s.Position.Label
	case SymbolType:
		return s.Symbol.Label
	case SliceType:
		if s.Position != nil {
			return s.Position.Label
		}
	}
	return ""
}

type File struct {
	SchemaVersion        string
	CoordinateConvention string
	Defaults             Defaults
	Seeds                []Seed
}

type fileWire struct {
	SchemaVersion        string            `json:"schema_version"`
	CoordinateConvention string            `json:"coordinate_convention"`
	Defaults             Defaults          `json:"defaults,omitempty"`
	Seeds                []json.RawMessage `json:"seeds"`
}

type kindWire struct {
	Type string `json:"type"`
}
type positionWire struct {
	Type   string `json:"type"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Line   uint64 `json:"line"`
	Column uint64 `json:"column"`
}
type symbolWire struct {
	Type   string `json:"type"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Symbol string `json:"symbol"`
}
type sliceWire struct {
	Type      string          `json:"type"`
	Label     string          `json:"label"`
	Target    json.RawMessage `json:"target"`
	DownDepth *int            `json:"down_depth,omitempty"`
	UpDepth   *int            `json:"up_depth,omitempty"`
}

func decodeObject(raw []byte, dst any) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return errors.New("object required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}

// Decode strictly decodes and validates one seed file against a canonical
// absolute workspace. It reads no files and does not require paths to exist.
func Decode(raw []byte, workspace string) (File, error) {
	var out File
	if len(raw) > MaxInputBytes {
		return out, fmt.Errorf("seed input exceeds %d bytes", MaxInputBytes)
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return out, err
	}
	var wire fileWire
	if err := decodeObject(raw, &wire); err != nil {
		return out, err
	}
	out = File{SchemaVersion: wire.SchemaVersion, CoordinateConvention: wire.CoordinateConvention, Defaults: wire.Defaults, Seeds: make([]Seed, len(wire.Seeds))}
	for i, encoded := range wire.Seeds {
		seed, err := decodeSeed(encoded, false)
		if err != nil {
			return File{}, fmt.Errorf("seeds[%d]: %w", i, err)
		}
		out.Seeds[i] = seed
	}
	if err := Validate(out, workspace); err != nil {
		return File{}, err
	}
	return out, nil
}

func decodeSeed(raw []byte, nested bool) (Seed, error) {
	var kind kindWire
	if err := decodeObject(raw, &kind); err != nil && !strings.Contains(err.Error(), "unknown field") {
		return Seed{}, err
	}
	switch kind.Type {
	case PositionType:
		var w positionWire
		if err := decodeObject(raw, &w); err != nil {
			return Seed{}, err
		}
		return Seed{Type: w.Type, Position: &Position{Label: w.Label, Path: w.Path, Line: w.Line, Column: w.Column}}, nil
	case SymbolType:
		var w symbolWire
		if err := decodeObject(raw, &w); err != nil {
			return Seed{}, err
		}
		return Seed{Type: w.Type, Symbol: &Symbol{Label: w.Label, Path: w.Path, Symbol: w.Symbol}}, nil
	case SliceType:
		if nested {
			return Seed{}, errors.New("slice target cannot be slice")
		}
		var w sliceWire
		if err := decodeObject(raw, &w); err != nil {
			return Seed{}, err
		}
		target, err := decodeSeed(w.Target, true)
		if err != nil {
			return Seed{}, fmt.Errorf("target: %w", err)
		}
		return Seed{Type: w.Type, Position: &Position{Label: w.Label}, Target: &target, DownDepth: w.DownDepth, UpDepth: w.UpDepth}, nil
	default:
		return Seed{}, fmt.Errorf("type must be %q, %q, or %q", PositionType, SymbolType, SliceType)
	}
}

// Validate applies deterministic semantic validation without filesystem access.
func Validate(file File, workspace string) error {
	if file.SchemaVersion != Version {
		return fmt.Errorf("schema_version must be %q", Version)
	}
	if file.CoordinateConvention != CoordinateConvention {
		return fmt.Errorf("coordinate_convention must be %q", CoordinateConvention)
	}
	if !canonicalWorkspace(workspace) {
		return errors.New("workspace must be a canonical absolute path")
	}
	if len(file.Seeds) < 1 || len(file.Seeds) > MaxSeeds {
		return fmt.Errorf("seeds must contain 1..%d entries", MaxSeeds)
	}
	if err := validateDepth(file.Defaults.DownDepth, "defaults.down_depth"); err != nil {
		return err
	}
	if err := validateDepth(file.Defaults.UpDepth, "defaults.up_depth"); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, seed := range file.Seeds {
		label := seed.Label()
		if !labelPattern.MatchString(label) {
			return fmt.Errorf("seeds[%d].label is invalid", i)
		}
		if seen[label] {
			return fmt.Errorf("duplicate seed label %q", label)
		}
		seen[label] = true
		if err := validateSeed(seed, workspace, false); err != nil {
			return fmt.Errorf("seeds[%d]: %w", i, err)
		}
	}
	return nil
}

func validateSeed(seed Seed, workspace string, nested bool) error {
	switch seed.Type {
	case PositionType:
		if seed.Position == nil || seed.Symbol != nil || seed.Target != nil {
			return errors.New("position has invalid representation")
		}
		if !labelPattern.MatchString(seed.Position.Label) {
			return errors.New("label is invalid")
		}
		if _, err := resolvePath(workspace, seed.Position.Path); err != nil {
			return err
		}
		if seed.Position.Line < 1 || seed.Position.Column < 1 || seed.Position.Line > math.MaxUint32+1 || seed.Position.Column > math.MaxUint32+1 {
			return errors.New("line and column must be positive one-based uint32 coordinates")
		}
	case SymbolType:
		if seed.Symbol == nil || seed.Position != nil || seed.Target != nil {
			return errors.New("symbol has invalid representation")
		}
		if !labelPattern.MatchString(seed.Symbol.Label) {
			return errors.New("label is invalid")
		}
		if _, err := resolvePath(workspace, seed.Symbol.Path); err != nil {
			return err
		}
		if seed.Symbol.Symbol == "" || strings.TrimSpace(seed.Symbol.Symbol) != seed.Symbol.Symbol {
			return errors.New("symbol must be nonempty and trimmed")
		}
	case SliceType:
		if nested || seed.Target == nil || seed.Position == nil || seed.Position.Label == "" || seed.Symbol != nil {
			return errors.New("slice target must be exactly position or symbol")
		}
		if seed.Target.Type == SliceType {
			return errors.New("slice target cannot be slice")
		}
		if err := validateDepth(seed.DownDepth, "down_depth"); err != nil {
			return err
		}
		if err := validateDepth(seed.UpDepth, "up_depth"); err != nil {
			return err
		}
		if err := validateSeed(*seed.Target, workspace, true); err != nil {
			return fmt.Errorf("target: %w", err)
		}
	default:
		return errors.New("unknown seed type")
	}
	return nil
}

func validateDepth(value *int, name string) error {
	if value != nil && (*value < 0 || *value > MaxDepth) {
		return fmt.Errorf("%s must be between 0 and %d", name, MaxDepth)
	}
	return nil
}

func canonicalWorkspace(workspace string) bool {
	return workspace != "" && filepath.IsAbs(workspace) && filepath.Clean(workspace) == workspace
}

func resolvePath(workspace, name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\x00') || filepath.ToSlash(name) != name || filepath.Clean(name) != name || name == "." {
		return "", errors.New("path must be normalized")
	}
	resolved := name
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(workspace, filepath.FromSlash(name))
	}
	if filepath.Clean(resolved) != resolved {
		return "", errors.New("path must be canonical")
	}
	rel, err := filepath.Rel(workspace, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path must resolve within workspace")
	}
	return resolved, nil
}

type TranslateOptions struct {
	Workspace       string
	Limits          acquisitionops.Limits
	TopmostSiblings bool
}

// Translate creates a slice acquisition manifest. It preserves seed order by
// mapping the first seed to Root and each later seed to RequiredTargets.
func Translate(file File, options TranslateOptions) (acquisitionops.Manifest, error) {
	var out acquisitionops.Manifest
	if err := Validate(file, options.Workspace); err != nil {
		return out, err
	}
	if err := requireLimits(options.Limits); err != nil {
		return out, err
	}
	used := map[string]bool{}
	targets := make([]acquisitionops.Target, len(file.Seeds))
	for i, seed := range file.Seeds {
		id := seed.Label()
		if i == 0 {
			id = "root"
		} else {
			id = collisionSafeID(id, used)
		}
		used[id] = true
		target, err := translateSeed(seed, file.Defaults, options.Workspace, id)
		if err != nil {
			return out, err
		}
		targets[i] = target
	}
	required := make([]acquisitionops.Target, len(targets)-1)
	copy(required, targets[1:])
	out = acquisitionops.Manifest{SchemaVersion: acquisitionops.ManifestVersion, CoordinateConvention: "zero-based-session", Root: targets[0], RequiredTargets: required, Limits: options.Limits, Expansion: acquisitionops.Expansion{TopmostSiblings: options.TopmostSiblings}}
	if _, err := out.Request(acquisitionops.Slice); err != nil {
		return acquisitionops.Manifest{}, err
	}
	return out, nil
}

func collisionSafeID(label string, used map[string]bool) string {
	candidate := label
	if candidate == "root" {
		candidate = "seed-root"
	}
	if !used[candidate] {
		return candidate
	}
	base := candidate
	for suffix := 2; ; suffix++ {
		candidate = fmt.Sprintf("%s-%d", base, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

func translateSeed(seed Seed, defaults Defaults, workspace, id string) (acquisitionops.Target, error) {
	down, up := defaults.DownDepth, defaults.UpDepth
	base := seed
	if seed.Type == SliceType {
		base = *seed.Target
		if seed.DownDepth != nil {
			down = seed.DownDepth
		}
		if seed.UpDepth != nil {
			up = seed.UpDepth
		}
	}
	if down == nil {
		down = ptrInt(2)
	}
	if up == nil {
		up = ptrInt(2)
	}
	locator := acquisition.Locator{}
	switch base.Type {
	case PositionType:
		absolute, err := resolvePath(workspace, base.Position.Path)
		if err != nil {
			return acquisitionops.Target{}, err
		}
		line, character := uint32(base.Position.Line-1), uint32(base.Position.Column-1)
		locator = acquisition.Locator{URI: (&url.URL{Scheme: "file", Path: absolute}).String(), Line: &line, Character: &character}
	case SymbolType:
		absolute, err := resolvePath(workspace, base.Symbol.Path)
		if err != nil {
			return acquisitionops.Target{}, err
		}
		locator = acquisition.Locator{URI: (&url.URL{Scheme: "file", Path: absolute}).String(), Symbol: base.Symbol.Symbol}
	default:
		return acquisitionops.Target{}, errors.New("untranslatable seed type")
	}
	return acquisitionops.Target{ID: id, Locator: locator, DownDepth: ptrInt(*down), UpDepth: ptrInt(*up)}, nil
}

func ptrInt(value int) *int { return &value }

func requireLimits(l acquisitionops.Limits) error {
	if l.MaxNodes == nil || l.MaxRequests == nil || l.MaxEvidenceBytes == nil || l.MaxPathWork == nil || l.TimeoutMS == nil || l.RequestTimeoutMS == nil || l.MaxResponseBytes == nil || l.MaxMessages == nil {
		return errors.New("all acquisition global limits must be caller-supplied")
	}
	return nil
}
