//go:build darwin || linux

package censuslineagepilot

import (
	"fmt"
	"os"
	"syscall"
)

func retainedLifecycleNamePrivateV3(id string, g uint64) string {
	return fmt.Sprintf("retained-lifecycle-%s-%020d.json", id[7:], g)
}
func retainedJournalIdentityPrivateV3(l PageLedgerPrivateV3, name string) error {
	info, e := l.state.root.Lstat(name)
	if e != nil {
		return e
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || st.Nlink != 1 {
		return ErrInvalid
	}
	return nil
}

// Lifecycle markers are immutable expected-tip bindings, not a mutable head or
// proof of authenticated history. An older generation refuses even if it was
// never claimed and the caller supplies the ACTUAL newest reservation tip.
func verifyRetainedLifecyclePrivateV3(l PageLedgerPrivateV3, m RetainedPageCheckpointManifestPrivateV3, id string) error {
	name := retainedLifecycleNamePrivateV3(m.LifecycleID, m.Generation)
	if e := retainedJournalIdentityPrivateV3(l, name); e != nil {
		return e
	}
	var marker struct{ Schema, Checkpoint string }
	if e := l.state.readNamed(name, &marker); e != nil || marker.Schema != m.Schema || marker.Checkpoint != id {
		return ErrInvalid
	}
	if _, e := l.state.root.Lstat(retainedLifecycleNamePrivateV3(m.LifecycleID, m.Generation+1)); !os.IsNotExist(e) {
		return ErrInvalid
	}
	if m.Generation > 1 {
		name = retainedLifecycleNamePrivateV3(m.LifecycleID, m.Generation-1)
		if e := retainedJournalIdentityPrivateV3(l, name); e != nil {
			return e
		}
		if e := l.state.readNamed(name, &marker); e != nil || marker.Schema != m.Schema || marker.Checkpoint != m.PreviousCheckpoint {
			return ErrInvalid
		}
	}
	return retainedJournalIdentityPrivateV3(l, retainedAnchorNamePrivateV3(id))
}
