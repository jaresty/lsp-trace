package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"lsp-trace/internal/adr0011builtinprofile"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
)

// privateBuiltinOwner is a host-selected synthetic fixture only. It does not
// hold a session manager, accept a request, or enter acquireManaged.
type privateBuiltinOwner struct {
	root               *publication.Root
	prewrite           adr0011builtinprofile.HeldV1
	query              adr0011querytarget.Query
	descendantSelector string
	readbackTestHook   func(*publication.Root, string, int64) ([]byte, error) // nil uses fresh verified custody read
}

type privateBuiltinSymbolResult struct {
	DocumentSymbols []byte // typed documentSymbol result JSON, never a wire frame
	Claim           adr0011builtinprofile.Originals
}

func newPrivateBuiltinOwner(root *publication.Root, selected adr0011builtinprofile.Originals, q adr0011querytarget.Query, selector string) (*privateBuiltinOwner, error) {
	if len(selected.Descendants) != 0 {
		return nil, ErrAcquisition
	}
	held, err := adr0011builtinprofile.FreezeV1(selected)
	if err != nil {
		return nil, ErrAcquisition
	}
	return privateBuiltinFromHeld(root, held, q, selector)
}

// privateBuiltinFromHeld checks the same host-selected target coordinates on
// fresh freeze and restart. It does not consult installed current defaults.
func privateBuiltinFromHeld(root *publication.Root, held adr0011builtinprofile.HeldV1, q adr0011querytarget.Query, selector string) (*privateBuiltinOwner, error) {
	if root == nil || root.ValidatePrivate() != nil || !privateBuiltinSelector(selector) {
		return nil, ErrAcquisition
	}
	selected := held.Snapshot()
	if len(selected.Descendants) != 0 || held.Replay(selected) != nil {
		return nil, ErrAcquisition
	}
	// The host's schema-valid plan, not a callback or per-request override,
	// fixes the target coordinates before any synthetic invocation.
	var plan struct {
		SessionID       string `json:"session_id"`
		Generation      uint64 `json:"generation"`
		QueryURI        string `json:"query_uri"`
		Line            uint32 `json:"line"`
		Character       uint32 `json:"character"`
		Encoding        string `json:"encoding"`
		DocumentVersion int    `json:"document_version"`
		DocumentDigest  string `json:"document_digest"`
	}
	if json.Unmarshal(selected.Plan, &plan) != nil || plan.SessionID != q.SessionID || plan.Generation != q.Generation || plan.QueryURI != q.URI || plan.Line != q.Line || plan.Character != q.Character || plan.Encoding != q.Encoding || strconv.Itoa(plan.DocumentVersion) != q.DocumentVersion || plan.DocumentDigest != q.SourceDigest || q.OccurrenceID == "" {
		return nil, ErrAcquisition
	}
	return &privateBuiltinOwner{root: root, prewrite: held, query: q, descendantSelector: selector}, nil
}

// privateHostSnapshot returns a copy and a separately carried digest. The host
// must retain them independently; this does not authenticate their issuer.
func (o *privateBuiltinOwner) privateHostSnapshot() (adr0011builtinprofile.Originals, string) {
	if o == nil {
		return adr0011builtinprofile.Originals{}, ""
	}
	return o.prewrite.Snapshot(), o.prewrite.SnapshotDigest()
}

func restorePrivateBuiltinOwner(root *publication.Root, retained adr0011builtinprofile.Originals, independentlyHeldDigest string, q adr0011querytarget.Query, selector string) (*privateBuiltinOwner, error) {
	held, err := adr0011builtinprofile.RestoreV1(retained, independentlyHeldDigest)
	if err != nil {
		return nil, ErrAcquisition
	}
	return privateBuiltinFromHeld(root, held, q, selector)
}

const privateBuiltinDescendantRole = "synthetic-document-symbol-target-v1"

func privateBuiltinSelector(s string) bool {
	if len(s) < 6 || len(s) > 128 || !strings.HasSuffix(s, ".json") {
		return false
	}
	for _, c := range s {
		if c != '.' && c != '-' && c != '_' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return s[0] >= 'a' && s[0] <= 'z'
}

// The role, selector and derivation rule are fixed before invoking callback;
// the record bytes depend on the subsequently observed typed symbol result.
func privateBuiltinDescendant(q adr0011querytarget.Query, c adr0011querytarget.Candidate) []byte {
	raw, _ := json.Marshal(map[string]any{
		"role":                          privateBuiltinDescendantRole,
		"occurrence_id":                 q.OccurrenceID,
		"symbol_id":                     c.SymbolID,
		"document_symbol_result_digest": c.DocumentSymbolResultSHA256,
	})
	return raw
}

func (o *privateBuiltinOwner) acquirePrivateBuiltin(callback func(adr0011builtinprofile.Originals) (privateBuiltinSymbolResult, error)) (adr0011querytarget.Candidate, error) {
	var absent adr0011querytarget.Candidate
	if o == nil || o.root == nil || callback == nil || o.root.ValidatePrivate() != nil {
		return absent, ErrAcquisition
	}
	// A copied snapshot is the only input visible to the synthetic producer.
	pre := o.prewrite.Snapshot()
	if o.prewrite.Replay(pre) != nil {
		return absent, ErrAcquisition
	}
	result, err := callback(pre)
	if err != nil {
		return absent, ErrAcquisition
	}
	claimantBase := result.Claim
	descendants := claimantBase.Descendants
	claimantBase.Descendants = nil
	if o.prewrite.Replay(claimantBase) != nil {
		return absent, ErrAcquisition
	}
	candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(o.query, result.DocumentSymbols)
	if err != nil {
		return absent, ErrAcquisition
	}
	expected := privateBuiltinDescendant(o.query, candidate)
	if len(expected) == 0 || len(expected) > 1<<20 {
		return absent, ErrAcquisition
	}
	receipt, err := publication.PublishBoundFile(o.root, o.descendantSelector, expected, func(got []byte) error {
		if !bytes.Equal(got, expected) {
			return ErrAcquisition
		}
		return nil
	})
	if err != nil || receipt == nil || receipt.VerificationStatus != "VERIFIED" || !receipt.NamespaceAtomic || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.FinalSelector != o.descendantSelector || receipt.ByteLength != uint64(len(expected)) {
		return absent, ErrAcquisition
	}
	sum := sha256.Sum256(expected)
	if receipt.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return absent, ErrAcquisition
	}
	readback := publication.ReadVerifiedBoundFile
	if o.readbackTestHook != nil {
		readback = o.readbackTestHook
	}
	observed, err := readback(o.root, o.descendantSelector, 1<<20)
	if err != nil || !bytes.Equal(observed, expected) {
		return absent, ErrAcquisition
	}
	held, err := o.prewrite.HoldDescendants(map[string][]byte{privateBuiltinDescendantRole: observed})
	if err != nil || len(descendants) != 1 || !bytes.Equal(descendants[privateBuiltinDescendantRole], expected) {
		return absent, ErrAcquisition
	}
	result.Claim.Descendants = descendants
	if held.Replay(result.Claim) != nil || o.prewrite.Replay(claimantBase) != nil {
		return absent, ErrAcquisition
	}
	return candidate, nil
}
