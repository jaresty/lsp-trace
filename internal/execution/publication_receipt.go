package execution

import "lsp-trace/internal/verification"

// A file sync performed by Publisher is insufficient for CHECKED. Only issue
// the receipt after the pinned artifact parent directory has actually synced.
func receiptForPublishedArtifact(data []byte, syncDirectory func() (bool, error)) ([]byte, error) {
	checked, err := syncDirectory()
	if err != nil {
		return nil, err
	}
	durability := verification.DirectoryDurabilityUnavailable
	if checked {
		durability = verification.DirectoryDurabilityChecked
	}
	return verification.ReceiptBytes(data, durability)
}
