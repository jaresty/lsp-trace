//go:build darwin || linux

package privatehostproductv3

import "os"

// CheckOriginalFresh is a read-only precheck, not a claim or authority. It rejects
// a spent original attempt BEFORE fresh source bytes/server bytes are acquired.
// The later create-only ClaimOriginal remains the decisive concurrent claim.
func CheckOriginalFresh(c Config) error {
	if ValidateConfig(c) != nil {
		return ErrInvalid
	}
	s, e := openStore(c)
	if e != nil {
		return e
	}
	defer s.close()
	_, e = s.root.Lstat("claim/claim-00000000.json")
	if os.IsNotExist(e) {
		return nil
	}
	return ErrInvalid
}
