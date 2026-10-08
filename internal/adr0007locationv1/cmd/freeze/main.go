package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const root = "docs/pilot/adr0007/experiment/location-intersection-prospective-v1"

var cases = []struct {
	name, out                     string
	input, eligible, ranked, work int
}{
	{"01-exact-file-intersects", "COMPLETE", 1, 1, 1, 3}, {"02-exact-file-ineligible", "COMPLETE", 1, 0, 0, 3}, {"03-range-contained-by", "COMPLETE", 1, 1, 1, 4}, {"04-range-contains", "COMPLETE", 1, 1, 1, 4}, {"05-range-partial-intersection", "COMPLETE", 1, 1, 1, 4}, {"06-adjacent-not-intersection", "COMPLETE", 1, 0, 0, 4}, {"07-range-union-multiple", "COMPLETE", 2, 2, 2, 7}, {"08-prefix-frozen-expansion", "COMPLETE", 2, 2, 2, 6}, {"09-invalid-selector-union", "INVALID_SELECTOR", 0, 0, 0, 0}, {"10-invalid-canonical-path", "INVALID_SELECTOR", 0, 0, 0, 0}, {"11-invalid-empty-range", "INVALID_RANGE", 0, 0, 0, 0}, {"12-stale-admission-binding", "SOURCE_ADMISSION_MISMATCH", 0, 0, 0, 0}, {"13-admission-unavailable", "SOURCE_ADMISSION_UNAVAILABLE", 0, 0, 0, 0}, {"14-unavailable-member", "COMPLETE", 1, 0, 0, 3}, {"15-duplicate-member", "COMPLETE", 2, 1, 1, 4}, {"16-invalid-member-location", "COMPLETE", 1, 0, 0, 3}, {"17-policy-filtered-member", "COMPLETE", 1, 0, 0, 3}, {"18-denominator-exceeds-topk", "COMPLETE", 3, 3, 1, 5}, {"19-member-limit", "RESOURCE_LIMIT", 0, 0, 0, 0}, {"20-prefix-expansion-limit", "RESOURCE_LIMIT", 0, 0, 0, 0}, {"21-witness-limit", "RESOURCE_LIMIT", 0, 0, 0, 0}, {"22-exact-work-boundary", "COMPLETE", 1, 1, 1, 4}, {"23-work-plus-one", "RESOURCE_LIMIT", 0, 0, 0, 0}, {"24-cancel-deadline-precommit", "CANCELLED", 0, 0, 0, 0}}

func main() {
	must(os.MkdirAll(filepath.Join(root, "schemas"), 0755))
	docs := map[string]string{"SOURCE_ADMISSION_BINDING.md": "# SOURCE ADMISSION BINDING\n\nRequests bind the exact admission digest and every selected source path, revision, file digest, and object digest. Prefix selectors carry a frozen explicit expansion.\n", "POLICY.md": "# POLICY\n\nOnly canonical relative NFC UTF-8 paths and nonempty UTF-16 half-open ranges are admitted. Ranked output is a projection; all ordinals remain in terminal accounting.\n", "LIMITS.md": "# LIMITS\n\nFinite positive maxima govern request bytes, members, paths, ranges, prefix expansion, witnesses, source bytes, output bytes, and work. Work is deterministically precharged before evaluation.\n", "REVIEW_POLICY.md": "# REVIEW POLICY\n\nReview booleans cover schema validity, expected outcome, complete accounting, concrete witnesses, and exact binding. The adapter recomputes the conjunction and ignores a supplied verdict.\n", "PRE_EXECUTION_AUDIT.md": "# PRE-EXECUTION AUDIT\n\nPending independent audit. DispatchAllowed=false; LocationExecuted=false; LocationDesignGO=false. No fixture is execution evidence.\n", "AUTHORIZATION_CANDIDATE.json": "{\"DispatchAllowed\":false,\"LocationDesignGO\":false,\"LocationExecuted\":false,\"schema\":\"lsp-trace.adr0007.location.authorization-candidate.v1\",\"state\":\"DESIGN_FROZEN_NON_DISPATCHING\"}\n"}
	for n, b := range docs {
		must(os.WriteFile(filepath.Join(root, n), []byte(b), 0644))
	}
	schema := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false, "required": []string{"schema", "case", "expectedOutcome", "accounting"}, "properties": map[string]any{"schema": map[string]any{"type": "string", "const": "lsp-trace.adr0007.location.case.v1"}, "case": map[string]any{"type": "string", "minLength": 1}, "expectedOutcome": map[string]any{"type": "string"}, "accounting": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"input", "eligible", "ranked", "work"}, "properties": map[string]any{"input": map[string]any{"type": "integer", "minimum": 0}, "eligible": map[string]any{"type": "integer", "minimum": 0}, "ranked": map[string]any{"type": "integer", "minimum": 0}, "work": map[string]any{"type": "integer", "minimum": 0}}}}}
	writeJSON(filepath.Join(root, "schemas", "Case.schema.json"), schema)
	for i, c := range cases {
		d := filepath.Join(root, "cases", c.name)
		must(os.MkdirAll(d, 0755))
		v := map[string]any{"schema": "lsp-trace.adr0007.location.case.v1", "case": fmt.Sprintf("%02d", i+1), "expectedOutcome": c.out, "accounting": map[string]int{"input": c.input, "eligible": c.eligible, "ranked": c.ranked, "work": c.work}}
		writeJSON(filepath.Join(d, "EXPECTED.json"), v)
		must(os.WriteFile(filepath.Join(d, "DESIGN_FIXTURE"), []byte("prospective; not executed\n"), 0644))
	}
	freeze()
}
func freeze() {
	var files []string
	filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() && !strings.HasSuffix(p, "FREEZE.json") {
			files = append(files, p)
		}
		return nil
	})
	filepath.Walk("internal/adr0007locationv1", func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	filepath.Walk("internal/sourceadmissionv1", func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !i.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	type item struct {
		Path, SHA256 string
		Bytes        int
	}
	items := []item{}
	for _, p := range files {
		b := mustRead(p)
		s := sha256.Sum256(b)
		items = append(items, item{p, "sha256:" + hex.EncodeToString(s[:]), len(b)})
	}
	writeJSON(filepath.Join(root, "FREEZE.json"), map[string]any{"schema": "lsp-trace.adr0007.location.freeze.v1", "state": "DESIGN_FROZEN_NON_DISPATCHING", "DispatchAllowed": false, "LocationExecuted": false, "LocationDesignGO": false, "files": items})
}
func writeJSON(p string, v any) {
	b, e := json.Marshal(v)
	must(e)
	must(os.WriteFile(p, append(b, '\n'), 0644))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
func mustRead(p string) []byte { b, e := os.ReadFile(p); must(e); return b }
