package continuationhost

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
)

func pinnedFile(t *testing.T, dir, name, contents string, mode os.FileMode) describeworker.FilePin {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	return describeworker.FilePin{Path: path, SHA256: Digest([]byte(contents))}
}

func validHostConfig(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return Config{
		Root:           root,
		MaxObjectBytes: 1 << 20,
		Worker: describeworker.Config{
			Worker:            pinnedFile(t, dir, "worker", "worker", 0o700),
			Model:             pinnedFile(t, dir, "model", "model", 0o600),
			Library:           pinnedFile(t, dir, "library", "library", 0o600),
			SandboxExecutable: pinnedFile(t, dir, "sandbox", "sandbox", 0o700),
			SandboxProfile:    pinnedFile(t, dir, "sandbox.sb", "(version 1)\n(deny default)\n(deny network*)", 0o600),
			Grammar:           pinnedFile(t, dir, "grammar", "grammar", 0o600),
			RuntimeIdentity:   "runtime-v1", AdapterIdentity: "adapter-v1", ModelIdentity: "model-v1",
			Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 4096, StderrBytes: 4096, WorkBytes: 1 << 20, TempBytes: 1 << 20},
		},
	}
}

func TestFactoryUsesExactHostPinsAndNoRequestOverrides(t *testing.T) {
	config := validHostConfig(t)
	var captured describeworker.Config
	fake := func(got describeworker.Config) (*describeworker.Runner, error) { captured = got; return nil, nil }
	bundle, err := New(config, Primitives{NewRunner: fake})
	if runtime.GOOS != "darwin" {
		var failure *Failure
		if err == nil || !errors.As(err, &failure) || failure.Code() != CodePolicyMismatch {
			t.Fatalf("ASSERT_UNSUPPORTED_SANDBOX_FAILS_CLOSED err=%v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Store == nil {
		t.Fatal("ASSERT_FACTORY_STORE")
	}
	if !reflect.DeepEqual(captured, config.Worker) {
		t.Fatalf("ASSERT_EXACT_PINNED_RUNNER\n got=%#v\nwant=%#v", captured, config.Worker)
	}
	request := RequestOptions{WorkerPath: "/tmp/attacker", ModelPath: "/tmp/attacker", SandboxPath: "/tmp/attacker"}
	if err := bundle.ValidateRequestOptions(request); err == nil {
		t.Fatal("ASSERT_REQUEST_CANNOT_OVERRIDE_PINS")
	}
}

func TestFactoryAbsentAndMalformedConfigurationAreTypedAndSourceSafe(t *testing.T) {
	for name, config := range map[string]Config{"absent": {}, "malformed": {MaxObjectBytes: -1}} {
		t.Run(name, func(t *testing.T) {
			_, err := New(config, Primitives{})
			var failure *Failure
			if err == nil || !errors.As(err, &failure) || failure.Stage() != StagePreflight {
				t.Fatalf("ASSERT_TYPED_OPT_IN_FAILURE err=%v", err)
			}
			if text := err.Error(); text == "" || filepath.IsAbs(text) || containsHostPath(text) {
				t.Fatalf("ASSERT_SOURCE_SAFE_FAILURE %q", text)
			}
		})
	}
}

func containsHostPath(s string) bool {
	return filepath.IsAbs(s) || len(s) > 0 && (s[0] == '/' || s[0] == '\\')
}
