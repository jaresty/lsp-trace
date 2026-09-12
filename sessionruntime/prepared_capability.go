package sessionruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// PreparedOperationBinding identifies the one trace operation allowed to reuse
// an already-observed document supply. All fields participate in admission.
type PreparedOperationBinding struct {
	OperationNonce string
	RequestID      string
	SessionID      string
	Generation     uint64
	URI            string
	LanguageID     string
}

// PreparedDocumentCapability is an opaque, single-use authorization. Its
// fields are deliberately private: DocumentResult remains evidence, not power.
type PreparedDocumentCapability struct {
	mu       *sync.Mutex
	consumed *bool
	binding  PreparedOperationBinding
	document DocumentResult
	content  [32]byte
	params   [32]byte
}

type DocumentPreparer interface {
	PrepareDocument(context.Context, DocumentRequest) DocumentResult
}

// PrepareDocumentForOperation performs preparation and seals its exact result
// to one operation. It is orchestration, not a general DocumentResult wrapper.
func PrepareDocumentForOperation(ctx context.Context, runtime DocumentPreparer, req DocumentRequest, requestID string) (DocumentResult, PreparedDocumentCapability, error) {
	if runtime == nil {
		return DocumentResult{}, PreparedDocumentCapability{}, fmt.Errorf("prepared operation identity required")
	}
	var nonceBytes [32]byte
	if _, err := rand.Read(nonceBytes[:]); err != nil {
		return DocumentResult{}, PreparedDocumentCapability{}, err
	}
	doc := runtime.PrepareDocument(ctx, req)
	if doc.Failure != "" || doc.Supply == nil {
		return doc, PreparedDocumentCapability{}, fmt.Errorf("prepared document supply unavailable")
	}
	used := false
	capability := PreparedDocumentCapability{
		mu: &sync.Mutex{}, consumed: &used,
		binding:  PreparedOperationBinding{OperationNonce: hex.EncodeToString(nonceBytes[:]), RequestID: requestID, SessionID: req.SessionID, Generation: req.Generation, URI: req.URI, LanguageID: doc.LanguageID},
		document: cloneDocumentResult(doc), content: sha256.Sum256(doc.Supply.Content), params: sha256.Sum256(doc.Supply.Params),
	}
	return doc, capability, nil
}

// Binding returns the operation identity selected by trusted orchestration. It
// exposes no document bytes and cannot be used to construct another capability.
func (c PreparedDocumentCapability) Binding() PreparedOperationBinding { return c.binding }

// ConsumePreparedDocumentCapability admits exactly one matching use.
func ConsumePreparedDocumentCapability(c PreparedDocumentCapability, binding PreparedOperationBinding) (DocumentResult, error) {
	if c.mu == nil || c.consumed == nil || c.document.Supply == nil || c.binding != binding {
		return DocumentResult{}, fmt.Errorf("prepared document capability mismatch")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if *c.consumed {
		return DocumentResult{}, fmt.Errorf("prepared document capability already consumed")
	}
	if sha256.Sum256(c.document.Supply.Content) != c.content || sha256.Sum256(c.document.Supply.Params) != c.params ||
		c.document.URI != binding.URI || c.document.LanguageID != binding.LanguageID || c.document.Supply.SessionID != binding.SessionID ||
		c.document.Supply.Generation != binding.Generation || c.document.Supply.URI != binding.URI || c.document.Supply.DocumentVersion != c.document.Version {
		return DocumentResult{}, fmt.Errorf("prepared document capability content mismatch")
	}
	*c.consumed = true
	return cloneDocumentResult(c.document), nil
}

func cloneDocumentResult(in DocumentResult) DocumentResult {
	out := in
	if in.Supply != nil {
		s := *in.Supply
		s.Content = bytes.Clone(in.Supply.Content)
		s.Params = bytes.Clone(in.Supply.Params)
		out.Supply = &s
	}
	return out
}
