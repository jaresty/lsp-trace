// Package censuscontinuation owns the immutable committed handoff between a
// completed census and deterministic Program C reconstruction.
package censuscontinuation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/strictjson"
)

const SchemaVersion = "lsp-trace.census-continuation.v1"
const CompletenessUnknown = "UNKNOWN"

const identityDomain = "lsp-trace:census-continuation:v1"

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type WorkspaceIdentity struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
}

type BuildInput struct {
	Result      censusresult.Result
	Projection  censusacquisition.Projection
	Publication censusprogramc.VerifiedPublication
	Metadata    programccompose.ExactMetadata
	Workspace   WorkspaceIdentity
}

type metadataWire struct {
	WorkspaceIdentity    string `json:"workspace_identity"`
	RevisionCustody      string `json:"revision_custody"`
	PositionEncoding     string `json:"position_encoding"`
	AcquisitionSemantics string `json:"acquisition_semantics"`
	PrivacyPolicy        string `json:"privacy_policy"`
}

type projectionWire struct {
	Session       censusacquisition.SessionIdentity `json:"session"`
	CensusID      string                            `json:"census_id"`
	Batches       []censusacquisition.BatchRequest  `json:"batches"`
	Constituents  []censusacquisition.Constituent   `json:"constituents"`
	Manifest      captureset.Manifest               `json:"manifest"`
	ManifestBytes []byte                            `json:"manifest_bytes"`
}

type handoffWire struct {
	SchemaVersion    string                               `json:"schema_version"`
	HandoffID        string                               `json:"handoff_id"`
	CensusID         string                               `json:"census_id"`
	Result           censusresult.Result                  `json:"result"`
	Receipt          captureset.PublicationReceipt        `json:"publication_receipt"`
	Manifest         captureset.Manifest                  `json:"manifest"`
	Constituents     []censusprogramc.ResolvedConstituent `json:"constituents"`
	Projection       projectionWire                       `json:"projection"`
	Metadata         metadataWire                         `json:"exact_metadata"`
	Workspace        WorkspaceIdentity                    `json:"workspace_identity"`
	PositionEncoding string                               `json:"position_encoding"`
	Authority        int                                  `json:"authority"`
	Accepted         bool                                 `json:"accepted"`
	Completeness     string                               `json:"completeness"`
}

// CommittedHandoff has no exported mutable state. Accessors return defensive copies.
type CommittedHandoff struct{ wire handoffWire }

func BuildHandoff(in BuildInput) (CommittedHandoff, error) {
	projection, err := censusacquisition.CloneProjection(in.Projection)
	if err != nil {
		return CommittedHandoff{}, fmt.Errorf("projection: %w", err)
	}
	if err := censusresult.Validate(in.Result); err != nil {
		return CommittedHandoff{}, fmt.Errorf("census result: %w", err)
	}
	wire := handoffWire{
		SchemaVersion: SchemaVersion,
		CensusID:      projection.CensusID,
		Result:        in.Result,
		Receipt:       in.Publication.Receipt,
		Manifest:      in.Publication.Manifest,
		Constituents:  append([]censusprogramc.ResolvedConstituent(nil), in.Publication.Resolved...),
		Projection: projectionWire{
			Session: projection.Session, CensusID: projection.CensusID,
			Batches: projection.Batches, Constituents: projection.Constituents,
			Manifest: projection.Manifest, ManifestBytes: projection.ManifestBytes,
		},
		Metadata: metadataFrom(in.Metadata), Workspace: in.Workspace,
		PositionEncoding: in.Metadata.PositionEncoding,
		Authority:        0, Accepted: false, Completeness: CompletenessUnknown,
	}
	for i := range wire.Constituents {
		wire.Constituents[i].Bytes = append([]byte(nil), wire.Constituents[i].Bytes...)
	}
	wire.HandoffID, err = identity(wire)
	if err != nil {
		return CommittedHandoff{}, err
	}
	h := CommittedHandoff{wire: wire}
	if err := h.Validate(); err != nil {
		return CommittedHandoff{}, err
	}
	return cloneHandoff(h)
}

func Parse(raw []byte) (CommittedHandoff, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' || !bytes.Equal(raw, trimmed) {
		return CommittedHandoff{}, errors.New("canonical JSON object required")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return CommittedHandoff{}, err
	}
	var wire handoffWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return CommittedHandoff{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CommittedHandoff{}, errors.New("one JSON value required")
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(raw, canonical) {
		return CommittedHandoff{}, errors.New("noncanonical handoff JSON")
	}
	h := CommittedHandoff{wire: wire}
	if err := h.Validate(); err != nil {
		return CommittedHandoff{}, err
	}
	return cloneHandoff(h)
}

func (h CommittedHandoff) Bytes() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(h.wire)
}

func (h CommittedHandoff) Validate() error {
	w := h.wire
	if w.SchemaVersion != SchemaVersion || w.CensusID == "" || w.HandoffID == "" {
		return errors.New("invalid handoff identity")
	}
	if w.Authority != 0 || w.Accepted || w.Completeness != CompletenessUnknown {
		return errors.New("invalid handoff authority ceiling")
	}
	if err := validateWorkspace(w.Workspace); err != nil {
		return err
	}
	if w.PositionEncoding == "" || w.PositionEncoding != w.Metadata.PositionEncoding {
		return errors.New("position encoding mismatch")
	}
	if err := censusresult.Validate(w.Result); err != nil {
		return fmt.Errorf("census result: %w", err)
	}
	if w.Result.Publication.VerificationStatus != "VERIFIED" || w.Result.Publication.DirectorySyncStatus != censusresult.DirectorySyncComplete || w.Result.Publication.CloseStatus != censusresult.CloseComplete {
		return errors.New("validated committed census result required")
	}
	if w.CensusID != w.Result.CensusID || w.Projection.CensusID != w.CensusID || w.Result.CaptureSetID != w.Manifest.LogicalDigest || w.Result.SessionID != w.Projection.Session.SessionID || w.Result.Generation != w.Projection.Session.Generation || w.Result.BatchCount != len(w.Projection.Batches) {
		return errors.New("census identity reconciliation failed")
	}
	if w.Result.Publication.Selector != w.Receipt.Selector || w.Result.Publication.Digest != w.Receipt.ArtifactSHA256 || w.Result.Publication.ByteLength != w.Receipt.ByteLength || w.Result.Publication.VerificationStatus != w.Receipt.VerificationStatus {
		return errors.New("publication receipt reconciliation failed")
	}
	if !equalJSON(w.Manifest, w.Projection.Manifest) {
		return errors.New("projection manifest mismatch")
	}
	projection, err := projectionFrom(w.Projection, w.Workspace)
	if err != nil {
		return err
	}
	publication := censusprogramc.VerifiedPublication{Receipt: w.Receipt, Manifest: w.Manifest, Resolved: cloneResolved(w.Constituents)}
	if _, err := censusprogramc.Compose(censusprogramc.Request{Projection: projection, Publication: publication, Metadata: metadataTo(w.Metadata)}); err != nil {
		return fmt.Errorf("handoff reconciliation: %w", err)
	}
	want, err := identity(w)
	if err != nil || want != w.HandoffID {
		return errors.New("handoff identity mismatch")
	}
	return nil
}

func (h CommittedHandoff) HandoffID() string                    { return h.wire.HandoffID }
func (h CommittedHandoff) CensusID() string                     { return h.wire.CensusID }
func (h CommittedHandoff) WorkspaceIdentity() WorkspaceIdentity { return h.wire.Workspace }
func (h CommittedHandoff) Authority() int                       { return h.wire.Authority }
func (h CommittedHandoff) Accepted() bool                       { return h.wire.Accepted }
func (h CommittedHandoff) Completeness() string                 { return h.wire.Completeness }
func (h CommittedHandoff) PositionEncoding() string             { return h.wire.PositionEncoding }

func (h CommittedHandoff) Projection() (censusacquisition.Projection, error) {
	if err := h.Validate(); err != nil {
		return censusacquisition.Projection{}, err
	}
	return projectionFrom(h.wire.Projection, h.wire.Workspace)
}

func ReconstructProgramC(h CommittedHandoff, seed uint64) (censusprogramc.Result, error) {
	if err := h.Validate(); err != nil {
		return censusprogramc.Result{}, err
	}
	projection, err := projectionFrom(h.wire.Projection, h.wire.Workspace)
	if err != nil {
		return censusprogramc.Result{}, err
	}
	return censusprogramc.Compose(censusprogramc.Request{Projection: projection, Publication: censusprogramc.VerifiedPublication{Receipt: h.wire.Receipt, Manifest: h.wire.Manifest, Resolved: cloneResolved(h.wire.Constituents)}, Metadata: metadataTo(h.wire.Metadata), Seed: seed})
}

func projectionFrom(w projectionWire, workspace WorkspaceIdentity) (censusacquisition.Projection, error) {
	u, err := url.Parse(workspace.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.Path == "" {
		return censusacquisition.Projection{}, errors.New("canonical file workspace URI required")
	}
	p := censusacquisition.Projection{Session: w.Session, CensusID: w.CensusID, Batches: w.Batches, Constituents: w.Constituents, Manifest: w.Manifest, ManifestBytes: w.ManifestBytes, Workspace: u.Path}
	return censusacquisition.CloneProjection(p)
}

func validateWorkspace(w WorkspaceIdentity) error {
	u, err := url.Parse(w.URI)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.Path == "" || u.String() != w.URI || strings.Contains(w.URI, `\\`) || !digestPattern.MatchString(w.Digest) {
		return errors.New("invalid canonical workspace identity")
	}
	return nil
}

func identity(w handoffWire) (string, error) {
	w.HandoffID = ""
	raw, err := json.Marshal(w)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(identityDomain))
	h.Write([]byte{0})
	h.Write(raw)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func cloneHandoff(h CommittedHandoff) (CommittedHandoff, error) {
	raw, err := json.Marshal(h.wire)
	if err != nil {
		return CommittedHandoff{}, err
	}
	var wire handoffWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return CommittedHandoff{}, err
	}
	return CommittedHandoff{wire: wire}, nil
}

func cloneResolved(in []censusprogramc.ResolvedConstituent) []censusprogramc.ResolvedConstituent {
	out := append([]censusprogramc.ResolvedConstituent(nil), in...)
	for i := range out {
		out[i].Bytes = append([]byte(nil), in[i].Bytes...)
	}
	return out
}

func metadataFrom(m programccompose.ExactMetadata) metadataWire {
	return metadataWire{WorkspaceIdentity: m.WorkspaceIdentity, RevisionCustody: m.RevisionCustody, PositionEncoding: m.PositionEncoding, AcquisitionSemantics: m.AcquisitionSemantics, PrivacyPolicy: m.PrivacyPolicy}
}
func metadataTo(m metadataWire) programccompose.ExactMetadata {
	return programccompose.ExactMetadata{WorkspaceIdentity: m.WorkspaceIdentity, RevisionCustody: m.RevisionCustody, PositionEncoding: m.PositionEncoding, AcquisitionSemantics: m.AcquisitionSemantics, PrivacyPolicy: m.PrivacyPolicy}
}
func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
