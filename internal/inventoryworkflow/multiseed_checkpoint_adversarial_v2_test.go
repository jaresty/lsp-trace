package inventoryworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
)

type wrongControlledImplementationReaderV2 struct {
	controlledCheckpointImplementationReaderV2
}

func (r wrongControlledImplementationReaderV2) ReadImplementationFileV2(name string, limit uint64) ([]byte, error) {
	raw, err := r.controlledCheckpointImplementationReaderV2.ReadImplementationFileV2(name, limit)
	if err == nil && len(raw) > 0 {
		raw[0] ^= 1
	}
	return raw, err
}
func controlledArtifactBindingV2(schema, identity string, raw []byte) programcrepresentative.ArtifactBindingDTO {
	return programcrepresentative.ArtifactBindingDTO{SchemaVersion: schema, Identity: identity, SHA256: workflowDigest(raw), ByteLength: uint64(len(raw))}
}
func rebindControlledResultV2(t *testing.T, r programcrepresentative.ResultV2) (programcrepresentative.ArtifactBindingDTO, []byte) {
	t.Helper()
	encode := func(v any) []byte {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	r.Composition.Identity = ""
	r.Composition.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-representative-composition:v2", encode(r.Composition))
	cb := controlledArtifactBindingV2(r.Composition.SchemaVersion, r.Composition.Identity, encode(r.Composition))
	r.Grouping.Composition = cb
	r.Grouping.Identity = ""
	r.Grouping.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-representative-grouping:v2", encode(r.Grouping))
	gb := controlledArtifactBindingV2(r.Grouping.SchemaVersion, r.Grouping.Identity, encode(r.Grouping))
	r.Qualification.Composition = cb
	r.Qualification.Grouping = gb
	r.Qualification.Envelope = r.Composition.QualificationEnvelope
	r.Qualification.Identity = ""
	r.Qualification.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-representative-derived-qualification:v2", encode(r.Qualification))
	r.Identity = ""
	r.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-representative-result:v2", encode(r))
	raw := encode(r)
	return controlledArtifactBindingV2(r.SchemaVersion, r.Identity, raw), raw
}

func assertControlledPriorSpliceV2(t *testing.T, base MultiSeedCheckpointRequestV2, stateDir string, priorRaw []byte) {
	t.Helper()
	ctx := context.Background()
	root, err := base.Workflow.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	current, err := heads.Load(ctx, base.Workflow.SuccessorHeadKey)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := inventorysuccessor.ParseHeadRevision(priorRaw)
	if err != nil {
		t.Fatal(err)
	}
	oldKey, _ := json.Marshal(base.Workflow.SuccessorWorkflowKey)
	foreignKey, _ := json.Marshal("foreign-" + base.Workflow.SuccessorWorkflowKey)
	changed := bytes.Replace(priorRaw, append([]byte(`"workflow_key":`), oldKey...), append([]byte(`"workflow_key":`), foreignKey...), 1)
	changed = bytes.Replace(changed, []byte(`"selector":"`+revision.Selector+`"`), []byte(`"selector":""`), 1)
	selector := workflowDigest(changed)
	changed = bytes.Replace(changed, []byte(`"selector":""`), []byte(`"selector":"`+selector+`"`), 1)
	forgedRevision, err := inventorysuccessor.ParseHeadRevision(changed)
	if err != nil || forgedRevision.Selector != selector {
		t.Fatal("SPLICE_FIXTURE_STRICT_V1_CANONICAL")
	}
	b := controlledArtifactBindingV2("lsp-trace.inventory-successor-head.private.v1", selector, changed)
	if err = objects.Retain(ctx, b, changed); err != nil {
		t.Fatal(err)
	}
	if _, err = objects.Resolve(ctx, b); err != nil {
		t.Fatal(err)
	}
	var successor programcrepresentative.SuccessorHeadV2
	if err = json.Unmarshal(current, &successor); err != nil {
		t.Fatal(err)
	}
	successor.PriorSuccessor = b
	successor.Selector = ""
	unsigned, _ := json.Marshal(successor)
	successor.Selector = workflowDigest(unsigned)
	forged, _ := json.Marshal(successor)
	if _, err = inventorysuccessor.ParseMultiSeedSuccessorHeadRevisionV2(forged); err != nil {
		t.Fatal("SPLICE_FIXTURE_STRICT_V2_CANONICAL")
	}
	path := filepath.Join(stateDir, "inventory-successor", "heads", "private", "v1", strings.TrimPrefix(workflowDigest([]byte(base.Workflow.SuccessorHeadKey)), "sha256:")+".json")
	if err = os.WriteFile(path, forged, 0600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(path, current, 0600); err != nil {
			t.Fatal(err)
		}
	}()
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	before := workflowStateSnapshot(t, stateDir)
	observations := inventorysuccessor.MultiSeedOperationObservationsV2{}
	one := base
	one.Operation = inventorysuccessor.MultiSeedSuccessorOperationV2
	output, err := runMultiSeedCheckpointV2(ctx, one, func(actual inventorysuccessor.MultiSeedOperationObservationsV2) { observations = actual })
	if err == nil || len(output.GapLedger) != 0 || observations.CASAttempts != 0 || observations.ImmutablePublishAttempts != 0 || !reflect.DeepEqual(before, workflowStateSnapshot(t, stateDir)) {
		t.Fatal("HASH_CORRECT_PRIOR_WORKFLOW_SPLICE_REFUSAL")
	}
}

func assertControlledCheckpointAdversariesV2(t *testing.T, base MultiSeedCheckpointRequestV2, stateDir string) {
	t.Helper()
	ctx := context.Background()
	root, err := base.Workflow.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	originals := map[string][]byte{}
	bodies := map[string][]byte{}
	for _, kind := range []string{"denominator", "lineage"} {
		heads, err := inventorysuccessor.NewMultiSeedEvidenceHeadsV2(root, kind)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := heads.Load(ctx, base.Workflow.SuccessorHeadKey)
		if err != nil {
			t.Fatal(err)
		}
		originals[kind] = raw
		var h programcrepresentative.EvidenceHeadV2
		if err = json.Unmarshal(raw, &h); err != nil {
			t.Fatal(err)
		}
		body, err := objects.Resolve(ctx, h.Artifact)
		if err != nil {
			t.Fatal(err)
		}
		bodies[kind] = body
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	cases := []string{"denominator-omission", "run-reorder", "seed-substitution", "community-splice", "constituent-splice", "group-omission", "group-reorder", "group-seed-substitution", "false-singleton", "result-distance", "foreign-qualification", "stale-epoch", "wrong-reviewed-source", "cancel"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			one := base
			one.Operation = inventorysuccessor.MultiSeedSuccessorOperationV2
			kind := ""
			fragment := ""
			callCtx := ctx
			if name == "wrong-reviewed-source" {
				one.ImplementationSource = wrongControlledImplementationReaderV2{base.ImplementationSource.(controlledCheckpointImplementationReaderV2)}
				fragment = "implementation source bytes mismatch"
			} else if name == "cancel" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				callCtx = cancelled
				fragment = "canceled"
			} else {
				root, err := base.Workflow.ProgramC.OpenState()
				if err != nil {
					t.Fatal(err)
				}
				objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
				if err != nil {
					t.Fatal(err)
				}
				var binding programcrepresentative.ArtifactBindingDTO
				var raw []byte
				switch name {
				case "denominator-omission", "run-reorder", "seed-substitution", "community-splice", "constituent-splice":
					kind = "denominator"
					fragment = "does not equal complete reconstructed inventory"
					d, err := programcrepresentative.ParseDenominatorV2(bodies[kind])
					if err != nil {
						t.Fatal(err)
					}
					switch name {
					case "denominator-omission":
						d.Entries = d.Entries[:len(d.Entries)-1]
						d.Accounting.D--
						d.Accounting.G -= 2
					case "run-reorder":
						d.Constituents[0].Runs[0], d.Constituents[0].Runs[1] = d.Constituents[0].Runs[1], d.Constituents[0].Runs[0]
					case "seed-substitution":
						d.Entries[0].Seed = 2
					case "community-splice":
						d.Entries[0].Mechanics = d.Entries[1].Mechanics
					case "constituent-splice":
						for _, entry := range d.Entries {
							if entry.ConstituentOrdinal != d.Entries[0].ConstituentOrdinal {
								d.Entries[0].Custody = entry.Custody
								break
							}
						}
					}
					for i := range d.Entries {
						d.Entries[i], _, _, err = programcrepresentative.EncodeDenominatorEntryV2(d.Entries[i])
						if err != nil {
							t.Fatal(err)
						}
					}
					_, binding, raw, err = programcrepresentative.EncodeDenominatorV2(d)
					if err != nil {
						t.Fatal(err)
					}
				default:
					kind = "lineage"
					fragment = "v2 retained result refusal"
					l, err := programcrepresentative.ParseLineageV2(bodies[kind])
					if err != nil {
						t.Fatal(err)
					}
					resultRaw, err := objects.Resolve(ctx, l.Records[0].Result)
					if err != nil {
						t.Fatal(err)
					}
					result, err := programcrepresentative.ParseMultiSeedResultV2(resultRaw)
					if err != nil {
						t.Fatal(err)
					}
					r, err := result.DTO()
					if err != nil {
						t.Fatal(err)
					}
					switch name {
					case "group-omission":
						r.GroupResults = r.GroupResults[:1]
					case "group-reorder":
						r.GroupResults[0], r.GroupResults[1] = r.GroupResults[1], r.GroupResults[0]
					case "group-seed-substitution":
						r.GroupResults[0].SeedGroupIdentity = r.GroupResults[1].SeedGroupIdentity
					case "result-distance":
						r.GroupResults[0].Record.Distance++
					case "stale-epoch":
						r.Qualification.ValidFrom--
						r.Qualification.ValidThrough--
						fragment = "foreign qualification/context"
					case "foreign-qualification":
						b := r.Composition.QualificationEnvelope
						b.Identity = workflowDigest([]byte("controlled foreign envelope identity"))
						b.SHA256 = workflowDigest([]byte("controlled foreign envelope bytes"))
						r.Composition.QualificationEnvelope = b
						fragment = "foreign qualification/context"
					case "false-singleton":
						m := &r.Composition.Mechanics
						m.SelectionContext.SeedGroups = m.SelectionContext.SeedGroups[:1]
						m.SelectionContext.Identity = ""
						body, _ := json.Marshal(m.SelectionContext)
						m.SelectionContext.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-stage6-context:v2", body)
						m.Identity = ""
						body, _ = json.Marshal(m)
						m.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-stage6-mechanics:v2", body)
						r.Grouping.SelectionContext = m.SelectionContext
						r.GroupResults = r.GroupResults[:1]
						r.GroupAccounting = programcrepresentative.GroupAccountingV2{Expected: 1}
						switch r.GroupResults[0].Record.Outcome {
						case "SELECTED":
							r.GroupAccounting.Selected = 1
						case "NO_CALLS_REPRESENTATIVE":
							r.GroupAccounting.NoCalls = 1
						case "CALLS_EVIDENCE_INELIGIBLE":
							r.GroupAccounting.CallsIneligible = 1
						}
						fragment = "foreign qualification/context"
					}
					rb, body := rebindControlledResultV2(t, r)
					if err = objects.Retain(ctx, rb, body); err != nil {
						t.Fatal(err)
					}
					l.Records[0].Result = rb
					l.Records[0], _, _, err = programcrepresentative.EncodeLineageRecordV2(l.Records[0])
					if err != nil {
						t.Fatal(err)
					}
					_, binding, raw, err = programcrepresentative.EncodeLineageV2(l)
					if err != nil {
						t.Fatal(err)
					}
				}
				if err = objects.Retain(ctx, binding, raw); err != nil {
					t.Fatal(err)
				}
				var head programcrepresentative.EvidenceHeadV2
				if err = json.Unmarshal(originals[kind], &head); err != nil {
					t.Fatal(err)
				}
				head.Artifact = binding
				if kind == "denominator" {
					head.Denominator = binding
				}
				head.Selector = ""
				unsigned, _ := json.Marshal(head)
				head.Selector = workflowDigest(unsigned)
				changed, _ := json.Marshal(head)
				path := filepath.Join(stateDir, "program-c-representative", kind, "heads", "private", "v2", strings.TrimPrefix(workflowDigest([]byte(base.Workflow.SuccessorHeadKey)), "sha256:")+".json")
				if err = os.WriteFile(path, changed, 0600); err != nil {
					t.Fatal(err)
				}
				if err = root.Close(); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.WriteFile(path, originals[kind], 0600); err != nil {
						t.Fatal(err)
					}
				}()
			}
			before := workflowStateSnapshot(t, stateDir)
			observations := inventorysuccessor.MultiSeedOperationObservationsV2{}
			output, err := runMultiSeedCheckpointV2(callCtx, one, func(actual inventorysuccessor.MultiSeedOperationObservationsV2) { observations = actual })
			if err == nil || !strings.Contains(err.Error(), fragment) || len(output.GapLedger) != 0 || output.Head.Generation != 0 || observations.CASAttempts != 0 || observations.ImmutablePublishAttempts != 0 {
				t.Fatalf("RESEALED_CHECKPOINT_REFUSAL_AT_VALIDATOR case=%s wanted=%q err=%v observations=%+v", name, fragment, err, observations)
			}
			if !reflect.DeepEqual(before, workflowStateSnapshot(t, stateDir)) {
				t.Fatal("REFUSAL_ZERO_WRITES_NO_PARTIAL_EXPOSURE")
			}
		})
	}
}
