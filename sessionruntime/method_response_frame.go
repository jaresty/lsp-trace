package sessionruntime

// CompletedReferencesResponseFrame returns a defensive copy only for an
// opted-in, accepted, bounded successful references response. It does not
// verify the query target, admit returned locations, or authenticate a provider.
func (r RoundTripResult) CompletedReferencesResponseFrame() ([]byte, bool) {
	if r.referencesResponseFrame == nil {
		return nil, false
	}
	return append([]byte(nil), r.referencesResponseFrame...), true
}

// CompletedMethodResponseFrame is a method-neutral private accessor for the
// selected definition or references frame. It is neither immutable nor a receipt.
func (r RoundTripResult) CompletedMethodResponseFrame() ([]byte, bool) {
	if r.requestWrite == nil {
		return nil, false
	}
	switch r.requestWrite.Method {
	case "textDocument/definition":
		return r.CompletedDefinitionResponseFrame()
	case "textDocument/references":
		return r.CompletedReferencesResponseFrame()
	default:
		return nil, false
	}
}
