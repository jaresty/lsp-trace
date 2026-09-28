package adr0011acquisition

import (
	"bytes"
	"errors"
	"strings"

	"lsp-trace/internal/publication"
)

const privateBodyPublicationLimit = 4194304

var errPrivateBodyPublication = errors.New("private body publication failed")

type privateBodyPublication struct {
	selector, digest string
	byteCount        int
	stage            string // ABSENT, COMMITTED_UNVERIFIED, VERIFIED
}

// publishPrivateBody publishes only the complete original body held by a slice.
// read is per-call injection for synthetic testing; nil uses the custody reader.
func publishPrivateBody(root *publication.Root, kind string, slice privateBodySlice, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	result := privateBodyPublication{stage: "ABSENT"}
	if root == nil || (kind != "write" && kind != "read") || len(slice.body) == 0 || len(slice.body) > privateBodyPublicationLimit || slice.bodyDigest != privateDigest(slice.body) || slice.offset < 0 || slice.length < 0 || slice.offset > len(slice.body) || slice.length > len(slice.body)-slice.offset || !bytes.Equal(slice.value, slice.body[slice.offset:slice.offset+slice.length]) || slice.valueDigest != privateDigest(slice.value) {
		return result, errPrivateBodyPublication
	}
	selector := "adr0011-references-" + kind + "-v1-" + strings.TrimPrefix(slice.bodyDigest, "sha256:") + ".bin"
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	receipt, err := publication.PublishBoundFile(root, selector, slice.body, func(got []byte) error {
		if !bytes.Equal(got, slice.body) || privateDigest(got) != slice.bodyDigest {
			return errPrivateBodyPublication
		}
		return nil
	})
	if err != nil || receipt == nil {
		return result, errPrivateBodyPublication
	}
	result = privateBodyPublication{selector: selector, digest: slice.bodyDigest, byteCount: len(slice.body), stage: "COMMITTED_UNVERIFIED"}
	if receipt.VerificationStatus != "VERIFIED" || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.FinalSelector != selector || receipt.Digest != result.digest || receipt.ByteLength != uint64(result.byteCount) || !receipt.NamespaceAtomic {
		return result, errPrivateBodyPublication
	}
	got, err := read(root, selector, privateBodyPublicationLimit)
	if err != nil || !bytes.Equal(got, slice.body) || privateDigest(got) != slice.bodyDigest {
		return result, errPrivateBodyPublication
	}
	result.stage = "VERIFIED"
	return result, nil
}
