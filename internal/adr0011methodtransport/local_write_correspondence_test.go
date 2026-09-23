package adr0011methodtransport

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type correspondenceChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader
}

func newCorrespondenceChild() *correspondenceChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &correspondenceChild{input: input, stdin: stdin, output: output, stdout: stdout}
	go func() {
		request, err := lspwire.NewReader(input, lspwire.DefaultLimits()).Read()
		if err == nil {
			_ = lspwire.NewWriter(output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: request.ID, Result: json.RawMessage(`[]`)})
		}
	}()
	return child
}
func (c *correspondenceChild) Stdin() io.WriteCloser { return c.stdin }
func (c *correspondenceChild) Stdout() io.ReadCloser { return c.stdout }
func (c *correspondenceChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	_ = c.input.Close()
	_ = c.stdin.Close()
	_ = c.output.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *correspondenceChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

type correspondenceStarter struct{ child sessionruntime.Child }

func (s correspondenceStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func actualCompletedWrite(t *testing.T, method string) (Request, sessionruntime.RoundTripResult) {
	t.Helper()
	child := newCorrespondenceChild()
	t.Cleanup(func() { child.Close() })
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: correspondenceStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.Failure != "" {
		t.Fatalf("ASSERT_LOCAL_WRITE_START_CONTROL: %+v", started)
	}
	if ready := manager.ObserveInitialization(started.SessionID, started.Generation, true); ready.State != session.Ready {
		t.Fatalf("ASSERT_LOCAL_WRITE_READY_CONTROL: %+v", ready)
	}
	req := request(method)
	req.SessionID, req.Generation = started.SessionID, started.Generation
	result := manager.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{
		SessionID: req.SessionID, Generation: req.Generation, Method: req.Method, Params: req.Params,
		Deadline: req.Deadline, MaxMessages: req.MaxMessages, MaxBytes: req.MaxBytes,
	})
	if result.Failure != "" || string(result.Result) != `[]` {
		t.Fatalf("ASSERT_LOCAL_WRITE_RESULT_CONTROL: %+v", result)
	}
	if _, ok := result.CompletedRequestWrite(); !ok {
		t.Fatal("ASSERT_LOCAL_WRITE_OWNER_CONTROL: manager result did not observe write completion")
	}
	return req, result
}

func TestLocalWriteCorrespondenceWithManagerProducedResult(t *testing.T) {
	for _, method := range []string{MethodDefinition, MethodReferences} {
		t.Run(method, func(t *testing.T) {
			req, result := actualCompletedWrite(t, method)
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, result: result}
			got := New(f).Execute(context.Background(), req)
			if got.status != transportSuccess || string(got.Raw()) != `[]` || got.observation == nil || got.observation.LocalWriteCorrespondence != LocalWriteMatch {
				t.Fatalf("ASSERT_LOCAL_WRITE_EXACT_MATCH: outcome=%s raw=%s observation=%+v", got.status, got.Raw(), got.observation)
			}
		})
	}
}

func TestLocalWriteCorrespondenceRejectsSubstitutionWithoutChangingOutcome(t *testing.T) {
	req, original := actualCompletedWrite(t, MethodDefinition)
	cases := []struct {
		name   string
		change func(*Request, *sessionruntime.RoundTripResult)
		want   LocalWriteCorrespondence
	}{
		{"uri", func(r *Request, _ *sessionruntime.RoundTripResult) {
			r.Params = json.RawMessage(strings.Replace(string(r.Params), "a.go", "b.go", 1))
		}, ""},
		// Raw params change, but json.Marshal compacts RawMessage in the frame.
		{"whitespace", func(r *Request, _ *sessionruntime.RoundTripResult) {
			r.Params = append(json.RawMessage{' '}, r.Params...)
		}, LocalWriteMatch},
		{"session", func(r *Request, _ *sessionruntime.RoundTripResult) { r.SessionID += "-replayed" }, ""},
		{"generation", func(r *Request, _ *sessionruntime.RoundTripResult) { r.Generation++ }, ""},
		{"method", func(r *Request, _ *sessionruntime.RoundTripResult) {
			r.Method = MethodReferences
			r.Params = json.RawMessage(strings.TrimSuffix(string(r.Params), "}") + `,"context":{"includeDeclaration":true}}`)
		}, ""},
		{"key", func(_ *Request, result *sessionruntime.RoundTripResult) { result.Key.ID++ }, ""},
		{"request bytes", func(_ *Request, result *sessionruntime.RoundTripResult) { result.RequestBytes++ }, ""},
		{"request messages", func(_ *Request, result *sessionruntime.RoundTripResult) { result.RequestMessages++ }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed, replayed := req, original
			tc.change(&changed, &replayed)
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true}, result: replayed}
			got := New(f).Execute(context.Background(), changed)
			want := tc.want
			if want == "" {
				want = LocalWriteConflict
			}
			if got.status != transportSuccess || string(got.Raw()) != `[]` || got.observation == nil || got.observation.LocalWriteCorrespondence != want {
				status := LocalWriteNotObserved
				if got.observation != nil {
					status = got.observation.LocalWriteCorrespondence
				}
				t.Fatalf("ASSERT_LOCAL_WRITE_REPLAY_CLASSIFICATION: case=%s want=%s outcome=%s status=%s", tc.name, want, got.status, status)
			}
			if tc.name == "whitespace" && got.observation.ParamsSHA256 == fmt.Sprintf("sha256:%x", sha256.Sum256(req.Params)) {
				t.Fatal("ASSERT_LOCAL_WRITE_RAW_PARAMS_DISTINCT_FROM_IDENTICAL_FRAME: raw digest unchanged")
			}
		})
	}
}

func TestLocalWriteCorrespondenceAbsentFromFakeResult(t *testing.T) {
	f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}, result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`)}}
	got := New(f).Execute(context.Background(), request(MethodDefinition))
	if got.status != transportSuccess || got.observation == nil || got.observation.LocalWriteCorrespondence != LocalWriteNotObserved {
		t.Fatalf("ASSERT_LOCAL_WRITE_FAKE_NOT_OBSERVED: outcome=%s observation=%+v", got.status, got.observation)
	}
}
