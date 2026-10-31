//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const censusTerminalSegmentStatusPrivate = "TERMINAL_ACCOUNTED_GAP_NOT_NATIVE_DELIVERY"
const censusComputedSegmentGapPrivate = "COMPUTED_NOT_NATIVE_DELIVERED_SEGMENT_GAP"

type censusTerminalRetentionPrivate struct {
	Reason string
	Copies [2]pilot.ProducerPacketCopyReceiptPrivateV2
}

func censusTerminalEnabledPrivate(c censusInventoryConfigPrivate) bool {
	return c.Application != nil && c.Application.Profile == censusApplicationTerminalProfilePrivate
}
func censusTerminalOriginStatusPrivate(status string) bool {
	return status == pilot.TerminalTargetCapPrivateV2 || status == pilot.TerminalNoTargetsPrivateV2
}
func censusTerminalCleanupPrivate(origin censusInventoryOriginPrivate) bool {
	c := origin.Cleanup
	return c != nil && origin.StopIntentID != "" && c.ResourceClose == managedprocess.ResourcesClosed && c.Reap == managedprocess.ReapComplete && (c.Death == managedprocess.DeathExited || c.Death == managedprocess.DeathSignaled) && (c.RetiredTransportReused || (c.ShutdownWrite == sessionruntime.CleanupSucceededPrivateV2 && c.ShutdownRead == sessionruntime.ShutdownMatchedPrivateV2 && (c.ExitWrite == sessionruntime.CleanupSucceededPrivateV2 || c.Death == managedprocess.DeathExited)))
}
func (s *censusApplicationProgressPrivate) verifyTerminalOriginPrivate(attempt string, j int, origin censusInventoryOriginPrivate, manifest []byte) error {
	if !censusTerminalEnabledPrivate(s.config) || origin.Terminal == nil || origin.Terminal.Reason != origin.Status || !censusTerminalOriginStatusPrivate(origin.Status) || len(origin.Batches) != 0 || origin.ProducerCopies[0].Receipt != nil || origin.ProducerCopies[1].Receipt != nil {
		return errors.New("application terminal origin binding")
	}
	copies := origin.Terminal.Copies
	for _, copy := range copies {
		if copy.Receipt == nil || copy.Outcome != "NAMESPACE_COMMITTED" || copy.Receipt.VerificationStatus != "VERIFIED" || !copy.Receipt.NamespaceAtomic || copy.Receipt.Mechanism != publication.BoundFileMechanism || copy.Receipt.DirectorySyncStatus != publication.DirectorySyncComplete || copy.Receipt.CrashDurability != publication.DirectorySyncComplete || copy.Receipt.CloseStatus != publication.CloseComplete {
			return errors.New("application terminal receipt missing")
		}
	}
	first, other := copies[0].Receipt, copies[1].Receipt
	if first.Digest != other.Digest || first.ByteLength != other.ByteLength || first.FinalSelector != first.Digest || other.FinalSelector != first.Digest || first.ByteLength > 1<<20 {
		return errors.New("application terminal pair identity")
	}
	raw, err := s.pair(attempt, fmt.Sprintf("terminal-%d", j), first.Digest, first.ByteLength)
	if err != nil {
		return err
	}
	v, err := pilot.InspectTerminalDiscoveryPrivateV2(raw)
	if err != nil {
		return err
	}
	discovery, _ := json.Marshal(v.Discovery)
	policy, _ := json.Marshal(s.plan.Configuration.Policy)
	if !bytes.Equal(v.FrozenManifest, manifest) || v.ScopeSHA256 != origin.ScopeSHA256 || v.Status != origin.Status || !bytes.Equal(discovery, origin.Discovery) || !bytes.Equal(v.EffectivePolicy, policy) || v.Discovery.Session != origin.Session || v.Discovery.Workspace != s.options.Workspace || v.Discovery.Accounting.FileDenominator != len(origin.Files) {
		return errors.New("application terminal original source/discovery mismatch")
	}
	return nil
}
