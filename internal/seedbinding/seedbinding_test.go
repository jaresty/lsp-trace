package seedbinding

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type revisionFixture string

func (r revisionFixture) Verify(_ context.Context, claim CustodyClaim) error {
	if claim.SourceRevision != string(r) {
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
		{"revision", BindingUnavailable, func(m *Manifest) { m.SourceRevision = "forged" }},
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

func TestHostReceiptAuthorityBindsOpenedTarget(t *testing.T) {
	root, manifest := seedFixture(t)
	manifestBytes, _ := json.Marshal(manifest)
	manifestSum := sha256.Sum256(manifestBytes)
	receipt := HostCustodyReceipt{
		Authenticated: true, Repository: root, SourceRevision: manifest.SourceRevision,
		TargetPath: manifest.ExpectedDeclaringFile, TargetSourceSHA256: manifest.SourceSHA256,
		SeedManifestSHA256: fmt.Sprintf("%x", manifestSum),
	}
	if got := ValidateMechanical(context.Background(), root, manifest, HostReceiptAuthority{Receipt: receipt}); got.Status != Match {
		t.Fatalf("ASSERT_HOST_RECEIPT_EXACT_TARGET_MATCH: %+v", got)
	}
	receipt.TargetSourceSHA256 = strings.Repeat("0", 64)
	if got := ValidateMechanical(context.Background(), root, manifest, HostReceiptAuthority{Receipt: receipt}); got.Status == Match {
		t.Fatal("ASSERT_HOST_RECEIPT_TARGET_DIGEST_MISMATCH_REJECTED")
	}
	receipt.TargetSourceSHA256 = manifest.SourceSHA256
	receipt.SeedManifestSHA256 = strings.Repeat("0", 64)
	if got := ValidateMechanical(context.Background(), root, manifest, HostReceiptAuthority{Receipt: receipt}); got.Status == Match {
		t.Fatal("ASSERT_HOST_RECEIPT_MANIFEST_DIGEST_MISMATCH_REJECTED")
	}
	if got := ValidateMechanical(context.Background(), root, manifest, HostReceiptAuthority{}); got.Status == Match {
		t.Fatal("ASSERT_HOST_RECEIPT_UNAUTHENTICATED_FAILS_CLOSED")
	}
}

func TestCallerAssertedLocalMechanicalMatrix(t *testing.T) {
	root, base := seedFixture(t)
	base.SchemaVersion = VersionV3
	base.CustodyMode = CallerAssertedLocal
	base.Validator.Class = "managed-stdio"
	base.Validator.ExecutableSHA256 = "sha256:" + strings.Repeat("1", 64)
	base.Validator.PayloadSHA256 = "sha256:" + strings.Repeat("2", 64)
	base.Validator.ConfigSHA256 = "sha256:" + strings.Repeat("3", 64)
	if got := ValidateMechanical(context.Background(), root, base, nil); got.Status != Match || got.Provenance != CallerAssertedLocal {
		t.Fatalf("ASSERT_CALLER_ASSERTED_LOCAL_EXPLICIT_SUCCESS: %+v", got)
	}
	cases := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"revision", func(m *Manifest) { m.SourceRevision = "" }},
		{"digest", func(m *Manifest) { m.SourceSHA256 = strings.Repeat("0", 64) }},
		{"name", func(m *Manifest) { m.ExpectedSymbol = "" }},
		{"range", func(m *Manifest) { m.ExpectedDeclarationRange.StartCharacter = 58 }},
		{"provider", func(m *Manifest) { m.Validator.PayloadSHA256 = "" }},
		{"prepared-digest", func(m *Manifest) { m.PreparedManifestSHA256 = "not-a-digest" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.mutate(&m)
			if got := ValidateMechanical(context.Background(), root, m, nil); got.Status == Match || got.Provenance != CallerAssertedLocal {
				t.Fatalf("ASSERT_CALLER_ASSERTED_LOCAL_%s_MISMATCH: %+v", strings.ToUpper(tc.name), got)
			}
		})
	}
	wrong := filepath.Join(root, "Wrong.cs")
	body, err := os.ReadFile(filepath.Join(root, base.ExpectedDeclaringFile))
	if err != nil || os.WriteFile(wrong, body, 0600) != nil {
		t.Fatal(err)
	}
	m := base
	m.Locator.URI = (&url.URL{Scheme: "file", Path: wrong}).String()
	if got := ValidateMechanical(context.Background(), root, m, nil); got.Status == Match {
		t.Fatalf("ASSERT_CALLER_ASSERTED_LOCAL_WRONG_FILE_VALID_TEXT: %+v", got)
	}
	verified := base
	verified.CustodyMode = VerifiedHost
	if got := ValidateMechanical(context.Background(), root, verified, nil); got.Status != Unavailable || got.Provenance != VerifiedHost {
		t.Fatalf("ASSERT_HOST_FAILURE_CANNOT_DOWNGRADE: %+v", got)
	}
}

func TestBindingRejectsDuplicateKeysAndOversizeManifest(t *testing.T) {
	if _, err := DecodeV2([]byte(`{"schema_version":"lsp-trace.seed-binding.v2","schema_version":"lsp-trace.seed-binding.v2"}`)); err == nil {
		t.Fatal("ASSERT_SEED_BINDING_DUPLICATE_KEY_REJECTED")
	}
	if _, err := DecodeV2(make([]byte, MaxManifestBytes+1)); err == nil {
		t.Fatal("ASSERT_SEED_BINDING_MANIFEST_SIZE_BOUND")
	}
}

func TestDocumentSymbolStrictRequiredMembers(t *testing.T) {
	_, manifest := seedFixture(t)
	source := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	validRange := `{"start":{"line":0,"character":30},"end":{"line":0,"character":57}}`
	cases := []string{
		`[{"name":"GetSchoolFilterModel","range":` + validRange + `,"selectionRange":` + validRange + `}]`,
		`[{"name":"GetSchoolFilterModel","kind":6,"range":` + validRange + `}]`,
		`[{"name":"GetSchoolFilterModel","kind":6,"location":null}]`,
		`[{"name":"GetSchoolFilterModel","kind":6,"location":{"range":` + validRange + `}}]`,
		`[{"name":"GetSchoolFilterModel","name":"GetSchoolFilterModel","kind":6,"range":` + validRange + `,"selectionRange":` + validRange + `}]`,
	}
	for i, raw := range cases {
		if got := ValidateDocumentSymbols([]byte(raw), manifest, source, "utf-8"); got.Status != Invalid {
			t.Fatalf("ASSERT_STRICT_DOCUMENT_SYMBOL_REQUIRED_%d: %+v", i, got)
		}
	}
	if got := ValidateDocumentSymbols(make([]byte, MaxDocumentSymbolBytes+1), manifest, source, "utf-8"); got.Status != Invalid {
		t.Fatalf("ASSERT_DOCUMENT_SYMBOL_RESPONSE_SIZE_BOUND: %+v", got)
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
