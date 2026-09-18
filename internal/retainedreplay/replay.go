// Package retainedreplay replays one completely self-contained retained projection packet.
package retainedreplay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedinspection"
	"lsp-trace/internal/retainedmanifest"
	"lsp-trace/internal/retainedoperation"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedresolver"
	"lsp-trace/internal/sourceobject"
)

// Packet contains every exact byte string and source object needed for replay.
type Packet struct {
	Manifest          []byte
	Graph             []byte
	GraphProvenanceV5 []byte
	SourceSnapshotV2  []byte
	Request           []byte
	Objects           []sourceobject.Object
}

type failure struct{ cause error }

func (e *failure) Error() string { return "retained replay rejected" }
func (e *failure) Unwrap() error { return e.cause }

func fail(err error) ([]byte, error) {
	if err == nil {
		err = errors.New("replay failed")
	}
	return nil, &failure{cause: err}
}

// Replay validates and executes a self-contained packet without external capabilities.
func Replay(packet Packet) ([]byte, error) {
	manifest, _, err := retainedmanifest.Admit(packet.Manifest, packet.Graph, packet.GraphProvenanceV5)
	if err != nil {
		return fail(err)
	}
	if _, err = retainedprojection.Admit(packet.SourceSnapshotV2); err != nil {
		return fail(err)
	}
	decoded, err := retainedinspection.Decode(packet.Request)
	if err != nil || decoded.Projection == nil || decoded.Legacy != nil {
		return fail(errors.New("retained projection request required"))
	}
	projection := decoded.Projection
	evidence := projection.RetainedSourceEvidence
	if evidence.PublicationSnapshotV2 != nil || evidence.ContentAddressedSnapshotV2 != nil || evidence.InlineSnapshotV2 == "" || !bytes.Equal([]byte(evidence.InlineSnapshotV2), packet.SourceSnapshotV2) {
		return fail(errors.New("exact inline retained snapshot required"))
	}

	objects := make(map[sourceobject.Identity]sourceobject.Object, len(packet.Objects))
	var previous sourceobject.Identity
	for i, object := range packet.Objects {
		id := object.Identity
		if !canonicalIdentity(id) || uint64(len(object.Bytes)) != id.ByteLength || digest(object.Bytes) != id.Digest {
			return fail(errors.New("non-canonical source object"))
		}
		if i > 0 && !identityLess(previous, id) {
			return fail(errors.New("source objects not unique and strictly ordered"))
		}
		if !authorized(manifest, id) {
			return fail(errors.New("source object not authorized by manifest"))
		}
		objects[id] = sourceobject.Object{Identity: id, Bytes: append([]byte(nil), object.Bytes...)}
		previous = id
	}

	lookup := &exactLookup{objects: objects, calls: make(map[sourceobject.Identity]int)}
	result, operationFailure := retainedoperation.NewInspectHydratedHandler(lookup)(context.Background(), operation.Request{Input: append([]byte(nil), packet.Request...)})
	if operationFailure != nil || result.Artifact == nil {
		if operationFailure != nil {
			return fail(operationFailure)
		}
		return fail(errors.New("replay produced no artifact"))
	}
	for _, calls := range lookup.calls {
		if calls != 1 {
			return fail(errors.New("source object lookup was not exact-once"))
		}
	}
	return append([]byte(nil), result.Artifact...), nil
}

type exactLookup struct {
	objects map[sourceobject.Identity]sourceobject.Object
	calls   map[sourceobject.Identity]int
}

func (l *exactLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	l.calls[id]++
	if l.calls[id] != 1 {
		return sourceobject.Object{}, &sourceobject.Error{Code: sourceobject.CodePolicy}
	}
	object, ok := l.objects[id]
	if !ok {
		return sourceobject.Object{}, &sourceobject.Error{Code: sourceobject.CodeMissing}
	}
	return sourceobject.Object{Identity: object.Identity, Bytes: append([]byte(nil), object.Bytes...)}, nil
}

func authorized(manifest retainedmanifest.Manifest, id sourceobject.Identity) bool {
	for _, entry := range manifest.Entries {
		if entry.Source == id && entry.Availability == "AVAILABLE" && (entry.StorageClass == "CONTENT_ADDRESS" || entry.StorageClass == "EMBEDDED_IMMUTABLE") && entry.CustodyIdentity == retainedresolver.ContentCustodyIdentity(entry.StorageClass, id) {
			return true
		}
	}
	return false
}

func canonicalIdentity(id sourceobject.Identity) bool {
	if len(id.Digest) != 71 || !strings.HasPrefix(id.Digest, "sha256:") || strings.ToLower(id.Digest) != id.Digest {
		return false
	}
	_, err := hex.DecodeString(id.Digest[7:])
	return err == nil
}

func identityLess(left, right sourceobject.Identity) bool {
	return left.Digest < right.Digest || left.Digest == right.Digest && left.ByteLength < right.ByteLength
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
