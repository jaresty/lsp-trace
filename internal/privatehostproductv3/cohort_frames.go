//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type cohortFrameHeader struct {
	Protocol, Role, RequestDigest, ConfigSHA256, Grant string
	Sequence                                           int
}

func parseCohortFrame(raw []byte) (cohortFrameHeader, error) {
	var h cohortFrameHeader
	if len(raw) > 1<<20 || json.Unmarshal(raw, &h) != nil || h.Sequence < 1 || h.Sequence > 16 || (h.Role != "interpreter" && h.Role != "reviewer") {
		return h, ErrInvalid
	}
	switch h.Protocol {
	case CohortProtocol:
		var t CohortTask
		if strict(raw, &t) != nil {
			return h, ErrInvalid
		}
		digest := t.RequestDigest
		t.RequestDigest = ""
		if hash(mustJSON(t)) != digest {
			return h, ErrInvalid
		}
	case CohortNavigationProtocol:
		var t CohortNavigationTask
		if strict(raw, &t) != nil {
			return h, ErrInvalid
		}
		digest := t.RequestDigest
		t.RequestDigest = ""
		if hash(mustJSON(t)) != digest {
			return h, ErrInvalid
		}
	default:
		return h, ErrInvalid
	}
	return h, nil
}
func ReadCohortNavigationBundle(b Bundle) (CohortNavigationTask, error) {
	s, e := mailboxStore(b.Directory)
	if e != nil {
		return CohortNavigationTask{}, e
	}
	defer s.close()
	return readCohortNavigationBundleRoot(b, s)
}
func readCohortNavigationBundleRoot(b Bundle, s *store) (CohortNavigationTask, error) {
	var t CohortNavigationTask
	if b.Protocol != CohortNavigationProtocol || b.Event != "NeedsHost" || b.Bytes < 1 || b.Bytes > 1<<20 || len(b.Parts) < 1 || len(b.Parts) > 128 {
		return t, ErrInvalid
	}
	receipt := b.ReceiptSHA256
	b.ReceiptSHA256 = ""
	if hash(mustJSON(b)) != receipt {
		return t, ErrInvalid
	}
	b.ReceiptSHA256 = receipt
	registered, e := s.read(fmt.Sprintf("request-%08d.json", b.Sequence), 47999)
	if e != nil || !bytes.Equal(registered, mustJSON(b)) {
		return t, ErrInvalid
	}
	var raw []byte
	for n, p := range b.Parts {
		if p.Path != filepath.Join(b.Directory, fmt.Sprintf("part-%08d-%04d.json", b.Sequence, n)) || p.Bytes < 1 || p.Bytes > 20000 {
			return t, ErrInvalid
		}
		data, e := s.read(filepath.Base(p.Path), 20000)
		if e != nil || len(data) != p.Bytes || hash(data) != p.SHA256 || len(raw) > b.Bytes-len(data) {
			return t, ErrInvalid
		}
		raw = append(raw, data...)
	}
	h, e := parseCohortFrame(raw)
	if e != nil || len(raw) != b.Bytes || h.Role != b.Role || uint64(h.Sequence) != b.Sequence || h.RequestDigest != b.RequestDigest || strict(raw, &t) != nil {
		return t, ErrInvalid
	}
	return t, nil
}
