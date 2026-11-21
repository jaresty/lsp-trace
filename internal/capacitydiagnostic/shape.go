package capacitydiagnostic

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

//go:generate go run ./genschema

// The descriptor below, not the DTO tags or generated JSON file, owns the V3
// wire shape. Keep this deliberately local: it is not a general schema engine.
// Validation visits raw exact names before any encoding/json struct assignment.
type wireField struct {
	name     string
	optional bool
	rule     wireRule
}
type wireRule struct {
	kind     string
	values   []any
	minimum  uint64
	maximum  uint64
	multiple uint64
	object   []wireField
}
type reasonShape struct {
	reason string
	// nil forbids metrics; otherwise metrics are required, with these refinements.
	metrics []wireField
}

func constant(v any) wireRule { return wireRule{values: []any{v}} }
func integer(min, max uint64) wireRule {
	return wireRule{kind: "integer", minimum: min, maximum: max}
}

var capacityReasons = []reasonShape{
	{reason: "CAPACITY_INVALID_QUOTE"},
	{reason: "CAPACITY_BYTES", metrics: []wireField{
		{name: "unit", rule: constant("bytes")},
		{name: "limit", rule: integer(0, 64<<20)},
		{name: "protected", rule: integer(1, 64<<20)},
	}},
	{reason: "CAPACITY_WORK", metrics: []wireField{
		{name: "unit", rule: constant("work_units")},
		{name: "limit", rule: constant(uint64(1 << 32))},
		{name: "protected", rule: wireRule{kind: "integer", minimum: 1, maximum: 32 * (64 << 20), multiple: 32}},
	}},
	{reason: "CAPACITY_PACKETS", metrics: []wireField{
		{name: "unit", rule: constant("packets")},
		{name: "limit", rule: constant(uint64(64))},
		{name: "spent", rule: constant(uint64(0))},
		{name: "protected", rule: integer(1, 423)},
	}},
}

var capacityShape = func() wireRule {
	reasons, units := []any{}, []any{}
	for _, branch := range capacityReasons {
		reasons = append(reasons, branch.reason)
		for _, field := range branch.metrics {
			if field.name == "unit" {
				units = append(units, field.rule.values...)
			}
		}
	}
	return wireRule{kind: "object", object: []wireField{
		{name: "schema_version", rule: constant(CapacityDiagnosticSchemaVersion)},
		{name: "operation", rule: constant("start")},
		{name: "reason", rule: wireRule{values: reasons}},
		{name: "action", rule: constant("STOP")},
		{name: "retry", rule: constant(false)},
		{name: "metrics", optional: true, rule: wireRule{kind: "object", object: []wireField{
			{name: "unit", rule: wireRule{values: units}},
			{name: "spent", rule: integer(0, 1<<32)},
			{name: "protected", rule: integer(1, 32*(64<<20))},
			{name: "limit", rule: integer(0, 1<<32)},
			{name: "available", rule: integer(0, 1<<32)},
			{name: "deficit", rule: integer(1, 32*(64<<20))},
		}}},
	}}
}()

// check defines a deliberately small, disjoint grammar: homogeneous nonempty
// scalar values (string/bool/uint64), an integer range, or a nonempty closed
// object. Values never combine with kind/range/object; zero multiple means none.
// Active field addresses detect cycles through slice backing arrays while
// allowing shared acyclic subtrees. No descriptor is interpreted before checking.
func (r wireRule) check(active map[*wireField]bool) error {
	bad := fmt.Errorf("invalid capacity wire descriptor: kind %q", r.kind)
	if r.values != nil {
		if len(r.values) == 0 || r.kind != "" || r.minimum != 0 || r.maximum != 0 || r.multiple != 0 || r.object != nil {
			return bad
		}
		seen := map[any]bool{}
		kind := ""
		for _, value := range r.values {
			var k string
			switch value.(type) {
			case string:
				if !utf8.ValidString(value.(string)) {
					return bad
				}
				k = "string"
			case bool:
				k = "bool"
			case uint64:
				k = "uint64"
			default:
				return bad
			}
			if (kind != "" && kind != k) || seen[value] {
				return bad
			}
			kind, seen[value] = k, true
		}
		return nil
	}
	switch r.kind {
	case "integer":
		if r.object != nil || r.minimum > r.maximum {
			return bad
		}
	case "object":
		if len(r.object) == 0 || r.minimum != 0 || r.maximum != 0 || r.multiple != 0 {
			return bad
		}
		seen := map[string]bool{}
		for i := range r.object {
			field := &r.object[i]
			if field.name == "" || !utf8.ValidString(field.name) || seen[field.name] || active[field] {
				return bad
			}
			seen[field.name], active[field] = true, true
			err := field.rule.check(active)
			delete(active, field)
			if err != nil {
				return fmt.Errorf("field %q: %w", field.name, err)
			}
		}
	default:
		return bad
	}
	return nil
}

// checkedCapacityShape is the common admission/publication gate. Check every
// branch, including those not selected by the input, before either interpreter.
// Descriptors are package-private static data; do not mutate them concurrently.
func checkedCapacityShape() (wireRule, error) {
	r := capacityShape
	if err := r.check(map[*wireField]bool{}); err != nil {
		return wireRule{}, err
	}
	bad := fmt.Errorf("invalid capacity reason descriptor")
	if r.kind != "object" || len(capacityReasons) == 0 {
		return wireRule{}, bad
	}
	fields := map[string]wireField{}
	for _, field := range r.object {
		fields[field.name] = field
	}
	reason, metrics := fields["reason"], fields["metrics"]
	if reason.optional || len(reason.rule.values) != len(capacityReasons) || !metrics.optional || metrics.rule.kind != "object" {
		return wireRule{}, bad
	}
	base := map[string]wireField{}
	for _, field := range metrics.rule.object {
		base[field.name] = field
	}
	seen := map[string]bool{}
	for _, branch := range capacityReasons {
		if branch.reason == "" || seen[branch.reason] {
			return wireRule{}, bad
		}
		seen[branch.reason] = true
		found := false
		for _, value := range reason.rule.values {
			if value == branch.reason {
				found = true
			}
		}
		if !found {
			return wireRule{}, bad
		}
		if branch.metrics == nil {
			continue
		}
		if err := (wireRule{kind: "object", object: branch.metrics}).check(map[*wireField]bool{}); err != nil {
			return wireRule{}, err
		}
		for _, field := range branch.metrics {
			parent, found := base[field.name]
			// Branch schemas refine existing required scalar fields; they do
			// not introduce names, optionality, or nested object semantics.
			if !found || parent.optional || field.optional || field.rule.kind == "object" || parent.rule.kind == "object" {
				return wireRule{}, bad
			}
			if len(field.rule.values) != 0 {
				for _, value := range field.rule.values {
					raw, err := json.Marshal(value)
					if err != nil || !parent.rule.validUnchecked(raw) {
						return wireRule{}, bad
					}
				}
			} else if parent.rule.kind != "integer" || field.rule.minimum < parent.rule.minimum || field.rule.maximum > parent.rule.maximum || (parent.rule.multiple != 0 && field.rule.multiple%parent.rule.multiple != 0) || (parent.rule.multiple != 0 && field.rule.multiple == 0) {
				return wireRule{}, bad
			}
		}
	}
	return r, nil
}

func (r wireRule) valid(raw json.RawMessage) bool {
	return r.check(map[*wireField]bool{}) == nil && r.validUnchecked(raw)
}

func (r wireRule) validUnchecked(raw json.RawMessage) bool {
	if len(r.values) != 0 {
		for _, value := range r.values {
			// Typed decoding preserves the existing uint64 wire spelling and
			// rejects null rather than allowing its Go zero value.
			var actual any
			if json.Unmarshal(raw, &actual) != nil || actual == nil {
				return false
			}
			switch v := value.(type) {
			case uint64:
				var n uint64
				if json.Unmarshal(raw, &n) == nil && n == v {
					return true
				}
			default:
				if actual == v {
					return true
				}
			}
		}
		return false
	}
	switch r.kind {
	case "integer":
		var n *uint64
		return json.Unmarshal(raw, &n) == nil && n != nil && *n >= r.minimum && *n <= r.maximum && (r.multiple == 0 || *n%r.multiple == 0)
	case "object":
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return false
		}
		for _, field := range r.object {
			value, found := fields[field.name]
			if !found {
				if field.optional {
					continue
				}
				return false
			}
			if !field.rule.validUnchecked(value) {
				return false
			}
			delete(fields, field.name)
		}
		return len(fields) == 0
	}
	return false
}

func validCapacityShape(raw []byte) bool {
	shape, err := checkedCapacityShape()
	if err != nil || !shape.validUnchecked(raw) {
		return false
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields) // already admitted above
	var reason string
	_ = json.Unmarshal(fields["reason"], &reason)
	for _, branch := range capacityReasons {
		if branch.reason != reason {
			continue
		}
		metrics, present := fields["metrics"]
		if branch.metrics == nil {
			return !present
		}
		if !present {
			return false
		}
		var values map[string]json.RawMessage
		_ = json.Unmarshal(metrics, &values)
		for _, field := range branch.metrics {
			if !field.rule.validUnchecked(values[field.name]) {
				return false
			}
		}
		return true
	}
	return false
}

func (r wireRule) schema() (map[string]any, error) {
	if err := r.check(map[*wireField]bool{}); err != nil {
		return nil, err
	}
	return r.schemaUnchecked(), nil
}

func (r wireRule) schemaUnchecked() map[string]any {
	s := map[string]any{}
	if len(r.values) == 1 {
		s["const"] = r.values[0]
	} else if len(r.values) != 0 {
		s["enum"] = r.values
	}
	if r.kind != "" {
		s["type"] = r.kind
	}
	if r.kind == "integer" {
		s["minimum"], s["maximum"] = r.minimum, r.maximum
		if r.multiple != 0 {
			s["multipleOf"] = r.multiple
		}
	}
	if r.kind == "object" {
		properties, required := map[string]any{}, []string{}
		for _, field := range r.object {
			properties[field.name] = field.rule.schemaUnchecked()
			if !field.optional {
				required = append(required, field.name)
			}
		}
		s["properties"], s["required"], s["additionalProperties"] = properties, required, false
	}
	return s
}

// CapacityDiagnosticSchema returns deterministic publication bytes. It requires
// no runtime compiler, network, or source reads. The committed schema is generated
// from this same descriptor and pinned against these bytes by contract tests.
// Malformed descriptors return an error and no bytes; callers must not publish.
func CapacityDiagnosticSchema() ([]byte, error) {
	shape, err := checkedCapacityShape()
	if err != nil {
		return nil, err
	}
	s := shape.schemaUnchecked()
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = "https://jaresty.github.io/lsp-trace/schemas/" + CapacityDiagnosticSchemaVersion + ".schema.json"
	s["title"] = "Closed Start capacity refusal (not an MCP envelope)"
	s["$comment"] = "Generated from internal/capacitydiagnostic/shape.go; do not edit. Structural schema only. Full contract requires composed schema.ValidateFor or censusresult.DecodeCapacityDiagnostic: reject duplicate decoded keys/trailing JSON and enforce available=limit-spent, protected>available, deficit=protected-available. Preparation effects precede refusal; no durable head is published. No retry or acquisition permission."
	branches := []any{}
	for _, branch := range capacityReasons {
		properties := map[string]any{"reason": constant(branch.reason).schemaUnchecked()}
		item := map[string]any{"properties": properties}
		if branch.metrics == nil {
			item["not"] = map[string]any{"required": []string{"metrics"}}
		} else {
			constraints := map[string]any{}
			for _, field := range branch.metrics {
				constraints[field.name] = field.rule.schemaUnchecked()
			}
			properties["metrics"] = map[string]any{"properties": constraints}
			item["required"] = []string{"metrics"}
		}
		branches = append(branches, item)
	}
	s["oneOf"] = branches
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("capacity diagnostic schema: %w", err)
	}
	return append(raw, '\n'), nil
}
