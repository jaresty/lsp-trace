package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"lsp-trace/internal/strictjson"
)

func decodeCanonicalTypedV2[T any](raw []byte, size func(T) (uint64, error)) (T, error) {
	var zero, value T
	if len(raw) == 0 || uint64(len(raw)) > MaxArtifactCanonicalBytesV2 || !utf8.Valid(raw) {
		return zero, fmt.Errorf("v2 artifact size/UTF-8 refusal")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return zero, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return zero, err
	}
	n, err := size(value)
	canonical, err := marshalSizedV2(value, n, err)
	if err != nil || !bytes.Equal(raw, canonical) {
		return zero, fmt.Errorf("noncanonical v2 artifact")
	}
	return value, nil
}

func checkArtifactBindingV2(b ArtifactBindingDTO) error {
	if b.SchemaVersion == "" || !validDigest(b.Identity) || !validDigest(b.SHA256) || b.ByteLength == 0 || b.ByteLength > MaxArtifactCanonicalBytesV2 {
		return fmt.Errorf("invalid v2 artifact binding")
	}
	return nil
}

// VerifyRawArtifactBindingV2 verifies bounded populated bytes, independently of
// domain identity. The latter is checked by the exact schema's typed parser.
func VerifyRawArtifactBindingV2(b ArtifactBindingDTO, raw []byte) error {
	if b.SchemaVersion == "" || !validDigest(b.SHA256) || b.ByteLength == 0 || b.ByteLength > MaxArtifactCanonicalBytesV2 || uint64(len(raw)) != b.ByteLength || rawDigest(raw) != b.SHA256 {
		return fmt.Errorf("v2 populated artifact binding mismatch")
	}
	return nil
}

func checkedSumV2(values ...uint64) (uint64, error) {
	var sum uint64
	for _, v := range values {
		if ^uint64(0)-sum < v {
			return 0, fmt.Errorf("v2 accounting overflow")
		}
		sum += v
	}
	return sum, nil
}
func checkedProductV2(a, b uint64) (uint64, error) {
	if a != 0 && b > ^uint64(0)/a {
		return 0, fmt.Errorf("v2 accounting overflow")
	}
	return a * b, nil
}
