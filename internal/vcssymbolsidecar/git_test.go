package vcssymbolsidecar

import (
	"reflect"
	"testing"
)

func TestParseUnifiedZeroDiffAccountsOldAndNewLines(t *testing.T) {
	raw := []byte("diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -2,2 +2,3 @@\n-old\n-old2\n+new\n+new2\n+new3\n")
	got, err := parseUnifiedZeroDiff(raw, map[string]struct{}{"a.go": {}})
	if err != nil {
		t.Fatal(err)
	}
	want := []ChangedLine{{Side: "OLD", Path: "a.go", Line: 1}, {Side: "OLD", Path: "a.go", Line: 2}, {Side: "NEW", Path: "a.go", Line: 1}, {Side: "NEW", Path: "a.go", Line: 2}, {Side: "NEW", Path: "a.go", Line: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_EXACT_HUNK_ACCOUNTING: got=%+v want=%+v", got, want)
	}
}

func TestParseUnifiedZeroDiffAttributesDeletedFileOldLines(t *testing.T) {
	raw := []byte("diff --git a/a.go b/a.go\n--- a/a.go\n+++ /dev/null\n@@ -2,2 +0,0 @@\n-old\n-old2\n")
	got, err := parseUnifiedZeroDiff(raw, map[string]struct{}{"a.go": {}})
	if err != nil {
		t.Fatalf("ASSERT_SYMBOL_CHURN_DELETED_FILE_OLD_LINES: %v", err)
	}
	want := []ChangedLine{{Side: "OLD", Path: "a.go", Line: 1}, {Side: "OLD", Path: "a.go", Line: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_SYMBOL_CHURN_DELETED_FILE_OLD_LINES: got=%+v want=%+v", got, want)
	}
}

func TestParseUnifiedZeroDiffRejectsUnrequestedPath(t *testing.T) {
	raw := []byte("diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -0,0 +1 @@\n+x\n")
	if _, err := parseUnifiedZeroDiff(raw, map[string]struct{}{"a.go": {}}); err == nil {
		t.Fatal("ASSERT_SYMBOL_CHURN_REJECTS_UNREQUESTED_DIFF_PATH")
	}
}
