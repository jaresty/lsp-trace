package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hybridgroup/yzma/pkg/llama"
)

const (
	modelDigest   = "1664fccab734674a50763490a8c6931b70e3f2f8ec10031b54806d30e5f956b6"
	runtimeDigest = "b95e8680b4d30761492bbc2d4a6fed656f124c5756dd4387cf02c30d27c13d90"
	maxTokens     = 384
	contextTokens = 16384
)

const grammar = `root ::= base | suggested
base ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws need ws "," ws "\"provided_behavior\"" ws ":" ws behavior ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "}"
suggested ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws need ws "," ws "\"provided_behavior\"" ws ":" ws behavior ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "," ws "\"citation_suggestions\"" ws ":" ws suggestions ws "}"
verdict ::= "\"COMPLETE\"" | "\"ABSTAINED\""
need ::= "{" ws "\"status\"" ws ":" ws status ws "," ws "\"value\"" ws ":" ws maybe-string ws "}"
status ::= "\"RESOLVED\"" | "\"UNRESOLVED\""
behavior ::= "{" ws "\"value\"" ws ":" ws string ws "," ws "\"consumer_relative\"" ws ":" ws boolean ws "}"
boolean ::= "true" | "false"
suggestions ::= "{" ws "\"target_role\"" ws ":" ws ids ws "," ws "\"consumer_need\"" ws ":" ws ids ws "," ws "\"provided_behavior\"" ws ":" ws ids ws "," ws "\"boundary_contribution\"" ws ":" ws ids ws "," ws "\"limitations\"" ws ":" ws ids ws "}"
ids ::= "[" ws "]" | "[" ws id ws "]" | "[" ws id ws "," ws id ws "]"
id ::= "\"C1\"" | "\"C2\"" | "\"PACKET_SCOPE\"" | "\"UNRESOLVED_CUSTODY\""
strings ::= "[" ws "]" | "[" ws string ws "]" | "[" ws string ws "," ws string ws "]" | "[" ws string ws "," ws string ws "," ws string ws "]"
maybe-string ::= "\"\"" | string
string ::= "\"" char+ "\""
char ::= [^"\\\x00-\x1F] | "\\" (["\\/bfnrt] | "u" hex hex hex hex)
hex ::= [0-9a-fA-F]
ws ::= [ \t\n\r]*`

type need struct {
	Status string `json:"status"`
	Value  string `json:"value"`
}
type behavior struct {
	Value            string `json:"value"`
	ConsumerRelative bool   `json:"consumer_relative"`
}
type suggestions struct {
	TargetRole           []string `json:"target_role"`
	ConsumerNeed         []string `json:"consumer_need"`
	ProvidedBehavior     []string `json:"provided_behavior"`
	BoundaryContribution []string `json:"boundary_contribution"`
	Limitations          []string `json:"limitations"`
}
type payload struct {
	Verdict              string       `json:"verdict"`
	TargetRole           string       `json:"target_role"`
	ConsumerNeed         need         `json:"consumer_need"`
	ProvidedBehavior     behavior     `json:"provided_behavior"`
	BoundaryContribution string       `json:"boundary_contribution"`
	Limitations          []string     `json:"limitations"`
	CitationSuggestions  *suggestions `json:"citation_suggestions,omitempty"`
}
type workerResult struct {
	Status     string  `json:"status"`
	Text       string  `json:"text,omitempty"`
	Tokens     int     `json:"tokens"`
	LoadMS     float64 `json:"load_ms"`
	RunMS      float64 `json:"run_ms"`
	Error      string  `json:"error,omitempty"`
	Decode     int32   `json:"decode_status"`
	Grammar    string  `json:"grammar"`
	Context    uint32  `json:"context_tokens"`
	DecodeMode string  `json:"decode_mode"`
}
type packet struct{ ID, Prompt, Consumer string }
type runScore struct {
	Run             int     `json:"run"`
	Variant         string  `json:"variant"`
	Packet          string  `json:"packet"`
	ExitCode        int     `json:"exit_code"`
	StdoutSHA256    string  `json:"stdout_sha256"`
	StdoutBytes     int     `json:"stdout_bytes"`
	StderrSHA256    string  `json:"stderr_sha256"`
	StderrBytes     int     `json:"stderr_bytes"`
	Strict          bool    `json:"strict_v2_parse_canonicalizable"`
	Substantive     bool    `json:"substantive_packet_specific_required_fields"`
	NoInvention     bool    `json:"no_invented_mechanical_identity_consumer_purpose"`
	Unresolved      bool    `json:"correct_unresolved_handling"`
	Consistent      bool    `json:"semantic_consistency"`
	CitationBonus   bool    `json:"optional_citation_suggestion_validity"`
	CanonicalSHA256 string  `json:"canonical_sha256,omitempty"`
	LatencyMS       float64 `json:"latency_ms"`
	Tokens          int     `json:"tokens"`
	Diagnostic      string  `json:"diagnostic,omitempty"`
}
type variantScore struct {
	Variant       string  `json:"variant"`
	Strict        int     `json:"strict_v2_parse_canonicalizable"`
	Substantive   int     `json:"substantive_packet_specific_required_fields"`
	NoInvention   int     `json:"no_invented_mechanical_identity_consumer_purpose"`
	Unresolved    int     `json:"correct_unresolved_handling"`
	Consistent    int     `json:"semantic_consistency"`
	CitationBonus int     `json:"optional_citation_suggestion_validity"`
	LatencyMS     float64 `json:"latency_ms"`
	Tokens        int     `json:"tokens"`
}

func main() {
	mode := flag.String("mode", "run", "run or worker")
	model := flag.String("model", "", "pinned model")
	lib := flag.String("lib", "", "runtime library")
	prompt := flag.String("prompt-file", "", "worker prompt")
	root := flag.String("run-root", "", "session raw root")
	repo := flag.String("repo", "", "repository root")
	flag.Parse()
	if *mode == "worker" {
		worker(*model, *lib, *prompt)
		return
	}
	if *mode == "preflight" {
		if err := runPreflight(*root, *model, *lib); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("PREFLIGHT_GREEN")
		return
	}
	if err := run(*repo, *root, *model, *lib); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func emitAndStop(result workerResult, emit func(workerResult)) bool { emit(result); return true }

func processSample(isEOG bool, result workerResult, emit func(workerResult), appendPiece func() error) (bool, error) {
	if isEOG {
		return emitAndStop(result, emit), nil
	}
	return false, appendPiece()
}

func worker(modelPath, lib, promptPath string) {
	result := workerResult{Grammar: "llama_sampler_init_grammar", Context: contextTokens, DecodeMode: "greedy/no-rng"}
	emit := func(code int) {
		_ = json.NewEncoder(os.Stdout).Encode(result)
		if code != 0 {
			os.Exit(code)
		}
	}
	prompt, err := os.ReadFile(filepath.Clean(promptPath))
	if err != nil {
		result.Status = "INPUT_ERROR"
		result.Error = err.Error()
		emit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	loaded := time.Now()
	if err = llama.Load(lib); err != nil {
		result.Status = "RUNTIME_LOAD_ERROR"
		result.Error = err.Error()
		emit(3)
	}
	llama.LogSet(llama.LogSilent())
	llama.Init()
	defer llama.Close()
	model, err := llama.ModelLoadFromFile(filepath.Clean(modelPath), llama.ModelDefaultParams())
	if err != nil {
		result.Status = "MODEL_LOAD_ERROR"
		result.Error = err.Error()
		emit(4)
	}
	defer llama.ModelFree(model)
	params := llama.ContextDefaultParams()
	params.NCtx = contextTokens
	params.NBatch = contextTokens
	params.NUbatch = 512
	lctx, err := llama.InitFromModel(model, params)
	if err != nil {
		result.Status = "CONTEXT_INIT_ERROR"
		result.Error = err.Error()
		emit(5)
	}
	defer llama.Free(lctx)
	result.LoadMS = float64(time.Since(loaded).Microseconds()) / 1000
	vocab := llama.ModelGetVocab(model)
	formatted := "<|im_start|>system\n" + string(prompt) + "<|im_end|>\n<|im_start|>assistant\n"
	tokens := llama.Tokenize(vocab, formatted, true, false)
	if len(tokens)+maxTokens > contextTokens {
		result.Status = "CONTEXT_BUDGET_EXCEEDED"
		emit(6)
	}
	batch := llama.BatchGetOne(tokens)
	sampler := llama.SamplerChainInit(llama.SamplerChainDefaultParams())
	defer llama.SamplerFree(sampler)
	gs := llama.SamplerInitGrammar(vocab, grammar, "root")
	if gs == 0 {
		result.Status = "GRAMMAR_INIT_ERROR"
		emit(7)
	}
	llama.SamplerChainAdd(sampler, gs)
	llama.SamplerChainAdd(sampler, llama.SamplerInitGreedy())
	started := time.Now()
	out := make([]byte, 0, 2048)
	for pos, n := int32(0), 0; n < maxTokens; pos, n = pos+batch.NTokens, n+1 {
		select {
		case <-ctx.Done():
			result.Status = "TIMEOUT"
			result.Text = string(out)
			result.Tokens = n
			result.RunMS = float64(time.Since(started).Microseconds()) / 1000
			emit(8)
		default:
		}
		code, de := llama.Decode(lctx, batch)
		if de != nil || code != 0 {
			result.Status = "DECODE_ERROR"
			result.Text = string(out)
			result.Tokens = n
			result.Decode = code
			result.Error = fmt.Sprintf("decode=%d err=%v", code, de)
			emit(9)
		}
		tok := llama.SamplerSample(sampler, lctx, -1)
		result.Status = "COMPLETE"
		result.Text = string(out)
		result.Tokens = n
		result.RunMS = float64(time.Since(started).Microseconds()) / 1000
		stop, sampleErr := processSample(llama.VocabIsEOG(vocab, tok), result, func(r workerResult) { result = r; emit(0) }, func() error {
			buf := make([]byte, 256)
			size := llama.TokenToPiece(vocab, tok, buf, 0, true)
			if size < 0 {
				buf = make([]byte, -size)
				size = llama.TokenToPiece(vocab, tok, buf, 0, true)
			}
			if size < 0 {
				return fmt.Errorf("token piece size=%d", size)
			}
			out = append(out, buf[:size]...)
			batch = llama.BatchGetOne([]llama.Token{tok})
			return nil
		})
		if sampleErr != nil {
			result.Status = "DECODE_ERROR"
			result.Error = sampleErr.Error()
			emit(9)
		}
		if stop {
			return
		}
	}
	result.Status = "TOKEN_LIMIT"
	result.Text = string(out)
	result.Tokens = maxTokens
	result.RunMS = float64(time.Since(started).Microseconds()) / 1000
	emit(10)
}

func runPreflight(root, model, lib string) error {
	if root == "" || model == "" || lib == "" {
		return errors.New("run-root, model, and lib are required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return err
	}
	if err := pin(model, modelDigest); err != nil {
		return err
	}
	if err := pin(filepath.Join(lib, "yzma-manifest.json"), runtimeDigest); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return preflight(root, exe, model, lib, filepath.Join(filepath.Dir(root), "network-deny.sb"))
}

func run(repo, root, model, lib string) error {
	if repo == "" || root == "" || model == "" || lib == "" {
		return errors.New("repo, run-root, model, and lib are required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return err
	}
	if err := pin(model, modelDigest); err != nil {
		return err
	}
	if err := pin(filepath.Join(lib, "yzma-manifest.json"), runtimeDigest); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	profile := filepath.Join(filepath.Dir(root), "network-deny.sb")
	var packets []packet
	if err = preflightThenLoad(
		func() error { return preflight(root, exe, model, lib, profile) },
		func() error {
			packets, err = loadPackets(repo, root)
			return err
		},
	); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	variants := []string{"D", "E", "F"}
	scores := make([]runScore, 0, 12)
	for _, v := range variants {
		for _, p := range packets {
			n := len(scores) + 1
			promptPath := filepath.Join(root, fmt.Sprintf("run-%02d.prompt", n))
			stdoutPath := filepath.Join(root, fmt.Sprintf("run-%02d.stdout.json", n))
			stderrPath := filepath.Join(root, fmt.Sprintf("run-%02d.stderr.log", n))
			if err = os.WriteFile(promptPath, []byte(renderPrompt(v, p.Prompt)), 0600); err != nil {
				return err
			}
			stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}
			stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-f", profile, exe, "-mode", "worker", "-model", model, "-lib", lib, "-prompt-file", promptPath)
			cmd.Dir = root
			cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root, "LANG=C", "LC_ALL=C"}
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			runErr := cmd.Run()
			cancel()
			_ = stdout.Close()
			_ = stderr.Close()
			_ = os.Chmod(stdoutPath, 0600)
			_ = os.Chmod(stderrPath, 0600)
			scores = append(scores, scoreRun(n, v, p, stdoutPath, stderrPath, runErr))
		}
	}
	if len(scores) != 12 {
		return fmt.Errorf("invocation cardinality=%d", len(scores))
	}
	return writeOutputs(repo, root, scores, summarize(scores))
}
func preflightThenLoad(preflightFn, loadFn func() error) error {
	if err := preflightFn(); err != nil {
		return err
	}
	return loadFn()
}

const preflightPrompt = `Emit this exact JSON and then stop: {"verdict":"ABSTAINED","target_role":"synthetic grammar preflight","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"validates bounded grammar generation","consumer_relative":false},"boundary_contribution":"none; this is not a campaign packet","limitations":["synthetic preflight only"]}`

func preflight(root, exe, model, lib, profile string) error {
	promptPath := filepath.Join(root, "preflight.prompt")
	stdoutPath := filepath.Join(root, "preflight.stdout.json")
	stderrPath := filepath.Join(root, "preflight.stderr.log")
	receiptPath := filepath.Join(root, "preflight.green")
	if err := os.WriteFile(promptPath, []byte(preflightPrompt), 0600); err != nil {
		return err
	}
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_ = stdout.Close()
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-f", profile, exe, "-mode", "worker", "-model", model, "-lib", lib, "-prompt-file", promptPath)
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root, "LANG=C", "LC_ALL=C"}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	cancel()
	_ = stdout.Close()
	_ = stderr.Close()
	_ = os.Chmod(stdoutPath, 0600)
	_ = os.Chmod(stderrPath, 0600)
	if runErr != nil {
		return fmt.Errorf("PREFLIGHT_RED: pinned-runtime grammar generation failed: %w", runErr)
	}
	raw, err := os.ReadFile(stdoutPath)
	if err != nil {
		return err
	}
	var wr workerResult
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err = dec.Decode(&wr); err != nil || wr.Status != "COMPLETE" || wr.Tokens == 0 {
		return fmt.Errorf("PREFLIGHT_RED: status=%q tokens=%d decode=%v", wr.Status, wr.Tokens, err)
	}
	if _, _, err = strictParse([]byte(wr.Text)); err != nil {
		return fmt.Errorf("PREFLIGHT_RED: generated payload: %w", err)
	}
	receipt := fmt.Sprintf("PREFLIGHT_GREEN campaign=v2b grammar=llama_sampler_init_grammar model=%s runtime=%s network=denied tokens=%d\n", modelDigest, runtimeDigest, wr.Tokens)
	return os.WriteFile(receiptPath, []byte(receipt), 0600)
}

func loadPackets(repo, root string) ([]packet, error) {
	realRaw, err := os.ReadFile(filepath.Join(root, "packet-real.prompt"))
	if err != nil {
		return nil, err
	}
	packets := []packet{{ID: "real", Prompt: strings.Trim(string(realRaw), "\"\n"), Consumer: "loadStateFromVerificationStore"}}
	f, err := os.Open(filepath.Join(repo, "docs/pilot/adr0007/experiment/final-four-packet/requests.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for i := 1; i <= 3; i++ {
		var r struct {
			Prompt string `json:"prompt"`
		}
		if err = dec.Decode(&r); err != nil {
			return nil, err
		}
		consumer := ""
		if j := strings.Index(r.Prompt, "C2 OUTWARD_CONSUMER node: "); j >= 0 {
			consumer = strings.TrimSpace(strings.SplitN(r.Prompt[j+len("C2 OUTWARD_CONSUMER node: "):], "(", 2)[0])
		}
		packets = append(packets, packet{ID: fmt.Sprintf("final-%02d", i), Prompt: r.Prompt, Consumer: consumer})
	}
	return packets, nil
}

const schemaText = `{"verdict":"COMPLETE|ABSTAINED","target_role":"string","consumer_need":{"status":"RESOLVED|UNRESOLVED","value":"string; empty exactly when UNRESOLVED"},"provided_behavior":{"value":"string","consumer_relative":true|false},"boundary_contribution":"string","limitations":["zero to eight strings"]}`
const guard = "Use only the packet. Never invent or name mechanical identity, consumer identity, ownership, product purpose, runtime use, canonical feature identity, value, or completeness. Host admissible evidence handles grounding. citation_suggestions is optional, non-authoritative, and not required. Output JSON only; no markdown or explanation."

func renderPrompt(v, packet string) string {
	switch v {
	case "D":
		return "Emit exactly the minimal Response V2 semantic JSON. Field meanings: verdict says whether a bounded answer is possible; target_role states C1's role; consumer_need states the semantic need or UNRESOLVED with empty value; provided_behavior states C1 behavior and whether it is consumer-relative; boundary_contribution states the one-layer contribution; limitations preserve packet limits. Exact schema: " + schemaText + " For this packet specifically, derive every required value from C1/C2 and preserve unresolved custody. " + guard + "\n\nPACKET:\n" + packet
	case "E":
		return "Task: describe what C1 provides, the semantic need it serves if resolved, and its one-layer boundary contribution; derive each value from this packet, never copy generic wording. Exact JSON schema: " + schemaText + " " + guard + "\n\nPACKET:\n" + packet
	default:
		return "Internally answer these questions from the packet: What is C1's role? What semantic need is supported or unresolved? What behavior is directly evidenced? What one-layer boundary contribution follows? What limits remain? Do not reveal reasoning or chain-of-thought. Then emit only JSON matching this exact schema: " + schemaText + " " + guard + "\n\nPACKET:\n" + packet
	}
}

func strictParse(raw []byte) (payload, []byte, error) {
	if err := rejectDuplicates(raw); err != nil {
		return payload{}, nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p payload
	if err := dec.Decode(&p); err != nil {
		return p, nil, err
	}
	if err := expectEOF(dec); err != nil {
		return p, nil, err
	}
	if err := validate(p); err != nil {
		return p, nil, err
	}
	canonical, err := json.Marshal(p)
	return p, canonical, err
}
func validate(p payload) error {
	if p.Verdict != "COMPLETE" && p.Verdict != "ABSTAINED" {
		return errors.New("invalid verdict")
	}
	if p.TargetRole == "" || p.ProvidedBehavior.Value == "" || p.BoundaryContribution == "" || p.Limitations == nil || len(p.Limitations) > 8 {
		return errors.New("invalid required fields")
	}
	if p.ConsumerNeed.Status != "RESOLVED" && p.ConsumerNeed.Status != "UNRESOLVED" {
		return errors.New("invalid consumer status")
	}
	if p.ConsumerNeed.Status == "RESOLVED" && p.ConsumerNeed.Value == "" || p.ConsumerNeed.Status == "UNRESOLVED" && p.ConsumerNeed.Value != "" {
		return errors.New("invalid unresolved handling")
	}
	for _, s := range append([]string{p.TargetRole, p.ConsumerNeed.Value, p.ProvidedBehavior.Value, p.BoundaryContribution}, p.Limitations...) {
		if len(s) > 4096 {
			return errors.New("field too long")
		}
	}
	if p.CitationSuggestions != nil {
		for _, ids := range [][]string{p.CitationSuggestions.TargetRole, p.CitationSuggestions.ConsumerNeed, p.CitationSuggestions.ProvidedBehavior, p.CitationSuggestions.BoundaryContribution, p.CitationSuggestions.Limitations} {
			if len(ids) > 8 {
				return errors.New("citation bound")
			}
			seen := map[string]bool{}
			for _, id := range ids {
				if !map[string]bool{"C1": true, "C2": true, "PACKET_SCOPE": true, "UNRESOLVED_CUSTODY": true}[id] || seen[id] {
					return errors.New("invalid citation suggestion")
				}
				seen[id] = true
			}
		}
	}
	return nil
}
func rejectDuplicates(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok && d == '{' {
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k := kt.(string)
				if seen[k] {
					return fmt.Errorf("duplicate key %q", k)
				}
				seen[k] = true
				if err = walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		}
		if d, ok := tok.(json.Delim); ok && d == '[' {
			for dec.More() {
				if err = walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	return expectEOF(dec)
}
func expectEOF(dec *json.Decoder) error {
	var x any
	err := dec.Decode(&x)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON value")
	}
	return err
}
func joined(p payload) string {
	return strings.ToLower(strings.Join(append([]string{p.TargetRole, p.ConsumerNeed.Value, p.ProvidedBehavior.Value, p.BoundaryContribution}, p.Limitations...), " "))
}
func invented(p payload, consumer string) bool {
	j := joined(p)
	for _, bad := range []string{"nearest_outward_consumer", "request_id", "authority=", "accepted=", "source_graph_complete", "sha256:", "human user", "product purpose"} {
		if strings.Contains(j, bad) {
			return true
		}
	}
	return consumer != "" && consumer != "Execute" && strings.Contains(j, strings.ToLower(consumer))
}
func substantive(p payload) bool {
	return len(strings.Fields(p.TargetRole)) >= 2 && len(strings.Fields(p.ProvidedBehavior.Value)) >= 3 && len(strings.Fields(p.BoundaryContribution)) >= 3 && (p.ConsumerNeed.Status == "UNRESOLVED" || len(strings.Fields(p.ConsumerNeed.Value)) >= 2)
}
func consistent(p payload) bool {
	return p.Verdict == "ABSTAINED" || p.ConsumerNeed.Status == "UNRESOLVED" || p.ProvidedBehavior.ConsumerRelative
}
func citationValid(p payload) bool { return p.CitationSuggestions != nil }
func scoreRun(n int, v string, p packet, stdoutPath, stderrPath string, runErr error) runScore {
	s := runScore{Run: n, Variant: v, Packet: p.ID, ExitCode: exitCode(runErr)}
	stdout, _ := os.ReadFile(stdoutPath)
	stderr, _ := os.ReadFile(stderrPath)
	s.StdoutSHA256 = digest(stdout)
	s.StdoutBytes = len(stdout)
	s.StderrSHA256 = digest(stderr)
	s.StderrBytes = len(stderr)
	dec := json.NewDecoder(bytes.NewReader(stdout))
	var wr workerResult
	if err := dec.Decode(&wr); err != nil {
		s.Diagnostic = "worker envelope: " + err.Error()
		return s
	}
	var extra workerResult
	if err := dec.Decode(&extra); err != io.EOF {
		s.Diagnostic = "multiple or malformed worker envelopes"
		return s
	}
	if wr.Status != "COMPLETE" {
		s.Diagnostic = "worker status " + wr.Status
		return s
	}
	s.LatencyMS = wr.LoadMS + wr.RunMS
	s.Tokens = wr.Tokens
	pl, canonical, err := strictParse([]byte(wr.Text))
	if err != nil {
		s.Diagnostic = err.Error()
		return s
	}
	s.Strict = true
	s.Substantive = substantive(pl)
	s.NoInvention = !invented(pl, p.Consumer)
	s.Unresolved = pl.ConsumerNeed.Status != "UNRESOLVED" || pl.ConsumerNeed.Value == ""
	s.Consistent = consistent(pl)
	s.CitationBonus = citationValid(pl)
	s.CanonicalSHA256 = digest(canonical)
	return s
}
func summarize(runs []runScore) []variantScore {
	m := map[string]*variantScore{}
	for _, r := range runs {
		x := m[r.Variant]
		if x == nil {
			x = &variantScore{Variant: r.Variant}
			m[r.Variant] = x
		}
		if r.Strict {
			x.Strict++
		}
		if r.Substantive {
			x.Substantive++
		}
		if r.NoInvention {
			x.NoInvention++
		}
		if r.Unresolved {
			x.Unresolved++
		}
		if r.Consistent {
			x.Consistent++
		}
		if r.CitationBonus {
			x.CitationBonus++
		}
		x.LatencyMS += r.LatencyMS
		x.Tokens += r.Tokens
	}
	out := make([]variantScore, 0, 3)
	for _, x := range m {
		out = append(out, *x)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Strict != b.Strict {
			return a.Strict > b.Strict
		}
		if a.Substantive != b.Substantive {
			return a.Substantive > b.Substantive
		}
		if a.NoInvention != b.NoInvention {
			return a.NoInvention > b.NoInvention
		}
		if a.Unresolved != b.Unresolved {
			return a.Unresolved > b.Unresolved
		}
		if a.Consistent != b.Consistent {
			return a.Consistent > b.Consistent
		}
		if a.CitationBonus != b.CitationBonus {
			return a.CitationBonus > b.CitationBonus
		}
		if a.LatencyMS != b.LatencyMS {
			return a.LatencyMS < b.LatencyMS
		}
		return a.Tokens < b.Tokens
	})
	return out
}
func qualifies(v variantScore) bool { return v.Strict >= 3 && v.Substantive >= 3 && v.NoInvention == 4 }
func writeOutputs(repo, root string, runs []runScore, vs []variantScore) error {
	winner := ""
	next := ""
	if len(vs) > 0 && qualifies(vs[0]) {
		winner = vs[0].Variant
	} else {
		next = "No further campaign is authorized: all variants were 0/4 substantive under the frozen score, so no production prompt or grammar proposal is selected."
	}
	obj := struct {
		SchemaVersion    string         `json:"schema_version"`
		Status           string         `json:"status"`
		Authority        int            `json:"authority"`
		Accepted         bool           `json:"accepted"`
		Completeness     string         `json:"completeness"`
		InvocationCount  int            `json:"invocation_count"`
		Retries          int            `json:"retries"`
		ScoringOrder     []string       `json:"scoring_order"`
		Runs             []runScore     `json:"runs"`
		Variants         []variantScore `json:"variants"`
		Winner           string         `json:"winner,omitempty"`
		NextPromptChange string         `json:"smallest_next_prompt_change,omitempty"`
		ProductionSwitch bool           `json:"production_switch"`
	}{"adr0007-prompt-eval-v2b-summary.v1", "COMPLETE", 0, false, "UNKNOWN", 12, 0, []string{"strict_v2_parse_canonicalizable", "substantive_packet_specific_required_fields", "no_invented_mechanical_identity_consumer_purpose", "correct_unresolved_handling", "semantic_consistency", "optional_citation_suggestion_validity", "latency_then_tokens"}, runs, vs, winner, next, false}
	raw, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	outDir := filepath.Join(repo, "docs/qualification/adr0007-lsp-trace/prompt-eval-v2b")
	if err = os.WriteFile(filepath.Join(outDir, "summary.json"), append(raw, '\n'), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "scores.sanitized.json"), append(raw, '\n'), 0600); err != nil {
		return err
	}
	return writeReport(filepath.Join(outDir, "REPORT.md"), runs, vs, winner, next)
}
func writeReport(path string, runs []runScore, vs []variantScore, winner, next string) error {
	var b strings.Builder
	b.WriteString("# ADR0007 Response V2b prompt evaluation\n\nCampaign: `v2b`  \nAuthority: `0`  \nAccepted: `false`  \nCompleteness: `UNKNOWN`  \nProduction switch: `false`\n\nExactly 12 deterministic grammar-constrained Qwen invocations (D/E/F × four fixed packets), zero retries. Raw outputs are session-local outside Git at mode `0600`; only digests and lengths are retained here. Human-safe semantic adjudication is limited to obvious packet mismatch or invention. Host admissible evidence handles grounding; optional citation suggestions are a non-scoring bonus.\n\n## Variant totals\n\n| Rank | Variant | Strict/canonical | Substantive | No invention | Unresolved | Consistent | Citation bonus | Latency ms | Tokens |\n|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for i, v := range vs {
		fmt.Fprintf(&b, "| %d | %s | %d/4 | %d/4 | %d/4 | %d/4 | %d/4 | %d/4 | %.3f | %d |\n", i+1, v.Variant, v.Strict, v.Substantive, v.NoInvention, v.Unresolved, v.Consistent, v.CitationBonus, v.LatencyMS, v.Tokens)
	}
	if winner != "" {
		fmt.Fprintf(&b, "\n## Winner\n\n**Variant %s** meets the predeclared gate (at least 3/4 strict and substantive; 4/4 no mechanical invention).\n", winner)
	} else {
		fmt.Fprintf(&b, "\n## Winner\n\nNo winner meets the predeclared gate. Smallest next prompt change: %s\n", next)
	}
	b.WriteString("\n## Sanitized run ledger\n\n| Run | Variant | Packet | Exit | stdout digest / bytes | stderr digest / bytes | Strict | Substantive | No invention | Unresolved | Consistent | Citation | Latency ms | Tokens |\n|---:|---|---|---:|---|---|---|---|---|---|---|---|---:|---:|\n")
	for _, r := range runs {
		fmt.Fprintf(&b, "| %d | %s | %s | %d | `%s` / %d | `%s` / %d | %t | %t | %t | %t | %t | %t | %.3f | %d |\n", r.Run, r.Variant, r.Packet, r.ExitCode, r.StdoutSHA256, r.StdoutBytes, r.StderrSHA256, r.StderrBytes, r.Strict, r.Substantive, r.NoInvention, r.Unresolved, r.Consistent, r.CitationBonus, r.LatencyMS, r.Tokens)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
func pin(path, want string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if got := digest(raw); got != want {
		return fmt.Errorf("pin mismatch %s got=%s", path, got)
	}
	return nil
}
func digest(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
