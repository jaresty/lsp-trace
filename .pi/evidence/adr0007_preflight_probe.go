package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"lsp-trace/internal/describeworker"
)

type hostConfig struct {
	Continuation struct {
		Worker struct {
			Worker            describeworker.FilePin `json:"worker"`
			Model             describeworker.FilePin `json:"model"`
			Library           describeworker.FilePin `json:"library"`
			SandboxExecutable describeworker.FilePin `json:"sandbox_executable"`
			SandboxProfile    describeworker.FilePin `json:"sandbox_profile"`
			Grammar           describeworker.FilePin `json:"grammar"`
			RuntimeIdentity   string                 `json:"runtime_identity"`
			AdapterIdentity   string                 `json:"adapter_identity"`
			ModelIdentity     string                 `json:"model_identity"`
			Limits            describeworker.Limits  `json:"limits"`
		} `json:"worker"`
	} `json:"continuation"`
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: preflight HOST_CONFIG")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var h hostConfig
	if err := json.Unmarshal(raw, &h); err != nil {
		panic(err)
	}
	w := h.Continuation.Worker
	cfg := describeworker.Config{Worker: w.Worker, Model: w.Model, Library: w.Library, SandboxExecutable: w.SandboxExecutable, SandboxProfile: w.SandboxProfile, Grammar: w.Grammar, RuntimeIdentity: w.RuntimeIdentity, AdapterIdentity: w.AdapterIdentity, ModelIdentity: w.ModelIdentity, Limits: w.Limits}
	if _, err := describeworker.Preflight(cfg); err != nil {
		var f *describeworker.Failure
		if describeworker.AsFailure(err, &f) {
			fmt.Printf("PREFLIGHT_FAILED stage=%s code=%s subcode=%s\n", f.Stage(), f.Code(), f.Subcode())
			os.Exit(1)
		}
		panic(err)
	}
	for _, p := range []struct {
		name string
		pin  describeworker.FilePin
	}{{"WORKER", cfg.Worker}, {"MODEL", cfg.Model}, {"LIBRARY", cfg.Library}, {"SANDBOX_EXECUTABLE", cfg.SandboxExecutable}, {"SANDBOX_PROFILE", cfg.SandboxProfile}, {"GRAMMAR", cfg.Grammar}} {
		b, err := os.ReadFile(p.pin.Path)
		if err != nil {
			panic(err)
		}
		s := sha256.Sum256(b)
		fmt.Printf("PIN_OK name=%s sha256=sha256:%s\n", p.name, hex.EncodeToString(s[:]))
	}
	fmt.Println("PREFLIGHT_OK")
}
