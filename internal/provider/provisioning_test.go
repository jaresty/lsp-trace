package provider

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	assertDeclaration = "ASSERT_PROVIDER_DECLARATION_CLOSED: host declarations require absolute executable, stable identity/version/protocol, supported capabilities, and bounded limits"
	assertRegistry    = "ASSERT_PROVIDER_REGISTRY_DETERMINISTIC: admitted declarations construct order-independent metadata and runtime registries"
	assertSelector    = "ASSERT_PROVIDER_SELECTOR_CLOSED: selectors admit only auto, none, or registered stable identities"
	assertCopies      = "ASSERT_PROVIDER_PROVISIONING_IMMUTABLE: provisioned metadata and registrations do not alias host declaration slices"
)

func validDeclaration(id string) Declaration {
	return Declaration{
		Identity: id, Version: "1.2.3", Protocol: ProtocolIdentity{Name: "lsp-trace-provider", Version: "1"},
		Executable: "/opt/lsp-trace/providers/relations", Arguments: []string{"--stdio"}, Directory: "/opt/lsp-trace",
		Environment:  []string{"LANG=C"},
		Capabilities: Capabilities{Relations: []string{"PASSES_CALLBACK", "BINDS_ARGUMENT"}, Languages: []string{"typescript"}, Frameworks: []string{"ember"}},
		Limits:       Limits{RequestBytes: 4096, ResponseBytes: 8192, ProtocolMessages: 1, StderrBytes: 1024, WallTime: time.Second, TerminationGrace: 100 * time.Millisecond},
	}
}

func TestProviderDeclarationClosedValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Declaration)
	}{
		{"relative executable", func(d *Declaration) { d.Executable = "provider --stdio" }},
		{"empty identity", func(d *Declaration) { d.Identity = "" }},
		{"unstable identity", func(d *Declaration) { d.Identity = "/opt/provider" }},
		{"missing version", func(d *Declaration) { d.Version = "" }},
		{"missing protocol", func(d *Declaration) { d.Protocol.Version = "" }},
		{"unknown relation", func(d *Declaration) { d.Capabilities.Relations = []string{"EXECUTES_CALLBACK"} }},
		{"empty languages", func(d *Declaration) { d.Capabilities.Languages = nil }},
		{"empty frameworks", func(d *Declaration) { d.Capabilities.Frameworks = nil }},
		{"unbounded limit", func(d *Declaration) { d.Limits.ResponseBytes = MaxProviderLimits.ResponseBytes + 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDeclaration("ember-relations@1")
			tc.mutate(&d)
			if _, err := Provision([]Declaration{d}); err == nil {
				t.Fatalf("%s: accepted %s", assertDeclaration, tc.name)
			}
		})
	}
	if _, err := Provision([]Declaration{validDeclaration("ember-relations@1"), validDeclaration("ember-relations@1")}); err == nil {
		t.Fatalf("%s: accepted duplicate identity", assertDeclaration)
	}
}

func TestProviderRegistryDeterministic(t *testing.T) {
	a, b := validDeclaration("zeta@1"), validDeclaration("alpha@1")
	a.Capabilities = Capabilities{
		Relations:  []string{"UPDATES_STATE", "PASSES_CALLBACK"},
		Languages:  []string{"typescript", "javascript"},
		Frameworks: []string{"glimmer", "ember"},
	}
	first, err := Provision([]Declaration{a, b})
	if err != nil {
		t.Fatalf("%s: %v", assertRegistry, err)
	}
	second, err := Provision([]Declaration{b, a})
	if err != nil {
		t.Fatalf("%s: %v", assertRegistry, err)
	}
	if !reflect.DeepEqual(first.Admission, second.Admission) || !reflect.DeepEqual(first.Declarations, second.Declarations) {
		t.Fatalf("%s: first=%+v second=%+v", assertRegistry, first, second)
	}
	for _, id := range []string{"alpha@1", "zeta@1"} {
		if _, ok := first.Registry.Resolve(id); !ok {
			t.Fatalf("%s: missing %s", assertRegistry, id)
		}
	}
	got := first.Declarations[1].Capabilities
	if !reflect.DeepEqual(got.Relations, []string{"PASSES_CALLBACK", "UPDATES_STATE"}) ||
		!reflect.DeepEqual(got.Languages, []string{"javascript", "typescript"}) ||
		!reflect.DeepEqual(got.Frameworks, []string{"ember", "glimmer"}) {
		t.Fatalf("%s: capabilities not canonical: %+v", assertRegistry, got)
	}
}

func TestProviderSelectorClosed(t *testing.T) {
	p, err := Provision([]Declaration{validDeclaration("ember-relations@1")})
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"auto", "none", "ember-relations@1"} {
		if err := p.Admission.Validate(selector); err != nil {
			t.Fatalf("%s: rejected %q: %v", assertSelector, selector, err)
		}
	}
	for _, selector := range []string{"", "/opt/lsp-trace/providers/relations", "provider --stdio", "LANG=C", "unknown@1"} {
		if err := p.Admission.Validate(selector); err == nil {
			t.Fatalf("%s: accepted %q", assertSelector, selector)
		}
	}
	if got := p.Admission.Allowed(); !reflect.DeepEqual(got, []string{"auto", "none", "ember-relations@1"}) {
		t.Fatalf("%s: allowed=%v", assertSelector, got)
	}
}

func TestProviderProvisioningImmutable(t *testing.T) {
	d := validDeclaration("ember-relations@1")
	p, err := Provision([]Declaration{d})
	if err != nil {
		t.Fatal(err)
	}
	d.Arguments[0], d.Environment[0], d.Capabilities.Relations[0] = "mutated", "BAD=1", "UPDATES_STATE"
	reg, ok := p.Registry.Resolve("ember-relations@1")
	if !ok {
		t.Fatal(assertCopies)
	}
	if strings.Join(reg.Args, " ") != "--stdio" || strings.Join(reg.Env, " ") != "LANG=C" || strings.Join(p.Declarations[0].Arguments, " ") != "--stdio" || p.Declarations[0].Capabilities.Relations[0] != "BINDS_ARGUMENT" {
		t.Fatalf("%s: registration=%+v declaration=%+v", assertCopies, reg, p.Declarations[0])
	}
}
