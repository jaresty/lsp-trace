//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInventoryFreshIdentityFrameV3(t *testing.T) {
	const a = "ASSERT_FRESH_IDENTITY_TRANSPORT"
	q, i := freshIdentityFixture(t, "NON_GIT")
	var fresh InventoryRequestV3
	if json.Unmarshal(freshIdentityRawV3(t, q, i), &fresh) != nil {
		t.Fatal("SETUP_FRESH_FRAME")
	}
	admitted, run, j, tr := inventoryProductFixtureV3(t)
	f, _, _ := inventoryProductClaimV3(t, tr)
	f.SourceRevision = fresh.Inventory.SourceRevision
	f.RequestDigest = ""
	f.RequestDigest = hash(mustJSON(f))
	if _, e := parseInventoryFrameV3(mustJSON(f)); e != nil {
		t.Fatal(a, "FAIL fresh outer revision", e)
	}
	// Keep genuine admitted config/grant/pins; change only the source and its
	// self-digest. Publishing data does not make this foreign revision admitted.
	before := j.Head().Reserved
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	done := make(chan struct{})
	raw := mustJSON(f)
	header := cohortFrameHeader{Protocol: f.Protocol, Role: f.Role, RequestDigest: f.RequestDigest, ConfigSHA256: f.ConfigSHA256, Grant: f.Grant, Sequence: 1}
	go func() {
		defer close(done)
		_, _ = exchangePhysicalRoot(ctx, tr.held, tr.Binding.Mailbox, time.Second, header, raw)
	}()
	defer func() { cancel(); <-done }()
	if _, e := WaitInventoryV3(ctx, tr.Binding.Mailbox, "interpreter", 0, admitted, run); e != ErrInvalid {
		t.Fatal(a, "FAIL foreign source Wait", e)
	}
	if e := SubmitInventoryV3(tr.Binding.Mailbox, "interpreter", 1, []byte(`{}`), admitted, run); e != ErrInvalid {
		t.Fatal(a, "FAIL foreign source Submit", e)
	}
	if _, e := os.Stat(filepath.Join(tr.Binding.Mailbox, "reply-00000001.json")); !os.IsNotExist(e) || j.Head().Reserved != before {
		t.Fatal(a, "FAIL foreign source effects", e)
	}
	f.SourceRevision = "inventory-source.v1/sha256:invalid"
	f.RequestDigest = ""
	f.RequestDigest = hash(mustJSON(f))
	if _, e := parseInventoryFrameV3(mustJSON(f)); e == nil {
		t.Fatal(a, "FAIL malformed fresh revision")
	}
	t.Log(a + ": PASS")
}
