package adr0011lifecycle

// Private V2 record codec for a deliberately restricted JCS-equivalent subset:
// safe printable ASCII strings (no quotes, backslash, <, > or &), integral JSON
// numbers within the exact IEEE-754 range, booleans, null, arrays and ASCII-key
// objects. Unsupported RFC 8785 input fails closed; this is NOT a full JCS codec.
// Shape validation is not independent provisioning, reference-graph replay, or
// authority to adopt a head, fence, closure, cleanup or public operation.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const v2SchemaID = "https://jaresty.github.io/lsp-trace/schemas/adr0011-lifecycle-v2.proposed.schema.json"

type v2Role struct {
	domain, slug string
	host         bool
}

var v2Roles = map[string]v2Role{
	"trustedHostRoot": {"ADR0011_TRUSTED_HOST_ROOT_V2", "trusted-host-root", true},
	"hostHeadPrepare": {"ADR0011_HOST_HEAD_PREPARE_V2", "host-head-prepare", true},
	"hostHeadCommit":  {"ADR0011_HOST_HEAD_COMMIT_V2", "host-head-commit", true},
	"headTransition":  {"ADR0011_HEAD_TRANSITION_V2", "head-transition", false},
	"fenceIntent":     {"ADR0011_FENCE_INTENT_V2", "fence-intent", false},
	"closureSnapshot": {"ADR0011_CLOSURE_SNAPSHOT_V2", "closure-snapshot", false},
}

type v2Codec struct {
	roles, refs                    map[string]*jsonschema.Schema
	selectedSchema, selectedPolicy []byte // copies of independently supplied pinned bytes
}

func newV2Codec(schemaBytes, policyBytes []byte) (*v2Codec, error) {
	if err := partialPins(schemaBytes, policyBytes); err != nil {
		return nil, err
	}
	// Schema/policy bytes come from the independently selected fixture, never a
	// record or root path. Their exact pinned digests are checked above.
	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		return nil, err
	}
	doc, ok := schemaDoc.(map[string]any)
	if !ok || doc["$id"] != v2SchemaID || doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		return nil, errors.New("selected V2 Draft 2020-12 schema identity mismatch")
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(v2SchemaID, doc); err != nil {
		return nil, err
	}
	result := &v2Codec{roles: make(map[string]*jsonschema.Schema), refs: make(map[string]*jsonschema.Schema), selectedSchema: bytes.Clone(schemaBytes), selectedPolicy: bytes.Clone(policyBytes)}
	for role := range v2Roles {
		s, err := c.Compile(v2SchemaID + "#/$defs/" + role)
		if err != nil {
			return nil, err
		}
		result.roles[role] = s
	}
	for _, name := range []string{"privateRef", "hostSlotRef"} {
		s, err := c.Compile(v2SchemaID + "#/$defs/" + name)
		if err != nil {
			return nil, err
		}
		result.refs[name] = s
	}
	return result, nil
}
func v2RoleURI(role string) string { return v2SchemaID + "#/$defs/" + role }
func v2Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func v2String(s string) error {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 || b > 0x7e || b == '"' || b == '\\' || b == '<' || b == '>' || b == '&' {
			return errors.New("unsupported string outside restricted JCS subset")
		}
	}
	return nil
}
func v2Subset(value any, depth *int, work *int) (any, error) {
	*work++
	if *work > 262144 || *depth > 64 {
		return nil, errors.New("V2 canonical subset work/depth exceeded")
	}
	switch x := value.(type) {
	case nil, bool:
		return x, nil
	case string:
		if err := v2String(x); err != nil {
			return nil, err
		}
		return x, nil
	case json.Number:
		n, err := strconv.ParseInt(string(x), 10, 64)
		if err != nil || n < -9007199254740991 || n > 9007199254740991 {
			return nil, errors.New("unsupported non-integer or imprecise number")
		}
		return n, nil
	case int:
		return v2Subset(json.Number(strconv.FormatInt(int64(x), 10)), depth, work)
	case int64:
		return v2Subset(json.Number(strconv.FormatInt(x, 10)), depth, work)
	case float64:
		return nil, errors.New("floating point values not admitted by restricted JCS codec")
	case []any:
		*depth++
		defer func() { *depth-- }()
		if len(x) > 4096 {
			return nil, errors.New("array limit exceeded")
		}
		out := make([]any, len(x))
		for i, v := range x {
			normalized, err := v2Subset(v, depth, work)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case map[string]any:
		*depth++
		defer func() { *depth-- }()
		if len(x) > 4096 {
			return nil, errors.New("object limit exceeded")
		}
		out := make(map[string]any, len(x))
		for k, v := range x {
			if err := v2String(k); err != nil {
				return nil, err
			}
			normalized, err := v2Subset(v, depth, work)
			if err != nil {
				return nil, err
			}
			out[k] = normalized
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported JSON value %T", value)
	}
}
func v2Canonical(value any) ([]byte, any, error) {
	depth, work := 0, 0
	normalized, err := v2Subset(value, &depth, &work)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > 1048576 {
		return nil, nil, errors.New("V2 record byte limit exceeded")
	}
	// Within this restricted subset, Go's sorted ASCII map keys, unescaped safe
	// ASCII values, integer decimal and JSON punctuation match RFC 8785 bytes.
	return raw, normalized, nil
}
func v2StrictParse(raw []byte) (any, error) {
	if len(raw) < 1 || len(raw) > 1048576 {
		return nil, errors.New("V2 record byte limit invalid")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	depth, work := 0, 0
	value, err := v2ParseValue(dec, &depth, &work)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON bytes")
	}
	return value, nil
}
func v2ParseValue(dec *json.Decoder, depth, work *int) (any, error) {
	*work++
	if *work > 262144 || *depth > 64 {
		return nil, errors.New("JSON parsing limit exceeded")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); ok {
		*depth++
		defer func() { *depth-- }()
		if delim == '{' {
			obj := map[string]any{}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("invalid object key")
				}
				if _, exists := obj[key]; exists {
					return nil, errors.New("duplicate decoded JSON key")
				}
				v, err := v2ParseValue(dec, depth, work)
				if err != nil {
					return nil, err
				}
				obj[key] = v
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return nil, errors.New("invalid object close")
			}
			return obj, nil
		}
		if delim == '[' {
			items := []any{}
			for dec.More() {
				if len(items) >= 4096 {
					return nil, errors.New("array limit exceeded")
				}
				v, err := v2ParseValue(dec, depth, work)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return nil, errors.New("invalid array close")
			}
			return items, nil
		}
		return nil, errors.New("unexpected JSON delimiter")
	}
	return token, nil
}
func (c *v2Codec) encode(role string, fields map[string]any) (string, []byte, error) {
	selected, ok := v2Roles[role]
	if !ok || c == nil || c.roles[role] == nil {
		return "", nil, errors.New("V2 role not selected")
	}
	without := make(map[string]any, len(fields))
	for k, v := range fields {
		if k == "record_id" {
			return "", nil, errors.New("record_id is codec-derived")
		}
		without[k] = v
	}
	if without["role"] != selected.domain || without["version"] != int64(2) && without["version"] != 2 {
		return "", nil, errors.New("V2 record role/domain/version mismatch")
	}
	base, _, err := v2Canonical(without)
	if err != nil {
		return "", nil, err
	}
	identity := append(append([]byte(selected.domain), 0), base...)
	without["record_id"] = v2Digest(identity)
	raw, normalized, err := v2Canonical(without)
	if err != nil {
		return "", nil, err
	}
	if err := c.roles[role].Validate(normalized); err != nil {
		return "", nil, err
	}
	selector := "adr0011-" + selected.slug + "-v2-" + strings.TrimPrefix(v2Digest(raw), "sha256:") + ".json"
	return selector, raw, nil
}
func (c *v2Codec) decode(role, selector string, raw []byte) (map[string]any, error) {
	selected, ok := v2Roles[role]
	if !ok || c == nil || c.roles[role] == nil {
		return nil, errors.New("V2 role not selected")
	}
	parsed, err := v2StrictParse(raw)
	if err != nil {
		return nil, err
	}
	canonical, normalized, err := v2Canonical(parsed)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, canonical) {
		return nil, errors.New("noncanonical V2 record bytes")
	}
	record, ok := normalized.(map[string]any)
	if !ok {
		return nil, errors.New("V2 record must be an object")
	}
	if record["role"] != selected.domain {
		return nil, errors.New("cross-role V2 domain substitution")
	}
	if err := c.roles[role].Validate(record); err != nil {
		return nil, err
	}
	id, ok := record["record_id"].(string)
	if !ok {
		return nil, errors.New("record ID missing")
	}
	without := make(map[string]any, len(record)-1)
	for k, v := range record {
		if k != "record_id" {
			without[k] = v
		}
	}
	base, _, err := v2Canonical(without)
	if err != nil {
		return nil, err
	}
	if id != v2Digest(append(append([]byte(selected.domain), 0), base...)) {
		return nil, errors.New("V2 record ID mismatch")
	}
	expected := "adr0011-" + selected.slug + "-v2-" + strings.TrimPrefix(v2Digest(raw), "sha256:") + ".json"
	if selector != expected {
		return nil, errors.New("V2 role-specific selector mismatch")
	}
	return record, nil
}
func v2MatchHeldRef(expected, actual map[string]any) error {
	if expected == nil || actual == nil || !reflect.DeepEqual(expected, actual) {
		return errors.New("V2 ref differs from independently held expectation")
	}
	return nil
}
