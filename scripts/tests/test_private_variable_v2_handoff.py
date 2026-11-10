import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('handoff', Path(__file__).resolve().parents[1] / 'private-variable-v2-host-handoff.py')
h = importlib.util.module_from_spec(spec)
spec.loader.exec_module(h)

class HandoffTest(unittest.TestCase):
    def export_fixture(self, base, size=229327):
        base = Path(base)
        mailbox, control = base / 'mailbox', base / 'control'
        for p in (mailbox, control):
            p.mkdir(mode=0o700)
        limits = dict(MaxRequestBytes=1 << 20, MaxResponseBytes=1 << 20)
        c = dict(schema='private-variable-scope-v2-caller.v4', purpose='PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL',
                 code_root=str(base / 'code'), scope=dict(root=str(base / 'source')),
                 operation_directory=str(base / 'journal'), producer_outputs=[str(base / 'producer')],
                 post_outputs=[str(base / 'post')], protected_roots=[str(base / 'protected')],
                 report_path=str(base / 'report.json'), attempt_marker_path=str(base / 'attempt.json'),
                 automation=dict(Directory=str(mailbox), Invocation='owned-export', Limits=limits))
        cfg = base / 'config.json'
        cfg.write_bytes(h.canonical(c))
        cfg.chmod(0o600)
        prep = dict(Protocol=h.PROTOCOL, Invocation='owned-export', OwnerNonce='fresh-owned-nonce',
                    PreparationSHA256=h.digest(b'prepared'), Limits=limits)
        # Preserve source strings, direction, repeated occurrences, null/[],
        # exponent spelling and negative zero; this is synthetic, never replay.
        req = b'{"Stage":"INTERPRET","Numbers":[1e-9,-0,1e+21],"Direction":"OUTGOING","Occurrences":["dup","dup"],"Missing":null,"Empty":[],"Body":"'
        req += ('a' * 19137 + '界🙂e\u0301\r\n'.replace('\r', '\\r').replace('\n', '\\n')).encode()
        # Family-closure fixture adjustment: explicitly name the real legacy
        # semantic protocol while keeping the opaque byte/UTF8/lexeme exercise.
        tail = b'","Protocol":"private-provisional-inventory.v2","Tail":"OWNED_TAIL_SENTINEL"}'
        def framed(request):
            binding = dict(**{k: prep[k] for k in ('Protocol', 'Invocation', 'OwnerNonce', 'PreparationSHA256')},
                           Sequence=1, Stage='INTERPRET', RoleID='owned/interpreter', RequestSHA256=h.digest(request),
                           InputSHA256=h.digest(b'input'), Target='own-target', Attempt=0, PredecessorDigest='',
                           ResponseAllowance=8192, Deadline='2099-01-01T00:00:00Z')
            return b'{"Binding":' + h.canonical(binding) + b',"Request":' + request + b'}'
        pad = size - len(framed(req + tail))
        self.assertGreaterEqual(pad, 0)
        request = req + b'z' * pad + tail
        raw = framed(request)
        self.assertEqual(len(raw), size)
        fd = h.directory(str(mailbox))
        try:
            h.publish(fd, 'preparation.json', h.canonical(prep))
            h.publish(fd, 'request-000001.json', raw)
        finally:
            os.close(fd)
        return dict(base=base, mailbox=mailbox, control=control, config=cfg, pin=h.digest(cfg.read_bytes()),
                    dest=base / 'export', raw=raw, request=request)

    def export(self, f):
        fd = h.directory(str(f['mailbox']))
        try:
            return h.export_request(fd, str(f['mailbox']), 1, str(f['dest']), str(f['config']), f['pin'], str(f['control']))
        finally:
            os.close(fd)

    def export_command(self, f):
        return [h.sys.executable, str(Path(h.__file__)), 'read', '--mailbox', str(f['mailbox']), '--sequence', '1',
                '--export-dir', str(f['dest']), '--config', str(f['config']), '--config-sha256', f['pin'], '--control', str(f['control'])]

    def assert_export_exact(self, f, receipt):
        root = f['dest']
        manifest_raw = (root / receipt['Manifest']).read_bytes()
        self.assertEqual((len(manifest_raw), h.digest(manifest_raw)), (receipt['ManifestByteLength'], receipt['ManifestSHA256']))
        m = h.decode(manifest_raw)
        parts = []
        offset = 0
        for index, p in enumerate(m['Parts'], 1):
            self.assertEqual(p['Name'], f'part-{index:06d}.txt')
            part = (root / p['Name']).read_bytes()
            part.decode('utf-8', errors='strict')
            self.assertTrue(0 < len(part) <= 20000)
            self.assertEqual((p['Start'], p['End'], p['ByteLength'], p['SHA256']), (offset, offset + len(part), len(part), h.digest(part)))
            offset += len(part)
            parts.append(part)
        self.assertEqual(b''.join(parts), f['raw'], 'EXPORT_ORDERED_EXACT')
        self.assertEqual((root / 'frame.json').read_bytes(), f['raw'])
        self.assertEqual(m['Frame'], dict(Name='frame.json', SHA256=h.digest(f['raw']), ByteLength=len(f['raw'])))
        self.assertEqual((root / 'request.json').read_bytes(), f['request'])
        self.assertEqual((m['Request']['SHA256'], m['Request']['ByteLength']), (h.digest(f['request']), len(f['request'])))
        self.assertEqual(f['raw'][m['Request']['Start']:m['Request']['End']], f['request'])
        self.assertEqual(m['Binding'], h.decode(f['raw'])['Binding'])
        self.assertEqual((receipt['FrameSHA256'], receipt['FrameByteLength']), (h.digest(f['raw']), len(f['raw'])))
        self.assertEqual((receipt['RequestSHA256'], receipt['RequestByteLength']), (h.digest(f['request']), len(f['request'])))
        self.assertLessEqual(len(parts), h.EXPORT_MAX_PARTS)
        self.assertEqual(len(list(root.iterdir())), len(parts) + 3)
        for p in root.iterdir():
            info = p.stat()
            self.assertEqual((info.st_mode & 0o777, info.st_uid, info.st_nlink), (0o600, os.getuid(), 1))
        self.assertEqual(root.stat().st_mode & 0o777, 0o700)
        return m

    def test_export_actual_shape_and_configured_ceiling(self):
        for size in (229327, 1 << 20):
            with self.subTest(size=size), tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
                f = self.export_fixture(base, size)
                result = h.subprocess.run(self.export_command(f), capture_output=True, check=True)
                self.assertEqual(result.stderr, b'')
                self.assertLessEqual(len(result.stdout), h.EXPORT_RECEIPT_CAP + 1, 'EXPORT_METADATA_ONLY')
                self.assertNotIn(b'OWNED_TAIL_SENTINEL', result.stdout, 'EXPORT_METADATA_ONLY')
                self.assertNotIn(b'Body', result.stdout, 'EXPORT_METADATA_ONLY')
                receipt = h.json.loads(result.stdout)
                self.assert_export_exact(f, receipt)
                print(f'EXPORT_ORDERED_EXACT: A-pass size={size} tail, unicode, numeric lexemes, order, duplicates, null/[]')
                print('EXPORT_METADATA_ONLY: A-pass bounded stdout, no request/source')
                # Legacy full JSON read is still the same exact frame plus LF.
                legacy = h.subprocess.run(self.export_command(f)[:7], capture_output=True, check=True)
                self.assertEqual(legacy.stdout, f['raw'] + b'\n')

    def test_utf8_boundary(self):
        # Force a 4-byte codepoint to straddle the nominal first part boundary.
        with tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
            f = self.export_fixture(base)
            raw = f['raw']
            pos = raw.index('界'.encode())
            delta = 19999 - pos
            self.assertGreater(delta, 0)
            raw = raw[:pos] + b'a' * delta + raw[pos:]
            request = f['request'].replace(b'a' * 19137, b'a' * (19137 + delta), 1)
            raw = raw.replace(h.digest(f['request']).encode(), h.digest(request).encode(), 1)
            p = f['mailbox'] / 'request-000001.json'
            p.write_bytes(raw)
            f.update(raw=raw, request=request)
            receipt = self.export(f)
            m = self.assert_export_exact(f, receipt)
            self.assertEqual(m['Parts'][0]['End'], 19999)
            print('EXPORT_UTF8_BOUNDARY: A-pass first cut backs up to 19999')

    def test_export_counterfactual_tail_and_order(self):
        # Present-but-wrong production publication overlays; guards run on the
        # resulting files, not on a guessed compile/setup failure.
        for mutation in ('omit-tail', 'reverse-parts'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
                f = self.export_fixture(base)
                actual = h.publish
                held = []
                def wrong(fd, name, raw):
                    if name.startswith('part-'):
                        held.append((name, raw))
                        return
                    if name == 'manifest.json':
                        items = held[:-1] if mutation == 'omit-tail' else list(zip([n for n, _ in held], [r for _, r in reversed(held)]))
                        for n, r in items:
                            actual(fd, n, r)
                    return actual(fd, name, raw)
                with patch.object(h, 'publish', side_effect=wrong):
                    receipt = self.export(f)
                # The assertion itself evaluates assembled bytes even when a
                # part is absent; no file-not-found stands in for the RED.
                m = h.decode((f['dest'] / 'manifest.json').read_bytes())
                combined = b''.join((f['dest'] / p['Name']).read_bytes() if (f['dest'] / p['Name']).exists() else b'' for p in m['Parts'])
                with self.assertRaisesRegex(AssertionError, 'EXPORT_ORDERED_EXACT'):
                    self.assertEqual(combined, f['raw'], 'EXPORT_ORDERED_EXACT')
                print(f'EXPORT_ORDERED_EXACT: A-fail {mutation} (observed byte inequality)')
                f['dest'] = Path(base) / 'restored-export'
                self.assert_export_exact(f, self.export(f))
                print(f'EXPORT_ORDERED_EXACT: A-pass restored after {mutation}')

    def test_export_refusals_and_partial_diagnostics(self):
        for kind in ('existing', 'symlink', 'root-symlink', 'mailbox', 'control', 'source', 'protected', 'config', 'late', 'terminal', 'cancel', 'responded', 'bad-binding', 'malformed', 'duplicate-key', 'over-ceiling', 'partial', 'expires-during-export'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
                f = self.export_fixture(base, (1 << 20) + 1 if kind == 'over-ceiling' else 229327)
                if kind == 'existing':
                    f['dest'].mkdir(mode=0o700)
                    (f['dest'] / 'unrelated').write_bytes(b'untouched')
                elif kind == 'symlink':
                    f['dest'].symlink_to(f['control'], target_is_directory=True)
                elif kind == 'root-symlink':
                    (Path(base) / 'protected').symlink_to(base, target_is_directory=True)
                elif kind in ('mailbox', 'control', 'source', 'protected'):
                    f['dest'] = Path(base) / kind / 'export'
                elif kind == 'config':
                    f['dest'] = f['config']
                elif kind in ('late', 'bad-binding', 'malformed', 'duplicate-key'):
                    p = f['mailbox'] / 'request-000001.json'
                    raw = f['raw']
                    if kind == 'late':
                        raw = raw.replace(b'2099-', b'2000-')
                    elif kind == 'bad-binding':
                        raw = raw.replace(b'fresh-owned-nonce', b'wrong-owned-nonce')
                    elif kind == 'duplicate-key':
                        raw = raw.replace(b'"Sequence":1', b'"Sequence":1,"Sequence":1')
                    else:
                        raw = raw[:-1]
                    p.write_bytes(raw)
                elif kind in ('terminal', 'cancel', 'responded'):
                    target = f['control'] / 'stop.json' if kind == 'cancel' else f['mailbox'] / ('terminal.json' if kind == 'terminal' else 'response-000001.json')
                    target.write_bytes(b'{}')
                    target.chmod(0o600)
                actual = h.publish
                calls = []
                def partial(fd, name, raw):
                    calls.append(name)
                    if len(calls) == 2 and kind == 'partial':
                        raise OSError('owned injected partial failure')
                    actual(fd, name, raw)
                clock = h.time.time
                def now():
                    return 1e20 if kind == 'expires-during-export' and len(calls) >= 2 else clock()
                with patch.object(h, 'publish', side_effect=partial), patch.object(h.time, 'time', side_effect=now):
                    with self.assertRaises((OSError, ValueError, KeyError)):
                        self.export(f)
                self.assertFalse((f['dest'] / 'manifest.json').exists())
                if kind in ('partial', 'expires-during-export'):
                    self.assertTrue((f['dest'] / 'frame.json').exists())
                    with self.assertRaises(FileExistsError):
                        self.export(f)
                print(f'EXPORT_REFUSAL: A-pass {kind}')

    def test_export_directory_identity_alias(self):
        with tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
            f = self.export_fixture(base)
            actual = h.os.stat
            ancestor = actual(base)
            def aliased(path, *args, **kwargs):
                # Owned metadata overlay models a second spelling of an existing
                # protected directory on a case-insensitive filesystem.
                if path == str(Path(base) / 'protected'):
                    return ancestor
                return actual(path, *args, **kwargs)
            with patch.object(h.os, 'stat', side_effect=aliased):
                with self.assertRaisesRegex(ValueError, 'aliases supplied root'):
                    self.export(f)
            self.assertFalse(f['dest'].exists())
            print('EXPORT_DIRECTORY_ALIAS: A-pass same-directory identity refused before mkdir')

    def test_export_cli_bounded_untrusted_refusal(self):
        with tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
            f = self.export_fixture(base)
            p = f['mailbox'] / 'request-000001.json'
            p.write_bytes(f['raw'].replace(b'2099-01-01T00:00:00Z', b'UNTRUSTED_DEADLINE_SECRET' * 1000))
            result = h.subprocess.run(self.export_command(f), capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.stdout, b'')
            self.assertLessEqual(len(result.stderr), 128)
            self.assertNotIn(b'UNTRUSTED_DEADLINE_SECRET', result.stderr)
            self.assertFalse(f['dest'].exists())
            print('EXPORT_BOUNDED_REFUSAL: A-pass no untrusted parser/date text in stdout/stderr')

    def test_export_retains_one_validated_frame(self):
        with tempfile.TemporaryDirectory(prefix='.owned-export-', dir=Path.home()) as base:
            f = self.export_fixture(base)
            actual_publish, actual_read = h.publish, h.read
            reads = []
            def counted(fd, name, *args, **kwargs):
                reads.append(name)
                return actual_read(fd, name, *args, **kwargs)
            def swapped(fd, name, raw):
                if name == 'frame.json':
                    (f['mailbox'] / 'request-000001.json').write_bytes(b'{}')
                actual_publish(fd, name, raw)
            with patch.object(h, 'read', side_effect=counted), patch.object(h, 'publish', side_effect=swapped):
                receipt = self.export(f)
            self.assertEqual(reads.count('request-000001.json'), 1)
            self.assert_export_exact(f, receipt)
            print('EXPORT_SINGLE_READ: A-pass validated object retained across path substitution')

    def test_actual_respond_unsafe_late_and_oversized(self):
        for kind in ('hardlink', 'symlink', 'empty', 'mode', 'oversized', 'late', 'allowance'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory(prefix='.owned-return-', dir=Path.home()) as base:
                f = self.export_fixture(base)
                payloads = Path(base) / 'payloads'
                payloads.mkdir(mode=0o700)
                p = payloads / 'payload.json'
                p.write_bytes(b'[]')
                p.chmod(0o600)
                if kind == 'hardlink':
                    os.link(p, payloads / 'other')
                elif kind == 'symlink':
                    p.rename(payloads / 'other')
                    p.symlink_to(payloads / 'other')
                elif kind == 'mode':
                    p.chmod(0o644)
                elif kind == 'empty':
                    p.write_bytes(b'')
                elif kind == 'oversized':
                    p.write_bytes(b'x' * ((1 << 20) + 1))
                else:
                    q = f['mailbox'] / 'request-000001.json'
                    q.write_bytes(f['raw'].replace(b'2099-', b'2000-') if kind == 'late' else f['raw'].replace(b'"ResponseAllowance":8192', b'"ResponseAllowance":1'))
                result = h.subprocess.run([h.sys.executable, str(Path(h.__file__)), 'respond', '--mailbox', str(f['mailbox']), '--sequence', '1', '--payload', str(p)], capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, b'')
                self.assertFalse((f['mailbox'] / 'response-000001.json').exists())
                print(f'RESPONSE_REFUSAL: A-pass {kind}')

    def test_response_exact_counterfactual_normalization(self):
        expected = b' \r\n{"b":1e-9,"a":null,"dup":[1,1]} \r\n'
        for normalize in (True, False):
            with tempfile.TemporaryDirectory(prefix='.owned-return-', dir=Path.home()) as base:
                f = self.export_fixture(base)
                payloads = Path(base) / 'payloads'
                payloads.mkdir(mode=0o700)
                p = payloads / 'payload.json'
                p.write_bytes(expected)
                p.chmod(0o600)
                actual = h.read
                def read_wrong(fd, name, *args, **kwargs):
                    result = actual(fd, name, *args, **kwargs)
                    return result.strip() if normalize and name == 'payload.json' else result
                with patch.object(h, 'read', side_effect=read_wrong):
                    receipt = h.respond(str(f['mailbox']), 1, str(p))
                raw = (f['mailbox'] / 'response-000001.json').read_bytes()
                result = h.decode(raw)
                got = h.base64.b64decode(result['Payload'])
                if normalize:
                    with self.assertRaisesRegex(AssertionError, 'RESPONSE_EXACT_BYTES'):
                        self.assertEqual(got, expected, 'RESPONSE_EXACT_BYTES')
                    print('RESPONSE_EXACT_BYTES: A-fail normalization overlay (observed byte inequality)')
                else:
                    self.assertEqual(got, expected, 'RESPONSE_EXACT_BYTES')
                    self.assertEqual((result['PayloadSHA256'], result['PayloadByteLength']), (h.digest(expected), len(expected)))
                    self.assertEqual(receipt, dict(ResponseSHA256=h.digest(raw), ResponseByteLength=len(raw)))
                    self.assertEqual(result['Binding'], h.decode(f['raw'])['Binding'])
                    print('RESPONSE_EXACT_BYTES: A-pass restored raw payload, digest, length, binding')

    def test_verified_worker_object_survives_path_swap(self):
        self.assert_worker_swap(b'#!/bin/sh\nprintf "REVIEWED_OBJECT\\n"\n', b'REVIEWED_OBJECT')

    def test_native_worker_object_survives_path_swap(self):
        # Owned native executable, never the real acquisition entry or a copied
        # platform-signed system binary. Compile fixed inert source only.
        with tempfile.TemporaryDirectory(prefix='.owned-host-native-', dir=Path.home()) as base:
            binary = Path(base) / 'native'
            h.subprocess.run(['/usr/bin/cc', '-x', 'c', '-o', str(binary), '-'], input=b'#include <stdio.h>\nint main(void) { puts("REVIEWED_NATIVE"); return 0; }\n', check=True, stdout=h.subprocess.PIPE, stderr=h.subprocess.PIPE, env={'PATH': '/usr/bin:/bin'})
            self.assert_worker_swap(binary.read_bytes(), b'REVIEWED_NATIVE')

    def assert_worker_swap(self, original, expected):
        # Interpose only at actual process creation; execution is never mocked.
        with tempfile.TemporaryDirectory(prefix='.owned-host-exec-', dir=Path.home()) as base:
            base = Path(base)
            control = base / 'control'
            control.mkdir(mode=0o700)
            config = base / 'config.json'
            raw = h.canonical(dict(schema='private-variable-scope-v2-caller.v4', purpose='PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL', automation=dict(Invocation='owned'), operation_timeout_ms=2000, shutdown_timeout_ms=100))
            config.write_bytes(raw)
            config.chmod(0o600)
            worker = base / 'worker'
            worker.write_bytes(original)
            worker.chmod(0o700)
            foreign = base / 'foreign'
            foreign.write_bytes(b'#!/bin/sh\nprintf "FOREIGN_OBJECT\\n"\n')
            foreign.chmod(0o700)
            actual_popen = h.subprocess.Popen
            calls = []
            def swap_then_spawn(*args, **kwargs):
                calls.append(True)
                os.replace(foreign, worker)
                return actual_popen(*args, **kwargs)
            args = SimpleNamespace(config=str(config), config_sha256=h.digest(raw), worker=str(worker), worker_sha256=h.digest(original), control=str(control))
            with patch.object(h.subprocess, 'Popen', side_effect=swap_then_spawn):
                h.supervise(args)
            output = (control / 'worker-output.txt').read_bytes()
            receipt = h.decode((control / 'exit.json').read_bytes())
            self.assertEqual(len(calls), 1)
            self.assertTrue(expected in output and b'FOREIGN_OBJECT' not in output and receipt == dict(ExitCode=0, Stopped=False, Joined=True), 'HOST_WORKER_OBJECT: FAIL')
            self.assertFalse(any(p.name.startswith('.exec-') for p in control.iterdir()), 'private execution snapshot leaked')
            print('HOST_WORKER_OBJECT: PASS actual verified-object execution after pathname replacement')

    def test_go_numeric_lexemes_are_not_rewritten(self):
        raw = b'{"Request":{"tiny":1e-9,"zero":-0,"large":18446744073709551615},"Body":"\\\\u003cdata\\\\u003e"}'
        self.assertEqual(h.canonical(h.decode(raw)), raw)
        # This plausible ordinary Python re-encoding would change both 1e-9
        # and -0. Exact framed evidence must not take that path.
        ordinary = h.json.loads(raw)
        self.assertNotEqual(h.canonical(ordinary), raw)
    def test_exact_payload_and_duplicate(self):
        with tempfile.TemporaryDirectory(prefix='.owned-host-helper-', dir=Path.home()) as base:
            mailbox = Path(base) / 'mailbox'
            payloads = Path(base) / 'payloads'
            mailbox.mkdir(mode=0o700)
            payloads.mkdir(mode=0o700)
            fd = h.directory(str(mailbox))
            try:
                prep = dict(Protocol=h.PROTOCOL, Invocation='owned', OwnerNonce='nonce', PreparationSHA256=h.digest(b'prepared'))
                req = dict(Stage='NAVIGATE', Protocol='private-provisional-inventory.v2')
                binding = dict(**prep, Sequence=1, Stage='NAVIGATE', RoleID='same-model/host-role/session', RequestSHA256=h.digest(h.canonical(req)), InputSHA256='', Target='', Attempt=0, PredecessorDigest='', ResponseAllowance=4096, Deadline='2099-01-01T00:00:00Z')
                h.publish(fd, 'preparation.json', h.canonical(prep))
                h.publish(fd, 'request-000001.json', h.canonical(dict(Binding=binding, Request=req)))
                payload = payloads / 'payload.json'
                payload.write_bytes(b'not canonical model bytes\n')
                payload.chmod(0o600)
                h.respond(str(mailbox), 1, str(payload))
                response = h.decode(h.read(fd, 'response-000001.json'))
                self.assertEqual(h.base64.b64decode(response['Payload']), payload.read_bytes())
                with self.assertRaises(FileExistsError):
                    h.respond(str(mailbox), 1, str(payload))
                # Publication cannot replace an earlier exact response.
                self.assertEqual(h.base64.b64decode(h.decode(h.read(fd, 'response-000001.json'))['Payload']), payload.read_bytes())
            finally:
                os.close(fd)
    def test_alias_and_hardlink(self):
        with tempfile.TemporaryDirectory(prefix='.owned-host-helper-', dir=Path.home()) as base:
            fd = h.directory(base)
            try:
                h.publish(fd, 'one', b'x')
                os.link(Path(base) / 'one', Path(base) / 'two')
                with self.assertRaises(ValueError):
                    h.read(fd, 'one')
                os.symlink(Path(base) / 'one', Path(base) / 'alias')
                with self.assertRaises(OSError):
                    h.read(fd, 'alias')
            finally:
                os.close(fd)

class WaitTest(unittest.TestCase):
    export_fixture = HandoffTest.export_fixture

    def fixture(self):
        tmp = tempfile.TemporaryDirectory(prefix='.owned-wait-', dir=Path.home())
        self.addCleanup(tmp.cleanup)
        f = self.export_fixture(tmp.name)
        self.put(f['control'] / 'start.json', dict(Schema='private.onefile-actual-start.v2',
                 ParentInvokedOperator=True, Invocation='owned-export',
                 ScopeSHA256=h.digest(b'scope'), FullPlanSHA256=h.digest(b'plan'),
                 ActualRoleBindingsSHA256=h.digest(b'roles'),
                 StartedUnix=1000000000, HardDeadlineUnix=4102444800))
        return f

    def put(self, path, value):
        path.write_bytes(h.canonical(value))
        path.chmod(0o600)

    def make_request(self, f, n, role='owned/interpreter', body='OWNED_PAYLOAD_CANARY'):
        frame = h.decode(f['raw'])
        frame['Request'] = dict(Stage='INTERPRET', Body=body, Protocol='private-provisional-inventory.v2')
        frame['Binding'].update(Sequence=n, RoleID=role,
                                RequestSHA256=h.digest(h.canonical(frame['Request'])))
        self.put(f['mailbox'] / f'request-{n:06d}.json', frame)
        return frame['Binding']

    def wait(self, f, **kwargs):
        return h.wait_request(str(f['mailbox']), 'owned/interpreter', str(f['control']), **kwargs)

    def snapshot(self, f):
        return {str(p.relative_to(f['base'])): (p.read_bytes(), p.stat().st_mode)
                for root in (f['mailbox'], f['control']) for p in root.iterdir() if p.is_file()}

    def test_wait_old_predicate_red_then_real_green(self):
        f = self.fixture()
        binding = h.decode(f['raw'])['Binding']
        before = self.snapshot(f)
        # Frozen defect only: ordinary API, real existing request, no undefined
        # helper or setup error. The filename contains no 'binding' substring.
        def old_wait(mailbox, role_id, control, after_sequence=0, poll_ms=100):
            selected = [p for p in Path(mailbox).iterdir() if 'binding' in p.name]
            return {'Status': 'REQUEST', 'Binding': binding} if selected else {'Status': 'DEADLINE'}
        def discovery(selector):
            got = selector(str(f['mailbox']), 'owned/interpreter', str(f['control']))
            self.assertEqual(got, dict(Status='REQUEST', Binding=binding), 'WAIT_NESTED_ROLE')
        with self.assertRaisesRegex(AssertionError, 'WAIT_NESTED_ROLE'):
            discovery(old_wait)
        print('WAIT_NESTED_ROLE: A-fail frozen filename-binding predicate')
        with patch.object(h.time, 'sleep', side_effect=AssertionError('must notify before sleeping')):
            discovery(h.wait_request)
        print('WAIT_NESTED_ROLE: A-pass actual nested Binding selector')
        self.assertEqual(before, self.snapshot(f), 'WAIT_READ_ONLY')

    def test_wait_cli_binding_only_and_other_role_privacy(self):
        f = self.fixture()
        self.make_request(f, 1, 'other', 'OTHER_ROLE_REQUEST_CANARY')
        self.make_request(f, 2, 'other', 'OTHER_ROLE_REQUEST_CANARY')
        binding = self.make_request(f, 3)
        command = [h.sys.executable, h.__file__, 'wait', '--mailbox', str(f['mailbox']),
                   '--role-id', 'owned/interpreter', '--control', str(f['control'])]
        result = h.subprocess.run(command, capture_output=True, check=True)
        self.assertEqual(result.stderr, b'')
        self.assertEqual(result.stdout, h.canonical(dict(Status='REQUEST', Binding=binding)) + b'\n')
        self.assertNotIn(b'OTHER_ROLE_REQUEST_CANARY', result.stdout + result.stderr)
        self.assertNotIn(b'OWNED_PAYLOAD_CANARY', result.stdout + result.stderr)
        self.assertNotIn(b'"Request":', result.stdout)

    def test_wait_even_sequence_and_absolute_bound(self):
        f = self.fixture()
        self.make_request(f, 1, 'other')
        binding = self.make_request(f, 2)
        self.assertEqual(self.wait(f), dict(Status='REQUEST', Binding=binding))
        binding = self.make_request(f, 4096)
        self.assertEqual(self.wait(f, after_sequence=2), dict(Status='REQUEST', Binding=binding))

    def test_wait_response_and_cursor_no_replay_numeric_order(self):
        f = self.fixture()
        payload = f['base'] / 'payload'
        payload.write_bytes(b'[]')
        payload.chmod(0o600)
        h.respond(str(f['mailbox']), 1, str(payload))
        # Completion, not an expired deadline, determines whether to skip.
        for name in ('request-000001.json', 'response-000001.json'):
            path = f['mailbox'] / name
            frame = h.decode(path.read_bytes())
            frame['Binding']['Deadline'] = '2000-01-01T00:00:00Z'
            self.put(path, frame)
        self.make_request(f, 10)
        expected = self.make_request(f, 9)
        self.assertEqual(self.wait(f), dict(Status='REQUEST', Binding=expected))
        # Matching malformed completed bindings must not hide behind a valid
        # later pending row. Expired-but-well-formed history above remains valid.
        for field, value in (('Deadline', None), ('Deadline', 'not-a-deadline'),
                             ('Deadline', 123), ('RoleID', None), ('RoleID', 123),
                             ('Stage', None), ('Stage', 123)):
            with self.subTest(completed_field=field, value=value):
                paths = [f['mailbox'] / name for name in ('request-000001.json', 'response-000001.json')]
                originals = [path.read_bytes() for path in paths]
                try:
                    for path, raw in zip(paths, originals):
                        row = h.decode(raw)
                        if value is None:
                            del row['Binding'][field]
                        else:
                            row['Binding'][field] = value
                        self.put(path, row)
                    try:
                        self.wait(f)
                    except ValueError as error:
                        refused = str(error) == 'wait refused'
                    else:
                        refused = False
                    self.assertTrue(refused, f'WAIT_COMPLETED_METADATA:{field}:{value!r}')
                    print(f'WAIT_COMPLETED_METADATA:{field}:{value!r}: A-pass refused')
                finally:
                    for path, raw in zip(paths, originals):
                        path.write_bytes(raw)
        self.assertEqual(self.wait(f), dict(Status='REQUEST', Binding=expected))
        with patch.object(h.time, 'sleep', side_effect=lambda _: self.put(f['control'] / 'stop.json', {})):
            self.assertEqual(self.wait(f, after_sequence=10), {'Status': 'STOP'})

    def test_wait_closure_priority_and_no_payload(self):
        for owner, name, status in (('mailbox', 'terminal.json', 'TERMINAL'),
                                    ('control', 'stop.json', 'STOP'),
                                    ('control', 'actual-exit.json', 'STOP')):
            with self.subTest(name=name):
                f = self.fixture()
                self.put(f[owner] / name, dict(Secret='OTHER_ROLE_CANARY'))
                before = self.snapshot(f)
                self.assertEqual(self.wait(f), {'Status': status})
                self.assertEqual(before, self.snapshot(f))

    def test_wait_fixed_deadline_and_expired_request(self):
        f = self.fixture()
        with patch.object(h.time, 'time', return_value=4102444800):
            self.assertEqual(self.wait(f), {'Status': 'DEADLINE'})
        self.make_request(f, 1)
        path = f['mailbox'] / 'request-000001.json'
        frame = h.decode(path.read_bytes())
        frame['Binding']['Deadline'] = '2000-01-01T00:00:00Z'
        self.put(path, frame)
        self.assertEqual(self.wait(f), {'Status': 'STOP'})
        path.unlink()
        def deadline_on_sleep(_):
            start = h.decode((f['control'] / 'start.json').read_bytes())
            start['HardDeadlineUnix'] = 4102444801
            self.put(f['control'] / 'start.json', start)
        with patch.object(h.time, 'sleep', side_effect=deadline_on_sleep):
            with self.assertRaisesRegex(ValueError, '^wait refused$'):
                self.wait(f)

    def test_wait_empty_fixed_deadline_and_stop_during_read(self):
        f = self.fixture()
        p = f['mailbox'] / 'request-000001.json'
        p.unlink()
        clock = [4102444799.95]
        def advance(delay):
            self.assertLessEqual(delay, 0.051)
            clock[0] = 4102444800
        before = self.snapshot(f)
        with patch.object(h.time, 'time', side_effect=lambda: clock[0]), patch.object(h.time, 'sleep', side_effect=advance) as sleep:
            self.assertEqual(self.wait(f), {'Status': 'DEADLINE'})
            sleep.assert_called_once()
        self.assertEqual(before, self.snapshot(f))
        self.make_request(f, 1)
        original = h.request
        def stop_after_read(*args, **kwargs):
            frame = original(*args, **kwargs)
            self.put(f['control'] / 'stop.json', {})
            return frame
        with patch.object(h, 'request', side_effect=stop_after_read):
            self.assertEqual(self.wait(f), {'Status': 'STOP'})

    def test_wait_empty_then_publication_and_terminal_race(self):
        f = self.fixture()
        (f['mailbox'] / 'request-000001.json').unlink()
        with patch.object(h.time, 'sleep', side_effect=lambda _: self.make_request(f, 1)) as sleep:
            self.assertEqual(self.wait(f)['Status'], 'REQUEST')
            sleep.assert_called_once()
        original = h.request
        def terminal_after_read(*args, **kwargs):
            frame = original(*args, **kwargs)
            self.put(f['mailbox'] / 'terminal.json', {})
            return frame
        with patch.object(h, 'request', side_effect=terminal_after_read):
            self.assertEqual(self.wait(f), {'Status': 'TERMINAL'})

    def test_wait_unsafe_inputs_fail_closed(self):
        for kind in ('malformed', 'duplicate', 'digest', 'mode', 'symlink', 'hardlink',
                     'nonprivate', 'badname', 'overflow', 'wrong-response', 'response-mode',
                     'start-schema', 'start-invocation', 'start-missing', 'start-mode', 'start-time'):
            with self.subTest(kind=kind):
                f = self.fixture()
                p = f['mailbox'] / 'request-000001.json'
                if kind == 'malformed':
                    p.write_bytes(b'{"LEAK_CANARY":')
                elif kind == 'duplicate':
                    p.write_bytes(f['raw'].replace(b'"Sequence":1', b'"Sequence":1,"Sequence":1'))
                elif kind == 'digest':
                    p.write_bytes(f['raw'].replace(b'OWNED_TAIL_SENTINEL', b'OTHER_TAIL_SENTINEL'))
                elif kind == 'mode':
                    p.chmod(0o644)
                elif kind == 'symlink':
                    p.rename(f['base'] / 'target')
                    p.symlink_to(f['base'] / 'target')
                elif kind == 'hardlink':
                    os.link(p, f['base'] / 'alias')
                elif kind == 'nonprivate':
                    f['mailbox'].chmod(0o755)
                elif kind in ('badname', 'overflow'):
                    self.put(f['mailbox'] / ('request-1.json' if kind == 'badname' else 'request-004097.json'), {})
                elif kind in ('wrong-response', 'response-mode'):
                    b = h.decode(f['raw'])['Binding']
                    if kind == 'wrong-response':
                        b['RoleID'] = 'other'
                    q = f['mailbox'] / 'response-000001.json'
                    self.put(q, dict(Binding=b, Payload='W10=', PayloadSHA256=h.digest(b'[]'), PayloadByteLength=2))
                    if kind == 'response-mode':
                        q.chmod(0o644)
                else:
                    q = f['control'] / 'start.json'
                    if kind == 'start-missing':
                        q.unlink()
                    elif kind == 'start-mode':
                        q.chmod(0o644)
                    else:
                        start = h.decode(q.read_bytes())
                        start[{'start-schema': 'Schema', 'start-invocation': 'Invocation', 'start-time': 'HardDeadlineUnix'}[kind]] = 'LEAK_CANARY'
                        self.put(q, start)
                command = [h.sys.executable, h.__file__, 'wait', '--mailbox', str(f['mailbox']),
                           '--role-id', 'owned/interpreter', '--control', str(f['control'])]
                result = h.subprocess.run(command, capture_output=True)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, b'')
                self.assertEqual(result.stderr, b'private handoff refused: wait refused\n')

    def test_wait_argument_bounds_and_fractional_start(self):
        f = self.fixture()
        for args in (dict(after_sequence=-1), dict(after_sequence=4097), dict(poll_ms=0), dict(poll_ms=1001)):
            with self.subTest(args=args), self.assertRaisesRegex(ValueError, '^wait refused$'):
                self.wait(f, **args)
        start = h.decode((f['control'] / 'start.json').read_bytes())
        start.update(StartedUnix=h.FloatLexeme('1000000000.125'), HardDeadlineUnix=h.FloatLexeme('4102444800.125'))
        self.put(f['control'] / 'start.json', start)
        self.assertEqual(self.wait(f)['Status'], 'REQUEST')


class SourceHostTest(unittest.TestCase):
    put = WaitTest.put
    export = HandoffTest.export
    assert_export_exact = HandoffTest.assert_export_exact

    def fixture(self):
        tmp = tempfile.TemporaryDirectory(prefix='.owned-source-host-', dir=Path.home())
        self.addCleanup(tmp.cleanup)
        base = Path(tmp.name)
        mailbox, control = base / 'mailbox', base / 'control'
        mailbox.mkdir(mode=0o700)
        control.mkdir(mode=0o700)
        sha = h.digest(b'controlled source')
        limits = dict(MaxCorrections=0, MaxCalls=1, MaxRequestBytes=1 << 20,
                      MaxResponseBytes=16384, MaxTotalBytes=4 << 20, MaxOutputBytes=1 << 20)
        m = dict(Enabled=True, Directory=str(mailbox), Invocation='controlled-source',
                 InterpreterRole='canned-source-role', ReviewerRole='unused-reviewer', ScopeSHA256=sha,
                 TimeoutMS=1000, OverallTimeoutMS=1000, PollMS=1, Limits=limits, ApprovalSHA256='')
        m['ApprovalSHA256'] = h.digest(h.canonical(m))
        cfg = dict(Protocol=h.SOURCE_PROTOCOL, Mailbox=m)
        prompt = 'SOURCE_ONLY controlled fixture; not HOST ' + '界🙂e\u0301' * 10000
        envelope = dict(protocol='adr0007-semantic-worker', message_type='request', message_id='',
                        correlation_id='', admission_id=sha, corpus='revision-bound-code',
                        item_type='assembled-source-projection', acquisition_mode='TARGET',
                        input_sha256=h.hashlib.sha256(prompt.encode()).hexdigest(), input_bytes=len(prompt.encode()),
                        source_revision=sha[7:], deadline_ms=1000, target_message_id='', prompt=prompt)
        lineage = dict(packet_id=sha, census_id=sha, lineage_identity='',
                       consumer_resolution='PRIVATE_SOURCE_ONLY_NO_CONSUMER_CLAIM', alternative_id='SOURCE_ONLY',
                       alternative_ordinal=0, authority=0, accepted=False, completeness='UNKNOWN', source_graph_complete='UNKNOWN')
        record = dict(record_id='', envelope=envelope, lineage=lineage)
        def identity(kind):
            return h.hashlib.sha256(('lsp-trace.describerequest.' + kind + '\x00').encode() + h.canonical(record)).hexdigest()
        lineage['lineage_identity'] = identity('lineage.v2')
        message, correlation = identity('message.v1'), identity('correlation.v1')
        envelope.update(message_id=message, correlation_id=correlation)
        record['record_id'] = identity('record.v1')
        source = dict(HandoffID=sha, CheckpointID=sha, SourceUnitID=sha, SourceDigest=sha,
                      RequestRecordID=record['record_id'], AttemptID='canned-attempt',
                      Workspace='file:///controlled', Deadline='2099-01-01T00:00:00Z')
        source_raw = h.canonical(source)
        prep = dict(Protocol=h.SOURCE_PROTOCOL, Invocation=m['Invocation'], OwnerNonce='a' * 64,
                    ScopeSHA256=sha, PreparationSHA256=h.digest(source_raw), PreparationByteLength=len(source_raw),
                    Limits=limits, Deadline=source['Deadline'], SourceConfiguration=cfg, Preparation=source)
        binding = dict(Protocol=h.SOURCE_PROTOCOL, Invocation=m['Invocation'], OwnerNonce=prep['OwnerNonce'],
                       Sequence=1, Stage=h.SOURCE_FAMILY, RoleID=m['InterpreterRole'],
                       RequestSHA256=h.digest(h.canonical(record)), PreparationSHA256=prep['PreparationSHA256'],
                       Source=source, ResponseAllowance=limits['MaxResponseBytes'], Deadline=source['Deadline'])
        frame = dict(Binding=binding, Request=record)
        self.put(mailbox / 'preparation.json', prep)
        self.put(mailbox / 'request-000001.json', frame)
        export = dict(Schema='private-source-describe-host-export.v1', Protocol=h.SOURCE_PROTOCOL,
                      SourceConfiguration=cfg, Roots=[str(base / 'source')], ScopeSHA256=sha,
                      PreparationSHA256=prep['PreparationSHA256'], Authority=0, Accepted=False,
                      Identity='UNRESOLVED', Completeness='UNKNOWN', Absence='PROHIBITED')
        config = base / 'config.json'
        self.put(config, export)
        return dict(base=base, mailbox=mailbox, control=control, config=config,
                    pin=h.digest(config.read_bytes()), dest=base / 'export', raw=h.canonical(frame),
                    request=h.canonical(record), frame=frame, prep=prep)

    def test_connected_start_one_invocation(self):
        self.connected_start_run()

    def test_connected_start_cancel_before_ack(self):
        self.connected_start_run(cancel=True)

    def connected_start_run(self, cancel=False):
        # Controlled transport peer only: not a provider, kernel custody, or model.
        # The separate Go tests check the typed join and native owner boundaries.
        f = self.fixture()
        from datetime import datetime, timezone, timedelta
        deadline = (datetime.now(timezone.utc) + timedelta(seconds=8)).isoformat().replace('+00:00', 'Z')
        c = dict(Schema='private-source-start.v2', Invocation=f['prep']['Invocation'],
                 Base=str(f['base']), Pin=h.digest(b'config-pin'), LaunchDeadline=deadline,
                 ProducerSession='controlled-producer-session', ReviewerSession='controlled-reviewer-session',
                 Source=dict(Host=f['prep']['SourceConfiguration']))
        config = f['base'] / 'start-config.json'
        self.put(config, c)
        cap = dict(Schema='private-source-start-capability.v1', ConfigSHA256=h.digest(config.read_bytes()),
                   ConfigPin=c['Pin'], Invocation=c['Invocation'], ProducerSession=c['ProducerSession'],
                   ReviewerSession=c['ReviewerSession'], RoleID='canned-source-role',
                   Mailbox=str(f['mailbox']), Control=str(f['control']), SourceTotalMS=15000,
                   Nonce='b' * 64, LaunchDeadline=deadline, ApproveParentStart=True)
        capability = f['base'] / 'capability.json'
        self.put(capability, cap)
        tool = f['base'] / 'controlled-peer'
        tool.write_text('''#!/usr/bin/env python3
import hashlib,json,os,sys,time
c=json.load(open(sys.argv[3])); control=c['Base']+'/control'
digest=lambda b:'sha256:'+hashlib.sha256(b).hexdigest()
encode=lambda v:json.dumps(v,separators=(',',':')).encode()
r=dict(Schema='private-connected-source-ready.v2',Invocation=c['Invocation'],ConfigPin=c['Pin'],SourceTemplatePin=digest(b'template'),ControlReceipt='fixed-provider-control/v2 /Users/schwa/.local/bin/gopls v0.23.0 '+digest(b'controlled-receipt'),LaunchDeadline=c['LaunchDeadline'])
raw=encode(r)+b'\\n'
fd=int(sys.argv[5]);os.write(fd,raw);os.close(fd)
end=time.monotonic()+5
while not os.path.exists(control+'/start.json') and time.monotonic()<end:time.sleep(.001)
assert os.path.exists(control+'/start.json'),'MANUAL_START_GAP'
p=open(control+'/parent-ready.json','rb').read();s=open(control+'/start.json','rb').read()
pv=json.loads(p);sv=json.loads(s)
assert pv==dict(Schema='private-parent-start-ready.v2',Invocation=c['Invocation'],DriverReadySHA256=digest(raw),ProducerSession=c['ProducerSession'],ReviewerSession=c['ReviewerSession'],ProducerWaiterActive=True)
assert sv==dict(Operation='START_CONFIRM',Invocation=c['Invocation'],ProducerWaiterActive=True,DriverReadySHA256=digest(raw),ParentReadySHA256=digest(p),SourceTemplatePin=r['SourceTemplatePin'])
print('CONTROLLED_TYPED_JOIN_PASS')
mailbox=c['Source']['Host']['Mailbox']['Directory']
while not os.path.exists(mailbox+'/response-000001.json') and time.monotonic()<end:time.sleep(.001)
assert os.path.exists(mailbox+'/response-000001.json'),'CONTROLLED_RESPONSE_REQUIRED'
''')
        tool.chmod(0o700)
        bridge = Path(h.__file__).parents[1] / '.pi/skills/private-host-streaming-application/scripts/activation-bridge.py'
        command = [h.sys.executable, str(bridge), 'source', str(tool), str(config),
                   h.digest(config.read_bytes()), str(capability), h.digest(capability.read_bytes())]
        with h.subprocess.Popen(command, stdout=h.subprocess.PIPE, stderr=h.subprocess.PIPE) as owner:
            try:
                entry = owner.stdout.readline()
                self.assertIn(b'SOURCE_HOST_ENTRY', entry, 'CONNECTED_START_WITHOUT_MANUAL_PARENT')
                if cancel:
                    owner.terminate()
                    owner.communicate(timeout=8)
                    self.assertNotEqual(owner.returncode, 0, 'CONNECTED_CANCEL_JOINED_NO_START')
                    receipt = h.json.loads((f['control'] / 'source-completion.json').read_bytes())
                    self.assertTrue(receipt['Joined'], 'CONNECTED_CANCEL_JOINED_NO_START')
                    self.assertEqual(receipt['GoConsumption'], 'UNOBSERVED')
                    self.assertEqual(receipt['StartPublication'], 'NOT_ATTEMPTED')
                    self.assertFalse((f['control'] / 'start.json').exists(), 'CONNECTED_CANCEL_JOINED_NO_START')
                    print('CONNECTED_CANCEL_JOINED_NO_START: A-pass actual signal, owned child joined, no START')
                    return
                handler = h.subprocess.run([h.sys.executable, h.__file__, 'connected-wait',
                    '--control', str(f['control']), '--capability', str(capability),
                    '--capability-sha256', h.digest(capability.read_bytes())], capture_output=True, timeout=8)
                self.assertEqual(handler.returncode, 0, handler.stderr)
                result = h.json.loads(handler.stdout)
                self.assertEqual(result, dict(Status='REQUEST', Binding=f['frame']['Binding']), 'CONNECTED_SOURCE_BINDING_ONLY')
                self.assertNotIn(b'SOURCE_ONLY controlled fixture', handler.stdout)
                payload = f['base'] / 'canned-response.json'
                self.put(payload, dict(Controlled=True))
                h.respond(str(f['mailbox']), 1, str(payload))
                out, err = owner.communicate(timeout=8)
                self.assertEqual(owner.returncode, 0, (out, err))
                self.assertIn(b'CONTROLLED_TYPED_JOIN_PASS', (f['control'] / 'source-output.log').read_bytes())
                self.assertTrue((f['control'] / 'parent-ready.json').exists())
                self.assertTrue((f['control'] / 'start.json').exists())
                print('CONNECTED_START_WITHOUT_MANUAL_PARENT: A-pass; CONNECTED_SOURCE_BINDING_ONLY: A-pass')
            finally:
                if owner.poll() is None:
                    owner.terminate()
                    owner.wait(timeout=8)

    def test_connected_source_deadline_not_startup_deadline(self):
        f = self.fixture()
        from datetime import datetime, timezone
        launch = datetime.now(timezone.utc)
        bridge = SimpleNamespace(source_deadline=lambda v: datetime.fromisoformat(v.replace('Z', '+00:00')).timestamp())
        cap = dict(LaunchDeadline=launch.isoformat(), SourceTotalMS=15000,
                   Mailbox=str(f['mailbox']), Invocation=f['prep']['Invocation'], RoleID='canned-source-role')
        with patch.object(h.time, 'time', return_value=launch.timestamp() + 1):
            self.assertEqual(h.wait_connected_source(str(f['control']), cap, bridge),
                             dict(Status='REQUEST', Binding=f['frame']['Binding']), 'CONNECTED_ORIGINAL_SOURCE_WAIT')
        with patch.object(h.time, 'time', return_value=launch.timestamp() + 15):
            self.assertEqual(h.wait_connected_source(str(f['control']), cap, bridge),
                             dict(Status='DEADLINE'), 'CONNECTED_ORIGINAL_SOURCE_WAIT')
        print('CONNECTED_ORIGINAL_SOURCE_WAIT: A-pass after startup deadline; fixed total equality refuses')

    def connected_receipt_fixture(self):
        # Source-free metadata from the failed actual Go startup; no semantic
        # payload, provider invocation, accounting history or live capability IO.
        raw = (b'{"Schema":"private-connected-source-ready.v2","Invocation":"host-assisted-full-definition-r-XoBFapmT",'
               b'"ConfigPin":"sha256:5a28c0e53f3f0917c15057ef6bbbd7fdf18a4f2de0faf2cd14b7905cfd41f30e",'
               b'"SourceTemplatePin":"sha256:4eb411fef4f6f110f57e69d6b40694978e92a3c3ae7584556a945dcfdb315f38",'
               b'"ControlReceipt":"fixed-provider-control/v2 /Users/schwa/.local/bin/gopls v0.23.0 sha256:003dbef7d03f6b5c96fc9721a759049be1e35d3b8638c2f87c634d8de531cf2d",'
               b'"LaunchDeadline":"2026-11-10T06:41:32.522208Z"}\n')
        path = Path(h.__file__).parents[1] / '.pi/skills/private-host-streaming-application/scripts/activation-bridge.py'
        spec = importlib.util.spec_from_file_location('receipt_bridge_test', path)
        bridge = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(bridge)
        ready = bridge.host.strict(raw)
        cap = {k: ready[k] for k in ('Invocation', 'ConfigPin', 'LaunchDeadline')}
        return bridge, raw, ready, cap

    def test_connected_actual_go_receipt(self):
        bridge, raw, ready, cap = self.connected_receipt_fixture()
        self.assertEqual(len(raw), 485)
        self.assertEqual(bridge.source_ready(raw, cap), ready, 'CONNECTED_GO_RECEIPT')
        print('CONNECTED_GO_RECEIPT: A-pass actual 485-byte LF packet')

    def test_connected_receipt_and_identity_refusals(self):
        bridge, raw, ready, cap = self.connected_receipt_fixture()
        receipt = ready['ControlReceipt']
        # The fixed producer's path is exact, not whitespace-tokenized. A path
        # containing spaces is foreign here, not a reason to parse a new format.
        cases = [("ControlReceipt", value) for value in (
            h.digest(b'bare'), '', None, 1, 'INVALID',
            receipt.replace('control/v2', 'control/v1'),
            receipt.replace('/Users/schwa/.local/bin/gopls', '/tmp/provider with spaces/gopls'),
            receipt.replace('v0.23.0', 'v0.24.0'),
            receipt.replace('sha256:', 'sha512:'),
            receipt[:-1], receipt + '0', receipt + '\n', receipt + ' trailing',
            receipt[:-64] + 'G' * 64,
        )]
        cases += [('Schema', 'foreign'), ('Invocation', 'foreign'),
                  ('ConfigPin', h.digest(b'foreign')), ('LaunchDeadline', '2099-01-01T00:00:00Z'),
                  ('SourceTemplatePin', receipt), ('SourceTemplatePin', None), ('Unknown', True)]
        for key, value in cases:
            with self.subTest(key=key, value=value):
                changed = dict(ready, **{key: value})
                with self.assertRaisesRegex(ValueError, 'SOURCE connected identity'):
                    bridge.source_ready(bridge.source_encoded(changed), cap)
        print('CONNECTED_RECEIPT_IDENTITY: A-pass malformed receipt, forged head shape, foreign identity and unknown key refused')

    def test_connected_ack_exact_ready_bytes(self):
        bridge, raw, ready, cap = self.connected_receipt_fixture()
        cap['Nonce'] = 'a' * 64
        pin = h.digest(bridge.source_encoded(cap))
        ack = dict(Schema='private-source-connected-ack.v1', CapabilitySHA256=pin,
                   DriverReadySHA256=h.digest(raw), Nonce=cap['Nonce'], ProducerWaiterActive=True)
        bridge.source_ack(bridge.source_encoded(ack), raw, cap, pin)
        for changed in (raw[:-1], raw + b' ', bridge.source_encoded(ready)):
            with self.subTest(changed=changed[-10:]):
                with self.assertRaisesRegex(ValueError, 'SOURCE handler identity'):
                    bridge.source_ack(bridge.source_encoded(ack), changed, cap, pin)
        print('CONNECTED_ACK_BYTES: A-pass original LF bound; missing LF or extra byte refused')

    def test_connected_start_admission_guards(self):
        bridge_path = Path(h.__file__).parents[1] / '.pi/skills/private-host-streaming-application/scripts/activation-bridge.py'
        spec = importlib.util.spec_from_file_location('connected_bridge_test', bridge_path)
        bridge = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(bridge)
        from datetime import datetime, timezone, timedelta
        for mode in ('valid', 'wrong-invocation', 'wrong-session', 'stale-epoch', 'wrong-capability',
                     'unready', 'numeric-ready', 'expired', 'equal', 'cancel', 'replay', 'late-connected-ready'):
            with self.subTest(mode=mode):
                f = self.fixture()
                deadline = datetime.now(timezone.utc) + timedelta(seconds=30)
                cap = dict(Invocation='controlled', ConfigPin=h.digest(b'config'),
                           ProducerSession='producer', ReviewerSession='reviewer', Nonce='a' * 64,
                           LaunchDeadline=deadline.isoformat(), ApproveParentStart=True)
                c = dict(Invocation=cap['Invocation'], ProducerSession='producer', ReviewerSession='reviewer')
                ready = dict(Schema='private-connected-source-ready.v2', Invocation=cap['Invocation'],
                             ConfigPin=cap['ConfigPin'], SourceTemplatePin=h.digest(b'template'),
                             ControlReceipt='fixed-provider-control/v2 /Users/schwa/.local/bin/gopls v0.23.0 ' + h.digest(b'controlled-epoch'), LaunchDeadline=cap['LaunchDeadline'])
                pin = h.digest(bridge.source_encoded(cap))
                raw = bridge.source_encoded(ready) + b'\n'
                ack = dict(Schema='private-source-connected-ack.v1', CapabilitySHA256=pin,
                           DriverReadySHA256=h.digest(raw), Nonce=cap['Nonce'], ProducerWaiterActive=True)
                if mode == 'wrong-invocation':
                    ready['Invocation'] = 'foreign'
                    raw = bridge.source_encoded(ready) + b'\n'
                    ack['DriverReadySHA256'] = h.digest(raw)
                elif mode == 'wrong-session':
                    # The capability digest is the complete identity envelope,
                    # including session labels. Reusing an ack after changing a
                    # session must fail even when all visible marker files exist.
                    cap['ProducerSession'] = 'foreign'
                    pin = h.digest(bridge.source_encoded(cap))
                elif mode == 'stale-epoch':
                    ack['DriverReadySHA256'] = h.digest(b'old connected ready')
                elif mode == 'wrong-capability':
                    ack['CapabilitySHA256'] = h.digest(b'other approval')
                elif mode == 'unready':
                    ack['ProducerWaiterActive'] = False
                elif mode == 'numeric-ready':
                    ack['ProducerWaiterActive'] = 1
                elif mode == 'cancel':
                    self.put(f['control'] / 'stop.json', dict(StopRequested=True))
                elif mode == 'replay':
                    self.put(f['control'] / 'start.json', dict(Immutable='old start'))
                before = (f['control'] / 'start.json').read_bytes() if mode == 'replay' else None
                now = deadline.timestamp() - 1
                if mode in ('expired', 'late-connected-ready'):
                    now += 2
                elif mode == 'equal':
                    now += 1
                fd = bridge.private_root(str(f['control']))
                try:
                    with patch.object(bridge.time, 'time', return_value=now):
                        call = lambda: bridge.source_parent_join(fd, c, cap, pin, raw, bridge.source_encoded(ack))
                        if mode == 'valid':
                            call()
                            self.assertTrue((f['control'] / 'start.json').exists(), 'CONNECTED_ADMISSION')
                        else:
                            with self.assertRaises((ValueError, FileExistsError), msg='CONNECTED_ADMISSION'):
                                call()
                            if mode == 'replay':
                                self.assertEqual((f['control'] / 'start.json').read_bytes(), before)
                            else:
                                self.assertFalse((f['control'] / 'start.json').exists(), 'CONNECTED_ADMISSION')
                    print('CONNECTED_ADMISSION: A-pass', mode)
                finally:
                    os.close(fd)

    def test_source_export_respond_exact(self):
        f = self.fixture()
        fd = h.directory(str(f['mailbox']))
        try:
            self.assertEqual(h.canonical(h.request(fd, 1)), f['raw'], 'SOURCE_REQUEST_EXACT')
        finally:
            os.close(fd)
        receipt = self.export(f)
        m = self.assert_export_exact(f, receipt)
        self.assertGreater(len(m['Parts']), 1)
        payload = b'\x00\xff exact canned bytes { broken JSON\r\n'
        path = f['base'] / 'payload'
        path.write_bytes(payload)
        path.chmod(0o600)
        h.respond(str(f['mailbox']), 1, str(path))
        response = h.decode((f['mailbox'] / 'response-000001.json').read_bytes())
        self.assertEqual(h.base64.b64decode(response['Payload']), payload, 'SOURCE_RESPONSE_EXACT')
        self.assertEqual(response['Binding'], f['frame']['Binding'])
        print('SOURCE_REQUEST_EXACT: A-pass; SOURCE_RESPONSE_EXACT: A-pass; SOURCE_EXPORT_ORDERED: A-pass')

    def test_source_respond_final_deadline(self):
        # Test-only clocks advance during real payload/frame work, after request()
        # has admitted the original deadline. No sleeps or production clock hooks.
        for stage in ('payload-read', 'canonical', 'receipt-hash'):
            for expire in (False, True):
                with self.subTest(stage=stage, expire=expire):
                    f = self.fixture()
                    deadline = h.datetime.fromisoformat(f['frame']['Binding']['Deadline'].replace('Z', '+00:00')).timestamp()
                    clock = [deadline - 1]
                    reached = []
                    payload = b'\x00\xff exact source bytes { invalid JSON\r\n'
                    path = f['base'] / 'payload'
                    path.write_bytes(payload)
                    path.chmod(0o600)
                    actual_read, actual_canonical, actual_digest = h.read, h.canonical, h.digest
                    def advance():
                        reached.append(stage)
                        if expire:
                            clock[0] = deadline  # equality must already be refused
                    def read(*args, **kwargs):
                        value = actual_read(*args, **kwargs)
                        if stage == 'payload-read' and args[1] == 'payload':
                            advance()
                        return value
                    def canonical(value):
                        raw = actual_canonical(value)
                        if stage == 'canonical' and isinstance(value, dict) and 'PayloadSHA256' in value:
                            advance()
                        return raw
                    def digest(raw):
                        value = actual_digest(raw)
                        if stage == 'receipt-hash' and raw.startswith(b'{"Binding":') and b'"PayloadByteLength"' in raw:
                            advance()
                        return value
                    refused, receipt = None, None
                    with patch.object(h.time, 'time', side_effect=lambda: clock[0]), patch.object(h, 'read', side_effect=read), patch.object(h, 'canonical', side_effect=canonical), patch.object(h, 'digest', side_effect=digest):
                        try:
                            receipt = h.respond(str(f['mailbox']), 1, str(path))
                        except ValueError as error:
                            refused = error
                    self.assertEqual(reached, [stage], 'ASSERT_SOURCE_RESPONSE_WORK_REACHED')
                    response_path = f['mailbox'] / 'response-000001.json'
                    if expire:
                        self.assertFalse(response_path.exists(), 'ASSERT_SOURCE_RESPONSE_LATE_WORK_ABSENT')
                        self.assertIsInstance(refused, ValueError, 'ASSERT_SOURCE_RESPONSE_LATE_WORK_REFUSED')
                        self.assertEqual(str(refused), 'expired handoff', 'ASSERT_SOURCE_RESPONSE_LATE_WORK_REFUSED')
                        self.assertIsNone(receipt, 'ASSERT_SOURCE_RESPONSE_LATE_WORK_REFUSED')
                        print(f'ASSERT_SOURCE_RESPONSE_LATE_WORK_ABSENT: PASS stage={stage}; original deadline equality refused')
                    else:
                        self.assertIsNone(refused, 'ASSERT_SOURCE_RESPONSE_LIVE_WORK_EXACT')
                        raw = response_path.read_bytes()
                        response = h.decode(raw)
                        self.assertEqual(h.base64.b64decode(response['Payload']), payload, 'ASSERT_SOURCE_RESPONSE_LIVE_WORK_EXACT')
                        self.assertEqual(response['Binding'], f['frame']['Binding'], 'ASSERT_SOURCE_RESPONSE_LIVE_WORK_EXACT')
                        self.assertEqual(receipt, dict(ResponseSHA256=h.digest(raw), ResponseByteLength=len(raw)), 'ASSERT_SOURCE_RESPONSE_LIVE_WORK_EXACT')
                        print(f'ASSERT_SOURCE_RESPONSE_LIVE_WORK_EXACT: PASS stage={stage}')

    def test_source_closed_family(self):
        mutations = {
            'legacy-label': lambda q: q['Binding'].update(Stage='INTERPRET'),
            'legacy-protocol': lambda q: q['Binding'].update(Protocol=h.PROTOCOL),
            'unknown': lambda q: q['Request'].update(Items=[]),
            'nonce': lambda q: q['Binding'].update(OwnerNonce='b' * 64),
            'role': lambda q: q['Binding'].update(RoleID='unused-reviewer'),
            'sequence': lambda q: q['Binding'].update(Sequence=True),
            'attempt': lambda q: q['Binding']['Source'].update(AttemptID='foreign'),
            'source': lambda q: q['Binding']['Source'].update(SourceDigest=h.digest(b'foreign')),
            'record': lambda q: q['Request'].update(record_id='f' * 64),
            'deadline': lambda q: q['Binding'].update(Deadline='2100-01-01T00:00:00Z'),
            'bool-limit': lambda q: q['Binding'].update(ResponseAllowance=True),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                f = self.fixture()
                frame = h.decode(f['raw'])
                mutate(frame)
                self.put(f['mailbox'] / 'request-000001.json', frame)
                fd = h.directory(str(f['mailbox']))
                try:
                    with self.assertRaises((ValueError, KeyError, TypeError), msg='SOURCE_CLOSED_FAMILY'):
                        h.request(fd, 1)
                finally:
                    os.close(fd)
        print('SOURCE_CLOSED_FAMILY: A-pass')

    def test_source_family_countercontrol(self):
        f = self.fixture()
        frame = h.decode(f['raw'])
        frame['Binding']['Stage'] = 'INTERPRET'
        self.put(f['mailbox'] / 'request-000001.json', frame)
        def guard():
            fd = h.directory(str(f['mailbox']))
            try:
                with self.assertRaises(ValueError, msg='SOURCE_FAMILY_GUARD'):
                    h.request(fd, 1)
            finally:
                os.close(fd)
        with patch.object(h, 'source_frame', return_value=None):
            with self.assertRaisesRegex(AssertionError, 'SOURCE_FAMILY_GUARD'):
                guard()
            print('SOURCE_FAMILY_GUARD: A-fail with source validator removed')
        guard()
        print('SOURCE_FAMILY_GUARD: A-pass restored')

    def test_source_wait_privacy_and_stop(self):
        f = self.fixture()
        self.put(f['control'] / 'start.json', dict(Schema='private.onefile-actual-start.v2',
                 ParentInvokedOperator=True, Invocation=f['prep']['Invocation'], ScopeSHA256=f['prep']['ScopeSHA256'],
                 FullPlanSHA256=h.digest(b'plan'), ActualRoleBindingsSHA256=h.digest(b'canned labels'),
                 StartedUnix=1000000000, HardDeadlineUnix=4102444800))
        result = h.wait_request(str(f['mailbox']), 'canned-source-role', str(f['control']), poll_ms=1)
        self.assertEqual(result, dict(Status='REQUEST', Binding=f['frame']['Binding']))
        self.assertNotIn(f['frame']['Request']['envelope']['prompt'], str(result))
        self.put(f['control'] / 'stop.json', dict(StopRequested=True))
        self.assertEqual(h.wait_request(str(f['mailbox']), 'canned-source-role', str(f['control'])), dict(Status='STOP'))
        print('SOURCE_WAIT_BINDING_ONLY: A-pass; SOURCE_STOP: A-pass')


if __name__ == '__main__':
    unittest.main()
