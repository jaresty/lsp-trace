package adr0011acquisition

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSelectedCommit = "0123456789abcdef0123456789abcdef01234567"

func makeSelectedGit(t *testing.T, root, script string) selectedGitExecutable {
	t.Helper()
	path := filepath.Join(root, "git")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	selected, err := selectGitExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func scriptSelectedGit(root, head, status, stderr string) string {
	return "#!/bin/sh\ncase \"$*\" in\n 'rev-parse --show-toplevel') printf '%s' '" + root + "' ;;\n 'rev-parse HEAD') printf '%s' '" + head + "' ;;\n 'status --porcelain=v1 --untracked-files=all') printf '%s' '" + status + "' ;;\n *) exit 7 ;;\nesac\nprintf '%s' '" + stderr + "' >&2\n"
}

func TestSelectedGitObservation(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	good := scriptSelectedGit(root+"\n", testSelectedCommit+"\n", "", "")
	selected := makeSelectedGit(t, root, good)
	observed, err := observeSelectedGit(context.Background(), selected, root, testSelectedCommit)
	if err != nil {
		t.Fatal(err)
	}
	if observed.executable.path != selected.path || observed.cwdURI != observed.rootURI || observed.commands[0].argv[0] != "git" {
		t.Fatal(observed)
	}
	for _, command := range observed.commands {
		if command.stderr == nil || command.exit != 0 || command.at.IsZero() {
			t.Fatalf("lost empty stderr or command metadata: %+v", command)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := observeSelectedGit(context.Background(), selected, root, testSelectedCommit); err != nil {
		t.Fatal("PATH substitution:", err)
	}
	if err := os.WriteFile(selected.path, []byte(good+"# changed\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := finalizeSelectedGit(selected); err == nil {
		t.Fatal("changed executable accepted")
	}
}

func TestSelectedGitRejectsBadObservations(t *testing.T) {
	root := t.TempDir()
	cases := map[string]struct{ top, head, status, stderr string }{
		"wrong root":       {"/elsewhere\n", testSelectedCommit + "\n", "", ""},
		"extra LF":         {root + "\n\n", testSelectedCommit + "\n", "", ""},
		"CRLF":             {root + "\r\n", testSelectedCommit + "\n", "", ""},
		"wrong commit":     {root + "\n", strings.Repeat("f", 40) + "\n", "", ""},
		"dirty whitespace": {root + "\n", testSelectedCommit + "\n", " ", ""},
		"dirty CRLF":       {root + "\n", testSelectedCommit + "\n", "\r\n", ""},
		"stderr":           {root + "\n", testSelectedCommit + "\n", "", "warning"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			selected := makeSelectedGit(t, dir, scriptSelectedGit(tc.top, tc.head, tc.status, tc.stderr))
			if _, err := observeSelectedGit(context.Background(), selected, root, testSelectedCommit); err == nil {
				t.Fatal("accepted invalid observation")
			}
		})
	}
	t.Run("nonzero exit", func(t *testing.T) {
		selected := makeSelectedGit(t, t.TempDir(), "#!/bin/sh\nexit 9\n")
		if _, err := observeSelectedGit(context.Background(), selected, root, testSelectedCommit); err == nil {
			t.Fatal("accepted nonzero exit")
		}
	})
	t.Run("executable substitution", func(t *testing.T) {
		selected := makeSelectedGit(t, t.TempDir(), scriptSelectedGit(root+"\n", testSelectedCommit+"\n", "", ""))
		if err := os.WriteFile(selected.path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := observeSelectedGit(context.Background(), selected, root, testSelectedCommit); err == nil {
			t.Fatal("accepted substituted executable")
		}
	})
}

func TestSelectedGitFileURIReservedPathBytes(t *testing.T) {
	if got, want := selectedGitFileURI("/tmp/a @/git"), "file:///tmp/a%20%40/git"; got != want {
		t.Fatalf("ASSERT_SELECTED_GIT_URI_SPELLING: %q != %q", got, want)
	}
}

func TestSelectedGitStreamBounds(t *testing.T) {
	stream := &selectedGitStream{}
	if n, err := stream.Write(bytes.Repeat([]byte{'a'}, maxSelectedGitStream)); err != nil || n != maxSelectedGitStream {
		t.Fatalf("at bound: %d %v", n, err)
	}
	if _, err := stream.Write([]byte{'b'}); err == nil || !stream.exceeded {
		t.Fatal("over bound accepted")
	}
}
