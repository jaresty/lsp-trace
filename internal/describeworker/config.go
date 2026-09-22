package describeworker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Stage string
type Code string

const (
	StagePreflight       Stage = "PREFLIGHT"
	StageStart           Stage = "START"
	StageRun             Stage = "RUN"
	StageOutput          Stage = "OUTPUT"
	StageRecord          Stage = "RECORD"
	CodeCancelled        Code  = "CANCELLED"
	CodeTimeout          Code  = "TIMEOUT"
	CodeResourceLimit    Code  = "RESOURCE_LIMIT"
	CodeBackendFailure   Code  = "BACKEND_FAILURE"
	CodeOutputInvalid    Code  = "OUTPUT_INVALID"
	CodeModelUnavailable Code  = "MODEL_UNAVAILABLE"
	CodePolicyMismatch   Code  = "POLICY_MISMATCH"
)

type Failure struct {
	stage   Stage
	code    Code
	subcode string
}

func (f *Failure) Error() string              { return "describeworker: " + string(f.stage) + " " + string(f.code) }
func (f *Failure) Stage() Stage               { return f.stage }
func (f *Failure) Code() Code                 { return f.code }
func (f *Failure) Subcode() string            { return f.subcode }
func AsFailure(err error, out **Failure) bool { return errors.As(err, out) }
func failure(stage Stage, code Code) error    { return &Failure{stage: stage, code: code} }
func failureWithSubcode(stage Stage, code Code, subcode string) error {
	return &Failure{stage: stage, code: code, subcode: subcode}
}
func backendFailure(subcode string) error {
	return &Failure{stage: StageRun, code: CodeBackendFailure, subcode: subcode}
}

type FilePin struct {
	Path   string
	SHA256 string
}
type Config struct {
	Worker            FilePin
	Model             FilePin
	Library           FilePin
	SandboxExecutable FilePin
	SandboxProfile    FilePin
	Grammar           FilePin
	RuntimeIdentity   string
	AdapterIdentity   string
	ModelIdentity     string
	// ResponseVersion is additive: empty and V1 preserve the historical runner path.
	ResponseVersion string
	Limits          Limits
}
type ValidatedConfig struct{ config Config }

func (v ValidatedConfig) Config() Config { return v.config }

func Preflight(c Config) (ValidatedConfig, error) {
	if runtime.GOOS != "darwin" {
		return ValidatedConfig{}, failure(StagePreflight, CodePolicyMismatch)
	}
	if c.RuntimeIdentity == "" || c.AdapterIdentity == "" || c.ModelIdentity == "" || validateLimits(c.Limits) != nil {
		return ValidatedConfig{}, failure(StagePreflight, CodePolicyMismatch)
	}
	if c.ResponseVersion != "" && c.ResponseVersion != ResponseVersionV1 && c.ResponseVersion != ResponseVersionV2 {
		return ValidatedConfig{}, failure(StagePreflight, CodePolicyMismatch)
	}
	if err := verifyConfigPins(c); err != nil {
		return ValidatedConfig{}, failureWithSubcode(StagePreflight, CodeModelUnavailable, err.Error())
	}
	profile, err := os.ReadFile(c.SandboxProfile.Path)
	if err != nil || int64(len(profile)) > int64(c.Limits.WorkBytes) || !denyNetworkProfile(string(profile)) {
		return ValidatedConfig{}, failure(StagePreflight, CodePolicyMismatch)
	}
	return ValidatedConfig{config: c}, nil
}
func verifyConfigPins(c Config) error {
	pins := []struct {
		name string
		p    FilePin
		exec bool
	}{
		{"WORKER", c.Worker, true},
		{"MODEL", c.Model, false},
		{"LIBRARY", c.Library, false},
		{"SANDBOX_EXECUTABLE", c.SandboxExecutable, true},
		{"SANDBOX_PROFILE", c.SandboxProfile, false},
		{"GRAMMAR", c.Grammar, false},
	}
	for _, x := range pins {
		if err := verifyPin(x.name, x.p, x.exec); err != nil {
			return err
		}
	}
	return nil
}

func verifyPin(name string, p FilePin, executable bool) error {
	if p.Path == "" || !filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path {
		return errors.New(name + "_PIN_PATH_INVALID")
	}
	if !digestPattern.MatchString(p.SHA256) {
		return errors.New(name + "_PIN_DIGEST_INVALID")
	}
	info, err := os.Lstat(p.Path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New(name + "_PIN_FILE_MISSING")
	}
	if err != nil {
		return errors.New(name + "_PIN_STAT_FAILED")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New(name + "_PIN_SYMLINK")
	}
	if !info.Mode().IsRegular() {
		return errors.New(name + "_PIN_NOT_REGULAR")
	}
	if executable && info.Mode()&0111 == 0 {
		return errors.New(name + "_PIN_NOT_EXECUTABLE")
	}
	f, err := os.Open(p.Path)
	if err != nil {
		return errors.New(name + "_PIN_READ_FAILED")
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return errors.New(name + "_PIN_READ_FAILED")
	}
	if "sha256:"+hex.EncodeToString(h.Sum(nil)) != p.SHA256 {
		return errors.New(name + "_PIN_DIGEST_MISMATCH")
	}
	return nil
}
func denyNetworkProfile(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "deny network") || strings.Contains(s, "(deny (network")
}
