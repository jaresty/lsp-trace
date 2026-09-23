package sessionruntime

// CompletedMethodRequestFrame returns a defensive copy of the exact framed
// request only after both the owned write and keyed successful D/R response.
// No result is an immutable receipt or proof of provider receipt.
func (r RoundTripResult) CompletedMethodRequestFrame() ([]byte, bool) {
	if r.methodRequestFrame == nil {
		return nil, false
	}
	return append([]byte(nil), r.methodRequestFrame...), true
}
