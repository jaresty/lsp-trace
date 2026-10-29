//go:build darwin || linux

package publication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPostPreparationPrivateV2FixedProfile(t *testing.T) {
	path, _, _ := observedRootFixture(t)
	r, err := OpenContentRetentionRoot(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, n := range []int{4<<20 + 1, 16 << 20, 16<<20 + 1} {
		raw := bytes.Repeat([]byte{'v'}, n)
		id := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
		if _, err := r.Publish(context.Background(), id, raw); err == nil {
			t.Fatal("ASSERT_V2_OLD_2M_UNCHANGED")
		}
		if _, err := r.PublishPartialInspectionV2Private(context.Background(), id, raw); err == nil {
			t.Fatal("ASSERT_V2_OLD_4M_UNCHANGED")
		}
		receipt, err := r.PublishPostPreparationPrivateV2(context.Background(), id, raw)
		if n > 16<<20 {
			if err == nil || receipt != nil {
				t.Fatal("ASSERT_V2_ONE_OVER_16M_REFUSED")
			}
			continue
		}
		if err != nil || receipt == nil {
			t.Fatal(err)
		}
		got, err := r.ReadPostPreparationPrivateV2(context.Background(), id, int64(n))
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatal("ASSERT_V2_PROFILE_EXACT_READ", err)
		}
	}
	if _, err := r.ReadPostPreparationPrivateV2(context.Background(), "bad", 16<<20+1); err == nil {
		t.Fatal("ASSERT_V2_READ_LIMIT")
	}
}
func TestPostPreparationPrivateV2ObjectMetadataAndAbsence(t *testing.T) {
	ctx := context.Background()
	roots, inputs, paths := retentionTopologyFixturePrivate(t)
	outputs := [2]*ContentRetentionRoot{roots[0], roots[1]}
	if err := CheckPostPreparationRetentionTopologyPrivateV2(ctx, outputs, inputs[:2]); err != nil {
		t.Fatal(err)
	}
	if err := CheckPostPreparationRetentionTopologyPrivateV2(ctx, outputs, inputs[:1]); err == nil {
		t.Fatal("ASSERT_V2_MIN_TWO_PROTECTED_ROOTS")
	}
	path, id, body := observedRootFixture(t)
	input, err := OpenObservedRoot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			t.Error(err)
		}
	})
	canonical, identity, err := input.CanonicalIdentityPrivateV2()
	if err != nil || canonical != path || identity.Inode == 0 {
		t.Fatal("ASSERT_V2_ROOT_CURRENT_CANONICAL_IDENTITY", err)
	}
	outID := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("absent output")))
	if err := CheckPostPreparationInputObjectAliasPrivateV2(ctx, outputs, outID, input, id); err != nil {
		t.Fatal("ASSERT_V2_SIMPLE_ABSENCE_ALLOWED_PREWRITE", err)
	}
	// Exact input-object hardlink alias is rejected by metadata, without a body read.
	if err := os.Link(filepath.Join(path, id), filepath.Join(paths[0], id)); err != nil {
		t.Fatal(err)
	}
	if err := CheckPostPreparationInputObjectAliasPrivateV2(ctx, outputs, id, input, id); err == nil {
		t.Fatal("ASSERT_V2_OUTPUT_INPUT_OBJECT_ALIAS_REFUSED")
	}
	if err := os.Link(filepath.Join(path, id), filepath.Join(paths[1], id)); err != nil {
		t.Fatal(err)
	}
	if err := CheckPostPreparationObjectsDistinctPrivateV2(ctx, outputs, id); err == nil {
		t.Fatal("ASSERT_V2_PAIR_PHYSICAL_ALIAS_REFUSED")
	}
	_ = body
	for _, err := range []error{errors.Join(os.ErrNotExist, context.Canceled), errors.Join(os.ErrNotExist, errors.New("close")), context.Canceled, errors.New("corrupt")} {
		if pureMissingPostPrivateV2(err) {
			t.Fatal("ASSERT_V2_JOINED_NONABSENCE_REFUSED", err)
		}
	}
	if !pureMissingPostPrivateV2(&os.PathError{Op: "open", Path: "owned", Err: os.ErrNotExist}) {
		t.Fatal("ASSERT_V2_WRAPPED_ABSENCE")
	}
}
