package mcp

import (
	"reflect"
	"strings"
	"testing"
)

func TestCapabilitiesDescribeHostProvisionedBootstrapBoundary(t *testing.T) {
	for _, profile := range []ToolProfile{ToolProfileCompact, ToolProfileFull} {
		registry := NewRegistryWithProfile(false, profile)
		capabilities := registry.Capabilities()
		provisioning, ok := capabilities["managed_session_provisioning"].(map[string]any)
		if !ok {
			t.Fatalf("ASSERT_BOOTSTRAP_CAPABILITIES_PRESENT[%s]: %#v", profile, capabilities)
		}
		if provisioning["authority"] != "HOST_PROVISIONED_ONLY" || provisioning["bootstrap_option"] != "--bootstrap-config" || provisioning["public_template_command"] != "lsp-trace-mcp --print-bootstrap-example" {
			t.Fatalf("ASSERT_BOOTSTRAP_CAPABILITIES_AUTHORITY_AND_DISCOVERY[%s]: %#v", profile, provisioning)
		}
		if provisioning["caller_create_start_exposed"] != false || provisioning["full_profile_adds_create_start"] != false {
			t.Fatalf("ASSERT_BOOTSTRAP_CAPABILITIES_NO_CALLER_START[%s]: %#v", profile, provisioning)
		}
		policy, _ := provisioning["environment_policy"].(string)
		if !strings.Contains(policy, "private diagnostics are not configuration authority") {
			t.Fatalf("ASSERT_BOOTSTRAP_CAPABILITIES_PRIVATE_EVIDENCE_BOUNDARY[%s]: %q", profile, policy)
		}
		wantSequence := []string{"host configures bootstrap", "server starts trusted processes", "caller lists READY sessions", "caller binds exact session_id and generation", "caller invokes traversal"}
		if !reflect.DeepEqual(provisioning["discovery_sequence"], wantSequence) {
			t.Fatalf("ASSERT_BOOTSTRAP_CAPABILITIES_SEQUENCE[%s]: %#v", profile, provisioning["discovery_sequence"])
		}
		wantAdvertised := 28
		if profile == ToolProfileCompact {
			wantAdvertised = 10
		}
		if len(registry.Advertised()) != wantAdvertised || len(registry.Tools()) != 28 {
			t.Fatalf("ASSERT_BOOTSTRAP_GUIDANCE_PROFILE_NEUTRAL[%s]: advertised=%d dispatchable=%d", profile, len(registry.Advertised()), len(registry.Tools()))
		}
	}
}
