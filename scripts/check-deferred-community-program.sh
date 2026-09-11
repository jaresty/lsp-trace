#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
file="$root/docs/DEFERRED-COMMUNITY-AND-BOUNDARY-PROGRAM.md"
index="$root/qualification/program-c/gate-i-receipts.tsv"
mode=shape
failed=0

if [ "${1:-}" = "--admission" ]; then
  mode=admission
  shift
fi
if [ "$#" -gt 0 ]; then
  file=$1
  shift
fi
if [ "$#" -gt 0 ]; then
  index=$1
  shift
fi
if [ "$#" -ne 0 ]; then
  printf 'usage: %s [--admission] [decision-package [receipt-index]]\n' "$0" >&2
  exit 64
fi

assert_all() {
  id=$1
  shift
  for text in "$@"; do
    if ! grep -F -- "$text" "$file" >/dev/null; then
      printf '%s result=FAIL missing=%s\n' "$id" "$text"
      failed=1
      return
    fi
  done
  printf '%s result=PASS\n' "$id"
}

assert_gate() {
  id=$1
  prefix=$2
  first=$3
  last=$4
  shift 4
  got=$(grep -E "^\\| ${prefix}-[0-9][0-9] \\|" "$file" | cut -d'|' -f2 | tr -d ' ' | paste -sd, - || true)
  expected=''
  n=$first
  while [ "$n" -le "$last" ]; do
    item=$(printf '%s-%02d' "$prefix" "$n")
    if [ -z "$expected" ]; then expected=$item; else expected="$expected,$item"; fi
    n=$((n + 1))
  done
  if [ "$got" != "$expected" ]; then
    printf '%s result=FAIL got=%s want=%s\n' "$id" "$got" "$expected"
    failed=1
    return
  fi
  for text in "$@"; do
    if ! grep -F -- "$text" "$file" >/dev/null; then
      printf '%s result=FAIL missing=%s\n' "$id" "$text"
      failed=1
      return
    fi
  done
  printf '%s result=PASS ids=%s\n' "$id" "$got"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

assert_receipt_index() {
  if [ ! -f "$index" ] || [ -L "$index" ]; then
    printf 'ASSERT_GATE_I_RECEIPT_INDEX result=FAIL reason=missing-or-nonregular-index\n'
    failed=1
    return
  fi

  got=$(awk -F '\t' '!/^#/ && NF {print $1}' "$index" | paste -sd, -)
  expected='I-01,I-02,I-03,I-04,I-05,I-06,I-07,I-08'
  if [ "$got" != "$expected" ]; then
    printf 'ASSERT_GATE_I_RECEIPT_INDEX result=FAIL got=%s want=%s\n' "$got" "$expected"
    failed=1
    return
  fi

  if ! awk -F '\t' '
    /^#/ || NF == 0 { next }
    NF != 4 { exit 1 }
    $2 == "MISSING" { if ($3 != "-" || $4 != "-") exit 1; next }
    $2 == "PASS" { if ($3 == "-" || $4 !~ /^sha256:[0-9a-f]{64}$/) exit 1; next }
    { exit 1 }
  ' "$index"; then
    printf 'ASSERT_GATE_I_RECEIPT_INDEX result=FAIL reason=invalid-row-shape\n'
    failed=1
    return
  fi

  printf 'ASSERT_GATE_I_RECEIPT_INDEX result=PASS ids=%s\n' "$got"
}

assert_receipts() {
  tab=$(printf '\t')
  while IFS="$tab" read -r gate state receipt digest extra; do
    case "$gate" in ''|'#'*) continue ;; esac
    assertion=$(printf 'ASSERT_GATE_I_RECEIPT_%s' "$(printf '%s' "$gate" | tr '-' '_')")

    if [ "$state" != "PASS" ]; then
      if [ "$mode" = admission ]; then
        printf '%s result=FAIL state=%s\n' "$assertion" "$state"
        failed=1
      else
        printf '%s result=DEFERRED state=%s\n' "$assertion" "$state"
      fi
      continue
    fi

    case "$receipt" in
      /*|../*|*/../*|*/..)
        printf '%s result=FAIL reason=receipt-path-outside-repository\n' "$assertion"
        failed=1
        continue
        ;;
    esac
    receipt_file="$root/$receipt"
    if [ ! -f "$receipt_file" ] || [ -L "$receipt_file" ]; then
      printf '%s result=FAIL reason=receipt-not-addressable\n' "$assertion"
      failed=1
      continue
    fi
    actual="sha256:$(sha256_file "$receipt_file")"
    if [ "$actual" != "$digest" ]; then
      printf '%s result=FAIL reason=digest-mismatch\n' "$assertion"
      failed=1
      continue
    fi
    if ! grep -Fx -- "gate_id: $gate" "$receipt_file" >/dev/null ||
       ! grep -Fx -- 'result: PASS' "$receipt_file" >/dev/null ||
       ! grep -Fx -- 'current: true' "$receipt_file" >/dev/null; then
      printf '%s result=FAIL reason=receipt-semantics\n' "$assertion"
      failed=1
      continue
    fi
    if ! grep -Eq '^repository_revision: [^[:space:]].*$' "$receipt_file" ||
       ! grep -Eq '^command: [^[:space:]].*$' "$receipt_file" ||
       ! grep -Eq '^policy_matrix: [^[:space:]].*$' "$receipt_file" ||
       ! grep -Eq '^tool: [^[:space:]].*$' "$receipt_file" ||
       ! grep -Eq '^run_id: [^[:space:]].*$' "$receipt_file" ||
       ! grep -Eq '^input_sha256: sha256:[0-9a-f]{64}$' "$receipt_file" ||
       ! grep -Eq '^result_sha256: sha256:[0-9a-f]{64}$' "$receipt_file" ||
       ! grep -Eq '^outcomes: PASS=[0-9]+ FAIL=0 BLOCKED=0$' "$receipt_file"; then
      printf '%s result=FAIL reason=incomplete-local-reproducibility-fields\n' "$assertion"
      failed=1
      continue
    fi
    printf '%s result=PASS receipt=%s\n' "$assertion" "$receipt"
  done < "$index"
}

assert_all ASSERT_PROGRAM_C_REMAINS_DEFERRED \
  '**Status: DEFERRED.**' \
  'This package does not authorize implementation of community detection'
assert_all ASSERT_DECISION_PACKAGE_SCOPE \
  'It authorizes only the bounded, offline investigation described below after the investigation-admission gate passes.' \
  'Even when true, this rule permits a new implementation decision; it does not itself authorize implementation.' \
  'The spike must not add public schemas, CLI/MCP commands, operation-registry entries, production packages, deployment configuration, or community implementation code.'
assert_gate ASSERT_INVESTIGATION_GATE_EXACT I 1 8 \
  'INVESTIGATION_ADMITTED = PASS(I-01..I-08)' \
  'No partial admission exists.' \
  'Each passing receipt is a reproducibility record, not a cryptographic attestation.' \
  'Gate I does not require an authority, key, signature, opaque Program A token, or cross-user trust.'
assert_receipt_index
if [ "$failed" -eq 0 ]; then
  assert_receipts
fi
assert_all ASSERT_BOUNDED_INVESTIGATION \
  '- **Questions:**' \
  '- **Inputs:**' \
  '- **Candidate algorithms and implementations:**' \
  "the proposed primary is Gonum's native-Go Leiden implementation only after it appears in an exact tagged Gonum release" \
  'Gonum Louvain from that same release is the comparator.' \
  'Infomap is optional external-reference material' \
  '`vtraag/leidenalg` is rejected' \
  '- **Fixture cap:**' \
  '- **Execution cap:**' \
  '- **Resource cap:**' \
  '- **Outputs:**' \
  '- **Stopping conditions:**' \
  'D01 execution or deployment, network access, installation, product-repository work, and a full test suite are outside this package.'
assert_gate ASSERT_IMPLEMENTATION_GATE_EXACT A 1 10 \
  'IMPLEMENTATION_DECISION_ALLOWED = INVESTIGATION_ADMITTED && PASS(A-01..A-10)' \
  'it does not itself authorize implementation.'
assert_all ASSERT_BOUNDARY_NEUTRALITY \
  'Crossings are structural observations under an identified projection' \
  'never product, ownership, service, organizational, or business-boundary evidence.'
assert_all ASSERT_EXCLUDED_EXECUTION \
  'D01 execution or deployment, network access, installation, product-repository work, and a full test suite are outside this package.' \
  'The spike must not add public schemas, CLI/MCP commands, operation-registry entries, production packages, deployment configuration, or community implementation code.'

if [ "$failed" -ne 0 ]; then
  exit 1
fi
if [ "$mode" = admission ]; then
  printf 'PROGRAM C INVESTIGATION ADMISSION PASS\n'
else
  printf 'DEFERRED COMMUNITY PROGRAM CHECK PASS admission=NOT_EVALUATED\n'
fi
