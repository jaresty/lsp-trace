//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"os"
)

// ReconcileInventoryReturnsV3 retains only already-returned physical bytes. It
// deliberately does not admit current READY, produce semantic results, recreate
// a transport, retry an exchange, renew a clock, or change any reservation.
func (a *ApplicationJournal) ReconcileInventoryReturnsV3(q InventoryRequestV3, runID string) error {
	j := a.implementation
	if j.config.Schema != "private-host-application-config.v3" || !digestValue(runID) || q.ApprovalSHA256 != InventoryRequestDigestV3(q) || j.head.ApplicationSHA256 != ApplicationConfigDigest(q.Configuration) {
		return ErrInvalid
	}
	key := hash([]byte(j.head.ApplicationSHA256 + "/inventory-v3/" + runID))
	var ledger inventoryLedgerV3
	if e := j.object("inventory-ledger", key, &ledger); e != nil {
		return e
	}
	if e := a.claimInventoryRecoveryV3(key); e != nil {
		return e
	}
	b := ledger.Binding
	if b.RunID != runID || b.ConfigSHA256 != q.ApprovalSHA256 || b.Grant != ApplicationGrantDigest(q.Configuration.Grant) || b.Pins != q.Inventory.Pins {
		return ErrInvalid
	}
	s, e := mailboxStore(b.Mailbox)
	if e != nil {
		return e
	}
	defer s.close()
	identity, e := inventoryRootIdentityV3(s)
	if e != nil || identity != b.RootIdentity {
		return ErrInvalid
	}
	return a.reconcileInventoryRootV3(ledger, s)
}

// Sixteen attempts partition the original V3 128MiB/4096-record metadata
// prepayment: each pass has <=16 claims, <=1MiB aggregate request parts and
// <=16 bounded response prefixes/copies. No expiry or semantic grant is renewed.
func (a *ApplicationJournal) claimInventoryRecoveryV3(key string) error {
	j := a.implementation
	if j.closed || j.poisoned {
		return ErrApplicationAmbiguous
	}
	for n := 0; n < 16; n++ {
		id := hash([]byte(fmt.Sprintf("%s/cost-recovery/%d", key, n)))
		if j.exists("inventory-recovery", id) {
			continue
		}
		return j.commit("INVENTORY_COST_RECOVERY_ATTEMPT_V3", id, j.head, applicationObject{applicationObjectName("inventory-recovery", id), mustJSON(struct {
			Schema, Ledger string
			Attempt        int
		}{"private-inventory-cost-recovery.v3", key, n})})
	}
	return ErrPaused
}

func (a *ApplicationJournal) reconcileInventoryRootV3(ledger inventoryLedgerV3, s *store) error {
	j := a.implementation
	b := ledger.Binding
	identity, e := inventoryRootIdentityV3(s)
	if e != nil || identity != b.RootIdentity {
		return ErrInvalid
	}
	for seq := uint64(1); seq <= 16; seq++ {
		id := inventoryExchangeIDV3(b, seq)
		if !j.exists("inventory-exchange", id) {
			break
		}
		var claim inventoryExchangeV3
		if j.object("inventory-exchange", id, &claim) != nil || claim.Binding != b || claim.Sequence != seq || claim.Reservation.EventID != id {
			return ErrInvalid
		}
		raw, e := s.read(fmt.Sprintf("request-%08d.json", seq), 47999)
		var bundle Bundle
		if e != nil || strict(raw, &bundle) != nil || bundle.Directory != b.Mailbox || bundle.Sequence != seq {
			return ErrInvalid
		}
		frameRaw, e := readPhysicalBundleRoot(bundle, s)
		if e != nil || hash(frameRaw) != claim.FrameSHA256 {
			return ErrInvalid
		}
		f, e := parseInventoryFrameV3(frameRaw)
		idx := 0
		if f.Role == "reviewer" {
			idx = 1
		}
		if e != nil || f.Operation != claim.Operation || f.RequestDigest != claim.RequestSHA256 || f.ConfigSHA256 != b.ConfigSHA256 || f.Grant != b.Grant || f.RunID != b.RunID || f.ApplicationSHA256 != b.ApplicationSHA256 || !inventoryPinsV3(f.Pins, b.Pins) || f.LaunchID != b.Ready[idx].LaunchID || f.PromptSHA256 != b.Ready[idx].PromptSHA256 {
			return ErrInvalid
		}
		reply, e := readPhysicalReplyRoot(s, fmt.Sprintf("reply-%08d.json", seq), true)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil && len(reply) == 0 {
			return e
		}
		if j.exists("inventory-return", id) {
			var old InventoryPhysicalReturnV3
			if j.object("inventory-return", id, &old) != nil || old.Bytes != uint64(len(reply)) || old.ResponseSHA256 != hash(reply) {
				return ErrApplicationAmbiguous
			}
			var copy []byte
			for n := 0; len(copy) < len(reply); n++ {
				part, e := j.read(applicationObjectName(fmt.Sprintf("inventory-response-%d", n), id), 32768)
				if e != nil || len(part) == 0 || len(part) > len(reply)-len(copy) {
					return ErrApplicationAmbiguous
				}
				copy = append(copy, part...)
			}
			if !bytes.Equal(copy, reply) {
				return ErrApplicationAmbiguous
			}
			continue
		}
		if e = a.recordInventoryReturnV3(claim, reply, "RECOVERED_COST_ONLY_NO_REPLAY", 0); e != nil {
			return e
		}
	}
	return nil
}
