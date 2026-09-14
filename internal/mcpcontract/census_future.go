package mcpcontract

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"lsp-trace/internal/strictjson"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
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

// ValidateFutureCensusEnvelopeExclusive validates an unregistered census
// envelope against the closed future schema set without granting registration
// or dispatch authority.
func ValidateFutureCensusEnvelopeExclusive(data []byte) error {
	if err := strictjson.RejectDuplicates(data); err != nil {
		return errFutureCensusShape
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return errFutureCensusShape
	}
	named, _ := value["envelope_schema_id"].(string)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	ids := []string{FutureCensusResultID, FutureCensusSuccessID, FutureCensusDomainErrorID}
	for _, id := range ids {
		raw, err := FutureCensusSchemaJSON(id)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		if err := compiler.AddResource(id, doc); err != nil {
			return err
		}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return errFutureCensusShape
	}
	matches := make([]string, 0, 1)
	for _, id := range []string{FutureCensusSuccessID, FutureCensusDomainErrorID} {
		compiled, err := compiler.Compile(id)
		if err != nil {
			return err
		}
		if compiled.Validate(doc) == nil {
			matches = append(matches, id)
		}
	}
	if len(matches) != 1 || matches[0] != named {
		return errFutureCensusShape
	}
	return nil
}

// DecodeFutureCensusRequestV1 applies strict JSON precedence before semantics.
func DecodeFutureCensusRequestV1(raw []byte) (map[string]any, error) {
	// strictjson first validates one complete JSON value, so malformed input can
	// never be reclassified as a duplicate-member error.
	if err := strictjson.RejectDuplicates(raw); err != nil {
		if strings.Contains(err.Error(), "duplicate JSON member") {
			return nil, errFutureCensusDuplicate
		}
		return nil, errFutureCensusShape
	}
	var value map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil || value == nil {
		return nil, errFutureCensusShape
	}
	if err := ValidateFutureCensusRequestV1(value); err != nil {
		return nil, err
	}
	// These are bounded MCP transport defaults, not CLI invocation timeout
	// parity. Result semantic-field parity does not imply timeout parity.
	if _, ok := value["timeout_ms"]; !ok {
		value["timeout_ms"] = uint64(60000)
	}
	if _, ok := value["request_timeout_ms"]; !ok {
		value["request_timeout_ms"] = uint64(30000)
	}
	return value, nil
}

func ValidateFutureCensusRequestV1(v map[string]any) error {
	if v == nil || !allowed(v, "session_id", "generation", "sources", "includes", "excludes", "down_depth", "up_depth", "max_nodes", "timeout_ms", "request_timeout_ms") || !required(v, "session_id", "generation", "sources") {
		return errFutureCensusShape
	}
	if s, err := stringField(v, "session_id", 1, 1024, nil); err != nil || strings.TrimSpace(s) == "" {
		return errFutureCensusValue
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
	}{{"down_depth", 0, 64, 1}, {"up_depth", 0, 64, 0}, {"max_nodes", 1, 10000, 10000}, {"timeout_ms", 1, 60000, 60000}, {"request_timeout_ms", 1, 60000, 30000}} {
		if _, err := censusUint(v, b.k, b.min, b.max, false, b.def); err != nil {
			return err
		}
	}
	t, _ := censusUint(v, "timeout_ms", 1, 60000, false, 60000)
	rt, _ := censusUint(v, "request_timeout_ms", 1, 60000, false, 30000)
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
	if !ok || items == nil || len(items) > 10000 || requiredField && len(items) == 0 {
		return errFutureCensusShape
	}
	seen := map[string]bool{}
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return errFutureCensusValue
		}
		if _, err := stringField(map[string]any{"value": s}, "value", 1, 1024, futureCensusPattern); err != nil {
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
		s, e := stringField(v, k, 1, 1024, nil)
		if e != nil || strings.TrimSpace(s) == "" {
			return errFutureCensusValue
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
