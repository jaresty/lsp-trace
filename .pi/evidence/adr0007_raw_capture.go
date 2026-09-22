package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
)

const requestID = "c3d9892a7cbbda802f9feea1dd5299e382c6854c9733bb272a8c6fd0820bedb8"
const checkpointSelector = "sha256:c8635b72e56fa5a7636eaa09d3c607863a6de4d2a4d6548303ca17007fc317b9"

type hostConfig struct {
	Continuation struct {
		PublicationRoot string `json:"publication_root"`
		MaxObjectBytes  int64  `json:"max_object_bytes"`
		Worker          struct {
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

func digestFile(path string) (string, int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:]), int64(len(b)), nil
}

func main() {
	if len(os.Args) != 5 {
		panic("usage: capture HOST_CONFIG STDOUT STDERR LEDGER")
	}
	configRaw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var host hostConfig
	if err := json.Unmarshal(configRaw, &host); err != nil {
		panic(err)
	}
	w := host.Continuation.Worker
	cfg := describeworker.Config{Worker: w.Worker, Model: w.Model, Library: w.Library, SandboxExecutable: w.SandboxExecutable, SandboxProfile: w.SandboxProfile, Grammar: w.Grammar, RuntimeIdentity: w.RuntimeIdentity, AdapterIdentity: w.AdapterIdentity, ModelIdentity: w.ModelIdentity, Limits: w.Limits}
	if _, err := describeworker.Preflight(cfg); err != nil {
		panic(err)
	}
	root, err := publication.OpenRoot(host.Continuation.PublicationRoot)
	if err != nil {
		panic(err)
	}
	defer root.Close()
	store, err := continuationhost.NewStore(root, host.Continuation.MaxObjectBytes)
	if err != nil {
		panic(err)
	}
	chain, err := censuscontinuation.VerifyChain(context.Background(), store, checkpointSelector)
	if err != nil {
		panic(err)
	}
	var requestsSelector string
	for _, cp := range chain {
		for _, ref := range cp.Artifacts() {
			if ref.Kind == "requests" {
				requestsSelector = ref.ID
			}
		}
	}
	if requestsSelector == "" {
		panic("requests artifact unavailable")
	}
	requestRaw, err := store.Get(context.Background(), requestsSelector)
	if err != nil {
		panic(err)
	}
	requests, err := describerequest.Parse(requestRaw)
	if err != nil {
		panic(err)
	}
	var request describerequest.Record
	for _, candidate := range requests {
		if candidate.RecordID == requestID {
			request = candidate
			break
		}
	}
	if request.RecordID != requestID {
		panic("authorized request unavailable")
	}
	tempDir, err := os.MkdirTemp("", "lsp-trace-diagnostic-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)
	if err := os.Chmod(tempDir, 0700); err != nil {
		panic(err)
	}
	promptPath := filepath.Join(tempDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte(request.Envelope.Prompt), 0600); err != nil {
		panic(err)
	}
	stdout, err := os.OpenFile(os.Args[2], os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	stderr, err := os.OpenFile(os.Args[3], os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Limits.TimeoutMS)*time.Millisecond)
	defer cancel()
	args := []string{"-f", cfg.SandboxProfile.Path, cfg.Worker.Path, "-model", cfg.Model.Path, "-lib", filepath.Dir(cfg.Library.Path), "-prompt-file", promptPath, "-timeout", (time.Duration(cfg.Limits.TimeoutMS) * time.Millisecond).String(), "-max-tokens", strconv.Itoa(cfg.Limits.MaxTokens), "-context-tokens", strconv.Itoa(cfg.Limits.ContextTokens)}
	cmd := exec.CommandContext(ctx, cfg.SandboxExecutable.Path, args...)
	cmd.Dir = tempDir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + tempDir, "TMPDIR=" + tempDir, "LANG=C", "LC_ALL=C"}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)
	if err := stdout.Close(); err != nil {
		panic(err)
	}
	if err := stderr.Close(); err != nil {
		panic(err)
	}
	for _, path := range os.Args[2:5] {
		if err := os.Chmod(path, 0600); err != nil {
			panic(err)
		}
	}
	stdoutDigest, stdoutLen, err := digestFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	stderrDigest, stderrLen, err := digestFile(os.Args[3])
	if err != nil {
		panic(err)
	}
	exit := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exit = ee.ExitCode()
		} else {
			exit = -1
		}
	}
	ledger := fmt.Sprintf("retention=SESSION\nrequest_id=%s\nstdout_sha256=%s\nstdout_bytes=%d\nstderr_sha256=%s\nstderr_bytes=%d\nelapsed_ns=%d\nexit_code=%d\n", requestID, stdoutDigest, stdoutLen, stderrDigest, stderrLen, elapsed.Nanoseconds(), exit)
	if err := os.WriteFile(os.Args[4], []byte(ledger), 0600); err != nil {
		panic(err)
	}
	fmt.Printf("RAW_CAPTURE_COMPLETE request=%s stdout_sha256=%s stdout_bytes=%d stderr_sha256=%s stderr_bytes=%d elapsed_ns=%d exit_code=%d retention=SESSION\n", requestID, stdoutDigest, stdoutLen, stderrDigest, stderrLen, elapsed.Nanoseconds(), exit)
}
