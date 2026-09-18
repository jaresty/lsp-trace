package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type request struct {
	Protocol        string `json:"protocol"`
	MessageType     string `json:"message_type"`
	MessageID       string `json:"message_id"`
	Correlation     string `json:"correlation_id"`
	AdmissionID     string `json:"admission_id"`
	Corpus          string `json:"corpus"`
	ItemType        string `json:"item_type"`
	AcquisitionMode string `json:"acquisition_mode"`
	InputSHA256     string `json:"input_sha256"`
	InputBytes      int    `json:"input_bytes"`
	SourceRevision  string `json:"source_revision"`
	DeadlineMS      int    `json:"deadline_ms"`
	TargetMessageID string `json:"target_message_id"`
	Prompt          string `json:"prompt"`
}

type response struct {
	Protocol    string          `json:"protocol"`
	MessageType string          `json:"message_type"`
	MessageID   string          `json:"message_id"`
	Correlation string          `json:"correlation_id"`
	Status      string          `json:"status"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type workerResult struct {
	Status string `json:"status"`
}

func main() {
	worker, model, lib := os.Getenv("ADR0007_WORKER"), os.Getenv("ADR0007_MODEL"), os.Getenv("ADR0007_LIB")
	if worker == "" || model == "" || lib == "" {
		fatal("ADR0007_WORKER, ADR0007_MODEL, and ADR0007_LIB are required")
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var outMu sync.Mutex
	var mu sync.Mutex
	var wg sync.WaitGroup
	active := map[string]context.CancelFunc{}
	seen := map[string]bool{}
	write := func(resp response) { outMu.Lock(); defer outMu.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(resp) }
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			write(response{MessageType: "error", Status: "INVALID_INPUT", Error: err.Error()})
			continue
		}
		if req.MessageType == "cancel" {
			mu.Lock()
			cancel, ok := active[req.TargetMessageID]
			mu.Unlock()
			if !ok {
				write(response{Protocol: req.Protocol, MessageType: "error", MessageID: req.MessageID, Correlation: req.Correlation, Status: "INVALID_INPUT", Error: "unknown active message"})
			} else {
				cancel()
				write(response{Protocol: req.Protocol, MessageType: "response", MessageID: req.MessageID, Correlation: req.Correlation, Status: "CANCELLED"})
			}
			continue
		}
		if err := validate(req); err != nil {
			write(response{Protocol: req.Protocol, MessageType: "error", MessageID: req.MessageID, Correlation: req.Correlation, Status: "POLICY_MISMATCH", Error: err.Error()})
			continue
		}
		mu.Lock()
		if seen[req.MessageID] {
			mu.Unlock()
			write(response{Protocol: req.Protocol, MessageType: "error", MessageID: req.MessageID, Correlation: req.Correlation, Status: "DUPLICATE_INPUT", Error: "duplicate message_id"})
			continue
		}
		seen[req.MessageID] = true
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.DeadlineMS)*time.Millisecond)
		active[req.MessageID] = cancel
		mu.Unlock()
		wg.Add(1)
		go func(req request, ctx context.Context, cancel context.CancelFunc) {
			defer wg.Done()
			defer cancel()
			defer func() { mu.Lock(); delete(active, req.MessageID); mu.Unlock() }()
			payload, status, err := runWorker(ctx, worker, model, lib, req.Prompt)
			resp := response{Protocol: req.Protocol, MessageType: "response", MessageID: req.MessageID, Correlation: req.Correlation, Status: status, Payload: payload}
			if err != nil {
				resp.Error = err.Error()
			}
			write(resp)
		}(req, ctx, cancel)
	}
	if err := scanner.Err(); err != nil {
		fatal(err.Error())
	}
	wg.Wait()
}

func validate(req request) error {
	if req.Protocol != "adr0007-semantic-worker" || req.MessageType != "request" || req.MessageID == "" || req.Correlation == "" {
		return errors.New("invalid request envelope")
	}
	if req.AdmissionID == "" || req.Corpus == "" || req.ItemType == "" || req.SourceRevision == "" {
		return errors.New("missing admission or source identity")
	}
	if req.AcquisitionMode != "TARGET" {
		return errors.New("only TARGET acquisition is admitted")
	}
	if req.DeadlineMS < 1 || req.DeadlineMS > 90000 {
		return errors.New("deadline_ms must be between 1 and 90000")
	}
	if req.Prompt == "" || len([]byte(req.Prompt)) > 1<<20 {
		return errors.New("prompt is empty or exceeds 1 MiB")
	}
	digest := sha256.Sum256([]byte(req.Prompt))
	if req.InputBytes != len([]byte(req.Prompt)) || !strings.EqualFold(req.InputSHA256, hex.EncodeToString(digest[:])) {
		return errors.New("input digest or byte length mismatch")
	}
	return nil
}

func runWorker(ctx context.Context, worker, model, lib, prompt string) (json.RawMessage, string, error) {
	tmp, err := os.CreateTemp("", "adr0007-prompt-*.txt")
	if err != nil {
		return nil, "BACKEND_FAILURE", err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = io.WriteString(tmp, prompt); err != nil {
		return nil, "BACKEND_FAILURE", err
	}
	if err = tmp.Close(); err != nil {
		return nil, "BACKEND_FAILURE", err
	}

	cmd := exec.Command(filepath.Clean(worker), "-model", filepath.Clean(model), "-lib", filepath.Clean(lib), "-prompt-file", name, "-timeout", "90s", "-max-tokens", "384", "-context-tokens", "16384")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		return nil, "BACKEND_FAILURE", err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		waitErr = <-waitCh
		return nil, "TIMEOUT", ctx.Err()
	}
	if waitErr != nil {
		return nil, "BACKEND_FAILURE", err
	}
	out := stdout.Bytes()
	var result workerResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, "OUTPUT_INVALID", err
	}
	status := result.Status
	if status == "" {
		status = "OUTPUT_INVALID"
	}
	return json.RawMessage(out), status, nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
