package sessionruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

// This test is a new, narrow semantic RED. The historical private RED test is unchanged.
// A setup/control failure is never counted as the missing-lease assertion.
func b4LeaseAssertionFailure(t *testing.T, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"POSITIVE_ASSERTION_FAIL_NONQUALIFYING","assertion":"ASSERT_ADR0011_B4_PRIVATE_A_SELECTED_READ_LEASE","detail":%q}`, detail)
}
func b4LeaseInvalid(t *testing.T, category, detail string) {
	t.Helper()
	t.Fatalf(`{"kind":"INVALID_RED_ATTEMPT","category":%q,"detail":%q}`, category, detail)
}

func b4LeaseRead(t *testing.T, path string) []byte {
	t.Helper()
	root, err := b4ID1FixtureRoot()
	if err != nil {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "fixture root")
	}
	b, err := b4ID1ReadFile(root, path)
	if err != nil {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "fixture read "+path)
	}
	return b
}

func b4LeaseGet(t *testing.T, f b4ID1Fixture, path string) []byte {
	t.Helper()
	pin, ok := f.pins[path]
	if !ok {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "unlisted asset "+path)
	}
	b := b4LeaseRead(t, path)
	if len(b) != pin.Length || b4ID1Hash(b) != pin.SHA {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "asset read/digest "+path)
	}
	return b
}

func b4LeaseFixture(t *testing.T) b4ID1Fixture {
	t.Helper()
	for _, p := range []struct{ path, sha string }{
		{"SPEC.md", "8fa536964c6542b151b7a5e5bd10170ea63ee46374250087a7923c2cc8d2ebae"},
		{"DESIGN.md", "9eb8dfa992d4db3f4ea9b1a8eef64c18f7fec7c8e3103904d0a3dfd85cbb96dc"},
		{"DERIVATION.md", "6180b736af5ec7e70e366425e8e658ec48ba64e3aae40ccdb19ac6487888d069"},
		{"oracle.json", "d82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59"},
	} {
		if b4ID1Hash(b4LeaseRead(t, p.path)) != p.sha {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "accepted provenance pin "+p.path)
		}
	}
	raw := b4LeaseRead(t, "manifest.json")
	if b4ID1Hash(raw) != b4ID1Manifest {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "manifest SHA-256")
	}
	var manifest struct {
		Assets       []b4ID1Pin `json:"assets"`
		Validation   string     `json:"a4_b4a_b4b_runtime_validation"`
		Status       string     `json:"status"`
		Predecessors []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"predecessors"`
		Custody struct {
			Accepted               bool   `json:"accepted"`
			Authority              int    `json:"authority"`
			Completeness           string `json:"completeness"`
			ProducerAuthentication string `json:"producer_authentication"`
		} `json:"custody"`
		ManagerResponseObserved bool `json:"manager_response_observed"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil || len(manifest.Assets) != 57 || manifest.Validation != "PENDING" || manifest.Status != "PRIVATE_SYNTHETIC_MANAGER_ID1_ORIGINALS_PENDING_INDEPENDENT_REVIEW" || len(manifest.Predecessors) != 8 || manifest.Custody.Accepted || manifest.Custody.Authority != 0 || manifest.Custody.Completeness != "UNKNOWN" || manifest.Custody.ProducerAuthentication != "NO_PRODUCER_AUTHENTICATION" || manifest.ManagerResponseObserved {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "manifest structure or declared status")
	}
	predecessorPins := map[string]string{
		"DERIVATION.md":  "6180b736af5ec7e70e366425e8e658ec48ba64e3aae40ccdb19ac6487888d069",
		"A/source.bytes": "33fe4fbf4871969c874b738e0912e1199bee4781213bf50ed99bb62a19777374",
		"oracle.json":    "d82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59",
		"A/target-a.go":  "2b61fbe6058fe0c78aeb9ba3a52a92efc4ccb9fc3e9280275b44c85f9bb15a3c",
		"A/target-b.go":  "318cb48346e30c507f390c86e5dceaaac490a02f8df0deff7f205db558266176",
		"repository:docs/qualification/originals/adr0011-generic-envelope-v4.schema.json": "df4908187030e3d3110bd3745290eb25d8a3934ffb2a85c5d47d8d22dc28f6f3",
		"repository:docs/qualification/originals/generic-lsp-definition-exact-v5.json":    "da62f5a161532fcb7c79dd93b0dca940a460c6e0c5fab83e8987de21c91b4dd3",
		"repository:docs/qualification/originals/generic-lsp-exact-transport-v4.json":     "f3e25fab98cc4b8b96b76978cf56220ccc81f4ef41455ab0401041ea281d0bc8",
	}
	seenPredecessors := map[string]bool{}
	for _, p := range manifest.Predecessors {
		if seenPredecessors[p.Path] || predecessorPins[p.Path] != p.SHA {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "predecessor identity")
		}
		seenPredecessors[p.Path] = true
		if filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || strings.Contains(p.Path, `\`) || len(p.SHA) != 64 {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "predecessor path/digest")
		}
	}
	f := b4ID1Fixture{pins: make(map[string]b4ID1Pin, 57)}
	for _, pin := range manifest.Assets {
		if _, exists := f.pins[pin.Path]; exists {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "duplicate asset")
		}
		b := b4LeaseRead(t, pin.Path)
		if len(b) != pin.Length || b4ID1Hash(b) != pin.SHA {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "asset digest "+pin.Path)
		}
		f.pins[pin.Path] = pin
	}
	return f
}

func b4LeaseInput(t *testing.T, f b4ID1Fixture, c string) v5.B4bFullCandidateInput {
	t.Helper()
	get := func(path string) []byte { return b4LeaseGet(t, f, c+"/"+path) }
	var declaration struct {
		Session           string `json:"session"`
		Transaction       string `json:"transaction"`
		CompletedOwnerKey string `json:"completed_owner_key"`
		Workspace         string `json:"workspace"`
		URI               string `json:"uri"`
		SourceVersion     string `json:"source_version"`
		PositionEncoding  string `json:"position_encoding"`
		SourceCustody     string `json:"source_custody"`
		Language          string `json:"language"`
		Method            string `json:"method"`
		Generation        uint64 `json:"generation"`
		Line              uint64 `json:"line"`
		Character         uint64 `json:"character"`
	}
	if err := json.Unmarshal(get("DECLARATION.json"), &declaration); err != nil {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "declaration JSON")
	}
	if declaration.Method != "textDocument/definition" || declaration.Generation != 1 || declaration.Workspace != "file:///w" || declaration.URI != "file:///w/definition.go" || declaration.Line != 1 || declaration.Character != 5 || declaration.SourceVersion != "buffer:v1" || declaration.PositionEncoding != "utf-16" || declaration.Language != "go" || declaration.SourceCustody != "OWNER_BUFFER" {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "declaration identity")
	}
	if c == "A" && (declaration.Session != "sk1:fcbe2d08e21ac9e5dd973cf650d6b6e001fad9873942df4cecdb9bccb52246ab" || declaration.Transaction != "b4-manager-a-definition-id1" || declaration.CompletedOwnerKey != "b4-owner-a-id1") {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "A declaration")
	}
	if c == "B" && (declaration.Session != "sk1:4c9c6b713fe794d9bceb0926dec94939517eca5cbec20feffbe1a5f150c1ee30" || declaration.Transaction != "b4-manager-b-definition-id1" || declaration.CompletedOwnerKey != "b4-owner-b-id1") {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "B declaration")
	}
	in := v5.B4bFullCandidateInput{Write: v5.B4aInput{
		Session: declaration.Session, Generation: declaration.Generation, Transaction: declaration.Transaction,
		Workspace: declaration.Workspace, Method: declaration.Method, URI: declaration.URI, Version: declaration.SourceVersion,
		Encoding: declaration.PositionEncoding, Line: declaration.Line, Character: declaration.Character,
		Source: get("source.bytes"), QueryOriginal: get("query.original"), ApplicabilityOriginal: get("query-applicability.original"),
		Custody: declaration.SourceCustody, Language: &declaration.Language,
		RequestFrame: get("request.frame"), RequestParams: get("request.params"), RequestID: []byte("1"),
		CompletedKey: declaration.CompletedOwnerKey, CompletedOrdinal: 5, WriteCompleted: true,
	}, CapabilityEnvelope: get("capability-envelope.json"), CapabilityOriginal: get("capability-artifact.bin"), HeldClientSelector: get("client-selector.json")}
	for _, claim := range []struct{ role, envelope, original, artifact string }{
		{"SOURCE", "source-envelope.json", "source.bytes", "source-artifact.bin"},
		{"QUERY", "query-envelope.json", "query.original", "query.original"},
		{"QUERY_APPLICABILITY", "query-applicability-envelope.json", "query-applicability.original", "query-applicability.original"},
		{"REQUEST_WRITE", "request-write-envelope.json", "request.frame", "request.frame"},
	} {
		in.Write.Claims = append(in.Write.Claims, v5.B4aClaim{Role: claim.role, Envelope: get(claim.envelope), Original: get(claim.original), Artifact: get(claim.artifact)})
	}
	dirs := []string{"CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "CLIENT_TO_SERVER"}
	for i, dir := range dirs {
		path := fmt.Sprintf("cap-%d.frame", i)
		if i == 5 {
			path = "request.frame"
		}
		in.Frames = append(in.Frames, v5.B4Frame{Ordinal: uint64(i), Direction: dir, Bytes: get(path)})
	}
	return in
}

func b4LeaseControls(t *testing.T, f b4ID1Fixture, c string) {
	t.Helper()
	in := b4LeaseInput(t, f, c)
	for _, item := range []struct {
		role     string
		body     []byte
		original []byte
	}{
		{"SOURCE", in.Write.Claims[0].Envelope, in.Write.Claims[0].Original}, {"QUERY", in.Write.Claims[1].Envelope, in.Write.Claims[1].Original},
		{"QUERY_APPLICABILITY", in.Write.Claims[2].Envelope, in.Write.Claims[2].Original}, {"CAPABILITY_EVENTS", in.CapabilityEnvelope, in.CapabilityOriginal},
		{"REQUEST_WRITE", in.Write.Claims[3].Envelope, in.Write.Claims[3].Original},
	} {
		var descriptor struct {
			Role     string `json:"role"`
			Original struct {
				Length int    `json:"length"`
				SHA    string `json:"sha256"`
			} `json:"original"`
			Identity struct {
				Session     string `json:"session"`
				Generation  uint64 `json:"generation"`
				Transaction string `json:"transaction"`
			} `json:"identity"`
		}
		if err := json.Unmarshal(item.body, &descriptor); err != nil || descriptor.Role != item.role || descriptor.Identity.Session != in.Write.Session || descriptor.Identity.Generation != in.Write.Generation || descriptor.Identity.Transaction != in.Write.Transaction || descriptor.Original.Length != len(item.original) || descriptor.Original.SHA != "sha256:"+b4ID1Hash(item.original) {
			b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" envelope/original correspondence "+item.role)
		}
		if err := v5.A4Validate(item.role, item.body); err != nil {
			b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" A4 "+item.role+": "+err.Error())
		}
		t.Logf(`{"kind":"CONTROL_PASS","label":"CONTROL_%s_A4_%s_PASS"}`, c, item.role)
	}
	if err := v5.CheckB4a(in.Write); err != nil {
		b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" B4a: "+err.Error())
	}
	t.Logf(`{"kind":"CONTROL_PASS","label":"CONTROL_%s_B4A_PASS"}`, c)
	if outcome := v5.CheckB4bDefinitionSuccessor(in); outcome != "SUPPORTED" {
		b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" definition B4b: "+outcome)
	}
	t.Logf(`{"kind":"CONTROL_PASS","label":"CONTROL_%s_DEFINITION_B4B_SUPPORTED"}`, c)
}

// The child releases its held response only after reading the exact actual frame.
// This is a fixture ordering control, not producer authentication.
type b4LeaseChild struct {
	*b4ID1Child
	observed chan error
}

func b4LeaseScript(f b4ID1Fixture, t *testing.T, c string) *b4LeaseChild {
	return b4LeaseScriptWithResponse(f, t, c, false, nil, nil)
}

func b4LeaseScriptWithNotification(f b4ID1Fixture, t *testing.T, c string, notification bool) *b4LeaseChild {
	return b4LeaseScriptWithResponse(f, t, c, notification, nil, nil)
}

func b4LeaseScriptWithResponse(f b4ID1Fixture, t *testing.T, c string, notification bool, serverError *lspwire.RPCError, responses []lspwire.Message) *b4LeaseChild {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &b4LeaseChild{b4ID1Child: &b4ID1Child{input: input, stdin: stdin, output: output, stdout: stdout}, observed: make(chan error, 1)}
	want := b4LeaseGet(t, f, c+"/request.frame")
	response := b4LeaseGet(t, f, c+"/response.frame")
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		initialize, err := reader.Read()
		if err != nil {
			child.observed <- fmt.Errorf("fixture initialize read: %w", err)
			return
		}
		if initialize.Method != "initialize" || len(initialize.ID) == 0 {
			child.observed <- fmt.Errorf("fixture initialize not exact")
			return
		}
		if err := lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: append(json.RawMessage(nil), initialize.ID...), Result: json.RawMessage(`{"capabilities":{}}`)}); err != nil {
			child.observed <- fmt.Errorf("fixture initialize response: %w", err)
			return
		}
		initialized, err := reader.Read()
		if err != nil {
			child.observed <- fmt.Errorf("fixture initialized read: %w", err)
			return
		}
		if initialized.Method != "initialized" || len(initialized.ID) != 0 {
			child.observed <- fmt.Errorf("fixture initialized not exact")
			return
		}
		msg, _, actual, retained, err := reader.ReadWithFrameIfWithin(4096)
		if err != nil {
			child.observed <- fmt.Errorf("fixture WRITE read: %w", err)
			return
		}
		if !retained || !bytes.Equal(actual, want) || msg.Method != "textDocument/definition" || !bytes.Equal(msg.ID, []byte("1")) {
			child.observed <- fmt.Errorf("fixture WRITE not exact")
			return
		}
		if notification {
			if err := lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(`{"value":{}}`)}); err != nil {
				child.observed <- err
				return
			}
		}
		for i := range responses {
			if err := lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(responses[i]); err != nil {
				child.observed <- err
				return
			}
		}
		if serverError != nil {
			err = lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: append(json.RawMessage(nil), msg.ID...), Error: serverError})
			child.observed <- err
			return
		}
		_, err = output.Write(response)
		child.observed <- err
	}()
	return child
}

var _ Child = (*b4LeaseChild)(nil)

func b4LeaseManager(t *testing.T, f b4ID1Fixture, c string) (*Manager, StartResult, RoundTripRequest, B4DefinitionOwner, *b4LeaseChild) {
	t.Helper()
	var d struct {
		Transaction       string `json:"transaction"`
		CompletedOwnerKey string `json:"completed_owner_key"`
		Session           string `json:"session"`
		Generation        uint64 `json:"generation"`
		Method            string `json:"method"`
		ProfileSelector   struct {
			TrustDomain          string `json:"trust_domain"`
			Workspace            string `json:"workspace"`
			Profile              string `json:"profile"`
			EnvironmentReference string `json:"environment_reference"`
		} `json:"profile_selector"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, f, c+"/DECLARATION.json"), &d); err != nil || d.Transaction == "" || d.CompletedOwnerKey == "" || d.Method != "textDocument/definition" {
		b4LeaseInvalid(t, "FIXTURE_INVALID", c+" owner declaration")
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: d.ProfileSelector.TrustDomain, Workspace: d.ProfileSelector.Workspace, Profile: d.ProfileSelector.Profile, EnvironmentReference: d.ProfileSelector.EnvironmentReference})
	if err != nil {
		b4LeaseInvalid(t, "INVALID_SETUP", c+" profile: "+err.Error())
	}
	child := b4LeaseScript(f, t, c)
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		b4LeaseInvalid(t, "INVALID_SETUP", c+" manager: "+err.Error())
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		_ = m.Shutdown(context.Background())
	})
	s := m.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if s.SessionID != d.Session || s.Generation != d.Generation || s.Generation != 1 {
		b4LeaseInvalid(t, "INVALID_SETUP", c+" session/generation")
	}
	pending := m.BeginReadiness(context.Background(), s.SessionID, s.Generation, time.Now().Add(time.Second))
	ready, found := m.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != ReadinessReady || ready.Failure != "" {
		b4LeaseInvalid(t, "INVALID_SETUP", fmt.Sprintf("%s readiness=%+v found=%v", c, ready, found))
	}
	m.mu.Lock()
	historyEntries := m.sessions[s.SessionID].readinessHistory.entries
	m.mu.Unlock()
	if historyEntries != 3 {
		b4LeaseInvalid(t, "INVALID_SETUP", fmt.Sprintf("%s readiness history entries=%d want=3", c, historyEntries))
	}
	req := RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: d.Method, Params: json.RawMessage(b4LeaseGet(t, f, c+"/request.params")), Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(b4LeaseGet(t, f, c+"/response.frame"))), CaptureMethodRequestFrameMaxBytes: int64(len(b4LeaseGet(t, f, c+"/request.frame")))}
	return m, s, req, B4DefinitionOwner{Transaction: d.Transaction, CompletedOwnerKey: d.CompletedOwnerKey}, child
}

func b4LeaseExpectation(t *testing.T, f b4ID1Fixture, c string) {
	t.Helper()
	var expectation struct {
		PredictionOnly bool `json:"prediction_only"`
		CandidateCount int  `json:"candidate_count"`
		Targets        []struct {
			Ordinal            int                                                `json:"ordinal"`
			URI                string                                             `json:"uri"`
			SelectionRange     struct{ Start, End struct{ Line, Character int } } `json:"selection_range"`
			TargetSourceSHA256 string                                             `json:"target_source_sha256"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(b4LeaseGet(t, f, c+"/EXPECTATION.json"), &expectation); err != nil || !expectation.PredictionOnly || len(expectation.Targets) != expectation.CandidateCount {
		b4LeaseInvalid(t, "FIXTURE_INVALID", c+" expectation")
	}
	responseBody := b4LeaseGet(t, f, c+"/response.json")
	framed := b4LeaseGet(t, f, c+"/response.frame")
	header, body, ok := bytes.Cut(framed, []byte("\r\n\r\n"))
	if !ok || !bytes.Equal(header, []byte("Content-Length: "+strconv.Itoa(len(body)))) || !bytes.Equal(body, responseBody) {
		b4LeaseInvalid(t, "FIXTURE_INVALID", c+" response Content-Length/body")
	}
	var response struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil || string(response.ID) != "1" || !bytes.Equal(response.Result, b4LeaseGet(t, f, c+"/response.result")) {
		b4LeaseInvalid(t, "FIXTURE_INVALID", c+" response ID/result")
	}
	type loc struct {
		URI   string                                             `json:"uri"`
		Range struct{ Start, End struct{ Line, Character int } } `json:"range"`
	}
	var got []loc
	if c == "A" {
		var one loc
		if err := json.Unmarshal(response.Result, &one); err != nil {
			b4LeaseInvalid(t, "FIXTURE_INVALID", "A result")
		}
		got = []loc{one}
	} else if err := json.Unmarshal(response.Result, &got); err != nil {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "B result")
	}
	if len(got) != len(expectation.Targets) || (c == "A" && len(got) != 1) || (c == "B" && len(got) != 2) {
		b4LeaseInvalid(t, "FIXTURE_INVALID", c+" result cardinality")
	}
	for i, want := range expectation.Targets {
		source := fmt.Sprintf("target-%c.go", 'a'+rune(i))
		if want.Ordinal != i || got[i].URI != want.URI || got[i].Range.Start != want.SelectionRange.Start || got[i].Range.End != want.SelectionRange.End || b4ID1Hash(b4LeaseGet(t, f, c+"/"+source)) != want.TargetSourceSHA256 {
			b4LeaseInvalid(t, "FIXTURE_INVALID", c+" response/prediction/source mismatch")
		}
	}
}

func b4LeaseManagerControl(t *testing.T, f b4ID1Fixture, c string) {
	t.Helper()
	b4LeaseExpectation(t, f, c)
	m, s, req, _, child := b4LeaseManager(t, f, c)
	got := m.RoundTrip(context.Background(), req)
	select {
	case err := <-child.observed:
		if err != nil {
			b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" exact WRITE-before-READ: "+err.Error())
		}
	case <-time.After(time.Second):
		b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" missing exact WRITE-before-READ witness")
	}
	write, writeOK := got.CompletedMethodRequestFrame()
	read, readOK := got.CompletedDefinitionResponseFrame()
	writeObservation, writeObserved := got.CompletedRequestWrite()
	readObservation, readObserved := got.CompletedResponseRead()
	wantWrite := b4LeaseGet(t, f, c+"/request.frame")
	wantRead := b4LeaseGet(t, f, c+"/response.frame")
	framedHash := func(b []byte) string { return "sha256:" + b4ID1Hash(b) }
	if got.Failure != "" || got.ServerError != nil || got.Key != (lspwire.RequestKey{Generation: s.Generation, ID: 1}) || !writeOK || !readOK || !writeObserved || !readObserved || writeObservation.SessionID != s.SessionID || writeObservation.Generation != s.Generation || writeObservation.Key != got.Key || writeObservation.Method != "textDocument/definition" || writeObservation.FrameBytes != int64(len(wantWrite)) || writeObservation.FrameSHA256 != framedHash(wantWrite) || readObservation.SessionID != s.SessionID || readObservation.Generation != s.Generation || readObservation.Key != got.Key || readObservation.FrameBytes != int64(len(wantRead)) || readObservation.FrameSHA256 != framedHash(wantRead) || !bytes.Equal(write, wantWrite) || !bytes.Equal(read, wantRead) || !bytes.Equal(got.Result, b4LeaseGet(t, f, c+"/response.result")) {
		b4LeaseInvalid(t, "CONTROL_NOT_READY", c+" actual manager ID1 WRITE/READ mismatch")
	}
	t.Logf(`{"kind":"CONTROL_PASS","label":"CONTROL_MANAGER_%s_ID1_EXACT_WRITE_READ_PASS"}`, c)
}

func TestADR0011NarrowPrivateSelectedReadLeaseV1(t *testing.T) {
	f := b4LeaseFixture(t)
	for _, c := range []string{"A", "B"} {
		b4LeaseControls(t, f, c)
	}
	for _, c := range []string{"A", "B"} {
		b4LeaseManagerControl(t, f, c)
	}
	m, s, req, owner, child := b4LeaseManager(t, f, "A") // Fresh manager: generic controls never consume its ID1.
	selection := B4DefinitionSelectionKey{SessionID: s.SessionID, Key: lspwire.RequestKey{Generation: 1, ID: 1}, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
	if s.Generation != 1 || req.Method != "textDocument/definition" || owner.Transaction != "b4-manager-a-definition-id1" || owner.CompletedOwnerKey != "b4-owner-a-id1" {
		b4LeaseInvalid(t, "FIXTURE_INVALID", "private declaration")
	}
	result, lease := m.RoundTripPrivateB4(context.Background(), req, owner) // Exactly one private invocation.
	if result.Failure == session.ToolNotImplemented && lease == (B4DefinitionLease{}) {
		t.Fatalf(`{"kind":"SEMANTIC_RED_CANDIDATE","assertion":"ASSERT_ADR0011_B4_PRIVATE_A_SELECTED_READ_LEASE","observation":"ToolNotImplemented and zero lease"}`)
	}
	if result.Failure != "" || result.Key != selection.Key || lease == (B4DefinitionLease{}) {
		b4LeaseAssertionFailure(t, fmt.Sprintf("unexpected private failure=%s key=%+v lease_zero=%v", result.Failure, result.Key, lease == (B4DefinitionLease{})))
	}
	select {
	case err := <-child.observed:
		if err != nil {
			b4LeaseAssertionFailure(t, "exact WRITE-before-READ: "+err.Error())
		}
	case <-time.After(time.Second):
		b4LeaseAssertionFailure(t, "missing exact WRITE-before-READ witness")
	}
	expectedFrame := b4LeaseGet(t, f, "A/response.frame")
	expectedRequest := b4LeaseGet(t, f, "A/request.frame")
	expectedResult := b4LeaseGet(t, f, "A/response.result")
	hash := func(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
	requestFrame, requestOK := privateB4BorrowedRequestFrame(t, result)
	responseFrame, responseOK := result.CompletedDefinitionResponseFrame()
	write, writeOK := result.CompletedRequestWrite()
	read, readOK := result.CompletedResponseRead()
	if result.ServerError != nil || !requestOK || !responseOK || !writeOK || !readOK || write.SessionID != selection.SessionID || write.Generation != 1 || write.Key != selection.Key || write.Method != "textDocument/definition" || write.FrameBytes != int64(len(expectedRequest)) || write.FrameSHA256 != hash(expectedRequest) || read.SessionID != selection.SessionID || read.Generation != 1 || read.Key != selection.Key || read.FrameBytes != int64(len(expectedFrame)) || read.FrameSHA256 != hash(expectedFrame) || !bytes.Equal(requestFrame, expectedRequest) || !bytes.Equal(responseFrame, expectedFrame) || !bytes.Equal(privateB4BorrowedResult(t, result), expectedResult) {
		b4LeaseAssertionFailure(t, "private result lacks matching completed WRITE/selected READ observations")
	}
	capture, status := consumePrivateB4SnapshotForTest(m, lease, selection)
	if status == PrivateB4Unavailable || capture.SessionID != selection.SessionID || capture.Key != selection.Key || capture.Transaction != owner.Transaction || capture.CompletedOwnerKey != owner.CompletedOwnerKey || capture.Method != "textDocument/definition" || !bytes.Equal(capture.RequestFrame, expectedRequest) || !bytes.Equal(capture.RequestParams, b4LeaseGet(t, f, "A/request.params")) || !bytes.Equal(capture.ResponseFrame, expectedFrame) || !bytes.Equal(capture.Result, expectedResult) || capture.RequestFrameSHA256 != hash(expectedRequest) || capture.ResponseFrameSHA256 != hash(expectedFrame) || capture.ResultSHA256 != hash(expectedResult) {
		b4LeaseAssertionFailure(t, "lease capture not bound to A selected READ/WRITE")
	}
}
