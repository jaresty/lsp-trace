//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Private concrete IPC capability, not a caller-selected generic recovery path.
// Same-user records remain observations, not authentication or original owners.
type cohortMailboxCapability interface{ cohortRecoveryMailbox() (CohortMailbox, bool) }

func (m CohortMailbox) cohortRecoveryMailbox() (CohortMailbox, bool) { return m, true }
func (b cohortReservedBackend) cohortRecoveryMailbox() (CohortMailbox, bool) {
	if v, ok := b.backend.(cohortMailboxCapability); ok {
		return v.cohortRecoveryMailbox()
	}
	return CohortMailbox{}, false
}

type cohortMailboxPin struct {
	ConfigSHA256, Directory string
	Device, Inode           uint64
	ReadySHA256             [2]string
}

const cohortMailboxPinName = "host/state-00000001.json"

func mailboxPin(c CohortConfig, m CohortMailbox) (cohortMailboxPin, error) {
	s, e := mailboxStore(m.Directory)
	if e != nil {
		return cohortMailboxPin{}, e
	}
	defer s.close()
	return mailboxPinRoot(c, m.Directory, s)
}

// The caller owns this root. Never reopen Directory after validating it.
func mailboxPinRoot(c CohortConfig, directory string, s *store) (cohortMailboxPin, error) {
	p := cohortMailboxPin{ConfigSHA256: CohortConfigDigest(c), Directory: directory}
	related := func(a, b string) bool {
		rel, e := filepath.Rel(a, b)
		return e == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
	}
	if related(c.Store, directory) || related(directory, c.Store) {
		return p, ErrInvalid
	}
	info, e := s.root.Stat(".")
	if e != nil {
		return p, e
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return p, ErrInvalid
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return p, ErrInvalid
	}
	p.Device = uint64(stat.Dev)
	p.Inode = stat.Ino
	ids := map[string]bool{}
	for n, role := range []string{"interpreter", "reviewer"} {
		raw, e := s.read("ready-"+role+".json", 4096)
		var ready Ready
		if e != nil || strict(raw, &ready) != nil || ready.Protocol != CohortProtocol || ready.Role != role || ready.LaunchID == "" || len(ready.LaunchID) > 256 || ids[ready.LaunchID] || ready.PromptSHA256 != c.RolePromptSHA256[n] {
			return p, ErrInvalid
		}
		ids[ready.LaunchID] = true
		p.ReadySHA256[n] = hash(raw)
	}
	return p, nil
}

type cohortPinnedMailbox struct {
	root   *store
	pin    cohortMailboxPin
	config CohortConfig
}
type cohortPinnedMailboxKey struct{}

func pinCohortMailbox(s *store, c CohortConfig, b CohortBackend) (*cohortPinnedMailbox, error) {
	cap, ok := b.(cohortMailboxCapability)
	if !ok {
		return nil, nil
	}
	m, ok := cap.cohortRecoveryMailbox()
	if !ok {
		return nil, nil
	}
	root, e := mailboxStore(m.Directory)
	if e != nil {
		return nil, e
	}
	pin, e := mailboxPinRoot(c, m.Directory, root)
	if e == nil {
		e = s.write(cohortMailboxPinName, mustJSON(pin))
	}
	if e != nil {
		root.close()
		return nil, e
	}
	return &cohortPinnedMailbox{root, pin, c}, nil
}

// Observe the exact bounded frame before semantic parsing, reconciling two
// copies of ONE event. A disagreement is not license to choose cheaper bytes.
func observedCohortReply(s *store, c CohortConfig, seq int, claim []byte) ([]byte, error) {
	host, he := s.read(fmt.Sprintf("host/response-%08d.json", seq), (64<<10)+1)
	if he != nil && !os.IsNotExist(he) {
		return nil, he
	}
	pinRaw, pe := s.read(cohortMailboxPinName, 16384)
	if os.IsNotExist(pe) {
		return host, he
	}
	var pin cohortMailboxPin
	if pe != nil || strict(pinRaw, &pin) != nil || pin.ConfigSHA256 != CohortConfigDigest(c) {
		return nil, ErrInvalid
	}
	m, e := mailboxStore(pin.Directory)
	if e != nil {
		return nil, e
	}
	defer m.close()
	current, e := mailboxPinRoot(c, pin.Directory, m)
	if e != nil || !bytes.Equal(mustJSON(current), pinRaw) {
		return nil, ErrInvalid
	}
	return observedCohortReplyRoot(s, c, seq, claim, host, he, pin, m)
}

func observedCohortReplyRoot(s *store, c CohortConfig, seq int, claim, host []byte, he error, pin cohortMailboxPin, m *store) ([]byte, error) {
	frame, e := parseCohortFrame(claim)
	if e != nil || frame.Sequence != seq || frame.ConfigSHA256 != CohortConfigDigest(c) || frame.Grant != c.Grant {
		return nil, ErrInvalid
	}
	bundleRaw, e := m.read(fmt.Sprintf("request-%08d.json", seq), 47999)
	// A coordinator can die before disclosure; no mailbox request is then valid.
	if os.IsNotExist(e) {
		if _, re := m.read(fmt.Sprintf("reply-%08d.json", seq), (64<<10)+1); !os.IsNotExist(re) {
			return nil, ErrInvalid
		}
		return host, he
	}
	var bundle Bundle
	if e != nil || strict(bundleRaw, &bundle) != nil || bundle.Directory != pin.Directory || bundle.Sequence != uint64(seq) || bundle.Role != frame.Role || bundle.Protocol != frame.Protocol || bundle.RequestDigest != frame.RequestDigest {
		return nil, ErrInvalid
	}
	var request []byte
	if frame.Protocol == CohortProtocol {
		t, e := readCohortBundleRoot(bundle, m)
		if e != nil {
			return nil, e
		}
		request = mustJSON(t)
	} else {
		t, e := readCohortNavigationBundleRoot(bundle, m)
		if e != nil {
			return nil, e
		}
		request = mustJSON(t)
	}
	if !bytes.Equal(request, claim) {
		return nil, ErrInvalid
	}
	reply, re := m.read(fmt.Sprintf("reply-%08d.json", seq), (64<<10)+1)
	if re != nil && !os.IsNotExist(re) {
		return nil, re
	}
	if re == nil && he == nil && !bytes.Equal(reply, host) {
		return nil, ErrInvalid
	}
	if re == nil {
		receiptRaw, ce := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
		if ce == nil {
			var receipt cohortReceipt
			if strict(receiptRaw, &receipt) != nil || receipt.Sequence != seq || receipt.Bytes != len(reply) || receipt.ResponseSHA256 != hash(reply) {
				return nil, ErrInvalid
			}
		} else if !os.IsNotExist(ce) {
			return nil, ce
		}
		return reply, nil
	}
	return host, he
}
