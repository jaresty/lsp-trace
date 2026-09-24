package schemas

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// This is a synthetic contract-level replay counterexample, not host observation
// or evidence that the future producer retains these bytes.
type gitProbeStream struct {
	bytes            []byte
	selector, digest string
	length           int
}
type gitProbeCommand struct {
	argv           []string
	status         int
	stdout, stderr gitProbeStream
	at             string
}
type gitProbe struct {
	phase, rootURI, commit, cwdURI, executableURI, executableDigest string
	dirty                                                           bool
	commands                                                        []gitProbeCommand
}

func gitProbeOutput(b []byte) gitProbeStream {
	d := fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	return gitProbeStream{b, "adr0011-references-host-git-output-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin", d, len(b)}
}
func checkGitProbePair(before, after gitProbe, rootURI, commit, executableURI, executableDigest string) bool {
	args := [][]string{{"git", "rev-parse", "--show-toplevel"}, {"git", "rev-parse", "HEAD"}, {"git", "status", "--porcelain=v1", "--untracked-files=all"}}
	var last time.Time
	for i, p := range []gitProbe{before, after} {
		phase := "BEFORE"
		if i == 1 {
			phase = "AFTER"
		}
		if p.phase != phase || p.dirty || p.rootURI != rootURI || p.cwdURI != rootURI || p.commit != commit || p.executableURI != executableURI || p.executableDigest != executableDigest || len(p.commands) != 3 {
			return false
		}
		for j, cmd := range p.commands {
			if len(cmd.argv) != len(args[j]) || cmd.status != 0 {
				return false
			}
			for k, arg := range cmd.argv {
				if arg != args[j][k] {
					return false
				}
			}
			at, err := time.Parse(time.RFC3339Nano, cmd.at)
			if err != nil || !strings.HasSuffix(cmd.at, "Z") || (!last.IsZero() && at.Before(last)) {
				return false
			}
			last = at
			for _, stream := range []gitProbeStream{cmd.stdout, cmd.stderr} {
				want := gitProbeOutput(stream.bytes)
				if stream.length != want.length || stream.digest != want.digest || stream.selector != want.selector || len(stream.bytes) > 1048576 {
					return false
				}
			}
			if len(cmd.stderr.bytes) != 0 {
				return false
			}
		}
		rootPath := string(p.commands[0].stdout.bytes)
		if !strings.HasPrefix(rootPath, "/") || !strings.HasSuffix(rootPath, "\n") || strings.ContainsAny(rootPath[:len(rootPath)-1], "\r\n\x00") {
			return false
		}
		path := strings.TrimSuffix(rootPath, "\n")
		for _, segment := range strings.Split(path, "/") {
			if segment == "." || segment == ".." {
				return false
			}
		}
		if (&url.URL{Scheme: "file", Path: path}).String() != rootURI || string(p.commands[1].stdout.bytes) != commit+"\n" || len(p.commands[2].stdout.bytes) != 0 {
			return false
		}
	}
	return true
}
func TestReferencesHostGitProbeContractCounterexamples(t *testing.T) {
	root, commit, executable, executableDigest := "file:///worktree", strings.Repeat("a", 40), "file:///usr/bin/git", "sha256:"+strings.Repeat("b", 64)
	mk := func(phase string) gitProbe {
		cmd := func(argv []string, stdout string) gitProbeCommand {
			return gitProbeCommand{argv, 0, gitProbeOutput([]byte(stdout)), gitProbeOutput(nil), "2026-09-24T03:00:00Z"}
		}
		return gitProbe{phase, root, commit, root, executable, executableDigest, false, []gitProbeCommand{cmd([]string{"git", "rev-parse", "--show-toplevel"}, "/worktree\n"), cmd([]string{"git", "rev-parse", "HEAD"}, commit+"\n"), cmd([]string{"git", "status", "--porcelain=v1", "--untracked-files=all"}, "")}}
	}
	valid := func(b, a gitProbe) bool { return checkGitProbePair(b, a, root, commit, executable, executableDigest) }
	if !valid(mk("BEFORE"), mk("AFTER")) {
		t.Fatal("valid synthetic pair")
	}
	cases := map[string]func(*gitProbe){
		"missing stream":          func(p *gitProbe) { p.commands[0].stdout.bytes = nil },
		"changed output digest":   func(p *gitProbe) { p.commands[1].stdout.digest = "sha256:" + strings.Repeat("c", 64) },
		"changed output selector": func(p *gitProbe) { p.commands[1].stdout.selector = "elsewhere" },
		"truncated output":        func(p *gitProbe) { p.commands[0].stdout.length-- },
		"wrong root":              func(p *gitProbe) { p.commands[0].stdout = gitProbeOutput([]byte("/other\n")) },
		"wrong commit":            func(p *gitProbe) { p.commands[1].stdout = gitProbeOutput([]byte(strings.Repeat("c", 40) + "\n")) },
		"dirty":                   func(p *gitProbe) { p.commands[2].stdout = gitProbeOutput([]byte("?? secret\n")) },
		"dirty flag":              func(p *gitProbe) { p.dirty = true },
		"wrong argv":              func(p *gitProbe) { p.commands[0].argv[2] = "HEAD" },
		"nonzero exit":            func(p *gitProbe) { p.commands[0].status = 1 },
		"stderr":                  func(p *gitProbe) { p.commands[0].stderr = gitProbeOutput([]byte("warning")) },
		"missing time":            func(p *gitProbe) { p.commands[0].at = "" },
		"reordered time":          func(p *gitProbe) { p.commands[0].at = "2026-09-24T03:00:01Z" },
		"wrong binary":            func(p *gitProbe) { p.executableDigest = "sha256:" + strings.Repeat("c", 64) },
		"wrong phase":             func(p *gitProbe) { p.phase = "BEFORE" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			b, a := mk("BEFORE"), mk("AFTER")
			change(&a)
			if valid(b, a) {
				t.Fatal("substituted probe admitted")
			}
		})
	}
}
