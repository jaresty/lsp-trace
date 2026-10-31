"""MED regression: semantic family is independent of a valid outer digest.

The fixture deliberately creates a valid cohort NAVIGATE request, then places
those bytes under a legitimate legacy outer binding with a recomputed digest.
No source owner, provider, inference, or application execution is involved.
"""
import copy
import os
from pathlib import Path
import tempfile
import unittest

import test_private_variable_v2_handoff as legacy_tests
import test_private_variable_v2_cohort_handoff as cohort_tests

h = legacy_tests.h
LEGACY_SEMANTIC = 'private-provisional-inventory.v2'


class FamilyClosureTest(unittest.TestCase):
    def put(self, path, value):
        path.write_bytes(h.canonical(value))
        path.chmod(0o600)

    def fixture(self, base):
        f = legacy_tests.HandoffTest().export_fixture(base)
        frame = h.decode(f['raw'])
        # This valid-family fixture explicitly names the real Go semantic family.
        # Opaque request data remains allowed; no full-source validator is added.
        frame['Request']['Protocol'] = LEGACY_SEMANTIC
        frame['Binding']['RequestSHA256'] = h.digest(h.canonical(frame['Request']))
        self.put(f['mailbox'] / 'request-000001.json', frame)
        f['valid'] = copy.deepcopy(frame)
        prep = h.decode((f['mailbox'] / 'preparation.json').read_bytes())
        start = dict(Schema='private.onefile-actual-start.v2', ParentInvokedOperator=True,
                     Invocation=prep['Invocation'], StartedUnix=1, HardDeadlineUnix=4070908800,
                     ScopeSHA256=h.digest(b'scope'), FullPlanSHA256=h.digest(b'plan'),
                     ActualRoleBindingsSHA256=h.digest(b'roles'))
        self.put(f['control'] / 'start.json', start)
        return f

    def mixed_navigation(self, f):
        base = f['base'] / 'cohort-fixture'
        base.mkdir(mode=0o700)
        cf = cohort_tests.CohortHandoffTest().fixture(base)
        frame = copy.deepcopy(cf['frame'])
        sha = f['valid']['Binding']['PreparationSHA256']
        nav = dict(Protocol=h.COHORT_SEMANTIC_PROTOCOL, Stage='NAVIGATE', PreparationSHA256=sha,
                   InputSHA256='', Target='', PredecessorDigest='', Attempt=0, Bodies=None, Packet=None,
                   Revision=None, RevisionDigest='', Candidates=[], Navigation=None, Feedback='',
                   PostLimits={}, LedgerQuoteChunkBytes=240, PriorRevisions=None, PriorNavigationRevisions=None)
        frame['Request'] = dict(Protocol=h.COHORT_SEMANTIC_PROTOCOL, Stage='NAVIGATE',
                                PreparationSHA256=sha, Items=None, ItemBindings=None, PostLimits={},
                                LedgerQuoteChunkBytes=240, Navigation=nav)
        frame['Binding'].update(Stage='NAVIGATE', PreparationSHA256=sha, Items=None,
                                Navigation=dict(Attempt=0, PredecessorDigest='', RevisionDigest='',
                                                CandidatesSHA256=h.digest(b'[]')),
                                RequestSHA256=h.digest(h.canonical(frame['Request'])))
        prep = copy.deepcopy(cf['prep'])
        prep['PreparationSHA256'] = sha
        # Establish that the inner request is valid for its own cohort family,
        # before making the single invalidating change to the outer family.
        h.cohort_frame(frame, prep)
        mixed = copy.deepcopy(f['valid'])
        mixed['Request'] = frame['Request']
        mixed['Binding'].update(Stage='NAVIGATE', InputSHA256='', Target='',
                                RequestSHA256=h.digest(h.canonical(mixed['Request'])))
        self.put(f['mailbox'] / 'request-000001.json', mixed)
        return mixed

    def refused(self, action, f):
        fd = h.directory(str(f['mailbox']))
        try:
            if action == 'request':
                h.request(fd, 1)
            elif action == 'export':
                h.export_request(fd, str(f['mailbox']), 1, str(f['dest']),
                                 str(f['config']), f['pin'], str(f['control']))
            elif action == 'respond':
                payload = f['base'] / 'payload.json'
                payload.write_bytes(b'[]')
                payload.chmod(0o600)
                h.respond(str(f['mailbox']), 1, str(payload))
            elif action in ('wait-pending', 'wait-completed'):
                h.wait_request(str(f['mailbox']), 'owned/interpreter', str(f['control']), poll_ms=1)
            else:
                result = h.subprocess.run([h.sys.executable, h.__file__, 'read', '--mailbox',
                                           str(f['mailbox']), '--sequence', '1'], capture_output=True)
                return result.returncode != 0 and b'"Request":' not in result.stdout
        except ValueError:
            return True
        finally:
            os.close(fd)
        return False

    def mixed_guard(self, action):
        with tempfile.TemporaryDirectory(prefix='.owned-family-', dir=Path.home()) as base:
            f = self.fixture(base)
            mixed = self.mixed_navigation(f)
            if action == 'wait-completed':
                # Matching expired history may be skipped only if family-valid.
                mixed['Binding']['Deadline'] = '2000-01-01T00:00:00Z'
                self.put(f['mailbox'] / 'request-000001.json', mixed)
                self.put(f['mailbox'] / 'response-000001.json',
                         dict(Binding=mixed['Binding'], PayloadSHA256=h.digest(b'[]'),
                              PayloadByteLength=2, Payload='W10='))
                later = copy.deepcopy(f['valid'])
                later['Binding']['Sequence'] = 2
                self.put(f['mailbox'] / 'request-000002.json', later)
            assertion = 'ASSERT_LEGACY_COHORT_CLOSURE_' + action.upper().replace('-', '_')
            self.assertTrue(self.refused(action, f), assertion)
            if action == 'respond':
                self.assertFalse((f['mailbox'] / 'response-000001.json').exists(), assertion)
            if action == 'export':
                self.assertFalse(f['dest'].exists(), assertion)
            print(assertion + ': PASS')

    def test_mixed_request_refused(self):
        self.mixed_guard('request')

    def test_mixed_cli_read_refused(self):
        self.mixed_guard('read')

    def test_mixed_export_refused(self):
        self.mixed_guard('export')

    def test_mixed_respond_refused(self):
        self.mixed_guard('respond')

    def test_mixed_pending_wait_refused(self):
        self.mixed_guard('wait-pending')

    def test_mixed_completed_wait_refused(self):
        self.mixed_guard('wait-completed')

    def test_legacy_shape_markers_refused(self):
        cases = ('missing-protocol', 'foreign-protocol', 'items', 'item-bindings', 'navigation-wrapper',
                 'binding-items', 'binding-item-bindings', 'binding-navigation', 'binding-unknown')
        for kind in cases:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory(prefix='.owned-family-', dir=Path.home()) as base:
                f = self.fixture(base)
                frame = copy.deepcopy(f['valid'])
                if kind == 'missing-protocol':
                    del frame['Request']['Protocol']
                elif kind == 'foreign-protocol':
                    frame['Request']['Protocol'] = 'unknown-semantic-family'
                elif kind == 'items':
                    frame['Request']['Items'] = None
                elif kind == 'item-bindings':
                    frame['Request']['ItemBindings'] = []
                elif kind == 'navigation-wrapper':
                    frame['Request']['Navigation'] = {}
                else:
                    field = {'binding-items': 'Items', 'binding-item-bindings': 'ItemBindings',
                             'binding-navigation': 'Navigation', 'binding-unknown': 'UnknownBinding'}[kind]
                    frame['Binding'][field] = None
                frame['Binding']['RequestSHA256'] = h.digest(h.canonical(frame['Request']))
                self.put(f['mailbox'] / 'request-000001.json', frame)
                self.assertTrue(self.refused('request', f), 'ASSERT_LEGACY_SHAPE_' + kind)
                print('ASSERT_LEGACY_SHAPE_' + kind + ': PASS')

    def test_valid_legacy_navigation_and_completed_history(self):
        for stage in ('INTERPRET', 'REVIEW_TECHNICAL', 'NAVIGATE', 'REVIEW_NAVIGATION'):
            with self.subTest(stage=stage), tempfile.TemporaryDirectory(prefix='.owned-family-', dir=Path.home()) as base:
                f = self.fixture(base)
                frame = copy.deepcopy(f['valid'])
                frame['Binding']['Stage'] = frame['Request']['Stage'] = stage
                frame['Request']['Navigation'] = [] if stage == 'REVIEW_NAVIGATION' else None
                frame['Binding']['RequestSHA256'] = h.digest(h.canonical(frame['Request']))
                raw = h.canonical(frame)
                self.put(f['mailbox'] / 'request-000001.json', frame)
                fd = h.directory(str(f['mailbox']))
                try:
                    got, exact, _ = h.request(fd, 1, retain=True)
                finally:
                    os.close(fd)
                self.assertEqual(exact, raw, 'ASSERT_VALID_LEGACY_BYTES')
                self.assertEqual(h.canonical(got), raw, 'ASSERT_VALID_LEGACY_BYTES')
                # Valid expired completed history must not become a liveness error.
                frame['Binding']['Deadline'] = '2000-01-01T00:00:00Z'
                self.put(f['mailbox'] / 'request-000001.json', frame)
                self.put(f['mailbox'] / 'response-000001.json',
                         dict(Binding=frame['Binding'], PayloadSHA256=h.digest(b'[]'), PayloadByteLength=2, Payload='W10='))
                later = copy.deepcopy(f['valid'])
                later['Binding']['Sequence'] = 2
                self.put(f['mailbox'] / 'request-000002.json', later)
                self.assertEqual(h.wait_request(str(f['mailbox']), 'owned/interpreter', str(f['control']), poll_ms=1),
                                 dict(Status='REQUEST', Binding=later['Binding']), 'ASSERT_VALID_LEGACY_HISTORY')
                print('ASSERT_VALID_LEGACY_BYTES_HISTORY_' + stage + ': PASS')


if __name__ == '__main__':
    unittest.main()
