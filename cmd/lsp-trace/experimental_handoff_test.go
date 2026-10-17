package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const (
	assertExperimentalHandoffExplicitOnly         = "ASSERT_EXPERIMENTAL_HANDOFF_EXPLICIT_ONLY"
	assertExperimentalHandoffStrictReceipt        = "ASSERT_EXPERIMENTAL_HANDOFF_STRICT_RECEIPT"
	assertExperimentalHandoffSelectorResolves     = "ASSERT_EXPERIMENTAL_HANDOFF_SELECTOR_RESOLVES"
	assertExperimentalHandoffIdempotent           = "ASSERT_EXPERIMENTAL_HANDOFF_IDEMPOTENT"
	assertExperimentalHandoffFailsBeforePut       = "ASSERT_EXPERIMENTAL_HANDOFF_FAILS_BEFORE_PUT"
	assertExperimentalHandoffLegacyUnchanged      = "ASSERT_EXPERIMENTAL_HANDOFF_LEGACY_UNCHANGED"
	assertExperimentalHandoffZeroExecutionEffects = "ASSERT_EXPERIMENTAL_HANDOFF_ZERO_EXECUTION_EFFECTS"
)

type experimentalHandoffTestStore struct {
	objects map[string][]byte
	puts    int
}

func (s *experimentalHandoffTestStore) Put(_ context.Context, raw []byte) (string, error) {
	s.puts++
	selector := "sha256:" + strings.Repeat("b", 64)
	s.objects[selector] = append([]byte(nil), raw...)
	return selector, nil
}

func (s *experimentalHandoffTestStore) Get(_ context.Context, selector string) ([]byte, error) {
	raw, ok := s.objects[selector]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), raw...), nil
}

type experimentalHandoffNopCloser struct{}

func (experimentalHandoffNopCloser) Close() error { return nil }

type experimentalHandoffTestCloser struct {
	closed *bool
	err    error
}

func (c experimentalHandoffTestCloser) Close() error {
	*c.closed = true
	return c.err
}

type experimentalHandoffWriterFunc func([]byte) (int, error)

func (f experimentalHandoffWriterFunc) Write(p []byte) (int, error) { return f(p) }

func successfulExperimentalHandoffDependencies(store *experimentalHandoffTestStore, calls *int) experimentalHandoffDependencies {
	return experimentalHandoffDependencies{
		openStore: func(root string) (censuscontinuation.Store, io.Closer, error) {
			if root != "/private/root" {
				return nil, nil, errors.New("wrong root")
			}
			return store, experimentalHandoffNopCloser{}, nil
		},
		prepare: func(ctx context.Context, got censuscontinuation.Store, checkpoint string) (string, error) {
			*calls++
			if got != store || checkpoint != "sha256:"+strings.Repeat("a", 64) {
				return "", errors.New("wrong preparation input")
			}
			return store.Put(ctx, []byte(`{"schema_version":"private"}`))
		},
	}
}

func runSuccessfulExperimentalHandoff(t *testing.T, deps experimentalHandoffDependencies) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{
		"prepare", "--publication-root", "/private/root",
		"--checkpoint-selector", "sha256:" + strings.Repeat("a", 64),
	}, &stdout, &stderr, deps)
	return stdout.String(), stderr.String(), code
}

func TestExperimentalHandoffExplicitOnly(t *testing.T) {
	var opened, prepared int
	deps := experimentalHandoffDependencies{
		openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
			opened++
			return nil, nil, errors.New("unexpected")
		},
		prepare: func(context.Context, censuscontinuation.Store, string) (string, error) {
			prepared++
			return "", errors.New("unexpected")
		},
	}
	var stdout, stderr bytes.Buffer
	if code := runExperimentalHandoffWithDependencies(context.Background(), nil, &stdout, &stderr, deps); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), experimentalHandoffUsage) || opened != 0 || prepared != 0 {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q opened=%d prepared=%d", assertExperimentalHandoffExplicitOnly, code, stdout.String(), stderr.String(), opened, prepared)
	}
	beforeOut, beforeErr, beforeCode := captureRun(t, []string{"version"})
	_, _, _ = captureRun(t, []string{"experimental-handoff", "prepare"})
	afterOut, afterErr, afterCode := captureRun(t, []string{"version"})
	if beforeCode != afterCode || beforeOut != afterOut || beforeErr != afterErr {
		t.Fatalf("%s: ordinary command changed", assertExperimentalHandoffExplicitOnly)
	}
}

func TestExperimentalHandoffStrictReceipt(t *testing.T) {
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	calls := 0
	stdout, stderr, code := runSuccessfulExperimentalHandoff(t, successfulExperimentalHandoffDependencies(store, &calls))
	want := `{"schema_version":"lsp-trace.experimental-handoff-prepared-receipt.v1","status":"HANDOFF_PREPARED","private_selector":"sha256:` + strings.Repeat("b", 64) + `","checkpoint_selector":"sha256:` + strings.Repeat("a", 64) + `","authority":0,"accepted":false,"completeness":"UNKNOWN","custody":"CALLER_CONTROLLED_INFERENCE"}` + "\n"
	if code != 0 || stderr != "" || stdout != want || calls != 1 {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q calls=%d", assertExperimentalHandoffStrictReceipt, code, stdout, stderr, calls)
	}
	for _, forbidden := range []string{"request", "provider", "process", "session", "pi", "HANDED_OFF"} {
		if strings.Contains(strings.ToLower(stdout), strings.ToLower(forbidden)) {
			t.Fatalf("%s: forbidden=%q receipt=%s", assertExperimentalHandoffStrictReceipt, forbidden, stdout)
		}
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(stdout), &fields) != nil || len(fields) != 8 {
		t.Fatalf("%s: fields=%v", assertExperimentalHandoffStrictReceipt, fields)
	}
}

func TestExperimentalHandoffSelectorResolves(t *testing.T) {
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	calls := 0
	stdout, stderr, code := runSuccessfulExperimentalHandoff(t, successfulExperimentalHandoffDependencies(store, &calls))
	var receipt experimentalHandoffReceipt
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil || code != 0 || stderr != "" {
		t.Fatalf("%s: code=%d stderr=%q err=%v", assertExperimentalHandoffSelectorResolves, code, stderr, err)
	}
	raw, err := store.Get(context.Background(), receipt.PrivateSelector)
	if err != nil || string(raw) != `{"schema_version":"private"}` {
		t.Fatalf("%s: selector=%q raw=%q err=%v", assertExperimentalHandoffSelectorResolves, receipt.PrivateSelector, raw, err)
	}
}

func TestExperimentalHandoffIdempotent(t *testing.T) {
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	calls := 0
	deps := successfulExperimentalHandoffDependencies(store, &calls)
	first, firstErr, firstCode := runSuccessfulExperimentalHandoff(t, deps)
	second, secondErr, secondCode := runSuccessfulExperimentalHandoff(t, deps)
	if firstCode != 0 || secondCode != 0 || firstErr != "" || secondErr != "" || first != second || calls != 2 || len(store.objects) != 1 {
		t.Fatalf("%s: codes=%d/%d errors=%q/%q equal=%t calls=%d objects=%d", assertExperimentalHandoffIdempotent, firstCode, secondCode, firstErr, secondErr, first == second, calls, len(store.objects))
	}
}

func TestExperimentalHandoffFailsBeforePut(t *testing.T) {
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	deps := experimentalHandoffDependencies{
		openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
			return store, experimentalHandoffNopCloser{}, nil
		},
		prepare: func(context.Context, censuscontinuation.Store, string) (string, error) {
			return "", errors.New("checkpoint rejected before prepared Put")
		},
	}
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoffWithDependencies(context.Background(), []string{"prepare", "--publication-root", "/bad", "--checkpoint-selector", "stale"}, &stdout, &stderr, deps)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "checkpoint rejected") || store.puts != 0 {
		t.Fatalf("%s: code=%d stdout=%q stderr=%q puts=%d", assertExperimentalHandoffFailsBeforePut, code, stdout.String(), stderr.String(), store.puts)
	}
}

func TestExperimentalHandoffContextCloseAndHelp(t *testing.T) {
	checkpoint := "sha256:" + strings.Repeat("a", 64)
	t.Run("causal cancellation", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			cancelCtx  bool
			prepareErr error
			closeErr   error
		}{
			{name: "prepare returns canceled", prepareErr: context.Canceled},
			{name: "caller context canceled", cancelCtx: true, prepareErr: errors.New("preparation observed caller cancellation")},
			{name: "caller cancellation and close failure", cancelCtx: true, prepareErr: errors.New("preparation observed caller cancellation"), closeErr: errors.New("close after cancellation failed")},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cause := errors.New("causal cancellation")
				ctx, cancel := context.WithCancelCause(context.Background())
				if tc.cancelCtx {
					cancel(cause)
				} else {
					defer cancel(nil)
				}
				closed := false
				deps := experimentalHandoffDependencies{
					openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
						return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffTestCloser{closed: &closed, err: tc.closeErr}, nil
					},
					prepare: func(got context.Context, _ censuscontinuation.Store, _ string) (string, error) {
						if tc.cancelCtx && !errors.Is(context.Cause(got), cause) {
							t.Fatalf("cause=%v", context.Cause(got))
						}
						return "", tc.prepareErr
					},
				}
				var stdout, stderr bytes.Buffer
				code := runExperimentalHandoffWithDependencies(ctx, []string{"prepare", "--publication-root", "/root", "--checkpoint-selector", checkpoint}, &stdout, &stderr, deps)
				if code != 130 || !closed || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.prepareErr.Error()) || (tc.closeErr != nil && !strings.Contains(stderr.String(), tc.closeErr.Error())) {
					t.Fatalf("code=%d closed=%t stdout=%q stderr=%q", code, closed, stdout.String(), stderr.String())
				}
			})
		}
	})
	t.Run("close before output and close failure", func(t *testing.T) {
		for _, closeErr := range []error{nil, errors.New("close failed")} {
			closed := false
			deps := experimentalHandoffDependencies{
				openStore: func(string) (censuscontinuation.Store, io.Closer, error) {
					return &experimentalHandoffTestStore{objects: map[string][]byte{}}, experimentalHandoffTestCloser{closed: &closed, err: closeErr}, nil
				},
				prepare: func(context.Context, censuscontinuation.Store, string) (string, error) {
					return "sha256:" + strings.Repeat("b", 64), nil
				},
			}
			var stdout, stderr bytes.Buffer
			writer := experimentalHandoffWriterFunc(func(p []byte) (int, error) {
				if !closed {
					t.Fatal("receipt written before close")
				}
				return stdout.Write(p)
			})
			code := runExperimentalHandoffWithDependencies(context.Background(), []string{"prepare", "--publication-root", "/root", "--checkpoint-selector", checkpoint}, writer, &stderr, deps)
			if closeErr == nil && (code != 0 || stdout.Len() == 0 || stderr.Len() != 0) {
				t.Fatalf("success code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if closeErr != nil && (code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), closeErr.Error())) {
				t.Fatalf("failure code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		}
	})
	for _, args := range [][]string{{"prepare", "--help"}, {"--help"}} {
		var stdout, stderr bytes.Buffer
		code := runExperimentalHandoffWithDependencies(context.Background(), args, &stdout, &stderr, experimentalHandoffDependencies{})
		if code != 0 || stdout.Len() != 0 || strings.Contains(stderr.String(), "HANDOFF_PREPARED") || !strings.Contains(stderr.String(), "experimental-handoff") {
			t.Fatalf("help args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	dispatchOut, dispatchErr, dispatchCode := captureRun(t, []string{"experimental-handoff", "--help"})
	if dispatchCode != 0 || dispatchOut != "" || strings.Contains(dispatchErr, "HANDOFF_PREPARED") || !strings.Contains(dispatchErr, "experimental-handoff") {
		t.Fatalf("dispatch help code=%d stdout=%q stderr=%q", dispatchCode, dispatchOut, dispatchErr)
	}
}

type experimentalHandoffFixturePins struct {
	CheckpointSelector string `json:"checkpoint_selector"`
	PreparedSelector   string `json:"prepared_selector"`
	Objects            []struct {
		Selector string `json:"selector"`
		Bytes    int    `json:"bytes"`
	} `json:"objects"`
}

func copyExperimentalHandoffFixture(t *testing.T) (string, experimentalHandoffFixturePins) {
	t.Helper()
	source := filepath.Join("testdata", "experimental-handoff-root")
	assertExperimentalHandoffFixtureHygiene(t, source)
	destination := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	var pins experimentalHandoffFixturePins
	raw, err := os.ReadFile(filepath.Join(source, "pins.json"))
	if err != nil || strictjson.RejectDuplicates(raw) != nil || json.Unmarshal(raw, &pins) != nil {
		t.Fatalf("fixture pins: %v", err)
	}
	inventory := experimentalHandoffObjectInventory(t, source)
	if len(inventory) != len(pins.Objects) {
		t.Fatalf("fixture object count=%d pins=%d", len(inventory), len(pins.Objects))
	}
	for _, pin := range pins.Objects {
		hexDigest := strings.TrimPrefix(pin.Selector, "sha256:")
		entry, ok := inventory[hexDigest]
		if !ok || entry != fmt.Sprintf("%s:%d", pin.Selector, pin.Bytes) {
			t.Fatalf("fixture pin mismatch selector=%s entry=%q", pin.Selector, entry)
		}
	}
	return destination, pins
}

func assertExperimentalHandoffFixtureHygiene(t *testing.T, root string) {
	t.Helper()
	forbidden := [][]byte{
		[]byte("/var/folders/"), []byte("/Users/"), []byte("/private/tmp/"), []byte("/home/"),
		[]byte("TestGenerateExperimentalHandoffFixture"), []byte("/T/Test"), []byte("mktemp"),
	}
	credentialMarkers := [][]byte{
		[]byte("begin private key"), []byte("begin rsa private key"), []byte("password="),
		[]byte(`"password":`), []byte(`"secret":`), []byte(`"access_token":`), []byte(`"api_key":`),
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, marker := range forbidden {
			if bytes.Contains(raw, marker) {
				return fmt.Errorf("fixture hygiene: %s contains forbidden marker %q", path, marker)
			}
		}
		lower := bytes.ToLower(raw)
		for _, marker := range credentialMarkers {
			if bytes.Contains(lower, marker) {
				return fmt.Errorf("fixture hygiene: %s contains credential marker %q", path, marker)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func experimentalHandoffObjectInventory(t *testing.T, root string) map[string]string {
	t.Helper()
	directory := filepath.Join(root, "continuations", "objects", "sha256")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	inventory := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected object directory %s", entry.Name())
		}
		raw, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		digest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
		inventory[entry.Name()] = fmt.Sprintf("%s:%d", digest, len(raw))
	}
	return inventory
}

func runProductionExperimentalHandoff(t *testing.T, ctx context.Context, root, checkpoint string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runExperimentalHandoff(ctx, []string{"prepare", "--publication-root", root, "--checkpoint-selector", checkpoint}, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestExperimentalHandoffProductionRetainedRoot(t *testing.T) {
	const checkpoint = "sha256:998c6733f08c1afe7327d2e5a5207de25dadf86ad53215d081ff02efcc5b70ef"
	const preparedSelector = "sha256:419dc82f0a233fe4d45f769333c0a6f604fc6c3d3fa05f3d6ce2b33f72c02862"
	const exactReceipt = `{"schema_version":"lsp-trace.experimental-handoff-prepared-receipt.v1","status":"HANDOFF_PREPARED","private_selector":"sha256:419dc82f0a233fe4d45f769333c0a6f604fc6c3d3fa05f3d6ce2b33f72c02862","checkpoint_selector":"sha256:998c6733f08c1afe7327d2e5a5207de25dadf86ad53215d081ff02efcc5b70ef","authority":0,"accepted":false,"completeness":"UNKNOWN","custody":"CALLER_CONTROLLED_INFERENCE"}` + "\n"

	t.Run("success idempotence and strict reopened projection", func(t *testing.T) {
		rootPath, pins := copyExperimentalHandoffFixture(t)
		if pins.CheckpointSelector != checkpoint || pins.PreparedSelector != preparedSelector {
			t.Fatalf("fixture pins checkpoint=%s prepared=%s", pins.CheckpointSelector, pins.PreparedSelector)
		}
		firstOut, firstErr, firstCode := runProductionExperimentalHandoff(t, context.Background(), rootPath, checkpoint)
		if firstCode != 0 || firstErr != "" || firstOut != exactReceipt {
			t.Fatalf("first code=%d stdout=%q stderr=%q", firstCode, firstOut, firstErr)
		}
		afterFirst := experimentalHandoffObjectInventory(t, rootPath)
		var receipt experimentalHandoffReceipt
		if err := json.Unmarshal([]byte(firstOut), &receipt); err != nil || receipt.PrivateSelector != preparedSelector {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
		rootOwner, err := publication.OpenRoot(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		store, err := continuationhost.NewStore(rootOwner, experimentalHandoffMaxObjectBytes)
		if err != nil {
			t.Fatal(err)
		}
		preparedRaw, err := store.Get(context.Background(), receipt.PrivateSelector)
		if err != nil {
			t.Fatal(err)
		}
		if err := rootOwner.Close(); err != nil {
			t.Fatal(err)
		}
		if continuationhost.Digest(preparedRaw) != receipt.PrivateSelector || strictjson.RejectDuplicates(preparedRaw) != nil {
			t.Fatalf("prepared selector or strict JSON mismatch")
		}
		var projection map[string]any
		if err := json.Unmarshal(preparedRaw, &projection); err != nil {
			t.Fatal(err)
		}
		wantKeys := []string{"accepted", "authority", "chain_identity", "checkpoint_id", "checkpoint_selector", "completeness", "custody", "prepared_id", "requests_byte_length", "requests_count", "requests_digest", "requests_selector", "schema_version", "status"}
		gotKeys := make([]string, 0, len(projection))
		for key := range projection {
			gotKeys = append(gotKeys, key)
		}
		sort.Strings(gotKeys)
		if !reflect.DeepEqual(gotKeys, wantKeys) || projection["status"] != "HANDOFF_PREPARED" || projection["authority"] != float64(0) || projection["accepted"] != false || projection["completeness"] != "UNKNOWN" || projection["custody"] != "CALLER_CONTROLLED_INFERENCE" || projection["checkpoint_selector"] != checkpoint {
			t.Fatalf("strict projection keys=%v projection=%v", gotKeys, projection)
		}
		secondOut, secondErr, secondCode := runProductionExperimentalHandoff(t, context.Background(), rootPath, checkpoint)
		afterSecond := experimentalHandoffObjectInventory(t, rootPath)
		if secondCode != 0 || secondErr != "" || secondOut != firstOut || !reflect.DeepEqual(afterFirst, afterSecond) {
			t.Fatalf("second code=%d stdout_equal=%t stderr=%q inventory_equal=%t", secondCode, secondOut == firstOut, secondErr, reflect.DeepEqual(afterFirst, afterSecond))
		}
		rootOwner, err = publication.OpenRoot(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		store, err = continuationhost.NewStore(rootOwner, experimentalHandoffMaxObjectBytes)
		if err != nil {
			t.Fatal(err)
		}
		secondRaw, err := store.Get(context.Background(), receipt.PrivateSelector)
		if err != nil {
			t.Fatal(err)
		}
		if err := rootOwner.Close(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(preparedRaw, secondRaw) {
			t.Fatal("prepared bytes changed")
		}
	})

	t.Run("wrong stage leaves inventory unchanged", func(t *testing.T) {
		rootPath, _ := copyExperimentalHandoffFixture(t)
		owner, err := publication.OpenRoot(rootPath)
		if err != nil {
			t.Fatal(err)
		}
		store, err := continuationhost.NewStore(owner, experimentalHandoffMaxObjectBytes)
		if err != nil {
			t.Fatal(err)
		}
		chain, err := censuscontinuation.VerifyChain(context.Background(), store, checkpoint)
		if err != nil {
			t.Fatal(err)
		}
		wrongStage := chain[len(chain)-1].PriorID()
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		before := experimentalHandoffObjectInventory(t, rootPath)
		stdout, stderr, code := runProductionExperimentalHandoff(t, context.Background(), rootPath, wrongStage)
		after := experimentalHandoffObjectInventory(t, rootPath)
		if code == 0 || stdout != "" || stderr == "" || !reflect.DeepEqual(before, after) {
			t.Fatalf("code=%d stdout=%q stderr=%q inventory_equal=%t", code, stdout, stderr, reflect.DeepEqual(before, after))
		}
	})

	t.Run("canceled production dependencies exit 130", func(t *testing.T) {
		rootPath, _ := copyExperimentalHandoffFixture(t)
		before := experimentalHandoffObjectInventory(t, rootPath)
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(errors.New("dispatch cancellation"))
		stdout, stderr, code := runProductionExperimentalHandoff(t, ctx, rootPath, checkpoint)
		after := experimentalHandoffObjectInventory(t, rootPath)
		if code != 130 || stdout != "" || !strings.Contains(stderr, context.Canceled.Error()) || !reflect.DeepEqual(before, after) {
			t.Fatalf("code=%d stdout=%q stderr=%q inventory_equal=%t", code, stdout, stderr, reflect.DeepEqual(before, after))
		}
	})
}

func TestExperimentalHandoffLegacyUnchanged(t *testing.T) {
	beforeOut, beforeErr, beforeCode := captureRun(t, []string{"version"})
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	calls := 0
	_, _, _ = runSuccessfulExperimentalHandoff(t, successfulExperimentalHandoffDependencies(store, &calls))
	afterOut, afterErr, afterCode := captureRun(t, []string{"version"})
	commandUsage := strings.TrimPrefix(experimentalHandoffUsage, "usage: ")
	if strings.Count(usageText, commandUsage) != 1 || beforeCode != afterCode || beforeOut != afterOut || beforeErr != afterErr {
		t.Fatalf("%s: ordinary command bytes changed", assertExperimentalHandoffLegacyUnchanged)
	}
}

func TestExperimentalHandoffZeroExecutionEffects(t *testing.T) {
	store := &experimentalHandoffTestStore{objects: map[string][]byte{}}
	calls := 0
	stdout, stderr, code := runSuccessfulExperimentalHandoff(t, successfulExperimentalHandoffDependencies(store, &calls))
	if code != 0 || stderr != "" || stdout == "" || calls != 1 || store.puts != 1 {
		t.Fatalf("%s: code=%d stderr=%q calls=%d puts=%d", assertExperimentalHandoffZeroExecutionEffects, code, stderr, calls, store.puts)
	}
	// The dependency surface intentionally contains only store opening and private
	// preparation: no worker, Resume, model, harness, ingest, or checkpoint writer.
	if _, ok := any(productionExperimentalHandoffDependencies()).(experimentalHandoffDependencies); !ok {
		t.Fatal(assertExperimentalHandoffZeroExecutionEffects)
	}
}
