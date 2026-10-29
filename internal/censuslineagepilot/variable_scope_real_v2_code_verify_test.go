//go:build darwin || linux

package censuslineagepilot_test

// This is a bounded repository-input observation, NOT proof of the bytes used to
// compile the executing binary, toolchain/stdlib/external dependencies or SDKs.
import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	pilot "lsp-trace/internal/censuslineagepilot"
)

const variableCodeProfile = "HEAD_REPOSITORY_INPUTS_PRIVATE_V1"
const variableGitForbiddenConfigPattern = `^(filter\..*|extensions\.partialclone|remote\..*\.(promisor|partialclonefilter))$`

type variableCodeLimits struct{ MaxFiles, MaxFileBytes, MaxTotalBytes, MaxPackages, MaxDirectories, MaxEntries, MaxGitCommands, MaxDepth int }

func variableCodePolicy() variableCodeLimits {
	return variableCodeLimits{4096, 4 << 20, 64 << 20, 256, 2048, 65536, 12288, 64}
}

type variableCodeWitness struct {
	Profile, CodeRoot, CodeHEAD, CommitTreeOID, Module, InputsSHA256 string
	Files, Bytes, Packages, Directories, Entries, GitCommands        int
	Limits                                                           variableCodeLimits
}
type variableCodeVerifier func(context.Context, string, string) (variableCodeWitness, error)
type variableCodeEntry struct{ Mode, Kind, OID string }
type variableCodeDirectory struct {
	head   map[string]variableCodeEntry
	actual map[string]os.DirEntry
}
type variableCodeRecord struct {
	Path, BlobOID, SHA256 string
	Bytes                 int
}
type variableCodeCheck struct {
	ctx                      context.Context
	root, head, tree, module string
	limits                   variableCodeLimits
	witness                  variableCodeWitness
	dirs                     map[string]variableCodeDirectory
	records                  map[string]variableCodeRecord
	bodies                   map[string][]byte
}

// This first local build profile is pure Go. It does not parse a native
// preprocessor/assembler grammar or resolve includes. Headers and other data
// remain opaque byte-pinned auxiliaries, never admitted native compilation.
func variableUnsupportedNativeInput(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".s", ".sx", ".asm", ".c", ".cc", ".cpp", ".cxx", ".c++", ".m", ".mm", ".f", ".for", ".f90", ".f95", ".swig", ".swigcxx", ".syso", ".o", ".obj", ".a", ".so", ".dylib", ".dll", ".lib", ".bc", ".ll", ".rc":
		return true
	}
	return false
}

func variableCodeEnvironment(entries []string, requireIsolation bool) error {
	values := map[string]string{}
	for _, entry := range entries {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			values[k] = v
		}
	}
	if values["GOFLAGS"] != "" {
		return errors.New("repository-input profile rejects GOFLAGS (overlay/toolexec and other overrides)")
	}
	if values["GOWORK"] != "" && values["GOWORK"] != "off" {
		return errors.New("repository-input profile rejects external GOWORK")
	}
	if values["GOENV"] != "" && values["GOENV"] != "off" {
		return errors.New("repository-input profile rejects external GOENV")
	}
	if values["GO111MODULE"] != "" && values["GO111MODULE"] != "on" {
		return errors.New("repository-input profile requires module mode")
	}
	if requireIsolation && (values["GOWORK"] != "off" || values["GOENV"] != "off") {
		return errors.New("repository-input preflight requires explicit GOWORK=off and GOENV=off")
	}
	return nil
}
func variableCodeRel(rel string) bool {
	return rel == "" || fs.ValidPath(rel) && path.Clean(rel) == rel && !strings.ContainsAny(rel, "\\:\x00\r\n") && len(rel) <= 4096 && len(strings.Split(rel, "/")) <= 64
}
func (c *variableCodeCheck) git(args ...string) ([]byte, error) {
	if e := c.ctx.Err(); e != nil {
		return nil, e
	}
	c.witness.GitCommands++
	if c.witness.GitCommands > c.limits.MaxGitCommands {
		return nil, errors.New("repository-input git command budget")
	}
	return variableGit(c.ctx, c.root, args...)
}
func (c *variableCodeCheck) directory(rel string) (variableCodeDirectory, error) {
	if d, ok := c.dirs[rel]; ok {
		return d, nil
	}
	if !variableCodeRel(rel) {
		return variableCodeDirectory{}, errors.New("repository-input directory selector")
	}
	object := c.tree
	if rel != "" {
		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		pd, e := c.directory(parent)
		if e != nil {
			return variableCodeDirectory{}, e
		}
		name := path.Base(rel)
		h, hok := pd.head[name]
		a, aok := pd.actual[name]
		if !hok || !aok || h.Kind != "tree" || h.Mode != "040000" || !a.IsDir() || a.Type()&os.ModeSymlink != 0 {
			return variableCodeDirectory{}, errors.New("HEAD/working input directory mismatch, submodule or alias")
		}
		object = h.OID
	}
	c.witness.Directories++
	if c.witness.Directories > c.limits.MaxDirectories {
		return variableCodeDirectory{}, errors.New("repository-input directory budget")
	}
	absolute := filepath.Join(c.root, filepath.FromSlash(rel))
	if e := variableCanonical(absolute); e != nil {
		return variableCodeDirectory{}, e
	}
	f, e := os.OpenFile(absolute, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return variableCodeDirectory{}, e
	}
	entries, re := f.ReadDir(c.limits.MaxEntries - c.witness.Entries + 1)
	ce := f.Close()
	if re == io.EOF {
		re = nil
	}
	if e = errors.Join(re, ce); e != nil {
		return variableCodeDirectory{}, e
	}
	c.witness.Entries += len(entries)
	if c.witness.Entries > c.limits.MaxEntries {
		return variableCodeDirectory{}, errors.New("repository-input entry budget")
	}
	raw, e := c.git("cat-file", "tree", object)
	if e != nil {
		return variableCodeDirectory{}, fmt.Errorf("HEAD input directory %s: %w", rel, e)
	}
	d := variableCodeDirectory{head: map[string]variableCodeEntry{}, actual: map[string]os.DirEntry{}}
	for _, entry := range entries {
		if !variableCodeRel(entry.Name()) || strings.Contains(entry.Name(), "/") {
			return d, errors.New("repository-input filename unsupported")
		}
		d.actual[entry.Name()] = entry
	}
	// Check the entire original tree object, then decode its finite Git format:
	// ASCII mode, space, name, NUL, exactly 20 raw object-ID bytes. This binds
	// parent-tree entries to children instead of trusting a replaced projection.
	hash := sha1.New()
	_, _ = fmt.Fprintf(hash, "tree %d%c", len(raw), 0)
	_, _ = hash.Write(raw)
	if fmt.Sprintf("%x", hash.Sum(nil)) != object {
		return d, errors.New("unreplaced HEAD tree object identity mismatch")
	}
	for remaining := raw; len(remaining) > 0; {
		nul := bytes.IndexByte(remaining, 0)
		if nul < 0 || nul+21 > len(remaining) {
			return d, errors.New("HEAD binary tree encoding unsupported")
		}
		mode, name, ok := strings.Cut(string(remaining[:nul]), " ")
		if !ok || !variableCodeRel(name) || name == "" || strings.Contains(name, "/") {
			return d, errors.New("HEAD tree path unsupported")
		}
		oid := fmt.Sprintf("%x", remaining[nul+1:nul+21])
		remaining = remaining[nul+21:]
		kind := "blob"
		switch mode {
		case "40000":
			mode = "040000"
			kind = "tree"
		case "100644", "100755", "120000":
		case "160000":
			kind = "commit"
		default:
			return d, errors.New("HEAD tree mode unsupported")
		}
		if _, ok := d.head[name]; ok {
			return d, errors.New("duplicate HEAD entry")
		}
		c.witness.Entries++
		if c.witness.Entries > c.limits.MaxEntries {
			return d, errors.New("HEAD input entry budget")
		}
		d.head[name] = variableCodeEntry{mode, kind, oid}
	}
	c.dirs[rel] = d
	return d, nil
}
func variableCodeNames(d variableCodeDirectory) []string {
	m := map[string]bool{}
	for n := range d.head {
		m[n] = true
	}
	for n := range d.actual {
		m[n] = true
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
func variableCodeHidden(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
func variableCodeVCS(name string) bool {
	return name == ".git" || name == ".hg" || name == ".svn" || name == ".bzr"
}
func (c *variableCodeCheck) file(rel string, entry variableCodeEntry) ([]byte, error) {
	if raw, ok := c.bodies[rel]; ok {
		return raw, nil
	}
	if !variableCodeRel(rel) || entry.Kind != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, fmt.Errorf("HEAD matching regular input required: %s", rel)
	}
	if len(c.records) >= c.limits.MaxFiles {
		return nil, errors.New("repository-input file budget")
	}
	sizeRaw, e := c.git("cat-file", "-s", entry.OID)
	if e != nil {
		return nil, e
	}
	size, e := strconv.Atoi(strings.TrimSpace(string(sizeRaw)))
	if e != nil || size < 0 || size > c.limits.MaxFileBytes || size > c.limits.MaxTotalBytes-c.witness.Bytes {
		return nil, errors.New("repository-input prospective file/aggregate byte budget")
	}
	raw, e := variableCodeRead(filepath.Join(c.root, filepath.FromSlash(rel)), size)
	if e != nil {
		return nil, fmt.Errorf("working input %s: %w", rel, e)
	}
	if len(raw) > c.limits.MaxTotalBytes-c.witness.Bytes {
		return nil, errors.New("repository-input aggregate byte budget")
	}
	expected, e := c.git("cat-file", "blob", entry.OID)
	if e != nil {
		return nil, e
	}
	if len(expected) != size {
		return nil, errors.New("HEAD blob length changed")
	}
	hash := sha1.New()
	_, _ = fmt.Fprintf(hash, "blob %d%c", len(expected), 0)
	_, _ = hash.Write(expected)
	if fmt.Sprintf("%x", hash.Sum(nil)) != entry.OID {
		return nil, errors.New("HEAD blob object identity mismatch")
	}
	if !bytes.Equal(raw, expected) {
		return nil, fmt.Errorf("HEAD repository input bytes mismatch: %s", rel)
	}
	c.witness.Bytes += len(raw)
	c.witness.Files++
	c.records[rel] = variableCodeRecord{rel, entry.OID, pilot.Digest(raw), len(raw)}
	c.bodies[rel] = raw
	return raw, nil
}
func variableCodeRead(file string, limit int) (raw []byte, err error) {
	if e := variableCanonical(file); e != nil {
		return nil, e
	}
	parent, e := os.OpenRoot(filepath.Dir(file))
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	before, e := parent.Lstat(filepath.Base(file))
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, errors.New("repository-input regular file/byte bound")
	}
	f, e := parent.OpenFile(filepath.Base(file), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	supplied, e := f.Stat()
	if e != nil || !os.SameFile(before, supplied) {
		return nil, errors.Join(e, errors.New("repository-input identity changed"))
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil {
		return nil, e
	}
	after, e := parent.Lstat(filepath.Base(file))
	if e != nil || !os.SameFile(supplied, after) || len(raw) > limit || int64(len(raw)) != supplied.Size() {
		return nil, errors.Join(e, errors.New("repository-input bytes/identity changed"))
	}
	return raw, nil
}
func variableCodeModule(raw []byte) (string, error) {
	module := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "//", 2)[0])
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "replace" {
			return "", errors.New("repository-input profile does not support replace directives")
		}
		if fields[0] == "module" {
			if len(fields) != 2 || module != "" {
				return "", errors.New("single module declaration required")
			}
			module = fields[1]
			if strings.HasPrefix(module, "\"") {
				var e error
				module, e = strconv.Unquote(module)
				if e != nil {
					return "", e
				}
			}
			if !fs.ValidPath(module) || strings.ContainsAny(module, "\\:\x00\r\n\t ") {
				return "", errors.New("module path unsupported")
			}
		}
	}
	if module == "" {
		return "", errors.New("module declaration absent")
	}
	return module, nil
}
func variableCodeEmbeds(raw []byte) (patterns []string, err error) {
	fset := token.NewFileSet()
	f := fset.AddFile("verified.go", -1, len(raw))
	var s scanner.Scanner
	s.Init(f, raw, func(_ token.Position, msg string) {
		if err == nil {
			err = errors.New("Go scanner: " + msg)
		}
	}, scanner.ScanComments)
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && strings.HasPrefix(lit, "//go:cgo_") {
			return nil, errors.New("native cgo compiler directives unsupported in pure-Go repository-input profile")
		}
		if tok != token.COMMENT || !strings.HasPrefix(lit, "//go:embed ") && !strings.HasPrefix(lit, "//go:embed\t") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(lit, "//go:embed"))
		count := 0
		for rest != "" {
			p := ""
			if rest[0] == '"' || rest[0] == '`' {
				quoted, e := strconv.QuotedPrefix(rest)
				if e != nil {
					return nil, e
				}
				p, e = strconv.Unquote(quoted)
				if e != nil {
					return nil, e
				}
				rest = strings.TrimSpace(rest[len(quoted):])
			} else {
				i := strings.IndexAny(rest, " \t")
				if i < 0 {
					p = rest
					rest = ""
				} else {
					p = rest[:i]
					rest = strings.TrimSpace(rest[i:])
				}
			}
			if len(p) > 4096 || len(patterns) >= 256 {
				return nil, errors.New("embed pattern budget")
			}
			patterns = append(patterns, p)
			count++
		}
		if count == 0 {
			return nil, errors.New("empty embed directive")
		}
	}
	return patterns, err
}
func (c *variableCodeCheck) embedFiles(rel string, all bool) (int, error) {
	d, e := c.directory(rel)
	if e != nil {
		return 0, e
	}
	// Nested modules and vendor/embed indirection are not admitted by this profile.
	if _, ok := d.head["go.mod"]; ok {
		return 0, errors.New("embedded nested module unsupported")
	}
	if _, ok := d.actual["go.mod"]; ok {
		return 0, errors.New("embedded nested module unsupported")
	}
	count := 0
	for _, name := range variableCodeNames(d) {
		if variableCodeVCS(name) || !all && variableCodeHidden(name) {
			continue
		}
		n, e := c.embedNode(path.Join(rel, name), d, name, all)
		if e != nil {
			return 0, e
		}
		count += n
	}
	return count, nil
}
func (c *variableCodeCheck) embedNode(rel string, d variableCodeDirectory, name string, all bool) (int, error) {
	h, hok := d.head[name]
	a, aok := d.actual[name]
	if !hok || !aok {
		return 0, fmt.Errorf("HEAD/working embed membership mismatch: %s", rel)
	}
	if a.Type()&os.ModeSymlink != 0 {
		return 0, errors.New("embedded source alias")
	}
	if h.Kind == "tree" && a.IsDir() {
		return c.embedFiles(rel, all)
	}
	if a.IsDir() || h.Kind != "blob" {
		return 0, errors.New("embed type mismatch")
	}
	_, e := c.file(rel, h)
	if e != nil {
		return 0, e
	}
	return 1, nil
}
func (c *variableCodeCheck) embedPattern(base, pattern string) (int, error) {
	all := strings.HasPrefix(pattern, "all:")
	pattern = strings.TrimPrefix(pattern, "all:")
	if !variableCodeRel(pattern) {
		return 0, errors.New("embed pattern unsupported")
	}
	if _, e := path.Match(pattern, "x"); e != nil {
		return 0, e
	}
	parts := strings.Split(pattern, "/")
	var match func(string, int) (int, error)
	match = func(dir string, index int) (int, error) {
		d, e := c.directory(dir)
		if e != nil {
			return 0, e
		}
		count := 0
		for _, name := range variableCodeNames(d) {
			if variableCodeVCS(name) {
				continue
			}
			ok, e := path.Match(parts[index], name)
			if e != nil {
				return 0, e
			}
			if !ok {
				continue
			}
			rel := path.Join(dir, name)
			if index == len(parts)-1 {
				n, e := c.embedNode(rel, d, name, all)
				if e != nil {
					return 0, e
				}
				count += n
				continue
			}
			h, hok := d.head[name]
			a, aok := d.actual[name]
			if !hok || !aok || h.Kind != "tree" || !a.IsDir() || a.Type()&os.ModeSymlink != 0 {
				return 0, errors.New("embed directory mismatch/alias")
			}
			n, e := match(rel, index+1)
			if e != nil {
				return 0, e
			}
			count += n
		}
		return count, nil
	}
	n, e := match(base, 0)
	if e == nil && n == 0 {
		e = errors.New("embed pattern has no files")
	}
	return n, e
}
func variableVerifyCodeInputs(ctx context.Context, root, head string) (variableCodeWitness, error) {
	empty := variableCodeWitness{}
	if ctx == nil || !variableHex(head, 20) {
		return empty, errors.New("code-input context/HEAD")
	}
	if e := variableCodeEnvironment(os.Environ(), true); e != nil {
		return empty, e
	}
	if e := variableCanonical(root); e != nil {
		return empty, e
	}
	c := &variableCodeCheck{ctx: ctx, root: root, head: head, limits: variableCodePolicy(), dirs: map[string]variableCodeDirectory{}, records: map[string]variableCodeRecord{}, bodies: map[string][]byte{}}
	c.witness = variableCodeWitness{Profile: variableCodeProfile, CodeRoot: root, CodeHEAD: head, Limits: c.limits}
	actualHead, e := c.git("rev-parse", "HEAD")
	if e != nil || string(actualHead) != head+"\n" {
		return empty, errors.Join(e, errors.New("code-input HEAD changed"))
	}
	// Refuse lazy-fetch/partial-clone configurations rather than permit a read
	// command to obtain missing objects from a remote. Config queries are builtin
	// readonly reads; no helper, hook or arbitrary executable is invoked.
	config, ce := c.git("config", "--includes", "--get-regexp", variableGitForbiddenConfigPattern)
	if len(config) != 0 {
		return empty, errors.New("Git filters/partial clone/promisor repository inputs unsupported")
	}
	if ce != nil {
		var exit *exec.ExitError
		if !errors.As(ce, &exit) || exit.ExitCode() != 1 {
			return empty, ce
		}
	}
	// Bind the literal unreplaced commit bytes to its original tree. Every
	// subsequent directory is addressed by that tree's recorded object IDs.
	commit, e := c.git("cat-file", "commit", head)
	if e != nil {
		return empty, e
	}
	hash := sha1.New()
	_, _ = fmt.Fprintf(hash, "commit %d%c", len(commit), 0)
	_, _ = hash.Write(commit)
	if fmt.Sprintf("%x", hash.Sum(nil)) != head {
		return empty, errors.New("unreplaced HEAD commit object identity mismatch")
	}
	first, _, ok := strings.Cut(string(commit), "\n")
	tree, hasTree := strings.CutPrefix(first, "tree ")
	if !ok || !hasTree || !variableHex(tree, 20) {
		return empty, errors.New("unreplaced commit tree binding unsupported")
	}
	c.tree = tree
	c.witness.CommitTreeOID = tree
	top, e := c.directory("")
	if e != nil {
		return empty, e
	}
	for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum"} {
		h, hok := top.head[name]
		_, aok := top.actual[name]
		if !hok && !aok {
			if name == "go.mod" {
				return empty, errors.New("tracked go.mod required")
			}
			continue
		}
		if hok != aok {
			return empty, fmt.Errorf("module metadata membership mismatch: %s", name)
		}
		raw, e := c.file(name, h)
		if e != nil {
			return empty, e
		}
		if name == "go.mod" {
			c.module, e = variableCodeModule(raw)
			if e != nil {
				return empty, e
			}
		}
		if name == "go.work" || name == "go.work.sum" {
			return empty, errors.New("repository-input profile does not support workspace files")
		}
	}
	if _, ok := top.head["vendor"]; ok {
		return empty, errors.New("repository-input vendoring unsupported")
	}
	if _, ok := top.actual["vendor"]; ok {
		return empty, errors.New("repository-input vendoring unsupported")
	}
	queue := []string{"internal/censuslineagepilot"}
	seen := map[string]bool{}
	for len(queue) > 0 {
		rel := queue[0]
		queue = queue[1:]
		if seen[rel] {
			continue
		}
		seen[rel] = true
		c.witness.Packages++
		if c.witness.Packages > c.limits.MaxPackages {
			return empty, errors.New("repository-input package budget")
		}
		if !variableCodeRel(rel) {
			return empty, errors.New("local package path unsupported")
		}
		for _, part := range strings.Split(rel, "/") {
			if variableCodeHidden(part) {
				return empty, errors.New("hidden local package unsupported")
			}
		}
		d, e := c.directory(rel)
		if e != nil {
			return empty, e
		}
		goFiles := 0
		for _, name := range variableCodeNames(d) {
			if variableCodeHidden(name) {
				continue
			}
			h, hok := d.head[name]
			a, aok := d.actual[name]
			// Conservative superset: every visible direct package file, not merely a
			// guessed native extension list. Only imports/embeds traverse subdirectories.
			if hok && h.Kind == "tree" && aok && a.IsDir() {
				continue
			}
			if !hok && aok && a.IsDir() {
				continue
			}
			if hok && h.Kind == "tree" && !aok {
				continue
			}
			if !hok || !aok || a.IsDir() || h.Kind != "blob" {
				return empty, fmt.Errorf("HEAD/working package input membership mismatch: %s/%s", rel, name)
			}
			if name == "go.mod" && rel != "" {
				return empty, errors.New("local nested module unsupported")
			}
			if variableUnsupportedNativeInput(name) {
				return empty, fmt.Errorf("private repository-input profile rejects compiler-consumed native/assembly inputs and unsupported preprocessing/include indirection: %s/%s", rel, name)
			}
			raw, e := c.file(path.Join(rel, name), h)
			if e != nil {
				return empty, e
			}
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			goFiles++
			parsed, e := parser.ParseFile(token.NewFileSet(), name, raw, parser.ImportsOnly)
			if e != nil {
				return empty, e
			}
			for _, imp := range parsed.Imports {
				p, e := strconv.Unquote(imp.Path.Value)
				if e != nil {
					return empty, e
				}
				if p == "C" {
					return empty, errors.New("cgo/local native include indirection unsupported")
				}
				if strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00\r\n") || !fs.ValidPath(p) {
					return empty, errors.New("relative/local import unsupported")
				}
				if p == c.module || strings.HasPrefix(p, c.module+"/") {
					local := strings.TrimPrefix(strings.TrimPrefix(p, c.module), "/")
					if !variableCodeRel(local) {
						return empty, errors.New("local import path unsupported")
					}
					queue = append(queue, local)
				}
			}
			patterns, e := variableCodeEmbeds(raw)
			if e != nil {
				return empty, e
			}
			for _, p := range patterns {
				if _, e := c.embedPattern(rel, p); e != nil {
					return empty, e
				}
			}
		}
		if goFiles == 0 {
			return empty, errors.New("local package has no eligible Go source")
		}
		sort.Strings(queue)
	}
	records := make([]variableCodeRecord, 0, len(c.records))
	for _, r := range c.records {
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	raw, e := json.Marshal(struct {
		Profile, HEAD, CommitTreeOID, Module string
		Limits                               variableCodeLimits
		Inputs                               []variableCodeRecord
	}{variableCodeProfile, head, c.tree, c.module, c.limits, records})
	if e != nil {
		return empty, e
	}
	c.witness.Module = c.module
	c.witness.InputsSHA256 = pilot.Digest(raw)
	return c.witness, ctx.Err()
}
