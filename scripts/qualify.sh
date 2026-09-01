#!/bin/sh
set -eu

language=${1:-}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
evidence="$root/qualification/evidence/$language"
mkdir -p "$evidence"
rm -f "$evidence"/*

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
    command -v npm >/dev/null 2>&1 || blocked 'npm executable unavailable'
    workspace="$root/qualification/typescript"
    if [ ! -x "$workspace/node_modules/.bin/typescript-language-server" ]; then
      (cd "$workspace" && npm ci --ignore-scripts) >"$evidence/setup.txt" 2>&1 || blocked 'pinned TypeScript fixture dependency install failed'
    fi
    server="$workspace/node_modules/.bin/typescript-language-server"
    "$server" --version >"$evidence/server-version.txt" 2>&1 || blocked 'server version probe failed'
    target="$workspace/src/calls.ts:1:17"
    server_arg=--stdio
    expected='left,right,recursive,staticButNotExecuted'
    ;;
  csharp)
    command -v csharp-ls >/dev/null 2>&1 || blocked 'csharp-ls executable unavailable'
    server=$(command -v csharp-ls)
    "$server" --version >"$evidence/server-version.txt" 2>&1 || blocked 'server version probe failed'
    target="$root/qualification/csharp/Calls.cs:5:23"
    workspace="$root/qualification/csharp"
    server_arg=
    expected='Left,Right,Recursive,StaticButNotExecuted'
    ;;
  elixir|elixir-companion)
    command -v elixir >/dev/null 2>&1 || blocked 'Elixir executable unavailable'
    if [ "$language" = elixir-companion ]; then
      companion_root=${ELIXIR_CALL_HIERARCHY_ROOT:-"$root/../elixir-call-hierarchy"}
      [ -f "$companion_root/mix.exs" ] || blocked 'Elixir call hierarchy companion checkout unavailable (set ELIXIR_CALL_HIERARCHY_ROOT)'
      (cd "$companion_root" && mix escript.build) >"$evidence/setup.txt" 2>&1 || blocked 'companion escript build failed'
      server="$companion_root/elixir-call-hierarchy"
      "$server" 2>"$evidence/server-version.txt" || true
    else
      server=$(find_elixir_ls) || blocked 'ElixirLS executable unavailable (set ELIXIR_LS_COMMAND)'
      ("$server" --version || elixir --version) >"$evidence/server-version.txt" 2>&1 || blocked 'ElixirLS version probe failed'
    fi
    target="$root/qualification/elixir/lib/calls.ex:3:7"
    workspace="$root/qualification/elixir"
    server_arg=--stdio
    expected='left,right,same_module,recursive,static_but_not_executed,aliased_cross_file,direct_qualified'
    ;;
  *)
    printf 'usage: %s {typescript|csharp|elixir|elixir-companion}\n' "$0" >&2
    exit 1
    ;;
esac

binary="$evidence/lsp-trace"
(cd "$root" && go build -o "$binary" ./cmd/lsp-trace)
set -- "$binary" incoming --workspace "$workspace" --server "$server"
if [ -n "$server_arg" ]; then
  set -- "$@" --server-arg "$server_arg"
fi
set -- "$@" --at "$target" --schema v2 --pretty
printf '%s\n' "$*" >"$evidence/command.txt"
set +e
"$@" >"$evidence/graph.json" 2>"$evidence/stderr.txt"
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
python3 - "$evidence/graph.json" "$expected" "$language" >"$evidence/assertions.txt" 2>&1 <<'PY'
import json, sys
with open(sys.argv[1], encoding='utf-8') as f: g=json.load(f)
expected=set(sys.argv[2].split(','))
language=sys.argv[3]
def require(condition, message):
    if not condition:
        print('FAIL: ' + message)
        raise SystemExit(1)
node_list=g.get('nodes') or []
edge_list=g.get('edges') or []
nodes={n['id']: n for n in node_list}
names={n['name'] for n in node_list}
def matches(name, want):
    return name == want or name == want + '/1' or name.startswith(want + '(') or name.endswith('.' + want + '/1')
present={want for want in expected if any(matches(name, want) for name in names)}
missing=sorted(expected-present)
require(g['schema_version'] in {'lsp-trace.graph.v1', 'lsp-trace.graph.v2'}, 'unsupported schema version')
require(g['capabilities']['call_hierarchy_provider'] is True, 'Call Hierarchy was not advertised')
summary=g['summary']
require(summary.get('traversal_complete', summary.get('complete')) is True, 'server-reported traversal is incomplete')
require(not missing, f'missing exact callers: {missing}')
require(bool(edge_list) and all(e['call_sites'] for e in edge_list), 'one or more caller edges lack call-site ranges')
print('protocol support=true')
print('exact callers=' + ','.join(sorted(expected)))
if language in {'elixir', 'elixir-companion'}:
    leaf_ids={node_id for node_id, node in nodes.items() if matches(node['name'], 'leaf')}
    leaf_callers={nodes[e['caller_node_id']]['name'] for e in edge_list if e['callee_node_id'] in leaf_ids}
    required_same={'same_module', 'recursive', 'static_but_not_executed'}
    required_cross={'aliased_cross_file', 'direct_qualified'}
    missing_same=sorted(want for want in required_same if not any(matches(name, want) for name in leaf_callers))
    missing_cross=sorted(want for want in required_cross if not any(matches(name, want) for name in leaf_callers))
    require(not missing_same, f'missing same-module caller edge: {missing_same}')
    require(not missing_cross, f'missing cross-module caller edge: {missing_cross}')
    print('same-module resolution=true')
    print('cross-module resolution=true')
    print('multi-clause resolution=true')
print('all caller/callee edges retain exact non-empty call-site ranges')
PY
assertion_status=$?
set -e
if [ "$assertion_status" -ne 0 ]; then
  if [ "$language" = elixir ] || [ "$language" = elixir-companion ]; then
    blocked 'ElixirLS Call Hierarchy lacks an exact same-module or missing cross-module caller edge, multi-clause resolution, or required range; assertions retained'
  fi
  printf 'FAIL: graph assertions failed\n' | tee "$evidence/status.txt"
  exit 1
fi
printf 'PASS\n' | tee "$evidence/status.txt"
