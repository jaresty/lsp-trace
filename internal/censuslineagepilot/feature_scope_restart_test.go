//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func restartPin(raw []byte) scopeRestartBlob { return scopeRestartBlob{Digest(raw), uint64(len(raw))} }
func restartLocator(t *testing.T, o *scopeBundleOwner, final scopeBundleResult) scopeRestartLocator {
	t.Helper()
	h, e := parseScopeBundleHead(final.head)
	if e != nil {
		t.Fatal(e)
	}
	return scopeRestartLocator{h.Key, restartPin(final.head), restartPin(o.item(o.plan.state.references[8].Object).raw), restartPin(o.plan.metadataBytes())}
}
func restartFixture(t *testing.T, equal bool) (*scopeBundleOwner, featureScope, scopeRestartLocator) {
	t.Helper()
	o, s := bundleFixture(t, equal)
	f := runBundle(t, o)
	return o, s, restartLocator(t, o, f)
}
func restartSnapshot(t *testing.T, paths [3]string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range paths {
		entries, e := os.ReadDir(p)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range entries {
			path := filepath.Join(p, v.Name())
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			st, e := os.Stat(path)
			if e != nil {
				t.Fatal(e)
			}
			out[path] = fmt.Sprintf("%s %d %d", Digest(raw), st.Mode(), st.ModTime().UnixNano())
		}
	}
	return out
}
func TestScopeRestartExactCopiesAndIsolation(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(fmt.Sprint(equal), func(t *testing.T) {
			o, s, l := restartFixture(t, equal)
			before := restartSnapshot(t, o.paths)
			out, e := readRetainedFeatureScope(context.Background(), o.paths, l)
			if e != nil || out.state == nil {
				t.Fatal("P1 exact retained reconstruction", e)
			}
			got := out.Scope()
			if !reflect.DeepEqual(got.state, s.state) || got.ManifestDigest() == l.Scope.ID || len(out.LinkedReceiptBytes()) != len(o.plan.state.objects)+1 {
				t.Fatal("P1 exact scope/domain digest/storage-only dedup")
			}
			m := got.ManifestBytes()
			m[0] = '!'
			f := got.Files()
			f[0].Bytes[0] = '!'
			a := out.HeadBytes()
			a[0] = '!'
			md := out.MetadataBytes()
			md[0] = '!'
			rr := out.LinkedReceiptBytes()
			rr[0][0] = '!'
			got.state.manifest[0] = '!'
			if !bytes.Equal(out.Scope().ManifestBytes(), s.ManifestBytes()) || !bytes.Equal(out.Scope().Files()[0].Bytes, s.Files()[0].Bytes) || out.HeadBytes()[0] == '!' || out.MetadataBytes()[0] == '!' || out.LinkedReceiptBytes()[0][0] == '!' {
				t.Fatal("P4 accessor alias")
			}
			if !reflect.DeepEqual(before, restartSnapshot(t, o.paths)) {
				t.Fatal("P2 read-only observation wrote")
			}
		})
	}
}
func TestScopeRestartCopyAndReceiptFaults(t *testing.T) {
	for _, kind := range []string{"primary-missing", "backup-missing", "primary-corrupt", "backup-divergent", "manifest-missing", "metadata-corrupt", "receipt-missing", "receipt-corrupt", "head-corrupt", "head-key", "head-digest", "head-length", "scope-id", "scope-length", "metadata-id", "metadata-length", "wrong-root", "root-alias", "journal-alias", "root-mode", "root-symlink", "tmp-root"} {
		t.Run(kind, func(t *testing.T) {
			o, _, l := restartFixture(t, false)
			paths := o.paths
			var path string
			switch kind {
			case "primary-missing", "primary-corrupt":
				path = filepath.Join(paths[0], o.item(0).digest)
			case "backup-missing", "backup-divergent":
				path = filepath.Join(paths[1], o.item(1).digest)
			case "manifest-missing":
				path = filepath.Join(paths[0], l.Scope.ID)
			case "metadata-corrupt":
				path = filepath.Join(paths[1], l.Metadata.ID)
			case "receipt-missing", "receipt-corrupt":
				var h scopeBundleHead
				_ = json.Unmarshal(mustReadRestart(t, bundleHeadPath(o)), &h)
				path = filepath.Join(paths[2], scopeBundleReceiptSelector(h.Receipt))
			case "head-corrupt":
				path = bundleHeadPath(o)
			case "head-key":
				l.HeadKey = Digest([]byte("other"))
			case "head-digest":
				l.Head.ID = Digest([]byte("other"))
			case "head-length":
				l.Head.ByteLength++
			case "scope-id":
				l.Scope.ID = Digest([]byte("other"))
				l.HeadKey = scopeBundleKey(scopeBundleBinding{Scope: l.Scope.ID, Metadata: l.Metadata.ID, Policy: scopeBundlePolicy})
			case "scope-length":
				l.Scope.ByteLength++
			case "metadata-id":
				l.Metadata.ID = Digest([]byte("other"))
				l.HeadKey = scopeBundleKey(scopeBundleBinding{Scope: l.Scope.ID, Metadata: l.Metadata.ID, Policy: scopeBundlePolicy})
			case "metadata-length":
				l.Metadata.ByteLength++
			case "wrong-root":
				if e := os.Rename(paths[1], paths[1]+"-old"); e != nil {
					t.Fatal(e)
				}
				if e := os.Mkdir(paths[1], 0700); e != nil {
					t.Fatal(e)
				}
			case "root-alias":
				paths[1] = paths[0]
			case "journal-alias":
				paths[2] = paths[0]
			case "root-mode":
				if e := os.Chmod(paths[0], 0755); e != nil {
					t.Fatal(e)
				}
			case "root-symlink":
				p := paths[0] + "-link"
				if e := os.Symlink(paths[0], p); e != nil {
					t.Fatal(e)
				}
				paths[0] = p
			case "tmp-root":
				paths[0] = "/private/tmp"
			}
			if path != "" {
				var e error
				if strings.HasSuffix(kind, "missing") {
					e = os.Remove(path)
				} else {
					e = os.WriteFile(path, []byte("divergent"), 0600)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			// Snapshot the original roots only; substituted roots cannot acquire a writer.
			before := restartSnapshot(t, o.paths)
			out, e := readRetainedFeatureScope(context.Background(), paths, l)
			if e == nil || out.state != nil {
				t.Fatal("P1 bad retained input admitted", kind, e)
			}
			if !reflect.DeepEqual(before, restartSnapshot(t, o.paths)) {
				t.Fatal("P2 refusal wrote")
			}
		})
	}
}
func mustReadRestart(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// Rebind only disposable fixture objects and their head to reach strict bootstrap
// checks behind the hash pins. Receipts intentionally remain unmodified; each
// case must refuse BEFORE looking up receipts, not merely fail at the prefix.
func restartTamperBootstrap(t *testing.T, o *scopeBundleOwner, l scopeRestartLocator, kind string) scopeRestartLocator {
	t.Helper()
	manifest := mustReadRestart(t, filepath.Join(o.paths[0], l.Scope.ID))
	metadata := mustReadRestart(t, filepath.Join(o.paths[0], l.Metadata.ID))
	var m scopeManifest
	var md scopeRestartMetadata
	_ = json.Unmarshal(manifest, &m)
	_ = json.Unmarshal(metadata, &md)
	payload := map[int][]byte{}
	switch kind {
	case "authority":
		m.Authority = 1
	case "accepted":
		m.Accepted = true
	case "completeness":
		m.Completeness = "COMPLETE"
	case "identity":
		m.FeatureIdentity = "RESOLVED"
	case "revision":
		m.RevisionAssertion = "wrong"
	case "qualifier":
		m.RevisionQualification = "AUTHENTICATED"
	case "excluded":
		m.ExcludedRule = "NONE"
	case "included":
		m.IncludedRule = "ALL"
	case "role":
		m.Files[0].Role = "CONTEXT_ONLY"
	case "path":
		m.Files[0].Path = "other.go"
	case "uri":
		m.Files[0].URI = "file:///other.go"
	case "file-identity":
		m.Files[0].Identity.Inode = 0
	case "root-identity":
		m.RootIdentity.Inode = 0
	case "file-count":
		m.Files = m.Files[:5]
	case "partition-size":
		m.PartitionSize = 3
	case "selection":
		payload[6] = bytes.Replace(o.plan.state.objects[md.References[6].Object].raw, []byte(`"authority":0`), []byte(`"authority":1`), 1)
	case "plan":
		payload[7] = bytes.Replace(o.plan.state.objects[md.References[7].Object].raw, []byte(`"partition_size":2`), []byte(`"partition_size":3`), 1)
	case "metadata-schema":
		md.Schema = "unknown"
	case "metadata-authority":
		md.Authority = 1
	case "metadata-role":
		md.References[0].Role = "SOURCE_SELECTION"
	case "metadata-path":
		md.References[0].Path = "other.go"
	case "metadata-count":
		md.References = md.References[:8]
	case "metadata-ordinal":
		md.References[0].Object = 8
	case "metadata-length":
		md.References[0].ByteLength = featureScopeFileLimit + 1
	case "metadata-overflow":
		md.References[0].ByteLength = math.MaxUint64
	}
	for i, raw := range payload {
		r := &md.References[i]
		r.SHA256 = Digest(raw)
		r.ByteLength = uint64(len(raw))
		if i == 6 {
			m.SelectionSHA256 = r.SHA256
			m.SelectionByteLength = r.ByteLength
		} else {
			m.PlanSHA256 = r.SHA256
			m.PlanByteLength = r.ByteLength
		}
	}
	manifest, _ = json.Marshal(m)
	if kind == "manifest-duplicate" {
		manifest = bytes.Replace(manifest, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1)
	}
	if kind == "manifest-unknown" {
		manifest = bytes.Replace(manifest, []byte(`"authority":0`), []byte(`"other":0,"authority":0`), 1)
	}
	if kind == "manifest-space" {
		manifest = append(manifest, ' ')
	}
	l.Scope = restartPin(manifest)
	if len(md.References) == 9 {
		md.References[8].SHA256 = l.Scope.ID
		md.References[8].ByteLength = l.Scope.ByteLength
	}
	metadata, _ = json.Marshal(md)
	if kind == "metadata-duplicate" {
		metadata = bytes.Replace(metadata, []byte(`"authority":0`), []byte(`"authority":0,"authority":0`), 1)
	}
	if kind == "metadata-unknown" {
		metadata = bytes.Replace(metadata, []byte(`"authority":0`), []byte(`"unknown":0,"authority":0`), 1)
	}
	l.Metadata = restartPin(metadata)
	for _, p := range o.paths[:2] {
		for _, raw := range append([][]byte{manifest, metadata}, mapValuesRestart(payload)...) {
			if e := os.WriteFile(filepath.Join(p, Digest(raw)), raw, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	h, e := parseScopeBundleHead(mustReadRestart(t, bundleHeadPath(o)))
	if e != nil {
		t.Fatal(e)
	}
	h.Binding.Scope = l.Scope.ID
	h.Binding.Metadata = l.Metadata.ID
	h.Key = scopeBundleKey(h.Binding)
	l.HeadKey = h.Key
	raw := scopeBundleEncode(h)
	l.Head = restartPin(raw)
	if e = os.WriteFile(filepath.Join(o.paths[2], "feature-scope-bundle-head-private-v1-"+h.Key[7:]+".json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	return l
}
func mapValuesRestart(m map[int][]byte) [][]byte {
	var out [][]byte
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
func TestScopeRestartStrictBootstrap(t *testing.T) {
	for _, kind := range []string{"authority", "accepted", "completeness", "identity", "revision", "qualifier", "excluded", "included", "role", "path", "uri", "file-identity", "root-identity", "file-count", "partition-size", "selection", "plan", "manifest-duplicate", "manifest-unknown", "manifest-space", "metadata-schema", "metadata-authority", "metadata-role", "metadata-path", "metadata-count", "metadata-ordinal", "metadata-length", "metadata-overflow", "metadata-duplicate", "metadata-unknown"} {
		t.Run(kind, func(t *testing.T) {
			o, _, l := restartFixture(t, false)
			l = restartTamperBootstrap(t, o, l, kind)
			ops := defaultScopeRestartOps()
			calls := 0
			ops.receipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
				calls++
				return nil, errors.New("must not inspect chain")
			}
			out, e := readRetainedFeatureScopeWithOps(context.Background(), o.paths, l, ops)
			if e == nil || out.state != nil || calls != 0 {
				t.Fatal("P1 strict bootstrap not rejected before receipts", e, calls)
			}
		})
	}
}
func TestScopeRestartRefusesNonterminalWithoutObjectOrReceiptReads(t *testing.T) {
	for _, kind := range []string{"CLAIMED", "READY", "FAILED", "UNKNOWN"} {
		t.Run(kind, func(t *testing.T) {
			o, _ := bundleFixture(t, false)
			first, e := o.advance(context.Background(), nil)
			if e != nil {
				t.Fatal(e)
			}
			raw := first.head
			if kind == "READY" {
				v, e := o.advance(context.Background(), first.pending)
				if e != nil {
					t.Fatal(e)
				}
				raw = v.head
			}
			if kind == "FAILED" {
				h, _ := parseScopeBundleHead(raw)
				h.State = "FAILED"
				h.Generation = 2
				h.Predecessor = Digest(raw)
				h.Receipt = Digest(first.receipt)
				raw = scopeBundleEncode(h)
			}
			if kind == "UNKNOWN" {
				raw = bytes.Replace(raw, []byte(`"CLAIMED"`), []byte(`"UNKNOWN"`), 1)
			}
			if e = os.WriteFile(bundleHeadPath(o), raw, 0600); e != nil {
				t.Fatal(e)
			}
			l := scopeRestartLocator{scopeBundleKey(*o.binding), restartPin(raw), restartPin(o.plan.state.objects[o.plan.state.references[8].Object].raw), restartPin(o.plan.metadataBytes())}
			ops := defaultScopeRestartOps()
			calls := 0
			ops.get = func(context.Context, *publication.ContentRetentionRoot, string, int64) ([]byte, error) {
				calls++
				return nil, ErrInvalid
			}
			ops.receipt = func(context.Context, *publication.ContentRetentionRoot, string) ([]byte, error) {
				calls++
				return nil, ErrInvalid
			}
			before := restartSnapshot(t, o.paths)
			out, e := readRetainedFeatureScopeWithOps(context.Background(), o.paths, l, ops)
			if e == nil || out.state != nil || calls != 0 || !reflect.DeepEqual(before, restartSnapshot(t, o.paths)) {
				t.Fatal("P1/P2 nonterminal read/wrote", kind, e, calls)
			}
		})
	}
}
func TestScopeRestartHeadLinkedOnly(t *testing.T) {
	o, _, l := restartFixture(t, true)
	// Malformed orphan deliberately has a receipt-shaped name but no link. Reader
	// must succeed rather than enumerate or infer a new predecessor from it.
	orphan := filepath.Join(o.paths[2], scopeBundleReceiptSelector(Digest([]byte("orphan"))))
	if e := os.WriteFile(orphan, []byte("malformed unlinked orphan"), 0600); e != nil {
		t.Fatal(e)
	}
	if out, e := readRetainedFeatureScope(context.Background(), o.paths, l); e != nil || out.state == nil {
		t.Fatal("P1 orphan scan", e)
	}
	h, _ := parseScopeBundleHead(mustReadRestart(t, bundleHeadPath(o)))
	h.Receipt = Digest([]byte("orphan"))
	raw := scopeBundleEncode(h)
	l.Head = restartPin(raw)
	if e := os.WriteFile(bundleHeadPath(o), raw, 0600); e != nil {
		t.Fatal(e)
	}
	if out, e := readRetainedFeatureScope(context.Background(), o.paths, l); e == nil || out.state != nil {
		t.Fatal("P1 unlinked prefix accepted")
	}
}
func TestScopeRestartTerminalAndPrefixReconstruction(t *testing.T) {
	for _, kind := range []string{"index", "generation", "binding", "claim", "object", "length", "success", "previous"} {
		t.Run(kind, func(t *testing.T) {
			o, _, l := restartFixture(t, true)
			h, _ := parseScopeBundleHead(mustReadRestart(t, bundleHeadPath(o)))
			switch kind {
			case "index":
				h.Index--
				h.Generation = uint64(2 * h.Index)
			case "generation":
				h.Generation++
			case "binding":
				h.Binding.Roots[0].Inode++
			default:
				var r scopeBundleReceipt
				_ = json.Unmarshal(mustReadRestart(t, filepath.Join(o.paths[2], scopeBundleReceiptSelector(h.Receipt))), &r)
				switch kind {
				case "claim":
					r.Claim = Digest([]byte("wrong"))
				case "object":
					r.Object = Digest([]byte("wrong"))
				case "length":
					r.Length++
				case "success":
					r.Success = false
				case "previous":
					r.Previous = Digest([]byte("wrong"))
				}
				raw, _ := json.Marshal(r)
				h.Receipt = Digest(raw)
				if e := os.WriteFile(filepath.Join(o.paths[2], scopeBundleReceiptSelector(h.Receipt)), raw, 0600); e != nil {
					t.Fatal(e)
				}
			}
			raw := scopeBundleEncode(h)
			l.Head = restartPin(raw)
			if e := os.WriteFile(bundleHeadPath(o), raw, 0600); e != nil {
				t.Fatal(e)
			}
			before := restartSnapshot(t, o.paths)
			if out, e := readRetainedFeatureScope(context.Background(), o.paths, l); e == nil || out.state != nil {
				t.Fatal("P1 reconstructed terminal/prefix mismatch admitted", kind, e)
			}
			if !reflect.DeepEqual(before, restartSnapshot(t, o.paths)) {
				t.Fatal("P2 prefix refusal wrote")
			}
		})
	}
}

func TestScopeRestartErrorsCancellationAndBounds(t *testing.T) {
	o, _, l := restartFixture(t, true)
	for _, kind := range []string{"cancel", "read-cancel", "close-cancel", "read-and-close", "open-and-close", "all-close", "head-change"} {
		t.Run(kind, func(t *testing.T) {
			ops := defaultScopeRestartOps()
			normal := ops
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, b := errors.New("acquire/read fault"), errors.New("close fault")
			opened, closed := 0, 0
			ops.open = func(c context.Context, p string) (*publication.ContentRetentionRoot, error) {
				r, e := normal.open(c, p)
				opened++
				if kind == "open-and-close" && opened == 2 {
					return r, errors.Join(e, a)
				}
				return r, e
			}
			ops.close = func(r *publication.ContentRetentionRoot) error {
				closed++
				e := normal.close(r)
				if kind == "close-cancel" {
					cancel()
				}
				if kind == "read-and-close" || kind == "open-and-close" || kind == "all-close" {
					return errors.Join(e, b)
				}
				return e
			}
			ops.get = func(c context.Context, r *publication.ContentRetentionRoot, k string, n int64) ([]byte, error) {
				if kind == "read-cancel" {
					cancel()
				}
				if kind == "read-and-close" {
					return nil, a
				}
				return normal.get(c, r, k, n)
			}
			if kind == "cancel" {
				cancel()
			}
			if kind == "head-change" {
				calls := 0
				ops.head = func(c context.Context, r *publication.ContentRetentionRoot, k string) ([]byte, error) {
					raw, e := normal.head(c, r, k)
					calls++
					if calls == 2 {
						raw = append(raw, ' ')
					}
					return raw, e
				}
			}
			out, e := readRetainedFeatureScopeWithOps(ctx, o.paths, l, ops)
			if e == nil || out.state != nil || opened != closed {
				t.Fatal("P3 fail-closed/lifecycle", kind, e, opened, closed)
			}
			if (kind == "cancel" || kind == "read-cancel" || kind == "close-cancel") && !errors.Is(e, context.Canceled) {
				t.Fatal("P3 lost cancellation", e)
			}
			if (kind == "read-and-close" || kind == "open-and-close") && (!errors.Is(e, a) || !errors.Is(e, b)) {
				t.Fatal("P3 lost sibling causes", e)
			}
			if kind == "all-close" && !errors.Is(e, b) {
				t.Fatal("P3 lost close", e)
			}
		})
	}
	for _, field := range []string{"head", "scope", "metadata"} {
		for _, n := range []uint64{0, math.MaxUint64, scopeBundleWireLimit + 1} {
			bad := l
			switch field {
			case "head":
				bad.Head.ByteLength = n
			case "scope":
				bad.Scope.ByteLength = n
			case "metadata":
				bad.Metadata.ByteLength = n
			}
			ops := defaultScopeRestartOps()
			ops.open = func(context.Context, string) (*publication.ContentRetentionRoot, error) {
				t.Fatal("P1 bounds opened root")
				return nil, nil
			}
			if out, e := readRetainedFeatureScopeWithOps(context.Background(), o.paths, bad, ops); e == nil || out.state != nil {
				t.Fatal("P1 locator bound")
			}
		}
	}
	if out, e := readRetainedFeatureScope(nil, o.paths, l); e == nil || out.state != nil {
		t.Fatal("nil context")
	}
	if out, e := readRetainedFeatureScope(context.Background(), o.paths, scopeRestartLocator{}); e == nil || out.state != nil {
		t.Fatal("zero locator")
	}
	var zero scopeRestartResult
	if zero.Scope().state != nil || zero.HeadBytes() != nil || zero.MetadataBytes() != nil || zero.LinkedReceiptBytes() != nil {
		t.Fatal("zero result")
	}
}

type scopeRestartRequest struct {
	Roots   [3]string           `json:"roots"`
	Locator scopeRestartLocator `json:"locator"`
}

// This helper cannot retain or freeze inputs. It receives only a bounded root+
// locator packet on stdin; no scope, revision, source path, or source bytes.
func TestScopeRestartSubprocessHelper(t *testing.T) {
	if os.Getenv("LSP_TRACE_SCOPE_RESTART_FIXTURE_HELPER") != "read-only-fixture-v1" {
		t.Skip("fixture helper only")
	}
	raw, e := io.ReadAll(io.LimitReader(os.Stdin, scopeBundleWireLimit+1))
	if e != nil {
		t.Fatal(e)
	}
	var req scopeRestartRequest
	if scopeBundleStrict(raw, &req) != nil {
		t.Fatal("invalid helper packet")
	}
	// This subprocess entrypoint is fixture-only, unlike the separate reviewed
	// real read-only opt-in. Never route real rehearsal paths through the helper.
	base := filepath.Dir(req.Roots[0])
	if !strings.HasPrefix(filepath.Base(base), ".scope-retention-fixture-") {
		t.Fatal("helper requires disposable fixture roots")
	}
	for i, name := range []string{"primary", "backup", "journal"} {
		if req.Roots[i] != filepath.Join(base, name) {
			t.Fatal("helper requires exact fixture root family")
		}
	}
	out, e := readRetainedFeatureScope(context.Background(), req.Roots, req.Locator)
	if e != nil {
		t.Fatal(e)
	}
	s := out.Scope()
	t.Logf("PROCESS_RESTART_VERIFIED raw_scope=%s domain_scope=%s selection=%s plan=%s refs=9 linked=%d qualifier=CALLER_ASSERTED_NOT_AUTHENTICATED", Digest(s.ManifestBytes()), s.ManifestDigest(), Digest(s.SelectionBytes()), Digest(s.PlanBytes()), len(out.LinkedReceiptBytes()))
}
func TestScopeRestartActualProcessWithoutSource(t *testing.T) {
	for _, equal := range []bool{false, true} {
		t.Run(fmt.Sprint(equal), func(t *testing.T) {
			// Explicitly own the source fixture here so only this known disposable root
			// can ever be deleted. Never decode a retained Root field as cleanup authority.
			source := scopeFixture(t)
			if equal {
				for _, p := range featureScopePaths() {
					if e := os.WriteFile(filepath.Join(source, p), []byte("package fixture\n"), 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			s, e := freezeFeatureScope(context.Background(), source, fixtureRevision, nil)
			if e != nil {
				t.Fatal(e)
			}
			o, _ := bundleFixture(t, false)
			o, e = newScopeBundleOwner(s, o.paths[0], o.paths[1], o.paths[2])
			if e != nil {
				t.Fatal(e)
			}
			final := runBundle(t, o)
			l := restartLocator(t, o, final)
			before := restartSnapshot(t, o.paths)
			want := s.ManifestDigest()
			if e = os.RemoveAll(source); e != nil {
				t.Fatal(e)
			}
			if _, e = os.Stat(source); !os.IsNotExist(e) {
				t.Fatal("source fixture still accessible", e)
			}
			packet, _ := json.Marshal(scopeRestartRequest{o.paths, l})
			cmd := exec.Command(os.Args[0], "-test.run=^TestScopeRestartSubprocessHelper$", "-test.v")
			// Minimal environment; deliberately omit all source/rehearsal variables.
			cmd.Env = []string{"LSP_TRACE_SCOPE_RESTART_FIXTURE_HELPER=read-only-fixture-v1"}
			cmd.Stdin = bytes.NewReader(packet)
			output, e := cmd.CombinedOutput()
			t.Log(string(output))
			if e != nil || !bytes.Contains(output, []byte("PROCESS_RESTART_VERIFIED raw_scope="+l.Scope.ID+" domain_scope="+want)) {
				t.Fatal("P1/P2 actual process reconstruction", e, string(output))
			}
			if !reflect.DeepEqual(before, restartSnapshot(t, o.paths)) {
				t.Fatal("P2 subprocess wrote retained roots")
			}
		})
	}
}

// Opt-in READ ONLY after independent review. This is intentionally separate from
// the write-capable rehearsal. The locator file is generated from the prior
// native report, never a hardcoded real root/revision/manifest constant.
func TestScopeRealRetainedRestartReadOnly(t *testing.T) {
	opt := os.Getenv("LSP_TRACE_SCOPE_RESTART")
	if opt == "" {
		t.Skip("opt-in only: LSP_TRACE_SCOPE_RESTART=read-retained-inputs")
	}
	if opt != "read-retained-inputs" {
		t.Fatal("invalid restart opt-in")
	}
	path := os.Getenv("LSP_TRACE_SCOPE_RESTART_LOCATOR_FILE")
	if path == "" {
		t.Fatal("exact generated locator file required")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	raw, re := io.ReadAll(io.LimitReader(f, scopeBundleWireLimit+1))
	if e = errors.Join(re, f.Close()); e != nil {
		t.Fatal(e)
	}
	var req scopeRestartRequest
	if scopeBundleStrict(raw, &req) != nil {
		t.Fatal("invalid canonical locator packet")
	}
	out, e := readRetainedFeatureScope(context.Background(), req.Roots, req.Locator)
	if e != nil {
		t.Fatal(e)
	}
	s := out.Scope()
	report := struct {
		Verification string           `json:"verification"`
		Authority    uint8            `json:"authority"`
		Accepted     bool             `json:"accepted"`
		Completeness string           `json:"completeness"`
		Identity     string           `json:"feature_identity"`
		Qualifier    string           `json:"revision_qualification"`
		Scope        scopeRestartBlob `json:"scope"`
		Domain       string           `json:"scope_domain_digest"`
		Selection    scopeRestartBlob `json:"selection"`
		Plan         scopeRestartBlob `json:"plan"`
		Head         scopeRestartBlob `json:"head"`
		Metadata     scopeRestartBlob `json:"metadata"`
		Linked       int              `json:"linked_receipt_count"`
	}{"RETAINED_INPUT_BUNDLE_READ_ONLY_RECONSTRUCTED", 0, false, "UNKNOWN", "UNRESOLVED", "CALLER_ASSERTED_NOT_AUTHENTICATED", restartPin(s.ManifestBytes()), s.ManifestDigest(), restartPin(s.SelectionBytes()), restartPin(s.PlanBytes()), restartPin(out.HeadBytes()), restartPin(out.MetadataBytes()), len(out.LinkedReceiptBytes())}
	encoded, e := json.Marshal(report)
	if e != nil {
		t.Fatal(e)
	}
	t.Log(string(encoded))
}
