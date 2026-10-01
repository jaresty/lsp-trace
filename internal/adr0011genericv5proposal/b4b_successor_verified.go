package adr0011genericv5proposal

import (
	"encoding/json"
	"strings"
)

// B4bVerifiedSelector is a private, partial selector-only probe; it does not
// establish capability-event chronology or issue a capability claim.
func B4bVerifiedSelector(held B4aInput, selector json.RawMessage) string {
	if len(selector) == 0 {
		return "UNKNOWN"
	}
	var filters []map[string]json.RawMessage
	if err := json.Unmarshal(selector, &filters); err != nil || len(filters) == 0 {
		return "MALFORMED"
	}
	result := "UNSUPPORTED"
	for _, filter := range filters {
		if filter == nil {
			return "MALFORMED"
		}
		schemeRaw, exists := filter["scheme"]
		if !exists {
			result = "UNKNOWN"
			continue
		}
		var scheme string
		if err := json.Unmarshal(schemeRaw, &scheme); err != nil || !b4bVerifiedScheme(scheme) {
			return "MALFORMED"
		}
		colon := strings.IndexByte(held.URI, ':')
		if colon <= 0 || !b4bVerifiedScheme(held.URI[:colon]) {
			result = "UNKNOWN"
			continue
		}
		if !strings.EqualFold(held.URI[:colon], scheme) {
			continue
		}
		if language, hasLanguage := filter["language"]; hasLanguage {
			var value string
			if err := json.Unmarshal(language, &value); err != nil || value == "" {
				return "MALFORMED"
			}
			if held.Language == nil {
				result = "UNKNOWN"
				continue
			}
			if *held.Language != value {
				continue
			}
		}
		// Pattern and other constraints have no positive witness in this partial probe.
		if len(filter) != 1 {
			result = "UNKNOWN"
			continue
		}
		return "SUPPORTED"
	}
	return result
}

func b4bVerifiedScheme(s string) bool {
	if len(s) == 0 || !asciiAlpha(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !asciiAlpha(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func asciiAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
