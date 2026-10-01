package adr0011generic

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Private B3 inputs. They are not acquisition, selector issuance, admission or
// production authority. The verifier caller supplies held originals independently
// from claimant graph and claimant SOURCE records.
type SourceCustodyKind string

const (
	SourceOwnerBuffer             SourceCustodyKind = "OWNER_BUFFER"
	SourceManagedVirtual          SourceCustodyKind = "MANAGED_VIRTUAL"
	SourceCleanRegisteredWorktree SourceCustodyKind = "CLEAN_REGISTERED_WORKTREE"
	SourceImmutableSnapshot       SourceCustodyKind = "IMMUTABLE_SOURCE_SNAPSHOT"
)

type HeldSourceOriginal struct {
	TransactionIdentity string
	SourceSelector      string
	URIBytes            []byte
	VersionBytes        []byte
	ContentBytes        []byte
	CustodyKind         SourceCustodyKind
}

type claimedSourceOriginal struct {
	TransactionIdentity string
	SourceSelector      string
	URIBytes            []byte
	VersionBytes        []byte
	ContentBytes        []byte
	ContentLength       int
	ContentSHA256       string
	CustodyKind         SourceCustodyKind
	Verified            bool // never consulted as an independent expectation
}

func validSourceCustody(kind SourceCustodyKind) bool {
	switch kind {
	case SourceOwnerBuffer, SourceManagedVirtual, SourceCleanRegisteredWorktree, SourceImmutableSnapshot:
		return true
	default:
		return false
	}
}

func sourceByteBounds(uri, version, content []byte) error {
	if len(uri) < 1 || len(uri) > 16384 || !utf8.Valid(uri) {
		return fmt.Errorf("SOURCE URI byte bound or UTF-8")
	}
	if version == nil || len(version) > 4096 || !utf8.Valid(version) {
		return fmt.Errorf("SOURCE version absent, byte bound or UTF-8")
	}
	if content == nil || len(content) > 4194304 {
		return fmt.Errorf("SOURCE content absent or byte bound")
	}
	return nil
}

// The transaction-domain identifier is derived only from B2's independently
// held identity. It never incorporates an identity copied from a SOURCE claim.
func heldSourceTX(id syntheticTransactionIdentity) (string, error) {
	if len(id.session) < 1 || len(id.session) > 1024 || !utf8.ValidString(id.session) ||
		len(id.transaction) < 1 || len(id.transaction) > 1024 || !utf8.ValidString(id.transaction) || id.generation == 0 {
		return "", fmt.Errorf("held transaction identity bound")
	}
	var preimage []byte
	for _, value := range []string{"ADR0011-GENERIC-TRANSACTION/1", id.session, strconv.FormatUint(id.generation, 10), id.transaction} {
		preimage = append(preimage, component(value)...)
	}
	return "sha256:" + digest(preimage), nil
}

// The SOURCE-specific suffix is additive; QUERY and TARGET_EVENTS retain A2's
// unchanged selector preimages. Only embedded pinned originals supply pins.
func heldSourceSelector(method string, id syntheticTransactionIdentity, source HeldSourceOriginal, tx string) (string, error) {
	var profile, policy string
	switch method {
	case "textDocument/references":
		profile, policy = "GENERIC_LSP_REFERENCES_EXACT_V2", "REFERENCES"
	case "textDocument/definition":
		profile, policy = "GENERIC_LSP_DEFINITION_EXACT_V2", "DEFINITION"
	default:
		return "", fmt.Errorf("unsupported B3 method")
	}
	if err := sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
		return "", err
	}
	if !validSourceCustody(source.CustodyKind) {
		return "", fmt.Errorf("unselected SOURCE custody kind")
	}
	if source.TransactionIdentity != tx {
		return "", fmt.Errorf("held SOURCE cross-transaction identity")
	}
	var artifact []byte
	for _, field := range [][]byte{
		[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), source.URIBytes,
		[]byte("present"), source.VersionBytes,
		[]byte(strconv.Itoa(len(source.ContentBytes))), []byte(digest(source.ContentBytes)), []byte(source.CustodyKind),
	} {
		artifact = append(artifact, component(string(field))...)
	}
	schema, err := loadOriginal("SCHEMA")
	if err != nil {
		return "", err
	}
	transport, err := loadOriginal("TRANSPORT")
	if err != nil {
		return "", err
	}
	methodPolicy, err := loadOriginal(policy)
	if err != nil {
		return "", err
	}
	var preimage []byte
	for _, field := range []string{
		domain, profile, method, "SOURCE", digest(schema), digest(transport), digest(methodPolicy),
		id.session, strconv.FormatUint(id.generation, 10), digest(artifact), tx,
	} {
		preimage = append(preimage, component(field)...)
	}
	return "sha256:" + digest(preimage), nil
}

func snapshotB3Graph(graph []syntheticGraphRole, held syntheticGraphSelection) ([]syntheticGraphRole, syntheticGraphSelection) {
	copied := append([]syntheticGraphRole(nil), graph...)
	for i := range copied {
		copied[i].predecessors = append([]syntheticPredecessor(nil), graph[i].predecessors...)
	}
	held.roles = append([]syntheticPredecessor(nil), held.roles...)
	return copied, held
}

func snapshotB3Sources(claimed []claimedSourceOriginal, held []HeldSourceOriginal) ([]claimedSourceOriginal, []HeldSourceOriginal) {
	claims := append([]claimedSourceOriginal(nil), claimed...)
	for i := range claims {
		claims[i].URIBytes = bytes.Clone(claimed[i].URIBytes)
		claims[i].VersionBytes = bytes.Clone(claimed[i].VersionBytes)
		claims[i].ContentBytes = bytes.Clone(claimed[i].ContentBytes)
	}
	sources := append([]HeldSourceOriginal(nil), held...)
	for i := range sources {
		sources[i].URIBytes = bytes.Clone(held[i].URIBytes)
		sources[i].VersionBytes = bytes.Clone(held[i].VersionBytes)
		sources[i].ContentBytes = bytes.Clone(held[i].ContentBytes)
	}
	return claims, sources
}

func sameB3SourceSnapshot(claimed, copiedClaims []claimedSourceOriginal, held, copiedHeld []HeldSourceOriginal) bool {
	if len(claimed) != len(copiedClaims) || len(held) != len(copiedHeld) {
		return false
	}
	for i, s := range held {
		c := copiedHeld[i]
		if s.TransactionIdentity != c.TransactionIdentity || s.SourceSelector != c.SourceSelector || s.CustodyKind != c.CustodyKind ||
			(s.URIBytes == nil) != (c.URIBytes == nil) || (s.VersionBytes == nil) != (c.VersionBytes == nil) || (s.ContentBytes == nil) != (c.ContentBytes == nil) ||
			!bytes.Equal(s.URIBytes, c.URIBytes) || !bytes.Equal(s.VersionBytes, c.VersionBytes) || !bytes.Equal(s.ContentBytes, c.ContentBytes) {
			return false
		}
	}
	for i, s := range claimed {
		c := copiedClaims[i]
		if s.TransactionIdentity != c.TransactionIdentity || s.SourceSelector != c.SourceSelector || s.CustodyKind != c.CustodyKind ||
			s.ContentLength != c.ContentLength || s.ContentSHA256 != c.ContentSHA256 || s.Verified != c.Verified ||
			(s.URIBytes == nil) != (c.URIBytes == nil) || (s.VersionBytes == nil) != (c.VersionBytes == nil) || (s.ContentBytes == nil) != (c.ContentBytes == nil) ||
			!bytes.Equal(s.URIBytes, c.URIBytes) || !bytes.Equal(s.VersionBytes, c.VersionBytes) || !bytes.Equal(s.ContentBytes, c.ContentBytes) {
			return false
		}
	}
	return true
}

func validateSyntheticSources(graph []syntheticGraphRole, heldGraph syntheticGraphSelection, claimed []claimedSourceOriginal, sources []HeldSourceOriginal) error {
	return validateSyntheticSourcesWithDigest(graph, heldGraph, claimed, sources, digest)
}

// hash is injectable solely for the *reported content digest* comparison. The
// transaction and SOURCE selectors always use real SHA-256; exact byte equality
// remains mandatory even if an injected digest collides.
func validateSyntheticSourcesWithDigest(graph []syntheticGraphRole, heldGraph syntheticGraphSelection, claimed []claimedSourceOriginal, sources []HeldSourceOriginal, hash func([]byte) string) error {
	return validateSyntheticSourcesAtSnapshot(graph, heldGraph, claimed, sources, hash, nil)
}

// beforeSnapshot is a private deterministic test seam for the eligibility-to-copy
// boundary; normal replay always supplies nil. It is not a concurrent-safe API.
func validateSyntheticSourcesAtSnapshot(graph []syntheticGraphRole, heldGraph syntheticGraphSelection, claimed []claimedSourceOriginal, sources []HeldSourceOriginal, hash func([]byte) string, beforeSnapshot func()) error {
	if hash == nil {
		return fmt.Errorf("missing content digest verifier")
	}
	if len(graph) < 1 || len(graph) > 266 || len(heldGraph.roles) < 1 || len(heldGraph.roles) > 266 {
		return fmt.Errorf("B3 graph role bound")
	}
	if len(sources) < 1 || len(sources) > 256 {
		return fmt.Errorf("held SOURCE cardinality")
	}
	if len(claimed) < 1 || len(claimed) > 256 {
		return fmt.Errorf("claimant SOURCE cardinality")
	}
	// Eligibility is checked before any source-byte copy.
	if _, err := heldSourceTX(heldGraph.identity); err != nil {
		return err
	}
	for _, source := range sources {
		if err := sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
			return err
		}
	}
	for _, source := range claimed {
		if err := sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
			return fmt.Errorf("claimant %w", err)
		}
	}
	if beforeSnapshot != nil {
		beforeSnapshot()
	}
	graph, heldGraph = snapshotB3Graph(graph, heldGraph)
	copiedClaims, copiedSources := snapshotB3Sources(claimed, sources)
	if !sameB3SourceSnapshot(claimed, copiedClaims, sources, copiedSources) {
		return fmt.Errorf("SOURCE input changed during snapshot")
	}
	claimed, sources = copiedClaims, copiedSources
	// The pre-copy eligibility check is not the decision: only these detached,
	// rechecked bytes are used for B2 and B3 replay.
	for _, source := range sources {
		if err := sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
			return fmt.Errorf("held snapshot %w", err)
		}
	}
	for _, source := range claimed {
		if err := sourceByteBounds(source.URIBytes, source.VersionBytes, source.ContentBytes); err != nil {
			return fmt.Errorf("claimant snapshot %w", err)
		}
	}
	if err := validateSyntheticGraph(graph, heldGraph); err != nil {
		return fmt.Errorf("B2 graph: %w", err)
	}
	tx, err := heldSourceTX(heldGraph.identity)
	if err != nil {
		return err
	}
	selected := make(map[string]bool)
	for _, role := range heldGraph.roles {
		if role.role == "SOURCE" {
			selected[role.selector] = true
		}
	}
	if len(sources) != len(selected) || len(claimed) != len(selected) {
		return fmt.Errorf("missing or extra held/claimant SOURCE")
	}
	bySelector := make(map[string]claimedSourceOriginal, len(claimed))
	for _, claim := range claimed {
		if _, duplicate := bySelector[claim.SourceSelector]; duplicate {
			return fmt.Errorf("duplicate claimant SOURCE selector")
		}
		bySelector[claim.SourceSelector] = claim
	}
	seen := make(map[string]bool, len(sources))
	previous := ""
	for index, source := range sources {
		if seen[source.SourceSelector] {
			return fmt.Errorf("duplicate held SOURCE selector")
		}
		seen[source.SourceSelector] = true
		if index == 0 && source.SourceSelector != heldGraph.querySourceSelector {
			return fmt.Errorf("query SOURCE must be first")
		}
		if index > 1 && source.SourceSelector <= previous {
			return fmt.Errorf("held SOURCE order")
		}
		previous = source.SourceSelector
		if !selected[source.SourceSelector] {
			return fmt.Errorf("extra held SOURCE selector")
		}
		selector, err := heldSourceSelector(heldGraph.method, heldGraph.identity, source, tx)
		if err != nil {
			return err
		}
		if selector != source.SourceSelector {
			return fmt.Errorf("held SOURCE selector/domain mismatch")
		}
		claim, ok := bySelector[selector]
		if !ok {
			return fmt.Errorf("missing claimant SOURCE selector")
		}
		if claim.TransactionIdentity != tx {
			return fmt.Errorf("claimant SOURCE transaction mismatch")
		}
		if claim.CustodyKind != source.CustodyKind {
			return fmt.Errorf("SOURCE custody mismatch")
		}
		if !bytes.Equal(claim.URIBytes, source.URIBytes) || !bytes.Equal(claim.VersionBytes, source.VersionBytes) {
			return fmt.Errorf("SOURCE URI/version bytes mismatch")
		}
		if claim.ContentLength != len(source.ContentBytes) || claim.ContentSHA256 != "sha256:"+hash(source.ContentBytes) {
			return fmt.Errorf("SOURCE derived content length/digest mismatch")
		}
		if !bytes.Equal(claim.ContentBytes, source.ContentBytes) {
			return fmt.Errorf("SOURCE complete content bytes mismatch")
		}
	}
	if len(seen) != len(selected) {
		return fmt.Errorf("missing held SOURCE selector")
	}
	// A digest implementation returning invalid bytes is not a successful proof.
	for _, claim := range claimed {
		if !strings.HasPrefix(claim.ContentSHA256, "sha256:") || !validateSHA(strings.TrimPrefix(claim.ContentSHA256, "sha256:")) {
			return fmt.Errorf("invalid SOURCE digest")
		}
	}
	return nil
}
