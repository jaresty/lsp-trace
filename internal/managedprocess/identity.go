package managedprocess

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const executableIdentityByteLimit int64 = 1 << 20

var (
	identityOpen          = os.Open
	identityCanonicalPath = filepath.EvalSymlinks
	identityPathLstat     = os.Lstat
	identityPathStat      = os.Stat
)

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
	return observeIdentity(spec, configProvenance, workspace, nil)
}

func observeIdentity(spec Spec, configProvenance, workspace string, afterRead func()) Identity {
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
	seenNames := make(map[string]struct{}, len(spec.Env))
	for _, pair := range spec.Env {
		name, _, _ := strings.Cut(pair, "=")
		if _, duplicate := seenNames[name]; duplicate {
			return result
		}
		seenNames[name] = struct{}{}
		names = append(names, name)
		pairs = append(pairs, pair)
	}
	sort.Strings(names)
	sort.Strings(pairs)
	result.EnvironmentNames = digestDomain("environment-names", names...)
	result.EnvironmentPairs = digestDomain("environment-secret-pairs", pairs...)

	original, err := identityPathLstat(spec.Path)
	if err != nil || !original.Mode().IsRegular() {
		return result
	}
	f, err := identityOpen(spec.Path)
	if err != nil {
		return result
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm()&0o111 == 0 || opened.Mode().Perm()&0o022 != 0 {
		return result
	}
	h := sha256.New()
	_, _ = io.WriteString(h, "lsp-trace/managedprocess/identity/v1\x00executable-bytes\x00")
	n, readErr := io.CopyN(h, f, executableIdentityByteLimit)
	if readErr != nil && readErr != io.EOF {
		return result
	}
	_ = f.Close()
	if afterRead != nil {
		afterRead()
	}
	canonical, err := identityCanonicalPath(spec.Path)
	if err != nil {
		return result
	}
	resolvedOriginal, err := identityPathLstat(spec.Path)
	if err != nil || !resolvedOriginal.Mode().IsRegular() || !sameExecutable(opened, resolvedOriginal) {
		result.ExecutableStatus = IdentityRaced
		return result
	}
	linked, err := identityPathLstat(canonical)
	if err != nil || !linked.Mode().IsRegular() {
		return result
	}
	current, err := identityPathStat(canonical)
	if err != nil {
		return result
	}
	final, err := identityPathLstat(spec.Path)
	if err != nil || !final.Mode().IsRegular() {
		result.ExecutableStatus = IdentityRaced
		return result
	}
	if !sameExecutable(opened, original) || !sameExecutable(opened, current) || !sameExecutable(opened, final) {
		result.ExecutableStatus = IdentityRaced
		return result
	}
	result.ExecutableBytesRead = n
	copy(result.ExecutableBytes[:], h.Sum(nil))
	result.ExecutableStatus = IdentityObserved
	return result
}

func sameExecutable(opened, path os.FileInfo) bool {
	return os.SameFile(opened, path) && opened.Size() == path.Size() && opened.ModTime().Equal(path.ModTime())
}
