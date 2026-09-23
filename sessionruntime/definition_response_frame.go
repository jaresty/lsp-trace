package sessionruntime

// CompletedDefinitionResponseFrame returns an in-memory copy only for an opted-in,
// successful definition response after the manager accepts its exact key and
// checks the selected bounds. It is not immutable or provider-authenticated.
// No raw frame is returned for a server error or an unselected, over-cap, or
// failed response. Absence is not a method outcome or an empty-query verdict.
func (r RoundTripResult) CompletedDefinitionResponseFrame() ([]byte, bool) {
	if r.definitionResponseFrame == nil {
		return nil, false
	}
	return append([]byte(nil), r.definitionResponseFrame...), true
}
