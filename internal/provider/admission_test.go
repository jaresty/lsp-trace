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

func TestAdmissionClosedSelectionAndCapability(t *testing.T) {
	p := mustProvisionAdmission(t, admissionDeclaration("alpha@1", "PASSES_CALLBACK"))
	r, _ := NewAdmissionResolver(p, "semantic@1")
	for _, selector := range []string{"/tmp/provider", "alpha", "alpha --flag", "PATH=/tmp"} {
		if _, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{selector}}); err == nil {
			t.Fatalf("ASSERT_PROVIDER_ADMISSION_CLOSED_SELECTION: accepted %q", selector)
		}
	}
	if _, err := r.Admit(context.Background(), Selection{Relations: []string{"RENDERS_FROM"}, Providers: []string{"alpha@1"}}); err == nil {
		t.Fatal("ASSERT_PROVIDER_ADMISSION_RELATION_CAPABILITY: accepted unsupported relation")
	}
	got, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"alpha@1"}})
	if err != nil || got.ProviderID != "alpha@1" || got.AdapterID != "semantic@1" {
		t.Fatalf("ASSERT_PROVIDER_ADMISSION_RELATION_CAPABILITY: got=%+v err=%v", got, err)
	}
}

func TestAdmissionAutoNoneAndDeterminism(t *testing.T) {
	a := admissionDeclaration("alpha@1", "PASSES_CALLBACK")
	b := admissionDeclaration("beta@1", "PASSES_CALLBACK")
	for _, declarations := range [][]Declaration{{a, b}, {b, a}} {
		r, _ := NewAdmissionResolver(mustProvisionAdmission(t, declarations...), "semantic@1")
		if _, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"auto"}}); err == nil {
			t.Fatal("ASSERT_PROVIDER_ADMISSION_AUTO_UNAMBIGUOUS: accepted ambiguous auto")
		}
		if _, err := r.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"none"}}); err == nil {
			t.Fatal("ASSERT_PROVIDER_ADMISSION_CLOSED_SELECTION: none must disable provider admission")
		}
	}
	one := mustProvisionAdmission(t, a)
	r1, _ := NewAdmissionResolver(one, "semantic@1")
	x, err := r1.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: []string{"auto"}})
	y, err2 := r1.Admit(context.Background(), Selection{Relations: []string{"PASSES_CALLBACK"}, Providers: nil})
	if err != nil || err2 != nil || !reflect.DeepEqual(x, y) || x.ProviderID != "alpha@1" {
		t.Fatalf("ASSERT_PROVIDER_ADMISSION_DETERMINISTIC: x=%+v/%v y=%+v/%v", x, err, y, err2)
	}
}
