package provider

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func admissionDeclaration(id string, relations ...string) Declaration {
	return Declaration{Identity: id, Version: strings.Split(id, "@")[1], Protocol: ProtocolIdentity{Name: "lsp-trace.provider-observations", Version: "1"}, Executable: filepath.Join(string(filepath.Separator), "host", id), Capabilities: Capabilities{Relations: relations, Languages: []string{"typescript"}, Frameworks: []string{"opaque"}}, Limits: Limits{RequestBytes: 4096, ResponseBytes: 8192, ProtocolMessages: 2, StderrBytes: 128, WallTime: time.Second, TerminationGrace: time.Millisecond}}
}

func mustProvisionAdmission(t *testing.T, declarations ...Declaration) Provisioned {
	t.Helper()
	p, err := Provision(declarations)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAdmissionClosedSelection(t *testing.T) {
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, admissionDeclaration("alpha@1", "PASSES_CALLBACK")), "semantic@1")
	for _, selector := range []string{"/tmp/provider", "alpha", "alpha --flag", "PATH=/tmp", "none"} {
		if _, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{selector}}); err == nil {
			t.Fatalf("ASSERT_PROVIDER_ADMISSION_CLOSED_SELECTION: accepted %q", selector)
		}
	}
}

func TestAdmissionRelationCapability(t *testing.T) {
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, admissionDeclaration("alpha@1", "PASSES_CALLBACK")), "semantic@1")
	if _, err := r.Admit(context.Background(), Selection{Relations: []string{"RENDERS_FROM"}, Providers: []string{"alpha@1"}}); err == nil {
		t.Fatal("ASSERT_PROVIDER_ADMISSION_RELATION_CAPABILITY: accepted unsupported relation")
	}
	got, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"alpha@1"}})
	if err != nil || got.ProviderID != "alpha@1" || got.AdapterID != "semantic@1" {
		t.Fatalf("ASSERT_PROVIDER_ADMISSION_RELATION_CAPABILITY: got=%+v err=%v", got, err)
	}
}

func TestAdmissionAutoUnambiguous(t *testing.T) {
	a, b := admissionDeclaration("alpha@1", "PASSES_CALLBACK"), admissionDeclaration("beta@1", "PASSES_CALLBACK")
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, a, b), "semantic@1")
	if _, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"auto"}}); err == nil {
		t.Fatal("ASSERT_PROVIDER_ADMISSION_AUTO_UNAMBIGUOUS: accepted ambiguous auto")
	}
}

func TestAdmissionDeterministic(t *testing.T) {
	one := mustProvisionAdmission(t, admissionDeclaration("alpha@1", "PASSES_CALLBACK"))
	r, _ := NewAdmissionResolver(one, "semantic@1")
	x, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"auto"}})
	y, err2 := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: nil})
	if err != nil || err2 != nil || !reflect.DeepEqual(x, y) || x.ProviderID != "alpha@1" {
		t.Fatalf("ASSERT_PROVIDER_ADMISSION_DETERMINISTIC: x=%+v/%v y=%+v/%v", x, err, y, err2)
	}
}

func TestAdmissionAutoRequiresEveryConstraintAndExactlyOne(t *testing.T) {
	complete := admissionDeclaration("complete@1", "CALLS", "PASSES_CALLBACK")
	complete.Capabilities.Languages = []string{"javascript", "typescript"}
	complete.Capabilities.Frameworks = []string{"framework-a", "framework-b"}
	partialRelation := admissionDeclaration("partial-relation@1", "CALLS")
	partialRelation.Capabilities.Languages = append([]string(nil), complete.Capabilities.Languages...)
	partialRelation.Capabilities.Frameworks = append([]string(nil), complete.Capabilities.Frameworks...)
	partialLanguage := admissionDeclaration("partial-language@1", "CALLS", "PASSES_CALLBACK")
	partialLanguage.Capabilities.Languages = []string{"typescript"}
	partialLanguage.Capabilities.Frameworks = append([]string(nil), complete.Capabilities.Frameworks...)
	partialFramework := admissionDeclaration("partial-framework@1", "CALLS", "PASSES_CALLBACK")
	partialFramework.Capabilities.Languages = append([]string(nil), complete.Capabilities.Languages...)
	partialFramework.Capabilities.Frameworks = []string{"framework-a"}

	selection := Selection{Relations: []string{"CALLS", "PASSES_CALLBACK"}, Languages: []string{"javascript", "typescript"}, Frameworks: []string{"framework-a", "framework-b"}, Providers: []string{"auto"}}
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, complete, partialRelation, partialLanguage, partialFramework), "semantic@1")
	got, err := r.Admit(context.Background(), selection)
	if err != nil || got.ProviderID != "complete@1" {
		t.Fatalf("ASSERT_PROVIDER_AUTO_ALL_CONSTRAINTS: got=%+v err=%v", got, err)
	}

	other := complete
	other.Identity = "other@1"
	other.Executable = filepath.Join(string(filepath.Separator), "host", "other@1")
	r, _ = NewAdmissionResolver(mustProvisionAdmission(t, complete, other), "semantic@1")
	if _, err := r.Admit(context.Background(), selection); err == nil {
		t.Fatal("ASSERT_PROVIDER_AUTO_EXACTLY_ONE: accepted two complete matches")
	}
	r, _ = NewAdmissionResolver(mustProvisionAdmission(t, partialRelation, partialLanguage, partialFramework), "semantic@1")
	if _, err := r.Admit(context.Background(), selection); err == nil {
		t.Fatal("ASSERT_PROVIDER_AUTO_EXACTLY_ONE: accepted zero complete matches")
	}
}

func TestAdmissionExactRequiresEveryConstraint(t *testing.T) {
	declaration := admissionDeclaration("alpha@1", "CALLS")
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, declaration), "semantic@1")
	for name, selection := range map[string]Selection{
		"language":  {Relations: []string{"CALLS"}, Languages: []string{"javascript"}, Frameworks: []string{"opaque"}, Providers: []string{"alpha@1"}},
		"framework": {Relations: []string{"CALLS"}, Languages: []string{"typescript"}, Frameworks: []string{"framework-a"}, Providers: []string{"alpha@1"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.Admit(context.Background(), selection); err == nil {
				t.Fatalf("ASSERT_PROVIDER_EXACT_ALL_CONSTRAINTS: accepted incompatible %s", name)
			}
		})
	}
	if got, err := r.Admit(context.Background(), Selection{Relations: []string{"CALLS"}, Languages: []string{"typescript"}, Frameworks: []string{"opaque"}, Providers: []string{"alpha@1"}}); err != nil || got.ProviderID != "alpha@1" {
		t.Fatalf("ASSERT_PROVIDER_EXACT_ALL_CONSTRAINTS: compatible exact selection got=%+v err=%v", got, err)
	}
}

func TestAdmissionRequestCannotSupplyExecutionAuthority(t *testing.T) {
	r, _ := NewAdmissionResolver(mustProvisionAdmission(t, admissionDeclaration("alpha@1", "CALLS")), "semantic@1")
	if _, err := r.Admit(context.Background(), Selection{Relations: []string{"CALLS"}, Languages: []string{"typescript"}, Frameworks: []string{"opaque"}, Providers: []string{"/tmp/request-provider"}}); err == nil {
		t.Fatal("ASSERT_PROVIDER_REQUEST_NO_EXECUTION_AUTHORITY: accepted request executable")
	}
}
