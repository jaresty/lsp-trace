package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"testing"
)

func b4bFixture(t *testing.T) B4bInput {
	t.Helper()
	write := b4aFixture(t)
	frames := []B4Frame{
		{1, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)},
		{2, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":true}}}`)},
		{3, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)},
	}
	in := B4bInput{Write: write, Frames: frames}
	b4bRebuildClaim(t, &in)
	return in
}

// This test encoder is independent of the claimant. It takes only held frames and
// WRITE custody; production never reads its generated metadata as expected data.
func b4bRebuildClaim(t *testing.T, in *B4bInput) {
	t.Helper()
	desc := func(raw []byte) map[string]any {
		return map[string]any{"length": len(raw), "sha256": "sha256:" + hash(raw), "private_ref": "held:frame"}
	}
	var fields [][]byte
	add := func(s string) { fields = append(fields, []byte(s)) }
	add("ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2")
	add(in.Write.Transaction)
	add(strconv.FormatUint(in.Write.CompletedOrdinal, 10))
	var exchanges []map[string]any
	var observed []any
	for _, f := range in.Frames {
		observed = append(observed, map[string]any{"direction": f.Direction, "frame_ordinal": f.Ordinal, "frame": desc(f.Bytes)})
	}
	for i, f := range in.Frames {
		obj, err := b4aFrame(f.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		if len(obj["method"]) == 0 || string(obj["method"]) == `"initialized"` {
			continue
		}
		var method string
		if err := json.Unmarshal(obj["method"], &method); err != nil {
			t.Fatal(err)
		}
		var id, params any
		if err := json.Unmarshal(obj["id"], &id); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(obj["params"], &params); err != nil {
			t.Fatal(err)
		}
		ex := map[string]any{"request_direction": f.Direction, "request_frame": desc(f.Bytes), "request_id": id, "request_method": method, "request_params": params, "request_frame_ordinal": f.Ordinal, "response_direction": nil, "response_frame": nil, "response_id": nil, "response_frame_ordinal": nil, "response_status": "PENDING", "response_result": nil, "response_error": nil, "target_write_frame_ordinal": in.Write.CompletedOrdinal}
		for _, r := range in.Frames[i+1:] {
			ro, e := b4aFrame(r.Bytes)
			if e != nil {
				t.Fatal(e)
			}
			if len(ro["method"]) != 0 {
				continue
			}
			rid, e := parseID(ro["id"])
			if e != nil {
				continue
			}
			qid, e := parseID(obj["id"])
			if e != nil || !qid.SameValue(rid) {
				continue
			}
			var result, failure any
			if len(ro["result"]) != 0 {
				if e := json.Unmarshal(ro["result"], &result); e != nil {
					t.Fatal(e)
				}
				ex["response_status"] = "SUCCESS"
			} else if len(ro["error"]) != 0 {
				if e := json.Unmarshal(ro["error"], &failure); e != nil {
					t.Fatal(e)
				}
				ex["response_status"] = "ERROR"
			}
			ex["response_direction"], ex["response_frame"], ex["response_id"], ex["response_frame_ordinal"] = r.Direction, desc(r.Bytes), id, r.Ordinal
			ex["response_result"], ex["response_error"] = result, failure
			break
		}
		exchanges = append(exchanges, ex)
	}
	if len(exchanges) == 0 {
		t.Fatal("fixture requires initialize")
	}
	add(strconv.Itoa(len(exchanges)))
	for _, ex := range exchanges {
		request := in.Frames[0]
		for _, f := range in.Frames {
			if f.Ordinal == ex["request_frame_ordinal"] {
				request = f
				break
			}
		}
		add(ex["request_direction"].(string))
		fields = append(fields, request.Bytes)
		id, _ := parseIDFromAny(ex["request_id"])
		add(id)
		add(ex["request_method"].(string))
		add(strconv.FormatUint(request.Ordinal, 10))
		if ex["response_frame"] == nil {
			add("ABSENT")
			fields = append(fields, nil)
			add("ABSENT")
			add("-1")
		} else {
			ordinal := ex["response_frame_ordinal"].(uint64)
			for _, f := range in.Frames {
				if f.Ordinal == ordinal {
					add(f.Direction)
					fields = append(fields, f.Bytes)
					break
				}
			}
			add(id)
			add(strconv.FormatUint(ordinal, 10))
		}
		add(ex["response_status"].(string))
	}
	var notification B4Frame
	for _, f := range in.Frames {
		obj, _ := b4aFrame(f.Bytes)
		if string(obj["method"]) == `"initialized"` {
			notification = f
			break
		}
	}
	add(notification.Direction)
	add(strconv.FormatUint(notification.Ordinal, 10))
	fields = append(fields, notification.Bytes)
	add(strconv.Itoa(len(in.Frames)))
	for _, f := range in.Frames {
		add(f.Direction)
		add(strconv.FormatUint(f.Ordinal, 10))
		fields = append(fields, f.Bytes)
	}
	var original bytes.Buffer
	for _, field := range fields {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(field)))
		original.Write(n[:])
		original.Write(field)
	}
	in.CapabilityOriginal = original.Bytes()
	payload := map[string]any{"initialize": exchanges[0], "initialized_notification": map[string]any{"direction": notification.Direction, "frame_ordinal": notification.Ordinal, "frame": desc(notification.Bytes)}, "events": exchanges[1:], "target_write_frame_ordinal": in.Write.CompletedOrdinal, "observed_frames": observed}
	envelope := map[string]any{"role": "CAPABILITY_EVENTS", "identity": map[string]any{"session": in.Write.Session, "generation": in.Write.Generation, "transaction": in.Write.Transaction}, "original": desc(in.CapabilityOriginal), "predecessors": []any{}, "payload": payload}
	in.CapabilityEnvelope = a4JSON(t, envelope)
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		t.Fatalf("positive CAPABILITY_EVENTS shape: %v", err)
	}
}

func parseIDFromAny(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	id, err := parseID(raw)
	if err != nil {
		return "", err
	}
	if id.Kind == "STRING" {
		return "string:" + id.String, nil
	}
	return "number:" + string(id.RawToken), nil
}

func TestB4bHeldStreamPositive(t *testing.T) {
	in := b4bFixture(t)
	got := CheckB4b(in)
	if got.Outcome != "SUPPORTED" {
		t.Fatalf("positive held stream: %+v", got)
	}
	t.Log("B4B_HELD_STREAM_POSITIVE")
}

func b4bChangeClaim(t *testing.T, in *B4bInput, change func(map[string]any)) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(in.CapabilityEnvelope, &envelope); err != nil {
		t.Fatal(err)
	}
	change(envelope)
	in.CapabilityEnvelope = a4JSON(t, envelope)
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		t.Fatalf("semantic guard must preserve claimant shape: %v", err)
	}
}

func TestB4bClaimedResponseIDCorrespondence(t *testing.T) {
	in := b4bFixture(t)
	b4bChangeClaim(t, &in, func(e map[string]any) {
		e["payload"].(map[string]any)["initialize"].(map[string]any)["response_id"] = "1"
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("claimed response_id differs from held numeric ID: %+v", got)
	}
}

func TestB4bClaimedResultCorrespondence(t *testing.T) {
	in := b4bFixture(t)
	b4bChangeClaim(t, &in, func(e map[string]any) {
		e["payload"].(map[string]any)["initialize"].(map[string]any)["response_result"] = map[string]any{"capabilities": map[string]any{}}
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("claimed response_result differs from held result: %+v", got)
	}
}

func TestB4bObservedOmissionCorrespondence(t *testing.T) {
	in := b4bFixture(t)
	b4bChangeClaim(t, &in, func(e map[string]any) {
		p := e["payload"].(map[string]any)
		p["observed_frames"] = p["observed_frames"].([]any)[:2]
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("observed list omitted held notification: %+v", got)
	}
}

func TestB4bOriginalArtifactSubstitution(t *testing.T) {
	in := b4bFixture(t)
	in.CapabilityOriginal = []byte("substituted artifact")
	b4bChangeClaim(t, &in, func(e map[string]any) {
		e["original"] = map[string]any{"length": len(in.CapabilityOriginal), "sha256": "sha256:" + hash(in.CapabilityOriginal), "private_ref": "held:substitution"}
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("substituted original with matching descriptor: %+v", got)
	}
}

func TestB4bRequestMirrorAndObservedFrame(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"params", func(p map[string]any) {
			p["initialize"].(map[string]any)["request_params"] = map[string]any{"changed": true}
		}},
		{"observed_frame", func(p map[string]any) {
			p["observed_frames"].([]any)[1].(map[string]any)["frame"] = map[string]any{"length": 1, "sha256": "sha256:" + hash([]byte("x")), "private_ref": "held:wrong"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bFixture(t)
			b4bChangeClaim(t, &in, func(e map[string]any) { tc.change(e["payload"].(map[string]any)) })
			if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
				t.Fatalf("claimant %s mismatch accepted: %+v", tc.name, got)
			}
		})
	}
}

func TestB4bOrdinalZeroHeldInitialize(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[0].Ordinal = 0
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("ordinal zero is a valid first held frame: %+v", got)
	}
}

func TestB4bEventOccurrenceCorrespondence(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)}, B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)})
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("valid event: %+v", got)
	}
	b4bChangeClaim(t, &in, func(e map[string]any) {
		e["payload"].(map[string]any)["events"].([]any)[0].(map[string]any)["response_id"] = "other"
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("event response ID substitution: %+v", got)
	}
}

func TestB4bLateFramesRetainedIneffective(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames = append(in.Frames, B4Frame{9, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"late","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)}, B4Frame{10, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"late","result":null}`)})
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "UNSUPPORTED" {
		t.Fatalf("late success affected earlier WRITE: %+v", got)
	}
	in.CapabilityOriginal = append([]byte(nil), in.CapabilityOriginal...)
	in.CapabilityOriginal[len(in.CapabilityOriginal)-1] ^= 1
	b4bChangeClaim(t, &in, func(e map[string]any) {
		e["original"] = map[string]any{"length": len(in.CapabilityOriginal), "sha256": "sha256:" + hash(in.CapabilityOriginal), "private_ref": "held:late-alternative"}
	})
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("late retained artifact not bound: %+v", got)
	}
}

// The claimed /2 preimage and observed list are rebuilt from the same independently
// held bytes for each case; rejection must therefore come from response accounting.
func TestB4bPostWriteResponseAccounting(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		want           string
	}{
		{"positive", "", "SUPPORTED"},
		{"duplicate_initialize", `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":true}}}`, "INVALID_CHRONOLOGY"},
		{"unmatched", `{"jsonrpc":"2.0","id":"orphan","result":null}`, "INVALID_CHRONOLOGY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bFixture(t)
			if tc.response != "" {
				in.Frames = append(in.Frames, B4Frame{9, "SERVER_TO_CLIENT", b4Frame(tc.response)})
			}
			b4bRebuildClaim(t, &in)
			if got := CheckB4b(in); got.Outcome != tc.want {
				t.Fatalf("post-WRITE %s: got %+v, want %s", tc.name, got, tc.want)
			}
		})
	}
}

func TestB4bPendingInitializeRED(t *testing.T) {
	in := b4bFixture(t)
	in.Frames = []B4Frame{in.Frames[0], in.Frames[2]}
	got := CheckB4b(in)
	if got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("pending initialize accepted: %+v", got)
	}
}

func b4bMatchHeldResponse(t *testing.T, in *B4bInput) {
	t.Helper()
	b4bRebuildClaim(t, in)
}

func TestB4bPendingRegisterUnknown(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	b4bMatchHeldResponse(t, &in)
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)})
	b4bRebuildClaim(t, &in)
	got := CheckB4b(in)
	if got.Outcome != "UNKNOWN" {
		t.Fatalf("pending registration without verified support must be UNKNOWN: %+v", got)
	}
}

func TestB4bPendingRegisterPreservesStaticSupport(t *testing.T) {
	in := b4bFixture(t)
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)})
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("pending registration erased verified static support: %+v", got)
	}
}

func TestB4bStaticWrongTypeMalformed(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":17}}}`)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "MALFORMED" {
		t.Fatalf("wrong static provider type must be MALFORMED: %+v", got)
	}
}

func TestB4bConfirmedRegisterRED(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	b4bMatchHeldResponse(t, &in)
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)}, B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)})
	b4bRebuildClaim(t, &in)
	got := CheckB4b(in)
	if got.Outcome != "SUPPORTED" {
		t.Fatalf("successful matching registration did not grant: %+v", got)
	}
}

func TestB4bStaticDynamicConflictRED(t *testing.T) {
	in := b4bFixture(t)
	in.Frames = append(in.Frames, B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)}, B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)})
	got := CheckB4b(in)
	if got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("static and dynamic coexistence: %+v", got)
	}
}

func TestB4bResponseGatedPendingUnregisterAndError(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	b4bMatchHeldResponse(t, &in)
	in.Frames = append(in.Frames,
		B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}}]}}`)},
		B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
		B4Frame{6, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"u","method":"client/unregisterCapability","params":{"unregisterations":[{"id":"x","method":"textDocument/references"}]}}`)},
	)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("pending unregister erased support: %+v", got)
	}
	in.Frames = append(in.Frames, B4Frame{7, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"u","error":{"code":-1,"message":"failed"}}`)})
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("error unregister erased support: %+v", got)
	}
	in.Frames[3].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"r","error":{"code":-1,"message":"failed"}}`)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("failed registration somehow supported unregistration: %+v", got)
	}
}

func TestB4bTypedIDAndOrderedOR(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":"1","result":{"capabilities":{}}}`)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("string ID matched numeric initialize ID: %+v", got)
	}
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1.0,"result":{"capabilities":{}}}`)
	b4bMatchHeldResponse(t, &in)
	in.Frames = append(in.Frames,
		B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":7,"method":"client/registerCapability","params":{"registrations":[{"id":"a","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"file"}]}},{"id":"b","method":"textDocument/references","registerOptions":{"documentSelector":[{"scheme":"untitled"}]}}]}}`)},
		B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":7.0,"result":null}`)},
	)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("ORDERED_OR erased proven match: %+v", got)
	}
}

// Each case changes independently held bytes, then reconstructs every claimant
// occurrence and the /2 original from those bytes before testing semantics.
func TestB4bSelectorClassification(t *testing.T) {
	for _, tc := range []struct {
		name, selectors, want string
	}{
		{"pattern_uncertain", `[{"scheme":"file","pattern":"*.go"}]`, "UNKNOWN"},
		{"scheme_ascii_case", `[{"scheme":"FILE"}]`, "SUPPORTED"},
		{"invalid_scheme_grammar", `[{"scheme":"fi*le"}]`, "MALFORMED"},
		{"invalid_scheme_first_character", `[{"scheme":"1file"}]`, "MALFORMED"},
		{"wrong_scheme_type", `[{"scheme":17}]`, "MALFORMED"},
		{"wrong_pattern_type", `[{"scheme":"file","pattern":17}]`, "MALFORMED"},
		{"wrong_language_type", `[{"scheme":"file","language":17}]`, "MALFORMED"},
		{"wrong_notebook_type", `[{"notebook":17}]`, "MALFORMED"},
		{"language_unavailable", `[{"scheme":"file","language":"go"}]`, "UNKNOWN"},
		{"ordinary_notebook_filter", `[{"scheme":"file","notebook":"file:///notebook"}]`, "UNSUPPORTED"},
		{"missing_scheme", `[{"pattern":"*.go"}]`, "UNKNOWN"},
		{"pattern_then_proven_match", `[{"scheme":"file","pattern":"*.go"},{"scheme":"file"}]`, "SUPPORTED"},
		{"proven_match_then_pattern", `[{"scheme":"file"},{"scheme":"file","pattern":"*.go"}]`, "SUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bFixture(t)
			in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
			in.Frames = append(in.Frames,
				B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":` + tc.selectors + `}}]}}`)},
				B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
			)
			b4bRebuildClaim(t, &in)
			if got := CheckB4b(in); got.Outcome != tc.want {
				t.Fatalf("selector %s: got %+v, want %s", tc.name, got, tc.want)
			}
		})
	}
}

// The fixtures rebuild the claimant and /2 artifact from independently held frames.
func b4bSelectorCase(t *testing.T, selector string) B4bInput {
	t.Helper()
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames = append(in.Frames,
		B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":{"documentSelector":` + selector + `}}]}}`)},
		B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
	)
	b4bRebuildClaim(t, &in)
	return in
}

func b4bHeldOrdinaryLanguage(t *testing.T, selectorJSON, language string) B4bInput {
	t.Helper()
	in := b4bSelectorCase(t, selectorJSON)
	in.Write.Language = &language
	app := &in.Write.Claims[2]
	var envelope map[string]any
	if err := json.Unmarshal(app.Envelope, &envelope); err != nil {
		t.Fatal(err)
	}
	payload := envelope["payload"].(map[string]any)
	tx := selector("ADR0011-GENERIC-TRANSACTION/1", in.Write.Session, "1", in.Write.Transaction)
	appArt := b4aArtifact("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1", tx, payload["query_selector"].(string), payload["query_source_selector"].(string), in.Write.URI, "present", language, "ORDINARY", "absent", "", "absent", "", "", in.Write.Workspace, in.Write.Session, "1", in.Write.Transaction)
	in.Write.ApplicabilityOriginal = appArt
	app.Original, app.Artifact = appArt, appArt
	payload["language_id_present"] = true
	payload["language_id_bytes"] = map[string]any{"length": len(language), "sha256": "sha256:" + hash([]byte(language)), "private_ref": "held:language"}
	envelope["original"] = map[string]any{"length": len(appArt), "sha256": "sha256:" + hash(appArt), "private_ref": "held:app"}
	app.Envelope = a4JSON(t, envelope)
	write := &in.Write.Claims[3]
	var writeEnvelope map[string]any
	if err := json.Unmarshal(write.Envelope, &writeEnvelope); err != nil {
		t.Fatal(err)
	}
	appSelector, err := A4Selector("references", "QUERY_APPLICABILITY", in.Write.Session, "1", appArt, tx)
	if err != nil {
		t.Fatal(err)
	}
	for _, pred := range writeEnvelope["predecessors"].([]any) {
		p := pred.(map[string]any)
		if p["role"] == "QUERY_APPLICABILITY" {
			p["selector"], p["digest"] = appSelector, "sha256:"+hash(appArt)
		}
	}
	write.Envelope = a4JSON(t, writeEnvelope)
	if err := CheckB4a(in.Write); err != nil {
		t.Fatalf("held language positive B4a control: %v", err)
	}
	return in
}

func TestB4bHeldLanguageExactMatch(t *testing.T) {
	if got := CheckB4b(b4bHeldOrdinaryLanguage(t, `[{"language":"go"}]`, "go")); got.Outcome != "SUPPORTED" {
		t.Fatalf("held language exact match must support: %+v", got)
	}
}

// Build a genuine B4a-held notebook cell: the cell URI is deliberately file:
// while the enclosing notebook URI uses vscode-notebook:.
func b4bHeldCell(t *testing.T, selectorJSON string) B4bInput {
	return b4bHeldCellLanguage(t, selectorJSON, "")
}

func b4bHeldCellLanguage(t *testing.T, selectorJSON, language string) B4bInput {
	t.Helper()
	in := b4bSelectorCase(t, selectorJSON)
	notebookType, notebookURI := "jupyter-notebook", "vscode-notebook:///workspace/book.ipynb"
	in.Write.NotebookType, in.Write.NotebookURI = &notebookType, &notebookURI
	in.Write.NotebookSourceSelector = "held:notebook-source"
	app := &in.Write.Claims[2]
	var envelope map[string]any
	if err := json.Unmarshal(app.Envelope, &envelope); err != nil {
		t.Fatal(err)
	}
	payload := envelope["payload"].(map[string]any)
	tx := selector("ADR0011-GENERIC-TRANSACTION/1", in.Write.Session, "1", in.Write.Transaction)
	languagePresence := "absent"
	if language != "" {
		languagePresence = "present"
		in.Write.Language = &language
		payload["language_id_present"] = true
		payload["language_id_bytes"] = map[string]any{"length": len(language), "sha256": "sha256:" + hash([]byte(language)), "private_ref": "held:language"}
	}
	art := b4aArtifact("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1", tx, payload["query_selector"].(string), payload["query_source_selector"].(string), in.Write.URI, languagePresence, language, "NOTEBOOK_CELL", "present", notebookType, "present", notebookURI, in.Write.NotebookSourceSelector, in.Write.Workspace, in.Write.Session, "1", in.Write.Transaction)
	in.Write.ApplicabilityOriginal = art
	app.Original, app.Artifact = art, art
	payload["document_kind"] = "NOTEBOOK_CELL"
	payload["notebook_type_bytes"] = map[string]any{"length": len(notebookType), "sha256": "sha256:" + hash([]byte(notebookType)), "private_ref": "held:notebook-type"}
	payload["notebook_uri_bytes"] = map[string]any{"length": len(notebookURI), "sha256": "sha256:" + hash([]byte(notebookURI)), "private_ref": "held:notebook-uri"}
	payload["notebook_source_selector"] = in.Write.NotebookSourceSelector
	envelope["original"] = map[string]any{"length": len(art), "sha256": "sha256:" + hash(art), "private_ref": "held:app"}
	app.Envelope = a4JSON(t, envelope)
	write := &in.Write.Claims[3]
	var w map[string]any
	if err := json.Unmarshal(write.Envelope, &w); err != nil {
		t.Fatal(err)
	}
	appSelector, err := A4Selector("references", "QUERY_APPLICABILITY", in.Write.Session, "1", art, tx)
	if err != nil {
		t.Fatal(err)
	}
	for _, predecessor := range w["predecessors"].([]any) {
		p := predecessor.(map[string]any)
		if p["role"] == "QUERY_APPLICABILITY" {
			p["selector"], p["digest"] = appSelector, "sha256:"+hash(art)
		}
	}
	write.Envelope = a4JSON(t, w)
	if err := CheckB4a(in.Write); err != nil {
		t.Fatalf("held cell fixture B4a: %v", err)
	}
	return in
}

func TestB4bHeldCellFixtureCompiles(t *testing.T) {
	in := b4bHeldCell(t, `[{"notebook":"jupyter-notebook"}]`)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("held cell control: %+v", got)
	}
}

func TestB4bHeldCellLanguageFixtureCompiles(t *testing.T) {
	in := b4bHeldCellLanguage(t, `[{"notebook":"jupyter-notebook","language":"python"}]`, "python")
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("held cell with independently held language and parent: %+v", got)
	}
}

func TestB4bNotebookSelectorSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, selector, want string
		cell                 bool
	}{
		{"held_type_string", `[{"notebook":"jupyter-notebook"}]`, "SUPPORTED", true},
		{"held_type_wildcard", `[{"notebook":"*"}]`, "SUPPORTED", true},
		{"held_type_mismatch", `[{"notebook":"other-notebook"}]`, "UNSUPPORTED", true},
		{"held_parent_scheme", `[{"notebook":{"notebookType":"jupyter-notebook","scheme":"vscode-notebook"}}]`, "SUPPORTED", true},
		{"cell_scheme_not_parent", `[{"notebook":{"scheme":"file"}}]`, "UNSUPPORTED", true},
		{"pattern_uncertain", `[{"notebook":{"notebookType":"jupyter-notebook","pattern":"*.ipynb"}}]`, "UNKNOWN", true},
		{"wrong_known_child", `[{"notebook":{"notebookType":17}}]`, "MALFORMED", true},
		{"invalid_child_scheme", `[{"notebook":{"scheme":"1bad"}}]`, "MALFORMED", true},
		{"empty_object", `[{"notebook":{}}]`, "MALFORMED", true},
		{"extension_only_no_positive", `[{"notebook":{"future":true}}]`, "UNKNOWN", true},
		{"extension_with_proven_type", `[{"notebook":{"notebookType":"jupyter-notebook","future":true}}]`, "SUPPORTED", true},
		{"empty_string_type", `[{"notebook":""}]`, "MALFORMED", true},
		{"ordinary_notebook_only", `[{"notebook":"jupyter-notebook"}]`, "UNSUPPORTED", false},
		{"proven_then_unknown", `[{"notebook":"jupyter-notebook"},{"notebook":{"pattern":"*.ipynb"}}]`, "SUPPORTED", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var in B4bInput
			if tc.cell {
				in = b4bHeldCell(t, tc.selector)
			} else {
				in = b4bSelectorCase(t, tc.selector)
			}
			if got := CheckB4b(in); got.Outcome != tc.want {
				t.Fatalf("notebook selector %s: got %+v, want %s", tc.name, got, tc.want)
			}
		})
	}
}

func TestB4bPresentNonObjectRegisterOptionsMalformed(t *testing.T) {
	for _, tc := range []struct{ name, options string }{
		{"null", `null`}, {"array", `[]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bFixture(t)
			in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
			in.Frames = append(in.Frames,
				B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":` + tc.options + `}]}}`)},
				B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
			)
			b4bRebuildClaim(t, &in)
			if got := CheckB4b(in); got.Outcome != "MALFORMED" {
				t.Fatalf("present non-object registerOptions %s: got %+v, want MALFORMED", tc.name, got)
			}
		})
	}
}

// Every case rebuilds the claimant and /2 original from independently held frames.
// The successful response and static provider coexist before the target WRITE.
func TestB4bStaticDynamicMalformedPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, options, want string }{
		{"null_options", `null`, "MALFORMED"},
		{"wrong_known_scheme_type", `{"documentSelector":[{"scheme":17}]}`, "MALFORMED"},
		{"valid_options", `{"documentSelector":[{"scheme":"file"}]}`, "INVALID_CHRONOLOGY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bFixture(t) // independently held referencesProvider:true
			in.Frames = append(in.Frames,
				B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references","registerOptions":` + tc.options + `}]}}`)},
				B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
			)
			b4bRebuildClaim(t, &in)
			if got := CheckB4b(in); got.Outcome != tc.want {
				t.Fatalf("static plus successful dynamic %s: got %s (%s), want %s", tc.name, got.Outcome, got.Cause, tc.want)
			}
		})
	}
}

func TestB4bLanguageMismatchInvalidSchemeGrammarBeforeChronology(t *testing.T) {
	in := b4bHeldOrdinaryLanguage(t, `[{"language":"python","scheme":"1bad"}]`, "go")
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":true}}}`)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "MALFORMED" {
		t.Fatalf("language mismatch cannot hide invalid scheme grammar before static/dynamic chronology: got %s (%s), want MALFORMED", got.Outcome, got.Cause)
	}
}

func TestB4bCompleteSelectorValidationBeforeApplicability(t *testing.T) {
	for _, tc := range []struct{ name, selector, language, want string }{
		{"matching_language_bad_scheme", `[{"language":"go","scheme":"1bad"}]`, "go", "MALFORMED"},
		{"supported_then_bad", `[{"scheme":"file"},{"scheme":"1bad"}]`, "go", "MALFORMED"},
		{"unsupported_then_bad", `[{"scheme":"untitled"},{"scheme":"1bad"}]`, "go", "MALFORMED"},
		{"ordinary_bad_notebook_child", `[{"notebook":{"scheme":"1bad"}}]`, "go", "MALFORMED"},
		{"unknown_inert_extension", `[{"scheme":"file","future":true}]`, "go", "SUPPORTED"},
		{"valid_static_dynamic", `[{"scheme":"file"}]`, "go", "INVALID_CHRONOLOGY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := b4bHeldOrdinaryLanguage(t, tc.selector, tc.language)
			if tc.name == "valid_static_dynamic" {
				in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":true}}}`)
				b4bRebuildClaim(t, &in)
			}
			if got := CheckB4b(in); got.Outcome != tc.want {
				t.Fatalf("complete selector %s: got %s (%s), want %s", tc.name, got.Outcome, got.Cause, tc.want)
			}
		})
	}
}

func TestB4bAbsentRegisterOptionsUnknown(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	in.Frames = append(in.Frames,
		B4Frame{4, "SERVER_TO_CLIENT", b4Frame(`{"jsonrpc":"2.0","id":"r","method":"client/registerCapability","params":{"registrations":[{"id":"x","method":"textDocument/references"}]}}`)},
		B4Frame{5, "CLIENT_TO_SERVER", b4Frame(`{"jsonrpc":"2.0","id":"r","result":null}`)},
	)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "UNKNOWN" {
		t.Fatalf("absent registerOptions: got %+v, want UNKNOWN", got)
	}
}

func TestB4bNotebookLanguageStarMatchesHeldPython(t *testing.T) {
	in := b4bHeldCellLanguage(t, `[{"notebook":"jupyter-notebook","language":"*"}]`, "python")
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("notebook language star: got %+v, want SUPPORTED", got)
	}
}

func TestB4bOrdinaryLanguageStarRemainsExact(t *testing.T) {
	in := b4bHeldOrdinaryLanguage(t, `[{"language":"*"}]`, "python")
	if got := CheckB4b(in); got.Outcome != "UNSUPPORTED" {
		t.Fatalf("ordinary language star: got %+v, want UNSUPPORTED", got)
	}
}

func TestB4bEmptySelectorMalformed(t *testing.T) {
	in := b4bSelectorCase(t, `[]`)
	if got := CheckB4b(in); got.Outcome != "MALFORMED" {
		t.Fatalf("empty selector must be malformed: %+v", got)
	}
}

func TestB4bStaticOptionsWrongKnownType(t *testing.T) {
	in := b4bFixture(t)
	in.Frames[1].Bytes = b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"referencesProvider":{"workDoneProgress":"yes"}}}}`)
	b4bRebuildClaim(t, &in)
	if got := CheckB4b(in); got.Outcome != "MALFORMED" {
		t.Fatalf("wrong known static option type must be malformed: %+v", got)
	}
}

func TestB4bNullSelectorHeldClientOrUnknown(t *testing.T) {
	in := b4bSelectorCase(t, `null`)
	if got := CheckB4b(in); got.Outcome != "UNKNOWN" {
		t.Fatalf("missing held selector must remain unknown: %+v", got)
	}
	in.HeldClientSelector = []byte(`[{"scheme":"file"}]`)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("independently held client selector must be evaluated: %+v", got)
	}
}

func TestB4bNullSelectorHeldClientPositive(t *testing.T) {
	in := b4bSelectorCase(t, `null`)
	in.HeldClientSelector = []byte(`[{"scheme":"file"}]`)
	if got := CheckB4b(in); got.Outcome != "SUPPORTED" {
		t.Fatalf("independently held client selector must support null documentSelector: got %s (%s), want SUPPORTED", got.Outcome, got.Cause)
	}
}

func TestB4bClaimantFrameSubstitutionRED(t *testing.T) {
	in := b4bFixture(t)
	var envelope map[string]any
	if err := json.Unmarshal(in.CapabilityEnvelope, &envelope); err != nil {
		t.Fatal(err)
	}
	exchange := envelope["payload"].(map[string]any)["initialize"].(map[string]any)
	alternative := b4Frame(`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`)
	exchange["response_frame"] = map[string]any{"length": len(alternative), "sha256": "sha256:" + hash(alternative), "private_ref": "held:alternative"}
	in.CapabilityEnvelope = a4JSON(t, envelope)
	if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		t.Fatalf("claimant shape changed: %v", err)
	}
	if got := CheckB4b(in); got.Outcome != "INVALID_CHRONOLOGY" {
		t.Fatalf("claimant response frame contradicts held original stream: %+v", got)
	}
}
