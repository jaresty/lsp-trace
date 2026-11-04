package censuscontinuation

import (
	"bytes"
	"context"
	"reflect"
	"testing"
)

func TestMultiSeedV2QualificationEvidenceReconstruction(t *testing.T) {
	t.Log("procedure: committed receipt inventory -> v2 adaptation/preflight -> terminal-byte-bound immutable evidence; no writes/CAS")
	planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 5)
	before := cloneProgramCV2ObjectMap(objects.m)
	raw := bytes.Clone(heads.raw)
	puts, cas, loads := objects.puts, heads.cas, heads.loads
	evidence, err := buildProgramCMultiSeedQualificationEvidenceV2WithRunner(context.Background(), request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
	if err != nil || len(evidence.inventory) != 1 || len(evidence.preflight.adaptations) != 1 || !bytes.Equal(evidence.terminalRaw, raw) {
		t.Fatalf("CONNECTED_V2_QUALIFICATION_EVIDENCE: inventory=%d err=%v", len(evidence.inventory), err)
	}
	if heads.loads != loads+1 || objects.puts != puts || heads.cas != cas || !reflect.DeepEqual(objects.m, before) {
		t.Fatal("QUALIFICATION_READ_ONLY_ONE_SNAPSHOT")
	}
	if evidence.preflight.accounting.eligibleRunCommunityResultsD != expected.eligibleRunCommunityResultsD || evidence.preflight.accounting.seedGroupResultsG != expected.seedGroupResultsG {
		t.Fatal("QUALIFICATION_EXACT_D_AND_G")
	}
	for _, m := range evidence.preflight.adaptations[0].mechanics {
		if !m.Trusted() || len(m.SelectionContext().SeedGroups) != 5 {
			t.Fatal("QUALIFICATION_COMPLETE_ORDERED_CONTEXT")
		}
	}
}

func TestMultiSeedV2QualificationEvidenceZeroExposure(t *testing.T) {
	for _, name := range []string{"bad-head", "missing-receipt", "wrong-D", "wrong-G", "cancel"} {
		t.Run(name, func(t *testing.T) {
			planRaw, objects, heads, runner, request, expected := programCV2CommittedFixture(t, 2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "bad-head":
				heads.raw[len(heads.raw)/2] ^= 1
			case "missing-receipt":
				for key := range objects.m {
					delete(objects.m, key)
					break
				}
			case "wrong-D":
				expected.eligibleRunCommunityResultsD++
			case "wrong-G":
				expected.seedGroupResultsG++
			case "cancel":
				cancel()
			}
			puts, cas := objects.puts, heads.cas
			got, err := buildProgramCMultiSeedQualificationEvidenceV2WithRunner(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, objects, heads, runner, expected)
			if err == nil || got.inventory != nil || got.preflight.adaptations != nil || got.terminalRaw != nil || objects.puts != puts || heads.cas != cas {
				t.Fatal("QUALIFICATION_FAILURE_ZERO_EXPOSURE")
			}
		})
	}
}
