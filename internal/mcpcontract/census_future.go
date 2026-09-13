package mcpcontract

import (
	"embed"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	FutureCensusTool          = "lsp_trace_v1_census"
	FutureCensusInputID       = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-census.v1.schema.json"
	FutureCensusResultID      = "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.census-result.v1.schema.json"
	FutureCensusSuccessID     = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-result.v1.schema.json"
	FutureCensusDomainErrorID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-census-domain-error.v1.schema.json"
)

//go:embed testdata/schemas/input-census.v1.schema.json testdata/schemas/lsp-trace.census-result.v1.schema.json testdata/schemas/envelope-census-result.v1.schema.json testdata/schemas/envelope-census-domain-error.v1.schema.json
var futureCensusFiles embed.FS

var (
	errFutureCensusShape     = errors.New("census semantic-v1: invalid shape")
	errFutureCensusValue     = errors.New("census semantic-v1: invalid value")
	errFutureCensusDuplicate = errors.New("census semantic-v1: duplicate normalized value")
	futureCensusPattern      = regexp.MustCompile(`^[^\\\x00]+$`)
)

func FutureCensusSchemaJSON(schemaID string) ([]byte, error) {
	paths := map[string]string{
		FutureCensusInputID:       "testdata/schemas/input-census.v1.schema.json",
		FutureCensusResultID:      "testdata/schemas/lsp-trace.census-result.v1.schema.json",
		FutureCensusSuccessID:     "testdata/schemas/envelope-census-result.v1.schema.json",
		FutureCensusDomainErrorID: "testdata/schemas/envelope-census-domain-error.v1.schema.json",
	}
	name, ok := paths[schemaID]
	if !ok {
		return nil, errors.New("unknown future census schema")
	}
	raw, err := futureCensusFiles.ReadFile(name)
	return append([]byte(nil), raw...), err
}

// WithFutureCensus returns a separately testable FUTURE/UNREGISTERED contract view.
// Current manifest, registry, dispatcher, profiles and capabilities do not call it.
func WithFutureCensus(base *Manifest) *Manifest {
	out := *base
	out.Schemas = append([]SchemaRegistration{}, base.Schemas...)
	out.Tools = append([]ToolContract{}, base.Tools...)
	out.Schemas = append(out.Schemas,
		SchemaRegistration{ID: FutureCensusInputID, Family: "https://jaresty.github.io/lsp-trace/mcp/input/census/v1", Layer: "input", Path: "schemas/input-census.v1.schema.json"},
		SchemaRegistration{ID: FutureCensusResultID, Family: "https://jaresty.github.io/lsp-trace/census-result/v1", Layer: "artifact", Path: "schemas/lsp-trace.census-result.v1.schema.json"},
		SchemaRegistration{ID: FutureCensusSuccessID, Family: "https://jaresty.github.io/lsp-trace/mcp/envelope/census-result/v1", Layer: "envelope", Path: "schemas/envelope-census-result.v1.schema.json"},
		SchemaRegistration{ID: FutureCensusDomainErrorID, Family: "https://jaresty.github.io/lsp-trace/mcp/envelope/census-domain-error/v1", Layer: "envelope", Path: "schemas/envelope-census-domain-error.v1.schema.json"},
	)
	out.Tools = append(out.Tools, ToolContract{Name: FutureCensusTool, Aliases: []string{"lsp_trace_census"}, InputSchemaID: FutureCensusInputID, EnvelopeSchemaIDs: []string{FutureCensusSuccessID, FutureCensusDomainErrorID}, ArtifactSchemaIDs: []string{FutureCensusResultID}, Advertised: false, Availability: "NOT_IMPLEMENTED"})
	return &out
}

// DecodeFutureCensusRequestV1 applies strict JSON precedence before semantics.
func DecodeFutureCensusRequestV1(raw []byte) (map[string]any, error) {
	if err := rejectFutureCensusDuplicateMembers(raw); err != nil {
		return nil, err
	}
	var value map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&value); err != nil || value == nil {
		return nil, errFutureCensusShape
	}
	if dec.Decode(new(any)) == nil {
		return nil, errFutureCensusShape
	}
	if err := ValidateFutureCensusRequestV1(value); err != nil {
		return nil, err
	}
	return value, nil
}

func rejectFutureCensusDuplicateMembers(raw []byte) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return errFutureCensusShape
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return errFutureCensusShape
				}
				key, ok := keyToken.(string)
				if !ok {
					return errFutureCensusShape
				}
				if seen[key] {
					return errFutureCensusDuplicate
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			if end, err := dec.Token(); err != nil || end != json.Delim('}') {
				return errFutureCensusShape
			}
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			if end, err := dec.Token(); err != nil || end != json.Delim(']') {
				return errFutureCensusShape
			}
		default:
			return errFutureCensusShape
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := dec.Token(); err == nil {
		return errFutureCensusShape
	}
	return nil
}

func ValidateFutureCensusRequestV1(v map[string]any) error {
	if v == nil || !allowed(v, "session_id", "generation", "sources", "includes", "excludes", "down_depth", "up_depth", "max_nodes", "timeout_ms", "request_timeout_ms") || !required(v, "session_id", "generation", "sources") {
		return errFutureCensusShape
	}
	if _, err := stringField(v, "session_id", 1, 256, nil); err != nil {
		return err
	}
	if _, err := censusUint(v, "generation", 1, futureMaxCount, true, 0); err != nil {
		return err
	}
	if err := validateCensusStrings(v, "sources", true, true); err != nil {
		return err
	}
	for _, key := range []string{"includes", "excludes"} {
		if err := validateCensusStrings(v, key, false, false); err != nil {
			return err
		}
	}
	for _, b := range []struct {
		k             string
		min, max, def uint64
	}{{"down_depth", 0, 64, 1}, {"up_depth", 0, 64, 0}, {"max_nodes", 1, 10000, 10000}, {"timeout_ms", 1, 60000, 60000}, {"request_timeout_ms", 1, 60000, 60000}} {
		if _, err := censusUint(v, b.k, b.min, b.max, false, b.def); err != nil {
			return err
		}
	}
	t, _ := censusUint(v, "timeout_ms", 1, 60000, false, 60000)
	rt, _ := censusUint(v, "request_timeout_ms", 1, 60000, false, t)
	if rt > t {
		return errFutureCensusValue
	}
	return nil
}

func validateCensusStrings(v map[string]any, key string, requiredField, roots bool) error {
	raw, ok := v[key]
	if !ok {
		if requiredField {
			return errFutureCensusShape
		}
		return nil
	}
	items, ok := raw.([]any)
	if !ok || items == nil || requiredField && len(items) == 0 {
		return errFutureCensusShape
	}
	seen := map[string]bool{}
	for _, item := range items {
		s, ok := item.(string)
		if !ok || s == "" || !futureCensusPattern.MatchString(s) {
			return errFutureCensusValue
		}
		n := path.Clean(s)
		if roots && (filepath.IsAbs(s) || !filepath.IsLocal(s) || n != s || strings.HasPrefix(s, "../")) {
			return errFutureCensusValue
		}
		if !roots {
			if n != s || strings.HasPrefix(s, "../") {
				return errFutureCensusValue
			}
			if _, err := path.Match(s, "probe/path.go"); err != nil {
				return errFutureCensusValue
			}
		}
		if seen[n] {
			return errFutureCensusDuplicate
		}
		seen[n] = true
	}
	return nil
}

func censusUint(v map[string]any, key string, min, max uint64, requiredField bool, def uint64) (uint64, error) {
	raw, ok := v[key]
	if !ok {
		if requiredField {
			return 0, errFutureCensusShape
		}
		return def, nil
	}
	if n, ok := raw.(json.Number); ok {
		i, err := n.Int64()
		if err != nil || i < 0 || uint64(i) < min || uint64(i) > max {
			return 0, errFutureCensusValue
		}
		return uint64(i), nil
	}
	return uintField(v, key, min, max)
}

// ValidateFutureCensusResultV1 validates the closed CLI-parity projection only;
// it does not infer workspace containment, source completeness or runtime authority.
func ValidateFutureCensusResultV1(v map[string]any) error {
	keys := []string{"schema_version", "status", "census_id", "capture_set_id", "session_id", "generation", "target_count", "batch_count", "file_accounting", "symbol_accounting", "authority", "source_graph_complete", "native_aggregate_custody", "cross_capture_calls", "leiden_admissible", "publication"}
	if v == nil || !closed(v, keys...) {
		return errFutureCensusShape
	}
	for k, w := range map[string]string{"schema_version": "lsp-trace.census-result.v1", "status": "SUCCEEDED", "source_graph_complete": "UNKNOWN"} {
		if v[k] != w {
			return errFutureCensusValue
		}
	}
	for _, k := range []string{"census_id", "capture_set_id", "session_id"} {
		if _, e := stringField(v, k, 1, 256, nil); e != nil {
			return e
		}
	}
	authority, err := censusUint(v, "authority", 0, 0, true, 0)
	if err != nil || authority != 0 || v["native_aggregate_custody"] != false || v["leiden_admissible"] != false {
		return errFutureCensusValue
	}
	calls, ok := v["cross_capture_calls"].([]any)
	if !ok || calls == nil || len(calls) != 0 {
		return errFutureCensusValue
	}
	gen, e := censusUint(v, "generation", 1, futureMaxCount, true, 0)
	_ = gen
	if e != nil {
		return e
	}
	targets, e := censusUint(v, "target_count", 1, 10000, true, 0)
	if e != nil {
		return e
	}
	batches, e := censusUint(v, "batch_count", 1, 159, true, 0)
	if e != nil || batches != (targets+62)/63 {
		return errFutureCensusValue
	}
	if e := validateCensusAccounting(v, "file_accounting", []string{"processed", "excluded", "forbidden", "unreadable", "unsupported", "document_symbol_failed", "omitted", "incomplete"}); e != nil {
		return e
	}
	if e := validateCensusAccounting(v, "symbol_accounting", []string{"prepared", "unsupported", "preparation_failed", "prepare_missing", "non_callable", "omitted", "incomplete"}); e != nil {
		return e
	}
	p, e := objectField(v, "publication")
	if e != nil || !closed(p, "selector", "digest", "byte_length", "verification_status", "directory_sync_status", "close_status") {
		return errFutureCensusShape
	}
	sel, e := stringField(p, "selector", 1, 1024, nil)
	if e != nil || !safeFutureSelector(sel) {
		return errFutureCensusValue
	}
	if _, e = stringField(p, "digest", 71, 71, regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)); e != nil {
		return e
	}
	if _, e = censusUint(p, "byte_length", 1, futureMaxCount, true, 0); e != nil {
		return e
	}
	if !oneOf(p["verification_status"], "VERIFIED", "COMMITTED_VERIFICATION_FAILED") || !oneOf(p["directory_sync_status"], "COMPLETE", "FAILED", "NOT_ATTEMPTED_POST_COMMIT") || !oneOf(p["close_status"], "COMPLETE", "FAILED") {
		return errFutureCensusValue
	}
	return nil
}
func validateCensusAccounting(v map[string]any, key string, parts []string) error {
	a, e := objectField(v, key)
	if e != nil {
		return e
	}
	keys := append([]string{"denominator"}, parts...)
	if !closed(a, keys...) {
		return errFutureCensusShape
	}
	d, e := censusUint(a, "denominator", 0, 10000, true, 0)
	if e != nil {
		return e
	}
	var sum uint64
	for _, k := range parts {
		n, e := censusUint(a, k, 0, 10000, true, 0)
		if e != nil {
			return e
		}
		sum += n
	}
	if sum != d {
		return errFutureCensusValue
	}
	return nil
}
func safeFutureSelector(s string) bool {
	return s != "" && !filepath.IsAbs(s) && filepath.IsLocal(s) && path.Clean(s) == s && !strings.Contains(s, `\`)
}
func oneOf(v any, w ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, x := range w {
		if s == x {
			return true
		}
	}
	return false
}
