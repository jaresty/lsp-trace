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
                req = dict(Stage='NAVIGATE')
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

if __name__ == '__main__':
    unittest.main()
