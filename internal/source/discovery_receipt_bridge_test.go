package source

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBridgeDiscoveryReceiptsPreservesEveryDiscoveryRecord(t *testing.T) {
	records := []Record{
		{Name: "missing.go", Kind: SourceOther, Inclusion: Unavailable, Reason: "source input does not exist"},
		{Name: "pipe", Kind: SourceOther, Inclusion: Unsupported, Reason: "unsupported filesystem kind"},
		{Name: "_outside/escape.go", Kind: SourceOther, Inclusion: OutsideBase, Reason: "outside bounded base"},
		{Name: "src", Kind: SourceDirectory, Inclusion: Included},
	}
	artifacts, err := BridgeDiscoveryReceipts(records, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != len(records) {
		t.Fatalf("ASSERT_DISCOVERY_RECORD_PRESERVED: artifacts=%+v", artifacts)
	}
	byName := make(map[string]ReceiptArtifact, len(artifacts))
	for _, artifact := range artifacts {
		byName[artifact.Record.Name] = artifact
	}
	for _, record := range records {
		artifact, ok := byName[record.Name]
		if !ok || artifact.Record != record || artifact.Receipt.Status != Unreadable || artifact.Content != nil || artifact.FailureStage != FailureStageSource || artifact.Receipt.Failure == nil || artifact.Receipt.Failure.Reason == "" {
			t.Fatalf("ASSERT_DISCOVERY_RECORD_PRESERVED: record=%+v artifact=%+v present=%t", record, artifact, ok)
		}
	}
}

func TestBridgeDiscoveryReceiptsCanonicalOrder(t *testing.T) {
	records := []Record{
		{Name: "z.go", Kind: SourceRegularFile, Inclusion: Included},
		{Name: "a.go", Kind: SourceRegularFile, Inclusion: Included},
	}
	acquired := map[string]AcquiredSource{"z.go": {Bytes: []byte("z")}, "a.go": {Bytes: []byte("a")}}
	artifacts, err := BridgeDiscoveryReceipts(records, acquired)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{artifacts[0].Record.Name, artifacts[1].Record.Name}
	if !reflect.DeepEqual(got, []string{"a.go", "z.go"}) {
		t.Fatalf("ASSERT_DISCOVERY_RECEIPT_CANONICAL_ORDER: got=%v", got)
	}
}

func TestBridgeDiscoveryReceiptsPreservesAcquiredBytes(t *testing.T) {
	content := []byte("z\r\na\n")
	record := Record{Name: "main.go", Kind: SourceRegularFile, Inclusion: Included}
	artifacts, err := BridgeDiscoveryReceipts([]Record{record}, map[string]AcquiredSource{record.Name: {Bytes: content}})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || !bytes.Equal(artifacts[0].Content, content) || artifacts[0].Receipt.ContentIdentity == nil {
		t.Fatalf("ASSERT_ACQUIRED_BYTES_PRESERVED: artifact=%+v want=%q", artifacts, content)
	}
}

func TestBridgeDiscoveryReceiptsAttributesSourceStageFailures(t *testing.T) {
	record := Record{Name: "locked.go", Kind: SourceRegularFile, Inclusion: Included}
	artifacts, err := BridgeDiscoveryReceipts([]Record{record}, map[string]AcquiredSource{record.Name: {Err: errors.New("permission denied")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].FailureStage != FailureStageSource || artifacts[0].Receipt.Status != Unreadable || artifacts[0].Content != nil || artifacts[0].Receipt.Failure == nil || !strings.Contains(artifacts[0].Receipt.Failure.Reason, "permission denied") {
		t.Fatalf("ASSERT_SOURCE_STAGE_FAILURE_ATTRIBUTED: artifact=%+v", artifacts)
	}
}

func TestBridgeDiscoveryReceiptsDoesNotExposeManifestDecisions(t *testing.T) {
	artifactType := reflect.TypeOf(ReceiptArtifact{})
	for _, forbidden := range []string{"Manifest", "Decision", "Included"} {
		if _, exists := artifactType.FieldByName(forbidden); exists {
			t.Fatalf("ASSERT_NO_MANIFEST_MEMBERSHIP: field=%s", forbidden)
		}
	}
}
