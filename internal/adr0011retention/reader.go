// Package adr0011retention contains private, unqualified frame-window probes.
// It does not issue method receipts, authenticate producers, or admit occurrences.
package adr0011retention

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const (
	ManifestVersion  = "lsp-trace.private.adr0011-frame-window.v0"
	MaxManifestBytes = 4096
	MaxFrameBytes    = 2 << 20
	ReplayWindow     = 24 * time.Hour
)

var (
	ErrManifestInvalid  = errors.New("private frame manifest invalid")
	ErrWindowClosed     = errors.New("private frame replay window closed")
	ErrClockInvalid     = errors.New("private frame clock invalid")
	ErrFrameUnavailable = errors.New("private frame unavailable or inconsistent")
)

// Manifest is a proposed private frame pointer, NOT a method receipt. Frame
// bytes and metadata require a separately qualified publication/inventory path.
// CaptureStartedUnixNano and ExpiresUnixNano are UTC Unix nanoseconds.
type Manifest struct {
	Version                string `json:"version"`
	FrameSelector          string `json:"frame_selector"`
	FrameSHA256            string `json:"frame_sha256"`
	FrameByteLength        int    `json:"frame_byte_length"`
	CaptureStartedUnixNano int64  `json:"capture_started_unix_nano"`
	ExpiresUnixNano        int64  `json:"expires_unix_nano"`
}

// ReadFrame is an expiry-aware, pinned-root private reader. Root custodians
// can still directly access expired disk bytes; this is not physical deletion.
// A process clock rolled backward before entry cannot be detected without a
// separately persisted trusted watermark, so this is not a qualified policy.
func ReadFrame(root *publication.Root, manifestSelector string) ([]byte, error) {
	return readFrameAt(root, manifestSelector, time.Now)
}

func readFrameAt(root *publication.Root, manifestSelector string, now func() time.Time) ([]byte, error) {
	if root == nil || now == nil || root.ValidatePrivate() != nil {
		return nil, ErrManifestInvalid
	}
	raw, err := root.ReadSelector(manifestSelector, MaxManifestBytes)
	if err != nil || strictjson.RejectDuplicates(raw) != nil {
		return nil, ErrManifestInvalid
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil {
		return nil, ErrManifestInvalid
	}
	canonical, err := json.Marshal(manifest)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || manifest.Version != ManifestVersion || manifest.FrameSelector == "" || manifest.FrameSelector == manifestSelector || manifest.FrameByteLength < 1 || manifest.FrameByteLength > MaxFrameBytes {
		return nil, ErrManifestInvalid
	}
	if manifest.CaptureStartedUnixNano <= 0 || manifest.CaptureStartedUnixNano > int64(^uint64(0)>>1)-int64(ReplayWindow) || manifest.ExpiresUnixNano != manifest.CaptureStartedUnixNano+int64(ReplayWindow) {
		return nil, ErrManifestInvalid
	}
	if len(manifest.FrameSHA256) != len("sha256:")+64 || manifest.FrameSHA256[:7] != "sha256:" {
		return nil, ErrManifestInvalid
	}
	if _, err := hex.DecodeString(manifest.FrameSHA256[7:]); err != nil || manifest.FrameSHA256 != "sha256:"+lowerHex(manifest.FrameSHA256[7:]) {
		return nil, ErrManifestInvalid
	}
	before := now().UnixNano()
	if before < manifest.CaptureStartedUnixNano {
		return nil, ErrClockInvalid
	}
	if before >= manifest.ExpiresUnixNano {
		return nil, ErrWindowClosed
	}
	frame, err := root.ReadSelector(manifest.FrameSelector, int64(manifest.FrameByteLength))
	if err != nil || len(frame) != manifest.FrameByteLength {
		return nil, ErrFrameUnavailable
	}
	sum := sha256.Sum256(frame)
	if "sha256:"+hex.EncodeToString(sum[:]) != manifest.FrameSHA256 {
		return nil, ErrFrameUnavailable
	}
	after := now().UnixNano()
	if after < before {
		return nil, ErrClockInvalid
	}
	if after >= manifest.ExpiresUnixNano {
		return nil, ErrWindowClosed
	}
	return frame, nil
}

func lowerHex(s string) string {
	return string(bytes.ToLower([]byte(s)))
}
