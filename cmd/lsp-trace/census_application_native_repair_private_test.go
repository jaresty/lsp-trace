//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func TestCensusApplicationRepairReportBindingsPrivate(t *testing.T) {
	original, report, _, _ := applicationMockCompletedPrivate(t)
	for _, kind := range []string{"feature", "source", "absence", "policy", "outcome-ceiling", "outcome-preparation", "copy-reference", "controls"} {
		t.Run(kind, func(t *testing.T) {
			s, r, reseal := applicationRepairClonePrivate(t, original, report)
			raw, _ := json.Marshal(r)
			var negative censusInventoryReportPrivate
			if e := json.Unmarshal(raw, &negative); e != nil {
				t.Fatal(e)
			}
			r = negative
			switch kind {
			case "feature":
				r.FeatureIdentity = "ACCEPTED"
			case "source":
				r.SourceCompleteness = "COMPLETE"
			case "absence":
				r.AbsenceInference = "ALLOWED"
			case "policy":
				r.Outcome.PolicySHA256 = "foreign"
			case "outcome-ceiling":
				r.Outcome.Workflow = "COMPLETE"
			case "outcome-preparation":
				r.Outcome.PreparationSHA256 = "foreign"
			case "copy-reference":
				r.Outcome.Copies[1].Receipt.Digest = "foreign"
			case "controls":
				r.Excludes = []string{"foreign"}
			}
			reseal(r)
			_, e := s.verify(0)
			assertInventoryPrivate(t, "ASSERT_REPAIR_REPORT_BINDING_"+kind, e != nil)
			t.Logf("report-target kind=%s observed=%v", kind, e)
		})
	}
}

func TestCensusApplicationRepairNativePrivate(t *testing.T) {
	runCensusApplicationRepairNativePrivate(t, []string{"source-completeness", "absence", "ordinary-native", "describe", "workflow", "policy", "unknown", "duplicate", "omitted", "ledger-family", "ledger-ceiling", "inventory-ceiling", "preparation-family", "preparation-limits", "discovery-binding", "part-chain", "part-hash", "part-family", "preparation-unknown", "preparation-duplicate"})
}

func TestCensusApplicationRepairSourceRecordsPrivate(t *testing.T) {
	runCensusApplicationRepairNativePrivate(t, []string{"record-source-completeness", "record-absence", "record-foreign", "record-missing", "record-duplicate", "record-handle", "record-kind", "record-unknown", "record-object-duplicate", "record-claim-value", "record-claim-missing", "record-claim-duplicate", "record-claim-handle", "record-claim-kind"})
}

func runCensusApplicationRepairNativePrivate(t *testing.T, kinds []string) {
	original, report, _, _ := applicationMockCompletedPrivate(t)
	template, _ := json.Marshal(report)
	t.Cleanup(func() {
		after, _ := json.Marshal(report)
		if !bytes.Equal(template, after) {
			t.Fatal("original template changed")
		}
	})
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			s, r, reseal := applicationRepairClonePrivate(t, original, report)
			// Detach all report pointers before altering negative-fixture hash bindings.
			detached, _ := json.Marshal(r)
			var negative censusInventoryReportPrivate
			if e := json.Unmarshal(detached, &negative); e != nil {
				t.Fatal(e)
			}
			r = negative
			names, e := os.ReadDir(filepath.Join(r.AttemptDirectory, "inventory-0"))
			if e != nil || len(names) != 1 {
				t.Fatal("native fixture setup", e)
			}
			raw, e := os.ReadFile(filepath.Join(r.AttemptDirectory, "inventory-0", names[0].Name()))
			if e != nil {
				t.Fatal(e)
			}
			if _, _, err := prep.ValidateAutomaticPublicationObservationPrivateV2(s.store.ctx, raw, s.plan.Configuration.Preparation, s.plan.Configuration.Post); err != nil {
				t.Fatal("helper control refused", err)
			}
			replace := func(old, new string) {
				t.Helper()
				if !bytes.Contains(raw, []byte(old)) {
					t.Fatalf("missing mutation token %s", old)
				}
				raw = bytes.Replace(raw, []byte(old), []byte(new), 1)
			}
			switch kind {
			case "source-completeness":
				replace(`"SourceCompleteness":"UNKNOWN"`, `"SourceCompleteness":"COMPLETE"`)
			case "absence":
				replace(`"Absence":"PROHIBITED"`, `"Absence":"ALLOWED"`)
			case "ordinary-native":
				replace(`"OrdinaryNative":"NOT_ATTEMPTED"`, `"OrdinaryNative":"COMPLETE"`)
			case "describe":
				replace(`"Describe":"NOT_ATTEMPTED"`, `"Describe":"COMPLETE"`)
			case "workflow":
				replace(`"Workflow":"NOT_ATTEMPTED"`, `"Workflow":"COMPLETE"`)
			case "policy":
				var v struct{ PolicySHA256 string }
				if e = json.Unmarshal(raw, &v); e != nil {
					t.Fatal(e)
				}
				replace(`"PolicySHA256":"`+v.PolicySHA256+`"`, `"PolicySHA256":"sha256:`+strings.Repeat("0", 64)+`"`)
			case "unknown":
				raw = append(append(bytes.Clone(raw[:len(raw)-1]), []byte(`,"Unknown":null`)...), '}')
			case "duplicate":
				replace(`"SchemaVersion":`, `"SchemaVersion":"ignored","SchemaVersion":`)
			case "omitted":
				replace(`"Accepted":false,`, ``)
			case "ledger-family", "ledger-ceiling", "inventory-ceiling":
				var envelope map[string]json.RawMessage
				if e = json.Unmarshal(raw, &envelope); e != nil {
					t.Fatal(e)
				}
				field := "Ledger"
				if kind == "inventory-ceiling" {
					field = "Inventory"
				}
				old := envelope[field]
				next := bytes.Clone(old)
				if kind == "ledger-family" {
					next = bytes.Replace(next, []byte(prep.RequestTypePrivateV2), []byte("FOREIGN_PREPARATION"), 1)
				} else {
					next = bytes.Replace(next, []byte(`"SourceCompleteness":"UNKNOWN"`), []byte(`"SourceCompleteness":"COMPLETE"`), 1)
				}
				if bytes.Equal(old, next) {
					t.Fatal("mutation not applied")
				}
				raw = bytes.Replace(raw, old, next, 1)
			default:
				var envelope struct{ Preparation json.RawMessage }
				if e = json.Unmarshal(raw, &envelope); e != nil {
					t.Fatal(e)
				}
				var p prep.Result
				if e = json.Unmarshal(envelope.Preparation, &p); e != nil {
					t.Fatal(e)
				}
				switch kind {
				case "preparation-family":
					p.RequestType = "FOREIGN_PREPARATION"
				case "preparation-limits":
					p.Limits.MaxSourceBytes--
				case "discovery-binding":
					p.Coverage[0].DiscoverySHA256 = "sha256:" + strings.Repeat("0", 64)
					r.Coverage[0].DiscoverySHA256 = p.Coverage[0].DiscoverySHA256
					var fields map[string]json.RawMessage
					if e = json.Unmarshal(raw, &fields); e != nil {
						t.Fatal(e)
					}
					var ledger prep.StageLedgerPrivateV2
					if e = json.Unmarshal(fields["Ledger"], &ledger); e != nil {
						t.Fatal(e)
					}
					ledger.Coverage[0].DiscoverySHA256 = p.Coverage[0].DiscoverySHA256
					updated, _ := json.Marshal(ledger)
					raw = bytes.Replace(raw, fields["Ledger"], updated, 1)
				case "part-chain":
					p.Packets[0].Parts[0].Index++
				case "part-hash":
					p.Packets[0].PartSHA256[0] = "sha256:" + strings.Repeat("0", 64)
				case "part-family":
					p.Packets[0].Parts[0].RequestType = "FOREIGN_PREPARATION"
				}
				if strings.HasPrefix(kind, "record-") {
					mutateApplicationSourceRecordPrivate(t, &p, kind)
					// Rebind every part digest occurrence, including ordered submission pins.
					for i := range p.Packets {
						for j, part := range p.Packets[i].Parts {
							b, _ := json.Marshal(part)
							old := p.Packets[i].PartSHA256[j]
							next := pilot.Digest(b)
							p.Packets[i].PartSHA256[j] = next
							// Preparation is replaced separately below; only ancestors change here.
							raw = bytes.ReplaceAll(raw, []byte(old), []byte(next))
						}
					}
					p.PartBytes = 0
					for _, packet := range p.Packets {
						for _, part := range packet.Parts {
							b, _ := json.Marshal(part)
							p.PartBytes += len(b)
						}
					}
					// Restore original preparation span for exact replacement below.
					var fields map[string]json.RawMessage
					if e = json.Unmarshal(raw, &fields); e != nil {
						t.Fatal(e)
					}
					raw = bytes.Replace(raw, fields["Preparation"], envelope.Preparation, 1)
				}
				next, _ := json.Marshal(p)
				if kind == "preparation-unknown" {
					next = append(append(bytes.Clone(next[:len(next)-1]), []byte(`,"Unknown":null`)...), '}')
				}
				if kind == "preparation-duplicate" {
					next = bytes.Replace(next, []byte(`"Authority":0`), []byte(`"Authority":1,"Authority":0`), 1)
				}
				oldSHA := pilot.Digest(envelope.Preparation)
				newSHA := pilot.Digest(next)
				raw = bytes.Replace(raw, envelope.Preparation, next, 1)
				raw = bytes.ReplaceAll(raw, []byte(oldSHA), []byte(newSHA))
				r.PreparationSHA256 = newSHA
				r.PreparationBytes = len(next)
				r.Inventory.PreparationSHA256 = newSHA
				r.Outcome.PreparationSHA256 = newSHA
				if kind == "discovery-binding" || strings.HasPrefix(kind, "record-") {
					var fields map[string]json.RawMessage
					if e = json.Unmarshal(raw, &fields); e != nil {
						t.Fatal(e)
					}
					var ledger prep.StageLedgerPrivateV2
					if e = json.Unmarshal(fields["Ledger"], &ledger); e != nil {
						t.Fatal(e)
					}
					for i := range ledger.Targets {
						row := &ledger.Targets[i]
						if row.Submission != nil {
							b, _ := json.Marshal(row.Submission)
							row.SubmissionSHA256 = pilot.Digest(b)
							if row.Review != nil {
								row.Review.SubmissionSHA256 = row.SubmissionSHA256
							}
						}
					}
					for i := range ledger.Leads {
						lead := &ledger.Leads[i]
						b, _ := json.Marshal(lead.Submission)
						lead.SubmissionSHA256 = pilot.Digest(b)
						lead.Review.SubmissionSHA256 = lead.SubmissionSHA256
					}
					updated, _ := json.Marshal(ledger)
					raw = bytes.Replace(raw, fields["Ledger"], updated, 1)
				}
			}
			digest := pilot.Digest(raw)
			// Retain two separately-created physical files, then reseal all ancestors.
			for j := 0; j < 2; j++ {
				dir := filepath.Join(r.AttemptDirectory, "inventory-"+string(rune('0'+j)))
				old := filepath.Join(dir, names[0].Name())
				if e = os.Remove(old); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(dir, digest), raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			r.Outcome.EnvelopeSHA256 = digest
			r.Outcome.ByteLength = uint64(len(raw))
			for i := range r.Outcome.Copies {
				r.Outcome.Copies[i].Receipt.Digest = digest
				r.Outcome.Copies[i].Receipt.FinalSelector = digest
				r.Outcome.Copies[i].Receipt.ByteLength = uint64(len(raw))
			}
			reseal(r)
			if strings.HasPrefix(kind, "record-") {
				_, _, err := prep.ValidateAutomaticPublicationObservationPrivateV2(s.store.ctx, raw, s.plan.Configuration.Preparation, s.plan.Configuration.Post)
				assertInventoryPrivate(t, "ASSERT_REPAIR_RECORD_HELPER_"+kind, err != nil)
				t.Logf("record-helper kind=%s observed=%v", kind, err)
			}
			_, e = s.verify(0)
			reached := e == nil || strings.Contains(e.Error(), "application native")
			assertInventoryPrivate(t, "ASSERT_REPAIR_NATIVE_"+kind, e != nil && reached)
			t.Logf("native-target kind=%s observed=%v", kind, e)
		})
	}
}

func mutateApplicationSourceRecordPrivate(t *testing.T, p *prep.Result, kind string) {
	t.Helper()
	want := "SOURCE_BINDING_PRIVATE_V2"
	if strings.HasPrefix(kind, "record-claim-") {
		want = "SOURCE_CLAIM"
	}
	for i := range p.Packets[0].Parts {
		part := &p.Packets[0].Parts[i]
		for j := range part.Records {
			rec := &part.Records[j]
			if rec.Kind != want {
				continue
			}
			switch kind {
			case "record-missing", "record-claim-missing":
				part.Records = append(part.Records[:j], part.Records[j+1:]...)
			case "record-duplicate", "record-claim-duplicate":
				part.Records = append(part.Records, *rec)
			case "record-handle", "record-claim-handle":
				rec.Handle += "/foreign"
			case "record-kind", "record-claim-kind":
				rec.Kind = "FOREIGN_CONTROL"
			case "record-claim-value":
				rec.Value = json.RawMessage(`"absence allowed"`)
			case "record-source-completeness", "record-absence", "record-foreign":
				source := p.Coverage[0].PrivateV2.Source
				switch kind {
				case "record-source-completeness":
					source.SourceCompleteness = "COMPLETE"
				case "record-absence":
					source.AbsenceInference = "ALLOWED"
				case "record-foreign":
					source.PacketSHA256 = "sha256:" + strings.Repeat("0", 64)
				}
				rec.Value, _ = json.Marshal(source)
			case "record-unknown":
				rec.Value = append(bytes.Clone(rec.Value[:len(rec.Value)-1]), []byte(`,"Unknown":null}`)...)
			case "record-object-duplicate":
				rec.Value = bytes.Replace(rec.Value, []byte(`"Authority":0`), []byte(`"Authority":1,"Authority":0`), 1)
			default:
				t.Fatal("unhandled record mutation", kind)
			}
			return
		}
	}
	t.Fatal("missing known record", want)
}
