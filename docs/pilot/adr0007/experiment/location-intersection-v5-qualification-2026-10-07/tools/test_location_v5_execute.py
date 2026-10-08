import json, pathlib, subprocess, sys

ROOT = pathlib.Path('docs/pilot/adr0007/experiment/location-intersection-v5-qualification-2026-10-07')
SCRIPT = ROOT/'tools/location_v5_execute.py'

def test_assert_freeze():
    out = subprocess.check_output([sys.executable, str(SCRIPT), 'assert-freeze'], text=True)
    assert 'FROZEN230_UNCHANGED sha256:1195a420cc2ae215ff1627dbf23b606caaa243aa9fc0ae234242acceddafb48d' in out

def test_assignments_predeclare_26_each():
    data = json.loads((ROOT/'ASSIGNMENTS.json').read_text())
    assert len(data['cases']) == 26
    assert all(c['producer']['attemptID'] == 'attempt-01' for c in data['cases'])
    assert all(c['reviewer']['attemptID'] == 'attempt-01' for c in data['cases'])
    assert all('oracle-candidate' in c['producer']['forbidden'] for c in data['cases'])

def test_authority_ceiling_zero():
    auth = json.loads((ROOT/'AUTHORIZATION.json').read_text())
    assert auth['authorityCeiling'] == 0
    assert auth['headAuthorityCeiling'] == 0
    assert auth['accepted'] is False
    assert auth['completeness'] == 'UNKNOWN'
    assert auth['featureIdentity'] == 'UNRESOLVED'
