package graphprovenance

import (
	"bytes"
	"encoding/json"
	"errors"
)

func validateSupplyV2(r *Receipt) error {
	s := r.Supply
	if s == nil {
		return errors.New("V2 missing supply metadata")
	}
	var expected any
	switch {
	case s.Method == "textDocument/didOpen" && s.Version == 1:
		var p struct {
			TextDocument struct {
				LanguageID string `json:"languageId"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(s.Params, &p) != nil || p.TextDocument.LanguageID == "" {
			return errors.New("V2 missing supplied language")
		}
		expected = map[string]any{"textDocument": map[string]any{"uri": r.URI, "languageId": p.TextDocument.LanguageID, "version": s.Version, "text": string(r.Content)}}
	case s.Method == "textDocument/didChange" && s.Version > 1:
		expected = map[string]any{"textDocument": map[string]any{"uri": r.URI, "version": s.Version}, "contentChanges": []any{map[string]any{"text": string(r.Content)}}}
	default:
		return errors.New("V2 invalid notification/version")
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	wanted, err := canonicalV2(raw)
	if err != nil {
		return err
	}
	got, err := canonicalV2(s.Params)
	if err != nil {
		return err
	}
	if !bytes.Equal(wanted, got) {
		return errors.New("V2 notification parameters differ from supplied bytes/version")
	}
	return nil
}
