package runtimeprofile

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveProducesCanonicalImmutableIdentity(t *testing.T) {
	options := []Option{{Name: "analysis", Value: "strict"}}
	selector, err := Validate(Selector{TrustDomain: "local", Workspace: "/work/a/../repo", Profile: "gopls", EnvironmentReference: "developer-shell", Options: options})
	if err != nil {
		t.Fatal(err)
	}
	options[0].Value = "mutated"
	first := Resolve(selector)
	second := Resolve(selector)
	if first != second {
		t.Fatal("ASSERT_RUNTIME_PROFILE_DETERMINISTIC: same validated selector produced different profiles")
	}
	if first.Workspace().String() != "/work/repo" {
		t.Fatalf("ASSERT_RUNTIME_PROFILE_CANONICAL_WORKSPACE: got %q", first.Workspace().String())
	}
	if first.ProfileName() != "gopls" || first.Environment().String() != "developer-shell" {
		t.Fatalf("ASSERT_RUNTIME_PROFILE_RESOLVED_IDENTITY: got profile=%q environment=%q", first.ProfileName(), first.Environment().String())
	}
}

func TestResolverBoundaryHasNoSecretOrExecutableAuthority(t *testing.T) {
	typ := reflect.TypeOf(Profile{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, forbidden := range []string{"secret", "token", "password", "command", "executable", "permission"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("ASSERT_RUNTIME_PROFILE_AUTHORITY_CEILING: forbidden field %q", typ.Field(i).Name)
			}
		}
	}
	if _, err := Validate(Selector{TrustDomain: "local", Workspace: "/work/repo", Profile: "gopls", EnvironmentReference: "TOKEN=raw-secret"}); err == nil {
		t.Fatal("ASSERT_RUNTIME_PROFILE_ENV_REFERENCE_ONLY: literal environment assignment accepted")
	}
}

func TestValidateRejectsNonCanonicalOrIncompleteSelector(t *testing.T) {
	cases := []Selector{
		{},
		{TrustDomain: "local", Workspace: "relative", Profile: "gopls", EnvironmentReference: "developer-shell"},
		{TrustDomain: "local", Workspace: "/work/repo", Profile: "gopls", EnvironmentReference: "developer shell"},
		{TrustDomain: "local", Workspace: "/work/repo", Profile: "gopls", EnvironmentReference: "developer-shell", Options: []Option{{Name: "z", Value: "1"}, {Name: "a", Value: "2"}}},
	}
	for i, input := range cases {
		if _, err := Validate(input); err == nil {
			t.Fatalf("ASSERT_RUNTIME_PROFILE_VALIDATED_SELECTOR_ONLY: case %d accepted", i)
		}
	}
}
