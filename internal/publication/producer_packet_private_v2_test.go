//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceProtectionRootPrivateV2(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir()
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	source, err := OpenSourceProtectionRootPrivateV2(ctx, path)
	if err != nil {
		t.Fatal("ASSERT_SOURCE_TMP_0755_TOPOLOGY_ONLY", err)
	}
	got, err := source.PathPrivateV2()
	if err != nil || got != path {
		t.Fatal(err)
	}
	_, ino, err := source.IdentityPrivateV2()
	if err != nil || ino == 0 {
		t.Fatal(err)
	}
	if r, e := OpenObservedRoot(ctx, path); e == nil {
		r.Close()
		t.Fatal("ASSERT_SOURCE_PROTECTION_DOES_NOT_WIDEN_OBSERVED_ROOT")
	}
	copy := *source
	if _, e := copy.PathPrivateV2(); e == nil {
		t.Fatal("ASSERT_SOURCE_CAP_SELF_IDENTITY")
	}
	alias := filepath.Join(t.TempDir(), "source-link")
	if err = os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if r, e := OpenSourceProtectionRootPrivateV2(ctx, alias); e == nil {
		r.Close()
		t.Fatal("ASSERT_SOURCE_SYMLINK_REFUSED")
	}
	parent := filepath.Dir(alias)
	ancestorAlias := parent + "-alias"
	if err = os.Symlink(parent, ancestorAlias); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ancestorAlias)
	if r, e := OpenSourceProtectionRootPrivateV2(ctx, filepath.Join(ancestorAlias, "source-link")); e == nil {
		r.Close()
		t.Fatal("ASSERT_SOURCE_SYMLINK_ANCESTOR_REFUSED")
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, e := source.IdentityPrivateV2(); e == nil {
		t.Fatal("ASSERT_SOURCE_CLOSE_INVALIDATES")
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestSourceProtectionRootPrivateV2CloseFailureConsumes(t *testing.T) {
	path := t.TempDir()
	path, _ = filepath.EvalSymlinks(path)
	source, err := OpenSourceProtectionRootPrivateV2(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err == nil {
		t.Fatal("ASSERT_SOURCE_CLOSE_ERROR_RETAINED")
	}
	if _, _, err = source.IdentityPrivateV2(); err == nil {
		t.Fatal("ASSERT_SOURCE_CLOSE_ERROR_INVALIDATES")
	}
}
func TestProducerPacketTopologyPrivateV2(t *testing.T) {
	for _, kind := range []string{"source-tmp-0755", "equal-outputs", "source-equals-output", "source-under-output", "source-above-output", "optional-alias", "optional-input-input-alias", "over-count", "closed-source", "cancel", "output-tmp", "output-mode", "source-replaced"} {
		t.Run(kind, func(t *testing.T) {
			old, ins, paths := retentionTopologyFixturePrivate(t)
			outs := [2]*ContentRetentionRoot{old[0], old[1]}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			path := t.TempDir()
			path, _ = filepath.EvalSymlinks(path)
			if e := os.Chmod(path, 0755); e != nil {
				t.Fatal(e)
			}
			var additional []*ObservedRoot
			switch kind {
			case "equal-outputs":
				outs[1] = outs[0]
			case "source-equals-output":
				path = paths[0]
			case "source-under-output":
				path = filepath.Join(paths[0], "owned-source")
				if e := os.Mkdir(path, 0755); e != nil {
					t.Fatal(e)
				}
			case "source-above-output":
				path = filepath.Dir(paths[0])
			case "optional-alias":
				r, e := OpenObservedRoot(ctx, paths[0])
				if e != nil {
					t.Fatal(e)
				}
				defer r.Close()
				additional = append(additional, r)
			case "optional-input-input-alias":
				additional = []*ObservedRoot{ins[0], ins[0]}
			case "over-count":
				for len(additional) < 16 {
					additional = append(additional, ins[0])
				}
			case "output-tmp":
				p := t.TempDir()
				p, _ = filepath.EvalSymlinks(p)
				r, e := OpenContentRetentionRoot(ctx, p)
				if e != nil {
					return
				}
				defer r.Close()
				outs[0] = r
			case "output-mode":
				if e := os.Chmod(paths[0], 0755); e != nil {
					t.Fatal(e)
				}
			}
			source, e := OpenSourceProtectionRootPrivateV2(ctx, path)
			if e != nil {
				t.Fatal(e)
			}
			defer source.Close()
			switch kind {
			case "closed-source":
				if e = source.Close(); e != nil {
					t.Fatal(e)
				}
			case "cancel":
				cancel()
			case "source-replaced":
				if e = os.Rename(path, path+"-moved"); e != nil {
					t.Fatal(e)
				}
				if e = os.Mkdir(path, 0755); e != nil {
					t.Fatal(e)
				}
				defer os.RemoveAll(path + "-moved")
			}
			err := CheckProducerPacketRetentionTopologyPrivateV2(ctx, outs, source, additional)
			allowed := kind == "source-tmp-0755" || kind == "optional-input-input-alias"
			if (err == nil) != allowed {
				t.Fatal("ASSERT_PACKET_TOPOLOGY", kind, err)
			}
		})
	}
}
func TestProducerPacketContentPrivateV2FixedProfile(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	r, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	raw := bytes.Repeat([]byte("x"), int(ProducerPacketPrivateV2ObjectLimit))
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	ctx := context.Background()
	if _, e := r.Publish(ctx, id, raw); e == nil {
		t.Fatal("ASSERT_PACKET_LEGACY_2M_UNCHANGED")
	}
	receipt, err := r.PublishProducerPacketPrivateV2(ctx, id, raw)
	if err != nil || receipt == nil {
		t.Fatal("ASSERT_PACKET_FIXED_16M_ACCEPTS_EQUALITY", err)
	}
	got, err := r.ReadProducerPacketPrivateV2(ctx, id, int64(len(raw)))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("ASSERT_PACKET_FIXED_16M_REREAD", err)
	}
	if _, e := r.Read(ctx, id, int64(len(raw))); e == nil {
		t.Fatal("ASSERT_PACKET_OLD_READ_PROFILE_UNCHANGED")
	}
	raw = append(raw, 'x')
	id = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	if _, e := r.PublishProducerPacketPrivateV2(ctx, id, raw); e == nil {
		t.Fatal("ASSERT_PACKET_FIXED_16M_ONE_OVER_REFUSED")
	}
	if _, e := r.ReadProducerPacketPrivateV2(ctx, id, ProducerPacketPrivateV2ObjectLimit+1); e == nil {
		t.Fatal("ASSERT_PACKET_READ_ONE_OVER_REFUSED")
	}
}
