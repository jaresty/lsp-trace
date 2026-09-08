package seedbinding

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type revisionFixture string

func (r revisionFixture) Verify(_ context.Context, _, revision string) error {
	if revision != string(r) {
		return fmt.Errorf("rejected")
	}
	return nil
}

type validatorFixture struct {
	identity ValidatorIdentity
	result   ValidationResult
	calls    int
}

func (v *validatorFixture) Identity() ValidatorIdentity { return v.identity }
func (v *validatorFixture) Validate(in ValidationInput) ValidationResult {
	v.calls++
	if string(in.Source) == "" {
		return ValidationResult{Status: Invalid}
	}
	return v.result
}

func seedFixture(t *testing.T) (string, Manifest, *validatorFixture) {
	t.Helper()
	root := t.TempDir()
	body := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	path := filepath.Join(root, "VariableApiController.cs")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	id := ValidatorIdentity{Language: "csharp", Authority: "SYNTHETIC_DECLARATION_FIXTURE", Name: "exact-fixture", Version: "1"}
	m := Manifest{SchemaVersion: VersionV2, ID: "synthetic", Locator: Locator{URI: (&url.URL{Scheme: "file", Path: path}).String(), Line: 0, Character: 35, Encoding: "utf-8"}, ExpectedSymbol: "GetSchoolFilterModel", ExpectedDeclaringFile: "VariableApiController.cs", ExpectedDeclarationRange: Range{StartLine: 0, StartCharacter: 35, EndLine: 0, EndCharacter: 55}, SourceRevision: "rev", SourceSHA256: fmt.Sprintf("%x", sum), Validator: id}
	return root, m, &validatorFixture{identity: id, result: ValidationResult{Status: Match}}
}

func TestBindingMechanicalAndSemanticMatrix(t *testing.T) {
	root, base, validator := seedFixture(t)
	if got := Validate(context.Background(), root, base, revisionFixture("rev"), validator); got.Status != Match || validator.calls != 1 {
		t.Fatalf("ASSERT_SEED_BINDING_MATCH_REQUIRED: %+v calls=%d", got, validator.calls)
	}
	cases := []struct {
		name, want string
		mutate     func(*Manifest)
		validator  func(*validatorFixture)
	}{
		{"wrong-file", BindingMismatch, func(m *Manifest) { m.ExpectedDeclaringFile = "Other.cs" }, nil},
		{"wrong-name", BindingMismatch, func(m *Manifest) { m.ExpectedSymbol = "Other" }, func(v *validatorFixture) { v.result = ValidationResult{Status: Mismatch, PrivateDetail: "name"} }},
		{"wrong-range", BindingMismatch, func(m *Manifest) { m.ExpectedDeclarationRange.StartCharacter++ }, func(v *validatorFixture) { v.result = ValidationResult{Status: Mismatch, PrivateDetail: "range"} }},
		{"encoding", LocatorInvalid, func(m *Manifest) { m.Locator.Encoding = "utf-7" }, nil},
		{"uri-alias", LocatorInvalid, func(m *Manifest) {
			m.Locator.URI = strings.Replace(m.Locator.URI, "VariableApiController.cs", "./VariableApiController.cs", 1)
		}, nil},
		{"digest", SourceMismatch, func(m *Manifest) { m.SourceSHA256 = strings.Repeat("0", 64) }, nil},
		{"unavailable", BindingUnavailable, func(*Manifest) {}, func(v *validatorFixture) { v.result = ValidationResult{Status: Unavailable} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			v := *validator
			tc.mutate(&m)
			if tc.validator != nil {
				tc.validator(&v)
			}
			got := Validate(context.Background(), root, m, revisionFixture("rev"), &v)
			if got.Terminal != tc.want {
				t.Fatalf("ASSERT_SEED_BINDING_%s: %+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got)
			}
		})
	}
}

func TestBindingClosedSchemaAndSymlinkEscape(t *testing.T) {
	_, m, _ := seedFixture(t)
	raw := fmt.Sprintf(`{"schema_version":%q,"unknown":true}`, m.SchemaVersion)
	if _, err := DecodeV2([]byte(raw)); err == nil {
		t.Fatal("ASSERT_SEED_BINDING_V2_CLOSED_SCHEMA")
	}
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "x.cs")
	_ = os.WriteFile(target, []byte("x"), 0600)
	link := filepath.Join(root, "x.cs")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	sum := sha256.Sum256([]byte("x"))
	m.Locator.URI = (&url.URL{Scheme: "file", Path: link}).String()
	m.ExpectedDeclaringFile = "x.cs"
	m.SourceSHA256 = fmt.Sprintf("%x", sum)
	if got := Validate(context.Background(), root, m, revisionFixture("rev"), nil); got.Terminal != LocatorInvalid {
		t.Fatalf("ASSERT_SEED_BINDING_SYMLINK_ESCAPE: %+v", got)
	}
}
