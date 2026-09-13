package captureset

import (
	"encoding/binary"
	"strings"
	"testing"
)

func fixtureBundle(t *testing.T) ([]byte, ExactBytesAuthority) {
	t.Helper()
	m, exact, authority := transactionalFixture(t)
	manifestRaw, err := EncodeCanonical(m)
	if err != nil {
		t.Fatal(err)
	}
	bySelector := make(map[string][]byte, len(exact))
	for _, raw := range exact {
		c, err := authority.Constituent(raw)
		if err != nil {
			t.Fatal(err)
		}
		bySelector[c.ImmutableSelector] = raw
	}
	bundle, err := encodePrivateBundle(m, manifestRaw, bySelector)
	if err != nil {
		t.Fatal(err)
	}
	return bundle, authority
}

func TestPrivateBundleRejectsTruncationDigestMismatchAndTrailingBytes(t *testing.T) {
	bundle, authority := fixtureBundle(t)
	for _, cut := range []int{0, 1, len(bundle) - 1} {
		if _, err := decodePrivateBundle(bundle[:cut], authority); err == nil {
			t.Fatalf("accepted truncation %d", cut)
		}
	}
	mutated := append([]byte(nil), bundle...)
	mutated[len(mutated)-1] ^= 1
	if _, err := decodePrivateBundle(mutated, authority); err == nil {
		t.Fatal("accepted constituent digest mismatch")
	}
	trailing := append(append([]byte(nil), bundle...), 0)
	if _, err := decodePrivateBundle(trailing, authority); err == nil {
		t.Fatal("accepted trailing bytes")
	}
}

func TestPrivateBundleRejectsDuplicateSelector(t *testing.T) {
	bundle, authority := fixtureBundle(t)
	manifestLen := int(binary.BigEndian.Uint64(bundle[len(bundleMagic):]))
	off := len(bundleMagic) + 8 + manifestLen + 4
	firstLen := int(binary.BigEndian.Uint16(bundle[off:]))
	first := append([]byte(nil), bundle[off+2:off+2+firstLen]...)
	off += 2 + firstLen
	firstBytesLen := int(binary.BigEndian.Uint64(bundle[off:]))
	off += 8 + 32 + firstBytesLen
	secondLen := int(binary.BigEndian.Uint16(bundle[off:]))
	if secondLen != firstLen {
		t.Fatal("fixture selector lengths differ")
	}
	copy(bundle[off+2:off+2+secondLen], first)
	if _, err := decodePrivateBundle(bundle, authority); err == nil || !strings.Contains(err.Error(), "duplicate bundle selector") {
		t.Fatalf("duplicate selector: %v", err)
	}
}

func TestCaptureSetSelectorRejectsUnicodeAndTraversal(t *testing.T) {
	for _, selector := range []string{"../capture.bundle", CaptureSetSelectorPrefix + strings.Repeat("é", 32) + ".bundle", CaptureSetSelectorPrefix + strings.Repeat("a", 64) + "/../x.bundle"} {
		if err := ValidatePublicationSelector(selector); err == nil {
			t.Fatalf("accepted unsafe selector %q", selector)
		}
	}
}
