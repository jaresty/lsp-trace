package liveprojection

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/sessionruntime"
)

func TestC05CompletePrivacyEligibilityAndNonDisclosureCorpus(t *testing.T) {
	const assertion = "ASSERT_C05_COMPLETE_PRIVACY_ELIGIBILITY_NON_DISCLOSURE"
	const publicURI = "file:///workspace/public.go"
	markers := []string{
		"/private/operator/root",
		"file:///arbitrary/outside/workspace.go",
		"SECRET_ACCESS_TOKEN_7df1",
		"rm -rf /forbidden-command",
		"PRODUCTION_PASSWORD=forbidden-environment",
		"raw resolver error: permission denied at /private/operator/root",
	}
	markerBlob := []byte("public\n" + markers[2] + "\n" + markers[3] + "\n" + markers[4] + "\n" + markers[5] + "\n")

	candidate := func(id, role, uri, privacy string) sourceprojection.Candidate {
		return sourceprojection.Candidate{
			UnitID: id, CitationID: id + "-citation", Role: role, GraphSubjectID: id + "-subject",
			LogicalSourceID: uri, Range: sourceprojection.Range{Start: sourceprojection.Position{0, 0}, End: sourceprojection.Position{0, 6}},
			PositionEncoding: "utf-8", PrivacyClassification: privacy,
		}
	}
	public := candidate("public", "ENDPOINT", publicURI, "PUBLIC")
	restricted := candidate("restricted", "ENDPOINT", "file:///workspace/restricted.go", "RESTRICTED")
	withheld := candidate("withheld", "ENDPOINT", markers[0], "WITHHELD")
	ancillary := candidate("ancillary", "ANCILLARY", "file:///workspace/ancillary.go", "PRIVATE")
	unauthorized := candidate("unauthorized", "ENDPOINT", markers[1], "UNAUTHORIZED")

	plan := PlanDocuments([]sourceprojection.Candidate{public, restricted, withheld, ancillary, unauthorized}, publicURI)
	if !reflect.DeepEqual(plan, []string{publicURI}) {
		t.Fatalf("%s_PRIVACY_BEFORE_ACQUISITION: plan=%q", assertion, plan)
	}
	preparer := &recordingPreparer{responses: map[string]sessionruntime.DocumentResult{
		publicURI: supplied(sessionruntime.DocumentRequest{SessionID: "session-c05", Generation: 1, URI: publicURI, CaptureSupply: true}, markerBlob),
	}}
	prepared := Prepare(context.Background(), preparer, "session-c05", 1, "go", plan, generousLimits())
	if prepared.Status != PreparationComplete || len(preparer.requests) != 1 || preparer.requests[0].URI != publicURI || prepared.Accounting.Attempted != 1 || prepared.Accounting.Succeeded != 1 {
		t.Fatalf("%s_ONLY_AUTHORIZED_ACQUIRED: requests=%+v result=%+v", assertion, preparer.requests, prepared)
	}

	publicSource := map[string]sourceprojection.Source{publicURI: {
		LogicalSourceID: publicURI, Digest: "sha256:public", ByteLength: len(markerBlob), Bytes: markerBlob, Available: true,
	}}
	metadata, err := sourceprojection.Project([]sourceprojection.Candidate{public}, publicSource, sourceprojection.Policy{PolicyID: "metadata-default"})
	if err != nil || metadata.PrivacySummary.BodyRequested || metadata.PrivacySummary.BodyReturned != 0 || len(metadata.EmittedSpans) != 0 || len(metadata.Units) != 1 || metadata.Units[0].BodyDisposition != "NOT_REQUESTED" {
		t.Fatalf("%s_METADATA_DEFAULT: projection=%+v err=%v", assertion, metadata, err)
	}

	omitted := []sourceprojection.Candidate{restricted, withheld, unauthorized}
	omittedSources := make(map[string]sourceprojection.Source, len(omitted))
	for i := range omitted {
		omitted[i].Withheld = true
		omittedSources[omitted[i].LogicalSourceID] = sourceprojection.Source{
			LogicalSourceID: omitted[i].LogicalSourceID, Digest: "sha256:" + omitted[i].UnitID,
			ByteLength: len(markerBlob), Bytes: markerBlob, Available: true,
		}
	}
	withheldResult, err := sourceprojection.Project(omitted, omittedSources, sourceprojection.Policy{PolicyID: "deny-private", BodyRequested: true})
	if err != nil || withheldResult.Accounting.Selected != 0 || withheldResult.Accounting.Omitted != len(omitted) || withheldResult.PrivacySummary.BodyReturned != 0 || withheldResult.PrivacySummary.BodyWithheld != len(omitted) || len(withheldResult.Omissions) != len(omitted) {
		t.Fatalf("%s_RESTRICTED_WITHHELD_ANCILLARY_UNAUTHORIZED: projection=%+v err=%v", assertion, withheldResult, err)
	}
	for _, omission := range withheldResult.Omissions {
		if omission.Cause != "POLICY_WITHHELD" {
			t.Fatalf("%s_TYPED_POLICY_ACCOUNTING: omissions=%+v", assertion, withheldResult.Omissions)
		}
	}

	unavailableSources := map[string]sourceprojection.Source{publicURI: {
		LogicalSourceID: publicURI, Digest: "sha256:unavailable", ByteLength: len(markerBlob), Bytes: markerBlob, Available: false,
	}}
	unavailableResult, err := sourceprojection.Project([]sourceprojection.Candidate{public}, unavailableSources, sourceprojection.Policy{PolicyID: "public", BodyRequested: true})
	if err != nil || unavailableResult.Accounting.Selected != 0 || unavailableResult.Accounting.Omitted != 1 || len(unavailableResult.Omissions) != 1 || unavailableResult.Omissions[0].Cause != "SOURCE_UNAVAILABLE" || unavailableResult.PrivacySummary.BodyReturned != 0 {
		t.Fatalf("%s_UNAVAILABLE_ACCOUNTING: projection=%+v err=%v", assertion, unavailableResult, err)
	}

	for name, value := range map[string]any{
		"plan": plan, "preparation": prepared, "metadata": metadata, "withheld": withheldResult, "unavailable": unavailableResult,
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s_SERIALIZE_%s: %v", assertion, name, err)
		}
		for _, marker := range markers {
			if bytes.Contains(raw, []byte(marker)) {
				t.Fatalf("%s_MARKER_LEAK_%s: marker=%q output=%s", assertion, name, marker, raw)
			}
		}
	}
}
