package seedbinding

import (
	"fmt"
	"strings"
	"testing"
)

func TestDocumentSymbolSemanticAdmissionMatrix(t *testing.T) {
	_, manifest := seedFixture(t)
	manifest.ExpectedDeclarationRange = Range{StartLine: 0, StartCharacter: 30, EndLine: 0, EndCharacter: 57}
	r := `{"start":{"line":0,"character":30},"end":{"line":0,"character":57}}`
	s := `{"start":{"line":0,"character":35},"end":{"line":0,"character":55}}`
	hier := fmt.Sprintf(`{"name":"GetSchoolFilterModel","kind":6,"range":%s,"selectionRange":%s}`, r, s)
	flat := fmt.Sprintf(`{"name":"GetSchoolFilterModel","kind":6,"location":{"uri":%q,"range":%s}}`, manifest.Locator.URI, r)
	cases := []struct {
		name, raw string
		want      Status
		detail    string
	}{
		{"hierarchical", `[{"name":"VariableApiController","kind":5,"range":` + r + `,"selectionRange":` + s + `,"children":[` + hier + `]}]`, Match, "hierarchical"},
		{"flat", `[` + flat + `]`, Match, "distinct name range unavailable"},
		{"wrong-name", `[` + strings.Replace(hier, "GetSchoolFilterModel", "Other", 1) + `]`, Mismatch, "matches=0"},
		{"wrong-range", `[` + strings.Replace(hier, `"character":57`, `"character":56`, 1) + `]`, Mismatch, "matches=0"},
		{"ambiguous", `[` + hier + `,` + hier + `]`, Mismatch, "matches=2"},
		{"mixed", `[` + hier + `,` + flat + `]`, Invalid, "mixed"},
		{"foreign-uri", `[` + strings.Replace(flat, manifest.Locator.URI, "file:///foreign/VariableApiController.cs", 1) + `]`, Mismatch, "matches=0"},
	}
	source := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateDocumentSymbols([]byte(tc.raw), manifest, source, "utf-8")
			if got.Status != tc.want || !strings.Contains(got.PrivateDetail, tc.detail) {
				t.Fatalf("ASSERT_SEMANTIC_%s: %+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got)
			}
		})
	}
}
