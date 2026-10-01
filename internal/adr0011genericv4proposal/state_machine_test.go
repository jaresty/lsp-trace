package adr0011genericv4proposal

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This is a test-only contract oracle, never a B4 replay or an issuer.
type heldCapabilityExchange struct {
	kind         string
	request      candidateFrame
	response     *candidateFrame
	notification *candidateFrame // Complete client→server initialized notification; only for initialize.
}
type heldCapabilityFrame struct {
	direction string
	ordinal   int
	raw       []byte
}
type heldSnapshot struct {
	status string
	active []string
}

func testExchange(t *testing.T, kind string, requestID, requestOrdinal, responseOrdinal int, entries []map[string]any, success bool) heldCapabilityExchange {
	t.Helper()
	wire := func(v any) []byte {
		body, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append([]byte("Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
	}
	method, reqDir, respDir, key := "initialize", "CLIENT_TO_SERVER", "SERVER_TO_CLIENT", ""
	if kind == "register" {
		method, reqDir, respDir, key = "client/registerCapability", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "registrations"
	}
	if kind == "unregister" {
		method, reqDir, respDir, key = "client/unregisterCapability", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "unregisterations"
	}
	params := map[string]any{}
	if key != "" {
		params[key] = entries
	}
	r := candidateFrame{reqDir, requestOrdinal, wire(map[string]any{"jsonrpc": "2.0", "id": requestID, "method": method, "params": params})}
	var response *candidateFrame
	if responseOrdinal >= 0 {
		v := map[string]any{"jsonrpc": "2.0", "id": requestID, "result": nil}
		if kind == "initialize" {
			v["result"] = map[string]any{"capabilities": map[string]any{}}
		}
		if !success {
			delete(v, "result")
			v["error"] = map[string]any{"code": -32603, "message": "failure"}
		}
		response = &candidateFrame{respDir, responseOrdinal, wire(v)}
	}
	var notification *candidateFrame
	if kind == "initialize" && response != nil && success {
		notification = &candidateFrame{"CLIENT_TO_SERVER", responseOrdinal + 1, wire(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})}
	}
	return heldCapabilityExchange{kind: kind, request: r, response: response, notification: notification}
}
func reg(id, method, language string) map[string]any {
	selector := []any{map[string]any{"language": language}}
	if language == "UNKNOWN" {
		selector = []any{map[string]any{"pattern": "**/*.go"}}
	}
	return map[string]any{"id": id, "method": method, "registerOptions": map[string]any{"documentSelector": selector}}
}
func regWithMatchAndUnknown(id, method string, patternFirst bool) map[string]any {
	entry := reg(id, method, "go")
	match := map[string]any{"language": "go"}
	unknown := map[string]any{"pattern": "**/*.go"}
	filters := []any{match, unknown}
	if patternFirst {
		filters = []any{unknown, match}
	}
	entry["registerOptions"] = map[string]any{"documentSelector": filters}
	return entry
}
func unreg(id, method string) map[string]any { return map[string]any{"id": id, "method": method} }
func stream(exchanges []heldCapabilityExchange) []candidateFrame {
	var frames []candidateFrame
	for _, x := range exchanges {
		frames = append(frames, x.request)
		if x.response != nil {
			frames = append(frames, *x.response)
		}
		if x.notification != nil {
			frames = append(frames, *x.notification)
		}
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i].ordinal < frames[j].ordinal })
	return frames
}
func candidateEntrySelector(entry map[string]json.RawMessage, queryLanguage string) (string, error) {
	var options struct {
		DocumentSelector []map[string]json.RawMessage `json:"documentSelector"`
	}
	if json.Unmarshal(entry["registerOptions"], &options) != nil {
		return "", fmt.Errorf("invalid registration options")
	}
	match, unknown := false, len(options.DocumentSelector) == 0
	for _, filter := range options.DocumentSelector {
		if _, ok := filter["pattern"]; ok {
			unknown = true
			continue
		}
		var language string
		if json.Unmarshal(filter["language"], &language) == nil && language == queryLanguage {
			match = true
		}
	}
	if match {
		return "MATCH", nil
	}
	if unknown {
		return "UNKNOWN", nil
	}
	return "NO_MATCH", nil
}
func stateAt(exchanges []heldCapabilityExchange, frames []candidateFrame, write int, method string, static map[string]bool, queryLanguage, claimantSummary string) heldSnapshot {
	_ = claimantSummary // A claimant result is never a state input.
	invalid := heldSnapshot{status: "INVALID_CHRONOLOGY"}
	type transition struct {
		ordinal, requestOrdinal int
		kind                    string
		success                 bool
		entries                 []map[string]json.RawMessage
	}
	var transitions []transition
	var pendingRegistrations [][]map[string]json.RawMessage
	var expected []candidateFrame
	initialized, initializedOrdinal, initializeCount := false, -1, 0
	for _, x := range exchanges {
		if x.request.ordinal >= write {
			continue
		}
		request, err := decodeCandidateFrame(x.request.raw)
		if err != nil || string(request["jsonrpc"]) != `"2.0"` || len(request["id"]) == 0 {
			return invalid
		}
		var outer string
		if json.Unmarshal(request["method"], &outer) != nil {
			return invalid
		}
		want, requestDirection, responseDirection := "initialize", "CLIENT_TO_SERVER", "SERVER_TO_CLIENT"
		if x.kind == "register" {
			want, requestDirection, responseDirection = "client/registerCapability", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER"
		}
		if x.kind == "unregister" {
			want, requestDirection, responseDirection = "client/unregisterCapability", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER"
		}
		if x.kind != "initialize" && x.kind != "register" && x.kind != "unregister" || outer != want || x.request.direction != requestDirection {
			return invalid
		}
		if x.kind == "initialize" {
			initializeCount++
			if initializeCount != 1 {
				return invalid
			}
		} else if initializedOrdinal < 0 || x.request.ordinal <= initializedOrdinal || x.notification != nil {
			return invalid
		}
		expected = append(expected, x.request)
		var params map[string]json.RawMessage
		if json.Unmarshal(request["params"], &params) != nil {
			return invalid
		}
		key := "registrations"
		if x.kind == "unregister" {
			key = "unregisterations"
		}
		var entries []map[string]json.RawMessage
		if x.kind != "initialize" {
			if json.Unmarshal(params[key], &entries) != nil || len(entries) == 0 {
				return invalid
			}
		}
		if x.response == nil || x.response.ordinal >= write {
			if x.kind == "initialize" {
				return invalid
			}
			if x.kind == "register" {
				pendingRegistrations = append(pendingRegistrations, entries)
			}
			continue
		}
		r := *x.response
		if r.ordinal <= x.request.ordinal || r.direction != responseDirection {
			return invalid
		}
		response, err := decodeCandidateFrame(r.raw)
		if err != nil || string(response["jsonrpc"]) != `"2.0"` {
			return invalid
		}
		if _, hasMethod := response["method"]; hasMethod {
			return invalid
		}
		if !strings.EqualFold(string(request["id"]), string(response["id"])) || string(request["id"]) != string(response["id"]) {
			return invalid
		}
		resultRaw, hasResult := response["result"]
		errorRaw, hasError := response["error"]
		if hasResult == hasError {
			return invalid
		}
		if hasError {
			if x.kind == "initialize" {
				return invalid
			}
			var body map[string]json.RawMessage
			if json.Unmarshal(errorRaw, &body) != nil || body == nil || len(body["code"]) == 0 || len(body["message"]) == 0 {
				return invalid
			}
			for name := range body {
				if name != "code" && name != "message" && name != "data" {
					return invalid
				}
			}
			if _, err := strconv.ParseInt(string(body["code"]), 10, 64); err != nil {
				return invalid
			}
			var message string
			if json.Unmarshal(body["message"], &message) != nil {
				return invalid
			}
		} else if x.kind == "initialize" {
			var body map[string]json.RawMessage
			if json.Unmarshal(resultRaw, &body) != nil || body == nil {
				return invalid
			}
			var capabilities map[string]json.RawMessage
			if json.Unmarshal(body["capabilities"], &capabilities) != nil || capabilities == nil {
				return invalid
			}
		} else if string(resultRaw) != "null" {
			return invalid
		}
		expected = append(expected, r)
		if x.kind == "initialize" {
			if x.notification == nil || x.notification.direction != "CLIENT_TO_SERVER" || x.notification.ordinal <= r.ordinal || x.notification.ordinal >= write {
				return invalid
			}
			n, err := decodeCandidateFrame(x.notification.raw)
			if err != nil || len(n) != 3 || string(n["jsonrpc"]) != `"2.0"` || string(n["method"]) != `"initialized"` {
				return invalid
			}
			var params map[string]json.RawMessage
			if json.Unmarshal(n["params"], &params) != nil || params == nil || len(params) != 0 {
				return invalid
			}
			initializedOrdinal = x.notification.ordinal
			expected = append(expected, *x.notification)
		}
		if !hasError || x.kind == "unregister" {
			transitions = append(transitions, transition{r.ordinal, x.request.ordinal, x.kind, !hasError, entries})
		}

	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].ordinal < expected[j].ordinal })
	var observed []candidateFrame
	for _, f := range frames {
		if f.ordinal < write {
			observed = append(observed, f)
		}
	}
	if len(observed) != len(expected) {
		return invalid
	}
	for i, f := range observed {
		if i > 0 && f.ordinal <= observed[i-1].ordinal || f.ordinal != expected[i].ordinal || f.direction != expected[i].direction || string(f.raw) != string(expected[i].raw) {
			return invalid
		}
	}
	sort.Slice(transitions, func(i, j int) bool { return transitions[i].ordinal < transitions[j].ordinal })
	for i := 1; i < len(transitions); i++ {
		if transitions[i].ordinal == transitions[i-1].ordinal {
			return invalid
		}
	}
	type registration struct {
		method, selector  string
		request, response int
	}
	active := map[string]registration{}
	for _, tr := range transitions {
		if tr.kind == "initialize" {
			initialized = true
			continue
		}
		if !initialized {
			return invalid
		}
		for _, entry := range tr.entries {
			var id, target string
			if json.Unmarshal(entry["id"], &id) != nil || id == "" || json.Unmarshal(entry["method"], &target) != nil || target == "" {
				return invalid
			}
			if tr.kind == "unregister" {
				a, ok := active[id]
				if !ok || a.method != target {
					return invalid
				}
				if tr.success {
					delete(active, id)
				}
				continue
			}
			if _, exists := active[id]; exists {
				return invalid
			}
			selector, err := candidateEntrySelector(entry, queryLanguage)
			if err != nil {
				return invalid
			}
			active[id] = registration{target, selector, tr.requestOrdinal, tr.ordinal}
		}
	}
	if !initialized || initializeCount != 1 || initializedOrdinal < 0 {
		return invalid
	}
	result := heldSnapshot{status: "UNSUPPORTED"}
	if static[method] {
		result.status = "SUPPORTED"
	}
	ids := make([]string, 0, len(active))
	for id := range active {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	unknown, matching := false, false
	for _, id := range ids {
		a := active[id]
		if a.method != method {
			continue
		}
		if static[method] {
			return invalid
		}
		result.active = append(result.active, fmt.Sprintf("%s@%d:%d", id, a.request, a.response))
		if a.selector == "UNKNOWN" {
			unknown = true
		}
		if a.selector == "MATCH" {
			matching = true
		}
	}
	if matching {
		result.status = "SUPPORTED"
	} else if unknown {
		result.status = "UNKNOWN"
	}
	if result.status != "SUPPORTED" {
		for _, entries := range pendingRegistrations {
			for _, entry := range entries {
				var target string
				if json.Unmarshal(entry["method"], &target) != nil || target == "" {
					return invalid
				}
				if target != method {
					continue
				}
				selector, err := candidateEntrySelector(entry, queryLanguage)
				if err != nil {
					return invalid
				}
				if selector == "MATCH" || selector == "UNKNOWN" {
					result.status = "UNKNOWN"
				}
			}
		}
	}
	return result
}
func TestV4PrematureInitializationIsInvalid(t *testing.T) {
	ref := "textDocument/references"
	for _, tc := range []struct {
		name string
		init heldCapabilityExchange
	}{
		{"initialize pending at target WRITE", testExchange(t, "initialize", 1, 0, -1, nil, true)},
		{"initialize error response", testExchange(t, "initialize", 1, 0, 1, nil, false)},
	} {
		got := stateAt([]heldCapabilityExchange{tc.init}, stream([]heldCapabilityExchange{tc.init}), 5, ref, map[string]bool{ref: true}, "go", "SAME_CLAIM")
		if got.status != "INVALID_CHRONOLOGY" {
			t.Errorf("%s: got %s want INVALID_CHRONOLOGY", tc.name, got.status)
		}
	}
}
func TestV4InitializedNotificationChronology(t *testing.T) {
	ref, def := "textDocument/references", "textDocument/definition"
	valid := testExchange(t, "initialize", 1, 0, 1, nil, true) // response 1, initialized 2
	register := testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true)
	unregister := testExchange(t, "unregister", 3, 3, 4, []map[string]any{unreg("r", ref)}, true)
	check := func(name string, exchanges []heldCapabilityExchange, frames []candidateFrame, write int, method string, static map[string]bool, want string) {
		t.Helper()
		got := stateAt(exchanges, frames, write, method, static, "go", "SAME_CLAIM")
		if got.status != want {
			t.Errorf("%s: got %s want %s", name, got.status, want)
		}
	}
	for _, method := range []string{ref, def} {
		check("static immediately after initialized "+method, []heldCapabilityExchange{valid}, stream([]heldCapabilityExchange{valid}), 3, method, map[string]bool{method: true}, "SUPPORTED")
		check("initialize pending at target WRITE "+method, []heldCapabilityExchange{testExchange(t, "initialize", 1, 0, -1, nil, true)}, stream([]heldCapabilityExchange{testExchange(t, "initialize", 1, 0, -1, nil, true)}), 3, method, map[string]bool{method: true}, "INVALID_CHRONOLOGY")
		check("initialize error response "+method, []heldCapabilityExchange{testExchange(t, "initialize", 1, 0, 1, nil, false)}, stream([]heldCapabilityExchange{testExchange(t, "initialize", 1, 0, 1, nil, false)}), 3, method, nil, "INVALID_CHRONOLOGY")
		missing := testExchange(t, "initialize", 1, 0, 1, nil, true)
		missing.notification = nil
		check("missing initialized notification "+method, []heldCapabilityExchange{missing}, stream([]heldCapabilityExchange{missing}), 3, method, nil, "INVALID_CHRONOLOGY")
		before := testExchange(t, "initialize", 1, 0, 3, nil, true)
		before.notification.ordinal = 2
		check("initialized notification before response "+method, []heldCapabilityExchange{before}, stream([]heldCapabilityExchange{before}), 5, method, nil, "INVALID_CHRONOLOGY")
		duplicate := stream([]heldCapabilityExchange{valid})
		duplicate = append(duplicate, candidateFrame{"CLIENT_TO_SERVER", 3, valid.notification.raw})
		check("duplicate initialized notification "+method, []heldCapabilityExchange{valid}, duplicate, 4, method, nil, "INVALID_CHRONOLOGY")
		check("missing notification before target WRITE "+method, []heldCapabilityExchange{valid}, stream([]heldCapabilityExchange{valid}), 2, method, map[string]bool{method: true}, "INVALID_CHRONOLOGY")
	}
	for _, e := range []struct {
		name     string
		exchange heldCapabilityExchange
	}{
		{"register request before initialize response", testExchange(t, "register", 2, 1, 5, []map[string]any{reg("r", ref, "go")}, true)},
		{"unregister request before initialize response", testExchange(t, "unregister", 2, 1, 5, []map[string]any{unreg("r", ref)}, true)},
	} {
		init := testExchange(t, "initialize", 1, 0, 2, nil, true)
		all := []heldCapabilityExchange{init, e.exchange}
		check(e.name, all, stream(all), 7, ref, nil, "INVALID_CHRONOLOGY")
	}
	initLateNotification := testExchange(t, "initialize", 1, 0, 2, nil, true)
	initLateNotification.notification.ordinal = 5
	between := testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true)
	all := []heldCapabilityExchange{initLateNotification, between}
	check("registration response after initialize response but before initialized", all, stream(all), 7, ref, nil, "INVALID_CHRONOLOGY")
	for _, event := range []heldCapabilityExchange{register, unregister} {
		all = []heldCapabilityExchange{valid, event}
		want := "SUPPORTED"
		if event.kind == "unregister" {
			want = "INVALID_CHRONOLOGY" // unknown active ID, not a valid unregister.
		}
		check("valid initialized then "+event.kind, all, stream(all), 5, ref, nil, want)
	}
	validDef := testExchange(t, "register", 2, 3, 4, []map[string]any{reg("d", def, "go")}, true)
	all = []heldCapabilityExchange{valid, validDef}
	check("valid initialized then definition registration", all, stream(all), 5, def, nil, "SUPPORTED")
	claimed := []heldCapabilityExchange{valid}
	check("unchanged claimant, original held notification ordinal", claimed, stream(claimed), 5, ref, map[string]bool{ref: true}, "SUPPORTED")
	altered := stream(claimed)
	for i := range altered {
		if altered[i].ordinal == valid.notification.ordinal {
			altered[i].ordinal = 4
		}
	}
	check("unchanged claimant, changed held notification ordinal", claimed, altered, 5, ref, map[string]bool{ref: true}, "INVALID_CHRONOLOGY")
}
func TestV4HeldCapabilityStateAtWrites(t *testing.T) {
	ref, def := "textDocument/references", "textDocument/definition"
	init := testExchange(t, "initialize", 1, 0, 1, nil, true)
	steps := []heldCapabilityExchange{init,
		testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r1", ref, "go")}, true),
		testExchange(t, "register", 3, 6, 7, []map[string]any{reg("r2", ref, "go")}, true),
		testExchange(t, "unregister", 4, 9, 10, []map[string]any{unreg("r1", ref)}, true),
		testExchange(t, "unregister", 5, 12, 13, []map[string]any{unreg("r2", ref)}, false),
		testExchange(t, "unregister", 6, 15, 16, []map[string]any{unreg("r2", ref)}, true),
	}
	for _, tc := range []struct {
		write  int
		status string
		active []string
	}{
		{3, "UNSUPPORTED", nil}, {5, "SUPPORTED", []string{"r1@3:4"}}, {8, "SUPPORTED", []string{"r1@3:4", "r2@6:7"}},
		{11, "SUPPORTED", []string{"r2@6:7"}}, {14, "SUPPORTED", []string{"r2@6:7"}}, {17, "UNSUPPORTED", nil},
	} {
		got := stateAt(steps, stream(steps), tc.write, ref, nil, "go", "CLAIMANT_UNCHANGED")
		if got.status != tc.status || fmt.Sprint(got.active) != fmt.Sprint(tc.active) {
			t.Errorf("six-step/write=%d got %+v want %s %v", tc.write, got, tc.status, tc.active)
		}
	}
	cases := []struct {
		name   string
		events []heldCapabilityExchange
		write  int
		method string
		static map[string]bool
		status string
		active []string
	}{
		{"failed registration", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, false)}, 5, ref, nil, "UNSUPPORTED", nil},
		{"pending registration", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, -1, []map[string]any{reg("r", ref, "go")}, true)}, 5, ref, nil, "UNKNOWN", nil},
		{"pending nonmatching registration", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, -1, []map[string]any{reg("r", ref, "python")}, true)}, 5, ref, nil, "UNSUPPORTED", nil},
		{"pending unrelated registration", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, -1, []map[string]any{reg("r", def, "go")}, true)}, 5, ref, nil, "UNSUPPORTED", nil},
		{"late response earlier WRITE", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 11, []map[string]any{reg("r", ref, "go")}, true)}, 10, ref, nil, "UNKNOWN", nil},
		{"late response later WRITE", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 11, []map[string]any{reg("r", ref, "go")}, true)}, 12, ref, nil, "SUPPORTED", []string{"r@3:11"}},
		{"definition pending at WRITE", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 11, []map[string]any{reg("d", def, "go")}, true)}, 10, def, nil, "UNKNOWN", nil},
		{"definition later response", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 11, []map[string]any{reg("d", def, "go")}, true)}, 12, def, nil, "SUPPORTED", []string{"d@3:11"}},
		{"pending with independently static support", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, -1, []map[string]any{reg("r", ref, "go")}, true)}, 5, ref, map[string]bool{ref: true}, "SUPPORTED", nil},
		{"out-of-order pending responses", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 8, []map[string]any{reg("r1", ref, "go")}, true), testExchange(t, "register", 3, 4, 6, []map[string]any{reg("r2", ref, "go")}, true)}, 9, ref, nil, "SUPPORTED", []string{"r1@3:8", "r2@4:6"}},
		{"one applicable one nonmatching", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r1", ref, "python"), reg("r2", ref, "go")}, true)}, 5, ref, nil, "SUPPORTED", []string{"r1@3:4", "r2@3:4"}},
		{"all nonmatching", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "python")}, true)}, 5, ref, nil, "UNSUPPORTED", []string{"r@3:4"}},
		{"unknown selector", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "UNKNOWN")}, true)}, 5, ref, nil, "UNKNOWN", []string{"r@3:4"}},
		{"references active match plus unknown", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("a", ref, "go"), reg("u", ref, "UNKNOWN")}, true)}, 5, ref, nil, "SUPPORTED", []string{"a@3:4", "u@3:4"}},
		{"definition active match plus unknown", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("a", def, "go"), reg("u", def, "UNKNOWN")}, true)}, 5, def, nil, "SUPPORTED", []string{"a@3:4", "u@3:4"}},
		{"references match then unknown filter", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{regWithMatchAndUnknown("r", ref, false)}, true)}, 5, ref, nil, "SUPPORTED", []string{"r@3:4"}},
		{"references unknown then match filter", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{regWithMatchAndUnknown("r", ref, true)}, true)}, 5, ref, nil, "SUPPORTED", []string{"r@3:4"}},
		{"definition match then unknown filter", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{regWithMatchAndUnknown("d", def, false)}, true)}, 5, def, nil, "SUPPORTED", []string{"d@3:4"}},
		{"definition unknown then match filter", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{regWithMatchAndUnknown("d", def, true)}, true)}, 5, def, nil, "SUPPORTED", []string{"d@3:4"}},
		{"duplicate active ID", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "register", 3, 5, 6, []map[string]any{reg("r", ref, "go")}, true)}, 7, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"unknown unregister ID", []heldCapabilityExchange{init, testExchange(t, "unregister", 2, 3, 4, []map[string]any{unreg("missing", ref)}, true)}, 5, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"failed unknown unregister ID", []heldCapabilityExchange{init, testExchange(t, "unregister", 2, 3, 4, []map[string]any{unreg("missing", ref)}, false)}, 5, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"wrong unregister method", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 6, []map[string]any{unreg("r", def)}, true)}, 7, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"failed wrong unregister method", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 6, []map[string]any{unreg("r", def)}, false)}, 7, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"pending references unregister retains active", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 8, []map[string]any{unreg("r", ref)}, true)}, 6, ref, nil, "SUPPORTED", []string{"r@3:4"}},
		{"acknowledged references unregister removes later", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 8, []map[string]any{unreg("r", ref)}, true)}, 9, ref, nil, "UNSUPPORTED", nil},
		{"pending definition unregister retains active", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("d", def, "go")}, true), testExchange(t, "unregister", 3, 5, 8, []map[string]any{unreg("d", def)}, true)}, 6, def, nil, "SUPPORTED", []string{"d@3:4"}},
		{"acknowledged definition unregister removes later", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("d", def, "go")}, true), testExchange(t, "unregister", 3, 5, 8, []map[string]any{unreg("d", def)}, true)}, 9, def, nil, "UNSUPPORTED", nil},
		{"failed ID retry", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, false), testExchange(t, "register", 3, 5, 6, []map[string]any{reg("r", ref, "go")}, true)}, 7, ref, nil, "SUPPORTED", []string{"r@5:6"}},
		{"unregistered ID reused", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 6, []map[string]any{unreg("r", ref)}, true), testExchange(t, "register", 4, 7, 8, []map[string]any{reg("r", ref, "go")}, true)}, 9, ref, nil, "SUPPORTED", []string{"r@7:8"}},
		{"static dynamic same method", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true)}, 5, ref, map[string]bool{ref: true}, "INVALID_CHRONOLOGY", nil},
		{"mixed methods selected references", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go"), reg("d", def, "go")}, true)}, 5, ref, nil, "SUPPORTED", []string{"r@3:4"}},
		{"mixed methods selected definition", []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go"), reg("d", def, "go")}, true)}, 5, def, nil, "SUPPORTED", []string{"d@3:4"}},
		{"pending initialize gates registration", []heldCapabilityExchange{testExchange(t, "initialize", 1, 0, -1, nil, true), testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true)}, 5, ref, nil, "INVALID_CHRONOLOGY", nil},
		{"registration response precedes initialize response", []heldCapabilityExchange{testExchange(t, "initialize", 1, 0, 8, nil, true), testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true)}, 9, ref, nil, "INVALID_CHRONOLOGY", nil},
	}
	for _, tc := range cases {
		got := stateAt(tc.events, stream(tc.events), tc.write, tc.method, tc.static, "go", "CLAIMANT_UNCHANGED")
		if got.status != tc.status || tc.active != nil && fmt.Sprint(got.active) != fmt.Sprint(tc.active) {
			t.Errorf("%s got %+v want %s %v", tc.name, got, tc.status, tc.active)
		}
	}
	// The caller-supplied summary stays fixed while only held response order changes.
	a := []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 6, []map[string]any{unreg("r", ref)}, true), testExchange(t, "register", 4, 7, 8, []map[string]any{reg("r", ref, "go")}, true)}
	b := []heldCapabilityExchange{init, testExchange(t, "register", 2, 3, 4, []map[string]any{reg("r", ref, "go")}, true), testExchange(t, "unregister", 3, 5, 9, []map[string]any{unreg("r", ref)}, true), testExchange(t, "register", 4, 7, 8, []map[string]any{reg("r", ref, "go")}, true)}
	if x, y := stateAt(a, stream(a), 10, ref, nil, "go", "SAME_CLAIM"), stateAt(b, stream(b), 10, ref, nil, "go", "SAME_CLAIM"); x.status != "SUPPORTED" || y.status != "INVALID_CHRONOLOGY" {
		t.Errorf("held order with unchanged claimant x=%+v y=%+v", x, y)
	}
	// An extra complete response is not erased by a claimant's list of exchanges.
	extra := append(stream(a), candidateFrame{"CLIENT_TO_SERVER", 9, a[3].response.raw})
	sort.Slice(extra, func(i, j int) bool { return extra[i].ordinal < extra[j].ordinal })
	if got := stateAt(a, extra, 10, ref, nil, "go", "SAME_CLAIM"); got.status != "INVALID_CHRONOLOGY" {
		t.Errorf("extra response not rejected %+v", got)
	}
	_ = strings.TrimSpace // used by the strict raw-frame implementation after the RED.
}
