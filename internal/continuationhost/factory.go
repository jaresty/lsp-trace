package continuationhost

import (
	"errors"
	"runtime"

	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
)

type Stage string
type Code string

const (
	StagePreflight               Stage = "PREFLIGHT"
	CodeConfigurationUnavailable Code  = "CONFIGURATION_UNAVAILABLE"
	CodePolicyMismatch           Code  = "POLICY_MISMATCH"
)

type Failure struct {
	stage Stage
	code  Code
}

func (f *Failure) Error() string {
	return "continuationhost: " + string(f.stage) + " " + string(f.code)
}
func (f *Failure) Stage() Stage { return f.stage }
func (f *Failure) Code() Code   { return f.code }
func failure(code Code) error   { return &Failure{stage: StagePreflight, code: code} }

// Config is host-owned. It is never populated from a continuation request.
type Config struct {
	Root           *publication.Root
	MaxObjectBytes int64
	Worker         describeworker.Config
}

type Primitives struct {
	NewRunner func(describeworker.Config) (*describeworker.Runner, error)
}

type Bundle struct {
	Store  *Store
	Runner *describeworker.Runner
}

func New(config Config, primitives Primitives) (*Bundle, error) {
	if config.Root == nil || config.MaxObjectBytes < 1 {
		return nil, failure(CodeConfigurationUnavailable)
	}
	if runtime.GOOS != "darwin" {
		return nil, failure(CodePolicyMismatch)
	}
	store, err := NewStore(config.Root, config.MaxObjectBytes)
	if err != nil {
		return nil, failure(CodeConfigurationUnavailable)
	}
	newRunner := primitives.NewRunner
	if newRunner == nil {
		newRunner = describeworker.NewRunner
	}
	runner, err := newRunner(config.Worker)
	if err != nil {
		var workerFailure *describeworker.Failure
		if errors.As(err, &workerFailure) && workerFailure.Code() == describeworker.CodePolicyMismatch {
			return nil, failure(CodePolicyMismatch)
		}
		return nil, failure(CodeConfigurationUnavailable)
	}
	return &Bundle{Store: store, Runner: runner}, nil
}

// RequestOptions is deliberately override-free in production. Nonempty legacy or
// adversarial fields are rejected rather than merged with host authority.
type RequestOptions struct {
	WorkerPath  string
	ModelPath   string
	SandboxPath string
}

func (b *Bundle) ValidateRequestOptions(options RequestOptions) error {
	if options != (RequestOptions{}) {
		return failure(CodePolicyMismatch)
	}
	return nil
}
