package adr0011retention

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func frameFixture(t *testing.T) (*publication.Root, string, Manifest, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	frame := []byte("Content-Length: 2\r\n\r\n{}")
	sum := sha256.Sum256(frame)
	started := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC).UnixNano()
	m := Manifest{Version: ManifestVersion, FrameSelector: "frame", FrameSHA256: "sha256:" + hex.EncodeToString(sum[:]), FrameByteLength: len(frame), CaptureStartedUnixNano: started, ExpiresUnixNano: started + int64(ReplayWindow)}
	if err := os.WriteFile(filepath.Join(path, "frame"), frame, 0o600); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, path, m)
	return root, path, m, frame
}

func writeManifest(t *testing.T, path string, m Manifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "manifest"), append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func at(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestFrameReplayWindowAndClock(t *testing.T) {
	root, path, m, want := frameFixture(t)
	start := time.Unix(0, m.CaptureStartedUnixNano)
	for _, instant := range []time.Time{start, start.Add(ReplayWindow - time.Nanosecond)} {
		got, err := readFrameAt(root, "manifest", at(instant))
		if err != nil || string(got) != string(want) {
			t.Fatalf("eligible exact frame: %q %v", got, err)
		}
	}
	if _, err := readFrameAt(root, "manifest", at(start.Add(-time.Nanosecond))); !errors.Is(err, ErrClockInvalid) {
		t.Fatalf("future capture accepted: %v", err)
	}
	// Expiry is checked before the frame selector: the missing frame must not
	// displace the terminal window-closed result.
	if err := os.Remove(filepath.Join(path, "frame")); err != nil {
		t.Fatal(err)
	}
	for _, instant := range []time.Time{start.Add(ReplayWindow), start.Add(ReplayWindow + time.Hour)} {
		if _, err := readFrameAt(root, "manifest", at(instant)); !errors.Is(err, ErrWindowClosed) {
			t.Fatalf("expired frame accepted or read: %v", err)
		}
	}
}

func TestFrameReplayWindowClosesDuringReadAndOnRollback(t *testing.T) {
	root, _, m, _ := frameFixture(t)
	start := time.Unix(0, m.CaptureStartedUnixNano)
	for _, tc := range []struct {
		after time.Time
		want  error
	}{
		{start.Add(ReplayWindow), ErrWindowClosed},
		{start.Add(time.Minute - time.Nanosecond), ErrClockInvalid},
	} {
		count := 0
		clock := func() time.Time {
			count++
			if count == 1 {
				return start.Add(time.Minute)
			}
			return tc.after
		}
		if _, err := readFrameAt(root, "manifest", clock); !errors.Is(err, tc.want) {
			t.Fatalf("mid-read time violation not rejected: %v", err)
		}
	}
}

func TestFrameWindowRejectsMutationAndMalformedMetadata(t *testing.T) {
	root, path, m, _ := frameFixture(t)
	clock := at(time.Unix(0, m.CaptureStartedUnixNano).Add(time.Minute))
	if err := os.WriteFile(filepath.Join(path, "frame"), []byte(strings.Repeat("x", m.FrameByteLength)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFrameAt(root, "manifest", clock); !errors.Is(err, ErrFrameUnavailable) {
		t.Fatalf("equal-length changed frame accepted: %v", err)
	}
	bad := []Manifest{
		{Version: ManifestVersion, FrameSelector: "manifest", FrameSHA256: m.FrameSHA256, FrameByteLength: m.FrameByteLength, CaptureStartedUnixNano: m.CaptureStartedUnixNano, ExpiresUnixNano: m.ExpiresUnixNano},
		{Version: ManifestVersion, FrameSelector: "frame", FrameSHA256: m.FrameSHA256, FrameByteLength: m.FrameByteLength, CaptureStartedUnixNano: m.CaptureStartedUnixNano, ExpiresUnixNano: m.ExpiresUnixNano + 1},
		{Version: ManifestVersion, FrameSelector: "frame", FrameSHA256: m.FrameSHA256, FrameByteLength: MaxFrameBytes + 1, CaptureStartedUnixNano: m.CaptureStartedUnixNano, ExpiresUnixNano: m.ExpiresUnixNano},
		{Version: "unknown", FrameSelector: "frame", FrameSHA256: m.FrameSHA256, FrameByteLength: m.FrameByteLength, CaptureStartedUnixNano: m.CaptureStartedUnixNano, ExpiresUnixNano: m.ExpiresUnixNano},
	}
	for _, candidate := range bad {
		writeManifest(t, path, candidate)
		if _, err := readFrameAt(root, "manifest", clock); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("bad manifest accepted: %+v %v", candidate, err)
		}
	}
	for _, raw := range []string{`{"version":"x","version":"y"}`, `{"version":"x","unknown":1}`, `null`, `{}`, `{} {}`} {
		if err := os.WriteFile(filepath.Join(path, "manifest"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readFrameAt(root, "manifest", clock); !errors.Is(err, ErrManifestInvalid) {
			t.Fatalf("malformed manifest accepted: %q %v", raw, err)
		}
	}
}
