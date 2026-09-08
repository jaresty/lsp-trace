#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
file=${1:-"$root/docs/DEFERRED-COMMUNITY-AND-BOUNDARY-PROGRAM.md"}
failed=0

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

assert_all ASSERT_PROGRAM_C_REMAINS_DEFERRED \
  '**Status: DEFERRED.**' \
  'This package does not authorize implementation of community detection'
assert_all ASSERT_DECISION_PACKAGE_SCOPE \
  'It authorizes only the bounded, offline investigation described below after the investigation-admission gate passes.' \
  'Even when true, this rule permits a new implementation decision; it does not itself authorize implementation.' \
  'The spike must not add public schemas, CLI/MCP commands, operation-registry entries, production packages, deployment configuration, or community implementation code.'
assert_gate ASSERT_INVESTIGATION_GATE_EXACT I 1 8 \
  'INVESTIGATION_ADMITTED = PASS(I-01..I-08)' \
  'No partial admission exists.'
assert_all ASSERT_BOUNDED_INVESTIGATION \
  '- **Questions:**' \
  '- **Inputs:**' \
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
printf 'DEFERRED COMMUNITY PROGRAM CHECK PASS\n'
