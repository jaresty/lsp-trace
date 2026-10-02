package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"lsp-trace/internal/adr0011cobserve"
	"lsp-trace/internal/lspwire"
)

// P0 extraction of the unchanged historical helper declarations and bodies.
// Exact originals and the original manifest are package-local; relocation.json
// separately binds their source identity. Missing or changed originals block runs.
const cCumulativeManifestSHA = "b44515d1f806842069824b1c8e7039edc9e080a020a64dcbfc7352acdeaad61e"
const cCumulativeRoot = "testdata/adr0011-c-cumulative-v1"

type cCumulativeFramePin struct {
	Path            string `json:"path"`
	Ordinal         int    `json:"ordinal"`
	Length          int    `json:"length"`
	HeaderLength    int    `json:"header_length"`
	SeparatorLength int    `json:"separator_length"`
	BodyLength      int    `json:"body_length"`
	PrefixTotal     int    `json:"prefix_total"`
	SHA             string `json:"sha256"`
	Role            string `json:"role"`
}
type cCumulativeCasePin struct {
	Frames    []cCumulativeFramePin `json:"frames"`
	Total     int                   `json:"total"`
	Aggregate string                `json:"ordered_aggregate_sha256"`
}
type cCumulativeManifest struct {
	Schema               string                        `json:"schema"`
	FixtureHead          string                        `json:"fixture_head"`
	Cases                map[string]cCumulativeCasePin `json:"cases"`
	FixtureRequestParams struct {
		Length int    `json:"length"`
		SHA    string `json:"sha256"`
	} `json:"fixture_request_params"`
	FixtureRequestFrame struct {
		Length int    `json:"length"`
		SHA    string `json:"sha256"`
	} `json:"fixture_request_frame"`
	FixtureResponseFrame struct {
		Length int    `json:"length"`
		SHA    string `json:"sha256"`
	} `json:"fixture_response_frame"`
	Limits struct {
		MaxBytes                               int64 `json:"MaxBytes"`
		MaxMessages                            int   `json:"MaxMessages"`
		CaptureDefinitionResponseFrameMaxBytes int64 `json:"CaptureDefinitionResponseFrameMaxBytes"`
		CaptureMethodRequestFrameMaxBytes      int64 `json:"CaptureMethodRequestFrameMaxBytes"`
		CompleteFrameCeiling                   int   `json:"complete_frame_ceiling"`
		ReservationBytes                       int64 `json:"reservation_bytes"`
		ReservationCeiling                     int64 `json:"reservation_ceiling"`
	} `json:"limits"`
}

func cCumulativeSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func cCumulativePins(t *testing.T) (cCumulativeManifest, map[string][][]byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(cCumulativeRoot, "manifest.json"))
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: manifest read: %v", err)
	}
	if cCumulativeSHA(raw) != cCumulativeManifestSHA {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: manifest SHA")
	}
	var manifest cCumulativeManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: manifest JSON: %v", err)
	}
	if manifest.Schema != "adr0011-c-cumulative-original-wire-v1" || manifest.FixtureHead != "7995dfd0f55c450bf7b69601dc53e948c2a8fa5c" || len(manifest.Cases) != 2 || manifest.Limits.MaxBytes != 4194304 || manifest.Limits.MaxMessages != 6 || manifest.Limits.CaptureDefinitionResponseFrameMaxBytes != 163 || manifest.Limits.CaptureMethodRequestFrameMaxBytes != 178 || manifest.Limits.CompleteFrameCeiling != 2097152 || manifest.Limits.ReservationBytes != 8389974 || manifest.Limits.ReservationCeiling != 16777216 || 2*(manifest.Limits.MaxBytes+163+178+86+256) != manifest.Limits.ReservationBytes || manifest.FixtureRequestParams.Length != 86 || manifest.FixtureRequestParams.SHA != "adc9520f602074f012824ccd016ff7627d247879297da5c74e517f925327172d" || manifest.FixtureRequestFrame.Length != 178 || manifest.FixtureRequestFrame.SHA != "a985bd69c4392c1cdc8970813c1d3743abd645a7568aef3c4370e5730517b849" || manifest.FixtureResponseFrame.Length != 163 || manifest.FixtureResponseFrame.SHA != "c6e6819ef4de0da57f5794395f4b5850e5aa04f44085da8b8fb98042d0dec5a0" {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: manifest limits/request pins")
	}
	expectedSHA := [6]string{"9aa9b622f5778c82f7f469f381405d0bf4227eacfa2b351f5a44068ade7f34e2", "58967be486f466020bc6bc270ee723c8474993127bfe9e0f5309755a3a9699ab", "86173f36bc69e92b88dafd15e837a44ded43f659d3652ded5e060f5403ee8301", "e15e0faf4aa8f4785282bc83bbb1cb3a218095664cd9bb7d98eaada2256d1b8a", "6dc1ffc415a700fca455f2a9391bb4a3d495ff3e0529815c33ea4bba0b070e3c", "c6e6819ef4de0da57f5794395f4b5850e5aa04f44085da8b8fb98042d0dec5a0"}
	expectedLength := [6]int{2097072, 2097072, 2097072, 2097072, 157, 163}
	expectedPrefix := [6]int{2097072, 4194144, 6291216, 8388288, 8388445, 8388608}
	originals := make(map[string][][]byte, 2)
	for _, name := range []string{"at-cap", "plus-one"} {
		pin, ok := manifest.Cases[name]
		if !ok || len(pin.Frames) != 6 {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame count", name)
		}
		total, wires := 0, make([][]byte, 0, 6)
		aggregate := sha256.New()
		for i, f := range pin.Frames {
			wantSHA, wantLen, wantPrefix := expectedSHA[i], expectedLength[i], expectedPrefix[i]
			if name == "plus-one" && i == 5 {
				wantSHA = "669b01d515a27abbb1cd9b3f998912927dc33e872580ff99289f72fcc23de7e1"
				wantLen++
				wantPrefix++
			}
			role := "notification"
			if i == 5 {
				role = "matched_definition_response"
			}
			if f.Ordinal != i || f.Path != fmt.Sprintf("%s/%02d.frame", name, i) || f.SHA != wantSHA || f.Length != wantLen || f.PrefixTotal != wantPrefix || f.Role != role || f.SeparatorLength != 4 || f.HeaderLength+4+f.BodyLength != wantLen || f.Length > 2097152 {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame %d manifest pin", name, i)
			}
			wire, err := os.ReadFile(filepath.Join(cCumulativeRoot, f.Path))
			if err != nil {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame %d read: %v", name, i, err)
			}
			head, body, found := bytes.Cut(wire, []byte("\r\n\r\n"))
			if len(wire) != wantLen || cCumulativeSHA(wire) != wantSHA || !found || len(head) != f.HeaderLength || len(body) != f.BodyLength || !bytes.Equal(head, []byte(fmt.Sprintf("Content-Length: %d", len(body)))) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame %d original", name, i)
			}
			msg, observed, _, _, err := lspwire.NewReader(bytes.NewReader(wire), lspwire.DefaultLimits()).ReadWithFrameIfWithin(163)
			if err != nil || observed.FrameBytes != int64(wantLen) || (i < 5 && msg.Method != "$/progress") || (i == 5 && (string(msg.ID) != "1" || len(msg.Result) == 0)) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s frame %d valid decode: %v", name, i, err)
			}
			total += len(wire)
			aggregate.Write(wire)
			wires = append(wires, wire)
		}
		wantTotal, wantAggregate := 8388608, "1416b181797fc5d1c66135a6390d593f9c72138da8bfcd49f810eb79c6880202"
		if name == "plus-one" {
			wantTotal = 8388609
			wantAggregate = "b942b9a85d70b53c7c52378d0ae9abded5602d96e66f2e70c7dd157669c45902"
		}
		if total != wantTotal || pin.Total != wantTotal || hex.EncodeToString(aggregate.Sum(nil)) != wantAggregate || pin.Aggregate != wantAggregate {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s aggregate", name)
		}
		originals[name] = wires
	}
	return manifest, originals
}

type cCumulativeMarkers struct {
	mu        sync.Mutex
	decoded   int
	decode    [6]int
	retained  [6]int
	b4        int
	publish   int
	ambiguous bool
}

func (p *cCumulativeMarkers) observe(stage adr0011cobserve.Stage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch stage {
	case adr0011cobserve.RetainEntry:
		if p.decoded >= 6 {
			p.ambiguous = true
		} else {
			p.retained[p.decoded]++
		}
	case adr0011cobserve.DecodeEntry:
		if p.decoded >= 6 {
			p.ambiguous = true
		} else {
			p.decode[p.decoded]++
			p.decoded++
		}
	case adr0011cobserve.B4EvaluationEntry:
		p.b4++
	case adr0011cobserve.PrivateCandidatePublishEntry:
		p.publish++
	default:
		p.ambiguous = true
	}
}
func (p *cCumulativeMarkers) snapshot() cCumulativeMarkers {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cCumulativeMarkers{decoded: p.decoded, decode: p.decode, retained: p.retained, b4: p.b4, publish: p.publish, ambiguous: p.ambiguous}
}

type cCumulativeDelivery struct {
	bytes [6]int
	errs  [6]error
}

func cCumulativeChild(inFrame []byte, wires [][]byte) (*composedChild, <-chan cCumulativeDelivery) {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
	delivered := make(chan cCumulativeDelivery, 1)
	go func() {
		msg, _, frame, retained, err := lspwire.NewReader(input, lspwire.DefaultLimits()).ReadWithFrameIfWithin(4096)
		if err != nil || !retained || !bytes.Equal(frame, inFrame) || msg.Method != "textDocument/definition" || string(msg.ID) != "1" {
			child.observed <- fmt.Errorf("exact WRITE: %v", err)
			return
		}
		child.observed <- nil
		var d cCumulativeDelivery
		for i, wire := range wires {
			d.bytes[i], d.errs[i] = output.Write(wire)
			if d.errs[i] != nil {
				break
			}
		}
		delivered <- d
	}()
	return child, delivered
}
