// Package custodyevidence composes source-owned identity and schema-owned trust
// contracts without making either lower layer depend on the other consumer.
package custodyevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"sort"

	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
)

const Version = "lsp-trace.operational-custody.v1"
const Adapter = "operational-byte-bundle"

type Output struct {
	ID      string                   `json:"id"`
	Path    string                   `json:"path"`
	Status  source.AcquisitionStatus `json:"status"`
	Content []byte                   `json:"content"`
}

type Admission struct {
	Policy         string                         `json:"policy"`
	SnapshotID     string                         `json:"snapshot_id"`
	Status         schema.AuthenticationStatus    `json:"status"`
	Receipt        []byte                         `json:"receipt"`
	GitAttestation *schema.GitAttestationEvidence `json:"git_attestation"`
}

type Evidence struct {
	SchemaVersion        string                        `json:"schema_version"`
	InputEvidence        source.InputEvidence          `json:"input_evidence"`
	Outputs              []Output                      `json:"outputs"`
	Identity             source.ObservedIdentityResult `json:"identity"`
	Admission            Admission                     `json:"admission"`
	AcquisitionStatus    string                        `json:"acquisition_status"`
	RequireAuthenticated bool                          `json:"require_authenticated"`
	PublicationPermitted bool                          `json:"publication_permitted"`
}

func AcquisitionStatus(e source.InputEvidence) string {
	for _, in := range e.Inputs {
		if in.Receipt.Status != source.Readable {
			return "PARTIAL"
		}
	}
	return "READABLE"
}
func MayPublish(required bool, status string, admission schema.AuthenticationStatus) bool {
	return !required || (status == "READABLE" && admission == schema.AuthenticationAuthenticated)
}

// ValidateFor is the shared CLI/operation validator. The schema layer cannot
// import source (which already imports schema); it fails closed for this family
// unless this composed structural-then-semantic entry point is used.
func ValidateFor(data []byte, family, version string) (string, error) {
	if family != schema.FamilyOperationalCustody {
		return schema.ValidateFor(data, family, version)
	}
	structural, err := schema.ValidateStructure(data, family, version)
	if err != nil {
		return "", err
	}
	var e Evidence
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&e); err != nil {
		return "", err
	}
	if err := Validate(e); err != nil {
		return "", fmt.Errorf("semantic validation %s: %w", Version, err)
	}
	return structural.Version, nil
}

// Validate checks retained joins, not authenticity of a reported historical host
// decision. Live authentication is exclusively HostTrustStore.Admit. Neither
// validation nor AUTHENTICATED custody establishes source/dependency completeness.
func Validate(e Evidence) error {
	if e.SchemaVersion != Version {
		return fmt.Errorf("wrong operational evidence version")
	}
	context := e.Identity.Acquisition
	uri, err := url.Parse(context.WorkspaceURI)
	if err != nil || uri.Scheme != "file" || uri.Host != "" || uri.RawQuery != "" || uri.Fragment != "" || !path.IsAbs(uri.Path) || path.Clean(uri.Path) != uri.Path || context.Adapter != Adapter || context.InvocationID == "" {
		return fmt.Errorf("invalid actual acquisition context")
	}
	rebuilt, err := source.BuildObservedIdentity(source.ObservedIdentityRequest{Evidence: e.InputEvidence, Acquisition: context, Revision: e.Identity.Revision})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(rebuilt, e.Identity) {
		return fmt.Errorf("observed identity does not match retained inputs")
	}
	if len(e.InputEvidence.Inputs) == 0 || len(e.Outputs) != len(e.InputEvidence.Inputs) || len(e.InputEvidence.Contributions) != len(e.Outputs) {
		return fmt.Errorf("byte bundle output accounting mismatch")
	}
	contributions := map[string]source.InputContribution{}
	for _, c := range e.InputEvidence.Contributions {
		contributions[c.ID] = c
	}
	inputs := map[string]source.InputReceipt{}
	for _, in := range e.InputEvidence.Inputs {
		inputs[in.Receipt.Item.Locator] = in
	}
	seen := map[string]bool{}
	for _, out := range e.Outputs {
		in, ok := inputs[out.Path]
		if !ok || seen[out.Path] || out.ID != "input:"+out.Path || out.Status != in.Receipt.Status || !bytes.Equal(out.Content, in.Content) || (out.Status == source.Unreadable && out.Content != nil) {
			return fmt.Errorf("retained byte output does not match observed input")
		}
		seen[out.Path] = true
		c, ok := contributions[out.ID]
		if !ok || len(c.ReceiptIDs) != 1 || c.ReceiptIDs[0] != in.ID {
			return fmt.Errorf("output contribution does not bind exact observed receipt")
		}
	}
	if !sort.SliceIsSorted(e.Outputs, func(i, j int) bool { return e.Outputs[i].ID < e.Outputs[j].ID }) {
		return fmt.Errorf("noncanonical output ordering")
	}
	if e.AcquisitionStatus != AcquisitionStatus(e.InputEvidence) || e.PublicationPermitted != MayPublish(e.RequireAuthenticated, e.AcquisitionStatus, e.Admission.Status) {
		return fmt.Errorf("acquisition/publication outcome mismatch")
	}
	a := e.Admission
	if a.Policy != rebuilt.Policy || a.SnapshotID != rebuilt.SnapshotID {
		return fmt.Errorf("admission policy/snapshot mismatch")
	}
	switch a.Status {
	case schema.AuthenticationMissingTrust:
		if len(a.Receipt) != 0 {
			return fmt.Errorf("missing trust has presented receipt")
		}
	case schema.AuthenticationRejected:
		// Rejected material is retained verbatim, including malformed receipts/evidence.
	case schema.AuthenticationAuthenticated:
		if _, err := schema.ValidateFor(a.Receipt, schema.FamilyTrustProvisioningReceipt, "v1"); err != nil {
			return err
		}
		var receipt struct {
			Snapshot string `json:"source_snapshot_identity"`
			Result   string `json:"verification_result"`
		}
		if err := json.Unmarshal(a.Receipt, &receipt); err != nil {
			return err
		}
		if receipt.Snapshot != rebuilt.SnapshotID || receipt.Result != "VERIFIED" {
			return fmt.Errorf("reported host approval has wrong receipt snapshot/result")
		}
		if a.GitAttestation != nil && (a.GitAttestation.EvidenceType != schema.GitCommitAttestation || a.GitAttestation.SourceSnapshotIdentity != rebuilt.SnapshotID || a.GitAttestation.CommitIdentity == "") {
			return fmt.Errorf("reported attestation has wrong binding")
		}
	default:
		return fmt.Errorf("unknown trust outcome")
	}
	return nil
}
