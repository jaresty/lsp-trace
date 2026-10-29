//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/publication"
)

func TestProducerPacketPrivateV2PairFaults(t *testing.T) {
	for _, kind := range []string{"late-exist", "joined-notexist-close", "second-copy", "partial-write", "receipt-close", "committed-error", "corrupt-read", "read-close", "cancel", "source-close", "source-replaced"} {
		t.Run(kind, func(t *testing.T) {
			f, o, captures, _ := packetFixturePrivateV2(t, 1, 0)
			roots, pp, _ := packetRootsPrivateV2(t, f)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("owned packet fault")
			ops := producerPacketRetentionOpsPrivateV2()
			read, publish := ops.read, ops.publish
			reads, writes := 0, 0
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
				reads++
				if kind == "joined-notexist-close" {
					return nil, errors.Join(os.ErrNotExist, sentinel)
				}
				raw, e := read(ctx, r, id, n)
				if reads == 3 {
					if kind == "corrupt-read" && len(raw) > 0 {
						raw[0] ^= 1
					}
					if kind == "read-close" {
						return raw, errors.Join(e, sentinel)
					}
				}
				return raw, e
			}
			ops.publish = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, raw []byte) (*publication.BoundFileReceipt, error) {
				writes++
				if kind == "partial-write" {
					if e := os.WriteFile(filepath.Join(pp, ".owned-partial-packet"), raw[:len(raw)/2], 0600); e != nil {
						t.Fatal(e)
					}
					return nil, sentinel
				}
				if kind == "late-exist" {
					return nil, os.ErrExist
				}
				if kind == "second-copy" && writes == 2 {
					return nil, sentinel
				}
				receipt, e := publish(ctx, r, id, raw)
				if e != nil {
					return receipt, e
				}
				if writes == 1 {
					if kind == "receipt-close" {
						receipt.CloseStatus = publication.CloseFailed
					}
					if kind == "committed-error" {
						return receipt, sentinel
					}
					if kind == "cancel" {
						cancel()
					}
				}
				if writes == 2 {
					switch kind {
					case "source-close":
						if e = roots.state.source.Close(); e != nil {
							t.Fatal(e)
						}
					case "source-replaced":
						path := f.state.spec.Root
						saved := path + "-packet-original"
						if e = os.Rename(path, saved); e != nil {
							t.Fatal(e)
						}
						if e = os.Mkdir(path, 0755); e != nil {
							t.Fatal(e)
						}
						t.Cleanup(func() {
							if e := os.Remove(path); e != nil {
								t.Error(e)
							}
							if e := os.Rename(saved, path); e != nil {
								t.Error(e)
							}
						})
					}
				}
				return receipt, nil
			}
			out, err := retainCompleteScopePrivateV2(ctx, f, o, captures, roots, nil, ops)
			if err == nil || out.VerifiedPair().valid() {
				t.Fatal("ASSERT_PACKET_FAULT_NO_VERIFIED_PAIR", kind, out.Phase(), err)
			}
			if kind == "late-exist" && (writes != 1 || reads != 1) {
				t.Fatal("ASSERT_PACKET_LATE_EXIST_NO_REREAD_RETRY", writes, reads)
			}
			if kind == "joined-notexist-close" && writes != 0 {
				t.Fatal("ASSERT_PACKET_JOINED_ABSENCE_NOT_ADMITTED")
			}
			if kind != "late-exist" && kind != "joined-notexist-close" && kind != "partial-write" {
				r := out.CopyReceipts()[0].Receipt
				if r == nil || !r.NamespaceAtomic {
					t.Fatal("ASSERT_PACKET_COMMITTED_RECEIPT_SURVIVES", kind)
				}
			}
			if kind == "partial-write" && (writes != 1 || out.CopyReceipts()[0].Outcome != "INDETERMINATE") {
				t.Fatal("ASSERT_PACKET_PARTIAL_WRITE_NOT_A_COMMITTED_RECEIPT")
			}
			if writes > 2 {
				t.Fatal("ASSERT_PACKET_NO_PUBLICATION_RETRY")
			}
		})
	}
}
func TestProducerPacketPrivateV2QualificationRejectsChangedCopies(t *testing.T) {
	for _, kind := range []string{"primary-bit", "backup-bit", "hardlink-alias", "read-close", "cancel", "zero-state", "copied-state"} {
		t.Run(kind, func(t *testing.T) {
			f, o, captures, _ := packetFixturePrivateV2(t, 1, 0)
			roots, pp, bp := packetRootsPrivateV2(t, f)
			out, err := RetainCompleteScopePrivateV2(context.Background(), f, o, captures, roots)
			if err != nil {
				t.Fatal(err)
			}
			pair := out.VerifiedPair()
			id := pair.state.digest
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ops := producerPacketRetentionOpsPrivateV2()
			normal := ops.read
			calls := 0
			switch kind {
			case "primary-bit", "backup-bit":
				path := pp
				if kind == "backup-bit" {
					path = bp
				}
				raw := bytes.Clone(pair.state.raw)
				raw[len(raw)-1] ^= 1
				if err = os.WriteFile(filepath.Join(path, id), raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "hardlink-alias":
				if err = os.Remove(filepath.Join(bp, id)); err != nil {
					t.Fatal(err)
				}
				if err = os.Link(filepath.Join(pp, id), filepath.Join(bp, id)); err != nil {
					t.Fatal(err)
				}
			case "read-close":
				ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
					raw, e := normal(ctx, r, id, n)
					return raw, errors.Join(e, errors.New("owned close failure"))
				}
			case "cancel":
				cancel()
			case "zero-state":
				pair = VerifiedProducerPacketPairPrivateV2{}
			case "copied-state":
				copy := *pair.state
				pair = VerifiedProducerPacketPairPrivateV2{&copy}
			}
			wrapped := ops.read
			ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
				calls++
				return wrapped(ctx, r, id, n)
			}
			q, err := qualifyProducerPacketPrivateV2(ctx, pair, ops)
			if err == nil || q.valid() {
				t.Fatal("ASSERT_PACKET_INDEPENDENT_QUALIFIER_REFUSES", kind)
			}
			if calls > 2 {
				t.Fatal("ASSERT_PACKET_QUALIFICATION_NO_RETRY")
			}
		})
	}
}
func TestProducerPacketPrivateV2InspectorMutations(t *testing.T) {
	f, o, captures, _ := packetFixturePrivateV2(t, 2, 1)
	budget, _ := newProducerPacketBudgetPrivateV2(nil)
	raw, err := buildProducerPacketPrivateV2(context.Background(), f, o, captures, budget)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing-role", "role-index", "spec-bit", "body-bit", "body-identity", "body-mode", "uri", "role", "language", "file-plan", "policy", "missing-batch", "duplicate-batch", "ordinal", "seeds", "capture-origin", "source-policy", "request-context", "native-bit", "terminal", "origin-observations", "file-membership"} {
		t.Run(kind, func(t *testing.T) {
			p, e := PureParseProducerPacketPrivateV2(raw)
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "missing-role":
				p.Roles = p.Roles[:len(p.Roles)-1]
			case "role-index":
				p.Roles[0].Index++
			case "spec-bit":
				p.FrozenSpec[0] ^= 1
			case "body-bit":
				p.Files[0].Body[0] ^= 1
			case "body-identity":
				p.Files[0].Record.Identity.Inode++
			case "body-mode":
				p.Files[0].Record.Mode++
			case "uri":
				p.Files[0].Record.URI = "file:///foreign.go"
			case "role":
				p.Files[0].Record.Role = "CONTEXT_ONLY"
			case "language":
				var s ScopeSpecPrivateV2
				_ = json.Unmarshal(p.FrozenSpec, &s)
				s.Members[0].LanguageID = "foreign"
				p.FrozenSpec, _ = json.Marshal(s)
			case "file-plan":
				p.FilePlan = []byte(`{}`)
			case "policy":
				p.EffectivePolicy = []byte(`{}`)
			case "missing-batch":
				p.Batches = p.Batches[:len(p.Batches)-1]
			case "duplicate-batch":
				p.Batches[1] = p.Batches[0]
			case "ordinal":
				p.Batches[0].Ordinal++
			case "seeds":
				p.Batches[0].Seeds = []byte("substituted seed")
			case "capture-origin":
				p.Batches[0].Origin = []byte(`{}`)
			case "source-policy", "request-context", "native-bit":
				source, e := graphprovenance.ParseSourceWithholdingPrivateV2(p.Batches[0].SourceArtifact)
				if e != nil {
					t.Fatal(e)
				}
				switch kind {
				case "source-policy":
					source.Policy.Members[0].SHA256 = Digest([]byte("foreign"))
				case "request-context":
					source.Evidence.Acquisition.Request.Context.ID = "foreign"
				case "native-bit":
					source.Evidence.GraphBytes[0] ^= 1
				}
				altered, e := json.Marshal(source)
				if e != nil {
					altered = []byte(`{"invalid":"owned tamper"}`)
				}
				p.Batches[0].SourceArtifact = append(altered, '\n')
				var co captureOriginFramePrivateV2
				_ = json.Unmarshal(p.Batches[0].Origin, &co)
				co.ArtifactSHA256 = Digest(p.Batches[0].SourceArtifact)
				co.ArtifactByteLength = uint64(len(p.Batches[0].SourceArtifact))
				p.Batches[0].Origin, _ = json.Marshal(co)
			case "terminal":
				p.Batches[0].ReleasedObserved = false
			case "origin-observations", "file-membership":
				tamperCanonicalPacketDiscoveryPrivateV2(t, &p, kind)
			}
			if kind != "missing-role" && kind != "role-index" {
				p.Roles = producerPacketRolesPrivateV2(&p)
			}
			encoded, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = PureParseProducerPacketPrivateV2(encoded); e == nil {
				t.Fatal("ASSERT_PACKET_INSPECTOR_REFUSES_TAMPER", kind)
			}
		})
	}
}

// The semantic negatives must first pass the actual canonical decoder. Repair
// every outer commitment so neither hash drift nor JSON field order can mask the
// intended validator. Direct calls and one-field repair controls localize the gap.
func tamperCanonicalPacketDiscoveryPrivateV2(t *testing.T, p *ProducerPacketInspectionPrivateV2, kind string) {
	t.Helper()
	var frame originFramePrivateV2
	var spec ScopeSpecPrivateV2
	var discovery censusacquisition.Discovery
	if err := producerPacketStrictPrivateV2(p.EnumerationOrigin, &frame); err != nil {
		t.Fatal(err)
	}
	if err := producerPacketStrictPrivateV2(p.FrozenSpec, &spec); err != nil {
		t.Fatal(err)
	}
	if err := producerPacketStrictPrivateV2(frame.Enumeration, &discovery); err != nil {
		t.Fatal(err)
	}
	originalEnumeration := bytes.Clone(frame.Enumeration)
	var nominations []censusacquisition.SourceFile
	for i, file := range p.Files {
		if file.Record.Role == "NOMINATION" {
			nominations = append(nominations, censusacquisition.SourceFile{Path: file.Record.Path, URI: file.Record.URI, LanguageID: spec.Members[i].LanguageID})
		}
	}
	if err := validatePacketDiscoveryPrivateV2(discovery, nominations, spec.EffectivePolicy.Discovery.MaxSymbols); err != nil {
		t.Fatal("ASSERT_PACKET_DISCOVERY_BASELINE_VALID", err)
	}
	originalIdentity := discovery.FileLedger.Entries[0].Identity
	originalObservations := discovery.Observations
	if kind == "origin-observations" {
		discovery.Observations = []censusacquisition.DiscoveryObservation{{Dimension: censusacquisition.DiscoveryNodeCeiling, Known: true}}
	} else {
		discovery.FileLedger.Entries[0].Identity = "foreign.go"
	}
	// Marshal the actual descriptor, never a map: producer field order survives.
	changed, err := json.Marshal(discovery)
	if err != nil {
		t.Fatal(err)
	}
	var canonical censusacquisition.Discovery
	if err = producerPacketStrictPrivateV2(changed, &canonical); err != nil {
		t.Fatal("ASSERT_PACKET_DISCOVERY_MUTATION_CANONICAL", kind, err)
	}
	t.Log("ASSERT_PACKET_DISCOVERY_MUTATION_CANONICAL", kind, "PASS")
	planned, err := (censusacquisition.Core{Planning: &spec.EffectivePolicy.Planning}).PlanDiscovery(canonical.Session, canonical)
	if err != nil || !reflect.DeepEqual(planned, frame.PlannedBatches) {
		t.Fatal("ASSERT_PACKET_DISCOVERY_MUTATION_TARGET_PLAN_UNCHANGED", kind, err)
	}

	var repaired censusacquisition.Discovery
	if err = producerPacketStrictPrivateV2(changed, &repaired); err != nil {
		t.Fatal(err)
	}
	if kind == "origin-observations" {
		repaired.Observations = originalObservations
	} else {
		repaired.FileLedger.Entries[0].Identity = originalIdentity
	}
	repairedBytes, err := json.Marshal(repaired)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(repairedBytes, originalEnumeration) {
		t.Fatal("ASSERT_PACKET_DISCOVERY_ONLY_INTENDED_FIELD_CHANGED", kind)
	}
	if err = validatePacketDiscoveryPrivateV2(repaired, nominations, spec.EffectivePolicy.Discovery.MaxSymbols); err != nil {
		t.Fatal("ASSERT_PACKET_DISCOVERY_ONE_FIELD_REPAIR_VALID", kind, err)
	}

	semanticErr := validatePacketDiscoveryPrivateV2(canonical, nominations, spec.EffectivePolicy.Discovery.MaxSymbols)
	if kind == "origin-observations" {
		terminalErr := censusacquisition.ValidateTerminalDiscovery(canonical, censusacquisition.TerminalDiscoveryOutcome(canonical))
		if terminalErr == nil {
			t.Fatal("ASSERT_PACKET_DISCOVERY_OBSERVATION_TERMINAL_CONTRADICTION")
		}
		if semanticErr == nil {
			t.Error("ASSERT_PACKET_DISCOVERY_OBSERVATIONS_SEMANTIC_REJECTION")
		} else if semanticErr.Error() != terminalErr.Error() {
			t.Fatal("ASSERT_PACKET_DISCOVERY_OBSERVATION_ERROR_ORIGIN", semanticErr, terminalErr)
		}
	} else {
		if err = censusacquisition.ValidateTerminalDiscovery(canonical, censusacquisition.TerminalDiscoveryOutcome(canonical)); err != nil {
			t.Fatal("ASSERT_PACKET_DISCOVERY_MEMBERSHIP_UPSTREAM_VALID", err)
		}
		if !errors.Is(semanticErr, ErrInvalid) {
			t.Error("ASSERT_PACKET_DISCOVERY_FILE_MEMBERSHIP_SEMANTIC_REJECTION", semanticErr)
		}
	}

	replacePacketEnumerationPrivateV2(t, p, frame, changed)
	// A positive packet-level control restores ONLY the semantic change and its
	// commitments. This also checks that the origin repair itself is sufficient.
	badPacket, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var control ProducerPacketInspectionPrivateV2
	if err = producerPacketStrictPrivateV2(badPacket, &control); err != nil {
		t.Fatal(err)
	}
	replacePacketEnumerationPrivateV2(t, &control, frame, repairedBytes)
	controlBytes, err := json.Marshal(control)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PureParseProducerPacketPrivateV2(controlBytes); err != nil {
		t.Fatal("ASSERT_PACKET_DISCOVERY_REPAIRED_PACKET_VALID", kind, err)
	}
	if kind == "origin-observations" {
		if _, err = PureParseProducerPacketPrivateV2(badPacket); semanticErr != nil && (err == nil || err.Error() != semanticErr.Error()) {
			t.Fatal("ASSERT_PACKET_DISCOVERY_PACKET_REACHES_OBSERVATION_SEMANTICS", err, semanticErr)
		}
	}
}

func replacePacketEnumerationPrivateV2(t *testing.T, p *ProducerPacketInspectionPrivateV2, frame originFramePrivateV2, enumeration []byte) {
	t.Helper()
	frame.Enumeration = bytes.Clone(enumeration)
	frame.EnumerationSHA256 = Digest(enumeration)
	var err error
	p.EnumerationOrigin, err = json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	for i := range p.Batches {
		var origin captureOriginFramePrivateV2
		if err = producerPacketStrictPrivateV2(p.Batches[i].Origin, &origin); err != nil {
			t.Fatal(err)
		}
		origin.EnumerationOrigin = bytes.Clone(p.EnumerationOrigin)
		origin.EnumerationOriginSHA256 = Digest(p.EnumerationOrigin)
		p.Batches[i].Origin, err = json.Marshal(origin)
		if err != nil {
			t.Fatal(err)
		}
	}
	p.Roles = producerPacketRolesPrivateV2(p)
}

func TestProducerPacketPrivateV2BudgetArithmetic(t *testing.T) {
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	if e := b.reserve(math.MaxUint64, 0, 0); e != ErrProducerPacketOverBoundPrivateV2 || b.spent.LogicalBytes != 0 {
		t.Fatal("ASSERT_PACKET_OVERFLOW_REFUSED", e)
	}
	if _, e := newProducerPacketBudgetPrivateV2([]ProducerPacketLimitsPrivateV2{{ReadHashBytes: math.MaxUint64}}); e != ErrProducerPacketOverBoundPrivateV2 {
		t.Fatal("ASSERT_PACKET_PROFILE_CANNOT_INCREASE", e)
	}
	if _, e := PureParseProducerPacketPrivateV2(make([]byte, producerPacketBytesPrivateV2+1)); e != ErrProducerPacketOverBoundPrivateV2 {
		t.Fatal("ASSERT_PACKET_WIRE_ONE_OVER", e)
	}
	p := ProducerPacketInspectionPrivateV2{FrozenSpec: make([]byte, 12<<20), Files: []ProducerPacketFilePrivateV2{}, Batches: []ProducerPacketBatchPrivateV2{}}
	if _, e := producerPacketEncodedLengthPrivateV2(p); e != ErrProducerPacketOverBoundPrivateV2 {
		t.Fatal("ASSERT_PACKET_BASE64_EXPANSION_INCLUDED", e)
	}
}
