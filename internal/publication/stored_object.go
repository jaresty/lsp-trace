package publication

import (
	"bytes"
	"encoding/hex"
	"errors"
)

// StoredObject is a lookup reference inside an application-owned Root, not an
// import credential. The caller reserves bounded read/validation costs first.
type StoredObject struct {
	ID    string
	Bytes int
}

func (v StoredObject) Valid() bool {
	b, e := hex.DecodeString(v.ID)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == v.ID && v.Bytes > 0 && v.Bytes <= 8<<20
}
func StoreObject(root *Root, prefix string, raw []byte) (StoredObject, error) {
	v := StoredObject{DigestForTest(raw)[7:], len(raw)}
	if !v.Valid() {
		return StoredObject{}, errors.New("invalid stored object")
	}
	name := prefix + v.ID + ".json"
	if got, e := ReadVerifiedBoundFileChecked(root, name, int64(v.Bytes)); e == nil && bytes.Equal(got, raw) {
		return v, nil
	}
	r, e := PublishBoundFile(root, name, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errors.New("stored object differs")
		}
		return nil
	})
	if e != nil || r == nil || !r.NamespaceAtomic || r.VerificationStatus != "VERIFIED" || r.DirectorySyncStatus != DirectorySyncComplete || r.CloseStatus != CloseComplete {
		return StoredObject{}, errors.Join(errors.New("stored object unverified"), e)
	}
	got, e := ReadObject(root, prefix, v)
	if e != nil || !bytes.Equal(got, raw) {
		return StoredObject{}, errors.New("stored object readback differs")
	}
	return v, nil
}
func ReadObject(root *Root, prefix string, v StoredObject) ([]byte, error) {
	if !v.Valid() {
		return nil, errors.New("invalid stored reference")
	}
	raw, e := ReadVerifiedBoundFileChecked(root, prefix+v.ID+".json", int64(v.Bytes))
	if e != nil || len(raw) != v.Bytes || DigestForTest(raw) != "sha256:"+v.ID {
		return nil, errors.Join(errors.New("stored object unavailable"), e)
	}
	return raw, nil
}
