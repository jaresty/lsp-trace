//go:build darwin || linux

package applicationinventory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDurableManifestClosedMaximum(t *testing.T) {
	m := PublicationManifestV3{Version: "lsp-trace.application-inventory-publication.v3", Status: "PROVISIONAL", Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", CallerBytes: 64 << 20, SemanticReservedNS: int64(120 * time.Second)}
	for _, name := range []string{"catalog-base.json", "catalog-proposed.json", "catalog-corrected.json", "report.json"} {
		m.Artifacts = append(m.Artifacts, PublishedArtifactV3{Name: name, SHA256: "sha256:" + strings.Repeat("f", 64), Bytes: 8 << 20})
	}
	raw, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	if len(raw) > 2048 {
		t.Fatalf("closed durable manifest exceeds2048: %d", len(raw))
	}
	t.Logf("maximum closed manifest=%d bound=2048", len(raw))
}
