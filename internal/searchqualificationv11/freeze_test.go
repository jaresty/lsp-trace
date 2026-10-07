package searchqualificationv11

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateRepositoryFreeze(t *testing.T) {
	root := os.Getenv("SEARCH_V11_FREEZE_ROOT")
	if root == "" {
		t.Skip("set SEARCH_V11_FREEZE_ROOT for explicit design-freeze generation")
	}
	var generated string
	var err error
	if source := os.Getenv("SEARCH_V11_DESCRIBE_SOURCE"); source != "" {
		generated, err = GenerateArtifacts(root, source, ".")
	} else {
		generated, err = GenerateFreeze(root)
	}
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyFreeze(root)
	if err != nil || generated != verified {
		t.Fatalf("generated=%s verified=%s err=%v", generated, verified, err)
	}
	if err = VerifyArtifacts(root); err != nil {
		t.Fatal(err)
	}
	t.Log("FREEZE_VERIFIED " + verified)
}

func TestFreezeGeneratorVerifierTwice(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "DESIGN.md"), []byte("frozen\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a, e := GenerateFreeze(d)
	if e != nil {
		t.Fatal(e)
	}
	av, e := VerifyFreeze(d)
	if e != nil || av != a {
		t.Fatalf("verify1 %v", e)
	}
	b, e := GenerateFreeze(d)
	if e != nil {
		t.Fatal(e)
	}
	bv, e := VerifyFreeze(d)
	if e != nil || b != a || bv != a {
		t.Fatalf("unstable %s %s %v", a, b, e)
	}
}
func TestCoreHasNoPiDependency(t *testing.T) {
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range entries {
		if x.IsDir() || !strings.HasSuffix(x.Name(), ".go") || strings.HasSuffix(x.Name(), "_test.go") {
			continue
		}
		b, e := os.ReadFile(x.Name())
		if e != nil {
			t.Fatal(e)
		}
		s := strings.ToLower(string(b))
		for _, forbidden := range []string{"pi-coding", "openai", "anthropic", "model.invoke", "exec.command", "os/exec"} {
			if strings.Contains(s, forbidden) {
				t.Fatalf("%s contains %q", x.Name(), forbidden)
			}
		}
	}
}
func TestNonPiAdapterFixture(t *testing.T) {
	r := fixtureRequest(t)
	raw := hostRaw(r, "UNAVAILABLE", 0)
	if _, e := ParseHostResult(raw, r, "assignment-1", Digest(raw)); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.ToLower(string(raw)), "pi") {
		t.Fatal("fixture names Pi")
	}
}
func generateFixtureTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tree")
	source := filepath.Join("..", "..", "docs", "pilot", "adr0007", "experiment", "describe-custody-successor-2026-10-07")
	if _, err := GenerateArtifacts(root, source, "."); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifacts(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestArtifactGeneratorVerifierTwiceStable(t *testing.T) {
	root := generateFixtureTree(t)
	first, err := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("..", "..", "docs", "pilot", "adr0007", "experiment", "describe-custody-successor-2026-10-07")
	if _, err = GenerateArtifacts(root, source, "."); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, "FREEZE.json"))
	if err != nil || string(first) != string(second) {
		t.Fatal("unstable generation")
	}
	if err = VerifyArtifacts(root); err != nil {
		t.Fatal(err)
	}
}

func mutateFixtureJSON(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	b, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAttemptRawBytesCustodyCondition(t *testing.T) {
	writeAttempt := func(t *testing.T, name string, value map[string]any) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	producer := func(size int, states []string, reason, resultID string) map[string]any {
		return map[string]any{
			"RawBytes":       base64.StdEncoding.EncodeToString(make([]byte, size)),
			"States":         states,
			"TerminalReason": reason,
			"ResultID":       resultID,
		}
	}
	reviewer := func(size int, states []string, reason, reviewID string) map[string]any {
		return map[string]any{
			"RawBytes":       base64.StdEncoding.EncodeToString(make([]byte, size)),
			"States":         states,
			"TerminalReason": reason,
			"ReviewID":       reviewID,
		}
	}

	for name, tc := range map[string]struct {
		file  string
		value map[string]any
		valid bool
	}{
		"producer_normal_boundary":           {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes, []string{"RECEIVED", "ADAPTED", "COMMITTED"}, "", "result-1"), true},
		"producer_raw_size_plus_one":         {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+1, []string{"RECEIVED", "TERMINAL_INVALID"}, "RAW_SIZE", ""), true},
		"producer_raw_size_plus_two":         {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+2, []string{"RECEIVED", "TERMINAL_INVALID"}, "RAW_SIZE", ""), false},
		"producer_plus_one_wrong_reason":     {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+1, []string{"RECEIVED", "TERMINAL_INVALID"}, "FRAME", ""), false},
		"producer_plus_one_wrong_state":      {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+1, []string{"RECEIVED", "ADAPTED", "COMMITTED"}, "RAW_SIZE", ""), false},
		"producer_plus_one_committed":        {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+1, []string{"RECEIVED", "ADAPTED", "COMMITTED"}, "", "result-1"), false},
		"producer_plus_one_with_result":      {"PRODUCER_ATTEMPT.json", producer(MaxResponseBytes+1, []string{"RECEIVED", "TERMINAL_INVALID"}, "RAW_SIZE", "result-1"), false},
		"reviewer_normal_boundary":           {"REVIEW_ATTEMPT.json", reviewer(MaxResponseBytes, []string{"RECEIVED", "ADAPTED", "COMMITTED"}, "", "review-1"), true},
		"reviewer_plus_one_terminal_invalid": {"REVIEW_ATTEMPT.json", reviewer(MaxResponseBytes+1, []string{"RECEIVED", "TERMINAL_INVALID"}, "RAW_SIZE", ""), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := verifyAttemptFileRawBytes(writeAttempt(t, tc.file, tc.value))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestArtifactMutationGuards(t *testing.T) {
	mutations := map[string]func(*testing.T, string){
		"review_request": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "REVIEW_REQUEST.json"), func(v map[string]any) { v["RequestDigest"] = Digest([]byte("mutated")) })
		},
		"review_raw": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "REVIEW_RAW"), func(v map[string]any) {
				v["checks"].(map[string]any)["semantic_complete"] = false
				v["reasons"] = map[string]any{"semantic_complete": "mutated"}
			})
		},
		"account_selection": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "ACCOUNT.json"), func(v map[string]any) { v["Selection"].(map[string]any)["ResultID"] = "mutated" })
		},
		"expected_ledger": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "EXPECTED.json"), func(v map[string]any) { v["Ledger"].([]any)[0].(map[string]any)["Outcome"] = "BELOW_THRESHOLD" })
		},
		"producer_invalid_base64": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "PRODUCER_ATTEMPT.json"), func(v map[string]any) { v["RawBytes"] = "!!!" })
		},
		"producer_bad_padding": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "PRODUCER_ATTEMPT.json"), func(v map[string]any) { v["RawBytes"] = "YQ=" })
		},
		"producer_decoded_plus_one": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "PRODUCER_ATTEMPT.json"), func(v map[string]any) {
				v["RawBytes"] = base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes+1))
			})
		},
		"reviewer_invalid_base64": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "REVIEW_ATTEMPT.json"), func(v map[string]any) { v["RawBytes"] = "!!!" })
		},
		"reviewer_bad_padding": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "REVIEW_ATTEMPT.json"), func(v map[string]any) { v["RawBytes"] = "YQ=" })
		},
		"reviewer_decoded_plus_one": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "REVIEW_ATTEMPT.json"), func(v map[string]any) {
				v["RawBytes"] = base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes+1))
			})
		},
		"custody_ascii_identity_257": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "CUSTODY.json"), func(v map[string]any) { v["ReviewerTaskID"] = strings.Repeat("x", MaxCustodyIDBytes+1) })
		},
		"custody_multibyte_identity_over_256": func(t *testing.T, root string) {
			mutateFixtureJSON(t, filepath.Join(root, "cases", "01-complete-top5", "CUSTODY.json"), func(v map[string]any) { v["ReviewerTaskID"] = strings.Repeat("é", MaxCustodyIDBytes/2+1) })
		},

		"fixture_drop": func(t *testing.T, root string) { os.Remove(filepath.Join(root, "cases", "01-complete-top5", "RAW")) },
		"fixture_substitute": func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "cases", "01-complete-top5", "RAW"), []byte("substitute"), 0644)
		},
		"schema_substitution": func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "schemas", "SearchRequest.schema.json"), []byte("{}\n"), 0644)
		},
		"policy_substitution": func(t *testing.T, root string) {
			os.WriteFile(filepath.Join(root, "POLICY.json"), []byte("{}\n"), 0644)
		},
		"extra_file":   func(t *testing.T, root string) { os.WriteFile(filepath.Join(root, "EXTRA"), []byte("x"), 0644) },
		"stale_freeze": func(t *testing.T, root string) { os.WriteFile(filepath.Join(root, "DESIGN.md"), []byte("stale"), 0644) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root := generateFixtureTree(t)
			mutate(t, root)
			switch name {
			case "review_request", "review_raw", "account_selection", "expected_ledger", "producer_invalid_base64", "producer_bad_padding", "producer_decoded_plus_one", "reviewer_invalid_base64", "reviewer_bad_padding", "reviewer_decoded_plus_one", "custody_ascii_identity_257", "custody_multibyte_identity_over_256":
				if _, err := GenerateFreezeWithSources(root, "."); err != nil {
					t.Fatal(err)
				}
			}
			if VerifyArtifacts(root) == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
}

func TestDescribeSourceMutationRejected(t *testing.T) {
	source := filepath.Join("..", "..", "docs", "pilot", "adr0007", "experiment", "describe-custody-successor-2026-10-07")
	tmp := t.TempDir()
	for _, name := range []string{"producer-committed-attempts.json", "producer-request-records.json"} {
		b, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(tmp, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(tmp, "producer-request-records.json"))
	b = append(b[:len(b)-2], []byte(",{}]\n")...)
	os.WriteFile(filepath.Join(tmp, "producer-request-records.json"), b, 0644)
	if _, err := GenerateArtifacts(filepath.Join(t.TempDir(), "tree"), tmp, "."); err == nil {
		t.Fatal("mutated source accepted")
	}
}

func TestFrozenWorkBoundaryAndOperationCausality(t *testing.T) {
	boundary := AdmissionState{RequestValid: true, AssignmentValid: true, DigestValid: true, RawBytes: 1, UTF8: true, Frame: true, StrictJSON: true, Frontier: 24, Evaluations: 23, Work: 230, NextWork: 10}
	if got := ClassifyAdmission(boundary); got != "ADMIT" {
		t.Fatalf("boundary got %s", got)
	}
	plusOne := boundary
	plusOne.Work = 231
	if got := ClassifyAdmission(plusOne); got != "WORK_PRECHARGE" {
		t.Fatalf("plus-one got %s", got)
	}

	root := generateFixtureTree(t)
	want := map[string]string{"05-mismatch": "INDEX_MISMATCH", "13-response-boundary-65536": "BACKEND_FAILURE", "14-response-plus-one": "RESOURCE_LIMIT", "17-malformed-score": "BACKEND_FAILURE", "18-unknown-member": "BACKEND_FAILURE", "22-invalid-utf8": "BACKEND_FAILURE", "23-partial-frame": "BACKEND_FAILURE", "24-duplicate-member": "BACKEND_FAILURE"}
	entries, err := os.ReadDir(filepath.Join(root, "cases"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		var exp expectedFixture
		if err = readStrict(filepath.Join(root, "cases", entry.Name(), "EXPECTED.json"), &exp); err != nil {
			t.Fatal(err)
		}
		if !validOperationOutcome(exp.OperationOutcome) {
			t.Errorf("%s non-enum operation %q", entry.Name(), exp.OperationOutcome)
		}
		if exp.Cause != "" && exp.OperationOutcome != operationForCause(exp.Cause) {
			t.Errorf("%s cause=%s operation=%s", entry.Name(), exp.Cause, exp.OperationOutcome)
		}
		if expected, ok := want[entry.Name()]; ok && exp.OperationOutcome != expected {
			t.Errorf("%s got %s want %s", entry.Name(), exp.OperationOutcome, expected)
		}
	}
	var admission expectedFixture
	if err = readStrict(filepath.Join(root, "cases", "12-work-limit-before-24th", "EXPECTED.json"), &admission); err != nil {
		t.Fatal(err)
	}
	if admission.Cause != "WORK_PRECHARGE" || admission.OperationOutcome != "RESOURCE_LIMIT" || admission.Begun != 23 || admission.Unevaluated != 1 || admission.Work != 231 {
		t.Fatalf("admission %+v", admission)
	}

	mutateFixtureJSON(t, filepath.Join(root, "cases", "12-work-limit-before-24th", "EXPECTED.json"), func(v map[string]any) { v["Cause"] = "ADMIT" })
	if _, err = GenerateFreezeWithSources(root, "."); err != nil {
		t.Fatal(err)
	}
	if err = VerifyArtifacts(root); err == nil {
		t.Fatal("ADMIT + RESOURCE_LIMIT accepted")
	}
}

func schemaValidate(schema map[string]any, value any) bool {
	if allOf, ok := schema["allOf"].([]any); ok {
		for _, item := range allOf {
			if !schemaValidate(item.(map[string]any), value) {
				return false
			}
		}
	}
	if condition, ok := schema["if"].(map[string]any); ok {
		branch := schema["else"].(map[string]any)
		if schemaValidate(condition, value) {
			branch = schema["then"].(map[string]any)
		}
		if !schemaValidate(branch, value) {
			return false
		}
	}
	typeName, _ := schema["type"].(string)
	switch typeName {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return false
		}
		props, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]string); ok {
			for _, name := range required {
				if _, ok := obj[name]; !ok {
					return false
				}
			}
		}
		if schema["additionalProperties"] == false {
			for name := range obj {
				if _, ok := props[name]; !ok {
					return false
				}
			}
		}
		for name, child := range props {
			if got, ok := obj[name]; ok && !schemaValidate(child.(map[string]any), got) {
				return false
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return false
		}
		if min, ok := schema["minItems"].(int); ok && len(items) < min {
			return false
		}
		if max, ok := schema["maxItems"].(int); ok && len(items) > max {
			return false
		}
		if unique, _ := schema["uniqueItems"].(bool); unique {
			seen := map[string]bool{}
			for _, item := range items {
				key := string(canonical(item))
				if seen[key] {
					return false
				}
				seen[key] = true
			}
		}
		for _, item := range items {
			if !schemaValidate(schema["items"].(map[string]any), item) {
				return false
			}
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return false
		}
		if min, ok := schema["minLength"].(int); ok && len([]rune(s)) < min {
			return false
		}
		if max, ok := schema["maxLength"].(int); ok && len([]rune(s)) > max {
			return false
		}
		if max, ok := schema["x-maxUtf8Bytes"].(int); ok && len([]byte(s)) > max {
			return false
		}
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(s) {
			return false
		}
		if max, ok := schema["x-maxDecodedBytes"].(int); ok {
			decoded, err := base64.StdEncoding.Strict().DecodeString(s)
			if err != nil || len(decoded) > max {
				return false
			}
		}
		if exact, ok := schema["x-decodedBytesConst"].(int); ok {
			decoded, err := base64.StdEncoding.Strict().DecodeString(s)
			if err != nil || len(decoded) != exact {
				return false
			}
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != float64(int(n)) {
			return false
		}
		if min, ok := schema["minimum"].(int); ok && n < float64(min) {
			return false
		}
		if max, ok := schema["maximum"].(int); ok && n > float64(max) {
			return false
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return false
		}
	}
	if constant, ok := schema["const"]; ok && !reflect.DeepEqual(value, constant) {
		return false
	}
	if values, ok := schema["enum"].([]string); ok {
		found := false
		for _, candidate := range values {
			found = found || value == candidate
		}
		if !found {
			return false
		}
	}
	return true
}

func assertSchemaClosed(t *testing.T, path string, schema map[string]any) {
	t.Helper()
	if schema["type"] == "object" {
		if schema["additionalProperties"] != false {
			t.Fatalf("%s object open", path)
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok || len(props) == 0 {
			t.Fatalf("%s properties", path)
		}
		required, ok := schema["required"].([]string)
		if !ok || len(required) > len(props) {
			t.Fatalf("%s required", path)
		}
		for name, child := range props {
			assertSchemaClosed(t, path+"."+name, child.(map[string]any))
		}
	}
	if schema["type"] == "array" {
		assertSchemaClosed(t, path+"[]", schema["items"].(map[string]any))
	}
}

func TestPublishedSchemasClosedAndRejectMalformed(t *testing.T) {
	root := generateFixtureTree(t)
	schemas := schemaMap()
	for name, schema := range schemas {
		if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" || schema["$id"] != Version+".schema."+name {
			t.Fatalf("%s draft/id", name)
		}
		assertSchemaClosed(t, name, schema)
		if schemaValidate(schema, map[string]any{}) {
			t.Errorf("%s accepted empty", name)
		}
	}
	samples := map[string]string{"SearchRequest": "REQUEST.json", "HostResult": "RAW", "SearchResult": "RESULT.json", "ProducerAttempt": "PRODUCER_ATTEMPT.json", "ReviewRequest": "REVIEW_REQUEST.json", "ReviewerRaw": "REVIEW_RAW", "ReviewerAttempt": "REVIEW_ATTEMPT.json", "Account": "ACCOUNT.json", "ParentCustodyEvidence": "CUSTODY.json"}
	caseDir := filepath.Join(root, "cases", "01-complete-top5")
	for name, file := range samples {
		var sample any
		b, err := os.ReadFile(filepath.Join(caseDir, file))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &sample); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !schemaValidate(schemas[name], sample) {
			t.Errorf("%s rejected valid Go-contract sample", name)
		}
	}
	requestBytes, _ := os.ReadFile(filepath.Join(caseDir, "REQUEST.json"))
	var request map[string]any
	json.Unmarshal(requestBytes, &request)
	mutations := map[string]func(map[string]any){
		"unknown":         func(v map[string]any) { v["unknown"] = true },
		"missing":         func(v map[string]any) { delete(v, "RequestID") },
		"bad_digest":      func(v map[string]any) { v["DescribeDigest"] = "bad" },
		"bad_count":       func(v map[string]any) { v["Denominator"] = "23" },
		"bad_cardinality": func(v map[string]any) { v["OrderedMembers"] = v["OrderedMembers"].([]any)[:23] },
	}
	for name, mutate := range mutations {
		var v map[string]any
		json.Unmarshal(requestBytes, &v)
		mutate(v)
		if schemaValidate(schemas["SearchRequest"], v) {
			t.Errorf("accepted %s", name)
		}
	}
	resultBytes, _ := os.ReadFile(filepath.Join(caseDir, "RESULT.json"))
	var result map[string]any
	json.Unmarshal(resultBytes, &result)
	result["Outcome"] = "TERMINAL_INVALID"
	if schemaValidate(schemas["SearchResult"], result) {
		t.Error("accepted bad enum")
	}
	hostBytes, _ := os.ReadFile(filepath.Join(caseDir, "RAW"))
	var host map[string]any
	json.Unmarshal(hostBytes, &host)
	host["Evaluations"].([]any)[0].(map[string]any)["Score"] = "0.7"
	if schemaValidate(schemas["HostResult"], host) {
		t.Error("accepted bad score")
	}

	producerRawSchema := schemas["ProducerAttempt"]["properties"].(map[string]any)["RawBytes"].(map[string]any)
	producerConditional := schemas["ProducerAttempt"]["allOf"].([]any)[0].(map[string]any)
	producerExactRawSchema := producerConditional["then"].(map[string]any)["properties"].(map[string]any)["RawBytes"].(map[string]any)
	reviewerRawSchema := schemas["ReviewerAttempt"]["properties"].(map[string]any)["RawBytes"].(map[string]any)
	if producerRawSchema["x-maxDecodedBytes"] != MaxResponseBytes+1 || producerRawSchema["description"] != "Canonical base64; decoded length is at most 65536 except the conditional exact 65537-byte RAW_SIZE terminal-invalid branch." || producerExactRawSchema["x-decodedBytesConst"] != MaxResponseBytes+1 || reviewerRawSchema["x-maxDecodedBytes"] != MaxResponseBytes {
		t.Fatal("attempt schema decoded-byte annotations")
	}
	case14Bytes, err := os.ReadFile(filepath.Join(root, "cases", "14-response-plus-one", "PRODUCER_ATTEMPT.json"))
	if err != nil {
		t.Fatal(err)
	}
	var case14 map[string]any
	if err = json.Unmarshal(case14Bytes, &case14); err != nil {
		t.Fatal(err)
	}
	if !schemaValidate(schemas["ProducerAttempt"], case14) {
		t.Fatal("ProducerAttempt schema rejected exact RAW_SIZE plus-one fixture")
	}
	for name, mutate := range map[string]func(map[string]any){
		"plus_two": func(v map[string]any) {
			v["RawBytes"] = base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes+2))
		},
		"wrong_reason": func(v map[string]any) { v["TerminalReason"] = "FRAME" },
		"wrong_state":  func(v map[string]any) { v["States"] = []any{"RECEIVED", "ADAPTED", "COMMITTED"} },
		"with_result":  func(v map[string]any) { v["ResultID"] = "result-1" },
	} {
		t.Run("ProducerAttempt/conditional_"+name, func(t *testing.T) {
			var attempt map[string]any
			if err := json.Unmarshal(case14Bytes, &attempt); err != nil {
				t.Fatal(err)
			}
			mutate(attempt)
			if schemaValidate(schemas["ProducerAttempt"], attempt) {
				t.Fatal("ProducerAttempt conditional accepted invalid oversized attempt")
			}
		})
	}

	for schemaName, fileName := range map[string]string{"ProducerAttempt": "PRODUCER_ATTEMPT.json", "ReviewerAttempt": "REVIEW_ATTEMPT.json"} {
		attemptBytes, err := os.ReadFile(filepath.Join(caseDir, fileName))
		if err != nil {
			t.Fatal(err)
		}
		for name, raw := range map[string]string{
			"invalid_base64":        "!!!",
			"bad_base64_padding":    "YQ=",
			"decoded_plus_one":      base64.StdEncoding.EncodeToString(make([]byte, MaxResponseBytes+1)),
			"encoded_length_plus_1": strings.Repeat("A", 87385),
		} {
			t.Run(schemaName+"/"+name, func(t *testing.T) {
				var attempt map[string]any
				if err := json.Unmarshal(attemptBytes, &attempt); err != nil {
					t.Fatal(err)
				}
				attempt["RawBytes"] = raw
				if schemaValidate(schemas[schemaName], attempt) {
					t.Fatalf("%s schema accepted malformed RawBytes", schemaName)
				}
			})
		}
	}

	custodyBytes, err := os.ReadFile(filepath.Join(caseDir, "CUSTODY.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, identity := range map[string]string{
		"ascii_identity_257_bytes":      strings.Repeat("x", MaxCustodyIDBytes+1),
		"multibyte_identity_over_bytes": strings.Repeat("é", MaxCustodyIDBytes/2+1),
	} {
		t.Run(name, func(t *testing.T) {
			var custody map[string]any
			if err := json.Unmarshal(custodyBytes, &custody); err != nil {
				t.Fatal(err)
			}
			custody["ReviewerTaskID"] = identity
			if schemaValidate(schemas["ParentCustodyEvidence"], custody) {
				t.Fatal("ParentCustodyEvidence schema accepted oversized identity")
			}
		})
	}
}

func TestPredecessorsBlocked(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "DESIGN.md"), []byte("x"), 0644)
	if _, e := GenerateFreeze(d); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(d, "FREEZE.json"))
	if e != nil {
		t.Fatal(e)
	}
	for i := 1; i <= 10; i++ {
		if !strings.Contains(string(b), `"v`+string(rune('0'+i%10))+`"`) && i < 10 {
			t.Fatalf("missing v%d", i)
		}
	}
	if !strings.Contains(string(b), `"v10"`) {
		t.Fatal("missing v10")
	}
}
