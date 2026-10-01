// Package adr0011builtinprofile provides a private, default-off V1 original-
// byte comparator. It neither selects a host principal nor invokes LSP.
package adr0011builtinprofile

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"lsp-trace/internal/strictjson"
)

//go:embed originals/*.json
var originals embed.FS

var ErrHeldMismatch = errors.New("private held V1 originals mismatch")
var ErrInvalidFreeze = errors.New("invalid private V1 freeze")

// Originals are complete exact bytes, not a digest-only assertion. Descendants
// are independently selected at freeze time; claimant records cannot add roles.
type Originals struct {
	Profile, TargetPolicy, TargetSchema, Measurement, Plan, Selection []byte
	Descendants                                                       map[string][]byte
}
type HeldV1 struct{ selected Originals }

func copyBytes(b []byte) []byte { return append([]byte(nil), b...) }
func clone(x Originals) Originals {
	y := Originals{copyBytes(x.Profile), copyBytes(x.TargetPolicy), copyBytes(x.TargetSchema), copyBytes(x.Measurement), copyBytes(x.Plan), copyBytes(x.Selection), make(map[string][]byte, len(x.Descendants))}
	for k, v := range x.Descendants {
		y.Descendants[k] = copyBytes(v)
	}
	return y
}
func BuiltinV1() Originals {
	p, _ := originals.ReadFile("originals/profile-v1.json")
	policy, _ := originals.ReadFile("originals/target-policy-v1.json")
	schema, _ := originals.ReadFile("originals/target-schema-v1.json")
	return Originals{Profile: p, TargetPolicy: policy, TargetSchema: schema}
}
func digest(b []byte) string { sum := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(sum[:]) }
func domainDigest(domain string, b []byte) string {
	return digest(append(append([]byte(domain), 0), b...))
}
func canonical(x any) []byte { b, _ := json.Marshal(x); return b }
func object(b []byte) map[string]any {
	var x map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if dec.Decode(&x) != nil {
		return nil
	}
	return x
}
func validOriginal(b []byte) bool {
	if len(b) == 0 || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	x := object(b)
	return x != nil && bytes.Equal(b, canonical(x))
}
func field(x map[string]any, k string) string          { s, _ := x[k].(string); return s }
func nested(x map[string]any, k string) map[string]any { v, _ := x[k].(map[string]any); return v }
func targetImplementationFor(measure, policy, schema string) string {
	p := object(BuiltinV1().Profile)
	target := nested(p, "target_query")
	return domainDigest("ADR0011_DOCUMENT_SYMBOL_TARGET_IMPLEMENTATION_V1", canonical(map[string]any{"measurement_digest": measure, "target_policy_digest": policy, "target_schema_digest": schema, "target_role_uri": field(target, "schema_role_uri")}))
}
func targetImplementation(measure string) string {
	p := BuiltinV1()
	return targetImplementationFor(measure, digest(p.TargetPolicy), digest(p.TargetSchema))
}
func validateV1Roles(x Originals) error {
	schemaBytes, err := originals.ReadFile("originals/profile-schema-v1.json")
	if err != nil || digest(schemaBytes) != "sha256:d6bed3453c002d7d509a8b7fbfb17a78aeb61fee0a576f9728ac78dccaf0bdce" {
		return ErrInvalidFreeze
	}
	var document map[string]any
	if json.Unmarshal(schemaBytes, &document) != nil {
		return ErrInvalidFreeze
	}
	id, ok := document["$id"].(string)
	if !ok {
		return ErrInvalidFreeze
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if compiler.AddResource(id, document) != nil {
		return ErrInvalidFreeze
	}
	for _, role := range []struct {
		name string
		raw  []byte
	}{
		{"profile", x.Profile}, {"implementationMeasurement", x.Measurement},
		{"plan", x.Plan}, {"hostSelection", x.Selection},
	} {
		compiled, err := compiler.Compile(id + "#/$defs/" + role.name)
		if err != nil {
			return ErrInvalidFreeze
		}
		var value any
		if json.Unmarshal(role.raw, &value) != nil || compiled.Validate(value) != nil {
			return ErrInvalidFreeze
		}
	}
	return nil
}
func check(x Originals) error {
	pinned := BuiltinV1()
	if digest(pinned.Profile) != "sha256:ae5e66838d140399603536021ada70317b95e836c0887028180fa58c0123881d" || digest(pinned.TargetPolicy) != "sha256:1b95b7059c1246fb09a160a72e0424266cc79f53cc984b7f56e74982d7078134" || digest(pinned.TargetSchema) != "sha256:bf49de9460fee5a68ba13204f633dc72d2dca566fa5f92d704bc19b5514618ba" {
		return ErrInvalidFreeze
	}
	if !bytes.Equal(x.Profile, pinned.Profile) || !bytes.Equal(x.TargetPolicy, pinned.TargetPolicy) || !bytes.Equal(x.TargetSchema, pinned.TargetSchema) {
		return ErrInvalidFreeze
	}
	for _, b := range [][]byte{x.Profile, x.TargetPolicy, x.Measurement, x.Plan, x.Selection} {
		if !validOriginal(b) {
			return ErrInvalidFreeze
		}
	}
	if validateV1Roles(x) != nil {
		return ErrInvalidFreeze
	}
	profile, measurement, plan, selection := object(x.Profile), object(x.Measurement), object(x.Plan), object(x.Selection)
	if field(profile, "model") != "BUILTIN_LOCAL_REFERENCES_V1" || field(measurement, "role") != "ADR0011_INSTALLED_EXECUTABLE_MEASUREMENT_V1" || field(plan, "role") != "ADR0011_LOCAL_REFERENCES_PLAN_V1" || field(selection, "role") != "ADR0011_LOCAL_REFERENCES_SELECTION_V1" {
		return ErrInvalidFreeze
	}
	pd, md, td, sd := digest(x.Profile), digest(x.Measurement), digest(x.TargetPolicy), digest(x.TargetSchema)
	impl := targetImplementationFor(md, td, sd)
	if field(measurement, "profile_digest") != pd || field(plan, "profile_digest") != pd || field(plan, "implementation_digest") != md || field(plan, "target_policy_digest") != td || field(plan, "target_schema_digest") != sd || field(plan, "target_implementation_digest") != impl {
		return ErrInvalidFreeze
	}
	for _, k := range []string{"profile_digest", "implementation_digest", "target_policy_digest", "target_schema_digest", "target_implementation_digest", "schema_digest", "invocation_nonce"} {
		if field(plan, k) == "" || field(plan, k) != field(selection, k) {
			return ErrInvalidFreeze
		}
	}
	selectedPolicies := nested(plan, "policy_digests")
	profilePolicies := nested(profile, "policies")
	if len(selectedPolicies) != 4 {
		return ErrInvalidFreeze
	}
	for _, role := range []string{"method", "admission", "privacy", "retention"} {
		if field(selectedPolicies, role) != field(nested(profilePolicies, role), "digest") {
			return ErrInvalidFreeze
		}
	}
	if field(plan, "schema_digest") != field(nested(profile, "admission_schema"), "digest") ||
		!bytes.Equal(canonical(plan["policy_digests"]), canonical(selection["policy_digests"])) ||
		field(selection, "plan_id") != domainDigest("ADR0011_LOCAL_REFERENCES_PLAN_V1", x.Plan) ||
		field(nested(selection, "plan_ref"), "digest") != digest(x.Plan) ||
		fmt.Sprint(nested(selection, "plan_ref")["byte_length"]) != fmt.Sprint(len(x.Plan)) {
		return ErrInvalidFreeze
	}
	// This is a private test/implementation seam, not schema validation or a
	// production admission rule; callers must independently own the freeze.
	for k, v := range x.Descendants {
		if k == "" || !validOriginal(v) {
			return ErrInvalidFreeze
		}
	}
	return nil
}

// FreezeV1 accepts only the installed, exact built-in profile and target
// originals and holds all remaining selected bytes independently of replay.
func FreezeV1(x Originals) (HeldV1, error) {
	if err := check(x); err != nil {
		return HeldV1{}, err
	}
	return HeldV1{selected: clone(x)}, nil
}

// HoldDescendants is a private host-side transition after pre-WRITE freeze.
// It must receive host-selected original bytes, never values from a claimant
// result. It does not mutate the previous expectation.
func (h HeldV1) HoldDescendants(selected map[string][]byte) (HeldV1, error) {
	if len(h.selected.Profile) == 0 || len(h.selected.Descendants) != 0 || len(selected) == 0 {
		return HeldV1{}, ErrInvalidFreeze
	}
	next := clone(h.selected)
	for k, v := range selected {
		if k == "" || !validOriginal(v) {
			return HeldV1{}, ErrInvalidFreeze
		}
		next.Descendants[k] = copyBytes(v)
	}
	return HeldV1{selected: next}, nil
}

// Replay compares complete claimant bytes before interpreting any descendant;
// rehashing a coherent claim cannot change this held expectation.
func (h HeldV1) Replay(claim Originals) error {
	if len(h.selected.Profile) == 0 {
		return ErrHeldMismatch
	}
	for _, pair := range [][2][]byte{{h.selected.Profile, claim.Profile}, {h.selected.TargetPolicy, claim.TargetPolicy}, {h.selected.TargetSchema, claim.TargetSchema}, {h.selected.Measurement, claim.Measurement}, {h.selected.Plan, claim.Plan}, {h.selected.Selection, claim.Selection}} {
		if !bytes.Equal(pair[0], pair[1]) {
			return ErrHeldMismatch
		}
	}
	if len(h.selected.Descendants) != len(claim.Descendants) {
		return ErrHeldMismatch
	}
	keys := make([]string, 0, len(h.selected.Descendants))
	for k := range h.selected.Descendants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := claim.Descendants[k]
		if !ok || !bytes.Equal(h.selected.Descendants[k], v) {
			return fmt.Errorf("%w: descendant %s", ErrHeldMismatch, k)
		}
	}
	return nil
}

// Snapshot is a copy for private retained original-byte storage. Restoring
// requires an independent digest recorded outside the claimant snapshot.
func (h HeldV1) Snapshot() Originals { return clone(h.selected) }
func RestoreV1(snapshot Originals, independentlyHeldDigest string) (HeldV1, error) {
	if independentlyHeldDigest == "" || independentlyHeldDigest != snapshotDigest(snapshot) {
		return HeldV1{}, ErrHeldMismatch
	}
	return FreezeV1(snapshot)
}
func snapshotDigest(x Originals) string {
	parts := [][]byte{x.Profile, x.TargetPolicy, x.TargetSchema, x.Measurement, x.Plan, x.Selection}
	keys := make([]string, 0, len(x.Descendants))
	for k := range x.Descendants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, []byte(k), x.Descendants[k])
	}
	var framed []byte
	for _, part := range parts {
		framed = append(framed, []byte(fmt.Sprintf("%d:", len(part)))...)
		framed = append(framed, part...)
	}
	return domainDigest("ADR0011_HELD_V1_SNAPSHOT", framed)
}
func (h HeldV1) SnapshotDigest() string { return snapshotDigest(h.selected) }
