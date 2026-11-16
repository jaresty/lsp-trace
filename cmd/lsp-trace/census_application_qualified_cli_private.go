//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/inventoryscope"
)

const censusQualifiedCLIFamilyPrivate = "census-qualified-cli-preparation-envelope/v1"
const censusQualifiedCLIInputLimitPrivate = 32 << 20

var errCensusQualifiedCLIPrivate = errors.New("qualified preparation refused; no execution admission")

// Complete is explicitly a caller claim about the supplied universe, never a
// claim that discovery was exhaustive. The digest binds ALL supplied candidates
// through the accepted scope denominator formula, not a pruned discovery result.
type censusQualifiedUniversePrivate struct {
	Complete                  bool                         `json:"complete"`
	Claim                     string                       `json:"claim"`
	ListMethod                string                       `json:"list_method"`
	Provenance                inventoryscope.ArtifactRefV1 `json:"provenance"`
	OriginalDenominatorCount  int                          `json:"original_denominator_count"`
	OriginalDenominatorDigest string                       `json:"original_denominator_digest"`
}
type censusQualifiedCLIEnvelopePrivate struct {
	Family          string                                   `json:"family"`
	Universe        censusQualifiedUniversePrivate           `json:"universe"`
	Originals       []censusacquisition.SourceFile           `json:"originals"`
	Qualification   inventoryscope.InputV1                   `json:"qualification"`
	Selection       inventoryscope.SelectionV1               `json:"selection"`
	Workspace       string                                   `json:"workspace"`
	PartitionSize   int                                      `json:"partition_size"`
	Preparation     censusApplicationQualifiedOptionsPrivate `json:"preparation"`
	TimeoutMS       int                                      `json:"timeout_ms"`
	OutputDirectory string                                   `json:"output_directory"`
	OutputName      string                                   `json:"output_name"`
}

// This preflight walks tokens against the actual typed schema before any typed
// collections or RawMessages are allocated. All fields are required except
// explicitly omitempty fields; null, duplicate and alternate-case keys fail.
func decodeCensusQualifiedCLIPrivate(raw []byte, limits inventoryscope.LimitsV1) (censusQualifiedCLIEnvelopePrivate, error) {
	var out censusQualifiedCLIEnvelopePrivate
	l, e := limits.EffectiveV1()
	if e != nil {
		return out, errCensusQualifiedCLIPrivate
	}
	if len(raw) == 0 || int64(len(raw)) > l.MaxInputBytes || raw[0] != '{' || raw[len(raw)-1] != '}' || !utf8.Valid(raw) {
		return out, errCensusQualifiedCLIPrivate
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	work, chars, refs := l.MaxWork, l.MaxUTF8Bytes, l.MaxReferences
	var visit func(reflect.Type, string, int) error
	visit = func(typ reflect.Type, field string, depth int) error {
		if depth > 32 || work < 1 {
			return errCensusQualifiedCLIPrivate
		}
		work--
		tok, e := d.Token()
		if e != nil || tok == nil {
			return errCensusQualifiedCLIPrivate
		}
		switch typ.Kind() {
		case reflect.Struct:
			if tok != json.Delim('{') {
				return errCensusQualifiedCLIPrivate
			}
			fields := map[string]reflect.StructField{}
			required := map[string]bool{}
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")
				name := tag[0]
				if name == "" {
					name = f.Name
				}
				if name == "-" {
					continue
				}
				fields[name] = f
				required[name] = !strings.Contains(f.Tag.Get("json"), ",omitempty")
			}
			seen := map[string]bool{}
			for d.More() {
				if work < 1 {
					return errCensusQualifiedCLIPrivate
				}
				work--
				k, e := d.Token()
				key, ok := k.(string)
				if e != nil || !ok || len(key) > 128 || len(key) > chars || seen[key] {
					return errCensusQualifiedCLIPrivate
				}
				f, ok := fields[key]
				if !ok {
					return errCensusQualifiedCLIPrivate
				}
				chars -= len(key)
				seen[key] = true
				if e = visit(f.Type, key, depth+1); e != nil {
					return e
				}
			}
			for key, required := range required {
				if required && !seen[key] {
					return errCensusQualifiedCLIPrivate
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return errCensusQualifiedCLIPrivate
			}
		case reflect.Slice:
			if tok != json.Delim('[') {
				return errCensusQualifiedCLIPrivate
			}
			cap := l.MaxCandidates
			switch field {
			case "observations":
				cap = l.MaxObservations
			case "resolutions":
				cap = l.MaxResolutions
			case "observation_refs":
				cap = l.MaxReferences
			}
			n := 0
			for d.More() {
				if n >= cap {
					return errCensusQualifiedCLIPrivate
				}
				n++
				if field == "observation_refs" {
					if refs < 1 {
						return errCensusQualifiedCLIPrivate
					}
					refs--
				}
				if e = visit(typ.Elem(), field, depth+1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return errCensusQualifiedCLIPrivate
			}
		case reflect.String:
			s, ok := tok.(string)
			if !ok || len(s) > chars {
				return errCensusQualifiedCLIPrivate
			}
			chars -= len(s)
		case reflect.Bool:
			if _, ok := tok.(bool); !ok {
				return errCensusQualifiedCLIPrivate
			}
		case reflect.Int, reflect.Int64, reflect.Uint64, reflect.Uint:
			num, ok := tok.(json.Number)
			if !ok {
				return errCensusQualifiedCLIPrivate
			}
			if _, e := num.Int64(); e != nil {
				return errCensusQualifiedCLIPrivate
			}
		default:
			return errCensusQualifiedCLIPrivate
		}
		return nil
	}
	if e = visit(reflect.TypeOf(out), "", 0); e != nil {
		return out, e
	}
	if _, e = d.Token(); e != io.EOF {
		return out, errCensusQualifiedCLIPrivate
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&out); e != nil {
		return censusQualifiedCLIEnvelopePrivate{}, errCensusQualifiedCLIPrivate
	}
	return out, nil
}

func validateCensusQualifiedCLIPrivate(in censusQualifiedCLIEnvelopePrivate) (censussourceplan.QualifiedInputV1, error) {
	var zero censussourceplan.QualifiedInputV1
	u := in.Universe
	o := in.Preparation
	p := o.Policy
	if in.Family != censusQualifiedCLIFamilyPrivate || !u.Complete || u.Claim != "CALLER_DECLARED_COMPLETE_NOT_FILESYSTEM_VERIFIED" || u.ListMethod == "" || len(u.ListMethod) > 4096 || u.Provenance.ArtifactID == "" || len(u.Provenance.ArtifactID) > 256 || !qualifiedCLIDigestPrivate(u.Provenance.Digest) || in.Selection.State != inventoryscope.CallerSelectedV1 || in.TimeoutMS < 1 || in.TimeoutMS > 30000 || !filepath.IsAbs(in.Workspace) || filepath.Clean(in.Workspace) != in.Workspace || !filepath.IsAbs(in.OutputDirectory) || filepath.Clean(in.OutputDirectory) != in.OutputDirectory || !qualifiedCLILeafPrivate(in.OutputName) || o.MaxArtifactBytes < 1 || o.MaxArtifactBytes > 32<<20 || o.MaxMetadataBytes < 1 || o.MaxMetadataBytes > 48000 || !qualifiedCLIHexPrivate(o.RevisionAssertion, 40) {
		return zero, errCensusQualifiedCLIPrivate
	}
	// Preserve existing private policy ceilings, including no-work preparations
	// that do not call freeze. These are validation limits, never native grants.
	bounds := [][2]int{{p.MaxNominationFiles, 8}, {p.MaxContextFiles, 8}, {p.MaxMembers, 16}, {p.FileBytes, 2 << 20}, {p.TotalBytes, 8 << 20}, {p.MaxPathDepth, 32}, {p.Discovery.MaxFiles, 8}, {p.Discovery.MaxSymbols, 256}, {p.Discovery.MaxDepth, 64}, {p.Discovery.MaxStringBytes, 1 << 20}, {p.MaxTargets, 63}, {p.Planning.MaxBatchTargets, 63}, {p.Planning.DownDepth, 8}, {p.Planning.UpDepth, 8}, {p.MaxNodes, 256}, {p.MaxRequests, 256}, {p.MaxEvidenceBytes, 4 << 20}, {p.MaxPathWork, 100000}, {p.TimeoutMS, 30000}, {p.RequestTimeoutMS, 5000}, {p.MaxResponseBytes, 2 << 20}, {p.MaxMessages, 64}}
	for _, b := range bounds {
		if b[0] < 1 || b[0] > b[1] {
			return zero, errCensusQualifiedCLIPrivate
		}
	}
	if p.Discovery.MaxFiles != p.MaxNominationFiles || p.Planning.MaxBatchTargets > p.MaxTargets || p.RequestTimeoutMS > p.TimeoutMS || p.FileBytes > p.TotalBytes {
		return zero, errCensusQualifiedCLIPrivate
	}
	m, e := inventoryscope.QualifyV1(in.Qualification, inventoryscope.LimitsV1{})
	if e != nil {
		return zero, errCensusQualifiedCLIPrivate
	}
	if u.OriginalDenominatorCount != m.OriginalDenominatorCount || u.OriginalDenominatorDigest != m.OriginalDenominatorDigest {
		return zero, errCensusQualifiedCLIPrivate
	}
	if e = inventoryscope.ValidateSelectionV1(in.Selection, m); e != nil {
		return zero, errCensusQualifiedCLIPrivate
	}
	q, e := censussourceplan.BuildQualifiedV1(in.Originals, in.Qualification, m, in.Selection, in.Workspace, in.PartitionSize, inventoryscope.LimitsV1{})
	if e != nil {
		return zero, errCensusQualifiedCLIPrivate
	}
	return q, nil
}
func qualifiedCLIHexPrivate(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func qualifiedCLIDigestPrivate(s string) bool {
	return strings.HasPrefix(s, "sha256:") && qualifiedCLIHexPrivate(strings.TrimPrefix(s, "sha256:"), 64)
}
func qualifiedCLILeafPrivate(s string) bool {
	if len(s) < 1 || len(s) > 128 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// Identity includes the COMPLETE caller envelope, canonical qualified evidence
// and preparation options. Output destination and timeout are bound too; this
// identity is a nonoperative provenance claim, never an authorization fingerprint.
func censusQualifiedCLIIdentityPrivate(in censusQualifiedCLIEnvelopePrivate, q censussourceplan.QualifiedInputV1) ([]byte, string, error) {
	in.Qualification = q.Input
	in.Originals = q.Originals
	raw, e := json.Marshal(struct {
		Family          string                            `json:"family"`
		Envelope        censusQualifiedCLIEnvelopePrivate `json:"envelope"`
		QualifiedDigest string                            `json:"qualified_digest"`
	}{censusQualifiedCLIFamilyPrivate, in, q.QualifiedDigest})
	if e != nil {
		return nil, "", errCensusQualifiedCLIPrivate
	}
	identity := censusApplicationQualifiedArtifactDigestPrivate(append([]byte("lsp-trace:census-qualified-cli-preparation-envelope/v1\n"), raw...))
	return raw, identity, nil
}

type censusQualifiedCLIRefPrivate struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
	Bytes  int    `json:"bytes"`
}
type censusQualifiedCLIResultPrivate struct {
	Family               string                       `json:"family"`
	Prepared             bool                         `json:"prepared"`
	ExecutionDisposition string                       `json:"execution_disposition"`
	Authority            int                          `json:"authority"`
	Accepted             bool                         `json:"accepted"`
	FeatureIdentity      string                       `json:"feature_identity"`
	AbsenceInference     string                       `json:"absence_inference"`
	Completeness         string                       `json:"completeness"`
	Identity             string                       `json:"preparation_identity"`
	OriginalCount        int                          `json:"original_count"`
	Included             int                          `json:"included"`
	Excluded             int                          `json:"excluded"`
	Unresolved           int                          `json:"unresolved"`
	Retained             int                          `json:"artifacts_retained"`
	Returned             int                          `json:"artifacts_returned"`
	Omitted              int                          `json:"artifacts_omitted"`
	ArtifactBytes        int                          `json:"artifact_bytes"`
	OutputName           string                       `json:"output_name"`
	Index                censusQualifiedCLIRefPrivate `json:"index"`
	Envelope             censusQualifiedCLIRefPrivate `json:"envelope"`
	Preparation          censusQualifiedCLIRefPrivate `json:"preparation"`
}

func censusQualifiedCLIRefForPrivate(name string, b []byte) censusQualifiedCLIRefPrivate {
	return censusQualifiedCLIRefPrivate{name, censusApplicationQualifiedArtifactDigestPrivate(b), len(b)}
}

// All errors crossing the CLI boundary are content-free. The one absolute
// deadline starts before intake and is only shortened by the caller envelope.
func runCensusApplicationQualifiedCLIPrivate(options censusCLIOptions, stdout, stderr io.Writer) int {
	started := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), started.Add(30*time.Second))
	defer cancel()
	raw, e := readCensusQualifiedCLIPrivate(ctx, options.PrivateInventoryQualifiedPrepareConfigPath, censusQualifiedCLIInputLimitPrivate)
	if e != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, errCensusQualifiedCLIPrivate)
	}
	in, e := decodeCensusQualifiedCLIPrivate(raw, inventoryscope.LimitsV1{})
	if e != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, errCensusQualifiedCLIPrivate)
	}
	q, e := validateCensusQualifiedCLIPrivate(in)
	if e != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, errCensusQualifiedCLIPrivate)
	}
	ctx, shorten := context.WithDeadline(ctx, started.Add(time.Duration(in.TimeoutMS)*time.Millisecond))
	defer shorten()
	p, metadata, e := buildCensusApplicationQualifiedFullPlanPrivate(ctx, q, in.Preparation, inventoryscope.LimitsV1{})
	if e != nil {
		return writeCensusFailure(stderr, censusStageConfig, true, errCensusQualifiedCLIPrivate)
	}
	result, e := publishCensusQualifiedCLIPrivate(ctx, in, q, p, metadata)
	if e != nil {
		// A failed/uncertain publication is never a Prepared result. Cleanup is
		// best effort, not a same-user mutation lease or a claim of durability.
		diagnostic, _ := json.Marshal(struct {
			Family      string `json:"family"`
			Prepared    bool   `json:"prepared"`
			Authority   int    `json:"authority"`
			Accepted    bool   `json:"accepted"`
			Disposition string `json:"execution_disposition"`
			Cleanup     string `json:"cleanup"`
		}{Family: censusQualifiedCLIFamilyPrivate, Disposition: "PUBLICATION_UNCONFIRMED", Cleanup: "BEST_EFFORT_UNCONFIRMED"})
		_, _ = stderr.Write(append(diagnostic, '\n'))
		return 1
	}
	if _, e = stdout.Write(append(result, '\n')); e != nil {
		return 1
	}
	return 0
}

func readCensusQualifiedCLIPrivate(ctx context.Context, path string, limit int) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errCensusQualifiedCLIPrivate
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(path))
	if e != nil || parent != filepath.Dir(path) || ctx == nil || ctx.Err() != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	s, e := openCensusApplicationStorePrivate(ctx, parent, limit+1)
	if e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	// Observe the leaf itself: confined reads do not establish leaf-link refusal.
	// These namespace checks are not an atomic open or opened-file identity proof.
	leaf := filepath.Base(path)
	before, e := s.root.Lstat(leaf)
	if e == nil && (!before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0) {
		e = errCensusQualifiedCLIPrivate
	}
	var b []byte
	if e == nil {
		b, e = s.read(leaf, limit)
	}
	if e == nil {
		current, statErr := s.root.Lstat(leaf)
		if statErr != nil || !current.Mode().IsRegular() || current.Mode().Perm()&0077 != 0 || !os.SameFile(before, current) {
			e = errCensusQualifiedCLIPrivate
		}
	}
	if e == nil {
		e = s.check()
	}
	ce := s.close()
	if e != nil || ce != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	return b, nil
}

func syncCensusQualifiedDirectoryPrivate(s *censusApplicationStorePrivate) error {
	if e := s.check(); e != nil {
		return e
	}
	f, e := s.root.Open(".")
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close(), s.check())
}

// Qualified publication requires an exact0700 child at each bounded observation.
// Check both the held directory and its namespace, not a fresh path-only open.
// This detects observed changes; it is not a same-user mutation lease.
func checkCensusQualifiedChildDirectoryPrivate(s *censusApplicationStorePrivate) error {
	if s.check() != nil {
		return errCensusQualifiedCLIPrivate
	}
	opened, e := s.root.Stat(".")
	if e != nil || !opened.IsDir() || opened.Mode().Perm() != 0700 {
		return errCensusQualifiedCLIPrivate
	}
	current, e := os.Lstat(s.path)
	if e != nil || !current.IsDir() || current.Mode().Perm() != 0700 || !os.SameFile(opened, s.identity) || !os.SameFile(current, s.identity) {
		return errCensusQualifiedCLIPrivate
	}
	return nil
}

func publishCensusQualifiedCLIPrivate(ctx context.Context, in censusQualifiedCLIEnvelopePrivate, q censussourceplan.QualifiedInputV1, p censusApplicationQualifiedFullPlanPrivate, metadata []byte) ([]byte, error) {
	return publishCensusQualifiedCLIWithWritePrivate(ctx, in, q, p, metadata, (*censusApplicationStorePrivate).write)
}

// The callback is an internal refusal-test seam. Production always uses the
// descriptor-relative exclusive writer; it cannot be selected by the envelope.
func publishCensusQualifiedCLIWithWritePrivate(ctx context.Context, in censusQualifiedCLIEnvelopePrivate, q censussourceplan.QualifiedInputV1, p censusApplicationQualifiedFullPlanPrivate, metadata []byte, write func(*censusApplicationStorePrivate, string, []byte, int) error) (result []byte, err error) {
	if ctx == nil || ctx.Err() != nil || write == nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	envelope, identity, e := censusQualifiedCLIIdentityPrivate(in, q)
	if e != nil {
		return nil, e
	}
	artifacts := make(map[string][]byte, len(p.Artifacts)+4)
	for name, b := range p.Artifacts {
		artifacts[name] = b
	}
	artifacts["preparation-envelope.v1.json"] = envelope
	artifacts["preparation-metadata.v1.json"] = metadata
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	refs := make([]censusQualifiedCLIRefPrivate, 0, len(names))
	total := 0
	for _, name := range names {
		if !qualifiedCLILeafPrivate(name) {
			return nil, errCensusQualifiedCLIPrivate
		}
		b := artifacts[name]
		if len(b) > in.Preparation.MaxArtifactBytes-total {
			return nil, errCensusQualifiedCLIPrivate
		}
		total += len(b)
		refs = append(refs, censusQualifiedCLIRefForPrivate(name, b))
	}
	index, e := json.Marshal(refs)
	if e != nil || len(index) > in.Preparation.MaxArtifactBytes-total {
		return nil, errCensusQualifiedCLIPrivate
	}
	total += len(index)
	artifacts["artifact-index.v1.json"] = index
	names = append(names, "artifact-index.v1.json")
	r := censusQualifiedCLIResultPrivate{Family: censusQualifiedCLIFamilyPrivate, Prepared: true, ExecutionDisposition: p.ExecutionDisposition, FeatureIdentity: "UNRESOLVED", AbsenceInference: "PROHIBITED", Completeness: in.Universe.Claim, Identity: identity, OriginalCount: len(q.Originals), Included: q.Scope.IncludedCount, Excluded: q.Scope.ExcludedCount, Unresolved: q.Scope.UnresolvedCount, Retained: len(artifacts), Returned: 3, Omitted: len(artifacts) - 3, ArtifactBytes: total, OutputName: in.OutputName, Index: censusQualifiedCLIRefForPrivate("artifact-index.v1.json", index), Envelope: censusQualifiedCLIRefForPrivate("preparation-envelope.v1.json", envelope), Preparation: censusQualifiedCLIRefForPrivate("preparation-metadata.v1.json", metadata)}
	result, e = json.Marshal(r)
	if e != nil || len(result)+1 > in.Preparation.MaxMetadataBytes || len(result)+1 > 48000 || len(result) > in.Preparation.MaxArtifactBytes-total {
		return nil, errCensusQualifiedCLIPrivate
	}
	// Metadata is retained too; counts describe referenced artifacts, not itself.
	total += len(result)
	canonicalDirectory, e := filepath.EvalSymlinks(in.OutputDirectory)
	if e != nil || canonicalDirectory != in.OutputDirectory {
		return nil, errCensusQualifiedCLIPrivate
	}
	parent, e := openCensusApplicationStorePrivate(ctx, in.OutputDirectory, in.Preparation.MaxArtifactBytes)
	if e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	defer func() {
		if e := parent.close(); e != nil {
			result = nil
			err = errCensusQualifiedCLIPrivate
		}
	}()
	if e = parent.root.Mkdir(in.OutputName, 0700); e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	path := filepath.Join(in.OutputDirectory, in.OutputName)
	child, e := openCensusApplicationStorePrivate(ctx, path, in.Preparation.MaxArtifactBytes)
	if e != nil {
		_ = parent.root.Remove(in.OutputName)
		return nil, errCensusQualifiedCLIPrivate
	}
	written := []string{}
	success := false
	defer func() {
		if !success {
			for _, name := range written {
				_ = child.root.Remove(name)
			}
		}
		ce := child.close()
		if !success {
			_ = parent.root.Remove(in.OutputName)
		}
		if ce != nil {
			result = nil
			err = errCensusQualifiedCLIPrivate
		}
	}()
	for _, name := range names {
		b := artifacts[name]
		if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
			return nil, e
		}
		written = append(written, name)
		if e = write(child, name, b, in.Preparation.MaxArtifactBytes); e != nil {
			return nil, errCensusQualifiedCLIPrivate
		}
		if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
			return nil, e
		}
		check, e := child.read(name, len(b))
		if e != nil || !bytes.Equal(b, check) || checkCensusQualifiedChildDirectoryPrivate(child) != nil {
			return nil, errCensusQualifiedCLIPrivate
		}
	}
	if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
		return nil, e
	}
	written = append(written, "result.v1.json")
	if e = write(child, "result.v1.json", result, 48000); e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
		return nil, e
	}
	check, e := child.read("result.v1.json", len(result))
	if e != nil || !bytes.Equal(check, result) || checkCensusQualifiedChildDirectoryPrivate(child) != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	if e = syncCensusQualifiedDirectoryPrivate(child); e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
		return nil, e
	}
	if e = syncCensusQualifiedDirectoryPrivate(parent); e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	if e = ctx.Err(); e != nil {
		return nil, errCensusQualifiedCLIPrivate
	}
	if e = checkCensusQualifiedChildDirectoryPrivate(child); e != nil {
		return nil, e
	}
	success = true
	return result, nil
}
