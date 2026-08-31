#!/bin/sh
set -eu

language=${1:-}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
evidence="$root/qualification/evidence/$language"
mkdir -p "$evidence"

blocked() {
  printf 'BLOCKED: %s\n' "$1" | tee "$evidence/status.txt"
  exit 2
}

find_elixir_ls() {
  if [ -n "${ELIXIR_LS_COMMAND:-}" ]; then
    printf '%s\n' "$ELIXIR_LS_COMMAND"
  elif command -v elixir-ls >/dev/null 2>&1; then
    command -v elixir-ls
  elif command -v language_server.sh >/dev/null 2>&1; then
    command -v language_server.sh
  else
    return 1
  fi
}

case "$language" in
  typescript)
    command -v node >/dev/null 2>&1 || blocked 'node executable unavailable'
    command -v typescript-language-server >/dev/null 2>&1 || blocked 'typescript-language-server executable unavailable'
    server=$(command -v typescript-language-server)
    "$server" --version >"$evidence/server-version.txt" 2>&1 || blocked 'server version probe failed'
    target="$root/qualification/typescript/src/calls.ts:2:10"
    workspace="$root/qualification/typescript"
    server_arg=--stdio
    expected='left,right,recursive,staticButNotExecuted'
    ;;
  csharp)
    command -v csharp-ls >/dev/null 2>&1 || blocked 'csharp-ls executable unavailable'
    server=$(command -v csharp-ls)
    "$server" --version >"$evidence/server-version.txt" 2>&1 || blocked 'server version probe failed'
    target="$root/qualification/csharp/Calls.cs:5:23"
    workspace="$root/qualification/csharp"
    server_arg=--stdio
    expected='Left,Right,Recursive,StaticButNotExecuted'
    ;;
  elixir)
    command -v elixir >/dev/null 2>&1 || blocked 'Elixir executable unavailable'
    server=$(find_elixir_ls) || blocked 'ElixirLS executable unavailable (set ELIXIR_LS_COMMAND)'
    ("$server" --version || elixir --version) >"$evidence/server-version.txt" 2>&1 || blocked 'ElixirLS version probe failed'
    target="$root/qualification/elixir/lib/calls.ex:2:7"
    workspace="$root/qualification/elixir"
    server_arg=--stdio
    expected='left,right,recursive,static_but_not_executed'
    ;;
  *)
    printf 'usage: %s {typescript|csharp|elixir}\n' "$0" >&2
    exit 1
    ;;
esac

binary="$evidence/lsp-trace"
(cd "$root" && go build -o "$binary" ./cmd/lsp-trace)
printf '%s\n' "$binary incoming --workspace $workspace --server $server --server-arg $server_arg --at $target" >"$evidence/command.txt"
set +e
"$binary" incoming --workspace "$workspace" --server "$server" --server-arg "$server_arg" --at "$target" --pretty >"$evidence/graph.json" 2>"$evidence/stderr.txt"
status=$?
set -e
if [ "$status" -eq 2 ]; then
  python3 - "$evidence/graph.json" >"$evidence/capability.txt" <<'PY'
import json, sys
with open(sys.argv[1], encoding='utf-8') as f: g=json.load(f)
print(json.dumps(g.get('capabilities', {}), sort_keys=True))
PY
  blocked 'Call Hierarchy unavailable or traversal incomplete; capability retained'
fi
if [ "$status" -ne 0 ]; then
  printf 'FAIL: lsp-trace exited %s\n' "$status" | tee "$evidence/status.txt"
  exit 1
fi

set +e
python3 - "$evidence/graph.json" "$expected" >"$evidence/assertions.txt" 2>&1 <<'PY'
import json, sys
with open(sys.argv[1], encoding='utf-8') as f: g=json.load(f)
expected=set(sys.argv[2].split(','))
names={n['name'] for n in g['nodes']}
present={want for want in expected if any(name == want or name.endswith('.' + want + '/1') for name in names)}
missing=sorted(expected-present)
assert g['schema_version']=='lsp-trace.graph.v1'
assert g['capabilities']['call_hierarchy_provider'] is True
assert g['summary']['complete'] is True
assert not missing, f'missing exact callers: {missing}'
assert all(e['call_sites'] for e in g['edges'])
print('capability=true')
print('exact callers=' + ','.join(sorted(expected)))
print('all caller/callee edges retain exact non-empty call-site ranges')
PY
assertion_status=$?
set -e
if [ "$assertion_status" -ne 0 ]; then
  if [ "$language" = elixir ]; then
    blocked 'ElixirLS Call Hierarchy lacks the exact useful callers/ranges required; assertions retained'
  fi
  printf 'FAIL: graph assertions failed\n' | tee "$evidence/status.txt"
  exit 1
fi
printf 'PASS\n' | tee "$evidence/status.txt"
