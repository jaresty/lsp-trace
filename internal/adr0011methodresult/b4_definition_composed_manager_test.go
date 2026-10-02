package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// This is a private synthetic composed-path guard, not admission, authentication,
// completeness, or qualification. A failed fixture/control is BLOCKED_NOT_RED.
const composedManifestSHA = "e2d1880bd3fc09ca6c2e1f6f186d4d387d745cc10e72522597101d1d661ceb4b"

func bridgeDigest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

type composedChild struct {
	input    *io.PipeReader
	stdin    *io.PipeWriter
	output   *io.PipeWriter
	stdout   *io.PipeReader
	observed chan error
}

func (c *composedChild) Stdin() io.WriteCloser { return c.stdin }
func (c *composedChild) Stdout() io.ReadCloser { return c.stdout }
func (c *composedChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *composedChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type composedStarter struct{ child sessionruntime.Child }

func (s composedStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func composedFixture(t *testing.T, assets map[string]bridgeAsset, root, caseName string) (v5.B4bFullCandidateInput, string, map[string][]byte, []bridgeOracleTarget, func() *composedChild, sessionruntime.RoundTripRequest, sessionruntime.B4DefinitionOwner, runtimeprofile.Selector) {
	t.Helper()
	get := func(p string) []byte { return bridgeAssetBytes(t, root, caseName+"/"+p, assets) }
	var d struct {
		Session, Transaction        string
		CompletedOwnerKey           string `json:"completed_owner_key"`
		Workspace, URI, Method      string
		SourceVersion               string `json:"source_version"`
		SourceCustody               string `json:"source_custody"`
		PositionEncoding            string `json:"position_encoding"`
		Language                    string
		Generation, Line, Character uint64
		QueryOccurrenceID           string `json:"query_occurrence_id"`
		ProfileSelector             struct {
			TrustDomain          string `json:"trust_domain"`
			Workspace, Profile   string
			EnvironmentReference string `json:"environment_reference"`
		} `json:"profile_selector"`
	}
	bridgeJSON(t, get("DECLARATION.json"), &d)
	if d.Method != "textDocument/definition" || d.Generation != 1 || d.Workspace != "file:///w" || d.Language != "go" || d.PositionEncoding != "utf-16" || d.SourceCustody != "OWNER_BUFFER" || d.QueryOccurrenceID == "" || d.Transaction == "" || d.CompletedOwnerKey == "" {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s declaration", caseName)
	}
	in := v5.B4bFullCandidateInput{Write: v5.B4aInput{
		Session: d.Session, Generation: d.Generation, Transaction: d.Transaction, Workspace: d.Workspace, Method: d.Method, URI: d.URI,
		Version: d.SourceVersion, Encoding: d.PositionEncoding, Line: d.Line, Character: d.Character, Source: get("source.bytes"),
		QueryOriginal: get("query.original"), ApplicabilityOriginal: get("query-applicability.original"), Custody: d.SourceCustody, Language: &d.Language,
		RequestFrame: get("request.frame"), RequestParams: get("request.params"), RequestID: []byte("1"),
		CompletedKey: d.CompletedOwnerKey, CompletedOrdinal: 5, WriteCompleted: true,
	}, CapabilityEnvelope: get("capability-envelope.json"), CapabilityOriginal: get("capability-artifact.bin"), HeldClientSelector: get("client-selector.json")}
	for _, p := range []struct{ role, envelope, original, artifact string }{
		{"SOURCE", "source-envelope.json", "source.bytes", "source-artifact.bin"},
		{"QUERY", "query-envelope.json", "query.original", "query.original"},
		{"QUERY_APPLICABILITY", "query-applicability-envelope.json", "query-applicability.original", "query-applicability.original"},
		{"REQUEST_WRITE", "request-write-envelope.json", "request.frame", "request.frame"},
	} {
		envelope, original := get(p.envelope), get(p.original)
		if err := v5.A4Validate(p.role, envelope); err != nil {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s A4 %s: %v", caseName, p.role, err)
		}
		var record struct {
			Role     string `json:"role"`
			Original struct {
				Length int    `json:"length"`
				SHA    string `json:"sha256"`
			} `json:"original"`
		}
		bridgeJSON(t, envelope, &record)
		if record.Role != p.role || record.Original.Length != len(original) || record.Original.SHA != "sha256:"+bridgeDigest(original) {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s %s original binding", caseName, p.role)
		}
		in.Write.Claims = append(in.Write.Claims, v5.B4aClaim{Role: p.role, Envelope: envelope, Original: original, Artifact: get(p.artifact)})
	}
	if err := v5.A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s CAP A4: %v", caseName, err)
	}
	if err := v5.CheckB4a(in.Write); err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s B4a: %v", caseName, err)
	}
	dirs := []string{"CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "SERVER_TO_CLIENT", "CLIENT_TO_SERVER", "CLIENT_TO_SERVER"}
	for i, dir := range dirs {
		p := fmt.Sprintf("cap-%d.frame", i)
		if i == 5 {
			p = "request.frame"
		}
		in.Frames = append(in.Frames, v5.B4Frame{Ordinal: uint64(i), Direction: dir, Bytes: get(p)})
	}
	if terminal := v5.CheckB4bDefinitionSuccessor(in); terminal != "SUPPORTED" {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s definition B4b: %s", caseName, terminal)
	}
	var prediction struct {
		PredictionOnly bool                 `json:"prediction_only"`
		CandidateCount int                  `json:"candidate_count"`
		Targets        []bridgeOracleTarget `json:"targets"`
		Custody        struct {
			Authority              int
			Accepted               bool
			Completeness           string
			ProducerAuthentication string `json:"producer_authentication"`
		} `json:"custody"`
	}
	bridgeJSON(t, get("EXPECTATION.json"), &prediction)
	if !prediction.PredictionOnly || len(prediction.Targets) != prediction.CandidateCount || prediction.CandidateCount != map[string]int{"A": 1, "B": 2}[caseName] || prediction.Custody.Authority != 0 || prediction.Custody.Accepted || prediction.Custody.Completeness != "UNKNOWN" || prediction.Custody.ProducerAuthentication != "NO_PRODUCER_AUTHENTICATION" {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s prediction", caseName)
	}
	source := map[string][]byte{}
	for i, w := range prediction.Targets {
		if w.Ordinal != i || w.Kind != "LOCATION" || w.TargetRange != nil {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s oracle ordinal/kind/range", caseName)
		}
		b := get(fmt.Sprintf("target-%c.go", 'a'+i))
		if bridgeDigest(b) != w.TargetSourceSHA {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s target source", caseName)
		}
		source[w.URI] = b
	}
	// The independently held response is a fixture control only; it is never passed
	// into the composed adapter. Preserve exact order, multiplicity and ranges.
	expectedResponse := get("response.json")
	expectedFrame := get("response.frame")
	if header, body, ok := bytes.Cut(expectedFrame, []byte("\r\n\r\n")); !ok || !bytes.Equal(header, []byte(fmt.Sprintf("Content-Length: %d", len(body)))) || !bytes.Equal(body, expectedResponse) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame/body", caseName)
	}
	var response struct {
		ID     json.RawMessage
		Result json.RawMessage
	}
	bridgeJSON(t, expectedResponse, &response)
	if string(response.ID) != "1" || !bytes.Equal(response.Result, get("response.result")) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s response result", caseName)
	}
	synthetic := CheckB4DefinitionBridge(DefinitionBridgeInput{Replay: in, ResponseFrame: expectedResponse, QueryOccurrenceID: d.QueryOccurrenceID, TargetSources: source})
	if synthetic.Status != DefinitionBridgeCandidateItems || len(synthetic.Candidates) != prediction.CandidateCount {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s existing synthetic bridge control: %s/%d", caseName, synthetic.Status, len(synthetic.Candidates))
	}
	for i, w := range prediction.Targets {
		if !composedTargetEqual(synthetic.Candidates[i], w) {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s synthetic oracle target %d", caseName, i)
		}
	}
	buildChild := func() *composedChild {
		input, stdin := io.Pipe()
		stdout, output := io.Pipe()
		child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
		go func() {
			r := lspwire.NewReader(input, lspwire.DefaultLimits())
			msg, _, frame, retained, err := r.ReadWithFrameIfWithin(4096)
			if err != nil || !retained || !bytes.Equal(frame, in.Write.RequestFrame) || msg.Method != d.Method || !bytes.Equal(msg.ID, []byte("1")) {
				child.observed <- fmt.Errorf("exact WRITE: %v", err)
				return
			}
			_, err = output.Write(expectedFrame)
			child.observed <- err
		}()
		return child
	}
	req := sessionruntime.RoundTripRequest{SessionID: d.Session, Generation: d.Generation, Method: d.Method, Params: json.RawMessage(get("request.params")), Deadline: time.Now().Add(3 * time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureDefinitionResponseFrameMaxBytes: int64(len(expectedFrame)), CaptureMethodRequestFrameMaxBytes: int64(len(in.Write.RequestFrame))}
	selector := runtimeprofile.Selector{TrustDomain: d.ProfileSelector.TrustDomain, Workspace: d.ProfileSelector.Workspace, Profile: d.ProfileSelector.Profile, EnvironmentReference: d.ProfileSelector.EnvironmentReference}
	return in, d.QueryOccurrenceID, source, prediction.Targets, buildChild, req, sessionruntime.B4DefinitionOwner{Transaction: d.Transaction, CompletedOwnerKey: d.CompletedOwnerKey}, selector
}
func composedTargetEqual(c DefinitionCandidate, w bridgeOracleTarget) bool {
	if c.Ordinal != w.Ordinal || c.TargetURI != w.URI || string(c.TargetKind) != w.Kind || c.TargetSelectionRange != (Range{Start: w.SelectionRange.Start, End: w.SelectionRange.End}) {
		return false
	}
	if w.TargetRange == nil {
		return c.TargetRange == nil
	}
	return c.TargetRange != nil && *c.TargetRange == (Range{Start: w.TargetRange.Start, End: w.TargetRange.End})
}

func TestADR0011PrivateComposedManagerDefinition(t *testing.T) {
	root := filepath.Join("..", "..", ".pi", "evidence", "adr0011-composed-b4-manager-id1-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), composedManifestSHA)
	for _, c := range []string{"A", "B"} {
		t.Run(c, func(t *testing.T) {
			in, occurrence, sources, wants, buildChild, req, owner, profile := composedFixture(t, assets, root, c)
			validated, err := runtimeprofile.Validate(profile)
			if err != nil {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s profile: %v", c, err)
			}
			child := buildChild()
			manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
			if err != nil {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s manager: %v", c, err)
			}
			t.Cleanup(func() {
				_ = child.Teardown(context.Background())
				_ = child.Close()
				_ = manager.Shutdown(context.Background())
			})
			started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
			if started.SessionID != req.SessionID || started.Generation != req.Generation || manager.ObserveInitialization(started.SessionID, started.Generation, true).State != session.Ready {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s manager readiness/identity", c)
			}
			result, lease := manager.RoundTripPrivateB4(context.Background(), req, owner)
			if result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) || result.Key != (lspwire.RequestKey{Generation: 1, ID: 1}) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s private manager selection failure=%s", c, result.Failure)
			}
			select {
			case err := <-child.observed:
				if err != nil {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s WRITE/READ: %v", c, err)
				}
			case <-time.After(3 * time.Second):
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s WRITE witness timeout", c)
			}
			requestFrame, writeOK := result.CompletedMethodRequestFrame()
			responseFrame, readOK := result.CompletedDefinitionResponseFrame()
			if !writeOK || !readOK || !bytes.Equal(requestFrame, in.Write.RequestFrame) || !bytes.Equal(responseFrame, bridgeAssetBytes(t, root, c+"/response.frame", assets)) || !bytes.Equal(result.Result, bridgeAssetBytes(t, root, c+"/response.result", assets)) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s exact manager WRITE/READ/result", c)
			}
			selection := sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
			decision := CheckPrivateComposedB4Definition(manager, lease, selection, in, occurrence, sources)
			if decision.Status != DefinitionBridgeCandidateItems || decision.ChronologyTerminal != "SUPPORTED" || len(decision.Candidates) != len(wants) {
				t.Fatalf("SEMANTIC_RED_CANDIDATE composed adapter %s: status=%s chronology=%s candidates=%d want=%d", c, decision.Status, decision.ChronologyTerminal, len(decision.Candidates), len(wants))
			}
			for i, w := range wants {
				if !composedTargetEqual(decision.Candidates[i], w) || decision.Candidates[i].QueryOccurrenceID != occurrence {
					t.Errorf("semantic assertion %s candidate[%d] order/multiplicity/range/occurrence: %+v want %+v", c, i, decision.Candidates[i], w)
				}
			}
			if decision.Authority != 0 || decision.Accepted || decision.Completeness != "UNKNOWN" || decision.ClaimCeiling != "NO_PRODUCER_AUTHENTICATION" {
				t.Errorf("semantic assertion %s trust ceiling: %+v", c, decision)
			}
		})
	}
}
