//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
)

// Cost-only reader: a changed READY never authorizes semantic continuation.
// The registered request, exact old directory identity and held root still bind
// physical bytes. A replacement root or conflicting copy is refused.
func applicationPinnedPhysicalReply(s *store, c CohortConfig, seq int, claim []byte) ([]byte, error) {
	pinRaw, e := s.read(cohortMailboxPinName, 16384)
	var pin cohortMailboxPin
	if e != nil || strict(pinRaw, &pin) != nil || pin.ConfigSHA256 != CohortConfigDigest(c) {
		return nil, ErrInvalid
	}
	root, e := mailboxStore(pin.Directory)
	if e != nil {
		return nil, e
	}
	defer root.close()
	st, e := root.root.Stat(".")
	if e != nil {
		return nil, e
	}
	id, ok := st.Sys().(*syscall.Stat_t)
	if !ok || uint64(id.Dev) != pin.Device || id.Ino != pin.Inode {
		return nil, ErrInvalid
	}
	metadata, e := root.read(fmt.Sprintf("request-%08d.json", seq), 47999)
	var bundle Bundle
	if e != nil || strict(metadata, &bundle) != nil || bundle.Directory != pin.Directory || bundle.Sequence != uint64(seq) {
		return nil, ErrInvalid
	}
	task, e := readCohortBundleRoot(bundle, root)
	if e != nil || !bytes.Equal(mustJSON(task), claim) {
		return nil, ErrInvalid
	}
	reply, e := root.read(fmt.Sprintf("reply-%08d.json", seq), (64<<10)+1)
	if e != nil {
		return nil, e
	}
	own, oe := s.read(fmt.Sprintf("host/response-%08d.json", seq), (64<<10)+1)
	if oe == nil && !bytes.Equal(own, reply) {
		return nil, ErrInvalid
	}
	if oe != nil && !os.IsNotExist(oe) {
		return nil, ErrInvalid
	}
	return reply, nil
}

type ApplicationCohortRecovery struct {
	UnitID  string
	Config  CohortConfig
	Mailbox string
}

// At most sixteen already-spent subordinate claims. No source owner is restored,
// no reply semantics adopted, no request redisclosed. Existing own-store and
// concrete mailbox recovery guards remain decisive. Known local observations
// are retained even if exact current READY/source validation is unavailable.
func (a *ApplicationJournal) ReconcileCohortReturns(r ApplicationCohortRecovery) error {
	if ValidateCohortConfig(r.Config) != nil || !digestValue(r.UnitID) {
		return ErrInvalid
	}
	j := a.implementation
	var unit applicationUnitLedger
	if j.object("unit", r.UnitID, &unit) != nil || unit.Binding.ApplicationSHA256 != a.Head().ApplicationSHA256 || unit.Binding.ScopeSHA256 != r.Config.ScopeSHA256 {
		return ErrInvalid
	}
	_, _ = DeliverCohortFailure(r.Config, r.Config.Requested, "APPLICATION_RESTART_NO_REPLAY")
	s, e := openCohortStore(r.Config)
	if e != nil {
		return e
	}
	defer s.close()
	for seq := 1; seq <= r.Config.MaxCalls; seq++ {
		id := applicationExchangeID(a.Head().ApplicationSHA256, r.UnitID, uint64(seq))
		var claim applicationExchange
		if e = j.object("exchange", id, &claim); os.IsNotExist(e) {
			break
		} else if e != nil {
			return e
		}
		if j.exists("return", id) {
			continue
		}
		raw, e := s.read(fmt.Sprintf("claim/claim-%08d.json", seq), 1<<20)
		if e != nil {
			return e
		}
		h, e := parseCohortFrame(raw)
		if e != nil || h.ConfigSHA256 != CohortConfigDigest(r.Config) || h.Grant != r.Config.Grant || h.RequestDigest != claim.RequestSHA256 || uint64(len(raw)) != claim.Reservation.Bounds.RequestBytes {
			return ErrInvalid
		}
		// The concrete guarded reader can reconcile mailbox bytes before cohort host
		// persistence. If current pin/READY validation refuses, only own-store bytes
		// are considered and remain UNKNOWN observations, never exact authority.
		reply, re := observedCohortReply(s, r.Config, seq, raw)
		if re != nil {
			reply, re = applicationPinnedPhysicalReply(s, r.Config, seq, raw)
		}
		if re != nil {
			reply, e = s.read(fmt.Sprintf("host/response-%08d.json", seq), (64<<10)+1)
			if e != nil {
				continue
			}
			receiptRaw, receiptErr := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
			var receipt cohortReceipt
			if receiptErr != nil || strict(receiptRaw, &receipt) != nil || receipt.Sequence != seq || receipt.Bytes != len(reply) || receipt.ResponseSHA256 != hash(reply) {
				continue
			}
		}
		if len(reply) == 0 {
			continue
		}
		if e = a.RecordReturnedBytes(id, ObservedWork{EventID: id, RequestBytes: uint64(len(raw)), ResponseBytes: uint64(len(reply)), ResponseSHA256: hash(reply), Accounting: "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES"}); e != nil {
			return e
		}
	}
	return nil
}
