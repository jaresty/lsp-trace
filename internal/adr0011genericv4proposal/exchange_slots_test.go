package adr0011genericv4proposal

import (
	"strings"
	"testing"
)

// Assert only the positional exchange kind, not method-specific nested targets.
func TestV4ExchangeSlotPermutations(t *testing.T) {
	s, e := compiler(t).Compile(schemaURI + "#/$defs/CAPABILITY_EVENTS")
	if e != nil {
		t.Fatal(e)
	}
	desc := map[string]any{"length": 0, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "held:frame"}
	id := map[string]any{"session": "s", "generation": 1, "transaction": "tx"}
	exchange := func(method string) map[string]any {
		reqDir, respDir := "SERVER_TO_CLIENT", "CLIENT_TO_SERVER"
		params := map[string]any{}
		switch method {
		case "initialize":
			reqDir, respDir = "CLIENT_TO_SERVER", "SERVER_TO_CLIENT"
		case "client/registerCapability":
			params["registrations"] = []any{map[string]any{"id": "r", "method": "textDocument/definition", "registerOptions": map[string]any{}}}
		case "client/unregisterCapability":
			params["unregisterations"] = []any{map[string]any{"id": "r", "method": "textDocument/definition"}}
		}
		return map[string]any{"request_direction": reqDir, "request_frame": desc, "request_id": 1, "request_method": method, "request_params": params, "request_frame_ordinal": 1, "response_direction": respDir, "response_frame": desc, "response_id": 1, "response_frame_ordinal": 2, "response_status": "SUCCESS", "response_result": nil, "response_error": nil, "target_write_frame_ordinal": 10}
	}
	for _, slot := range []string{"initialize", "events"} {
		for _, method := range []string{"initialize", "client/registerCapability", "client/unregisterCapability"} {
			init := exchange("initialize")
			events := []any{exchange("client/registerCapability")}
			if slot == "initialize" {
				init = exchange(method)
			} else {
				events = []any{exchange(method)}
			}
			v := map[string]any{"role": "CAPABILITY_EVENTS", "identity": id, "original": desc, "predecessors": []any{}, "payload": map[string]any{"initialize": init, "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 3, "frame": desc}, "events": events, "target_write_frame_ordinal": 10, "observed_frames": []any{}}}
			want := slot == "initialize" && method == "initialize" || slot == "events" && method != "initialize"
			err := s.Validate(v)
			if (err == nil) != want {
				t.Errorf("slot=%s outer=%s accepted=%v want=%v: %v", slot, method, err == nil, want, err)
			}
		}
	}
	for _, method := range []string{"client/registerCapability", "client/unregisterCapability"} {
		entry := map[string]any{"id": "r"}
		key := "registrations"
		if method == "client/unregisterCapability" {
			key = "unregisterations"
		} else {
			entry["registerOptions"] = map[string]any{}
		}
		wrong := exchange(method)
		wrong["request_params"] = map[string]any{key: []any{entry}}
		v := map[string]any{"role": "CAPABILITY_EVENTS", "identity": id, "original": desc, "predecessors": []any{}, "payload": map[string]any{"initialize": exchange("initialize"), "initialized_notification": map[string]any{"direction": "CLIENT_TO_SERVER", "frame_ordinal": 3, "frame": desc}, "events": []any{wrong}, "target_write_frame_ordinal": 10, "observed_frames": []any{}}}
		if err := s.Validate(v); err == nil {
			t.Errorf("%s nested entry without target method accepted", method)
		}
		entry["method"] = "textDocument/definition" // An unrelated method remains valid chronology.
		if err := s.Validate(v); err != nil {
			t.Errorf("%s unrelated nested method rejected: %v", method, err)
		}
	}
}
