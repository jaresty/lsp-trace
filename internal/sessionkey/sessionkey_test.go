package sessionkey

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeriveIsDeterministicAndDomainSeparated(t *testing.T) {
	material := Material{TrustDomain: "local", Workspace: "/work/repo", Profile: "gopls", EnvironmentReference: "developer-shell", OptionsDigest: "sha256:options"}
	first := Derive(material)
	second := Derive(material)
	if first != second {
		t.Fatal("ASSERT_SESSION_KEY_DETERMINISTIC: equal identity material produced different keys")
	}
	changed := material
	changed.Profile = "rust-analyzer"
	if first == Derive(changed) {
		t.Fatal("ASSERT_SESSION_KEY_COMPONENT_IDENTITY: profile change did not change key")
	}
}

func TestKeyDoesNotExposeIdentityMaterial(t *testing.T) {
	secret := "raw-secret-value"
	key := Derive(Material{TrustDomain: "local", Workspace: "/work/repo", Profile: "gopls", EnvironmentReference: "env:" + secret})
	text, err := key.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(key.String(), secret) || bytes.Contains(text, []byte(secret)) {
		t.Fatal("ASSERT_SESSION_KEY_SECRET_FREE: key representation exposed input material")
	}
	if !strings.HasPrefix(key.String(), "sk1:") {
		t.Fatalf("ASSERT_SESSION_KEY_VERSIONED: got %q", key.String())
	}
}
