package sessionruntime

// CompletedDocumentSymbolRequestFrame returns a defensive copy of the exact
// owned request frame only after a matching successful response. No receipt is issued.
func (r RoundTripResult) CompletedDocumentSymbolRequestFrame() ([]byte, bool) {
	if r.documentSymbolRequestFrame == nil {
		return nil, false
	}
	return append([]byte(nil), r.documentSymbolRequestFrame...), true
}

// CompletedDocumentSymbolResponseFrame returns a defensive copy of the exact
// selected successful response frame. It does not authenticate the provider.
func (r RoundTripResult) CompletedDocumentSymbolResponseFrame() ([]byte, bool) {
	if r.documentSymbolResponseFrame == nil {
		return nil, false
	}
	return append([]byte(nil), r.documentSymbolResponseFrame...), true
}
