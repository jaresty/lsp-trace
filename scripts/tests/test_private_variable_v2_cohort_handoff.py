import copy
import importlib.util
import os
from pathlib import Path
import tempfile
import time
import unittest

spec = importlib.util.spec_from_file_location('cohort_handoff', Path(__file__).resolve().parents[1] / 'private-variable-v2-host-handoff.py')
h = importlib.util.module_from_spec(spec)
spec.loader.exec_module(h)


class CohortHandoffTest(unittest.TestCase):
    def fixture(self, base):
        base = Path(base)
        mailbox, control = base / 'mailbox', base / 'control'
        for p in (mailbox, control):
            p.mkdir(mode=0o700)
        limits = dict(MaxCorrections=1, MaxCalls=16, MaxRequestBytes=1 << 20,
                      MaxResponseBytes=64 << 10, MaxTotalBytes=8 << 20, MaxOutputBytes=4 << 20)
        scope = h.digest(b'owned source-only scope')
        cfg = dict(Enabled=True, Mode='PRIVATE_OPT_IN_SEMANTIC_COHORT_V1', Protocol=h.COHORT_PROTOCOL,
                   MaxItems=6, Directory=str(mailbox), Invocation='owned-cohort',
                   InterpreterRole='same-model/session-i', ReviewerRole='same-model/session-r', ScopeSHA256=scope,
                   TimeoutMS=5000, OverallTimeoutMS=20000, PollMS=2, Limits=limits, ApprovalSHA256='')
        cfg['ApprovalSHA256'] = h.digest(h.canonical(cfg))
        prep = dict(Protocol=h.COHORT_PROTOCOL, Invocation=cfg['Invocation'], OwnerNonce='owned-nonce',
                    ScopeSHA256=scope, PreparationSHA256=h.digest(b'owned-preparation'), PreparationByteLength=0,
                    Limits=limits, Deadline='2099-01-01T00:00:00Z', Preparation=None, CohortConfiguration=cfg)
        items = []
        for i in range(2):
            packet = h.decode(br'{"Numbers":[1e-9,-0,1e+21],"DirectedCALLS":["a-\u003eb","a-\u003eb"],"Missing":null,"Empty":[]}')
            body = ('x' * 100001 + '界e\u0301\r\nOWN_COHORT_TAIL_' + str(i))
            items.append(dict(InputSHA256=h.digest(b'input'), Target=f'target-{i}', Attempt=0,
                              PredecessorDigest='', Packet=packet, Bodies=[dict(Handle=f'h{i}', Body=body)],
                              Feedback='', PriorRevisions=None, Interpretation=None, InterpretationDigest=''))
        bindings = [dict(InputSHA256=i['InputSHA256'], Target=i['Target'], Attempt=i['Attempt'],
                         PredecessorDigest=i['PredecessorDigest'], RequestSHA256=h.digest(h.canonical(i))) for i in items]
        request = dict(Protocol=h.COHORT_SEMANTIC_PROTOCOL, Stage='INTERPRET_COHORT',
                       PreparationSHA256=prep['PreparationSHA256'], Items=items, ItemBindings=bindings,
                       PostLimits={}, LedgerQuoteChunkBytes=240, Navigation=None)
        binding = dict(Protocol=h.COHORT_PROTOCOL, Invocation=cfg['Invocation'], OwnerNonce=prep['OwnerNonce'],
                       Sequence=1, Stage=request['Stage'], RoleID=cfg['InterpreterRole'],
                       RequestSHA256=h.digest(h.canonical(request)), PreparationSHA256=prep['PreparationSHA256'],
                       Items=bindings, Navigation=None, ResponseAllowance=64 << 10, Deadline=prep['Deadline'])
        frame = dict(Binding=binding, Request=request)
        metadata = dict(Schema='private-census-inventory-cohort-host-export.v1', Profile='PRIVATE_CENSUS_AST_V2',
                        Protocol=h.COHORT_PROTOCOL, Authority=0, Accepted=False, Qualified=False,
                        FeatureIdentity='UNRESOLVED', SourceCompleteness='UNKNOWN', AbsenceInference='PROHIBITED',
                        ScopeSHA256=scope, PreparationSHA256=prep['PreparationSHA256'], CohortAutomation=cfg,
                        Roots=[str(base / 'source'), str(base / 'publication')])
        config = base / 'config.json'
        config.write_bytes(h.canonical(metadata))
        config.chmod(0o600)
        start = dict(Schema='private.onefile-actual-start.v2', ParentInvokedOperator=True, Invocation=cfg['Invocation'],
                     StartedUnix=int(time.time()) - 1, HardDeadlineUnix=int(time.time()) + 10,
                     ScopeSHA256=scope, FullPlanSHA256=h.digest(b'plan'), ActualRoleBindingsSHA256=h.digest(b'roles'))
        for directory, name, value in ((mailbox, 'preparation.json', prep), (mailbox, 'request-000001.json', frame),
                                       (control, 'start.json', start)):
            fd = h.directory(str(directory))
            try:
                h.publish(fd, name, h.canonical(value))
            finally:
                os.close(fd)
        return dict(base=base, mailbox=mailbox, control=control, cfg=cfg, prep=prep, frame=frame,
                    config=config, pin=h.digest(config.read_bytes()))

    def test_exact_cohort_export_and_response(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            fd = h.directory(str(f['mailbox']))
            try:
                receipt = h.export_request(fd, str(f['mailbox']), 1, str(f['base'] / 'export'),
                                          str(f['config']), f['pin'], str(f['control']))
            finally:
                os.close(fd)
            root = f['base'] / 'export'
            manifest = h.decode((root / 'manifest.json').read_bytes())
            raw = b''.join((root / p['Name']).read_bytes() for p in manifest['Parts'])
            self.assertEqual(raw, h.canonical(f['frame']), 'ASSERT_COHORT_EXPORT_EXACT')
            self.assertEqual(manifest['Schema'], 'private-v2-cohort-host-export.v1')
            self.assertEqual(receipt['Schema'], 'private-v2-cohort-host-export-receipt.v1')
            self.assertIn(b'1e-9,-0,1e+21', raw)
            self.assertIn(b'OWN_COHORT_TAIL_1', raw)
            self.assertIn(b'"Missing":null,"Empty":[]', raw)
            self.assertIn(br'"DirectedCALLS":["a-\u003eb","a-\u003eb"]', raw)
            for p in manifest['Parts']:
                (root / p['Name']).read_bytes().decode('utf-8', errors='strict')
            payload = f['base'] / 'payload.json'
            payload.write_bytes(b'RAW role bytes; no inference or normalization\n')
            payload.chmod(0o600)
            h.respond(str(f['mailbox']), 1, str(payload))
            response = h.decode((f['mailbox'] / 'response-000001.json').read_bytes())
            self.assertEqual(response['Binding'], f['frame']['Binding'])
            self.assertEqual(h.base64.b64decode(response['Payload']), payload.read_bytes())
            print('ASSERT_COHORT_EXPORT_EXACT: PASS tails UTF8 numeric-lexemes direction duplicates null/array')

    def test_cohort_wait_nested_binding_only(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            got = h.wait_request(str(f['mailbox']), f['cfg']['InterpreterRole'], str(f['control']), poll_ms=1)
            self.assertEqual(got, dict(Status='REQUEST', Binding=f['frame']['Binding']))
            self.assertNotIn('Request', got)
            self.assertNotIn('OWN_COHORT_TAIL', str(got))
            print('ASSERT_COHORT_WAIT_BINDING_ONLY: PASS')

    def test_closed_family_bindings(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            h.cohort_frame(f['frame'], f['prep'])
            for kind in ('reordered', 'stale', 'duplicate', 'foreign', 'legacy-protocol', 'wrong-role', 'fake-root', 'null'):
                with self.subTest(kind=kind):
                    frame = copy.deepcopy(f['frame'])
                    if kind == 'reordered':
                        frame['Request']['Items'].reverse()
                    elif kind == 'stale':
                        frame['Request']['ItemBindings'][0]['PredecessorDigest'] = 'stale'
                    elif kind == 'duplicate':
                        frame['Request']['Items'][1] = frame['Request']['Items'][0]
                    elif kind == 'foreign':
                        frame['Binding']['Items'][0]['Target'] = 'foreign'
                    elif kind == 'legacy-protocol':
                        frame['Request']['Protocol'] = 'private-provisional-inventory.v2'
                    elif kind == 'wrong-role':
                        frame['Binding']['RoleID'] = f['cfg']['ReviewerRole']
                    elif kind == 'fake-root':
                        frame['Binding']['Target'] = 'aggregate-fake-root'
                    else:
                        frame['Request']['Items'] = None
                    with self.assertRaises((ValueError, TypeError)):
                        h.cohort_frame(frame, f['prep'])
            print('ASSERT_COHORT_CLOSED_BINDINGS: PASS')

    def test_approval_shape(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            for field in ('Mode', 'Protocol', 'MaxItems', 'ScopeSHA256', 'ReviewerRole', 'Limits'):
                cfg = copy.deepcopy(f['cfg'])
                if field == 'MaxItems':
                    cfg[field] += 1
                elif field == 'Limits':
                    cfg[field]['MaxCalls'] += 1
                else:
                    cfg[field] += '-changed'
                with self.assertRaises(ValueError):
                    h.cohort_config(cfg)
            print('ASSERT_COHORT_APPROVAL_EXACT: PASS')

    def test_export_binds_entire_cohort_configuration(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            metadata = h.decode(f['config'].read_bytes())
            cfg = metadata['CohortAutomation']
            cfg['MaxItems'] += 1
            cfg['ApprovalSHA256'] = ''
            cfg['ApprovalSHA256'] = h.digest(h.canonical(cfg))
            f['config'].write_bytes(h.canonical(metadata))
            fd = h.directory(str(f['mailbox']))
            try:
                with self.assertRaisesRegex(ValueError, 'export cohort configuration mismatch'):
                    h.export_request(fd, str(f['mailbox']), 1, str(f['base'] / 'export'),
                                     str(f['config']), h.digest(f['config'].read_bytes()), str(f['control']))
            finally:
                os.close(fd)
            self.assertFalse((f['base'] / 'export').exists())
            print('ASSERT_COHORT_PINNED_FULL_CONFIGURATION: PASS')

    def test_cohort_navigation_binding(self):
        with tempfile.TemporaryDirectory(prefix='.owned-cohort-', dir=Path.home()) as base:
            f = self.fixture(base)
            frame = copy.deepcopy(f['frame'])
            frame['Request']['Stage'] = frame['Binding']['Stage'] = 'NAVIGATE'
            frame['Request']['Items'] = frame['Request']['ItemBindings'] = frame['Binding']['Items'] = None
            nav = dict(Protocol=h.COHORT_SEMANTIC_PROTOCOL, Stage='NAVIGATE', PreparationSHA256=f['prep']['PreparationSHA256'],
                       InputSHA256='', Target='', Attempt=0, PredecessorDigest='', RevisionDigest='', Candidates=[])
            frame['Request']['Navigation'] = nav
            frame['Binding']['Navigation'] = dict(Attempt=0, PredecessorDigest='', RevisionDigest='', CandidatesSHA256=h.digest(b'[]'))
            h.cohort_frame(frame, f['prep'])
            frame['Binding']['Navigation']['CandidatesSHA256'] = h.digest(b'null')
            with self.assertRaises(ValueError):
                h.cohort_frame(frame, f['prep'])
            print('ASSERT_COHORT_NAVIGATION_BINDING: PASS')


if __name__ == '__main__':
    unittest.main()
