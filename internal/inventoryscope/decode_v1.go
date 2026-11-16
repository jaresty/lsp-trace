package inventoryscope

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

// DecodeInputV1 caps bytes with a max+1 sentinel before the full read, then
// token-checks work, aggregate decoded UTF-8, nesting, duplicates and collection
// counts before allocating the typed input. Errors never include caller data.
func DecodeInputV1(r io.Reader, limits LimitsV1) (InputV1, error) {
	var in InputV1
	l, e := limits.EffectiveV1()
	if e != nil {
		return in, e
	}
	if r == nil {
		return in, ErrInvalidV1
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxInputBytes+1))
	if e != nil {
		return in, ErrInvalidV1
	}
	if int64(len(b)) > l.MaxInputBytes {
		return in, ErrLimitV1
	}
	if !utf8.Valid(b) {
		return in, ErrInvalidV1
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	work, chars, refs := l.MaxWork, l.MaxUTF8Bytes, l.MaxReferences
	var visit func(string, int) error
	visit = func(field string, depth int) error {
		if depth > 32 || work == 0 {
			return ErrLimitV1
		}
		work--
		t, e := d.Token()
		if e != nil {
			return ErrInvalidV1
		}
		if s, ok := t.(string); ok {
			if len(s) > chars {
				return ErrLimitV1
			}
			chars -= len(s)
			return nil
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			// encoding/json matches fields case-insensitively; the new schema
			// intentionally does not admit those alternate spellings.
			allowed := ""
			switch field {
			case "":
				allowed = "|schema|declaration|candidates|observations|resolutions|"
			case "declaration":
				allowed = "|subject|application_roots|root_boundary|includes|excludes|source_id|config_digest|policy_id|"
			case "candidates":
				allowed = "|original_ordinal|canonical_relative_path|candidate_id|language_id|discovery_provenance|"
			case "observations":
				allowed = "|observation_id|candidate_id|assertion|kind|producer_identity|producer_version|artifact|source_id|config_digest|"
			case "resolutions":
				allowed = "|resolution_id|candidate_id|observation_refs|rule|outcome|caller_action_ref|"
			case "discovery_provenance", "artifact":
				allowed = "|artifact_id|digest|"
			default:
				return ErrInvalidV1
			}
			keys := map[string]bool{}
			for d.More() {
				if work == 0 {
					return ErrLimitV1
				}
				work--
				v, e := d.Token()
				if e != nil {
					return ErrInvalidV1
				}
				key, ok := v.(string)
				if !ok {
					return ErrInvalidV1
				}
				if len(key) > chars {
					return ErrLimitV1
				}
				if len(key) > 64 || keys[key] || strings.Contains(key, "|") || !strings.Contains(allowed, "|"+key+"|") {
					return ErrInvalidV1
				}
				chars -= len(key)
				keys[key] = true
				if e = visit(key, depth+1); e != nil {
					return e
				}
			}
			v, e := d.Token()
			if e != nil || v != json.Delim('}') {
				return ErrInvalidV1
			}
		case '[':
			count := 0
			max := l.MaxWork
			switch field {
			case "candidates":
				max = l.MaxCandidates
			case "observations":
				max = l.MaxObservations
			case "resolutions":
				max = l.MaxResolutions
			case "observation_refs":
				max = l.MaxReferences
			}
			for d.More() {
				if count >= max {
					return ErrLimitV1
				}
				count++
				if field == "observation_refs" {
					if refs == 0 {
						return ErrLimitV1
					}
					refs--
				}
				if e = visit(field, depth+1); e != nil {
					return e
				}
			}
			v, e := d.Token()
			if e != nil || v != json.Delim(']') {
				return ErrInvalidV1
			}
		default:
			return ErrInvalidV1
		}
		return nil
	}
	if e = visit("", 0); e != nil {
		return in, e
	}
	if _, e = d.Token(); e != io.EOF {
		return in, ErrInvalidV1
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&in); e != nil {
		return InputV1{}, ErrInvalidV1
	}
	if _, e = QualifyV1(in, l); e != nil {
		return InputV1{}, e
	}
	return in, nil
}
