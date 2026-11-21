package sessionruntime

// PrivateDiscoverySuppliedDocuments is scalar metadata indexed by the frozen
// construction's member order. It contains no URI, content, runtime pointer,
// native proof, approval, or execution permission. Availability is not authority.
// A missing hash is not a zero-valued hash: the complete result stays unavailable.
type PrivateDiscoverySuppliedDocuments struct {
	Available bool
	Count     uint8
	SHA256    [16][32]byte
}

// Eight detached scalar copies and eight bounded JSON representations. This is
// logical metadata accounting, not another source read/hash reservation.
const privateDiscoverySuppliedDocumentsCharge = uint64(8*(2+16*32) + 8*4096)

func (w *privateDiscoveryWindow) prepaySuppliedDocuments() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reserveLocked(PrivateDiscoveryAccounting, privateDiscoveryCharge{&w.remaining.RetainedBytes, privateDiscoverySuppliedDocumentsCharge}, privateDiscoveryCharge{&w.remaining.Work, 32 * privateDiscoverySuppliedDocumentsCharge})
}

// Called only after finalize has closed the session. The successful supplied
// result's retained hash is the sole source; no Manager map or filesystem lookup.
func (s *PrivateDiscoverySession) suppliedDocumentsSnapshot() PrivateDiscoverySuppliedDocuments {
	if s == nil {
		return PrivateDiscoverySuppliedDocuments{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed || len(s.members) == 0 || len(s.members) > 16 {
		return PrivateDiscoverySuppliedDocuments{}
	}
	var out PrivateDiscoverySuppliedDocuments
	for i, f := range s.members {
		if !s.supplied[f.URI] || !s.digestAvailable[i] {
			return PrivateDiscoverySuppliedDocuments{}
		}
		out.SHA256[i] = s.digests[i]
	}
	out.Available = true
	out.Count = uint8(len(s.members))
	return out
}
