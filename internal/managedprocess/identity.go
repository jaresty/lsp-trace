package managedprocess

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"sort"
	"strings"
)

const executableIdentityByteLimit int64 = 1 << 20

type IdentityStatus uint8

const (
	IdentityUnavailable IdentityStatus = iota
	IdentityObserved
	IdentityRaced
)

type Digest [32]byte

type Identity struct {
	PathLocator         Digest
	ExecutableBytes     Digest
	ExecutableBytesRead int64
	ExecutableStatus    IdentityStatus
	OrderedArgs         Digest
	ConfigProvenance    Digest
	EnvironmentNames    Digest
	EnvironmentPairs    Digest
	CWD                 Digest
	Workspace           Digest
}

func digestDomain(domain string, values ...string) Digest {
	h := sha256.New()
	write := func(value string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(value)))
		_, _ = h.Write(n[:])
		_, _ = io.WriteString(h, value)
	}
	write("lsp-trace/managedprocess/identity/v1")
	write(domain)
	for _, value := range values {
		write(value)
	}
	var out Digest
	copy(out[:], h.Sum(nil))
	return out
}

// ObserveIdentity derives privacy-safe identity from the exact Spec before Start.
// configProvenance and workspace are supplied by their owning runtime boundary.
func ObserveIdentity(spec Spec, configProvenance, workspace string) Identity {
	result := Identity{
		PathLocator:      digestDomain("path-locator", spec.Path),
		OrderedArgs:      digestDomain("ordered-args", spec.Args...),
		ConfigProvenance: digestDomain("config-provenance", configProvenance),
		CWD:              digestDomain("cwd", spec.Dir),
		Workspace:        digestDomain("workspace", workspace),
		ExecutableStatus: IdentityUnavailable,
	}
	names := make([]string, 0, len(spec.Env))
	pairs := make([]string, 0, len(spec.Env))
	for _, pair := range spec.Env {
		name, _, _ := strings.Cut(pair, "=")
		names = append(names, name)
		pairs = append(pairs, pair)
	}
	sort.Strings(names)
	result.EnvironmentNames = digestDomain("environment-names", names...)
	result.EnvironmentPairs = digestDomain("environment-secret-pairs", pairs...)
	before, err := os.Stat(spec.Path)
	if err != nil {
		return result
	}
	f, err := os.Open(spec.Path)
	if err != nil {
		return result
	}
	h := sha256.New()
	_, _ = io.WriteString(h, "lsp-trace/managedprocess/identity/v1\x00executable-bytes\x00")
	n, readErr := io.CopyN(h, f, executableIdentityByteLimit)
	if readErr != nil && readErr != io.EOF {
		_ = f.Close()
		return result
	}
	_ = f.Close()
	after, err := os.Stat(spec.Path)
	if err != nil {
		return result
	}
	result.ExecutableBytesRead = n
	copy(result.ExecutableBytes[:], h.Sum(nil))
	result.ExecutableStatus = IdentityObserved
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		result.ExecutableStatus = IdentityRaced
	}
	return result
}
