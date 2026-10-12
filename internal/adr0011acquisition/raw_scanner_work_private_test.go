package adr0011acquisition

import (
	"errors"
	"testing"
)

func TestPrivateWorkBytesScannedChargesMalformedTraversalBeforeFailure(t *testing.T) {
	identity := "occurrence-malformed-replay"
	var charged uint64
	work := rawScannerWorkContext{
		occurrenceIdentity: identity,
		chargeBytesScanned: func(gotIdentity string, amount uint64) error {
			if gotIdentity != identity {
				t.Fatalf("charged identity = %q, want %q", gotIdentity, identity)
			}
			charged += amount
			return nil
		},
	}
	malformed := []byte(`[1,] trailing`)

	first, err := scanRawTopLevelWithWork(malformed, work)
	if !errors.Is(err, errRawScanner) {
		t.Fatalf("first malformed traversal error = %v, want %v", err, errRawScanner)
	}
	if first.form != "MALFORMED" || first.workUnits != len(malformed) {
		t.Fatalf("first malformed traversal = %#v, want drained %d bytes", first, len(malformed))
	}
	if charged != uint64(len(malformed)) {
		t.Fatalf("first bytes_scanned = %d, want %d", charged, len(malformed))
	}

	second, err := scanRawTopLevelWithWork(malformed, work)
	if !errors.Is(err, errRawScanner) {
		t.Fatalf("replayed malformed traversal error = %v, want %v", err, errRawScanner)
	}
	if second.form != "MALFORMED" || second.workUnits != len(malformed) {
		t.Fatalf("replayed malformed traversal = %#v, want drained %d bytes", second, len(malformed))
	}
	if want := uint64(2 * len(malformed)); charged != want {
		t.Fatalf("replayed bytes_scanned = %d, want occurrence work %d", charged, want)
	}
}
