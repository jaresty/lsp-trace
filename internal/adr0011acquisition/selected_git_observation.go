package adr0011acquisition

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// selectedGitExecutable is an owner-selected executable, not the acquisition process.
// It is private until a typed hostGit record and its retention closure are implemented.
type selectedGitExecutable struct {
	path   string
	uri    string
	digest string
}

const maxSelectedGitStream = 1048576

var selectedGitCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)

func selectGitExecutable(path string) (selectedGitExecutable, error) {
	if path == "" {
		var err error
		path, err = exec.LookPath("git")
		if err != nil {
			return selectedGitExecutable{}, ErrAcquisition
		}
	}
	if !filepath.IsAbs(path) {
		return selectedGitExecutable{}, ErrAcquisition
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(canonical) || !filepath.IsLocal(filepath.Base(canonical)) {
		return selectedGitExecutable{}, ErrAcquisition
	}
	digest, err := digestSelectedGit(canonical)
	if err != nil {
		return selectedGitExecutable{}, err
	}
	return selectedGitExecutable{path: canonical, uri: selectedGitFileURI(canonical), digest: digest}, nil
}

func digestSelectedGit(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", ErrAcquisition
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() < 1 || stat.Size() > maxHostExecutable || stat.Mode().Perm()&0111 == 0 {
		return "", ErrAcquisition
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxHostExecutable+1))
	if err != nil || n != stat.Size() {
		return "", ErrAcquisition
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// selectedGitFileURI uses the same exact RFC 3986 byte spelling as the
// accepted host-Git root observation; url.URL.String can leave reserved path
// characters unescaped and is not that identity grammar.
func selectedGitFileURI(path string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.WriteString("file://")
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}

// finalizeSelectedGit refuses a changed executable at the same selected path.
func finalizeSelectedGit(selected selectedGitExecutable) error {
	digest, err := digestSelectedGit(selected.path)
	if err != nil || digest != selected.digest {
		return ErrAcquisition
	}
	return nil
}

type selectedGitCommand struct {
	argv   []string
	stdout []byte
	stderr []byte
	exit   int
	at     time.Time
}

type selectedGitObservation struct {
	executable selectedGitExecutable
	rootURI    string
	cwdURI     string
	commands   [3]selectedGitCommand
	root       string
	commit     string
	dirty      bool
}

type selectedGitStream struct {
	bytes    []byte
	exceeded bool
}

func (s *selectedGitStream) Write(p []byte) (int, error) {
	if len(p) > maxSelectedGitStream-len(s.bytes) {
		s.exceeded = true
		return 0, ErrAcquisition
	}
	s.bytes = append(s.bytes, p...)
	return len(p), nil
}

func runSelectedGitCommand(parent context.Context, selected selectedGitExecutable, root string, args []string) (selectedGitCommand, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	observation := selectedGitCommand{argv: append([]string{"git"}, args...), exit: -1, at: time.Now().UTC(), stdout: []byte{}, stderr: []byte{}}
	cmd := exec.CommandContext(ctx, selected.path, args...)
	cmd.Dir = root
	out, errs := &selectedGitStream{}, &selectedGitStream{}
	cmd.Stdout, cmd.Stderr = out, errs
	err := cmd.Run()
	observation.stdout = append(observation.stdout, out.bytes...)
	observation.stderr = append(observation.stderr, errs.bytes...)
	if cmd.ProcessState != nil {
		observation.exit = cmd.ProcessState.ExitCode()
	}
	if err != nil || ctx.Err() != nil || out.exceeded || errs.exceeded || observation.exit != 0 || len(observation.stderr) != 0 {
		return observation, ErrAcquisition
	}
	return observation, nil
}

// observeSelectedGit uses only the already-selected absolute executable; PATH is
// never consulted again. The root must be the exact registered canonical worktree.
func observeSelectedGit(ctx context.Context, selected selectedGitExecutable, root, revision string) (selectedGitObservation, error) {
	zero := selectedGitObservation{}
	if selected.path == "" || selected.digest == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.ContainsAny(root, "\r\n\x00") || !selectedGitCommit.MatchString(revision) {
		return zero, ErrAcquisition
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil || canonicalRoot != root || finalizeSelectedGit(selected) != nil {
		return zero, ErrAcquisition
	}
	rootURI := selectedGitFileURI(root)
	observation := selectedGitObservation{executable: selected, rootURI: rootURI, cwdURI: rootURI}
	argvs := [3][]string{{"rev-parse", "--show-toplevel"}, {"rev-parse", "HEAD"}, {"status", "--porcelain=v1", "--untracked-files=all"}}
	for i, args := range argvs {
		command, err := runSelectedGitCommand(ctx, selected, root, args)
		if err != nil {
			return zero, err
		}
		observation.commands[i] = command
	}
	if string(observation.commands[0].stdout) != root+"\n" || string(observation.commands[1].stdout) != revision+"\n" || len(observation.commands[2].stdout) != 0 {
		return zero, ErrAcquisition
	}
	observation.root, observation.commit = root, revision
	if err := finalizeSelectedGit(selected); err != nil {
		return zero, err
	}
	return observation, nil
}
