package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func restrictedPrivateResult(uri string) []byte {
	return []byte(`[{"uri":"` + uri + `","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`)
}

func restrictedProjectFrame(t *testing.T, frame []byte, oldURI, newURI string) []byte {
	t.Helper()
	_, body, ok := bytes.Cut(frame, []byte("\r\n\r\n"))
	if !ok {
		t.Fatal("BLOCKED_NOT_RED projected LSP frame separator")
	}
	body = bytes.ReplaceAll(body, []byte(oldURI), []byte(newURI))
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func restrictedProjectRequestClaim(t *testing.T, replay *v5.B4bFullCandidateInput, oldURI, newURI string) {
	t.Helper()
	project := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte(oldURI), []byte(newURI)) }
	replay.Write.RequestParams = project(replay.Write.RequestParams)
	replay.Write.RequestFrame = restrictedProjectFrame(t, replay.Write.RequestFrame, oldURI, newURI)
	for i := range replay.Frames {
		if bytes.Contains(replay.Frames[i].Bytes, []byte(oldURI)) {
			replay.Frames[i].Bytes = restrictedProjectFrame(t, replay.Frames[i].Bytes, oldURI, newURI)
		}
	}
	for i := range replay.Write.Claims {
		claim := &replay.Write.Claims[i]
		before := claim.Original
		claim.Original = project(claim.Original)
		claim.Artifact = project(claim.Artifact)
		if claim.Role == "REQUEST_WRITE" {
			claim.Original = append([]byte(nil), replay.Write.RequestFrame...)
			claim.Artifact = append([]byte(nil), replay.Write.RequestFrame...)
		}
		if bytes.Equal(before, claim.Original) {
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal(claim.Envelope, &envelope); err != nil {
			t.Fatal(err)
		}
		binding := func(v any, original []byte) {
			m := v.(map[string]any)
			sum := sha256.Sum256(original)
			m["length"] = float64(len(original))
			m["sha256"] = "sha256:" + hex.EncodeToString(sum[:])
		}
		binding(envelope["original"], claim.Original)
		if claim.Role == "REQUEST_WRITE" {
			payload := envelope["payload"].(map[string]any)
			binding(payload["frame"], replay.Write.RequestFrame)
			binding(payload["params"], replay.Write.RequestParams)
		}
		claim.Envelope, _ = json.Marshal(envelope)
	}
}

func restrictedB4Preimage(fields ...string) []byte {
	var out []byte
	for _, field := range fields {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		out = append(out, size[:]...)
		out = append(out, field...)
	}
	return out
}

func restrictedRebuildCapability(t *testing.T, replay *v5.B4bFullCandidateInput, tx string) {
	t.Helper()
	hash := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	desc := func(raw []byte) map[string]any {
		return map[string]any{"length": len(raw), "sha256": "sha256:" + hash(raw), "private_ref": "projected:frame"}
	}
	message := func(index int) map[string]json.RawMessage {
		_, body, ok := bytes.Cut(replay.Frames[index].Bytes, []byte("\r\n\r\n"))
		if !ok {
			t.Fatal("BLOCKED_NOT_RED projected capability frame separator")
		}
		var out map[string]json.RawMessage
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	idValue := func(raw json.RawMessage) any {
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	idToken := func(raw json.RawMessage) string {
		if len(raw) != 0 && raw[0] == '"' {
			var value string
			_ = json.Unmarshal(raw, &value)
			return "string:" + value
		}
		return "number:" + string(raw)
	}
	makeExchange := func(requestIndex, responseIndex int) map[string]any {
		request, response := message(requestIndex), message(responseIndex)
		var method string
		var params, result any
		_ = json.Unmarshal(request["method"], &method)
		_ = json.Unmarshal(request["params"], &params)
		_ = json.Unmarshal(response["result"], &result)
		return map[string]any{
			"request_direction": replay.Frames[requestIndex].Direction, "request_frame": desc(replay.Frames[requestIndex].Bytes),
			"request_id": idValue(request["id"]), "request_method": method, "request_params": params, "request_frame_ordinal": replay.Frames[requestIndex].Ordinal,
			"response_direction": replay.Frames[responseIndex].Direction, "response_frame": desc(replay.Frames[responseIndex].Bytes),
			"response_id": idValue(response["id"]), "response_frame_ordinal": replay.Frames[responseIndex].Ordinal,
			"response_status": "SUCCESS", "response_result": result, "response_error": nil, "target_write_frame_ordinal": replay.Write.CompletedOrdinal,
		}
	}
	initialize, event := makeExchange(0, 1), makeExchange(3, 4)
	fields := []string{"ADR0011-GENERIC-CAPABILITY-EXCHANGE-ARTIFACT/2", tx, strconv.FormatUint(replay.Write.CompletedOrdinal, 10), "2"}
	for _, pair := range [][2]int{{0, 1}, {3, 4}} {
		request, response := message(pair[0]), message(pair[1])
		var method string
		_ = json.Unmarshal(request["method"], &method)
		fields = append(fields, replay.Frames[pair[0]].Direction, string(replay.Frames[pair[0]].Bytes), idToken(request["id"]), method, strconv.FormatUint(replay.Frames[pair[0]].Ordinal, 10), replay.Frames[pair[1]].Direction, string(replay.Frames[pair[1]].Bytes), idToken(response["id"]), strconv.FormatUint(replay.Frames[pair[1]].Ordinal, 10), "SUCCESS")
	}
	fields = append(fields, replay.Frames[2].Direction, strconv.FormatUint(replay.Frames[2].Ordinal, 10), string(replay.Frames[2].Bytes), "5")
	observed := make([]any, 0, 5)
	for i := 0; i < 5; i++ {
		fields = append(fields, replay.Frames[i].Direction, strconv.FormatUint(replay.Frames[i].Ordinal, 10), string(replay.Frames[i].Bytes))
		observed = append(observed, map[string]any{"direction": replay.Frames[i].Direction, "frame_ordinal": replay.Frames[i].Ordinal, "frame": desc(replay.Frames[i].Bytes)})
	}
	replay.CapabilityOriginal = restrictedB4Preimage(fields...)
	payload := map[string]any{"initialize": initialize, "events": []any{event}, "initialized_notification": map[string]any{"direction": replay.Frames[2].Direction, "frame_ordinal": replay.Frames[2].Ordinal, "frame": desc(replay.Frames[2].Bytes)}, "observed_frames": observed, "target_write_frame_ordinal": replay.Write.CompletedOrdinal}
	envelope := map[string]any{"role": "CAPABILITY_EVENTS", "identity": map[string]any{"session": replay.Write.Session, "generation": replay.Write.Generation, "transaction": replay.Write.Transaction}, "original": desc(replay.CapabilityOriginal), "predecessors": []any{}, "payload": payload}
	replay.CapabilityEnvelope, _ = json.Marshal(envelope)
	if err := v5.A4Validate("CAPABILITY_EVENTS", replay.CapabilityEnvelope); err != nil {
		t.Fatalf("BLOCKED_NOT_RED projected capability envelope: %v", err)
	}
}

func restrictedRebindProjectedClaims(t *testing.T, replay *v5.B4bFullCandidateInput) {
	t.Helper()
	hash := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	descriptor := func(v any, raw []byte) {
		m := v.(map[string]any)
		m["length"] = float64(len(raw))
		m["sha256"] = "sha256:" + hash(raw)
	}
	setPredecessor := func(envelope map[string]any, role, selector, digest string) {
		for _, raw := range envelope["predecessors"].([]any) {
			p := raw.(map[string]any)
			if p["role"] == role {
				p["selector"], p["digest"] = selector, digest
			}
		}
	}
	gen := strconv.FormatUint(replay.Write.Generation, 10)
	tx := "sha256:" + hash(restrictedB4Preimage("ADR0011-GENERIC-TRANSACTION/1", replay.Write.Session, gen, replay.Write.Transaction))
	restrictedRebuildCapability(t, replay, tx)
	capabilitySelector, err := v5.A4Selector("definition", "CAPABILITY_EVENTS", replay.Write.Session, gen, replay.CapabilityOriginal, tx)
	if err != nil {
		t.Fatal(err)
	}
	sourceArtifact := restrictedB4Preimage("ADR0011-GENERIC-SOURCE-ARTIFACT/1", tx, replay.Write.URI, "present", replay.Write.Version, strconv.Itoa(len(replay.Write.Source)), hash(replay.Write.Source), replay.Write.Custody)
	sourceSelector, err := v5.A4Selector("definition", "SOURCE", replay.Write.Session, gen, sourceArtifact, tx)
	if err != nil {
		t.Fatal(err)
	}
	querySelector, err := v5.A4Selector("definition", "QUERY", replay.Write.Session, gen, replay.Write.QueryOriginal, replay.Write.URI, replay.Write.Version, hash(replay.Write.Source), replay.Write.Encoding, strconv.FormatUint(replay.Write.Line, 10), strconv.FormatUint(replay.Write.Character, 10))
	if err != nil {
		t.Fatal(err)
	}
	language := ""
	if replay.Write.Language != nil {
		language = *replay.Write.Language
	}
	appArtifact := restrictedB4Preimage("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1", tx, querySelector, sourceSelector, replay.Write.URI, "present", language, "ORDINARY", "absent", "", "absent", "", "", replay.Write.Workspace, replay.Write.Session, gen, replay.Write.Transaction)
	replay.Write.ApplicabilityOriginal = append([]byte(nil), appArtifact...)
	appSelector, err := v5.A4Selector("definition", "QUERY_APPLICABILITY", replay.Write.Session, gen, appArtifact, tx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range replay.Write.Claims {
		claim := &replay.Write.Claims[i]
		var envelope map[string]any
		if err := json.Unmarshal(claim.Envelope, &envelope); err != nil {
			t.Fatal(err)
		}
		identity := envelope["identity"].(map[string]any)
		identity["session"] = replay.Write.Session
		payload := envelope["payload"].(map[string]any)
		switch claim.Role {
		case "SOURCE":
			claim.Original = append([]byte(nil), replay.Write.Source...)
			claim.Artifact = append([]byte(nil), sourceArtifact...)
			payload["uri"] = replay.Write.URI
			descriptor(payload["source_bytes"], replay.Write.Source)
		case "QUERY":
			claim.Original = append([]byte(nil), replay.Write.QueryOriginal...)
			claim.Artifact = append([]byte(nil), replay.Write.QueryOriginal...)
			payload["uri"] = replay.Write.URI
			payload["source"] = "sha256:" + hash(replay.Write.Source)
			setPredecessor(envelope, "SOURCE", sourceSelector, "sha256:"+hash(replay.Write.Source))
		case "QUERY_APPLICABILITY":
			claim.Original = append([]byte(nil), appArtifact...)
			claim.Artifact = append([]byte(nil), appArtifact...)
			payload["session"], payload["workspace"] = replay.Write.Session, replay.Write.Workspace
			payload["query_selector"], payload["query_source_selector"] = querySelector, sourceSelector
			descriptor(payload["uri_bytes"], []byte(replay.Write.URI))
			setPredecessor(envelope, "QUERY", querySelector, "sha256:"+hash(replay.Write.QueryOriginal))
			setPredecessor(envelope, "SOURCE", sourceSelector, "sha256:"+hash(replay.Write.Source))
		case "REQUEST_WRITE":
			claim.Original = append([]byte(nil), replay.Write.RequestFrame...)
			claim.Artifact = append([]byte(nil), replay.Write.RequestFrame...)
			descriptor(payload["frame"], replay.Write.RequestFrame)
			descriptor(payload["params"], replay.Write.RequestParams)
			setPredecessor(envelope, "QUERY", querySelector, "sha256:"+hash(replay.Write.QueryOriginal))
			setPredecessor(envelope, "QUERY_APPLICABILITY", appSelector, "sha256:"+hash(appArtifact))
			setPredecessor(envelope, "CAPABILITY_EVENTS", capabilitySelector, "sha256:"+hash(replay.CapabilityOriginal))
		}
		descriptor(envelope["original"], claim.Original)
		claim.Envelope, err = json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := v5.CheckB4a(replay.Write); err != nil {
		t.Fatalf("BLOCKED_NOT_RED projected B4a: %v", err)
	}
}

func restrictedProjectedChild(t *testing.T, sources map[string][]byte, requestFrame, responseFrame []byte) *composedChild {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
	go func() {
		r := lspwire.NewReader(input, lspwire.DefaultLimits())
		if err := composedServeReadiness(r, output); err != nil {
			child.observed <- err
			return
		}
		seen := make(map[string]bool, len(sources))
		for range sources {
			msg, _, _, _, err := r.ReadWithFrameIfWithin(4096)
			if err != nil || msg.Method != "textDocument/didOpen" {
				child.observed <- fmt.Errorf("exact didOpen: %v method=%s", err, msg.Method)
				return
			}
			var params struct {
				TextDocument struct {
					URI     string `json:"uri"`
					Version int    `json:"version"`
					Text    string `json:"text"`
				} `json:"textDocument"`
			}
			if err := json.Unmarshal(msg.Params, &params); err != nil || params.TextDocument.Version != 1 || seen[params.TextDocument.URI] || !bytes.Equal([]byte(params.TextDocument.Text), sources[params.TextDocument.URI]) {
				child.observed <- fmt.Errorf("exact didOpen payload: %v uri=%s", err, params.TextDocument.URI)
				return
			}
			seen[params.TextDocument.URI] = true
		}
		msg, _, frame, retained, err := r.ReadWithFrameIfWithin(4096)
		if err != nil || !retained || !bytes.Equal(frame, requestFrame) || msg.Method != "textDocument/definition" || !bytes.Equal(msg.ID, []byte("1")) {
			child.observed <- fmt.Errorf("exact definition WRITE after didOpen: %v", err)
			return
		}
		_, err = output.Write(responseFrame)
		child.observed <- err
	}()
	return child
}

func restrictedTypedB4Input(t *testing.T) restrictedDefinitionB4Input {
	t.Helper()
	fixtureRoot := filepath.Join("testdata", "adr0011-composed-b4-manager-id1-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(fixtureRoot, "manifest.json"), composedManifestSHA)
	replay, occurrence, frozenSources, _, _, req, owner, profile := composedFixture(t, assets, fixtureRoot, "A")
	workspace := t.TempDir()
	workspaceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String()
	const frozenWorkspaceURI = "file:///w"
	project := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte(frozenWorkspaceURI), []byte(workspaceURI)) }
	replay.Write.Workspace = workspaceURI
	replay.Write.URI = string(project([]byte(replay.Write.URI)))
	restrictedProjectRequestClaim(t, &replay, frozenWorkspaceURI, workspaceURI)
	req.Params = json.RawMessage(project(req.Params))
	profile.Workspace = workspace
	if err := os.WriteFile(filepath.Join(workspace, "definition.go"), replay.Write.Source, 0o600); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string][]byte, len(frozenSources))
	for frozenURI, source := range frozenSources {
		uri := string(project([]byte(frozenURI)))
		parsed, err := url.Parse(uri)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.FromSlash(parsed.Path), source, 0o600); err != nil {
			t.Fatal(err)
		}
		sources[uri] = append([]byte(nil), source...)
	}
	responseFrame := restrictedProjectFrame(t, bridgeAssetBytes(t, fixtureRoot, "A/response.frame", assets), frozenWorkspaceURI, workspaceURI)
	req.CaptureMethodRequestFrameMaxBytes = int64(len(replay.Write.RequestFrame))
	req.CaptureDefinitionResponseFrameMaxBytes = int64(len(responseFrame))
	child := restrictedProjectedChild(t, sources, replay.Write.RequestFrame, responseFrame)
	validated, err := runtimeprofile.Validate(profile)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		_ = manager.Shutdown(context.Background())
	})
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.SessionID == "" || started.Generation != req.Generation {
		t.Fatal("BLOCKED_NOT_RED typed B4 manager identity")
	}
	composedRequireReadiness(t, manager, started)
	req.SessionID = started.SessionID
	replay.Write.Session = started.SessionID
	restrictedRebindProjectedClaims(t, &replay)
	_, responseBody, _ := bytes.Cut(responseFrame, []byte("\r\n\r\n"))
	preflight := CheckB4DefinitionBridge(DefinitionBridgeInput{Replay: replay, ResponseFrame: responseBody, QueryOccurrenceID: occurrence, TargetSources: sources})
	if preflight.Status != DefinitionBridgeCandidateItems || preflight.ChronologyTerminal != "SUPPORTED" || len(preflight.Candidates) != len(sources) {
		t.Fatalf("BLOCKED_NOT_RED projected bridge preflight status=%s chronology=%s candidates=%d", preflight.Status, preflight.ChronologyTerminal, len(preflight.Candidates))
	}
	uris := make([]string, 0, len(sources))
	for uri := range sources {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	for _, uri := range uris {
		prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: req.SessionID, Generation: req.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
		if prepared.Failure != "" || prepared.Supply == nil || !bytes.Equal(prepared.Supply.Content, sources[uri]) {
			t.Fatalf("BLOCKED_NOT_RED typed B4 target preparation uri=%s result=%+v", uri, prepared)
		}
		acquired, status := manager.PreparePrivateB4DefinitionSource(sessionruntime.B4DefinitionSourceReference{SessionID: req.SessionID, Generation: req.Generation, URI: uri, DocumentVersion: prepared.Version})
		if status != sessionruntime.PrivateB4Selected {
			t.Fatal("BLOCKED_NOT_RED typed B4 source lease")
		}
		owner.TargetSources = append(owner.TargetSources, acquired)
	}
	result, lease := manager.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) {
		t.Fatalf("BLOCKED_NOT_RED typed B4 selection: %s", result.Failure)
	}
	select {
	case err := <-child.observed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("BLOCKED_NOT_RED typed B4 didOpen/WRITE/READ timeout")
	}
	return restrictedDefinitionB4Input{Manager: manager, Lease: lease,
		Selection: sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey},
		Replay:    replay, QueryOccurrenceID: occurrence, TargetSources: sources}
}

func restrictedValidatedOn(t *testing.T, m *restrictedOwnerManager, p restrictedProcessingReservation) (restrictedProvenanceReservation, restrictedValidatedDefinition) {
	t.Helper()
	in := restrictedTypedB4Input(t)
	r, err := m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: restrictedProvenanceRawBytes, Records: restrictedProvenanceRecordBytes})
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.validateDefinitionProvenanceB4(r, p, in)
	if err != nil {
		t.Fatal(err)
	}
	return r, v
}

func restrictedValidatedFixture(t *testing.T, owner uint64) (*restrictedBudgetRoot, *restrictedOwnerManager, restrictedProcessingReservation, restrictedProvenanceReservation, restrictedValidatedDefinition) {
	t.Helper()
	root, m := newRestrictedRootManager(t, 0)
	p, err := m.reserveProcessing(owner, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	r, v := restrictedValidatedOn(t, m, p)
	return root, m, p, r, v
}

func TestRestrictedDefinitionProvenanceResultOnlyRefusalZeroEffect(t *testing.T) {
	cases := []struct {
		name       string
		provenance []byte
	}{
		{name: "bare-result-array", provenance: restrictedPrivateResult("file:///target.go")},
		{name: "null-result", provenance: []byte("null")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, m := newRestrictedRootManager(t, 0)
			p, err := m.reserveProcessing(1, 2, 3)
			if err != nil {
				t.Fatal(err)
			}
			r, err := m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: uint64(len(tc.provenance)), Records: restrictedProvenanceRecordBytes})
			if err != nil {
				t.Fatal(err)
			}
			a := &m.attempts[p.slot]
			beforeUsed, beforeAllocs := root.globalUsed, m.allocations
			beforeState, beforeEpoch := a.provenanceState, a.provenanceEpoch
			beforeRaw, beforeRecords := append([]byte(nil), a.provenanceRaw...), append([]byte(nil), a.provenanceRecords...)
			beforeRawRoot, beforeRecordsRoot := &root.provenanceRaw[p.slot][0], &root.provenanceRecords[p.slot][0]
			in := restrictedDefinitionInput{Owner: 1, Session: 2, Generation: 3, Attempt: p.Attempt, Version: p.Version, Provenance: tc.provenance}
			if tc.name == "bare-result-array" {
				in.TargetSources[0] = []byte("x")
			}
			v, err := m.validateDefinitionProvenance(r, in)
			if !errors.Is(err, errRestrictedIncomplete) {
				t.Fatalf("ASSERT_U2_RESULT_ONLY_REFUSED err=%v", err)
			}
			if v != (restrictedValidatedDefinition{}) {
				t.Fatal("ASSERT_U2_NO_VALIDATED_HANDLE")
			}
			if a.provenanceState != beforeState || a.provenanceEpoch != beforeEpoch {
				t.Fatal("ASSERT_U2_RESERVATION_STATE_UNCHANGED")
			}
			if a.provenanceCandidateCount != 0 {
				t.Fatal("ASSERT_U2_NO_PUBLISH_CANDIDATES")
			}
			if string(a.provenanceRaw) != string(beforeRaw) || string(a.provenanceRecords) != string(beforeRecords) {
				t.Fatal("ASSERT_U2_NO_BID30_OR_BID31_MUTATION")
			}
			if root.globalUsed != beforeUsed || &root.provenanceRaw[p.slot][0] != beforeRawRoot || &root.provenanceRecords[p.slot][0] != beforeRecordsRoot {
				t.Fatal("ASSERT_U2_EXACT_BACKING_ACCOUNTING_EQUALITY")
			}
			if m.allocations != beforeAllocs {
				t.Fatal("ASSERT_U2_EXACT_ALLOCATION_ACCOUNTING_EQUALITY")
			}
		})
	}
}

func TestRestrictedDefinitionProvenanceLayouts(t *testing.T) {
	if unsafe.Sizeof(DefinitionCandidate{}) != 136 {
		t.Fatalf("ASSERT_P7_CANDIDATE_CONTROL_SIZE got=%d", unsafe.Sizeof(DefinitionCandidate{}))
	}
	if restrictedProvenanceCandidateControlBytes != 8_704 || restrictedProvenanceCharge != 2_118_336 {
		t.Fatalf("ASSERT_P7_PROVENANCE_CHARGE control=%d charge=%d", restrictedProvenanceCandidateControlBytes, restrictedProvenanceCharge)
	}
	if unsafe.Sizeof(restrictedProvenanceReservation{}) != 96 {
		t.Fatalf("ASSERT_P6_RESERVATION_96 got=%d", unsafe.Sizeof(restrictedProvenanceReservation{}))
	}
	if unsafe.Sizeof(restrictedValidatedDefinition{}) != 96 {
		t.Fatalf("ASSERT_P6_VALIDATED_96 got=%d", unsafe.Sizeof(restrictedValidatedDefinition{}))
	}
	if unsafe.Offsetof(restrictedProvenanceReservation{}.recordVersion) != 12 || unsafe.Offsetof(restrictedProvenanceReservation{}.rawCapacity) != 56 || unsafe.Offsetof(restrictedProvenanceReservation{}.borrowerCount) != 88 {
		t.Fatal("ASSERT_P6_RESERVATION_OFFSETS")
	}
	if unsafe.Offsetof(restrictedValidatedDefinition{}.requestOff) != 56 || unsafe.Offsetof(restrictedValidatedDefinition{}.candidateCount) != 80 {
		t.Fatal("ASSERT_P6_VALIDATED_OFFSETS")
	}
	if restrictedRecordReserved+1152 != restrictedProvenanceRecordBytes {
		t.Fatal("ASSERT_P7_BID31_SECTIONS")
	}
}

func TestRestrictedDefinitionProvenanceReserveValidateProduce(t *testing.T) {
	root, m, p, _, v := restrictedValidatedFixture(t, 1)
	before := root.globalUsed
	l, view, err := m.produceDefinition(p, v, restrictedRecipient{ID: 9})
	if err != nil {
		t.Fatal(err)
	}
	output := view.Output()
	if len(output) != 1 {
		t.Fatal("ASSERT_P1_DERIVED_ORDINARY_OUTPUT")
	}
	target, parseErr := url.Parse(output[0].TargetURI)
	materialized, readErr := os.ReadFile(filepath.FromSlash(target.Path))
	if parseErr != nil || target.Scheme != "file" || target.Host != "" || filepath.Base(target.Path) != "target-a.go" || readErr != nil || len(materialized) == 0 || output[0].Ordinal != 0 {
		t.Fatal("ASSERT_P1_DERIVED_ORDINARY_OUTPUT")
	}
	if m.attempts[p.slot].provenanceState != restrictedProvenanceHandedOff {
		t.Fatal("ASSERT_P3_HANDOFF_STATE")
	}
	if root.globalUsed <= before {
		t.Fatal("ASSERT_P4_OUTPUT_CHARGED")
	}
	finishRestricted(t, m, l, view)
	if root.provenanceRaw[p.slot] != nil || root.provenanceRecords[p.slot] != nil {
		t.Fatal("ASSERT_P5_RELEASE_CLEARS_ROOTS")
	}
}

func TestRestrictedDefinitionProvenanceCrossAttemptSameCountRefused(t *testing.T) {
	_, m, p1, _, v1 := restrictedValidatedFixture(t, 1)
	p2, err := m.reserveProcessing(1, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	_, v2 := restrictedValidatedOn(t, m, p2)
	if v1.candidateCount != v2.candidateCount {
		t.Fatal("BLOCKED_NOT_RED same-count fixture")
	}
	if _, _, err = m.produceDefinition(p1, v2, restrictedRecipient{ID: 9}); !errors.Is(err, errRestrictedIdentity) {
		t.Fatalf("ASSERT_P2_CROSS_ATTEMPT err=%v", err)
	}
}

func TestRestrictedDefinitionProvenanceValidatedHandleTamper(t *testing.T) {
	_, m, p, _, v := restrictedValidatedFixture(t, 1)
	cases := []struct {
		name  string
		alter func(*restrictedValidatedDefinition)
	}{
		{"chronology", func(x *restrictedValidatedDefinition) { x.chronologyTerminal++ }}, {"flags", func(x *restrictedValidatedDefinition) { x.flags++ }},
		{"record-version", func(x *restrictedValidatedDefinition) { x.recordVersion++ }}, {"owner", func(x *restrictedValidatedDefinition) { x.Owner++ }},
		{"session", func(x *restrictedValidatedDefinition) { x.Session++ }}, {"generation", func(x *restrictedValidatedDefinition) { x.Generation++ }},
		{"attempt", func(x *restrictedValidatedDefinition) { x.Attempt++ }}, {"version", func(x *restrictedValidatedDefinition) { x.Version++ }},
		{"request-off", func(x *restrictedValidatedDefinition) { x.requestOff++ }}, {"request-len", func(x *restrictedValidatedDefinition) { x.requestLen++ }},
		{"response-off", func(x *restrictedValidatedDefinition) { x.responseOff++ }}, {"response-len", func(x *restrictedValidatedDefinition) { x.responseLen++ }},
		{"result-off", func(x *restrictedValidatedDefinition) { x.resultOff++ }}, {"result-len", func(x *restrictedValidatedDefinition) { x.resultLen++ }},
		{"candidate-count", func(x *restrictedValidatedDefinition) { x.candidateCount++ }}, {"borrower-count", func(x *restrictedValidatedDefinition) { x.borrowerCount++ }},
		{"obligations", func(x *restrictedValidatedDefinition) { x.obligationBits++ }}, {"reserved", func(x *restrictedValidatedDefinition) { x.reserved++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := v
			tc.alter(&x)
			if _, _, err := m.produceDefinition(p, x, restrictedRecipient{ID: 9}); !errors.Is(err, errRestrictedIdentity) {
				t.Fatalf("ASSERT_P6_HANDLE_TAMPER err=%v", err)
			}
		})
	}
}

func TestRestrictedDefinitionProvenanceLifecycleAndFailureZeroEffect(t *testing.T) {
	root, m, p, r, v := restrictedValidatedFixture(t, 1)
	before := root.globalUsed
	if _, err := m.validateDefinitionProvenance(r, restrictedDefinitionInput{}); !errors.Is(err, errRestrictedIdentity) && !errors.Is(err, errRestrictedBound) {
		t.Fatalf("ASSERT_P3_REPEATED_VALIDATE err=%v", err)
	}
	if root.globalUsed != before || m.attempts[p.slot].provenanceState != restrictedProvenanceValidated {
		t.Fatal("ASSERT_P3_ZERO_EFFECT")
	}
	bad := v
	bad.resultLen = ^uint32(0)
	if _, _, err := m.produceDefinition(p, bad, restrictedRecipient{ID: 9}); !errors.Is(err, errRestrictedIdentity) {
		t.Fatalf("ASSERT_P6_BOUNDS err=%v", err)
	}
}

func TestRestrictedDefinitionProvenanceReleaseReuse(t *testing.T) {
	root, m, p, _, v := restrictedValidatedFixture(t, 1)
	oldRaw := m.attempts[p.slot].provenanceRaw
	l, view, err := m.produceDefinition(p, v, restrictedRecipient{ID: 9})
	if err != nil {
		t.Fatal(err)
	}
	finishRestricted(t, m, l, view)
	after := root.globalUsed
	p2, err := m.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := m.reserveDefinitionProvenance(p2, restrictedDefinitionLengths{})
	if err != nil {
		t.Fatal(err)
	}
	if p2.slot != p.slot || &root.provenanceRaw[p2.slot][0] == &oldRaw[0] || root.globalUsed-after != m.attempts[p2.slot].processingCharge {
		t.Fatal("ASSERT_P5_REUSE_EXACT_ACCOUNTING")
	}
	if _, _, err = m.produceDefinition(p, v, restrictedRecipient{ID: 9}); !errors.Is(err, errRestrictedIdentity) {
		t.Fatalf("ASSERT_P5_OLD_HANDLE err=%v", err)
	}
	_ = r2
}

func TestRestrictedDefinitionB4ForgedInitialAcquisitionRefused(t *testing.T) {
	field, ok := reflect.TypeOf(sessionruntime.B4DefinitionOwner{}).FieldByName("TargetSources")
	if !ok || field.Type.Kind() != reflect.Slice || field.Type.Elem() == reflect.TypeOf(sessionruntime.B4DefinitionTargetSource{}) {
		t.Fatal("ASSERT_B4_FORGED_INITIAL_ACQUISITION_REFUSED: caller can manufacture source identity, digest, and bytes")
	}
}

func TestRestrictedDefinitionB4CommitPublicationCallbackIsInfallible(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ASSERT_B4_COMMIT_CALLBACK_SOURCE_LOCATION")
	}
	production, err := os.ReadFile(strings.TrimSuffix(file, "restricted_definition_provenance_private_test.go") + "restricted_definition_provenance_private.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(production)
	start := strings.Index(source, "publish := func(commitBorrow sessionruntime.PrivateB4DefinitionBorrow) {")
	if start < 0 {
		t.Fatal("ASSERT_B4_COMMIT_CALLBACK_PRESENT")
	}
	end := strings.Index(source[start:], "\n\t}\n\t_, status = in.Manager.CommitPrivateB4DefinitionBorrowed")
	if end < 0 {
		t.Fatal("ASSERT_B4_COMMIT_CALLBACK_BOUNDARY")
	}
	callback := source[start : start+end]
	for _, forbidden := range []string{"bool {", "return false", "tryBoth()", ".exact(m)", "make("} {
		if strings.Contains(callback, forbidden) {
			t.Fatalf("ASSERT_B4_COMMIT_CALLBACK_INFALLIBLE forbidden=%q body=%s", forbidden, callback)
		}
	}
}

func TestRestrictedDefinitionB4FailedPreparationLeavesLeaseReusable(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	p, err := m.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	in := restrictedTypedB4Input(t)
	r, err := m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: restrictedProvenanceRawBytes, Records: restrictedProvenanceRecordBytes})
	if err != nil {
		t.Fatal(err)
	}
	goodParams := append([]byte(nil), in.Replay.Write.RequestParams...)
	in.Replay.Write.RequestParams = []byte(`{"forged":true}`)
	if _, err = m.validateDefinitionProvenanceB4(r, p, in); !errors.Is(err, errRestrictedIdentity) {
		t.Fatalf("ASSERT_B4_FAILED_PREPARATION_REACHED err=%v", err)
	}
	in.Replay.Write.RequestParams = goodParams
	in.QueryOccurrenceID = "forged-query-occurrence"
	var expectedURI string
	var expectedSource []byte
	for uri, source := range in.TargetSources {
		expectedURI, expectedSource = uri, append([]byte(nil), source...)
	}
	in.TargetSources = map[string][]byte{expectedURI: []byte("substituted")}
	v, err := m.validateDefinitionProvenanceB4(r, p, in)
	if err != nil {
		t.Fatalf("ASSERT_B4_FAILED_PREPARATION_LEASE_REUSABLE err=%v", err)
	}
	if v.candidateCount != 1 || m.attempts[p.slot].provenanceCandidates[0].QueryOccurrenceID == in.QueryOccurrenceID {
		t.Fatal("ASSERT_B4_MANAGER_DERIVED_QUERY_IDENTITY")
	}
	a := &m.attempts[p.slot]
	off := binary.LittleEndian.Uint32(a.provenanceRecords[restrictedRecordReserved : restrictedRecordReserved+4])
	length := binary.LittleEndian.Uint32(a.provenanceRecords[restrictedRecordReserved+4 : restrictedRecordReserved+8])
	gotSource := a.provenanceRaw[off : off+length]
	gotDigest := sha256.Sum256(a.provenanceRaw[:a.provenanceRawUsed])
	if expectedURI == "" || !bytes.Equal(gotSource, expectedSource) || bytes.Equal(gotSource, in.TargetSources[expectedURI]) || !bytes.Equal(a.provenanceRecords[176:208], gotDigest[:]) {
		t.Fatal("ASSERT_B4_MANAGER_CAPTURED_SOURCE_AND_BID30_HASH")
	}
}

func TestRestrictedDefinitionB4ConcurrentConsumptionOneWinner(t *testing.T) {
	_, m := newRestrictedRootManager(t, 0)
	p, err := m.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	in := restrictedTypedB4Input(t)
	r, err := m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: restrictedProvenanceRawBytes, Records: restrictedProvenanceRecordBytes})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := m.validateDefinitionProvenanceB4(r, p, in)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("ASSERT_B4_CONCURRENT_CONSUMPTION_ONE_WINNER winners=%d", winners)
	}
}

func TestRestrictedDefinitionProvenanceBoundZeroEffect(t *testing.T) {
	root, m := newRestrictedRootManager(t, 0)
	p, err := m.reserveProcessing(1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	beforeUsed, beforeAllocs := root.globalUsed, m.allocations
	_, err = m.reserveDefinitionProvenance(p, restrictedDefinitionLengths{Provenance: restrictedProvenanceRawBytes + 1})
	if !errors.Is(err, errRestrictedBound) || root.globalUsed != beforeUsed || m.allocations != beforeAllocs {
		t.Fatalf("ASSERT_P3_BOUND_ZERO_EFFECT err=%v", err)
	}
}
