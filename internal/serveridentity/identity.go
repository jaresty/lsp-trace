// Package serveridentity measures the executing MCP process without claiming producer authentication.
package serveridentity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

const CustodySelfMeasured = "SELF_MEASURED"

type Identity struct {
	BinaryVersion    string `json:"binary_version"`
	SourceRevision   string `json:"source_revision"`
	Dirty            bool   `json:"dirty"`
	Custody          string `json:"custody"`
	ExecutableSHA256 string `json:"executable_sha256,omitempty"`
	InstanceID       string `json:"instance_id"`
}

func New(version, revision string, dirty bool, executable func() (string, error)) Identity {
	id := Identity{BinaryVersion: version, SourceRevision: revision, Dirty: dirty, Custody: CustodySelfMeasured, InstanceID: randomID()}
	if executable != nil {
		id.ExecutableSHA256 = measure(executable)
	}
	return id
}

func randomID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		panic("server instance entropy unavailable")
	}
	return "si1:" + hex.EncodeToString(b[:])
}

func measure(executable func() (string, error)) string {
	before, err := executable()
	if err != nil {
		return ""
	}
	info, err := os.Lstat(before)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(before)
	if err != nil {
		return ""
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	after, err := executable()
	if copyErr != nil || closeErr != nil || err != nil || before != after {
		return ""
	}
	final, err := os.Lstat(after)
	if err != nil || !final.Mode().IsRegular() || !os.SameFile(info, final) || info.Size() != final.Size() || !info.ModTime().Equal(final.ModTime()) {
		return ""
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
