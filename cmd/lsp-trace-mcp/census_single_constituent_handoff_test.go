package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
)

func singleConstituentProjection(t *testing.T, count int) censusacquisition.Projection {
	t.Helper()
	session := censusacquisition.SessionIdentity{SessionID: "publication-session", Generation: 7}
	d := censusacquisition.Discovery{Session: session, Workspace: "/private/work", Complete: true}
	d.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: censusacquisition.FileProcessed}}}
	d.Accounting.Files = []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}
	d.Accounting.FileDenominator = 1
	for i := 0; i < count; i++ {
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: fmt.Sprintf("f%03d.go", i), Line: uint64(i + 11), Column: uint64(i + 4)}}}}, d.Workspace)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("Target%d", i)
		d.Targets = append(d.Targets, censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: fmt.Sprintf("file:///private/work/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("f%03d.go#%d:%d:12:%s:%d", i, i+10, i+3, name, i)})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: d.Targets[i].SymbolIdentity, Disposition: censusacquisition.SymbolPrepared})
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
	}
	d.SymbolLedger.Denominator, d.Accounting.SymbolDenominator = count, count
	p, err := (censusacquisition.Core{Discoverer: mcpContinuationDiscoverer{d}, Acquirer: mcpContinuationAcquirer{}}).Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func committedSingleConstituentCompletion(t *testing.T, count int) censusCompletion {
	t.Helper()
	projection := singleConstituentProjection(t, count)
	want := count
	if count > 1 {
		want = 2
	}
	if got := len(projection.Constituents); got != want {
		t.Fatalf("ASSERT_SINGLE_CONSTITUENT_ACQUISITION_CARDINALITY count=%d got=%d want=%d", count, got, want)
	}
	rootDir := t.TempDir()
	if err := os.Chmod(rootDir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	publisher := captureset.NewPublisher(root)
	metadata := programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"}
	completion := completeCensusProjectionWithContinuation(context.Background(), projection, publisher, publisher, metadata)
	if completion.Result == nil || completion.Diagnostic != nil || completion.continuation == nil || completion.continuationError != "" {
		t.Fatalf("ASSERT_SINGLE_CONSTITUENT_VALIDATED_COMPLETION count=%d result=%v diagnostic=%v custody=%v continuation_error=%q", count, completion.Result != nil, completion.Diagnostic, completion.continuation != nil, completion.continuationError)
	}
	return completion
}

const (
	assertSingleConstituentCountID      = "ASSERT_SINGLE_CONSTITUENT_COUNT"
	assertSingleConstituentCompletionID = "ASSERT_SINGLE_CONSTITUENT_COMPLETION"
	assertSingleConstituentRejectionID  = "ASSERT_SINGLE_CONSTITUENT_REJECTION"
	assertSingleConstituentSuccessID    = "ASSERT_SINGLE_CONSTITUENT_SUCCESS"
)

func TestSingleConstituentHandoffRejectsOneAndAcceptsControl(t *testing.T) {
	one := committedSingleConstituentCompletion(t, 1)
	if err := assertSingleConstituentCount(len(one.continuation.publication.Manifest.Constituents), 1); err != nil {
		t.Fatal(err)
	}
	if err := assertSingleConstituentCompletion(one); err != nil {
		t.Fatal(err)
	}
	_, err := one.BuildCommittedHandoff()
	if err == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_HANDOFF_ERROR count=1 got nil")
	}
	if guardErr := assertSingleConstituentRejection(err); guardErr != nil {
		t.Fatal(guardErr)
	}
	var failure *censusprogramc.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("ASSERT_SINGLE_CONSTITUENT_TYPED_FAILURE count=1 err=%T:%v", err, err)
	}
	t.Logf("ASSERT_SINGLE_CONSTITUENT_REJECTED stage=%s count=1 error=%q", failure.Stage, failure.Err)

	control := committedSingleConstituentCompletion(t, 65)
	if err := assertSingleConstituentCompletion(control); err != nil {
		t.Fatal(err)
	}
	if err := assertSingleConstituentSuccess(func() error { _, err := control.BuildCommittedHandoff(); return err }()); err != nil {
		t.Fatal(err)
	}
	t.Logf("ASSERT_SINGLE_CONSTITUENT_CONTROL_ACCEPTED count=65 constituents=%d", len(control.continuation.publication.Manifest.Constituents))
}

func assertSingleConstituentCount(got, want int) error {
	if got != want {
		return fmt.Errorf("%s: got %d want %d", assertSingleConstituentCountID, got, want)
	}
	return nil
}

func assertSingleConstituentCompletion(c censusCompletion) error {
	if c.Result == nil || c.Diagnostic != nil || c.continuation == nil || c.continuationError != "" {
		return fmt.Errorf("%s: incomplete completion result=%v diagnostic=%v custody=%v continuation_error=%q", assertSingleConstituentCompletionID, c.Result != nil, c.Diagnostic, c.continuation != nil, c.continuationError)
	}
	r := c.continuation.publication.Receipt
	if r.VerificationStatus != "VERIFIED" || r.CloseStatus != publication.CloseComplete || r.DirectorySyncStatus != publication.DirectorySyncComplete {
		return fmt.Errorf("%s: incomplete verified publication: %+v", assertSingleConstituentCompletionID, r)
	}
	if len(c.continuation.publication.Manifest.Constituents) != len(c.continuation.publication.Resolved) || r.ConstituentCount != len(c.continuation.publication.Manifest.Constituents) {
		return fmt.Errorf("%s: manifest/resolved/count mismatch manifest=%d resolved=%d receipt=%d", assertSingleConstituentCompletionID, len(c.continuation.publication.Manifest.Constituents), len(c.continuation.publication.Resolved), r.ConstituentCount)
	}
	return nil
}

func assertSingleConstituentSuccess(err error) error {
	if err != nil {
		return fmt.Errorf("%s: expected successful handoff, got %T:%v", assertSingleConstituentSuccessID, err, err)
	}
	return nil
}

func assertSingleConstituentRejection(err error) error {
	if err == nil {
		return fmt.Errorf("%s: expected rejection, got nil", assertSingleConstituentRejectionID)
	}
	var failure *censusprogramc.Failure
	if !errors.As(err, &failure) || failure.Stage != censusprogramc.StageComposition || failure.Err == nil || failure.Err.Error() != fmt.Sprintf("input count 1 outside [2,%d]", programccompose.MaxInputs) {
		return fmt.Errorf("%s: wrong rejection discriminator: %T:%v", assertSingleConstituentRejectionID, err, err)
	}
	return nil
}

func TestSingleConstituentHandoffIndependentGuards(t *testing.T) {
	control := committedSingleConstituentCompletion(t, 65)
	one := committedSingleConstituentCompletion(t, 1)

	if err := assertSingleConstituentCount(len(control.continuation.publication.Manifest.Constituents), 1); err == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_COUNT_GUARD_WRONG_CARDINALITY: accepted control")
	} else {
		t.Logf("ASSERT_SINGLE_CONSTITUENT_COUNT_GUARD_WRONG_CARDINALITY rejected: %v", err)
	}
	if err := assertSingleConstituentCompletion(control); err != nil {
		t.Fatal(err)
	}
	_, controlErr := control.BuildCommittedHandoff()
	_, oneErr := one.BuildCommittedHandoff()
	if got := assertSingleConstituentRejection(controlErr); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_REJECTION_GUARD_CONTROL: accepted successful control")
	} else {
		t.Logf("ASSERT_SINGLE_CONSTITUENT_REJECTION_GUARD_CONTROL rejected: %v", got)
	}
	var actualFailure *censusprogramc.Failure
	if !errors.As(oneErr, &actualFailure) {
		t.Fatal("expected actual typed failure")
	}
	wrongStage := *actualFailure
	wrongStage.Stage = "OTHER"
	if got := assertSingleConstituentRejection(&wrongStage); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_REJECTION_GUARD_WRONG_STAGE: accepted wrong stage")
	}
	wrongMessage := *actualFailure
	wrongMessage.Err = errors.New("deliberately different test-only message")
	if got := assertSingleConstituentRejection(&wrongMessage); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_REJECTION: accepted wrong message")
	}
	if got := assertSingleConstituentRejection(fmt.Errorf("input count 1 outside [2,%d]", programccompose.MaxInputs)); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_REJECTION_GUARD_UNTYPED: accepted untyped error")
	}
	if err := assertSingleConstituentRejection(fmt.Errorf("input count 1 outside [2,%d]", programccompose.MaxInputs)); err == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_CONTROL_GUARD_FAILURE: accepted failure")
	} else {
		t.Logf("ASSERT_SINGLE_CONSTITUENT_CONTROL_GUARD_FAILURE rejected: %v", err)
	}
	if err := assertSingleConstituentCompletion(one); err != nil {
		t.Fatal(err)
	}
	missingCustody := one
	missingCustody.continuation = nil
	if got := assertSingleConstituentCompletion(missingCustody); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_COMPLETION: accepted missing custody")
	}
	for id, mutate := range map[string]func(*censusCompletionCustody){
		"ASSERT_SINGLE_CONSTITUENT_CUSTODY_GUARD_MISSING": func(c *censusCompletionCustody) { c.publication.Receipt.VerificationStatus = "" },
		"ASSERT_SINGLE_CONSTITUENT_CUSTODY_GUARD_CLOSE":   func(c *censusCompletionCustody) { c.publication.Receipt.CloseStatus = "FAILED" },
		"ASSERT_SINGLE_CONSTITUENT_CUSTODY_GUARD_SYNC":    func(c *censusCompletionCustody) { c.publication.Receipt.DirectorySyncStatus = "FAILED" },
	} {
		bad := one
		custody := *one.continuation
		mutate(&custody)
		bad.continuation = &custody
		if got := assertSingleConstituentCompletion(bad); got == nil {
			t.Fatalf("%s: accepted invalid custody", id)
		} else {
			t.Logf("%s rejected: %v", id, got)
		}
	}
	if got := assertSingleConstituentSuccess(oneErr); got == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_CONTROL_GUARD_ONE_INPUT: accepted rejected handoff")
	} else {
		t.Logf("ASSERT_SINGLE_CONSTITUENT_CONTROL_GUARD_ONE_INPUT rejected: %v", got)
	}
	trial := mcpContinuationProjection(t)
	if err := assertSingleConstituentCount(len(trial.Constituents), 1); err == nil {
		t.Fatal("ASSERT_SINGLE_CONSTITUENT_SMALLER_ARTIFACT_TRIAL: accepted original65 artifact")
	} else {
		t.Logf("ASSERT_SINGLE_CONSTITUENT_SMALLER_ARTIFACT_TRIAL rejected: %v", err)
	}
}
