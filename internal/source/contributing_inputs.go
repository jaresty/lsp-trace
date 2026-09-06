package source

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
)

// InputClass describes the role of bytes actually requested by acquisition,
// not a classification inferred from a repository census or file extension.
type InputClass string

const (
	InputSource           InputClass = "SOURCE"
	InputConfiguration    InputClass = "CONFIGURATION"
	InputDeclaration      InputClass = "DECLARATION"
	InputGeneratedMapping InputClass = "GENERATED_MAPPING"
)

// InputReceipt binds a classified observation to existing canonical receipt bytes.
type InputReceipt struct {
	ID               string     `json:"id"`
	Class            InputClass `json:"class"`
	Receipt          Receipt    `json:"receipt"`
	Content          []byte     `json:"content"`
	CanonicalReceipt []byte     `json:"canonical_receipt"`
}

type InputContribution struct {
	ID         string   `json:"id"`
	ReceiptIDs []string `json:"receipt_ids"`
}

// InputEvidence is bounded read evidence, never whole-source completeness or
// authenticated custody. Unobserved dependencies always remain explicit.
type InputEvidence struct {
	SchemaVersion     string              `json:"schema_version"`
	Inputs            []InputReceipt      `json:"inputs"`
	Contributions     []InputContribution `json:"contributions"`
	IncompleteReasons []string            `json:"incomplete_reasons"`
}

// InputRecorder must be used at the actual acquisition read seam. Discovery
// records and caller-declared receipt bytes cannot be inserted into it.
type InputRecorder struct {
	mu            sync.Mutex
	root          *os.Root
	inputs        map[string]InputReceipt
	contributions map[string][]string
}

func NewInputRecorder(root string) (*InputRecorder, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	return &InputRecorder{root: r, inputs: make(map[string]InputReceipt), contributions: make(map[string][]string)}, nil
}
func (r *InputRecorder) Close() error { r.mu.Lock(); defer r.mu.Unlock(); return r.root.Close() }

// ReadInput captures the exact bytes returned to acquisition, including failed
// read attempts. The returned ID is SHA-256 of the existing canonical receipt,
// whose mechanism binds the class. It is not a snapshot or trust identity.
// A failed scoped read returns its failure receipt ID alongside the read error.
// Malformed paths/classes are rejected before IO and produce no receipt.
func (r *InputRecorder) ReadInput(name string, class InputClass) ([]byte, string, error) {
	if name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00:") {
		return nil, "", errors.New("input path must be a canonical root-relative path")
	}
	switch class {
	case InputSource, InputConfiguration, InputDeclaration, InputGeneratedMapping:
	default:
		return nil, "", errors.New("unknown input classification")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	content, readErr := r.root.ReadFile(name)
	acquisition := Acquisition{Status: Readable, Provenance: Provenance{Mechanism: "bounded-input/" + string(class), Locator: name}}
	if readErr != nil {
		content = nil
		acquisition.Status = Unreadable
		// Avoid leaking machine-specific root paths into canonical evidence.
		reason := readErr.Error()
		var pe *os.PathError
		if errors.As(readErr, &pe) {
			reason = pe.Err.Error()
		}
		acquisition.Failure = &AcquisitionFailure{Reason: reason}
	}
	receipt, canonicalContent, canonical, err := CanonicalizeReceipt(DiscoveredItem{ID: name, Locator: name}, acquisition, content)
	if err != nil {
		return nil, "", err
	}
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(canonical))
	r.inputs[id] = InputReceipt{ID: id, Class: class, Receipt: receipt, Content: canonicalContent, CanonicalReceipt: canonical}
	return content, id, readErr
}

// BindContribution records actual use by a retained contribution. Callers must
// bind at the production consumer seam; neither this API nor discovery proves
// that the consumer reported every dependency. Failed reads may be bound but
// always keep the evidence incomplete. Duplicate contribution IDs are errors.
func (r *InputRecorder) BindContribution(id string, receiptIDs ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(id) == "" || len(receiptIDs) == 0 {
		return errors.New("contribution requires an id and observed inputs")
	}
	if _, ok := r.contributions[id]; ok {
		return errors.New("duplicate contribution")
	}
	seen := make(map[string]bool, len(receiptIDs))
	for _, ref := range receiptIDs {
		if _, ok := r.inputs[ref]; !ok {
			return errors.New("contribution references unobserved input")
		}
		if seen[ref] {
			return errors.New("duplicate input reference")
		}
		seen[ref] = true
	}
	refs := append([]string(nil), receiptIDs...)
	sort.Strings(refs)
	r.contributions[id] = refs
	return nil
}

// Evidence accounts for the supplied retained contribution IDs, not a repository
// census. Missing bindings are explicit rows and reasons, never silently dropped.
// All read attempts remain available, even when no retained contribution uses
// them. No API can turn this bounded observation into whole-source completeness.
func (r *InputRecorder) Evidence(ids []string) (InputEvidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	e := InputEvidence{SchemaVersion: "lsp-trace.contributing-input-evidence.v1", Inputs: []InputReceipt{}, Contributions: []InputContribution{}, IncompleteReasons: []string{"unobserved dependencies are unaccounted for"}}
	for i, id := range ordered {
		if strings.TrimSpace(id) == "" || (i > 0 && ordered[i-1] == id) {
			return InputEvidence{}, errors.New("expected contribution IDs must be nonempty and unique")
		}
		refs := r.contributions[id]
		if len(refs) == 0 {
			e.IncompleteReasons = append(e.IncompleteReasons, "missing input binding for contribution: "+id)
			refs = []string{}
		}
		e.Contributions = append(e.Contributions, InputContribution{ID: id, ReceiptIDs: refs})
	}
	for _, in := range r.inputs {
		e.Inputs = append(e.Inputs, in)
		if in.Receipt.Status != Readable {
			e.IncompleteReasons = append(e.IncompleteReasons, "failed input acquisition: "+in.ID)
		}
	}
	sort.Slice(e.Inputs, func(i, j int) bool { return e.Inputs[i].ID < e.Inputs[j].ID })
	sort.Strings(e.IncompleteReasons)
	// Clone nested pointers and all byte/reference slices across the API boundary.
	encoded, err := json.Marshal(e)
	if err != nil {
		return InputEvidence{}, err
	}
	var detached InputEvidence
	if err = json.Unmarshal(encoded, &detached); err != nil {
		return InputEvidence{}, err
	}
	return detached, nil
}
