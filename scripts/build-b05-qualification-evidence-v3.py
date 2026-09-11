#!/usr/bin/env python3
"""Build B05 v3 evidence. Read-only by default; --retain is explicit publication."""
import argparse, hashlib, json, os, tempfile
from pathlib import Path

root = Path(__file__).resolve().parents[1]
legacy = root / 'qualification/retained/b05/qualification-matrix.v2.json'
current_dir = root / 'qualification/retained/b05/current'
initial_target = current_dir / 'qualification-evidence.v3.json'
selector = current_dir / 'release-selection.v1.json'
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

def atomic_replace(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(dir=path.parent, prefix=path.name + '.', suffix='.tmp')
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
    finally:
        Path(name).unlink(missing_ok=True)

def build(source=legacy, generation=1, supersedes=None):
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
        'generation': generation,
        'supersedes': supersedes,
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
    return evidence

def substance(value):
    body = dict(value)
    for key in ('evidence_id', 'generation', 'supersedes'):
        body.pop(key, None)
    return body

def next_evidence(source):
    if not selector.exists():
        return build(source), initial_target
    selected = json.loads(selector.read_text())
    previous_path = root / selected['selected_evidence']
    previous = json.loads(previous_path.read_text())
    candidate = build(source, previous['generation'], previous.get('supersedes'))
    if substance(candidate) == substance(previous):
        return previous, previous_path
    generation = previous['generation'] + 1
    target = current_dir / f'qualification-evidence.v3.generation-{generation}.json'
    return build(source, generation, previous['evidence_id']), target

def release_selection(evidence, target):
    return {
        'schema_version': 'lsp-trace.b05-release-selection.v1',
        'selected_evidence': target.relative_to(root).as_posix(),
        'selected_evidence_id': evidence['evidence_id'],
        'admission_ceiling': 'PROGRAM_B_NOT_ADMITTED'
    }

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--retain', action='store_true', help='publish an immutable generation and update its release selector')
    parser.add_argument('--output', type=Path, help='write an unretained candidate at this path')
    parser.add_argument('--legacy-input', type=Path, default=legacy, help='v2 candidate to convert')
    args = parser.parse_args()
    if args.retain and args.output:
        parser.error('--retain and --output are mutually exclusive')
    evidence, target = next_evidence(args.legacy_input)
    selection = release_selection(evidence, target)
    data = (json.dumps(evidence, indent=2) + '\n').encode()
    if args.retain:
        atomic_no_replace_or_equal(target, data)
        atomic_replace(selector, (json.dumps(selection, indent=2) + '\n').encode())
        print(f'PASS ASSERT_B05_V3_RETAINED generation={evidence["generation"]} evidence_id={evidence["evidence_id"]}')
    else:
        output = args.output or Path(tempfile.mkstemp(prefix='b05-qualification-evidence-v3.', suffix='.json')[1])
        output.write_bytes(data)
        print(f'PASS ASSERT_B05_V3_READ_ONLY candidate={output} evidence_id={evidence["evidence_id"]}')

if __name__ == '__main__':
    main()
