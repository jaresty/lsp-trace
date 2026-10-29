//go:build darwin || linux

package censuslineagepilot_test

// Private test caller only. Configuration is data, not execution authority.
// No public CLI, MCP admission, callback factory or provenance mint is introduced.
import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/managedprocess"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

const variableCallerPurpose = "PRIVATE_VARIABLE_SCOPE_V2_PREPARATION_ONLY"
const variableCallerSchema = "private-variable-scope-v2-caller.v1"
const variableCallerConfigCap = 64 << 10
const variableCallerReportCap = 1 << 20

type variableCallerConfig struct {
	Schema             string                            `json:"schema"`
	Purpose            string                            `json:"purpose"`
	CodeRoot           string                            `json:"code_root"`
	CodeHEAD           string                            `json:"code_head"`
	Scope              pilot.ScopeSpecPrivateV2          `json:"scope"`
	Session            censusacquisition.SessionIdentity `json:"session"`
	Selector           runtimeprofile.Selector           `json:"selector"`
	Server             string                            `json:"server"`
	ServerSHA256       string                            `json:"server_sha256"`
	Args               []string                          `json:"args"`
	Environment        []string                          `json:"environment"`
	PrepLimits         prep.Limits                       `json:"prep_limits"`
	PostLimits         prep.PostPreparationLimits        `json:"post_limits"`
	Seeds              []uint64                          `json:"seeds"`
	OperationTimeoutMS int                               `json:"operation_timeout_ms"`
	ReadinessTimeoutMS int                               `json:"readiness_timeout_ms"`
	ShutdownTimeoutMS  int                               `json:"shutdown_timeout_ms"`
	OperationDirectory string                            `json:"operation_directory"`
	ProducerOutputs    [2]string                         `json:"producer_outputs"`
	PostOutputs        [2]string                         `json:"post_outputs"`
	ProtectedRoots     []string                          `json:"protected_roots"`
	AttemptMarkerPath  string                            `json:"attempt_marker_path"`
	ReportPath         string                            `json:"report_path"`
}

type variableCallerReport struct {
	Schema, Purpose, Mode, Phase                                                                   string
	Authority                                                                                      int
	Accepted                                                                                       bool
	OrdinaryNative, Describe, Workflow, FeatureIdentity, SourceCompleteness, RevisionQualification string
	ConfigSHA256, CodeHEAD, ScopeSHA256, ServerSHA256, MarkerSHA256                                string
	Config, Marker                                                                                 json.RawMessage
	InputSourceWitness                                                                             json.RawMessage
	Discovery                                                                                      json.RawMessage
	Runtime                                                                                        any
	Phases, Errors, CloseErrors                                                                    []string
	ProducerPhase                                                                                  string
	ProducerCopies                                                                                 [2]pilot.ProducerPacketCopyReceiptPrivateV2
	ProducerBudget                                                                                 pilot.ProducerPacketBudgetPrivateV2
	Qualification                                                                                  json.RawMessage
	PreparationSHA256                                                                              string
	PreparationBytes                                                                               int
	Coverage                                                                                       []prep.Coverage
	Ledger                                                                                         *prep.StageLedgerPrivateV2
	Outcome                                                                                        *prep.PostPublicationOutcomePrivateV2
	Cleanup                                                                                        variableCleanupObservation
	CodeInputs                                                                                     variableCodeWitness
}

func variableEnvironment(get func(string) string) (mode, path string, err error) {
	mode = get("LSP_TRACE_PRIVATE_V2_CALLER")
	if mode == "" {
		return "", "", nil
	}
	if mode != "preflight" && mode != "execute-once" {
		return "", "", errors.New("invalid private V2 caller sentinel")
	}
	path = get("LSP_TRACE_PRIVATE_V2_CALLER_CONFIG")
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", "", errors.New("canonical absolute private V2 config required")
	}
	return
}

// Token validation bounds collections and strings BEFORE typed decoding. Exact
// re-encoding then rejects casing aliases, omitted fields, duplicate/extra data,
// alternate numeric/string encodings and whitespace. Total input is capped first.
func variableJSON(raw []byte, out any) error {
	if len(raw) == 0 || len(raw) > variableCallerConfigCap || !utf8.Valid(raw) {
		return errors.New("config byte/UTF8 bound")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	work := 0
	var walk func(int) error
	walk = func(depth int) error {
		work++
		if depth > 16 || work > 2048 {
			return errors.New("config token/depth bound")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		switch x := t.(type) {
		case string:
			if len(x) > 8192 || strings.ContainsRune(x, 0) {
				return errors.New("config string bound")
			}
		case json.Delim:
			if x != '{' && x != '[' {
				return errors.New("config delimiter")
			}
			count := 0
			seen := map[string]bool{}
			for d.More() {
				count++
				if count > 64 || (x == '[' && count > 32) {
					return errors.New("config collection bound")
				}
				if x == '{' {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || len(s) > 128 || seen[s] {
						return errors.New("duplicate/invalid config key")
					}
					seen[s] = true
				}
				if e := walk(depth + 1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil {
				return e
			}
			if (x == '{' && end != json.Delim('}')) || (x == '[' && end != json.Delim(']')) {
				return errors.New("config delimiter mismatch")
			}
		}
		return nil
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	canonical, e := json.Marshal(out)
	if e != nil {
		return e
	}
	if !bytes.Equal(raw, canonical) {
		return errors.New("noncanonical config JSON")
	}
	return nil
}
func variableCanonical(path string) error {
	if path == "" || len(path) > 4096 || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("canonical absolute path required")
	}
	// Inspect every component, not just the final basename. This observation is
	// not a namespace lease; descriptor identities are checked again at use.
	p := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		p = filepath.Join(p, part)
		i, e := os.Lstat(p)
		if e != nil {
			return e
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return errors.New("path alias refused")
		}
	}
	return nil
}
func variableRead(path string, limit int) (raw []byte, info os.FileInfo, err error) {
	if err = variableCanonical(path); err != nil {
		return
	}
	root, e := os.OpenRoot(filepath.Dir(path))
	if e != nil {
		return nil, nil, e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	before, e := root.Lstat(filepath.Base(path))
	if e != nil {
		return nil, nil, e
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > int64(limit) {
		return nil, nil, errors.New("regular file byte bound")
	}
	f, e := root.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, nil, e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, e = f.Stat()
	if e != nil {
		return nil, nil, e
	}
	if !os.SameFile(before, info) {
		return nil, nil, errors.New("file identity changed")
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil {
		return nil, nil, e
	}
	after, e := root.Lstat(filepath.Base(path))
	if e != nil {
		return nil, nil, e
	}
	if !os.SameFile(info, after) || len(raw) > limit || int64(len(raw)) != info.Size() {
		return nil, nil, errors.New("file identity/length changed")
	}
	return raw, info, nil
}
func variableHex(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}
func variablePositiveFields(v any, ceilings any) bool {
	a, b := reflect.ValueOf(v), reflect.ValueOf(ceilings)
	if a.Type() != b.Type() {
		return false
	}
	for i := 0; i < a.NumField(); i++ {
		if a.Field(i).Kind() != reflect.Int || a.Field(i).Int() < 1 || a.Field(i).Int() > b.Field(i).Int() {
			return false
		}
	}
	return true
}
func variableValidate(c variableCallerConfig) error {
	if c.Schema != variableCallerSchema || c.Purpose != variableCallerPurpose || !variableHex(c.CodeHEAD, 20) || c.Session.Validate() != nil || len(c.Session.SessionID) > 1024 {
		return errors.New("private purpose/schema/code/session")
	}
	if len(c.Args) != 0 || c.Args == nil || len(c.Environment) == 0 || len(c.Environment) > 32 || c.ProtectedRoots == nil || len(c.ProtectedRoots) > 15 || len(c.Seeds) < 2 || len(c.Seeds) > 6 {
		return errors.New("explicit bounded args/env/roots/seeds")
	}
	if !strings.HasPrefix(c.ServerSHA256, "sha256:") || !variableHex(strings.TrimPrefix(c.ServerSHA256, "sha256:"), 32) {
		return errors.New("server SHA256 required")
	}
	for i, s := range c.Seeds {
		for _, p := range c.Seeds[:i] {
			if s == p {
				return errors.New("duplicate seed")
			}
		}
	}
	for _, x := range [][2]int{{c.OperationTimeoutMS, 240000}, {c.ReadinessTimeoutMS, 60000}, {c.ShutdownTimeoutMS, 10000}} {
		if x[0] < 1 || x[0] > x[1] {
			return errors.New("explicit timeout bound")
		}
	}
	if !variablePositiveFields(c.PrepLimits, prepLimitsV2()) || c.PrepLimits.MaxPartBytes < 2048 || c.PrepLimits.MaxTotalBytes < c.PrepLimits.MaxPartBytes || !variablePositiveFields(c.PostLimits, postLimitsV2()) {
		return errors.New("explicit preparation limits")
	}
	if e := variableCodeEnvironment(c.Environment, false); e != nil {
		return e
	}
	keys := map[string]bool{}
	for _, entry := range c.Environment {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || len(entry) > 4096 || !utf8.ValidString(entry) || strings.ContainsRune(entry, 0) || keys[key] {
			return errors.New("explicit unique environment")
		}
		keys[key] = true
		for i, ch := range []byte(key) {
			if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || i > 0 && ch >= '0' && ch <= '9') {
				return errors.New("environment key")
			}
		}
	}
	v, e := runtimeprofile.Validate(c.Selector)
	if e != nil || v.Workspace().String() != c.Scope.Root {
		return errors.New("exact scope selector")
	}
	for _, m := range c.Scope.Members {
		if m.LanguageID != "go" {
			return errors.New("first private profile requires go")
		}
	}
	return nil
}

type variableGitRunner func(context.Context, string, ...string) ([]byte, error)
type variableBoundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *variableBoundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("git output bound")
	}
	return b.Buffer.Write(p)
}

// Only these readonly raw-object/status forms are admitted. In particular,
// cat-file --filters/--textconv and arbitrary conversion commands cannot enter.
func variableReadOnlyGitArgs(args []string) bool {
	q := strings.Join(args, " ")
	switch q {
	case "rev-parse --show-toplevel", "rev-parse HEAD", "diff --cached --quiet", "ls-files --others --exclude-standard":
		return true
	}
	if len(args) == 4 && args[0] == "config" && args[1] == "--includes" && args[2] == "--get-regexp" && args[3] == variableGitForbiddenConfigPattern {
		return true
	}
	if len(args) == 3 && args[0] == "ls-tree" && args[1] == "-z" && variableHex(args[2], 20) {
		return true
	}
	if len(args) == 3 && args[0] == "cat-file" && (args[1] == "-s" || args[1] == "blob" || args[1] == "commit" || args[1] == "tree") && variableHex(args[2], 20) {
		return true
	}
	return false
}
func variableGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	if ctx == nil || !variableReadOnlyGitArgs(args) {
		return nil, errors.New("private Git command outside readonly allowlist/context")
	}
	// The real runner cannot redirect the code checkout to another clean repo.
	if strings.Join(args, " ") == "rev-parse --show-toplevel" {
		_, file, _, ok := runtime.Caller(0)
		expected := filepath.Dir(filepath.Dir(filepath.Dir(file)))
		if !ok || !filepath.IsAbs(expected) || root != expected {
			return nil, errors.New("config code root is not the compiled caller checkout")
		}
	}
	if len(args) > 0 && args[0] == "diff" {
		configured, ce := variableGit(ctx, root, "config", "--includes", "--get-regexp", variableGitForbiddenConfigPattern)
		if len(configured) != 0 {
			return nil, errors.New("Git clean filters/partial-clone helpers unsupported in readonly profile")
		}
		if ce != nil {
			var exit *exec.ExitError
			if !errors.As(ce, &exit) || exit.ExitCode() != 1 {
				return nil, ce
			}
		}
	}
	safe := append([]string{"--no-pager", "--no-replace-objects", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "core.untrackedCache=false", "--no-optional-locks", "-C", root}, args...)
	if len(args) > 0 && args[0] == "diff" {
		safe = append(safe, "--no-ext-diff", "--no-textconv", "--ignore-submodules=all")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/git", safe...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1"}
	limit := 8192
	if len(args) > 0 && (args[0] == "ls-tree" || args[0] == "cat-file" && len(args) > 1 && (args[1] == "blob" || args[1] == "commit" || args[1] == "tree")) {
		limit = 4 << 20
	}
	out := &variableBoundedBuffer{limit: limit}
	cmd.Stdout = out
	stderr := &variableBoundedBuffer{limit: 4096}
	cmd.Stderr = stderr
	e := cmd.Run()
	if stderr.Len() != 0 {
		return out.Bytes(), errors.New("readonly Git emitted stderr; refusing ambiguous status")
	}
	return out.Bytes(), e
}

type variablePreflight struct {
	cfg           variableCallerConfig
	frozen        pilot.FrozenScopePrivateV2
	raw           []byte
	info          os.FileInfo
	operationInfo os.FileInfo
	report        variableCallerReport
	codeVerifier  variableCodeVerifier
}

func variablePreflightRead(ctx context.Context, path string, git variableGitRunner) (variablePreflight, error) {
	return variablePreflightWithCode(ctx, path, git, variableVerifyCodeInputs)
}
func variablePreflightWithCode(ctx context.Context, path string, git variableGitRunner, verify variableCodeVerifier) (p variablePreflight, err error) {
	p.codeVerifier = verify
	if ctx == nil || git == nil || verify == nil {
		return p, errors.New("context/git required")
	}
	if err = ctx.Err(); err != nil {
		return
	}
	p.raw, p.info, err = variableRead(path, variableCallerConfigCap)
	if err != nil {
		return
	}
	if err = variableJSON(p.raw, &p.cfg); err != nil {
		return
	}
	c := p.cfg
	if err = variableValidate(c); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.OperationTimeoutMS)*time.Millisecond)
	defer cancel()
	for _, r := range []string{c.CodeRoot, c.Scope.Root} {
		if err = variableCanonical(r); err != nil {
			return
		}
	}
	// Source-path-matched repository, not binary/toolchain provenance. These
	// supplemental index/status checks never substitute for HEAD byte verification.
	// Working-tree diff is deliberately absent: clean conversion must never run.
	for _, q := range []struct {
		root string
		args []string
		want string
	}{
		{c.CodeRoot, []string{"rev-parse", "--show-toplevel"}, c.CodeRoot + "\n"},
		{c.CodeRoot, []string{"rev-parse", "HEAD"}, c.CodeHEAD + "\n"},
		{c.CodeRoot, []string{"diff", "--cached", "--quiet"}, ""},
		{c.CodeRoot, []string{"ls-files", "--others", "--exclude-standard"}, ""},
		{c.Scope.Root, []string{"rev-parse", "HEAD"}, c.Scope.RevisionAssertion + "\n"},
	} {
		var b []byte
		b, err = git(ctx, q.root, q.args...)
		if err != nil {
			return
		}
		if string(b) != q.want {
			err = errors.New("reviewed exact clean checkout/source revision required")
			return
		}
	}
	codeWitness, e := verify(ctx, c.CodeRoot, c.CodeHEAD)
	if e != nil {
		return p, e
	}
	var server []byte
	var si os.FileInfo
	server, si, err = variableRead(c.Server, 64<<20)
	if err != nil {
		return
	}
	if si.Mode().Perm()&0111 == 0 || pilot.Digest(server) != c.ServerSHA256 {
		err = errors.New("executable byte pin mismatch")
		return
	}
	paths := append([]string{c.Scope.Root, c.OperationDirectory}, c.ProducerOutputs[:]...)
	paths = append(paths, c.PostOutputs[:]...)
	paths = append(paths, c.ProtectedRoots...)
	infos := make([]os.FileInfo, len(paths))
	for i, r := range paths {
		if err = variableCanonical(r); err != nil {
			return
		}
		var root *os.Root
		root, err = os.OpenRoot(r)
		if err != nil {
			return
		}
		infos[i], err = root.Stat(".")
		err = errors.Join(err, root.Close())
		if err != nil {
			return
		}
		if !infos[i].IsDir() {
			err = errors.New("root is not directory")
			return
		}
		if i > 0 && i < 6 {
			if !realPostOutsideTemp(r) || infos[i].Mode().Perm() != 0700 {
				err = errors.New("private roots require existing 0700 outside /tmp")
				return
			}
			var f *os.File
			f, err = os.Open(r)
			if err != nil {
				return
			}
			var entries []os.DirEntry
			var re error
			entries, re = f.ReadDir(1)
			if re == io.EOF {
				re = nil
			}
			err = errors.Join(f.Close(), re)
			if err != nil {
				return
			}
			if len(entries) != 0 {
				err = errors.New("fresh empty roots required")
				return
			}
		}
		for j := 0; j < i; j++ {
			if os.SameFile(infos[i], infos[j]) || realPostContains(r, paths[j]) || realPostContains(paths[j], r) {
				err = errors.New("root equality/ancestry/physical alias")
				return
			}
		}
	}
	p.operationInfo = infos[1]
	for _, target := range []string{c.AttemptMarkerPath, c.ReportPath} {
		if filepath.Dir(target) != c.OperationDirectory || filepath.Clean(target) != target || target == path || target == c.Server || filepath.Base(target) == "." || filepath.Base(target) == ".." || len(target) > 4096 {
			err = errors.New("single direct child marker/report required")
			return
		}
		if _, e := os.Lstat(target); !errors.Is(e, os.ErrNotExist) {
			err = errors.New("marker/report not absent; attempt is spent")
			return
		}
	}
	if c.AttemptMarkerPath == c.ReportPath {
		err = errors.New("marker/report collision")
		return
	}
	for _, r := range append([]string{c.Scope.Root}, append(c.ProducerOutputs[:], c.PostOutputs[:]...)...) {
		if realPostContains(r, path) {
			err = errors.New("config/source/output collision")
			return
		}
	}
	p.frozen, err = pilot.FreezeScopePrivateV2(ctx, c.Scope)
	if err != nil {
		return
	}
	for _, member := range c.Scope.Members {
		// Prevent physical aliases to the configuration/executable as well as lexical
		// source overlap. The frozen reader independently checks source components.
		mi, me := os.Lstat(filepath.Join(c.Scope.Root, member.CanonicalRelativePath))
		if me != nil {
			return p, me
		}
		if os.SameFile(mi, p.info) || os.SameFile(mi, si) {
			return p, errors.New("source/config/executable physical alias")
		}
	}
	// Validate the genuine topology through readonly constructors, then close ALL.
	var closes []func() error
	defer func() {
		for i := len(closes) - 1; i >= 0; i-- {
			err = errors.Join(err, closes[i]())
		}
	}()
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, c.Scope.Root)
	if e != nil {
		return p, e
	}
	closes = append(closes, source.Close)
	var outputs [2]*publication.ContentRetentionRoot
	var observed []*publication.ObservedRoot
	for i, r := range c.ProducerOutputs {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, r)
		if e != nil {
			return p, e
		}
		closes = append(closes, outputs[i].Close)
		o, e := publication.OpenObservedRoot(ctx, r)
		if e != nil {
			return p, e
		}
		observed = append(observed, o)
		closes = append(closes, o.Close)
	}
	var additional []*publication.ObservedRoot
	for _, r := range c.ProtectedRoots {
		o, e := publication.OpenObservedRoot(ctx, r)
		if e != nil {
			return p, e
		}
		additional = append(additional, o)
		closes = append(closes, o.Close)
	}
	if _, e = pilot.NewProducerPacketRootsPrivateV2(ctx, outputs, source, additional); e != nil {
		return p, e
	}
	for i, r := range c.PostOutputs {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, r)
		if e != nil {
			return p, e
		}
		closes = append(closes, outputs[i].Close)
	}
	if e = publication.CheckPostPreparationRetentionTopologyPrivateV2(ctx, outputs, observed); e != nil {
		return p, e
	}
	p.report = variableCallerReport{Schema: variableCallerSchema, Purpose: c.Purpose, Mode: "preflight", Phase: "READONLY_PREFLIGHT_COMPLETE", OrdinaryNative: "NOT_ATTEMPTED", Describe: "NOT_ATTEMPTED", Workflow: "NOT_ATTEMPTED", FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", RevisionQualification: "CALLER_ASSERTED_NOT_AUTHENTICATED", ConfigSHA256: pilot.Digest(p.raw), CodeHEAD: c.CodeHEAD, ScopeSHA256: pilot.Digest(p.frozen.SpecBytes()), ServerSHA256: c.ServerSHA256, InputSourceWitness: p.frozen.ManifestBytes()}
	p.report.CodeInputs = codeWitness
	return p, ctx.Err()
}

// Package-private factory seam exists only in this _test.go file. Owned tests
// inject their own Starter; the real opt-in path always uses the managed backend.
type variableManagerFactory func() (*sessionruntime.Manager, error)

func variableRealManager() (*sessionruntime.Manager, error) {
	supervisor, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if e != nil {
		return nil, e
	}
	return sessionruntime.New(sessionruntime.Config{Starter: sessionruntime.ManagedStarter{Manager: supervisor}, Limits: variableRuntimeLimits()})
}
func variableRuntimeLimits() sessionruntime.Limits {
	return sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 1024, MaxOperations: 2}
}

// Exclusive direct-child writes through the pinned operation capability. A sync
// is practical durability, NOT authenticated history or a power-loss guarantee.
func variableWriteOnce(root *os.Root, name string, raw []byte, limit int, closeFile func(*os.File) error) (created bool, err error) {
	if len(raw) < 1 || len(raw) > limit || filepath.Base(name) != name {
		return false, errors.New("operation artifact bound/selector")
	}
	f, e := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return false, e
	}
	n, e := f.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	e = errors.Join(e, f.Sync(), closeFile(f))
	dir, de := root.Open(".")
	if de == nil {
		de = errors.Join(dir.Sync(), dir.Close())
	}
	return true, errors.Join(e, de)
}

// Cleanup observations are readonly status, not capabilities or execution permission.
// Unknown, pending, failed, foreign, and restart outcomes never qualify.
type variableCleanupObservation struct {
	SessionID                               string
	Generation                              uint64
	StopIntentID                            string
	StopCalls, ShutdownCalls, TerminalReads int
	ShutdownJoined, TerminalFound           bool
	Terminal                                sessionruntime.OperationSnapshot
	Status                                  string
}

func variableTerminalCleanup(o variableCleanupObservation) error {
	t := o.Terminal
	if !o.ShutdownJoined || o.StopCalls != 1 || o.ShutdownCalls != 1 || o.TerminalReads != 1 || o.StopIntentID == "" || !o.TerminalFound || t.ID != o.StopIntentID || t.SessionID != o.SessionID || t.Generation != o.Generation || t.Restart || t.CallerID != "private-variable-v2-caller-stop-once" || t.State != sessionruntime.OperationComplete || t.Failure != "" {
		return errors.New("private Stop terminal cleanup not verified: " + string(t.State) + " / " + string(t.Failure))
	}
	return nil
}

func variableRun(ctx context.Context, mode, path string, git variableGitRunner, factory variableManagerFactory, closeFile func(*os.File) error) (variableCallerReport, error) {
	return variableRunWithCode(ctx, mode, path, git, factory, closeFile, variableVerifyCodeInputs)
}
func variableRunWithCode(ctx context.Context, mode, path string, git variableGitRunner, factory variableManagerFactory, closeFile func(*os.File) error, verify variableCodeVerifier) (report variableCallerReport, err error) {
	if mode != "preflight" && mode != "execute-once" {
		return report, errors.New("explicit private caller mode required")
	}
	if ctx == nil {
		return report, errors.New("nil caller context")
	}
	p, e := variablePreflightWithCode(ctx, path, git, verify)
	if e != nil {
		return p.report, e
	}
	report = p.report
	report.Config = bytes.Clone(p.raw)
	if mode == "preflight" {
		return report, nil
	}
	if factory == nil || closeFile == nil {
		return report, errors.New("private runtime dependencies required")
	}
	c := p.cfg
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.OperationTimeoutMS)*time.Millisecond)
	defer cancel()
	root, e := os.OpenRoot(c.OperationDirectory)
	if e != nil {
		return report, e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, e := root.Stat(".")
	if e != nil || !os.SameFile(info, p.operationInfo) {
		return report, errors.Join(e, errors.New("operation root changed"))
	}
	// Recheck exact input bytes, executable pin, source and absence before claim.
	raw, ci, e := variableRead(path, variableCallerConfigCap)
	if e != nil || !os.SameFile(ci, p.info) || !bytes.Equal(raw, p.raw) {
		return report, errors.Join(e, errors.New("config changed before claim"))
	}
	server, _, e := variableRead(c.Server, 64<<20)
	if e != nil || pilot.Digest(server) != c.ServerSHA256 {
		return report, errors.Join(e, errors.New("server changed before claim"))
	}
	if _, e = pilot.FreezeScopePrivateV2(ctx, c.Scope); e != nil {
		return report, e
	}
	if _, e = root.Lstat(filepath.Base(c.ReportPath)); !errors.Is(e, os.ErrNotExist) {
		return report, errors.New("report no longer absent")
	}
	currentCode, e := p.codeVerifier(ctx, c.CodeRoot, c.CodeHEAD)
	if e != nil || currentCode != report.CodeInputs {
		return report, errors.Join(e, errors.New("repository code inputs changed before claim"))
	}
	marker, e := json.Marshal(struct {
		Status, ConfigSHA256, CodeHEAD, ScopeSHA256 string
		CodeInputs                                  variableCodeWitness
		PrepLimits                                  prep.Limits
		PostLimits                                  prep.PostPreparationLimits
		Authority                                   int
		Accepted                                    bool
	}{"RUN_CLAIMED_NOT_RECOVERABLE", report.ConfigSHA256, c.CodeHEAD, report.ScopeSHA256, report.CodeInputs, c.PrepLimits, c.PostLimits, 0, false})
	if e != nil {
		return report, e
	}
	// Even a write/sync/close failure after exclusive creation spends this attempt.
	created, claimErr := variableWriteOnce(root, filepath.Base(c.AttemptMarkerPath), marker, variableCallerConfigCap, func(f *os.File) error { return f.Close() })
	if !created {
		return report, claimErr
	}
	report.Mode = mode
	if claimErr == nil {
		report.MarkerSHA256 = pilot.Digest(marker)
		report.Marker = bytes.Clone(marker)
	}
	report.Phase = "CLAIMED"
	var closes []func() error
	var manager *sessionruntime.Manager
	var startedID, readinessID string
	shutdown := func() {
		if manager != nil {
			sctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ShutdownTimeoutMS)*time.Millisecond)
			// Shutdown only joins workers. A single explicit Stop owns child
			// teardown; inspect its terminal observation after that join.
			if readinessID != "" {
				_, _ = manager.WaitReadiness(sctx, readinessID)
			}
			observation := variableCleanupObservation{SessionID: c.Session.SessionID, Generation: c.Session.Generation, Status: "NO_STARTED_SESSION"}
			var stopErr error
			if startedID != "" {
				observation.StopCalls++
				stopped := manager.Stop(sctx, startedID, "private-variable-v2-caller-stop-once")
				observation.StopIntentID = stopped.IntentID
				if stopped.Failure != "" {
					stopErr = errors.New("private Stop: " + string(stopped.Failure))
				}
			}
			observation.ShutdownCalls++
			se := manager.Shutdown(sctx)
			observation.ShutdownJoined = se == nil
			// Parent clarification permits this readonly lookup after the join.
			// It does not enqueue work, rebind, restart or request provider data.
			if startedID != "" {
				observation.Status = "UNVERIFIED_FAILED_SPENT"
				if observation.StopIntentID != "" {
					observation.TerminalReads++
					observation.Terminal, observation.TerminalFound = manager.Operation(observation.StopIntentID)
				}
				terminalErr := variableTerminalCleanup(observation)
				se = errors.Join(se, stopErr, terminalErr)
				if se == nil {
					observation.Status = "JOINED_STOP_VERIFIED"
				}
			} else {
				se = errors.Join(se, stopErr)
			}
			report.Cleanup = observation
			cancel()
			manager = nil
			if se != nil {
				report.CloseErrors = append(report.CloseErrors, se.Error())
				err = errors.Join(err, se)
			}
		}
	}
	defer func() {
		shutdown()
		for i := len(closes) - 1; i >= 0; i-- {
			if e := closes[i](); e != nil {
				report.CloseErrors = append(report.CloseErrors, e.Error())
				err = errors.Join(err, e)
			}
		}
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			report.Phase = "FAILED_SPENT"
		}
		encoded, e := json.Marshal(report)
		if e == nil {
			_, e = variableWriteOnce(root, filepath.Base(c.ReportPath), encoded, variableCallerReportCap, closeFile)
		}
		err = errors.Join(err, e)
	}()
	if claimErr != nil {
		err = claimErr
		return
	}
	var source *publication.SourceProtectionRootPrivateV2
	source, err = publication.OpenSourceProtectionRootPrivateV2(ctx, c.Scope.Root)
	if err != nil {
		return
	}
	closes = append(closes, source.Close)
	var producer, post [2]*publication.ContentRetentionRoot
	var protected, additional []*publication.ObservedRoot
	for i, r := range c.ProducerOutputs {
		producer[i], err = publication.OpenContentRetentionRoot(ctx, r)
		if err != nil {
			return
		}
		closes = append(closes, producer[i].Close)
		var o *publication.ObservedRoot
		o, err = publication.OpenObservedRoot(ctx, r)
		if err != nil {
			return
		}
		protected = append(protected, o)
		closes = append(closes, o.Close)
	}
	for i, r := range c.PostOutputs {
		post[i], err = publication.OpenContentRetentionRoot(ctx, r)
		if err != nil {
			return
		}
		closes = append(closes, post[i].Close)
	}
	for _, r := range c.ProtectedRoots {
		var o *publication.ObservedRoot
		o, err = publication.OpenObservedRoot(ctx, r)
		if err != nil {
			return
		}
		additional = append(additional, o)
		closes = append(closes, o.Close)
	}
	var roots pilot.ProducerPacketRootsPrivateV2
	roots, err = pilot.NewProducerPacketRootsPrivateV2(ctx, producer, source, additional)
	if err != nil {
		return
	}
	currentCode, err = p.codeVerifier(ctx, c.CodeRoot, c.CodeHEAD)
	if err != nil || currentCode != report.CodeInputs {
		err = errors.Join(err, errors.New("repository code inputs changed before startup"))
		return
	}
	manager, err = factory()
	if err != nil {
		return
	}
	if manager == nil {
		err = errors.New("nil private manager")
		return
	}
	v, e := runtimeprofile.Validate(c.Selector)
	if e != nil {
		err = e
		return
	}
	start := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(v), LanguageID: "go", Process: managedprocess.Spec{Path: c.Server, Args: c.Args, Dir: c.Scope.Root, Env: c.Environment}})
	if start.Failure == "" {
		startedID = start.SessionID
	}
	if start.Failure != "" || start.SessionID != c.Session.SessionID || start.Generation != c.Session.Generation {
		err = errors.New("exact session startup mismatch")
		return
	}
	readyCtx, readyCancel := context.WithTimeout(ctx, time.Duration(c.ReadinessTimeoutMS)*time.Millisecond)
	ready := manager.BeginReadiness(readyCtx, start.SessionID, start.Generation, time.Now().Add(time.Duration(c.ReadinessTimeoutMS)*time.Millisecond))
	readinessID = ready.ID
	ready, found := manager.WaitReadiness(ctx, ready.ID)
	readyCancel()
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		err = errors.New("single readiness failed")
		return
	}
	meta, failure := manager.Metadata(start.SessionID, start.Generation)
	report.Runtime = meta
	if failure != "" || meta.ServerCommand != c.Server {
		err = errors.New("READY server command pin mismatch")
		return
	}
	report.Phases = append(report.Phases, "READY_OBSERVED_NOT_AUTHENTICATED")
	var origin pilot.EnumerationOriginPrivateV2
	var captures []pilot.OpaqueSuccessfulV2Capture
	err = manager.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: start.SessionID, Generation: start.Generation, Workspace: c.Scope.Root}, func(leased context.Context) error {
		var d pilot.ScopeDiscoveryReportPrivateV2
		var e error
		origin, d, e = pilot.DiscoverScopePrivateV2(leased, p.frozen, manager, c.Session)
		report.Discovery = d.Bytes()
		if e != nil {
			return e
		}
		for i := 0; i < origin.BatchCount(); i++ {
			b, e := origin.Batch(i)
			if e != nil {
				return e
			}
			capture, e := pilot.AcquireScopeBatchPrivateV2(leased, manager, origin, b)
			if e != nil {
				return e
			}
			captures = append(captures, capture)
		}
		return nil
	})
	shutdown()
	if err != nil {
		return
	}
	report.Phases = append(report.Phases, "LEASE_EXITED_MANAGER_SHUTDOWN")
	retained, e := pilot.RetainCompleteScopePrivateV2(ctx, p.frozen, origin, captures, roots)
	report.ProducerPhase = retained.Phase()
	report.ProducerCopies = retained.CopyReceipts()
	report.ProducerBudget = retained.BudgetSpent()
	if e != nil {
		err = e
		return
	}
	qualified, e := pilot.QualifyProducerPacketPrivateV2(ctx, retained.VerifiedPair())
	if e != nil {
		err = e
		return
	}
	proof, e := pilot.DeriveProducerPacketPrivateV2(ctx, qualified)
	if e != nil {
		err = e
		return
	}
	view, e := pilot.ReadDerivedProducerPacketPrivateV2(ctx, proof)
	if e != nil {
		err = e
		return
	}
	report.Qualification = view.QualificationReport
	var inputs []prep.InputPrivateV2
	for i := range view.Batches {
		projection, f := derived.ProjectPrivateV2(ctx, proof, i)
		if f != nil {
			err = f
			return
		}
		assessment, _, e := derived.InstabilityComputePrivateV2(ctx, projection, c.Seeds)
		if e != nil {
			err = e
			return
		}
		inputs = append(inputs, prep.InputPrivateV2{Projection2: projection, Assessment2: assessment})
	}
	prepared, e := prep.PreparePrivateV2(ctx, inputs, c.PrepLimits)
	if e != nil {
		err = e
		return
	}
	report.Coverage = prepared.Coverage
	h, e := prep.NewPostPreparationPrivateV2(ctx, inputs, c.PrepLimits, c.PostLimits)
	if e != nil {
		err = e
		return
	}
	preparedRaw, e := h.PreparedBytes()
	if e != nil {
		err = e
		return
	}
	report.PreparationBytes = len(preparedRaw)
	report.PreparationSHA256 = pilot.Digest(preparedRaw)
	ledger, e := h.Reconcile(ctx, nil, nil)
	if e != nil {
		err = e
		return
	}
	report.Ledger = &ledger
	outcome, e := prep.RetainPostPreparationPrivateV2(ctx, h, nil, nil, post, protected)
	report.Outcome = &outcome
	if e != nil {
		err = e
		return
	}
	if len(ledger.Leads) != 0 || outcome.Accepted || outcome.Authority != 0 {
		err = errors.New("private pending ceiling violated")
		return
	}
	for _, r := range post {
		b, e := r.ReadPostPreparationPrivateV2(ctx, outcome.EnvelopeSHA256, int64(outcome.ByteLength))
		if e != nil || !bytes.Equal(b, outcome.EnvelopeBytes()) {
			err = errors.Join(e, errors.New("exact envelope copy mismatch"))
			return
		}
	}
	report.Phase = "PENDING_INTERPRETATION_PRIVATE_V2"
	return
}
func TestPrivateVariableScopeRealV2(t *testing.T) {
	mode, path, e := variableEnvironment(os.Getenv)
	if e != nil {
		t.Fatal(e)
	}
	if mode == "" {
		t.Skip("default OFF: private V2 caller requires explicit preflight or execute-once")
	}
	if mode == "execute-once" && runtime.GOOS != "darwin" {
		t.Fatal("private real execution requires existing Darwin supervisor; no platform expansion")
	}
	// LocalDarwinSupervisor is the existing supported real-process backend. Do not
	// claim Linux real-provider support merely because owned tests compile there.
	report, e := variableRun(context.Background(), mode, path, variableGit, variableRealManager, func(f *os.File) error { return f.Close() })
	b, me := json.Marshal(report)
	if me != nil {
		t.Fatal(me)
	}
	if len(b) > variableCallerReportCap {
		t.Fatal("bounded observation report")
	}
	t.Log(string(b))
	if e != nil {
		t.Fatal(e)
	}
}
