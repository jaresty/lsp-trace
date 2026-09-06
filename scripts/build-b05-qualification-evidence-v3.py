#!/usr/bin/env python3
"""Build B05 v3 evidence. Read-only by default; --retain is explicit publication."""
import argparse, hashlib, json, os, tempfile
from pathlib import Path

root = Path(__file__).resolve().parents[1]
legacy = root / 'qualification/retained/b05/qualification-matrix.v2.json'
target = root / 'qualification/retained/b05/current/qualification-evidence.v3.json'
selector = root / 'qualification/retained/b05/current/release-selection.v1.json'
historical = 'git-blob:0bd7501a1fa9307002fd09c10e4bb3843141df0e'

def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()

def logical_id(value):
    body = dict(value)
    body.pop('evidence_id', None)
    return 'sha256:' + hashlib.sha256(canonical(body)).hexdigest()

def atomic_no_replace_or_equal(path, data):
    if path.exists():
        if path.read_bytes() != data:
            raise SystemExit(f'refusing to overwrite retained generation with different bytes: {path}')
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(dir=path.parent, prefix=path.name + '.', suffix='.tmp')
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.link(name, path)
    finally:
        Path(name).unlink(missing_ok=True)

def build(source=legacy):
    old = json.loads(source.read_text())
    boundary = {
        'transport': 'real-lsp-trace-mcp-stdio',
        'provider_identity': 'ember-glint@1',
        'provider_path_kind': 'independently-packed-installed',
        'workspace_custody': 'PROVIDER_PROVED',
        'provider_package_sha256': old['provider_package_sha256'],
        'provider_executable_sha256': old['provider_executable_sha256'],
        'evidence_kind': 'NATIVE_PROVIDER'
    }
    evidence = {
        'schema_version': 'lsp-trace.b05-qualification-evidence.v3',
        'evidence_id': '',
        'family': 'b05-production-provider-qualification',
        'generation': 1,
        'supersedes': None,
        'derived_from': [{
            'family': 'lsp-trace.b05-qualification-matrix',
            'schema_version': 'v2',
            'identity': historical,
            'relationship': 'ADDITIVE_SUCCESSOR'
        }],
        'production_boundary': boundary,
        'admission': {
            'rule': old['admission_rule'],
            'ceiling': 'PROGRAM_B_NOT_ADMITTED',
            'PROGRAM_B_ADMITTED': False,
            'blocked_relations': old['capabilities']['Blocked']
        },
        'seeds': old['seeds'],
        'attempts': [dict(attempt, EvidenceKind='NATIVE_PROVIDER') for attempt in old['attempts']],
        'stages': old['stages'],
        'capabilities': old['capabilities'],
        'release_check': old['release_check']
    }
    evidence['evidence_id'] = logical_id(evidence)
    selection = {
        'schema_version': 'lsp-trace.b05-release-selection.v1',
        'selected_evidence': 'qualification/retained/b05/current/qualification-evidence.v3.json',
        'selected_evidence_id': evidence['evidence_id'],
        'admission_ceiling': 'PROGRAM_B_NOT_ADMITTED'
    }
    return evidence, selection

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--retain', action='store_true', help='publish generation 1 and its release selector')
    parser.add_argument('--output', type=Path, help='write an unretained candidate at this path')
    parser.add_argument('--legacy-input', type=Path, default=legacy, help='v2 candidate to convert')
    args = parser.parse_args()
    if args.retain and args.output:
        parser.error('--retain and --output are mutually exclusive')
    evidence, selection = build(args.legacy_input)
    data = (json.dumps(evidence, indent=2) + '\n').encode()
    if args.retain:
        atomic_no_replace_or_equal(target, data)
        atomic_no_replace_or_equal(selector, (json.dumps(selection, indent=2) + '\n').encode())
        print(f'PASS ASSERT_B05_V3_RETAINED evidence_id={evidence["evidence_id"]}')
    else:
        output = args.output or Path(tempfile.mkstemp(prefix='b05-qualification-evidence-v3.', suffix='.json')[1])
        output.write_bytes(data)
        print(f'PASS ASSERT_B05_V3_READ_ONLY candidate={output} evidence_id={evidence["evidence_id"]}')

if __name__ == '__main__':
    main()
