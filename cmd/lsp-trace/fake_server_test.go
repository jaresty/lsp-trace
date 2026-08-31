package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graph"
)

type fakeMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type fakeItem struct {
	Name           string          `json:"name"`
	Kind           int             `json:"kind"`
	URI            string          `json:"uri"`
	Range          fakeRange       `json:"range"`
	SelectionRange fakeRange       `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}
type fakePosition struct{ Line, Character uint32 }
type fakeRange struct{ Start, End fakePosition }

func TestFakeLanguageServerProcess(t *testing.T) {
	if os.Getenv("LSP_TRACE_FAKE_SERVER") == "" {
		return
	}
	if err := serveFake(os.Getenv("LSP_TRACE_FAKE_SCENARIO"), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(0)
}

func TestSubprocessLifecycleAndHierarchyShapes(t *testing.T) {
	for _, scenario := range []string{"linear", "branch", "diamond", "cycle"} {
		t.Run(scenario, func(t *testing.T) {
			result, code := executeFake(t, scenario, 500*time.Millisecond)
			if code != 0 || !result.Summary.Complete {
				t.Fatalf("ASSERT_SUBPROCESS_%s_COMPLETE: code=%d summary=%#v diagnostics=%#v", strings.ToUpper(scenario), code, result.Summary, result.Diagnostics)
			}
			want := map[string][2]int{"linear": {2, 1}, "branch": {3, 2}, "diamond": {4, 4}, "cycle": {2, 2}}[scenario]
			if result.Summary.NodeCount != want[0] || result.Summary.EdgeCount != want[1] {
				t.Fatalf("ASSERT_SUBPROCESS_%s_SHAPE: nodes=%d edges=%d want=%d/%d", strings.ToUpper(scenario), result.Summary.NodeCount, result.Summary.EdgeCount, want[0], want[1])
			}
		})
	}
}

func TestSubprocessLabeledMultipleSeeds(t *testing.T) {
	for _, scenario := range []string{"multi-two", "multi-duplicate", "multi-mixed"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
				t.Fatal(err)
			}
			seedFile := filepath.Join(t.TempDir(), "seeds.json")
			body := `{"seeds":[{"label":"interface","at":"main.go:1:1"},{"label":"implementation","at":"main.go:2:1"}]}`
			if err := os.WriteFile(seedFile, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"incoming", "--workspace", workspace, "--server", os.Args[0], "--server-arg", "-test.run=^TestFakeLanguageServerProcess$", "--server-env", "LSP_TRACE_FAKE_SERVER=1", "--server-env", "LSP_TRACE_FAKE_SCENARIO=" + scenario, "--seed-file", seedFile, "--request-timeout", "500ms", "--timeout", "1s"}
			stdout, stderr, code := captureRun(t, args)
			if code != 0 && scenario != "multi-mixed" {
				t.Fatalf("ASSERT_REPEATABLE_AT_ACCEPTED: code=%d stderr=%s stdout=%s", code, stderr, stdout)
			}
			var got struct {
				Seeds []struct {
					Label     string       `json:"label"`
					Requested graph.Target `json:"requested_position"`
					Prepared  []string     `json:"prepared_target_ids"`
					Reached   []string     `json:"reached_node_ids"`
					Failure   any          `json:"failure"`
				} `json:"seeds"`
				Nodes   []graph.Node `json:"nodes"`
				Summary struct {
					TraversalComplete bool `json:"traversal_complete"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %v stdout=%q stderr=%q", err, stdout, stderr)
			}
			byLabel := map[string]struct {
				Requested graph.Target
				Prepared  []string
				Reached   []string
				Failure   any
			}{}
			for _, seed := range got.Seeds {
				byLabel[seed.Label] = struct {
					Requested graph.Target
					Prepared  []string
					Reached   []string
					Failure   any
				}{seed.Requested, seed.Prepared, seed.Reached, seed.Failure}
			}
			_, hasInterface := byLabel["interface"]
			_, hasImplementation := byLabel["implementation"]
			if len(got.Seeds) != 2 || !hasInterface || !hasImplementation || byLabel["interface"].Requested.Line != 1 || len(byLabel["implementation"].Prepared) == 0 {
				t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %#v", got.Seeds)
			}
			switch scenario {
			case "multi-duplicate":
				left, right := byLabel["interface"].Reached, byLabel["implementation"].Reached
				if len(got.Nodes) != 1 || len(left) != 1 || len(right) != 1 || left[0] != right[0] {
					t.Fatalf("ASSERT_DUPLICATE_SEEDS_COLLAPSE: nodes=%#v seeds=%#v", got.Nodes, got.Seeds)
				}
			case "multi-mixed":
				if code != 2 || got.Summary.TraversalComplete || len(got.Nodes) == 0 || byLabel["interface"].Failure == nil || len(byLabel["implementation"].Reached) == 0 {
					t.Fatalf("ASSERT_FAILED_SEED_INCOMPLETE_WITH_GRAPH: code=%d result=%#v stderr=%s", code, got, stderr)
				}
			default:
				if !got.Summary.TraversalComplete || len(got.Nodes) != 2 {
					t.Fatalf("ASSERT_FAILED_SEED_CONTINUES: %#v", got)
				}
			}
		})
	}
}

func TestSubprocessOpaqueDataAndShuffledOrdering(t *testing.T) {
	a, codeA := executeFake(t, "shuffle-forward", 500*time.Millisecond)
	b, codeB := executeFake(t, "shuffle-reverse", 500*time.Millisecond)
	if codeA != 0 || codeB != 0 {
		t.Fatalf("ASSERT_SUBPROCESS_SHUFFLE_SUCCESS: codes=%d/%d", codeA, codeB)
	}
	a.Invocation, b.Invocation = graph.Invocation{}, graph.Invocation{}
	for i := range a.Seeds {
		a.Seeds[i].Requested = graph.Target{}
	}
	for i := range b.Seeds {
		b.Seeds[i].Requested = graph.Target{}
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("ASSERT_SUBPROCESS_SHUFFLED_CANONICAL: forward=%s reverse=%s", ja, jb)
	}
	if !strings.Contains(string(ja), `"opaque":{"token":[1,"two"]}`) {
		t.Fatalf("ASSERT_SUBPROCESS_OPAQUE_DATA: %s", ja)
	}
}

func TestSubprocessFailureTraffic(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		code    int
		reason  graph.Reason
		phase   string
	}{
		{"null", 500 * time.Millisecond, 0, graph.PrepareReturnedNoItem, ""},
		{"error", 500 * time.Millisecond, 2, graph.ServerError, "traverse"},
		{"delay", 20 * time.Millisecond, 2, graph.RequestTimeout, ""},
		{"malformed", 500 * time.Millisecond, 1, "", "initialize"},
		{"exit-early", 500 * time.Millisecond, 1, "", "initialize"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, code := executeFake(t, tt.name, tt.timeout)
			if code != tt.code {
				t.Fatalf("ASSERT_SUBPROCESS_%s_EXIT: got=%d want=%d result=%#v", strings.ToUpper(tt.name), code, tt.code, r)
			}
			if tt.reason != "" && (len(r.Terminals) == 0 || r.Terminals[0].Reason != tt.reason) {
				t.Fatalf("ASSERT_SUBPROCESS_%s_REASON: %#v", strings.ToUpper(tt.name), r.Terminals)
			}
			if tt.phase != "" && (len(r.Diagnostics) == 0 || r.Diagnostics[0].Phase != tt.phase) {
				t.Fatalf("ASSERT_SUBPROCESS_%s_PHASE: %#v", strings.ToUpper(tt.name), r.Diagnostics)
			}
		})
	}
}

func executeFake(t *testing.T, scenario string, requestTimeout time.Duration) (graph.Result, int) {
	t.Helper()
	workspace := t.TempDir()
	target := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config{workspace: workspace, command: os.Args[0], at: "main.go:1:1", requestTimeout: requestTimeout, timeout: time.Second, maxDepth: 100, maxNodes: 100}
	cfg.args = []string{"-test.run=^TestFakeLanguageServerProcess$"}
	cfg.env = []string{"LSP_TRACE_FAKE_SERVER=1", "LSP_TRACE_FAKE_SCENARIO=" + scenario}
	return execute(context.Background(), cfg)
}

func serveFake(scenario string, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		m, err := readFake(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch m.Method {
		case "initialize":
			if scenario == "exit-early" {
				return nil
			}
			if scenario == "malformed" {
				_, err = fmt.Fprint(out, "Content-Length: 5\r\n\r\n{")
				return err
			}
			err = writeFake(out, m.ID, map[string]any{"capabilities": map[string]any{"callHierarchyProvider": true}}, nil)
		case "initialized", "textDocument/didOpen":
		case "textDocument/prepareCallHierarchy":
			if scenario == "null" {
				err = writeFake(out, m.ID, nil, nil)
				break
			}
			var p struct {
				Position fakePosition `json:"position"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if scenario == "multi-mixed" && p.Position.Line == 0 {
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32002, "message": "seed prepare failure"})
				break
			}
			line := p.Position.Line
			name := "leaf"
			if scenario == "multi-two" && line > 0 {
				name = "second"
			}
			if scenario == "multi-duplicate" {
				line = 0
			}
			err = writeFake(out, m.ID, []fakeItem{item(name, line)}, nil)
		case "callHierarchy/incomingCalls":
			var p struct {
				Item fakeItem `json:"item"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if scenario == "delay" {
				time.Sleep(80 * time.Millisecond)
			}
			if scenario == "error" {
				err = writeFake(out, m.ID, nil, map[string]any{"code": -32001, "message": "fake failure"})
				break
			}
			err = writeFake(out, m.ID, incoming(scenario, p.Item), nil)
		case "shutdown":
			err = writeFake(out, m.ID, nil, nil)
		case "exit":
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func item(name string, line uint32) fakeItem {
	r := fakeRange{Start: fakePosition{Line: line}, End: fakePosition{Line: line, Character: 20}}
	selection := fakeRange{Start: fakePosition{Line: line}, End: fakePosition{Line: line, Character: 4}}
	return fakeItem{Name: name, Kind: 12, URI: "file:///workspace/main.go", Range: r, SelectionRange: selection, Data: json.RawMessage(`{"opaque":{"token":[1,"two"]},"name":"` + name + `"}`)}
}
func incoming(scenario string, it fakeItem) []map[string]any {
	call := func(from fakeItem) map[string]any {
		return map[string]any{"from": from, "fromRanges": []fakeRange{{Start: fakePosition{Line: from.Range.Start.Line, Character: 5}, End: fakePosition{Line: from.Range.Start.Line, Character: 9}}}}
	}
	switch scenario {
	case "linear":
		if it.Name == "leaf" {
			return []map[string]any{call(item("root", 1))}
		}
	case "branch":
		if it.Name == "leaf" {
			return []map[string]any{call(item("a", 1)), call(item("b", 2))}
		}
	case "diamond", "shuffle-forward", "shuffle-reverse":
		if it.Name == "leaf" {
			x := []map[string]any{call(item("a", 1)), call(item("b", 2))}
			if scenario == "shuffle-reverse" {
				x[0], x[1] = x[1], x[0]
			}
			return x
		}
		if it.Name == "a" || it.Name == "b" {
			return []map[string]any{call(item("root", 3))}
		}
	case "cycle":
		if it.Name == "leaf" {
			return []map[string]any{call(item("root", 1))}
		}
		if it.Name == "root" {
			return []map[string]any{call(item("leaf", 0))}
		}
	}
	return []map[string]any{}
}
func writeFake(w io.Writer, id json.RawMessage, result any, rpcErr any) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result, "error": rpcErr})
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
func readFake(r *bufio.Reader) (fakeMessage, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return fakeMessage{}, err
	}
	if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
		return fakeMessage{}, fmt.Errorf("bad header %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Content-Length:")))
	if err != nil {
		return fakeMessage{}, err
	}
	if line, err = r.ReadString('\n'); err != nil || strings.TrimSpace(line) != "" {
		return fakeMessage{}, fmt.Errorf("bad separator")
	}
	body := make([]byte, n)
	if _, err = io.ReadFull(r, body); err != nil {
		return fakeMessage{}, err
	}
	var m fakeMessage
	err = json.Unmarshal(body, &m)
	return m, err
}
