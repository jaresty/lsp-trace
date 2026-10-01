package adr0011lifecycle

// This is an explicitly PARTIAL private synthetic test path, not a V2 lifecycle
// record writer, canonical RFC 8785 codec, independently provisioned authority,
// durable lease implementation, or production admission operation. Its selectors
// intentionally do not match V2 selectors. No caller outside this package exists.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"lsp-trace/internal/publication"
)

const (
	partialSchemaHash = "a8bc22e5cbf95cb317660cac115fbc7ce0f5c24c44b68af230721d45b2587801"
	partialPolicyHash = "a2aef1faafde98da6b14d9596c6c727ab9c304cb638d2e9d7bde563e4ee345d0"
	partialPrefix     = "partial-adr0011-"
	partialMaxObjects = 4096
	partialMaxRecord  = 1048576
	partialMaxTotal   = 67108864
)

type partialState struct {
	epoch  uint64
	head   string
	fenced bool
}

// Fixed-field JSON is checked byte-for-byte on replay. It is NOT the V2 JCS
// representation, and its refs are NOT V2 privateRef/hostSlotRef objects.
type partialRecord struct {
	Format     string `json:"format"`
	Role       string `json:"role"`
	Kind       string `json:"kind"`
	Epoch      uint64 `json:"epoch"`
	Root       string `json:"root"`
	Anchor     string `json:"anchor"`
	Schema     string `json:"schema"`
	Policy     string `json:"policy"`
	Previous   string `json:"previous"`
	Next       string `json:"next"`
	Prepare    string `json:"prepare"`
	Transition string `json:"transition"`
}

var partialRoles = map[string]string{
	"genesis": "ADR0011_TRUSTED_HOST_ROOT_V2", "fence": "ADR0011_FENCE_INTENT_V2",
	"prepare": "ADR0011_HOST_HEAD_PREPARE_V2", "transition": "ADR0011_HEAD_TRANSITION_V2", "commit": "ADR0011_HOST_HEAD_COMMIT_V2",
}

func partialHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func partialPins(schema, policy []byte) error {
	if partialHash(schema) != partialSchemaHash || partialHash(policy) != partialPolicyHash {
		return errors.New("selected V2 schema/policy bytes mismatch")
	}
	return nil
}
func (l *syntheticRoot) partialIdentity() (string, string, error) {
	root, err := directoryIdentity(l.rootInfo)
	if err != nil {
		return "", "", err
	}
	anchor, err := directoryIdentity(l.anchorInfo)
	return root, anchor, err
}
func (l *syntheticRoot) partialBase(kind string, epoch uint64, schema, policy []byte) (partialRecord, error) {
	root, anchor, err := l.partialIdentity()
	if err != nil {
		return partialRecord{}, err
	}
	return partialRecord{Format: "PARTIAL_PRIVATE_SYNTHETIC_NOT_V2", Role: partialRoles[kind], Kind: kind, Epoch: epoch, Root: root, Anchor: anchor, Schema: partialHash(schema), Policy: partialHash(policy)}, nil
}
func partialSelector(r partialRecord) (string, []byte, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", nil, err
	}
	if len(raw) > partialMaxRecord {
		return "", nil, errors.New("partial record exceeds byte limit")
	}
	return fmt.Sprintf("%s%s-v2-%s.json", partialPrefix, r.Kind, partialHash(raw)), raw, nil
}
func partialKind(name string) (string, bool) {
	for k := range partialRoles {
		if strings.HasPrefix(name, partialPrefix+k+"-v2-") {
			return k, true
		}
	}
	return "", false
}
func (l *syntheticRoot) partialRead(dir *publication.Root, name, kind string) (partialRecord, error) {
	if !strings.HasPrefix(name, partialPrefix+kind+"-v2-") || strings.ContainsAny(name, "/\\") {
		return partialRecord{}, errors.New("partial selector role mismatch")
	}
	path := filepath.Join(dir.Path(), name)
	before, err := os.Lstat(path)
	if err != nil {
		return partialRecord{}, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm() != 0600 || !singleLink(before) {
		return partialRecord{}, errors.New("partial selector mode or link count invalid")
	}
	f, err := openLockNoFollow(path)
	if err != nil {
		return partialRecord{}, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) || after.Size() > partialMaxRecord {
		return partialRecord{}, errors.New("partial selector inode or byte limit invalid")
	}
	raw, err := io.ReadAll(io.LimitReader(f, partialMaxRecord+1))
	if err != nil || len(raw) > partialMaxRecord {
		return partialRecord{}, errors.New("partial selector read limit invalid")
	}
	var r partialRecord
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, err
	}
	selector, canonical, err := partialSelector(r)
	if err != nil || selector != name || !bytes.Equal(canonical, raw) || r.Format != "PARTIAL_PRIVATE_SYNTHETIC_NOT_V2" || r.Kind != kind || r.Role != partialRoles[kind] {
		return r, errors.New("partial selector bytes, domain or role mismatch")
	}
	return r, nil
}
func (l *syntheticRoot) partialPublish(dir *publication.Root, r partialRecord) (string, error) {
	name, raw, err := partialSelector(r)
	if err != nil {
		return "", err
	}
	if err := l.validate(); err != nil {
		return "", err
	}
	f, err := os.OpenFile(filepath.Join(dir.Path(), name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	n, writeErr := f.Write(raw)
	syncErr := error(nil)
	if writeErr == nil && n == len(raw) {
		syncErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || n != len(raw) || syncErr != nil || closeErr != nil {
		return "", errors.Join(writeErr, syncErr, closeErr, errors.New("partial publication uncertain"))
	}
	got, err := l.partialRead(dir, name, r.Kind)
	if err != nil || got != r {
		return "", errors.Join(err, errors.New("partial publication readback mismatch"))
	}
	synced, err := dir.SyncDirectory()
	if err != nil || !synced {
		return "", errors.Join(err, errors.New("partial directory sync unverified"))
	}
	if err := l.validate(); err != nil {
		return "", err
	}
	return name, nil
}
func (l *syntheticRoot) partialEnumerate(dir *publication.Root, host bool) (map[string]map[uint64]string, error) {
	entries, err := os.ReadDir(dir.Path())
	if err != nil {
		return nil, err
	}
	if len(entries) > partialMaxObjects+1 {
		return nil, errors.New("partial object limit exceeded")
	}
	result := make(map[string]map[uint64]string)
	total := int64(0)
	for _, e := range entries {
		name := e.Name()
		if !host && name == "adr0011-root.lock" {
			continue
		}
		kind, ok := partialKind(name)
		if !ok || (host != (kind == "genesis" || kind == "prepare" || kind == "commit")) {
			return nil, fmt.Errorf("unexpected partial namespace member: %q", name)
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		total += info.Size()
		if info.Size() > partialMaxRecord || total > partialMaxTotal {
			return nil, errors.New("partial total/record byte limit exceeded")
		}
		r, err := l.partialRead(dir, name, kind)
		if err != nil {
			return nil, err
		}
		root, anchor, err := l.partialIdentity()
		if err != nil {
			return nil, err
		}
		if r.Root != root || r.Anchor != anchor || r.Schema != partialSchemaHash || r.Policy != partialPolicyHash {
			return nil, errors.New("partial root/anchor/pins identity mismatch")
		}
		if result[kind] == nil {
			result[kind] = make(map[uint64]string)
		}
		if result[kind][r.Epoch] != "" {
			return nil, errors.New("partial duplicate/fork at epoch")
		}
		result[kind][r.Epoch] = name
	}
	return result, nil
}
func (l *syntheticRoot) partialBudget() error {
	count := 0
	total := int64(0)
	for _, dir := range []*publication.Root{l.root, l.anchor} {
		entries, err := os.ReadDir(dir.Path())
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() == "adr0011-root.lock" && dir == l.root {
				continue
			}
			count++
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			if count > partialMaxObjects || total > partialMaxTotal {
				return errors.New("partial global object/byte limit exceeded")
			}
		}
	}
	return nil
}
func (l *syntheticRoot) partialReplay(schema, policy []byte) (partialState, error) {
	if err := partialPins(schema, policy); err != nil {
		return partialState{}, err
	}
	if err := l.validate(); err != nil {
		return partialState{}, err
	}
	root, err := l.partialEnumerate(l.root, false)
	if err != nil {
		return partialState{}, err
	}
	host, err := l.partialEnumerate(l.anchor, true)
	if err != nil {
		return partialState{}, err
	}
	if err := l.partialBudget(); err != nil {
		return partialState{}, err
	}
	if len(host["genesis"]) != 1 || host["genesis"][1] == "" {
		return partialState{}, errors.New("partial independently provisioned genesis missing or invalid")
	}
	g, err := l.partialRead(l.anchor, host["genesis"][1], "genesis")
	if err != nil {
		return partialState{}, err
	}
	expected, err := l.partialBase("genesis", 1, schema, policy)
	if err != nil {
		return partialState{}, err
	}
	if g != expected {
		return partialState{}, errors.New("partial genesis bytes differ from selected provision")
	}
	state := partialState{epoch: 1, head: host["genesis"][1]}
	commits := host["commit"]
	if len(commits) > partialMaxObjects/4 {
		return partialState{}, errors.New("partial transition bound exceeded")
	}
	for i := uint64(2); i < 2+uint64(len(commits)); i++ {
		names := []string{host["prepare"][i], root["fence"][i], root["transition"][i], commits[i]}
		for _, n := range names {
			if n == "" {
				return partialState{}, errors.New("partial epoch gap or unverified commit")
			}
		}
		prepare, e := l.partialRead(l.anchor, names[0], "prepare")
		if e != nil {
			return partialState{}, e
		}
		fence, e := l.partialRead(l.root, names[1], "fence")
		if e != nil {
			return partialState{}, e
		}
		transition, e := l.partialRead(l.root, names[2], "transition")
		if e != nil {
			return partialState{}, e
		}
		commit, e := l.partialRead(l.anchor, names[3], "commit")
		if e != nil {
			return partialState{}, e
		}
		for _, r := range []partialRecord{prepare, fence, transition, commit} {
			if r.Epoch != i || r.Previous != state.head {
				return partialState{}, errors.New("partial stale, divergent or forked head")
			}
		}
		if fence.Next != "" || prepare.Next != names[1] || transition.Next != names[1] || commit.Next != names[1] {
			return partialState{}, errors.New("partial next-head divergence")
		}
		if prepare.Prepare != "" || prepare.Transition != "" || fence.Prepare != "" || fence.Transition != "" || transition.Prepare != names[0] || transition.Transition != "" || commit.Prepare != names[0] || commit.Transition != names[2] {
			return partialState{}, errors.New("partial host/root reference divergence")
		}
		state = partialState{epoch: i, head: names[1], fenced: true}
	}
	for kind, epochs := range host {
		for epoch := range epochs {
			if kind != "genesis" && (epoch < 2 || epoch > state.epoch) {
				return partialState{}, errors.New("partial orphan host slot")
			}
		}
	}
	for _, epochs := range root {
		for epoch := range epochs {
			if epoch < 2 || epoch > state.epoch {
				return partialState{}, errors.New("partial orphan root record")
			}
		}
	}
	if len(root["fence"]) != len(commits) || len(root["transition"]) != len(commits) || len(host["prepare"]) != len(commits) {
		return partialState{}, errors.New("partial unverified prepare, transition or fence")
	}
	return state, nil
}
func (l *syntheticRoot) provisionPartial(schema, policy []byte) error {
	if err := partialPins(schema, policy); err != nil {
		return err
	}
	return l.withLock(func() error {
		root, err := l.partialEnumerate(l.root, false)
		if err != nil {
			return err
		}
		host, err := l.partialEnumerate(l.anchor, true)
		if err != nil {
			return err
		}
		if len(root) != 0 || len(host) != 0 {
			return errors.New("partial genesis requires empty synthetic directories")
		}
		g, err := l.partialBase("genesis", 1, schema, policy)
		if err != nil {
			return err
		}
		_, err = l.partialPublish(l.anchor, g)
		return err
	})
}
func (l *syntheticRoot) partialSnapshot(schema, policy []byte) (state partialState, err error) {
	err = l.withLock(func() error { var e error; state, e = l.partialReplay(schema, policy); return e })
	return
}

// partialCheckSyntheticUse tests denial only; nil is not a durable use lease.
func (l *syntheticRoot) partialCheckSyntheticUse(schema, policy []byte, cached partialState) error {
	return l.withLock(func() error {
		current, err := l.partialReplay(schema, policy)
		if err != nil {
			return err
		}
		if current.epoch != cached.epoch || current.head != cached.head {
			return errors.New("stale cached partial head denied")
		}
		if current.fenced {
			return errors.New("verified partial fence denies use")
		}
		// No durable lease exists: this is only a synthetic denial oracle, NOT admission.
		return nil
	})
}
func (l *syntheticRoot) partialFence(schema, policy []byte, previous partialState) error {
	if err := partialPins(schema, policy); err != nil {
		return err
	}
	return l.withLock(func() error {
		current, err := l.partialReplay(schema, policy)
		if err != nil {
			return err
		}
		if current != previous || current.fenced {
			return errors.New("stale or already fenced partial head")
		}
		if current.epoch >= 9007199254740991 {
			return errors.New("partial epoch exhausted")
		}
		epoch := current.epoch + 1
		fence, err := l.partialBase("fence", epoch, schema, policy)
		if err != nil {
			return err
		}
		fence.Previous = current.head
		// The fence cannot hash-reference itself; the next three records bind its selector.
		next, err := l.partialPublish(l.root, fence)
		if err != nil {
			return err
		}
		prepare, err := l.partialBase("prepare", epoch, schema, policy)
		if err != nil {
			return err
		}
		prepare.Previous = current.head
		prepare.Next = next
		prepareName, err := l.partialPublish(l.anchor, prepare)
		if err != nil {
			return err
		}
		transition, err := l.partialBase("transition", epoch, schema, policy)
		if err != nil {
			return err
		}
		transition.Previous = current.head
		transition.Next = next
		transition.Prepare = prepareName
		transitionName, err := l.partialPublish(l.root, transition)
		if err != nil {
			return err
		}
		commit, err := l.partialBase("commit", epoch, schema, policy)
		if err != nil {
			return err
		}
		commit.Previous = current.head
		commit.Next = next
		commit.Prepare = prepareName
		commit.Transition = transitionName
		if _, err = l.partialPublish(l.anchor, commit); err != nil {
			return err
		}
		_, err = l.partialReplay(schema, policy)
		return err
	})
}
