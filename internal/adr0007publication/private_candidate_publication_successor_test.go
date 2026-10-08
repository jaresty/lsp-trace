package adr0007publication

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const frozenDesignPredecessorCommit = "f6e4315ab242fb3c9eea1c1a30ab9efb093090d8"

func TestSuccessorClosedCommittedOutcomesAndPostRenameFailures(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("successor filesystem semantics are defined for Darwin and Linux")
	}

	root := t.TempDir()
	custodian := NewPrivateCandidatePublicationCustodian(PrivateCandidatePublicationOptions{
		RepositoryRoot:       root,
		PublicationNamespace: "closed-outcomes",
	})
	base := validPublicationRequest(root, "closed-outcomes", "candidate-closed", PublicationPredecessorToken{Absent: true})

	preRename := base
	preRename.Faults = PrivatePublicationFaults{FailBeforeRename: true}
	preResult, err := custodian.PublishCandidate(context.Background(), preRename)
	if err == nil {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_PRERENAME_FAILURE_RED: expected fail-closed error")
	}
	if preResult.PublicationOutcome != PublicationOutcomeNotCommitted || preResult.Retry != RetrySafePrecommitRetry {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_PRERENAME_NOT_COMMITTED: outcome=%s retry=%s err=%v", preResult.PublicationOutcome, preResult.Retry, err)
	}

	cases := []struct {
		name   string
		faults PrivatePublicationFaults
	}{
		{"rename success plus directory sync failure", PrivatePublicationFaults{FailDirectorySyncAfterRename: true}},
		{"rename success plus close failure", PrivatePublicationFaults{FailCloseAfterRename: true}},
		{"rename success plus reread failure", PrivatePublicationFaults{FailRereadAfterRename: true}},
		{"rename success plus verifier failure", PrivatePublicationFaults{FailVerifierAfterRename: true}},
		{"rename success plus cancellation", PrivatePublicationFaults{CancelAfterRename: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.CandidateID = strings.ReplaceAll("candidate-"+tc.name, " ", "-")
			req.Selector = req.CandidateID + ".selector"
			req.Faults = tc.faults
			result, err := custodian.PublishCandidate(context.Background(), req)
			if err == nil {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_POSTRENAME_FAILURE_RED: expected committed-family failure")
			}
			if result.PublicationOutcome != PublicationOutcomeCommittedVerificationFailed {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_POSTRENAME_COMMITTED_FAMILY: outcome=%s err=%v", result.PublicationOutcome, err)
			}
			if result.Retry != RetryReconcileOnly || result.Retry == RetrySafePrecommitRetry {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_NO_BLIND_RETRY: retry=%s", result.Retry)
			}
			if result.Partial.Selector != req.Selector || result.Partial.CandidateID != req.CandidateID || result.Partial.ManifestGeneration == "" || result.Partial.FailingPostCommitStep == "" {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_PARTIAL_FINAL_IDENTITY: partial=%+v req=%+v", result.Partial, req)
			}
			if !result.FinalNameVisibilityObserved {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_FINAL_VISIBILITY_OBSERVED: result=%+v", result)
			}
			if got := result.CustodyTimeline.AuthoritativeReceiptWritesAfterRename; got != 0 {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_NO_AUTHORITATIVE_POSTRENAME_RECEIPT: writes=%d timeline=%+v", got, result.CustodyTimeline)
			}
			if !result.CustodyTimeline.AllCustodyReceiptsVerifiedBeforeRename || result.CustodyTimeline.OptionalDiagnosticsAuthoritative {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_PRE_RENAME_CUSTODY_AND_NONAUTHORITATIVE_DIAGNOSTICS: timeline=%+v", result.CustodyTimeline)
			}
		})
	}
}

func TestSuccessorExactImmutableCollisionRequiresPinnedIdentityAndExactBytes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("successor filesystem semantics are defined for Darwin and Linux")
	}

	root := t.TempDir()
	custodian := NewPrivateCandidatePublicationCustodian(PrivateCandidatePublicationOptions{RepositoryRoot: root, PublicationNamespace: "collision"})
	predecessor := PublicationPredecessorToken{Selector: "previous.selector", Digest: sha256Text("previous"), ByteLength: int64(len("previous"))}
	req := validPublicationRequest(root, "collision", "candidate-collision", predecessor)
	manifestBytes := canonicalManifestBytes(t, req)
	manifestPath := filepath.Join(root, "collision", "manifests", req.ManifestGeneration+".json")
	writeFile(t, manifestPath, manifestBytes, 0o600)

	result, err := custodian.PublishCandidate(context.Background(), req)
	if err != nil {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_EXACT_COLLISION_IDEMPOTENT_RED: %v result=%+v", err, result)
	}
	if !result.Idempotent || result.CollisionOutcome != CollisionOutcomeExactIdempotent {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_EXACT_COLLISION_IDEMPOTENT: idempotent=%v collision=%s", result.Idempotent, result.CollisionOutcome)
	}
	if result.ManifestDigest != req.ExpectedManifestDigest || result.ManifestLength != req.ExpectedManifestLength || string(result.CanonicalManifestBytes) != string(manifestBytes) {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_EXACT_RAW_AND_CANONICAL_BYTES: result=%+v", result)
	}

	mismatches := []struct {
		name string
		mut  func(*CandidatePublicationRequest)
	}{
		{"raw digest", func(r *CandidatePublicationRequest) { r.ExpectedManifestDigest = sha256Text("wrong") }},
		{"raw length", func(r *CandidatePublicationRequest) { r.ExpectedManifestLength++ }},
		{"candidate", func(r *CandidatePublicationRequest) { r.CandidateID = "other-candidate" }},
		{"selector", func(r *CandidatePublicationRequest) { r.Selector = "other.selector" }},
		{"generation", func(r *CandidatePublicationRequest) { r.ExpectedManifestGeneration = "g-other" }},
		{"receipt digest", func(r *CandidatePublicationRequest) { r.ExpectedByteReceiptDigest = sha256Text("wrong-receipt") }},
		{"receipt length", func(r *CandidatePublicationRequest) { r.ExpectedByteReceiptLength++ }},
		{"publication predecessor token", func(r *CandidatePublicationRequest) {
			r.PublicationPredecessorToken = PublicationPredecessorToken{Absent: true}
		}},
	}
	for _, tc := range mismatches {
		t.Run(tc.name, func(t *testing.T) {
			bad := req
			tc.mut(&bad)
			result, err := custodian.PublishCandidate(context.Background(), bad)
			if err == nil || result.CollisionOutcome != CollisionOutcomeImmutableFailedClosed || result.Idempotent {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_COLLISION_MISMATCH_FAILS_CLOSED: case=%s result=%+v err=%v", tc.name, result, err)
			}
		})
	}

	t.Run("symlink directory hardlink and writable modes fail collision", func(t *testing.T) {
		badPaths := []struct {
			name  string
			setup func(string)
		}{
			{"symlink", func(p string) { os.Remove(p); must(t, os.Symlink("target", p)) }},
			{"directory", func(p string) { os.Remove(p); must(t, os.MkdirAll(p, 0o700)) }},
			{"hardlink", func(p string) { link := p + ".link"; os.Remove(link); must(t, os.Link(p, link)) }},
			{"group writable", func(p string) { must(t, os.Chmod(p, 0o620)) }},
			{"world writable", func(p string) { must(t, os.Chmod(p, 0o602)) }},
		}
		for _, bp := range badPaths {
			t.Run(bp.name, func(t *testing.T) {
				root := t.TempDir()
				custodian := NewPrivateCandidatePublicationCustodian(PrivateCandidatePublicationOptions{RepositoryRoot: root, PublicationNamespace: "collision-" + bp.name})
				req := validPublicationRequest(root, "collision-"+bp.name, "candidate-"+bp.name, predecessor)
				p := filepath.Join(root, "collision-"+bp.name, "manifests", req.ManifestGeneration+".json")
				writeFile(t, p, canonicalManifestBytes(t, req), 0o600)
				bp.setup(p)
				result, err := custodian.PublishCandidate(context.Background(), req)
				if err == nil || result.CollisionOutcome != CollisionOutcomeImmutableFailedClosed || result.Idempotent {
					t.Fatalf("ASSERT_ADR0007_SUCCESSOR_PINNED_NOFOLLOW_REGULAR_MODE_NLINK_COLLISION: case=%s result=%+v err=%v", bp.name, result, err)
				}
			})
		}
	})
}

func TestSuccessorDefaultLookupBypassRemovedForCustodyVerification(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	custodian := NewPrivateCandidatePublicationCustodian(PrivateCandidatePublicationOptions{RepositoryRoot: root, PublicationNamespace: "lookup"})
	req := validPublicationRequest(root, "lookup", "candidate-lookup", PublicationPredecessorToken{Selector: "previous.selector", Digest: sha256Text("previous"), ByteLength: int64(len("previous"))})
	publishExactCandidateFixture(t, custodian, req)

	locator, err := custodian.LocateCandidateByAlias(context.Background(), CandidateAliasLookupRequest{RepositoryRoot: root, PublicationNamespace: "lookup", Alias: req.CandidateID, MaxLookupBytes: 4096})
	if err != nil || !locator.Located || locator.Custody != LocatorCustodyNone || locator.CustodyStatus != "" {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_ALIAS_LOCATOR_NOT_CUSTODY: locator=%+v err=%v", locator, err)
	}

	good := req
	verified, err := custodian.VerifyCandidatePublicationCustody(context.Background(), good)
	if err != nil || verified.CustodyStatus != CustodyStatusVerified {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_CUSTODY_REQUIRES_VERIFIED_COMPONENTS_RED: verified=%+v err=%v", verified, err)
	}
	if verified.FrozenDesignPredecessorCommit != frozenDesignPredecessorCommit || verified.PublicationPredecessorToken != req.PublicationPredecessorToken {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_SEPARATE_PREDECESSOR_IDENTITIES: verified=%+v", verified)
	}

	cases := []struct {
		name string
		mut  func(*CandidatePublicationRequest)
	}{
		{"missing manifest generation", func(r *CandidatePublicationRequest) { r.ExpectedManifestGeneration = "" }},
		{"missing byte receipt digest", func(r *CandidatePublicationRequest) { r.ExpectedByteReceiptDigest = "" }},
		{"wrong selector digest", func(r *CandidatePublicationRequest) { r.ExpectedCurrentSelectorDigest = sha256Text("wrong-selector") }},
		{"missing selector digest", func(r *CandidatePublicationRequest) { r.ExpectedCurrentSelectorDigest = "" }},
		{"wrong selector length", func(r *CandidatePublicationRequest) { r.ExpectedCurrentSelectorByteLength++ }},
		{"missing selector length", func(r *CandidatePublicationRequest) { r.ExpectedCurrentSelectorByteLength = 0 }},
		{"missing publication predecessor token", func(r *CandidatePublicationRequest) { r.PublicationPredecessorToken = PublicationPredecessorToken{} }},
		{"wrong publication predecessor token", func(r *CandidatePublicationRequest) {
			r.PublicationPredecessorToken = PublicationPredecessorToken{Selector: "wrong", Digest: sha256Text("previous"), ByteLength: int64(len("previous"))}
		}},
		{"wrong frozen design predecessor commit", func(r *CandidatePublicationRequest) { r.FrozenDesignPredecessorCommit = strings.Repeat("0", 40) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := req
			tc.mut(&bad)
			result, err := custodian.VerifyCandidatePublicationCustody(context.Background(), bad)
			if err == nil || result.CustodyStatus == CustodyStatusVerified || result.CustodyStatus == "" {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_VERIFY_FAILS_CLOSED_WITHOUT_EXACT_CUSTODY: case=%s result=%+v err=%v", tc.name, result, err)
			}
		})
	}
}

func TestSuccessorStableRetainedLockInodeSerializesAndFailsClosedOnSubstitution(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("successor filesystem semantics are defined for Darwin and Linux")
	}

	root := t.TempDir()
	lockPath := filepath.Join(root, "locks", "publication.lock")
	locks := NewStablePublicationLockManager(StablePublicationLockOptions{RepositoryRoot: root, PublicationNamespace: "locks", LockName: "publication.lock"})
	first := acquireAndReleaseLock(t, locks)
	second := acquireAndReleaseLock(t, locks)
	if first.Inode == 0 || first.Device == 0 || first.Inode != second.Inode || first.Device != second.Device {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_RETAINS_INODE_SEQUENTIAL: first=%+v second=%+v", first, second)
	}
	if _, err := os.Lstat(lockPath); err != nil {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_REMAINS_AFTER_RELEASE: %v", err)
	}
	restarted := NewStablePublicationLockManager(StablePublicationLockOptions{RepositoryRoot: root, PublicationNamespace: "locks", LockName: "publication.lock"})
	third := acquireAndReleaseLock(t, restarted)
	if third.Inode != first.Inode || third.Device != first.Device {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_RETAINS_INODE_RESTART: first=%+v third=%+v", first, third)
	}

	serialized := make(chan LockIdentity, 2)
	barrier := make(chan struct{})
	go func() {
		holder, err := restarted.Acquire(context.Background())
		if err != nil {
			t.Errorf("ASSERT_ADR0007_SUCCESSOR_LOCK_HELPER_FIRST_ACQUIRE: %v", err)
			return
		}
		serialized <- holder.Identity()
		<-barrier
		_ = holder.Release()
	}()
	firstConcurrent := <-serialized
	go func() {
		holder, err := restarted.Acquire(context.Background())
		if err != nil {
			t.Errorf("ASSERT_ADR0007_SUCCESSOR_LOCK_HELPER_SECOND_ACQUIRE: %v", err)
			return
		}
		serialized <- holder.Identity()
		_ = holder.Release()
	}()
	close(barrier)
	secondConcurrent := <-serialized
	if firstConcurrent.Inode != secondConcurrent.Inode || firstConcurrent.Device != secondConcurrent.Device {
		t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_HELPER_PROCESS_SERIALIZES_SAME_INODE: first=%+v second=%+v", firstConcurrent, secondConcurrent)
	}

	badSetups := []struct {
		name  string
		setup func(string)
	}{
		{"symlink", func(p string) { os.Remove(p); must(t, os.Symlink("target", p)) }},
		{"nonregular directory", func(p string) { os.Remove(p); must(t, os.MkdirAll(p, 0o700)) }},
		{"hardlink nlink", func(p string) { must(t, os.Link(p, p+".link")) }},
		{"group writable", func(p string) { must(t, os.Chmod(p, 0o620)) }},
		{"world writable", func(p string) { must(t, os.Chmod(p, 0o602)) }},
		{"inode substitution before flock", func(p string) { os.Remove(p); writeFile(t, p, []byte("replacement"), 0o600) }},
	}
	for _, bs := range badSetups {
		t.Run(bs.name, func(t *testing.T) {
			root := t.TempDir()
			manager := NewStablePublicationLockManager(StablePublicationLockOptions{RepositoryRoot: root, PublicationNamespace: "locks", LockName: "publication.lock"})
			identity := acquireAndReleaseLock(t, manager)
			p := filepath.Join(root, "locks", "publication.lock")
			bs.setup(p)
			_, err := manager.Acquire(context.Background(), RequireExistingLockIdentity(identity))
			if err == nil || !IsLockSubstitutionFailedClosed(err) {
				t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_SUBSTITUTION_FAILS_CLOSED: case=%s err=%v", bs.name, err)
			}
		})
	}

	t.Run("crash releases flock while inode remains", func(t *testing.T) {
		identity, err := restarted.ExerciseCrashRelease(context.Background())
		if err != nil {
			t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_CRASH_RELEASES_FLOCK_RED: %v", err)
		}
		after := acquireAndReleaseLock(t, restarted)
		if identity.Inode != after.Inode || identity.Device != after.Device {
			t.Fatalf("ASSERT_ADR0007_SUCCESSOR_LOCK_CRASH_RELEASE_RETAINS_INODE: crash=%+v after=%+v", identity, after)
		}
	})
}

func validPublicationRequest(root, namespace, candidateID string, predecessor PublicationPredecessorToken) CandidatePublicationRequest {
	selectorBytes := []byte(candidateID + "\n")
	manifestGeneration := "g-" + strings.Repeat("a", 64)
	manifestSeed := candidateID + "|" + namespace + "|" + manifestGeneration
	byteReceipt := []byte("receipt|" + manifestSeed)
	manifest := []byte("manifest|" + manifestSeed + "|" + predecessor.String())
	return CandidatePublicationRequest{
		FrozenDesignPredecessorCommit:     frozenDesignPredecessorCommit,
		PublicationPredecessorToken:       predecessor,
		CandidateID:                       candidateID,
		Selector:                          candidateID + ".selector",
		ExpectedCurrentSelectorDigest:     sha256Bytes(selectorBytes),
		ExpectedCurrentSelectorByteLength: int64(len(selectorBytes)),
		ExpectedManifestGeneration:        manifestGeneration,
		ExpectedManifestDigest:            sha256Bytes(manifest),
		ExpectedManifestLength:            int64(len(manifest)),
		ExpectedByteReceiptDigest:         sha256Bytes(byteReceipt),
		ExpectedByteReceiptLength:         int64(len(byteReceipt)),
		RepositoryRoot:                    root,
		PublicationNamespace:              namespace,
		PlatformSupport:                   PlatformSupportCurrent,
		MaxManifestBytes:                  1 << 20,
		MaxAliasBytes:                     1 << 20,
		MaxReceiptBytes:                   1 << 20,
	}
}

func canonicalManifestBytes(t *testing.T, req CandidatePublicationRequest) []byte {
	t.Helper()
	return CanonicalCandidateManifestBytes(CandidateManifest{
		CandidateID:                 req.CandidateID,
		Selector:                    req.Selector,
		ManifestGeneration:          req.ExpectedManifestGeneration,
		ByteReceiptDigest:           req.ExpectedByteReceiptDigest,
		ByteReceiptLength:           req.ExpectedByteReceiptLength,
		PublicationPredecessorToken: req.PublicationPredecessorToken,
		FrozenDesignPredecessor:     req.FrozenDesignPredecessorCommit,
	})
}

func publishExactCandidateFixture(t *testing.T, custodian *PrivateCandidatePublicationCustodian, req CandidatePublicationRequest) {
	t.Helper()
	result, err := custodian.PublishCandidate(context.Background(), req)
	if err != nil || result.PublicationOutcome != PublicationOutcomeCommitted {
		t.Fatalf("publish exact candidate fixture: result=%+v err=%v", result, err)
	}
}

func acquireAndReleaseLock(t *testing.T, manager *StablePublicationLockManager) LockIdentity {
	t.Helper()
	holder, err := manager.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	identity := holder.Identity()
	if err := holder.Release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}
	return identity
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func sha256Text(s string) string { return sha256Bytes([]byte(s)) }

func sha256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
