#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
fail=0
check() {
  assertion=$1 file=$2 literal=$3
  if ! grep -Fq "$literal" "$root/$file" 2>/dev/null; then
    printf 'FAIL %s: missing %s in %s\n' "$assertion" "$literal" "$file"
    fail=1
  else
    printf 'PASS %s\n' "$assertion"
  fi
}
reject() {
  assertion=$1 file=$2 literal=$3
  if grep -Fq "$literal" "$root/$file" 2>/dev/null; then
    printf 'FAIL %s: forbidden %s in %s\n' "$assertion" "$literal" "$file"
    fail=1
  else
    printf 'PASS %s\n' "$assertion"
  fi
}
check ASSERT_VALIDATING_COMPOSITE_BOUNDARY internal/programcadmission/admission.go 'func Admit(composite []byte)'
check ASSERT_OPAQUE_COMPOSITE_ADMISSION internal/programcadmission/admission.go 'type CompositeProjectionAdmission struct {'
check ASSERT_SEPARATE_ADMISSION_IDENTITY internal/programcadmission/admission.go 'lsp-trace.private.program-c-composite-leiden-admission.v1'
check ASSERT_CONSERVATIVE_COMPOSITE_AUTHORITY internal/programcadmission/admission.go 'SourceGraphComplete: SourceGraphComplete'
check ASSERT_OPAQUE_COMPUTATION_SEAM internal/programc/core.go 'func ComputeComposite(a programcadmission.CompositeProjectionAdmission'
reject ASSERT_NO_RAW_PROJECTION_CONSTRUCTOR internal/programc/core.go 'func NewCompositeProjectionAdmission('
reject ASSERT_NO_RAW_PROJECTION_COMPUTE internal/programc/core.go 'func ComputeProjection('
exit "$fail"
