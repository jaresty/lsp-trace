package mcpcontract

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// SelectorLimits bounds semantic work after structural schema validation.
type SelectorLimits struct {
	MaxSessionIDBytes    int
	MaxTrustDomainBytes  int
	MaxWorkspaceBytes    int
	MaxProfileBytes      int
	MaxOptionNameBytes   int
	MaxValueBytes        int
	MaxTotalDecodedBytes int
	MaxDepth             int
	MaxCollectionSize    int
}

func SessionSelectorV1Limits() SelectorLimits {
	return SelectorLimits{
		MaxSessionIDBytes: 256, MaxTrustDomainBytes: 256, MaxWorkspaceBytes: 4096,
		MaxProfileBytes: 256, MaxOptionNameBytes: 128, MaxValueBytes: 4096,
		MaxTotalDecodedBytes: 64 * 1024, MaxDepth: 8, MaxCollectionSize: 64,
	}
}

// ValidateSessionSelectorSemantics rejects non-canonical or excessive selector
// values independently of lifecycle availability and dispatch.
func ValidateSessionSelectorSemantics(raw []byte, limits SelectorLimits) error {
	if limits.MaxSessionIDBytes < 1 || limits.MaxTrustDomainBytes < 1 || limits.MaxWorkspaceBytes < 1 || limits.MaxProfileBytes < 1 || limits.MaxOptionNameBytes < 1 || limits.MaxValueBytes < 0 || limits.MaxTotalDecodedBytes < 1 || limits.MaxDepth < 1 || limits.MaxCollectionSize < 1 {
		return fmt.Errorf("SELECTOR_LIMITS: invalid limits")
	}
	var input selectorInput
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return fmt.Errorf("SELECTOR_STRUCTURE: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("SELECTOR_TRAILING_JSON: one JSON value required")
	}
	hasID, hasKey := input.Selector.SessionID != nil, input.Selector.SessionKey != nil
	if hasID == hasKey {
		return fmt.Errorf("SELECTOR_UNION: exactly one selector member required")
	}
	total := 0
	add := func(n int) error {
		total += n
		if total > limits.MaxTotalDecodedBytes {
			return fmt.Errorf("SELECTOR_TOTAL_DECODED_BYTES: got %d max %d", total, limits.MaxTotalDecodedBytes)
		}
		return nil
	}
	if hasID {
		return checkSelectorString(*input.Selector.SessionID, limits.MaxSessionIDBytes, false, add)
	}
	key := input.Selector.SessionKey
	if key.Version != "1" {
		return fmt.Errorf("SELECTOR_VERSION: unrecognized version %q", key.Version)
	}
	for _, field := range []struct {
		value string
		max   int
	}{{key.Version, 1}, {key.TrustDomain, limits.MaxTrustDomainBytes}, {key.Workspace, limits.MaxWorkspaceBytes}, {key.Profile, limits.MaxProfileBytes}} {
		if err := checkSelectorString(field.value, field.max, true, add); err != nil {
			return err
		}
	}
	if err := checkWorkspacePath(key.Workspace); err != nil {
		return err
	}
	return checkOptionCollection(key.Options, 1, limits, add)
}

type selectorInput struct {
	Selector struct {
		SessionID  *string             `json:"session_id,omitempty"`
		SessionKey *selectorSessionKey `json:"session_key,omitempty"`
	} `json:"selector"`
	Generation *uint64 `json:"generation,omitempty"`
}

type selectorSessionKey struct {
	Version     string           `json:"version"`
	TrustDomain string           `json:"trust_domain"`
	Workspace   string           `json:"workspace"`
	Profile     string           `json:"profile"`
	Options     []selectorOption `json:"server_affecting_options"`
}

type selectorOption struct {
	Name  string        `json:"name"`
	Value selectorValue `json:"value"`
}

type selectorValue struct {
	Null        *bool             `json:"null,omitempty"`
	Bool        *bool             `json:"bool,omitempty"`
	Uint        *uint64           `json:"uint,omitempty"`
	Int         *int64            `json:"int,omitempty"`
	String      *string           `json:"string,omitempty"`
	BytesBase64 *string           `json:"bytes_base64,omitempty"`
	List        *[]selectorValue  `json:"list,omitempty"`
	Map         *[]selectorOption `json:"map,omitempty"`
}

type byteAdder func(int) error

func checkSelectorString(value string, max int, nonempty bool, add byteAdder) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("SELECTOR_UTF8: invalid UTF-8")
	}
	if !norm.NFC.IsNormalString(value) {
		return fmt.Errorf("SELECTOR_NON_NFC: string is not NFC")
	}
	n := len(value)
	if nonempty && n == 0 {
		return fmt.Errorf("SELECTOR_STRING_EMPTY: string is empty")
	}
	if n > max {
		return fmt.Errorf("SELECTOR_STRING_BYTES: got %d max %d", n, max)
	}
	return add(n)
}

func checkWorkspacePath(value string) error {
	n := strings.ReplaceAll(value, "\\", "/")
	if strings.HasPrefix(n, "//?/") || strings.HasPrefix(n, "//./") {
		return fmt.Errorf("SELECTOR_WORKSPACE_PATH: device path")
	}
	parts := []string(nil)
	if strings.HasPrefix(n, "//") {
		p := strings.Split(strings.TrimPrefix(n, "//"), "/")
		if len(p) < 2 || p[0] == "" || p[1] == "" {
			return fmt.Errorf("SELECTOR_WORKSPACE_PATH: invalid UNC path")
		}
		parts = p[2:]
	} else if len(n) >= 2 && n[1] == ':' {
		if len(n) < 3 || n[2] != '/' || !((n[0] >= 'a' && n[0] <= 'z') || (n[0] >= 'A' && n[0] <= 'Z')) {
			return fmt.Errorf("SELECTOR_WORKSPACE_PATH: drive-relative path")
		}
		parts = strings.Split(n[3:], "/")
	} else if strings.HasPrefix(n, "/") {
		parts = strings.Split(strings.TrimPrefix(n, "/"), "/")
	} else {
		return fmt.Errorf("SELECTOR_WORKSPACE_PATH: path is not absolute")
	}
	depth := 0
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			if depth == 0 {
				return fmt.Errorf("SELECTOR_WORKSPACE_PATH: path crosses root")
			}
			depth--
			continue
		}
		if strings.Contains(p, ":") {
			return fmt.Errorf("SELECTOR_WORKSPACE_PATH: alternate data stream")
		}
		depth++
	}
	return nil
}

func checkOptionCollection(options []selectorOption, depth int, limits SelectorLimits, add byteAdder) error {
	if len(options) > limits.MaxCollectionSize {
		return fmt.Errorf("SELECTOR_COLLECTION_SIZE: got %d max %d", len(options), limits.MaxCollectionSize)
	}
	previous := ""
	for i, option := range options {
		if err := checkSelectorString(option.Name, limits.MaxOptionNameBytes, true, add); err != nil {
			return err
		}
		if i > 0 && previous >= option.Name {
			return fmt.Errorf("SELECTOR_OPTIONS_ORDER: %q then %q", previous, option.Name)
		}
		previous = option.Name
		if err := checkSelectorValue(option.Value, depth, limits, add); err != nil {
			return err
		}
	}
	return nil
}

func checkSelectorValue(value selectorValue, depth int, limits SelectorLimits, add byteAdder) error {
	if depth > limits.MaxDepth {
		return fmt.Errorf("SELECTOR_DEPTH: got %d max %d", depth, limits.MaxDepth)
	}
	members := 0
	if value.Null != nil {
		members++
		if !*value.Null {
			return fmt.Errorf("SELECTOR_UNION: null must be true")
		}
	}
	if value.Bool != nil {
		members++
	}
	if value.Uint != nil {
		members++
	}
	if value.Int != nil {
		members++
	}
	if value.String != nil {
		members++
	}
	if value.BytesBase64 != nil {
		members++
	}
	if value.List != nil {
		members++
	}
	if value.Map != nil {
		members++
	}
	if members != 1 {
		return fmt.Errorf("SELECTOR_UNION: exactly one value member required")
	}
	if value.String != nil {
		return checkSelectorString(*value.String, limits.MaxValueBytes, false, add)
	}
	if value.BytesBase64 != nil {
		decoded, err := base64.StdEncoding.Strict().DecodeString(*value.BytesBase64)
		if err != nil {
			return fmt.Errorf("SELECTOR_BASE64: %w", err)
		}
		if base64.StdEncoding.EncodeToString(decoded) != *value.BytesBase64 {
			return fmt.Errorf("SELECTOR_BASE64_CANONICAL: non-canonical base64")
		}
		if len(decoded) > limits.MaxValueBytes {
			return fmt.Errorf("SELECTOR_DECODED_BYTES: got %d max %d", len(decoded), limits.MaxValueBytes)
		}
		return add(len(decoded))
	}
	if value.List != nil {
		if len(*value.List) > limits.MaxCollectionSize {
			return fmt.Errorf("SELECTOR_COLLECTION_SIZE: got %d max %d", len(*value.List), limits.MaxCollectionSize)
		}
		for _, child := range *value.List {
			if err := checkSelectorValue(child, depth+1, limits, add); err != nil {
				return err
			}
		}
		return nil
	}
	if value.Map != nil {
		return checkOptionCollection(*value.Map, depth+1, limits, add)
	}
	return nil
}
