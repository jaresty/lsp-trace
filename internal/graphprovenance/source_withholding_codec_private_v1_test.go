//go:build darwin || linux

package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/graph"
)

func TestPrivateSourceCodecRetainedContextRoundtrip(t *testing.T) {
	for _, nondefault := range []bool{false, true} {
		name := "default-wire-compatible"
		if nondefault {
			name = "nondefault-retained-context"
		}
		t.Run(name, func(t *testing.T) {
			r, p, expected, _ := withholdingFixturePrivateV1(t)
			if nondefault {
				r.Graph.Invocation.Server.Environment = map[string]string{"FIXTURE_TOKEN": "not-a-real-secret"}
				r.Graph.Invocation.EffectiveEnvironment = []string{"FIXTURE_TOKEN=not-a-real-secret", "PATH=/fixture/frozen-bin"}
				r.Graph.Invocation.WorkingDirectory = "/fixture/original-cwd"
			}
			original, err := json.Marshal(r.Graph)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := captureSourceWithholdingPrivateV1(context.Background(), r, p, expected, readSourcePrivateV1)
			if err != nil {
				t.Fatal("ASSERT_ACTUAL_PRIVATE_CAPTURE", err)
			}
			parsed, err := ParseSourceWithholdingPrivateV1(raw)
			if err != nil {
				t.Fatal("ASSERT_PRIVATE_NONDEFAULT_PARSE", err)
			}
			if !bytes.Equal(original, parsed.Evidence.GraphBytes) {
				t.Fatal("ASSERT_ORIGINAL_NATIVE_BYTES_UNCHANGED")
			}
			var native struct {
				Context graph.ProcessContext `json:"process_context"`
			}
			if json.Unmarshal(parsed.Evidence.GraphBytes, &native) != nil {
				t.Fatal("native context")
			}
			if nondefault && (native.Context.EffectiveEnvironmentVariableCount != 2 || len(native.Context.Environment) != 1) {
				t.Fatal("ASSERT_NONDEFAULT_CONTEXT_NONTRIVIAL")
			}
			roundtrip, err := json.Marshal(parsed)
			if err != nil || !bytes.Equal(bytes.TrimSpace(raw), roundtrip) {
				t.Fatal("ASSERT_PRIVATE_EXACT_WIRE_ROUNDTRIP", err)
			}
			again, err := ParseSourceWithholdingPrivateV1(roundtrip)
			if err != nil {
				t.Fatal(err)
			}
			var after struct {
				Context graph.ProcessContext `json:"process_context"`
			}
			if json.Unmarshal(again.Evidence.GraphBytes, &after) != nil || !reflect.DeepEqual(native, after) {
				t.Fatal("ASSERT_PRIVATE_CONTEXT_EXACT")
			}
			// The historical ordinary codec is intentionally unchanged. Its failure is
			// the exact pre-fix private parser's re-encoding failure, not authentication.
			type legacySource SourceWithholdingPrivateV1
			legacy, oldErr := json.Marshal(legacySource(parsed))
			if nondefault {
				if oldErr == nil {
					t.Fatal("ASSERT_ORDINARY_V2_NONDEFAULT_BEHAVIOR_UNCHANGED")
				}
				t.Logf("LEGACY_PRIVATE_REENCODE_RED: %v", oldErr)
				forged := bytes.Replace(raw, []byte(`"effective_environment_variable_count":2`), []byte(`"effective_environment_variable_count":3`), 1)
				if bytes.Equal(raw, forged) {
					t.Fatal("context mismatch mutation absent")
				}
				if _, err := ParseSourceWithholdingPrivateV1(forged); err == nil {
					t.Fatal("ASSERT_ACQUISITION_CONTEXT_SUBSTITUTION_REFUSED")
				}
			} else if oldErr != nil || !bytes.Equal(legacy, roundtrip) {
				t.Fatal("ASSERT_DEFAULT_LEGACY_BYTES_IDENTICAL", oldErr)
			}
			if ValidateV2(parsed.Evidence) == nil {
				t.Fatal("ASSERT_PRIVATE_NOT_ORDINARY_V2")
			}
			if _, err := ValidateFor(raw, Family, "v2"); err == nil {
				t.Fatal("ASSERT_PRIVATE_WIRE_NOT_ORDINARY_V2")
			}
			parsed.Evidence.Acquisition.Graph.Nodes[0].Name += "forged"
			if _, err := json.Marshal(parsed); err == nil {
				t.Fatal("ASSERT_TYPED_GRAPH_SUBSTITUTION_REFUSED")
			}
		})
	}
}
