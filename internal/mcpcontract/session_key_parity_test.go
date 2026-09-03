package mcpcontract

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"lsp-trace/internal/session"
)

const sessionKeyMappingFixture = "testdata/session-key-parity.v1.json"

type sessionKeyParityFixture struct {
	Selector        any            `json:"selector"`
	ProcessKeyHex   string         `json:"process_key_hex"`
	Resolved        resolvedInputs `json:"resolved"`
	EncodingHex     string         `json:"encoding_hex"`
	Digest          string         `json:"digest"`
	ProcessIdentity string         `json:"process_identity"`
}

type resolvedInputs struct {
	Command                string            `json:"command"`
	Arguments              []string          `json:"arguments"`
	Environment            map[string]string `json:"environment"`
	LanguageID             string            `json:"language_id"`
	InitializationOptions  optionValue       `json:"initialization_options"`
	WorkspaceConfiguration optionValue       `json:"workspace_configuration"`
}

type sessionKeySelector struct {
	Version                string        `json:"version"`
	TrustDomain            string        `json:"trust_domain"`
	Workspace              string        `json:"workspace"`
	Profile                string        `json:"profile"`
	ServerAffectingOptions []namedOption `json:"server_affecting_options"`
}

type namedOption struct {
	Name  string      `json:"name"`
	Value optionValue `json:"value"`
}

type optionValue map[string]any

func TestSessionKeyContractParity(t *testing.T) {
	for _, assertion := range []string{
		"ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS",
		"ASSERT_SESSION_KEY_PARITY_REJECTION",
		"ASSERT_SESSION_KEY_PARITY_EIGHT_FIELDS",
		"ASSERT_SESSION_KEY_PARITY_ENVIRONMENT",
		"ASSERT_SESSION_KEY_PARITY_CROSS_PACKAGE",
		"ASSERT_SESSION_KEY_PARITY_VALID_VECTORS",
		"ASSERT_SESSION_KEY_PARITY_REJECTION_VECTORS",
		"ASSERT_SESSION_KEY_PARITY_BASELINE_PRESERVED",
		"ASSERT_SESSION_KEY_PARITY_STAGES_RESERVED",
	} {
		t.Log("ASSERTION: " + assertion)
	}

	raw, err := os.ReadFile(sessionKeyMappingFixture)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_VALID_VECTORS: %v", err)
	}
	var fixture sessionKeyParityFixture
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(&fixture); err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_VALID_VECTORS: %v", err)
	}

	contract := loadStage2LifecycleContract(t)
	schemaID := "https://jaresty.github.io/lsp-trace/mcp/schemas/input-session-selector.v1.schema.json"
	compiler := compileStage2ContractSchemas(t, contract)
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_CROSS_PACKAGE: %v", err)
	}
	input := map[string]any{"selector": map[string]any{"session_key": fixture.Selector}}
	if err := schema.Validate(input); err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_VALID_VECTORS: %v", err)
	}

	selectorRaw, _ := json.Marshal(fixture.Selector)
	var selector sessionKeySelector
	selectorDecoder := json.NewDecoder(bytes.NewReader(selectorRaw))
	selectorDecoder.UseNumber()
	if err := selectorDecoder.Decode(&selector); err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS: %v", err)
	}
	values, err := parityValues(selector, fixture, false)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS: %v", err)
	}
	encoding, err := session.EncodeSessionKeyV1(values)
	if err != nil {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_EIGHT_FIELDS: %v", err)
	}
	if got := hex.EncodeToString(encoding); got != fixture.EncodingHex {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_EIGHT_FIELDS: encoding=%s want=%s", got, fixture.EncodingHex)
	}
	if got := session.DigestSessionKey(encoding); got != fixture.Digest {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_EIGHT_FIELDS: digest=%s want=%s", got, fixture.Digest)
	}
	if !session.Reusable(fixture.ProcessIdentity, encoding, fixture.ProcessIdentity, append([]byte(nil), encoding...)) ||
		session.Reusable(fixture.ProcessIdentity, encoding, fixture.ProcessIdentity+"-other", encoding) {
		t.Fatal("ASSERT_SESSION_KEY_PARITY_ENVIRONMENT: process-local reuse identity mismatch")
	}
	processKey, _ := hex.DecodeString(fixture.ProcessKeyHex)
	envA, _ := session.EnvironmentValue(processKey, map[string][]byte{"Z_REF": []byte("z"), "A_REF": []byte("a")})
	envB, _ := session.EnvironmentValue(processKey, map[string][]byte{"A_REF": []byte("a"), "Z_REF": []byte("z")})
	envChanged, _ := session.EnvironmentValue(processKey, map[string][]byte{"A_REF": []byte("changed"), "Z_REF": []byte("z")})
	otherKey := append([]byte(nil), processKey...)
	otherKey[0] ^= 0xff
	envOtherProcess, _ := session.EnvironmentValue(otherKey, map[string][]byte{"A_REF": []byte("a"), "Z_REF": []byte("z")})
	a, _ := envA.Encode()
	b, _ := envB.Encode()
	changed, _ := envChanged.Encode()
	otherProcess, _ := envOtherProcess.Encode()
	if !bytes.Equal(a, b) || bytes.Equal(a, changed) || bytes.Equal(a, otherProcess) {
		t.Fatal("ASSERT_SESSION_KEY_PARITY_ENVIRONMENT: ordering, runtime-value, or process-key discrimination failed")
	}

	assertScalarParity(t)
	assertRejectionVectors(t, schema, fixture, selector)
	assertReservationParity(t, contract)
}

func parityValues(selector sessionKeySelector, fixture sessionKeyParityFixture, reverseOptions bool) ([8]session.Value, error) {
	var values [8]session.Value
	processKey, err := hex.DecodeString(fixture.ProcessKeyHex)
	if err != nil {
		return values, err
	}
	environment := make(map[string][]byte, len(fixture.Resolved.Environment))
	for name, value := range fixture.Resolved.Environment {
		environment[name] = []byte(value)
	}
	environmentValue, err := session.EnvironmentValue(processKey, environment)
	if err != nil {
		return values, err
	}
	arguments := make([]session.Value, len(fixture.Resolved.Arguments))
	for i, argument := range fixture.Resolved.Arguments {
		arguments[i] = session.String(argument)
	}
	options := append([]namedOption(nil), selector.ServerAffectingOptions...)
	if reverseOptions {
		for i, j := 0, len(options)-1; i < j; i, j = i+1, j-1 {
			options[i], options[j] = options[j], options[i]
		}
	}
	optionEntries, err := parityEntries(options)
	if err != nil {
		return values, err
	}
	initValue, err := parityValue(fixture.Resolved.InitializationOptions)
	if err != nil {
		return values, err
	}
	workspaceValue, err := parityValue(fixture.Resolved.WorkspaceConfiguration)
	if err != nil {
		return values, err
	}
	values = [8]session.Value{
		session.String(selector.TrustDomain),
		session.Path(selector.Workspace),
		session.String(selector.Profile),
		session.String(fixture.Resolved.Command),
		session.List(arguments...),
		environmentValue,
		session.LanguageConfiguration(
			session.Entry{Name: "language_id", Value: session.String(fixture.Resolved.LanguageID)},
			session.Entry{Name: "initialization_options", Value: initValue},
			session.Entry{Name: "workspace_configuration", Value: workspaceValue},
		),
		session.ServerAffectingOptions(optionEntries...),
	}
	return values, nil
}

func parityEntries(options []namedOption) ([]session.Entry, error) {
	entries := make([]session.Entry, len(options))
	for i, option := range options {
		value, err := parityValue(option.Value)
		if err != nil {
			return nil, err
		}
		entries[i] = session.Entry{Name: option.Name, Value: value}
	}
	return entries, nil
}

func parityValue(value optionValue) (session.Value, error) {
	if len(value) != 1 {
		return session.Value{}, &parityError{"tagged value must contain exactly one member"}
	}
	for tag, raw := range value {
		switch tag {
		case "null":
			return session.Null(), nil
		case "bool":
			return session.Bool(raw.(bool)), nil
		case "uint":
			value, err := strconv.ParseUint(raw.(json.Number).String(), 10, 64)
			return session.Uint(value), err
		case "int":
			value, err := strconv.ParseInt(raw.(json.Number).String(), 10, 64)
			return session.Int(value), err
		case "string":
			return session.String(raw.(string)), nil
		case "bytes_base64":
			return session.DecodeBase64(raw.(string))
		case "list":
			items := raw.([]any)
			values := make([]session.Value, len(items))
			for i, item := range items {
				mapped, err := parityValue(item.(map[string]any))
				if err != nil {
					return session.Value{}, err
				}
				values[i] = mapped
			}
			return session.List(values...), nil
		case "map":
			items := raw.([]any)
			options := make([]namedOption, len(items))
			for i, item := range items {
				encoded, _ := json.Marshal(item)
				if err := json.Unmarshal(encoded, &options[i]); err != nil {
					return session.Value{}, err
				}
			}
			entries, err := parityEntries(options)
			return session.Map(entries...), err
		default:
			return session.Value{}, &parityError{"unknown selector value tag: " + tag}
		}
	}
	panic("unreachable")
}

type parityError struct{ message string }

func (e *parityError) Error() string { return e.message }

func assertScalarParity(t *testing.T) {
	t.Helper()
	vectors := []struct {
		name string
		wire optionValue
		want session.Value
	}{
		{"uint64 maximum", optionValue{"uint": json.Number("18446744073709551615")}, session.Uint(^uint64(0))},
		{"int64 minimum", optionValue{"int": json.Number("-9223372036854775808")}, session.Int(-1 << 63)},
		{"int64 maximum", optionValue{"int": json.Number("9223372036854775807")}, session.Int(1<<63 - 1)},
		{"NFC string", optionValue{"string": "e\u0301"}, session.String("é")},
	}
	for _, vector := range vectors {
		got, err := parityValue(vector.wire)
		if err != nil {
			t.Fatalf("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS[%s]: %v", vector.name, err)
		}
		gotEncoding, gotErr := got.Encode()
		wantEncoding, wantErr := vector.want.Encode()
		if gotErr != nil || wantErr != nil || !bytes.Equal(gotEncoding, wantEncoding) {
			t.Fatalf("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS[%s]: conversion mismatch", vector.name)
		}
	}
}

func assertRejectionVectors(t *testing.T, schema interface{ Validate(any) error }, fixture sessionKeyParityFixture, selector sessionKeySelector) {
	t.Helper()
	base := cloneJSON(fixture.Selector).(map[string]any)
	vectors := []struct {
		name  string
		value any
	}{
		{"missing profile", without(base, "profile")},
		{"caller resolved command", with(base, "command", "gopls")},
		{"caller resolved arguments", with(base, "arguments", []any{"serve"})},
		{"caller environment references", with(base, "environment", map[string]any{"GOPATH": "secret"})},
		{"caller language configuration", with(base, "language_configuration", map[string]any{"language_id": "go"})},
		{"caller process key", with(base, "process_key", "secret")},
		{"caller environment digest", with(base, "environment_digest", "sha256:secret")},
		{"caller session digest", with(base, "session_key_digest", "sha256:secret")},
	}
	for _, vector := range vectors {
		t.Run("reject/"+vector.name, func(t *testing.T) {
			input := map[string]any{"selector": map[string]any{"session_key": vector.value}}
			if err := schema.Validate(input); err == nil {
				t.Fatalf("ASSERT_SESSION_KEY_PARITY_REJECTION_VECTORS: accepted %s", vector.name)
			}
		})
	}

	canonical, err := parityValues(selector, fixture, false)
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := parityValues(selector, fixture, true)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := session.EncodeSessionKeyV1(canonical)
	b, _ := session.EncodeSessionKeyV1(reversed)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS: canonical option ordering changed encoding")
	}

	pathPairs := [][2]string{
		{"/work/./repo", "/work/repo"},
		{`c:\Work\.\Repo`, "C:/Work/Repo"},
		{`\\Srv\Share\a\..\repo`, "//Srv/Share/repo"},
	}
	for _, pair := range pathPairs {
		left, right := selector, selector
		left.Workspace, right.Workspace = pair[0], pair[1]
		leftValues, leftErr := parityValues(left, fixture, false)
		rightValues, rightErr := parityValues(right, fixture, false)
		leftEncoding, encodeLeftErr := session.EncodeSessionKeyV1(leftValues)
		rightEncoding, encodeRightErr := session.EncodeSessionKeyV1(rightValues)
		if leftErr != nil || rightErr != nil || encodeLeftErr != nil || encodeRightErr != nil || !bytes.Equal(leftEncoding, rightEncoding) {
			t.Fatalf("ASSERT_SESSION_KEY_PARITY_CANONICAL_INPUTS: path pair %q/%q differs", pair[0], pair[1])
		}
	}

	invalidPaths := []string{"relative/path", `C:relative`, `//server`}
	for _, workspace := range invalidPaths {
		changed := selector
		changed.Workspace = workspace
		values, err := parityValues(changed, fixture, false)
		if err == nil {
			_, err = session.EncodeSessionKeyV1(values)
		}
		if err == nil {
			t.Fatalf("ASSERT_SESSION_KEY_PARITY_REJECTION: accepted path %q", workspace)
		}
	}
}

func assertReservationParity(t *testing.T, contract stage2LifecycleContract) {
	t.Helper()
	for _, tool := range contract.Tools {
		if tool.Advertised || tool.Availability != "NOT_IMPLEMENTED" {
			t.Fatalf("ASSERT_SESSION_KEY_PARITY_STAGES_RESERVED: %+v", tool)
		}
	}
	stage1, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	advertised := 0
	for _, tool := range stage1.Tools {
		if tool.Advertised {
			advertised++
		}
		if strings.HasPrefix(tool.Name, "lsp_trace_v1_incoming") || strings.HasPrefix(tool.Name, "lsp_trace_v1_slice") {
			if tool.Advertised || tool.Availability != "NOT_IMPLEMENTED" {
				t.Fatalf("ASSERT_SESSION_KEY_PARITY_STAGES_RESERVED: %+v", tool)
			}
		}
	}
	if advertised != 6 {
		t.Fatalf("ASSERT_SESSION_KEY_PARITY_BASELINE_PRESERVED: advertised=%d", advertised)
	}
}

func cloneJSON(value any) any {
	raw, _ := json.Marshal(value)
	var cloned any
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func without(base map[string]any, name string) map[string]any {
	value := cloneJSON(base).(map[string]any)
	delete(value, name)
	return value
}

func with(base map[string]any, name string, replacement any) map[string]any {
	value := cloneJSON(base).(map[string]any)
	value[name] = replacement
	return value
}
