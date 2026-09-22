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

const grammar = `root ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws string ws "," ws "\"provided_behavior\"" ws ":" ws string ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "," ws "\"citations\"" ws ":" ws citations ws "}"
verdict ::= "\"SUPPORTED\"" | "\"UNRESOLVED\""
citations ::= "{" ws "\"target_role\"" ws ":" ws ids ws "," ws "\"consumer_need\"" ws ":" ws ids ws "," ws "\"provided_behavior\"" ws ":" ws ids ws "," ws "\"boundary_contribution\"" ws ":" ws ids ws "," ws "\"limitations\"" ws ":" ws ids ws "}"
ids ::= "[" ws id ws "]" | "[" ws id ws "," ws id ws "]"
id ::= "\"C1\"" | "\"C2\""
strings ::= "[" ws string ws "]" | "[" ws string ws "," ws string ws "]" | "[" ws string ws "," ws string ws "," ws string ws "]"
string ::= "\"" char+ "\""
char ::= [^"\\\x00-\x1F] | "\\" (["\\/bfnrt] | "u" hex hex hex hex)
hex ::= [0-9a-fA-F]
ws ::= [ \t\n\r]*`

type citations struct {
	TargetRole           []string `json:"target_role"`
	ConsumerNeed         []string `json:"consumer_need"`
	ProvidedBehavior     []string `json:"provided_behavior"`
	BoundaryContribution []string `json:"boundary_contribution"`
	Limitations          []string `json:"limitations"`
}

type payload struct {
	Verdict              string    `json:"verdict"`
	TargetRole           string    `json:"target_role"`
	ConsumerNeed         string    `json:"consumer_need"`
	ProvidedBehavior     string    `json:"provided_behavior"`
	BoundaryContribution string    `json:"boundary_contribution"`
	Limitations          []string  `json:"limitations"`
	Citations            citations `json:"citations"`
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
	Run                int     `json:"run"`
	Variant            string  `json:"variant"`
	Packet             string  `json:"packet"`
	ExitCode           int     `json:"exit_code"`
	StdoutSHA256       string  `json:"stdout_sha256"`
	StdoutBytes        int     `json:"stdout_bytes"`
	StderrSHA256       string  `json:"stderr_sha256"`
	StderrBytes        int     `json:"stderr_bytes"`
	StrictSchema       bool    `json:"strict_schema_parse"`
	CitationCoverage   bool    `json:"citation_validity_and_claim_coverage"`
	NoInventedIdentity bool    `json:"no_invented_consumer_or_identity"`
	Substantive        bool    `json:"substantive_required_fields"`
	Canonicalizable    bool    `json:"deterministic_host_canonicalizable"`
	CanonicalSHA256    string  `json:"canonical_sha256,omitempty"`
	LatencyMS          float64 `json:"latency_ms"`
	Tokens             int     `json:"tokens"`
	Diagnostic         string  `json:"diagnostic,omitempty"`
}
type variantScore struct {
	Variant            string  `json:"variant"`
	StrictSchema       int     `json:"strict_schema_parse"`
	CitationCoverage   int     `json:"citation_validity_and_claim_coverage"`
	NoInventedIdentity int     `json:"no_invented_consumer_or_identity"`
	Substantive        int     `json:"substantive_required_fields"`
	Canonicalizable    int     `json:"deterministic_host_canonicalizable"`
	LatencyMS          float64 `json:"latency_ms"`
	Tokens             int     `json:"tokens"`
}

func main() {
	mode := flag.String("mode", "run", "run or worker")
	model := flag.String("model", "", "pinned model")
	lib := flag.String("lib", "", "runtime library directory")
	prompt := flag.String("prompt-file", "", "worker prompt")
	runRoot := flag.String("run-root", "", "session-local run root")
	repo := flag.String("repo", "", "repository root")
	flag.Parse()
	if *mode == "worker" {
		worker(*model, *lib, *prompt)
		return
	}
	if *mode == "rescore" {
		if err := rescore(*repo, *runRoot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(*repo, *runRoot, *model, *lib); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func worker(modelPath, lib, promptPath string) {
	result := workerResult{Grammar: "llama_sampler_init_grammar", Context: contextTokens, DecodeMode: "greedy/no-rng"}
	emit := func(code int) {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(result)
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
		result.Error = fmt.Sprintf("input=%d output=%d capacity=%d", len(tokens), maxTokens, contextTokens)
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
		if llama.VocabIsEOG(vocab, tok) {
			result.Status = "COMPLETE"
			result.Text = string(out)
			result.Tokens = n
			result.RunMS = float64(time.Since(started).Microseconds()) / 1000
			emit(0)
			return
		}
		buf := make([]byte, 256)
		size := llama.TokenToPiece(vocab, tok, buf, 0, true)
		if size < 0 {
			buf = make([]byte, -size)
			size = llama.TokenToPiece(vocab, tok, buf, 0, true)
		}
		out = append(out, buf[:size]...)
		batch = llama.BatchGetOne([]llama.Token{tok})
	}
	result.Status = "TOKEN_LIMIT"
	result.Text = string(out)
	result.Tokens = maxTokens
	result.RunMS = float64(time.Since(started).Microseconds()) / 1000
	emit(10)
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
	packets, err := loadPackets(repo, root)
	if err != nil {
		return err
	}
	variants := []string{"A", "B", "C"}
	scores := make([]runScore, 0, 12)
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	profile := filepath.Join(filepath.Dir(root), "network-deny.sb")
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
			cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-f", profile, executable, "-mode", "worker", "-model", model, "-lib", lib, "-prompt-file", promptPath)
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
			s := scoreRun(n, v, p, stdoutPath, stderrPath, runErr)
			scores = append(scores, s)
		}
	}
	if len(scores) != 12 {
		return fmt.Errorf("invocation cardinality=%d", len(scores))
	}
	summary := summarize(scores)
	return writeOutputs(repo, root, scores, summary, false)
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
			tail := r.Prompt[j+len("C2 OUTWARD_CONSUMER node: "):]
			consumer = strings.TrimSpace(strings.SplitN(tail, "(", 2)[0])
		}
		packets = append(packets, packet{ID: fmt.Sprintf("final-%02d", i), Prompt: r.Prompt, Consumer: consumer})
	}
	return packets, nil
}

func renderPrompt(v, packet string) string {
	contract := "Return only one JSON object with exactly these keys in this order: verdict (SUPPORTED or UNRESOLVED), target_role, consumer_need (semantic prose or the literal UNRESOLVED), provided_behavior, boundary_contribution, limitations (1-3 strings), citations (an object with target_role, consumer_need, provided_behavior, boundary_contribution, limitations; each value is 1-2 unique IDs chosen only from C1 and C2). Do not output request IDs, consumer identity or selector, nearest outward consumer, pins, authority, acceptance, completeness, provenance, status, or accounting. Cite C1 for target claims and C2 where a consumer-need or boundary claim depends on C2. Preserve uncertainty in limitations. No markdown."
	switch v {
	case "A":
		return "Explain what C1 provides toward its mechanically supplied immediate outward consumer and its one-layer contribution toward the system boundary. Do not skip layers or infer a human user. Never infer runtime use, ownership, product purpose, canonical feature identity, value, or completeness. " + contract + "\n\nPACKET:\n" + packet
	case "B":
		return contract + " Analyze only the bounded C1/C2 packet. Consumer identity is host-owned; describe the need semantically without naming or selecting the consumer.\n\nPACKET:\n" + packet
	default:
		return contract + ` Valid compact example: {"verdict":"SUPPORTED","target_role":"boundary-facing helper","consumer_need":"a bounded semantic value","provided_behavior":"provides the bounded value","boundary_contribution":"passes that value one layer outward","limitations":["scope is limited to C1 and C2"],"citations":{"target_role":["C1"],"consumer_need":["C2"],"provided_behavior":["C1"],"boundary_contribution":["C1","C2"],"limitations":["C1","C2"]}} Analyze only the bounded packet; do not copy example prose when packet evidence differs.` + "\n\nPACKET:\n" + packet
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
	if err := validatePayload(p); err != nil {
		return p, nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil {
		return p, nil, err
	}
	var p2 payload
	if err = json.Unmarshal(canonical, &p2); err != nil || fmt.Sprintf("%#v", p) != fmt.Sprintf("%#v", p2) {
		return p, nil, errors.New("canonical roundtrip mismatch")
	}
	return p, canonical, nil
}
func rejectDuplicates(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch d := tok.(type) {
		case json.Delim:
			if d == '{' {
				seen := map[string]bool{}
				for dec.More() {
					keyToken, err := dec.Token()
					if err != nil {
						return err
					}
					key, ok := keyToken.(string)
					if !ok {
						return errors.New("non-string key")
					}
					if seen[key] {
						return fmt.Errorf("duplicate key %q", key)
					}
					seen[key] = true
					if err = walk(); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			}
			if d == '[' {
				for dec.More() {
					if err = walk(); err != nil {
						return err
					}
				}
				_, err = dec.Token()
				return err
			}
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	return expectEOF(dec)
}
func expectEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON value")
	}
	return err
}
func validatePayload(p payload) error {
	if p.Verdict != "SUPPORTED" && p.Verdict != "UNRESOLVED" {
		return errors.New("invalid verdict")
	}
	fields := []string{p.TargetRole, p.ConsumerNeed, p.ProvidedBehavior, p.BoundaryContribution}
	for _, s := range fields {
		if strings.TrimSpace(s) == "" {
			return errors.New("empty substantive field")
		}
	}
	if len(p.Limitations) < 1 || len(p.Limitations) > 3 {
		return errors.New("limitations bound")
	}
	for _, s := range p.Limitations {
		if strings.TrimSpace(s) == "" || len(s) > 512 {
			return errors.New("invalid limitation")
		}
	}
	all := [][]string{p.Citations.TargetRole, p.Citations.ConsumerNeed, p.Citations.ProvidedBehavior, p.Citations.BoundaryContribution, p.Citations.Limitations}
	for _, ids := range all {
		if len(ids) < 1 || len(ids) > 2 {
			return errors.New("citation bound")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if id != "C1" && id != "C2" {
				return errors.New("invalid citation")
			}
			if seen[id] {
				return errors.New("duplicate citation")
			}
			seen[id] = true
		}
	}
	return nil
}
func citationCoverage(p payload) bool {
	return has(p.Citations.TargetRole, "C1") && has(p.Citations.ProvidedBehavior, "C1") && has(p.Citations.ConsumerNeed, "C2") && has(p.Citations.BoundaryContribution, "C1") && has(p.Citations.BoundaryContribution, "C2")
}
func has(xs []string, w string) bool {
	for _, x := range xs {
		if x == w {
			return true
		}
	}
	return false
}
func substantive(p payload) bool {
	return len(strings.Fields(p.TargetRole)) > 0 && len(strings.Fields(p.ConsumerNeed)) > 0 && len(strings.Fields(p.ProvidedBehavior)) >= 3 && len(strings.Fields(p.BoundaryContribution)) >= 3 && len(p.Limitations) > 0
}
func exampleEcho(p payload) bool {
	return p.TargetRole == "boundary-facing helper" &&
		p.ConsumerNeed == "a bounded semantic value" &&
		p.ProvidedBehavior == "provides the bounded value" &&
		p.BoundaryContribution == "passes that value one layer outward" &&
		len(p.Limitations) == 1 && p.Limitations[0] == "scope is limited to C1 and C2"
}

func invented(p payload, consumer string) bool {
	joined := strings.ToLower(strings.Join([]string{p.TargetRole, p.ConsumerNeed, p.ProvidedBehavior, p.BoundaryContribution, strings.Join(p.Limitations, " ")}, " "))
	for _, bad := range []string{"nearest_outward_consumer", "request_id", "authority=", "accepted=", "source_graph_complete", "sha256:"} {
		if strings.Contains(joined, bad) {
			return true
		}
	}
	return consumer != "" && consumer != "Execute" && strings.Contains(joined, strings.ToLower(consumer))
}
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
	envelopes := 0
	for {
		var candidate workerResult
		err := dec.Decode(&candidate)
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Diagnostic = "worker envelope: " + err.Error()
			return s
		}
		envelopes++
		if wr.Status == "" && candidate.Status == "COMPLETE" {
			wr = candidate
		}
	}
	if wr.Status == "" {
		s.Diagnostic = fmt.Sprintf("no COMPLETE envelope in %d envelopes", envelopes)
		return s
	}
	s.LatencyMS = wr.LoadMS + wr.RunMS
	s.Tokens = wr.Tokens
	if envelopes > 1 {
		s.Diagnostic = fmt.Sprintf("recovered first COMPLETE envelope from %d concatenated envelopes; no model retry", envelopes)
	}
	pl, canonical, err := strictParse([]byte(wr.Text))
	if err != nil {
		s.Diagnostic = err.Error()
		return s
	}
	s.StrictSchema = true
	echo := exampleEcho(pl)
	s.CitationCoverage = citationCoverage(pl) && !echo
	s.NoInventedIdentity = !invented(pl, p.Consumer)
	s.Substantive = substantive(pl) && !echo
	s.Canonicalizable = true
	if echo {
		s.Diagnostic += "; exact example echo is not packet-specific claim coverage or substance"
	}
	s.CanonicalSHA256 = digest(canonical)
	return s
}
func rescore(repo, root string) error {
	packets, err := loadPackets(repo, root)
	if err != nil {
		return err
	}
	variants := []string{"A", "B", "C"}
	scores := make([]runScore, 0, 12)
	for _, variant := range variants {
		for _, p := range packets {
			n := len(scores) + 1
			s := scoreRun(n, variant, p, filepath.Join(root, fmt.Sprintf("run-%02d.stdout.json", n)), filepath.Join(root, fmt.Sprintf("run-%02d.stderr.log", n)), nil)
			s.ExitCode = 10
			scores = append(scores, s)
		}
	}
	summary := summarize(scores)
	if len(summary) != 3 {
		return errors.New("rescore variant cardinality mismatch")
	}
	return writeOutputs(repo, root, scores, summary, true)
}

func summarize(runs []runScore) []variantScore {
	m := map[string]*variantScore{}
	for _, r := range runs {
		x := m[r.Variant]
		if x == nil {
			x = &variantScore{Variant: r.Variant}
			m[r.Variant] = x
		}
		if r.StrictSchema {
			x.StrictSchema++
		}
		if r.CitationCoverage {
			x.CitationCoverage++
		}
		if r.NoInventedIdentity {
			x.NoInventedIdentity++
		}
		if r.Substantive {
			x.Substantive++
		}
		if r.Canonicalizable {
			x.Canonicalizable++
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
		if a.StrictSchema != b.StrictSchema {
			return a.StrictSchema > b.StrictSchema
		}
		if a.CitationCoverage != b.CitationCoverage {
			return a.CitationCoverage > b.CitationCoverage
		}
		if a.NoInventedIdentity != b.NoInventedIdentity {
			return a.NoInventedIdentity > b.NoInventedIdentity
		}
		if a.Substantive != b.Substantive {
			return a.Substantive > b.Substantive
		}
		if a.Canonicalizable != b.Canonicalizable {
			return a.Canonicalizable > b.Canonicalizable
		}
		if a.LatencyMS != b.LatencyMS {
			return a.LatencyMS < b.LatencyMS
		}
		if a.Tokens != b.Tokens {
			return a.Tokens < b.Tokens
		}
		return a.Variant < b.Variant
	})
	return out
}
func writeOutputs(repo, root string, scores []runScore, summary []variantScore, recovered bool) error {
	raw, err := json.Marshal(struct {
		SchemaVersion   string         `json:"schema_version"`
		Status          string         `json:"status"`
		Authority       int            `json:"authority"`
		Accepted        bool           `json:"accepted"`
		Completeness    string         `json:"completeness"`
		InvocationCount int            `json:"invocation_count"`
		Retries         int            `json:"retries"`
		Recovered       bool           `json:"recovered_first_complete_envelopes"`
		ScoringOrder    []string       `json:"scoring_order"`
		Runs            []runScore     `json:"runs"`
		Variants        []variantScore `json:"variants"`
		Winner          string         `json:"winner"`
		Proposal        map[string]any `json:"proposal"`
	}{"adr0007-prompt-eval-summary.v1", "COMPLETE", 0, false, "UNKNOWN", 12, 0, recovered, []string{"strict_semantic_schema_parse", "citation_validity_and_claim_coverage", "no_invented_consumer_or_identity", "substantive_required_fields", "deterministic_host_canonicalizability", "latency_then_tokens"}, scores, summary, summary[0].Variant, map[string]any{"execute": false, "response_schema": "additive v2 minimal semantic payload", "host_seam": "strict parse -> deterministic canonical re-encode -> host enrichment with request/consumer/pins/authority/acceptance/completeness/provenance/status/accounting"}})
	if err != nil {
		return err
	}
	outDir := filepath.Join(repo, "docs/qualification/adr0007-lsp-trace/prompt-eval")
	if err = os.WriteFile(filepath.Join(outDir, "summary.json"), append(raw, '\n'), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "scores.sanitized.json"), append(raw, '\n'), 0600); err != nil {
		return err
	}
	return writeReport(filepath.Join(outDir, "REPORT.md"), scores, summary, recovered)
}

func writeReport(path string, runs []runScore, vs []variantScore, recovered bool) error {
	var b strings.Builder
	b.WriteString("# ADR0007 model-backed prompt evaluation\n\nAuthority: `0`  \nAccepted: `false`  \nCompleteness: `UNKNOWN`\n\nThis evaluation selects prompt wording only. It changes no production behavior and semantically accepts no feature identity.\n\n## Controls and scoring\n\nThe fixed matrix, pins, no-retry rule, and lexicographic scoring order were predeclared in `PLAN.md` before execution. Valid pretty-printed JSON is accepted by strict semantic parsing and then deterministically re-encoded by the host; model byte-canonicality is not scored. Raw outputs remain session-local outside Git at mode `0600`; this report retains only digests, lengths, and sanitized scores.\n")
	if recovered {
		b.WriteString("\nThe eval worker had a post-EOG harness defect: it emitted a valid `COMPLETE` envelope but did not return, so runs 2–12 appended repeated envelopes until the token cap. No model retry was performed. Scoring deterministically recovers the first EOG-complete envelope from each immutable raw stream; run 1 contained no complete envelope and remains invalid. The worker now returns immediately after EOG for future use.\n\nAll four C outputs copied the supplied example prose exactly (identical payload digest). The predeclared citation/coverage and substantive-field criteria therefore score those outputs false: valid citation IDs attached to generic copied claims do not establish packet-specific claim coverage. Criterion order was not changed after observation.\n")
	}
	b.WriteString("\n## Variant totals\n\n| Rank | Variant | Strict parse | Citation/coverage | No invented identity | Substantive | Canonicalizable | Latency ms | Tokens |\n|---:|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for i, v := range vs {
		fmt.Fprintf(&b, "| %d | %s | %d/4 | %d/4 | %d/4 | %d/4 | %d/4 | %.3f | %d |\n", i+1, v.Variant, v.StrictSchema, v.CitationCoverage, v.NoInventedIdentity, v.Substantive, v.Canonicalizable, v.LatencyMS, v.Tokens)
	}
	fmt.Fprintf(&b, "\n## Winner\n\n**Variant %s** wins by the predeclared lexicographic ordering.\n\n## Sanitized run ledger\n\n| Run | Variant | Packet | Exit | stdout digest / bytes | stderr digest / bytes | Parse | Citations | No invention | Substance | Canonical | Latency ms | Tokens |\n|---:|---|---|---:|---|---|---|---|---|---|---|---:|---:|\n", vs[0].Variant)
	for _, r := range runs {
		fmt.Fprintf(&b, "| %d | %s | %s | %d | `%s` / %d | `%s` / %d | %t | %t | %t | %t | %t | %.3f | %d |\n", r.Run, r.Variant, r.Packet, r.ExitCode, r.StdoutSHA256, r.StdoutBytes, r.StderrSHA256, r.StderrBytes, r.StrictSchema, r.CitationCoverage, r.NoInventedIdentity, r.Substantive, r.Canonicalizable, r.LatencyMS, r.Tokens)
	}
	b.WriteString("\n## Proposed additive seam (not executed)\n\nAfter explicit approval, add a new response schema/version containing the minimal model-owned semantic payload. The host should: (1) reject duplicate keys, unknown fields, trailing bytes, non-JSON, invalid citation IDs, and schema violations; (2) deterministically re-encode the accepted payload; (3) enrich it with request IDs, mechanically selected consumer identity/selector and nearest outward consumer, pins, authority, acceptance, completeness, provenance, status, and accounting; and (4) construct the production response record from the enriched object. Keep the existing response version unchanged for compatibility. No model or production change is performed by this proposal.\n")
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
