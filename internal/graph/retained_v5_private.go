package graph

import (
	"bytes"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

// ProjectRetainedV3ToV5Private is a bounded structural serialization helper,
// never a source/producer/admission capability. The qualified private owner is
// responsible for original byte custody. It does no process or filesystem IO.
func ProjectRetainedV3ToV5Private(original []byte, workspaceURI, serverCommand string) ([]byte, error) {
	if len(original) == 0 || len(original) > 4<<20 {
		return nil, errors.New("private retained graph byte bound")
	}
	g, err := DecodeNativeV3(original)
	if err != nil {
		return nil, err
	}
	if g.SchemaVersion != SchemaVersionV3 || g.Invocation.Expansion.TopmostSiblings || len(g.SiblingCandidates) != 0 {
		return nil, errors.New("private retained graph profile")
	}
	g.SchemaVersion = SchemaVersionV5
	g.Invocation.WorkspaceURI = workspaceURI
	g.Invocation.Server.Command = serverCommand
	g.Invocation.Expansion.TopmostSiblings = false
	g.Invocation.Expansion.TopmostSiblingOutcome = TopmostSiblingNotRequested
	g.Canonicalize()
	return MarshalWithRetainedProcessContextPrivate(g, original)
}

// MarshalWithRetainedProcessContextPrivate compares/serializes the runtime model
// with metadata from an independently validated native V3 document. It supports
// private same-version carrier roundtrips as well as the derived V5 projection.
// It does not recover or synthesize omitted secrets and grants no authority.
func MarshalWithRetainedProcessContextPrivate(g Result, original []byte) ([]byte, error) {
	if len(original) == 0 || len(original) > 32<<20 || (g.SchemaVersion != SchemaVersionV3 && g.SchemaVersion != SchemaVersionV5) {
		return nil, errors.New("private retained graph profile/bound")
	}
	if err := strictjson.RejectDuplicates(original); err != nil {
		return nil, err
	}
	if err := ValidateSemanticBundle(original); err != nil {
		return nil, err
	}
	var retained bundleV3
	decoder := json.NewDecoder(bytes.NewReader(original))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&retained); err != nil {
		return nil, err
	}
	if retained.SchemaVersion != SchemaVersionV3 {
		return nil, errors.New("private retained original must be native v3")
	}
	// Use the standard canonical producer for every other field. Its temporary
	// process projection is NOT exposed: replace that typed projection with the
	// original validated metadata before computing the V5 semantic commitment.
	projected, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	var derived bundleV3
	if err := json.Unmarshal(projected, &derived); err != nil {
		return nil, err
	}
	derived.ProcessContext = retained.ProcessContext
	canonical, err := json.Marshal(derived.semanticV3)
	if err != nil {
		return nil, err
	}
	version, domain, scope := semanticReceiptCoordinates(g.SchemaVersion)
	derived.TraceReceipt = semanticReceiptV3{version, domainDigest(domain, canonical), scope}
	native, err := json.Marshal(derived)
	if err != nil {
		return nil, err
	}
	if err := ValidateSemanticBundle(native); err != nil {
		return nil, err
	}
	return native, nil
}
