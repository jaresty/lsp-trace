// Package sourceadmissionv1 defines the private immutable source binding used by ADR0007 experiments.
package sourceadmissionv1

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const Schema = "lsp-trace.adr0007.source-admission.private.v1"

type Outcome string

const (
	Complete        Outcome = "COMPLETE"
	InvalidRequest  Outcome = "INVALID_REQUEST"
	InvalidSource   Outcome = "INVALID_SOURCE"
	DuplicateSource Outcome = "DUPLICATE_SOURCE"
	ResourceLimit   Outcome = "RESOURCE_LIMIT"
)

type Limits struct{ MaxSources, MaxSourceBytes, MaxTotalBytes int }
type SelectedSource struct {
	Path, Revision, FileDigest, ObjectDigest string
	Bytes                                    []byte
}
type Binding struct {
	Schema, AdmissionDigest string
	Sources                 []SelectedSource
}
type Result struct {
	Outcome Outcome
	Binding *Binding
	Detail  string
}

func Admit(in []SelectedSource, lim Limits) Result {
	if lim.MaxSources < 1 || lim.MaxSourceBytes < 1 || lim.MaxTotalBytes < 1 || len(in) == 0 {
		return Result{Outcome: InvalidRequest, Detail: "invalid limits or empty source list"}
	}
	if len(in) > lim.MaxSources {
		return Result{Outcome: ResourceLimit, Detail: "source count"}
	}
	out := make([]SelectedSource, len(in))
	seen := map[string]bool{}
	total := 0
	for i, s := range in {
		if !canonicalPath(s.Path) || s.Revision == "" || !utf8.Valid(s.Bytes) || len(s.Bytes) == 0 || len(s.Bytes) > lim.MaxSourceBytes {
			return Result{Outcome: InvalidSource, Detail: "source binding"}
		}
		total += len(s.Bytes)
		if total > lim.MaxTotalBytes {
			return Result{Outcome: ResourceLimit, Detail: "source bytes"}
		}
		if seen[s.Path] {
			return Result{Outcome: DuplicateSource, Detail: s.Path}
		}
		seen[s.Path] = true
		fd := digest(s.Bytes)
		if s.FileDigest != "" && s.FileDigest != fd {
			return Result{Outcome: InvalidSource, Detail: "file digest"}
		}
		if s.ObjectDigest != "" && s.ObjectDigest != fd {
			return Result{Outcome: InvalidSource, Detail: "object digest"}
		}
		s.FileDigest = fd
		s.ObjectDigest = fd
		s.Bytes = append([]byte(nil), s.Bytes...)
		out[i] = s
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	h := sha256.New()
	h.Write([]byte(Schema))
	for _, s := range out {
		h.Write([]byte{0})
		h.Write([]byte(s.Path))
		h.Write([]byte{0})
		h.Write([]byte(s.Revision))
		h.Write([]byte{0})
		h.Write([]byte(s.FileDigest))
	}
	return Result{Outcome: Complete, Binding: &Binding{Schema: Schema, AdmissionDigest: "sha256:" + hex.EncodeToString(h.Sum(nil)), Sources: out}}
}
func Clone(b *Binding) *Binding {
	if b == nil {
		return nil
	}
	o := &Binding{Schema: b.Schema, AdmissionDigest: b.AdmissionDigest, Sources: make([]SelectedSource, len(b.Sources))}
	copy(o.Sources, b.Sources)
	for i := range o.Sources {
		o.Sources[i].Bytes = append([]byte(nil), o.Sources[i].Bytes...)
	}
	return o
}
func canonicalPath(p string) bool {
	return p != "" && utf8.ValidString(p) && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && p != "." && p != ".." && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/./") && !strings.Contains(p, "/../") && path.Clean(p) == p
}
func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

var ErrUnavailable = errors.New("source admission unavailable")
