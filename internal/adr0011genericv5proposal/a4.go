// Package adr0011genericv5proposal contains private offline proposal checks only.
package adr0011genericv5proposal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/strictjson"
)

// Pins are literal source bytes, not recomputed from a vector fixture.
var a4Pins = map[string]struct {
	name   string
	length int
	digest string
}{
	"schema":     {"adr0011-generic-envelope-v4.schema.json", 21409, "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3"},
	"transport":  {"generic-lsp-exact-transport-v4.json", 1784, "f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8"},
	"shared":     {"generic-lsp-selector-applicability-v1.json", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"},
	"references": {"generic-lsp-references-exact-v5.json", 2210, "7c46893fecf0dda7a4942b8f33d92f0c2f49793d003c7eab4e8995aeddf8da36"},
	"definition": {"generic-lsp-definition-exact-v5.json", 2119, "da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3"},
}

func a4BareDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil && s == strings.ToLower(s)
}
func a4Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// A4Original verifies the exact original role, byte length, and digest on each read,
// and returns an owned copy. Callers cannot mutate the verified source in memory.
func A4Original(role string) ([]byte, error) {
	pin, ok := a4Pins[role]
	if !ok {
		return nil, fmt.Errorf("unknown A4 original %q", role)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(source) {
		return nil, errors.New("A4 source root unavailable")
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "docs", "qualification", "originals", pin.name))
	if e != nil {
		return nil, e
	}
	if len(b) != pin.length || a4Hash(b) != pin.digest {
		return nil, fmt.Errorf("A4 original %s pin mismatch", role)
	}
	return append([]byte(nil), b...), nil
}

var a4Roles = map[string]bool{"POLICY": true, "SCHEMA": true, "QUERY": true, "SOURCE": true, "QUERY_APPLICABILITY": true, "PROCESS": true, "CAPABILITY_EVENTS": true, "REQUEST_WRITE": true, "INBOUND_FRAMES": true, "RESULT_READ": true, "TARGET_EVENTS": true, "TERMINAL": true, "READBACK": true, "TRANSACTION": true}

const a4SchemaURI = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v4.schema.json"

// A4Validate applies the complete pinned Draft 2020-12 schema or its complete
// named role definition. It deliberately makes no cross-record B4 claims.
func A4Validate(role string, body []byte) error {
	if !a4Roles[role] {
		return fmt.Errorf("unknown A4 role %q", role)
	}
	if err := strictjson.RejectDuplicates(body); err != nil {
		return err
	}
	source, e := A4Original("schema")
	if e != nil {
		return e
	}
	var schema, instance any
	if e = json.Unmarshal(source, &schema); e != nil {
		return e
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if e = decoder.Decode(&instance); e != nil {
		return e
	}
	// json.Unmarshal rejects trailing data before the schema validator sees it.
	if !json.Valid(body) {
		return errors.New("invalid complete JSON value")
	}
	c := jsonschema.NewCompiler()
	if e = c.AddResource(a4SchemaURI, schema); e != nil {
		return e
	}
	compiled, e := c.Compile(a4SchemaURI + "#/$defs/" + role)
	if e != nil {
		return e
	}
	return compiled.Validate(instance)
}

// A4Selector computes a private candidate only; its caller must supply a
// complete artifact and role suffix. Session and generation are explicit.
func A4Selector(method, role, session, generation string, artifact []byte, fields ...string) (string, error) {
	schema, e := A4Original("schema")
	if e != nil {
		return "", e
	}
	transport, e := A4Original("transport")
	if e != nil {
		return "", e
	}
	policy, e := A4Original(method)
	if e != nil {
		return "", e
	}
	return a4SelectorWithIdentity(method, role, session, generation, artifact, schema, transport, policy, fields...)
}
func a4SelectorWithIdentity(method, role, session, generation string, artifact, schema, transport, policy []byte, fields ...string) (string, error) {
	n, parseErr := strconv.ParseUint(generation, 10, 64)
	if session == "" || parseErr != nil || n == 0 || strconv.FormatUint(n, 10) != generation {
		return "", errors.New("invalid transaction identity")
	}
	if method != "references" && method != "definition" {
		return "", errors.New("unknown method")
	}
	counts := map[string]int{"QUERY": 6, "SOURCE": 1, "QUERY_APPLICABILITY": 1, "CAPABILITY_EVENTS": 1, "TARGET_EVENTS": 3}
	count, ok := counts[role]
	if !ok || len(fields) != count {
		return "", errors.New("unknown role or invalid suffix count")
	}
	if len(artifact) == 0 || len(schema) == 0 || len(transport) == 0 || len(policy) == 0 {
		return "", errors.New("missing selector preimage")
	}
	for _, field := range fields {
		if field == "" {
			return "", errors.New("empty selector suffix")
		}
	}
	switch role {
	case "QUERY":
		if !a4BareDigest(fields[2]) || (fields[3] != "utf-8" && fields[3] != "utf-16" && fields[3] != "utf-32") {
			return "", errors.New("invalid query suffix")
		}
		for _, i := range []int{4, 5} {
			v, e := strconv.ParseUint(fields[i], 10, 64)
			if e != nil || strconv.FormatUint(v, 10) != fields[i] {
				return "", errors.New("invalid query point")
			}
		}
	case "TARGET_EVENTS":
		if !a4BareDigest(fields[1]) {
			return "", errors.New("invalid target digest")
		}
		v, e := strconv.ParseUint(fields[2], 10, 64)
		if e != nil || strconv.FormatUint(v, 10) != fields[2] {
			return "", errors.New("invalid target ordinal")
		}
	case "SOURCE", "QUERY_APPLICABILITY", "CAPABILITY_EVENTS":
		if !strings.HasPrefix(fields[0], "sha256:") || !a4BareDigest(strings.TrimPrefix(fields[0], "sha256:")) {
			return "", errors.New("invalid transaction selector")
		}
	}
	parts := []string{"ADR0011-GENERIC-EXACT/5", "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V5", "textDocument/" + method, role, a4Hash(schema), a4Hash(transport), a4Hash(policy), session, generation, a4Hash(artifact)}
	parts = append(parts, fields...)
	var preimage []byte
	for _, p := range parts {
		chunk := []byte(p)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(chunk)))
		preimage = append(preimage, size[:]...)
		preimage = append(preimage, chunk...)
	}
	return "sha256:" + a4Hash(preimage), nil
}
