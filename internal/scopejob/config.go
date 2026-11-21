// Package scopejob implements a metadata-only, non-resumable scope-job candidate.
// It cannot acquire native evidence or publish an inventory.
package scopejob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/discoveryfilter"
)

const Schema = "lsp-trace.scope-job.v2"
const LegacySchema = "lsp-trace.scope-job.v1"

// NamespaceWork prepays the fixed child admission, header/receipt, and closure
// operations. Namespace logical bytes are included in the fixed HeadBytes reserve.
const NamespaceWork = uint64(32)
const NamespaceBytes = uint64(4096) // carved out of HeadBytes, defaults unchanged
const (
	BatchSteps   = uint64(128)
	HeadBytes    = uint64(256 << 10)
	ClaimBytes   = uint64(2 << 20)
	SegmentBytes = uint64(4 << 20)
	StopBytes    = uint64(2 << 20)
	// Publication writes and verifies exact bytes; four encoded-buffer passes
	// conservatively charge encoding, write, verification and later readback.
	BatchBytes = 4 * (ClaimBytes + SegmentBytes)
)

var ErrInvalid = errors.New("invalid scope job contract or artifact")
var ErrIntegrity = errors.New("scope job integrity failure")
var ErrUnsupportedVersion = errors.New("unsupported scope job stored version or layout")

type Budget struct {
	OperationTimeout time.Duration `json:"operation_timeout_ns"`
	RetainedBytes    uint64        `json:"retained_bytes"`
	LogicalBytes     uint64        `json:"logical_bytes"`
	Work             uint64        `json:"work"`
	Steps            uint64        `json:"steps"`
	Packets          uint64        `json:"packets"`
	Entries          uint64        `json:"entries"`
	Directories      uint64        `json:"directories"`
	PathBytes        int           `json:"path_bytes"`
	Depth            int           `json:"depth"`
}

// Defaults are provisional bounded-operation allowances, not measured fit.
func DefaultBudget() Budget {
	entries := uint64(1 << 19)
	steps := 2*entries + 128 // each entry plus directory EOF/close, plus roots
	batches := (steps + BatchSteps - 1) / BatchSteps
	return Budget{30 * time.Second, 2 << 30, batches*BatchBytes + 4*(HeadBytes+StopBytes), steps * (8 + 2*128 + 128 + 128 + 2*256), steps, 2*batches + 2, entries, entries, 1024, 128}
}

type Config struct {
	Workspace string   `json:"workspace"`
	Roots     []string `json:"roots"`
	Includes  []string `json:"includes"`
	Excludes  []string `json:"excludes"`
	Prunes    []string `json:"pruned_directories"`
	Profile   string   `json:"profile"`
	Language  string   `json:"language"`
	Budget    Budget   `json:"budget"`
	// Null is the canonical disabled overall deadline. Operation timeout is separate.
	JobDeadline *time.Time       `json:"job_deadline"`
	Selection   *SelectionPolicy `json:"selection_policy,omitempty"`
}

func canonical(xs []string) []string {
	ys := slices.Clone(xs)
	slices.Sort(ys)
	return slices.Compact(ys)
}
func local(p string, max int) bool {
	return utf8.ValidString(p) && !strings.ContainsRune(p, 0) && len(p) <= max && filepath.IsLocal(p) && filepath.ToSlash(filepath.Clean(p)) == p && !strings.Contains(p, "\\")
}

// Depth counts workspace-relative components, including "." as one, as in
// traversal. It never counts components of the absolute workspace prefix.
func pathDepth(p string) int { return strings.Count(p, "/") + 1 }

func beneath(parent, child string) bool {
	return parent == "." || child == parent || strings.HasPrefix(child, parent+"/")
}
func Canonical(c Config) (Config, error) {
	b := c.Budget
	if c.Selection != nil {
		if !c.Selection.valid() || (c.Profile == "" || len(c.Profile) > 128 || !utf8.ValidString(c.Profile) || strings.TrimSpace(c.Profile) != c.Profile || strings.ContainsRune(c.Profile, 0)) || c.Language != c.Selection.DefaultLanguage {
			return c, ErrInvalid
		}
		c.Selection = c.Selection.clone()
	} else if c.Profile != "gopls" || c.Language != "go" {
		return c, ErrInvalid
	}
	// Closed implementation ceilings bound malicious stored configuration and all arithmetic.
	if !filepath.IsAbs(c.Workspace) || filepath.Clean(c.Workspace) != c.Workspace || !utf8.ValidString(c.Workspace) || strings.ContainsRune(c.Workspace, 0) || len(c.Workspace) > 4096 || b.OperationTimeout <= 0 || b.OperationTimeout > time.Hour || b.RetainedBytes < HeadBytes+StopBytes || b.RetainedBytes > 1<<40 || b.LogicalBytes < 4*(HeadBytes+StopBytes) || b.LogicalBytes > 1<<50 || b.Work == 0 || b.Work > 1<<40 || b.Steps == 0 || b.Steps > 1<<30 || b.Packets < 2 || b.Packets > 1<<25 || b.Entries == 0 || b.Entries > 1<<28 || b.Directories == 0 || b.Directories > b.Entries || b.PathBytes < 1 || b.PathBytes > 1024 || b.Depth < 1 || b.Depth > 128 {
		return c, ErrInvalid
	}
	if len(c.Roots) == 0 || len(c.Roots) > 128 || len(c.Includes) > 128 || len(c.Excludes) > 128 || len(c.Prunes) > 128 {
		return c, ErrInvalid
	}
	for _, xs := range [][]string{c.Roots, c.Prunes} {
		for _, p := range xs {
			if !local(p, b.PathBytes) {
				return c, ErrInvalid
			}
		}
	}
	for _, p := range c.Roots {
		if pathDepth(p) > b.Depth {
			return c, ErrInvalid
		}
	}
	for _, xs := range [][]string{c.Includes, c.Excludes} {
		for _, p := range xs {
			if p != strings.TrimSpace(p) || len(p) > 1024 || !utf8.ValidString(p) || strings.ContainsRune(p, 0) || discoveryfilter.ValidatePattern(p) != nil {
				return c, ErrInvalid
			}
		}
	}
	if c.JobDeadline != nil {
		deadline := c.JobDeadline.UTC()
		if deadline.IsZero() {
			return c, ErrInvalid
		}
		c.JobDeadline = &deadline // never retain a caller-mutable deadline pointer
	}
	c.Roots = canonical(c.Roots)
	c.Prunes = canonical(c.Prunes)
	c.Includes = canonical(c.Includes)
	c.Excludes = canonical(c.Excludes)
	roots := []string{}
	for _, p := range c.Roots {
		overlap := false
		for _, r := range roots {
			if beneath(r, p) {
				overlap = true
				break
			}
		}
		if !overlap {
			roots = append(roots, p)
		}
	}
	c.Roots = roots
	raw, e := json.Marshal(c)
	if e != nil || len(raw) > int(HeadBytes/2) {
		return c, ErrInvalid
	}
	return c, nil
}
func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func strict(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return ErrIntegrity
	}
	canonical, e := json.Marshal(out)
	if e != nil || !bytes.Equal(raw, canonical) {
		return ErrIntegrity
	}
	return nil
}

type sealed struct {
	Digest  string          `json:"digest"`
	Payload json.RawMessage `json:"payload"`
}

func seal(v any) ([]byte, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return json.Marshal(sealed{digest(raw), raw})
}
func unseal(raw []byte, v any) error {
	var s sealed
	if strict(raw, &s) != nil || digest(s.Payload) != s.Digest {
		return ErrIntegrity
	}
	return strict(s.Payload, v)
}
