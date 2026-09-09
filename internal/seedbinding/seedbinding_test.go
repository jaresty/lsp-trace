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

func seedFixture(t *testing.T) (string, Manifest) {
	t.Helper()
	root := t.TempDir()
	body := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	path := filepath.Join(root, "VariableApiController.cs")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	identity := ValidatorIdentity{Language: "csharp", Authority: "MANAGED_LSP", Name: "csharp-ls", Version: "host-pinned"}
	m := Manifest{SchemaVersion: VersionV2, ID: "synthetic", Locator: Locator{URI: (&url.URL{Scheme: "file", Path: path}).String(), Line: 0, Character: 35, Encoding: "utf-8"}, ExpectedSymbol: "GetSchoolFilterModel", ExpectedDeclaringFile: "VariableApiController.cs", ExpectedDeclarationRange: Range{StartLine: 0, StartCharacter: 30, EndLine: 0, EndCharacter: 57}, SourceRevision: "rev", SourceSHA256: fmt.Sprintf("%x", sum), Validator: identity}
	return root, m
}

func TestBindingMechanicalMatrix(t *testing.T) {
	root, base := seedFixture(t)
	if got := ValidateMechanical(context.Background(), root, base, revisionFixture("rev")); got.Status != Match {
		t.Fatalf("ASSERT_SEED_BINDING_MECHANICAL_MATCH_REQUIRED: %+v", got)
	}
	cases := []struct {
		name, want string
		mutate     func(*Manifest)
	}{
		{"wrong-file", BindingMismatch, func(m *Manifest) { m.ExpectedDeclaringFile = "Other.cs" }},
		{"wrong-range", LocatorInvalid, func(m *Manifest) { m.ExpectedDeclarationRange.StartCharacter = 58 }},
		{"encoding", LocatorInvalid, func(m *Manifest) { m.Locator.Encoding = "utf-7" }},
		{"uri-alias", LocatorInvalid, func(m *Manifest) {
			m.Locator.URI = strings.Replace(m.Locator.URI, "VariableApiController.cs", "./VariableApiController.cs", 1)
		}},
		{"digest", SourceMismatch, func(m *Manifest) { m.SourceSHA256 = strings.Repeat("0", 64) }},
		{"revision", SourceMismatch, func(m *Manifest) { m.SourceRevision = "forged" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.mutate(&m)
			got := ValidateMechanical(context.Background(), root, m, revisionFixture("rev"))
			if got.Terminal != tc.want {
				t.Fatalf("ASSERT_SEED_BINDING_%s: %+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got)
			}
		})
	}
}

func TestBindingRejectsOversizeAndInvalidCodeUnitBoundaries(t *testing.T) {
	root, manifest := seedFixture(t)
	path := filepath.Join(root, manifest.ExpectedDeclaringFile)
	oversize := make([]byte, (64<<20)+1)
	for i := range oversize {
		oversize[i] = 'a'
	}
	if err := os.WriteFile(path, oversize, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(oversize[:64<<20])
	manifest.SourceSHA256 = fmt.Sprintf("%x", sum)
	if got := ValidateMechanical(context.Background(), root, manifest, revisionFixture("rev")); got.Terminal != LocatorInvalid {
		t.Fatalf("ASSERT_SEED_BINDING_OVERSIZE_REJECTED: status=%s terminal=%s", got.Status, got.Terminal)
	}
	if validPosition([]byte("é\n"), 0, 1, "utf-8") {
		t.Fatal("ASSERT_SEED_BINDING_UTF8_MID_CODEPOINT_REJECTED")
	}
	if validPosition([]byte("😀\n"), 0, 1, "utf-16") {
		t.Fatal("ASSERT_SEED_BINDING_UTF16_SURROGATE_HALF_REJECTED")
	}
}

func TestBindingClosedSchemaAndSymlinkEscape(t *testing.T) {
	_, m := seedFixture(t)
	raw := fmt.Sprintf(`{"schema_version":%q,"unknown":true}`, m.SchemaVersion)
	if _, err := DecodeV2([]byte(raw)); err == nil {
		t.Fatal("ASSERT_SEED_BINDING_V2_CLOSED_SCHEMA")
	}
	root, outside := t.TempDir(), t.TempDir()
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
	if got := ValidateMechanical(context.Background(), root, m, revisionFixture("rev")); got.Terminal != LocatorInvalid {
		t.Fatalf("ASSERT_SEED_BINDING_SYMLINK_ESCAPE: %+v", got)
	}
}
