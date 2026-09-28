// Package adr0011closure contains inert, test-owned dependency evidence.
// Its records only bind an expected set of references; they neither authenticate
// predecessors nor confer admission or issuance authority.
package adr0011closure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"lsp-trace/internal/publication"
)

var errClosure = errors.New("inert dependency closure unavailable")

const (
	version         = "ADR0011_INERT_CLOSURE_V1"
	maxRecordBytes  = 1 << 20
	maxDependencies = 256
)

type reference struct {
	Selector string `json:"selector"`
	Digest   string `json:"digest"`
	Role     string `json:"role"`
}
type record struct {
	Version      string      `json:"version"`
	Owner        string      `json:"owner"`
	Role         string      `json:"role"`
	Dependencies []reference `json:"dependencies"`
}

// uncertainty is private transaction evidence, never an issuing receipt.
type uncertainty struct{ Selector, Digest string }

// The variable is replaceable only by package-local tests. Production uses
// Root's independently opened, bounded, no-follow read of the installed file.
var readExact = func(root *publication.Root, selector string) ([]byte, error) {
	return root.ReadSelector(selector, maxRecordBytes)
}

func token(s string) bool {
	if len(s) < 1 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			if c < '0' || c > '9' {
				if c != '-' {
					return false
				}
			}
		}
	}
	return true
}
func validRef(r reference) bool {
	if !token(r.Role) || len(r.Selector) < 1 || len(r.Selector) > 200 || strings.ContainsAny(r.Selector, "/\\\x00") || !strings.HasSuffix(r.Selector, ".json") || len(r.Digest) != 71 || !strings.HasPrefix(r.Digest, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(r.Digest[7:])
	return err == nil && r.Digest == strings.ToLower(r.Digest)
}
func validRecord(r record) bool {
	if r.Version != version || !token(r.Owner) || !token(r.Role) || r.Dependencies == nil || len(r.Dependencies) > maxDependencies {
		return false
	}
	previous := ""
	for _, d := range r.Dependencies {
		if !validRef(d) {
			return false
		}
		key := d.Role + "\x00" + d.Selector + "\x00" + d.Digest
		if previous != "" && key <= previous {
			return false
		}
		previous = key
	}
	return true
}
func canonical(r record) ([]byte, error) {
	if !validRecord(r) {
		return nil, errClosure
	}
	raw, err := json.Marshal(r)
	if err != nil || len(raw) > maxRecordBytes {
		return nil, errClosure
	}
	return raw, nil
}
func identity(role string, raw []byte) reference {
	sum := sha256.Sum256(append(append([]byte(version+"\x00"+role+"\x00"), raw...), 0))
	digest := hex.EncodeToString(sum[:])
	return reference{Selector: "adr0011-inert-" + role + "-" + digest + ".json", Digest: "sha256:" + digest, Role: role}
}
func equalRefs(a, b []reference) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func replay(root *publication.Root, ref reference, owner, role string, expected []reference) error {
	if root == nil || !validRef(ref) || ref.Role != role || !token(owner) || !token(role) || !validRecord(record{version, owner, role, expected}) {
		return errClosure
	}
	raw, err := readExact(root, ref.Selector)
	if err != nil || len(raw) > maxRecordBytes {
		return errClosure
	}
	var observed record
	if json.Unmarshal(raw, &observed) != nil {
		return errClosure
	}
	encoded, err := canonical(observed)
	if err != nil || !bytes.Equal(raw, encoded) || observed.Owner != owner || observed.Role != role || !equalRefs(observed.Dependencies, expected) || identity(role, raw) != ref {
		return errClosure
	}
	return nil
}
func publish(root *publication.Root, owner, role string, dependencies []reference) (reference, *uncertainty, error) {
	zero := reference{}
	if root == nil {
		return zero, nil, errClosure
	}
	raw, err := canonical(record{version, owner, role, dependencies})
	if err != nil {
		return zero, nil, err
	}
	ref := identity(role, raw)
	result := publication.NewPublisher().Publish(publication.Request{Root: root, Selector: ref.Selector, Bytes: raw, ArtifactSchemaID: version})
	if result.Failure != nil {
		if result.Failure.AtomicRename {
			return zero, &uncertainty{ref.Selector, ref.Digest}, fmt.Errorf("%w: committed verification failed", errClosure)
		}
		return zero, nil, fmt.Errorf("%w: precommit publication failed", errClosure)
	}
	if result.Receipt == nil || replay(root, ref, owner, role, dependencies) != nil {
		return zero, &uncertainty{ref.Selector, ref.Digest}, fmt.Errorf("%w: committed verification failed", errClosure)
	}
	return ref, nil, nil
}
