package main

import (
	"encoding/json"
	"fmt"
	sts "lsp-trace/internal/adr0007sourcetextsearchv1"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: adr0007-source-text-search-designgen OUTDIR")
		os.Exit(2)
	}
	root := os.Args[1]
	must(os.MkdirAll(filepath.Join(root, "schemas"), 0755))
	for name, body := range docs {
		write(filepath.Join(root, name), body)
	}
	schemas := map[string]any{
		"source-text-search-input-v1.schema.json":   schema(sts.SchemaInputV1, []string{"schema_version", "query", "source", "limits"}),
		"source-text-search-result-v1.schema.json":  schema(sts.SchemaResultV1, []string{"schema_version", "accepted", "authority", "completeness", "feature_identity", "matches", "accounting"}),
		"source-text-search-failure-v1.schema.json": schema(sts.SchemaFailureV1, []string{"schema_version", "accepted", "authority", "completeness", "feature_identity", "code", "message", "accounting"}),
		"location-composition-v1.schema.json":       schema(sts.SchemaLocationV1, []string{"schema_version", "executed_location", "ranges", "source"}),
		"source-admission-binding-v1.schema.json":   schema(sts.SchemaAdmissionV1, []string{"path", "revision", "file_sha256", "object_id", "admission_id", "seal"}),
		"freeze-v1.schema.json":                     schema(sts.SchemaFreezeV1, []string{"schema_version", "root_sha256", "files"}),
	}
	for name, v := range schemas {
		b, _ := json.MarshalIndent(v, "", "  ")
		write(filepath.Join(root, "schemas", name), string(b)+"\n")
	}
	writeFreeze(root)
	writeFreeze(root)
}

func schema(id string, req []string) map[string]any {
	props := map[string]any{}
	for _, k := range req {
		props[k] = map[string]any{"type": []string{"string", "number", "boolean", "object", "array"}}
	}
	return map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": id, "type": "object", "additionalProperties": false, "required": req, "properties": props}
}
func write(p, s string) { must(os.WriteFile(p, []byte(s), 0644)) }
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func writeFreeze(root string) {
	f, err := sts.Census(root)
	must(err)
	b, _ := json.MarshalIndent(f, "", "  ")
	write(filepath.Join(root, "FREEZE.json"), string(b)+"\n")
}

var docs = map[string]string{
	"DESIGN.md":                   "# Source Text Search Prospective v1 Design Candidate\n\nImmutable private ADR0007 candidate. Normative scope is exactly nonempty case-sensitive UTF-8 literal search over admitted source text. The result reports overlapping matches with byte and UTF-16 half-open ranges, exact path/revision/file/object/admission binding, deterministic path-byte/offset ordering, explicit limits/work/output accounting, and strict fail-closed behavior. It makes authority0 acceptedfalse completenessUNKNOWN featureIdentityUNRESOLVED claims only. No regex, fuzzy matching, tokenization, ranking, inference, or feature identity is in scope.\n",
	"ALGORITHM.md":                "# Algorithm\n\n1. Validate schema_version, nonempty valid UTF-8 query, explicit positive work/output limits, and nonnegative caps.\n2. Independently hash source bytes and require equality with the bound file_sha256 before scanning.\n3. Scan every byte offset in deterministic path-byte then offset order; compare query bytes literally and case-sensitively; advance by one byte to preserve overlaps.\n4. Convert byte half-open endpoints to UTF-16 half-open endpoints by decoding UTF-8 before each endpoint; non-BMP code points count as two UTF-16 units.\n5. Precharge work as W=50+3J+5Q+7P+S+11T+13M+17R+19U+31B. Equality to max passes; max+1 fails.\n6. On any failed check emit failure with empty matches and zero counters.\n",
	"POLICY.md":                   "# Policy\n\nFail closed on invalid input, source binding mismatch, object/admission mismatch, over-limit work, over-limit output, path escape, symlink, or any unsupported operation. Failures carry accepted=false, authority=0, completeness=UNKNOWN, feature_identity=UNRESOLVED and zero accounting.\n",
	"LIMITS.md":                   "# Limits\n\nLimits are caller-provided and checked before and after deterministic serialization. Work formula symbols: J files, Q query bytes, P path bytes, S source bytes, T scanned source tuples, M matches, R range units, U UTF-16 units, B output bytes. Equality max passes and max+1 fails.\n",
	"SOURCE_ADMISSION_BINDING.md": "# Source Admission Binding\n\nThe candidate binds path, revision, file_sha256, object_id, admission_id, and seal. Source-admission v2 implementation is pinned only after independently verifying actual bytes and identity from the Location design branch/history; stop on mismatch.\n",
	"LOCATION_BINDING.md":         "# Location Binding\n\nVerified Location pins: design c943a484/root sha1195..., successor f0f8b49a, seal commit16f40dcb, final seal sha f498.... Composition is candidate generation only and never executes Location. Only Location v5 RANGE_UNION composition is allowed; overlap ranges and the same source tuples plus qualified seal are preserved.\n",
	"PREDECESSORS.md":             "# Predecessors\n\nBlocked predecessor and failed dispatch are immutable and non-normative. They may be cited as historical context but never as normative source text search behavior.\n",
	"SPEC_REVIEW_PENDING.md":      "# SPEC_REVIEW_PENDING\n\nThis candidate is not self-approved. No design verdict or execution authorization is issued by this package.\n",
	"REVIEW_ASSIGNMENT.md":        "# REVIEW_ASSIGNMENT\n\nIndependent reviewers must verify schema strictness, source-admission binding, Location candidate-only composition, deterministic freeze identity, and the discriminating input-only cases before any acceptance decision.\n",
	"TEST_PLAN.md":                "# Input-only discriminating cases\n\n1 empty query fail\n2 ASCII exact one\n3 ASCII no match\n4 case-sensitive miss\n5 overlapping aba in ababa\n6 adjacent aa in aaa\n7 query at byte 0\n8 query at EOF\n9 CRLF offset\n10 LF offset\n11 bare CR offset\n12 non-BMP before match\n13 non-BMP inside query\n14 combining mark literal\n15 invalid UTF-8 query fail\n16 source digest mismatch fail\n17 path binding mismatch fail\n18 revision absent fail\n19 object absent fail\n20 admission absent fail\n21 seal absent fail\n22 max matches equality pass\n23 max matches plus one fail\n24 max work equality pass\n25 max work plus one fail\n26 max output equality pass\n27 max output plus one fail\n28 deterministic path-byte order\n29 no ranking field\n30 regex metachar literal\n31 fuzzy near miss rejected\n32 token boundary ignored\n33 duplicate source tuple preserved\n34 Location RANGE_UNION preserves overlaps\n35 failure counters zero\n36 symlink rejected\n37 path traversal rejected\n38 freeze deterministic second generation equality\n",
}
