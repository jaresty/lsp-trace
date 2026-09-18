package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hybridgroup/yzma/pkg/llama"
)

type result struct {
	Status    string  `json:"status"`
	Text      string  `json:"text,omitempty"`
	Tokens    int     `json:"tokens"`
	LoadMS    float64 `json:"load_ms"`
	RunMS     float64 `json:"run_ms"`
	Cancelled bool    `json:"cancelled"`
	Decode    int32   `json:"decode_status"`
	Error     string  `json:"error,omitempty"`
	Grammar   string  `json:"grammar"`
	Context   uint32  `json:"context_tokens"`
}

const grammar = `root ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"nearest_outward_consumer\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws string ws "," ws "\"provided_behavior\"" ws ":" ws string ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "," ws "\"citations\"" ws ":" ws citations ws "}"
verdict ::= "\"SUPPORTED\""
citations ::= "[" ws "]" | "[" ws citation ws "]" | "[" ws citation ws "," ws citation ws "]" | "[" ws citation ws "," ws citation ws "," ws citation ws "]"
citation ::= "\"C1\"" | "\"C2\"" | "\"C3\"" | "\"C4\"" | "\"C5\""
strings ::= "[" ws "]" | "[" ws string ws "]" | "[" ws string ws "," ws string ws "]" | "[" ws string ws "," ws string ws "," ws string ws "]"
string ::= "\"" char* "\""
char ::= [^"\\\x00-\x1F] | "\\" (["\\/bfnrt] | "u" hex hex hex hex)
hex ::= [0-9a-fA-F]
ws ::= [ \t\n\r]*`

func main() {
	modelPath := flag.String("model", "", "model")
	lib := flag.String("lib", "", "native library directory")
	promptPath := flag.String("prompt-file", "", "prompt file")
	timeout := flag.Duration("timeout", 90*time.Second, "timeout")
	maxTokens := flag.Int("max-tokens", 384, "generation cap")
	contextTokens := flag.Uint("context-tokens", 16384, "context capacity")
	flag.Parse()

	prompt, err := os.ReadFile(filepath.Clean(*promptPath))
	if err != nil {
		emit(result{Status: "INPUT_ERROR", Error: err.Error(), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(2)
	}
	ctxCancel, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	loaded := time.Now()
	if err := llama.Load(*lib); err != nil {
		emit(result{Status: "RUNTIME_LOAD_ERROR", Error: err.Error(), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(3)
	}
	llama.LogSet(llama.LogSilent())
	llama.Init()
	defer llama.Close()
	model, err := llama.ModelLoadFromFile(filepath.Clean(*modelPath), llama.ModelDefaultParams())
	if err != nil {
		emit(result{Status: "MODEL_LOAD_ERROR", Error: err.Error(), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(4)
	}
	defer llama.ModelFree(model)
	params := llama.ContextDefaultParams()
	params.NCtx = uint32(*contextTokens)
	params.NBatch = 16384
	params.NUbatch = 512
	lctx, err := llama.InitFromModel(model, params)
	if err != nil {
		emit(result{Status: "CONTEXT_INIT_ERROR", Error: err.Error(), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(5)
	}
	defer llama.Free(lctx)
	loadMS := float64(time.Since(loaded).Microseconds()) / 1000
	vocab := llama.ModelGetVocab(model)
	formatted := "<|im_start|>system\nReturn only JSON matching the required contract. Treat the nearest evidenced caller toward the outside of the system as the target object’s relative user. Explain what the target provides to that consumer and how it contributes one layer toward the boundary. Do not skip intermediate layers or infer a human user. The packet supplies one mechanically established outward consumer, so verdict must be SUPPORTED. Preserve any interpretation uncertainty in limitations. Never infer runtime use, ownership, product purpose, canonical feature identity, value, or completeness. No markdown.<|im_end|>\n<|im_start|>user\n" + string(prompt) + "<|im_end|>\n<|im_start|>assistant\n"
	tokens := llama.Tokenize(vocab, formatted, true, false)
	if len(tokens)+*maxTokens > int(*contextTokens) {
		emit(result{Status: "CONTEXT_BUDGET_EXCEEDED", Tokens: len(tokens), LoadMS: loadMS, Error: fmt.Sprintf("input=%d output=%d capacity=%d", len(tokens), *maxTokens, *contextTokens), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(6)
	}
	batch := llama.BatchGetOne(tokens)
	sampler := llama.SamplerChainInit(llama.SamplerChainDefaultParams())
	defer llama.SamplerFree(sampler)
	gs := llama.SamplerInitGrammar(vocab, grammar, "root")
	if gs == 0 {
		emit(result{Status: "GRAMMAR_INIT_ERROR", LoadMS: loadMS, Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
		os.Exit(7)
	}
	llama.SamplerChainAdd(sampler, gs)
	llama.SamplerChainAdd(sampler, llama.SamplerInitGreedy())
	started := time.Now()
	out := make([]byte, 0, 2048)
	n := 0
	for pos := int32(0); n < *maxTokens; pos += batch.NTokens {
		select {
		case <-ctxCancel.Done():
			emit(result{Status: "TIMEOUT", Text: string(out), Tokens: n, LoadMS: loadMS, RunMS: float64(time.Since(started).Microseconds()) / 1000, Cancelled: true, Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
			os.Exit(8)
		default:
		}
		code, decodeErr := llama.Decode(lctx, batch)
		if decodeErr != nil || code != 0 {
			emit(result{Status: "DECODE_ERROR", Text: string(out), Tokens: n, LoadMS: loadMS, RunMS: float64(time.Since(started).Microseconds()) / 1000, Decode: code, Error: fmt.Sprintf("decode=%d err=%v", code, decodeErr), Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
			os.Exit(9)
		}
		tok := llama.SamplerSample(sampler, lctx, -1)
		if llama.VocabIsEOG(vocab, tok) {
			break
		}
		buf := make([]byte, 256)
		size := llama.TokenToPiece(vocab, tok, buf, 0, true)
		if size < 0 {
			buf = make([]byte, -size)
			size = llama.TokenToPiece(vocab, tok, buf, 0, true)
		}
		out = append(out, buf[:size]...)
		n++
		batch = llama.BatchGetOne([]llama.Token{tok})
	}
	emit(result{Status: "COMPLETE", Text: string(out), Tokens: n, LoadMS: loadMS, RunMS: float64(time.Since(started).Microseconds()) / 1000, Grammar: "llama_sampler_init_grammar", Context: uint32(*contextTokens)})
}

func emit(v result) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
