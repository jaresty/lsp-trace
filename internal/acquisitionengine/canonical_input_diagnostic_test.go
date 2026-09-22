package acquisitionengine

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validationDiagnostic(t *testing.T, raw []byte) ValidationDiagnostic {
	t.Helper()
	_, _, err := CanonicalInput(raw)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error type %T: %v", err, err)
	}
	return validation.Diagnostic
}

func TestCanonicalInputReturnsClosedPredicateDiagnostics(t *testing.T) {
	oversize := append([]byte(`{"session_id":"x","generation":1,"seed_manifest":{}}`), make([]byte, MaxInputBytes)...)
	cases := []struct {
		name, raw, field, invariant string
	}{
		{"byte limit", string(oversize), ValidationFieldEnvelope, ValidationInvariantInputByteLimit},
		{"duplicate member", `{"session_id":"x","session_id":"y"}`, ValidationFieldEnvelope, ValidationInvariantDuplicateJSONMember},
		{"object", `[]`, ValidationFieldEnvelope, ValidationInvariantTopLevelObject},
		{"typed", `{"session_id":"x","generation":"sensitive-value","seed_manifest":{}}`, ValidationFieldEnvelope, ValidationInvariantTypedJSONDecode},
		{"trailing complete value", `{"session_id":"x","generation":1,"seed_manifest":{}} 1`, ValidationFieldEnvelope, ValidationInvariantTrailingContent},
		{"malformed incomplete value", `{"session_id":"x","generation":1,"seed_manifest":{}`, ValidationFieldEnvelope, ValidationInvariantTypedJSONDecode},
		{"manifest null", `{"session_id":"x","generation":1,"seed_manifest":null}`, ValidationFieldSeedManifest, ValidationInvariantNullMember},
		{"position null", `{"session_id":"x","generation":1,"seed_manifest":{"schema_version":"x","coordinate_convention":"x","root":{"id":"x","locator":{"uri":"file:///x","line":null}},"required_targets":[]}}`, ValidationFieldSeedPosition, ValidationInvariantNullMember},
		{"options null", `{"session_id":"x","generation":1,"seed_manifest":{"schema_version":"x","coordinate_convention":"x","root":{"id":"x","locator":{"uri":"file:///x"}},"required_targets":[]},"group_options":null}`, ValidationFieldOptions, ValidationInvariantNullMember},
		{"empty symbol", `{"session_id":"x","generation":1,"seed_manifest":{"schema_version":"x","coordinate_convention":"x","root":{"id":"x","locator":{"uri":"file:///x","symbol":""}},"required_targets":[]}}`, ValidationFieldSeedLabel, ValidationInvariantEmptySelector},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validationDiagnostic(t, []byte(tc.raw))
			if d.FailedField != tc.field || d.FailedInvariant != tc.invariant {
				t.Fatalf("diagnostic=%+v want=%s/%s", d, tc.field, tc.invariant)
			}
			if strings.Contains(d.FailedField+d.FailedInvariant, "sensitive-value") {
				t.Fatal("diagnostic leaked input")
			}
		})
	}
}

func TestCanonicalInputValidValueCanonicalizes(t *testing.T) {
	in := Input{SessionID: "session", Generation: 1, SeedManifest: Manifest{SchemaVersion: "x", CoordinateConvention: "x", RequiredTargets: []Target{}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got, canonical, err := CanonicalInput(raw)
	if err != nil || got.SessionID != in.SessionID || !json.Valid(canonical) {
		t.Fatalf("got=%+v canonical=%s err=%v", got, canonical, err)
	}
}

func TestCanonicalInputDiagnosticSelectionIsDeterministic(t *testing.T) {
	raw := []byte(`{"session_id":"x","generation":1,"seed_manifest":null,"group_options":null}`)
	first := validationDiagnostic(t, raw)
	for i := 0; i < 20; i++ {
		if got := validationDiagnostic(t, raw); got != first {
			t.Fatalf("nondeterministic: first=%+v got=%+v", first, got)
		}
	}
}
