package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

const ObservedIdentityPolicyV1 = "lsp-trace.observed-input-identity.v1"
const ObservedManifestVersionV1 = "lsp-trace.observed-input-manifest.v1"

// ObservedIdentityRequest consumes bounded recorder evidence, not a file census.
// Consistency validation cannot prove that public structs came from actual reads:
// production callers must supply InputRecorder.Evidence at their acquisition seam.
type ObservedIdentityRequest struct {
	Evidence    InputEvidence
	Acquisition AcquisitionContext
	Revision    *RevisionAttestation
}

// ObservedIdentityResult uses a new policy, not an alias for IdentityPolicyV1.
// Later host admission must bind Policy and SnapshotID, not SourceID/CollectionID.
// Manifest and Revision are owned copies. Revision is unauthenticated metadata.
type ObservedIdentityResult struct {
	Policy       string
	SourceID     string
	SnapshotID   string
	CollectionID string
	Manifest     []byte
	Acquisition  AcquisitionContext
	Revision     *RevisionAttestation
}

// observedInput commits custody for either outcome without a fabricated content
// digest for failed reads. CanonicalReceipt includes the original failure reason.
type observedInput struct {
	Path             string              `json:"path"`
	Class            InputClass          `json:"class"`
	Status           AcquisitionStatus   `json:"status"`
	ReceiptID        string              `json:"receipt_id"`
	CanonicalReceipt []byte              `json:"canonical_receipt"`
	ContentDigest    string              `json:"content_digest,omitempty"`
	Failure          *AcquisitionFailure `json:"failure,omitempty"`
}

type observedManifest struct {
	SchemaVersion     string              `json:"schema_version"`
	Status            string              `json:"status"`
	Inputs            []observedInput     `json:"inputs"`
	Contributions     []InputContribution `json:"contributions"`
	IncompleteReasons []string            `json:"incomplete_reasons"`
}

type observedContent struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// BuildObservedIdentity validates and commits bounded observations, including
// failures that the historical content-only ManifestReceipt cannot represent.
// It never reads files, fabricates a receipt, admits trust, or claims completeness.
// Every readable observation contributes to source content, irrespective of
// contribution binding; bindings report use, not source membership decisions.
func BuildObservedIdentity(request ObservedIdentityRequest) (ObservedIdentityResult, error) {
	if strings.TrimSpace(request.Acquisition.Adapter) == "" || !utf8.ValidString(request.Acquisition.Adapter) || !utf8.ValidString(request.Acquisition.WorkspaceURI) || !utf8.ValidString(request.Acquisition.InvocationID) {
		return ObservedIdentityResult{}, fmt.Errorf("observed identity: acquisition adapter required")
	}
	var revision *RevisionAttestation
	if request.Revision != nil {
		if strings.TrimSpace(request.Revision.System) == "" || strings.TrimSpace(request.Revision.Revision) == "" || !utf8.ValidString(request.Revision.System) || !utf8.ValidString(request.Revision.Revision) {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: revision requires system and revision")
		}
		copy := *request.Revision
		revision = &copy
	}
	e := request.Evidence
	if e.SchemaVersion != "lsp-trace.contributing-input-evidence.v1" {
		return ObservedIdentityResult{}, fmt.Errorf("observed identity: unsupported evidence version")
	}
	m := observedManifest{SchemaVersion: ObservedManifestVersionV1, Status: "INCOMPLETE", Inputs: []observedInput{}, Contributions: []InputContribution{}, IncompleteReasons: append([]string{}, e.IncompleteReasons...)}
	reasons := map[string]bool{}
	for _, reason := range m.IncompleteReasons {
		if strings.TrimSpace(reason) == "" || reasons[reason] || !utf8.ValidString(reason) {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: empty or duplicate incomplete reason")
		}
		reasons[reason] = true
	}
	if !reasons["unobserved dependencies are unaccounted for"] {
		return ObservedIdentityResult{}, fmt.Errorf("observed identity: unobserved dependencies must remain incomplete")
	}
	sort.Strings(m.IncompleteReasons)
	paths := map[string]bool{}
	receiptIDs := map[string]bool{}
	content := []observedContent{}
	for _, in := range e.Inputs {
		if err := validateObservedInput(in); err != nil {
			return ObservedIdentityResult{}, err
		}
		name := in.Receipt.Item.Locator
		// One observation per path, even across classes or read versions. Callers must
		// split acquisitions explicitly; silently selecting first/last loses evidence.
		if paths[name] || receiptIDs[in.ID] {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: duplicate or conflicting observation for %q", name)
		}
		paths[name] = true
		receiptIDs[in.ID] = true
		record := observedInput{Path: name, Class: in.Class, Status: in.Receipt.Status, ReceiptID: in.ID, CanonicalReceipt: in.CanonicalReceipt, Failure: in.Receipt.Failure}
		if in.Receipt.Status == Readable {
			record.ContentDigest = in.Receipt.ContentIdentity.Digest
			content = append(content, observedContent{Path: name, Digest: record.ContentDigest})
		} else if !reasons["failed input acquisition: "+in.ID] {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: failed input must retain incomplete reason")
		}
		m.Inputs = append(m.Inputs, record)
	}
	sort.Slice(m.Inputs, func(i, j int) bool { return m.Inputs[i].Path < m.Inputs[j].Path })
	sort.Slice(content, func(i, j int) bool { return content[i].Path < content[j].Path })
	contributions := map[string]bool{}
	for _, c := range e.Contributions {
		if strings.TrimSpace(c.ID) == "" || contributions[c.ID] || !utf8.ValidString(c.ID) {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: empty or duplicate contribution")
		}
		contributions[c.ID] = true
		refs := append([]string{}, c.ReceiptIDs...)
		sort.Strings(refs)
		for i, id := range refs {
			if !receiptIDs[id] || (i > 0 && refs[i-1] == id) {
				return ObservedIdentityResult{}, fmt.Errorf("observed identity: duplicate or unobserved contribution reference")
			}
		}
		if len(refs) == 0 && !reasons["missing input binding for contribution: "+c.ID] {
			return ObservedIdentityResult{}, fmt.Errorf("observed identity: unbound contribution must remain incomplete")
		}
		m.Contributions = append(m.Contributions, InputContribution{ID: c.ID, ReceiptIDs: refs})
	}
	sort.Slice(m.Contributions, func(i, j int) bool { return m.Contributions[i].ID < m.Contributions[j].ID })
	manifest, err := json.Marshal(m)
	if err != nil {
		return ObservedIdentityResult{}, err
	}
	sourceBytes, err := json.Marshal(content)
	if err != nil {
		return ObservedIdentityResult{}, err
	}
	acquisition, err := json.Marshal(request.Acquisition)
	if err != nil {
		return ObservedIdentityResult{}, err
	}
	sourceID := observedPolicyIdentity("source", sourceBytes)
	snapshotID := observedPolicyIdentity("snapshot", []byte(sourceID), manifest)
	return ObservedIdentityResult{Policy: ObservedIdentityPolicyV1, SourceID: sourceID, SnapshotID: snapshotID, CollectionID: observedPolicyIdentity("collection", []byte(snapshotID), acquisition), Manifest: manifest, Acquisition: request.Acquisition, Revision: revision}, nil
}

func validateObservedInput(in InputReceipt) error {
	reject := func(reason string) error { return fmt.Errorf("observed identity: %s", reason) }
	name := in.Receipt.Item.Locator
	if !utf8.ValidString(name) || name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00:") {
		return reject("path must be canonical root-relative")
	}
	switch in.Class {
	case InputSource, InputConfiguration, InputDeclaration, InputGeneratedMapping:
	default:
		return reject("unknown input class")
	}
	r := in.Receipt
	if r.Item.ID != name || r.Provenance.Locator != name || r.Provenance.Mechanism != "bounded-input/"+string(in.Class) || r.Provenance.Revision != "" {
		return reject("receipt path/class/provenance mismatch")
	}
	switch r.Status {
	case Readable:
		if r.ContentIdentity == nil || r.Failure != nil {
			return reject("readable observation requires content identity and no failure")
		}
	case Unreadable:
		if r.Failure != nil && !utf8.ValidString(r.Failure.Reason) {
			return reject("failure reason must be valid UTF-8")
		}
		if r.ContentIdentity != nil || in.Content != nil {
			return reject("unreadable observation cannot have content or a content identity")
		}
	default:
		return reject("unknown acquisition status")
	}
	// Recompute only for validation. Retain the supplied exact canonical receipt,
	// never substitute a reconstructed receipt or use its hash as a content digest.
	expected, _, canonical, err := CanonicalizeReceipt(r.Item, Acquisition{Status: r.Status, Provenance: r.Provenance, Failure: r.Failure}, in.Content)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, r) || !bytes.Equal(canonical, in.CanonicalReceipt) {
		return reject("receipt/content/canonical bytes mismatch")
	}
	if in.ID != fmt.Sprintf("sha256:%x", sha256.Sum256(in.CanonicalReceipt)) {
		return reject("canonical receipt hash mismatch")
	}
	return nil
}

func observedPolicyIdentity(kind string, components ...[]byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(ObservedIdentityPolicyV1 + ":" + kind + "\x00"))
	for _, component := range components {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(component)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(component)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
