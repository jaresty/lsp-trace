package adr0011genericv5proposal

import "encoding/json"

// B4bSuccessorSelectorInput carries held correspondence and separately held
// selector bytes. This partial classifier does not replay capability events.
type B4bSuccessorSelectorInput struct {
	Correspondence     B4aInput
	ClaimedSelector    []byte
	HeldClientSelector []byte
}

// CheckB4bSuccessorSelector is a private partial result, not B4b acceptance.
func CheckB4bSuccessorSelector(in B4bSuccessorSelectorInput) string {
	if CheckB4a(in.Correspondence) != nil {
		return "MALFORMED"
	}
	if !json.Valid(in.ClaimedSelector) {
		return "MALFORMED"
	}
	if string(in.ClaimedSelector) != "null" {
		return "UNKNOWN"
	}
	if len(in.HeldClientSelector) == 0 {
		return "UNKNOWN"
	}
	var filters []map[string]json.RawMessage
	if err := json.Unmarshal(in.HeldClientSelector, &filters); err != nil || len(filters) == 0 {
		return "MALFORMED"
	}
	for _, filter := range filters {
		if len(filter) == 0 {
			return "MALFORMED"
		}
		for _, key := range []string{"scheme", "language", "pattern"} {
			if raw, exists := filter[key]; exists {
				var value string
				if json.Unmarshal(raw, &value) != nil {
					return "MALFORMED"
				}
			}
		}
	}
	return "SUPPORTED"
}
