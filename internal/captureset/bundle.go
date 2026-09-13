package captureset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	bundleMagic           = "LSPCSB01"
	MaxBundleConstituents = 1024
	MaxBundleBytes        = 64 << 20
)

type privateBundle struct {
	Manifest    Manifest
	ManifestRaw []byte
	BySelector  map[string][]byte
}

func encodePrivateBundle(m Manifest, manifestRaw []byte, bySelector map[string][]byte) ([]byte, error) {
	if len(m.Constituents) < 1 || len(m.Constituents) > MaxBundleConstituents {
		return nil, errors.New("bundle constituent count outside bounds")
	}
	var b bytes.Buffer
	b.WriteString(bundleMagic)
	_ = binary.Write(&b, binary.BigEndian, uint64(len(manifestRaw)))
	b.Write(manifestRaw)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(m.Constituents)))
	for _, c := range m.Constituents {
		raw, ok := bySelector[c.ImmutableSelector]
		if !ok {
			return nil, errors.New("manifest constituent association mismatch")
		}
		if len(c.ImmutableSelector) > 1<<16-1 {
			return nil, errors.New("constituent selector too long")
		}
		_ = binary.Write(&b, binary.BigEndian, uint16(len(c.ImmutableSelector)))
		b.WriteString(c.ImmutableSelector)
		_ = binary.Write(&b, binary.BigEndian, uint64(len(raw)))
		digest, err := hex.DecodeString(c.SHA256[len("sha256:"):])
		if err != nil || len(digest) != sha256.Size {
			return nil, errors.New("invalid constituent digest")
		}
		b.Write(digest)
		b.Write(raw)
		if b.Len() > MaxBundleBytes {
			return nil, errors.New("bundle byte size outside bounds")
		}
	}
	return b.Bytes(), nil
}

func decodePrivateBundle(raw []byte, authority ExactBytesAuthority) (privateBundle, error) {
	if len(raw) > MaxBundleBytes {
		return privateBundle{}, errors.New("bundle byte size outside bounds")
	}
	r := bytes.NewReader(raw)
	magic := make([]byte, len(bundleMagic))
	if _, err := io.ReadFull(r, magic); err != nil || string(magic) != bundleMagic {
		return privateBundle{}, errors.New("malformed bundle magic")
	}
	var manifestLen uint64
	if binary.Read(r, binary.BigEndian, &manifestLen) != nil || manifestLen > uint64(r.Len()) || manifestLen > MaxBundleBytes {
		return privateBundle{}, errors.New("truncated bundle manifest")
	}
	manifestRaw := make([]byte, int(manifestLen))
	if _, err := io.ReadFull(r, manifestRaw); err != nil {
		return privateBundle{}, errors.New("truncated bundle manifest")
	}
	m, err := Decode(manifestRaw)
	if err != nil {
		return privateBundle{}, err
	}
	canonical, err := EncodeCanonical(m)
	if err != nil || !bytes.Equal(canonical, manifestRaw) {
		return privateBundle{}, errors.New("non-canonical bundle manifest")
	}
	var count uint32
	if binary.Read(r, binary.BigEndian, &count) != nil || count < 1 || count > MaxBundleConstituents || int(count) != len(m.Constituents) {
		return privateBundle{}, errors.New("bundle constituent count mismatch")
	}
	bySelector := make(map[string][]byte, count)
	byNativeID := make(map[string]bool, count)
	for i := uint32(0); i < count; i++ {
		var selectorLen uint16
		if binary.Read(r, binary.BigEndian, &selectorLen) != nil || selectorLen == 0 || int(selectorLen) > r.Len() {
			return privateBundle{}, errors.New("truncated bundle selector")
		}
		s := make([]byte, selectorLen)
		if _, err := io.ReadFull(r, s); err != nil {
			return privateBundle{}, errors.New("truncated bundle selector")
		}
		selector := string(s)
		if _, duplicate := bySelector[selector]; duplicate {
			return privateBundle{}, errors.New("duplicate bundle selector")
		}
		var n uint64
		if binary.Read(r, binary.BigEndian, &n) != nil || n > uint64(r.Len()) || n > MaxBundleBytes {
			return privateBundle{}, errors.New("truncated bundle constituent")
		}
		digest := make([]byte, sha256.Size)
		if _, err := io.ReadFull(r, digest); err != nil || n > uint64(r.Len()) {
			return privateBundle{}, errors.New("truncated bundle constituent")
		}
		constituentRaw := make([]byte, int(n))
		if _, err := io.ReadFull(r, constituentRaw); err != nil {
			return privateBundle{}, errors.New("truncated bundle constituent")
		}
		sum := sha256.Sum256(constituentRaw)
		if !bytes.Equal(digest, sum[:]) {
			return privateBundle{}, errors.New("bundle constituent digest mismatch")
		}
		if i >= uint32(len(m.Constituents)) || m.Constituents[i].ImmutableSelector != selector {
			return privateBundle{}, errors.New("bundle manifest constituent order mismatch")
		}
		c := m.Constituents[i]
		if byNativeID[c.NativeV5Identity] {
			return privateBundle{}, errors.New("duplicate bundle native identity")
		}
		byNativeID[c.NativeV5Identity] = true
		if authority.AdmitGraphProvenanceV5 != nil {
			if err := authority.VerifyConstituent(c, constituentRaw); err != nil {
				return privateBundle{}, fmt.Errorf("bundle constituent admission: %w", err)
			}
		} else if c.SchemaID != NativeV5SchemaID || c.ByteLength != len(constituentRaw) || c.SHA256 != rawDigest(constituentRaw) || c.ImmutableSelector != ConstituentSelectorPrefix+strings.TrimPrefix(c.SHA256, "sha256:") || c.NativeV5Identity == "" {
			return privateBundle{}, errors.New("bundle constituent metadata mismatch")
		}
		bySelector[selector] = constituentRaw
	}
	if r.Len() != 0 {
		return privateBundle{}, errors.New("trailing bundle bytes")
	}
	return privateBundle{Manifest: m, ManifestRaw: manifestRaw, BySelector: bySelector}, nil
}
