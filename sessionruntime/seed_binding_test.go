package sessionruntime

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/seedbinding"
)

func bindingProfile(t *testing.T, workspace string) runtimeprofile.Profile {
	t.Helper()
	v, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "fixture", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeprofile.Resolve(v)
}

type bindingStarter struct{ calls int }

func (s *bindingStarter) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	s.calls++
	return nil, managedprocess.StartObservation{}
}

type bindingRevision struct{ revision string }

func (a bindingRevision) Verify(_ context.Context, workspace, revision string) error {
	if revision != a.revision {
		return fmt.Errorf("revision mismatch")
	}
	return nil
}

type bindingValidator struct{ result seedbinding.ValidationResult }

func (v bindingValidator) Identity() seedbinding.ValidatorIdentity {
	return seedbinding.ValidatorIdentity{Language: "csharp", Authority: "SYNTHETIC_DECLARATION_FIXTURE", Name: "fixture", Version: "1"}
}
func (v bindingValidator) Validate(seedbinding.ValidationInput) seedbinding.ValidationResult {
	return v.result
}

func TestSeedBindingRetainsPreflightSnapshotAcrossPathSwap(t *testing.T) {
	workspace := t.TempDir()
	original := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	path := filepath.Join(workspace, "VariableApiController.cs")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)
	identity := seedbinding.ValidatorIdentity{Language: "csharp", Authority: "SYNTHETIC_DECLARATION_FIXTURE", Name: "fixture", Version: "1"}
	manifest := seedbinding.Manifest{SchemaVersion: seedbinding.VersionV2, ID: "synthetic", Locator: seedbinding.Locator{URI: (&url.URL{Scheme: "file", Path: path}).String(), Line: 0, Character: 35, Encoding: "utf-8"}, ExpectedSymbol: "GetSchoolFilterModel", ExpectedDeclaringFile: "VariableApiController.cs", ExpectedDeclarationRange: seedbinding.Range{StartLine: 0, StartCharacter: 35, EndLine: 0, EndCharacter: 55}, SourceRevision: "rev", SourceSHA256: fmt.Sprintf("%x", sum), Validator: identity}
	runtime, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 4}, Starter: oneChildStarter{referenceChild{}}, SeedRevisionAuthority: bindingRevision{"rev"}, SeedBindingValidator: bindingValidator{seedbinding.ValidationResult{Status: seedbinding.Match}}})
	if err != nil {
		t.Fatal(err)
	}
	started := runtime.Start(context.Background(), StartRequest{Profile: bindingProfile(t, workspace), SeedBinding: &manifest})
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	replacement := []byte("class Other { }\n")
	if err := os.WriteFile(path, replacement, 0600); err != nil {
		t.Fatal(err)
	}
	got := runtime.sessions[started.SessionID].seedSources[manifest.Locator.URI]
	if string(got) != string(original) || string(got) == string(replacement) {
		t.Fatalf("ASSERT_SEED_SOURCE_READ_ONCE_DESCRIPTOR_SNAPSHOT: %q", got)
	}
}

func TestSeedBindingRejectsBeforeAttemptOrProviderStart(t *testing.T) {
	workspace := t.TempDir()
	source := []byte("class VariableApiController { void GetSchoolFilterModel() {} }\n")
	file := filepath.Join(workspace, "VariableApiController.cs")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	u := (&url.URL{Scheme: "file", Path: file}).String()
	sum := sha256.Sum256(source)
	manifest := seedbinding.Manifest{SchemaVersion: seedbinding.VersionV2, ID: "synthetic", Locator: seedbinding.Locator{URI: u, Line: 0, Character: 35, Encoding: "utf-8"}, ExpectedSymbol: "GetSchoolFilterModel", ExpectedDeclaringFile: "VariableApiController.cs", ExpectedDeclarationRange: seedbinding.Range{StartLine: 0, StartCharacter: 35, EndLine: 0, EndCharacter: 55}, SourceRevision: "rev", SourceSHA256: fmt.Sprintf("%x", sum), Validator: seedbinding.ValidatorIdentity{Language: "csharp", Authority: "SYNTHETIC_DECLARATION_FIXTURE", Name: "fixture", Version: "1"}}
	for _, tc := range []struct {
		name   string
		mutate func(*seedbinding.Manifest)
		result seedbinding.ValidationResult
		want   string
	}{
		{"wrong-name", func(m *seedbinding.Manifest) { m.ExpectedSymbol = "VariableApiController" }, seedbinding.ValidationResult{Status: seedbinding.Mismatch}, "SEED_BINDING_MISMATCH"},
		{"unavailable", func(*seedbinding.Manifest) {}, seedbinding.ValidationResult{Status: seedbinding.Unavailable}, "SEED_BINDING_UNAVAILABLE"},
		{"digest", func(m *seedbinding.Manifest) { m.SourceSHA256 = strings.Repeat("0", 64) }, seedbinding.ValidationResult{Status: seedbinding.Match}, "SEED_SOURCE_MISMATCH"},
		{"revision", func(m *seedbinding.Manifest) { m.SourceRevision = "wrong" }, seedbinding.ValidationResult{Status: seedbinding.Match}, "SEED_SOURCE_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest
			tc.mutate(&m)
			starter := &bindingStarter{}
			runtime, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 4}, Starter: starter, SeedRevisionAuthority: bindingRevision{"rev"}, SeedBindingValidator: bindingValidator{tc.result}})
			if err != nil {
				t.Fatal(err)
			}
			before := runtime.Census()
			got := runtime.Start(context.Background(), StartRequest{Profile: bindingProfile(t, workspace), SeedBinding: &m})
			if string(got.Failure) != tc.want || starter.calls != 0 || runtime.Census() != before || got.AttemptID != "" {
				t.Fatalf("ASSERT_SEED_BINDING_PRESTART_ZERO: result=%+v starts=%d before=%+v after=%+v", got, starter.calls, before, runtime.Census())
			}
			if strings.Contains(got.PublicDetail, workspace) || strings.Contains(got.PublicDetail, string(source)) {
				t.Fatalf("ASSERT_SEED_BINDING_PUBLIC_PRIVACY: %q", got.PublicDetail)
			}
		})
	}
}
