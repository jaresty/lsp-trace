//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// WithOwnedProducerPacketCodecPrivateV2 keeps the original nondefault process
// metadata intervention before capture, not a post-seal DTO mutation.
func WithOwnedProducerPacketCodecPrivateV2(t *testing.T, fn func(DerivedProducerPacketPrivateV2, [][]byte, func() int)) {
	withProducerPacketCodecPrivateV2(t, "owned-v2-codec-provider", fn)
}

// WithOwnedProducerPacketPrivateV2 exists ONLY in the package test build. It is
// not a production proof factory or arbitrary DTO constructor. Roots and source
// protection remain borrowed/alive during fn; Manager is already shut down.
func WithOwnedProducerPacketPrivateV2(t *testing.T, nom, con int, fn func(DerivedProducerPacketPrivateV2, func() int)) {
	t.Helper()
	withOwnedProducerPacketConfiguredPrivateV2(t, nom, con, false, false, fn)
}

// WithOwnedMatchedProducerPacketPrivateV2 is TEST ONLY. Names, ranges and source
// hashes are generated together BEFORE capture. Removing live owned files after
// derive demonstrates retained-byte preparation without removing the root cap.
func WithOwnedMatchedProducerPacketPrivateV2(t *testing.T, nom, con int, removeLiveFiles bool, fn func(DerivedProducerPacketPrivateV2, func() int)) {
	t.Helper()
	withOwnedProducerPacketConfiguredPrivateV2(t, nom, con, true, removeLiveFiles, fn)
}
func withOwnedProducerPacketConfiguredPrivateV2(t *testing.T, nom, con int, matched, removeLiveFiles bool, fn func(DerivedProducerPacketPrivateV2, func() int)) {
	t.Helper()
	f, o, captures, traffic := packetFixtureConfiguredPrivateV2(t, nom, con, matched, "owned-v2-fake-provider")
	before := traffic()
	roots, _, _ := packetRootsPrivateV2(t, f)
	retained, err := RetainCompleteScopePrivateV2(context.Background(), f, o, captures, roots)
	if err != nil {
		t.Fatal("retain owned V2 packet", err)
	}
	qualified, err := QualifyProducerPacketPrivateV2(context.Background(), retained.VerifiedPair())
	if err != nil {
		t.Fatal("qualify owned V2 packet", err)
	}
	derived, err := DeriveProducerPacketPrivateV2(context.Background(), qualified)
	if err != nil {
		t.Fatal("derive owned V2 packet", err)
	}
	if removeLiveFiles {
		for _, member := range f.state.spec.Members {
			if err := os.Remove(filepath.Join(f.state.spec.Root, member.CanonicalRelativePath)); err != nil {
				t.Fatal(err)
			}
		}
	}
	fn(derived, traffic)
	if traffic() != before {
		t.Fatal("ASSERT_V2_NO_PROVIDER_AFTER_CAPTURE", before, traffic())
	}
}
