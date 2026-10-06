package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/lspwire"
)

const (
	cFrameV2Root             = "testdata/adr0011-c-frames-v2"
	cFrameV2ManifestSHA      = "c7d4b32b19d31132bb5ed4c0aa2b24a491719ca985f836c20f18cf4c8db24968"
	cCumulativeV2Root        = "testdata/adr0011-c-cumulative-v2"
	cCumulativeV2ManifestSHA = "29c54b0215d77a1b2d4cd7a4e49fdc6f42c10dc64ddaa942b30cde2742a67507"
)

type cFrameV2Pin struct {
	Path            string `json:"path"`
	Length          int    `json:"length"`
	HeaderLength    int    `json:"header_length"`
	SeparatorLength int    `json:"separator_length"`
	BodyLength      int    `json:"body_length"`
	SHA             string `json:"sha256"`
}

type cFrameV2Manifest struct {
	Schema string                 `json:"schema"`
	Cases  map[string]cFrameV2Pin `json:"cases"`
	Limits struct {
		CompleteFrameCeiling        int `json:"complete_frame_ceiling"`
		DefinitionRemarshalMaxBytes int `json:"definition_remarshal_max_bytes"`
		ExactResultMaxBytes         int `json:"exact_result_max_bytes"`
	} `json:"limits"`
}

type cCumulativeV2Manifest struct {
	Schema string                        `json:"schema"`
	Cases  map[string]cCumulativeCasePin `json:"cases"`
	Limits struct {
		CompleteFrameCeiling        int `json:"complete_frame_ceiling"`
		CurrentAttemptWireMaxBytes  int `json:"current_attempt_wire_max_bytes"`
		DefinitionRemarshalMaxBytes int `json:"definition_remarshal_max_bytes"`
		ExactResultMaxBytes         int `json:"exact_result_max_bytes"`
		MessageAttemptCap           int `json:"message_attempt_cap"`
	} `json:"limits"`
}

func cFixtureV2Manifest(t *testing.T, root, digest string, target any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil || cCumulativeSHA(raw) != digest || json.Unmarshal(raw, target) != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: successor manifest %s: %v", root, err)
	}
}

func cFixtureV2Message(t *testing.T, wire []byte, wantWire, remarshalLimit, resultLimit int, response bool) lspwire.Message {
	t.Helper()
	if len(wire) != wantWire {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: successor wire total got=%d want=%d", len(wire), wantWire)
	}
	msg, observed, exact, retained, err := lspwire.NewReader(bytes.NewReader(wire), lspwire.DefaultLimits()).ReadWithFrameIfWithin(int64(len(wire)))
	if err != nil || !retained || observed.FrameBytes != int64(wantWire) || !bytes.Equal(exact, wire) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: successor complete wire: %v", err)
	}
	remarshal, err := json.Marshal(msg)
	if err != nil || len(remarshal) > remarshalLimit {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C09/C10 remarshal bytes=%d limit=%d err=%v", len(remarshal), remarshalLimit, err)
	}
	if response {
		if len(msg.Result) == 0 || len(msg.Result) > resultLimit || string(msg.ID) != "1" {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C11/C12 result bytes=%d limit=%d id=%s", len(msg.Result), resultLimit, msg.ID)
		}
	} else if msg.Method == "" || len(msg.Result) != 0 {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: notification shape method=%q result=%d", msg.Method, len(msg.Result))
	}
	return msg
}

func cFrameV2Original(t *testing.T, name string) []byte {
	t.Helper()
	var manifest cFrameV2Manifest
	cFixtureV2Manifest(t, cFrameV2Root, cFrameV2ManifestSHA, &manifest)
	pin, ok := manifest.Cases[name]
	want := cFrameCap
	if name == "plus-one" {
		want++
	}
	if manifest.Schema != "adr0011-c-frames-v2" || !ok || pin.Length != want || manifest.Limits.CompleteFrameCeiling != cFrameCap || manifest.Limits.DefinitionRemarshalMaxBytes != 1048576 || manifest.Limits.ExactResultMaxBytes != 524288 {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C01 v2 manifest %s", name)
	}
	wire, err := os.ReadFile(filepath.Join(cFrameV2Root, pin.Path))
	if err != nil || len(wire) != pin.Length || cCumulativeSHA(wire) != pin.SHA {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C01 v2 asset %s: %v", name, err)
	}
	head, body, found := bytes.Cut(wire, []byte("\r\n\r\n"))
	if !found || len(head) != pin.HeaderLength || len(body) != pin.BodyLength || pin.SeparatorLength != 4 || !bytes.Equal(head, []byte(fmt.Sprintf("content-length:\t%d\r\nX-C-Original: successor-v2", len(body)))) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C01 v2 framing %s", name)
	}
	_ = cFixtureV2Message(t, wire, want, manifest.Limits.DefinitionRemarshalMaxBytes, manifest.Limits.ExactResultMaxBytes, false)
	return wire
}

func cCumulativeV2Pins(t *testing.T) (cCumulativeV2Manifest, map[string][][]byte) {
	t.Helper()
	var manifest cCumulativeV2Manifest
	cFixtureV2Manifest(t, cCumulativeV2Root, cCumulativeV2ManifestSHA, &manifest)
	if manifest.Schema != "adr0011-c-cumulative-v2" || manifest.Limits.CompleteFrameCeiling != cFrameCap || manifest.Limits.CurrentAttemptWireMaxBytes != 8388608 || manifest.Limits.DefinitionRemarshalMaxBytes != 1048576 || manifest.Limits.ExactResultMaxBytes != 524288 || manifest.Limits.MessageAttemptCap != 64 {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C04 v2 limits")
	}
	wires := make(map[string][][]byte, 2)
	for _, name := range []string{"at-cap", "plus-one"} {
		pin, ok := manifest.Cases[name]
		wantTotal := manifest.Limits.CurrentAttemptWireMaxBytes
		if name == "plus-one" {
			wantTotal++
		}
		if !ok || len(pin.Frames) > manifest.Limits.MessageAttemptCap || pin.Total != wantTotal {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C04 v2 case %s", name)
		}
		total := 0
		aggregate := sha256.New()
		for i, f := range pin.Frames {
			wire, err := os.ReadFile(filepath.Join(cCumulativeV2Root, f.Path))
			if err != nil || f.Ordinal != i || len(wire) != f.Length || cCumulativeSHA(wire) != f.SHA || f.Length > manifest.Limits.CompleteFrameCeiling {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C04 v2 %s frame %d: %v", name, i, err)
			}
			total += len(wire)
			aggregate.Write(wire)
			if total != f.PrefixTotal {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C04 v2 %s prefix %d", name, i)
			}
			_ = cFixtureV2Message(t, wire, f.Length, manifest.Limits.DefinitionRemarshalMaxBytes, manifest.Limits.ExactResultMaxBytes, i == len(pin.Frames)-1)
			wires[name] = append(wires[name], wire)
		}
		if total != wantTotal || hex.EncodeToString(aggregate.Sum(nil)) != pin.Aggregate {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: C04 v2 %s aggregate", name)
		}
	}
	return manifest, wires
}
