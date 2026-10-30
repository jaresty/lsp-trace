#!/usr/bin/env python3
"""Host-owned PRIVATE_V2_AST file driver. No model client or harness.

The host, not this helper, invokes isolated interpreter/reviewer roles. Payloads
are exact files; respond only frames them. Approval is caller asserted.
"""
import argparse
import base64
import hashlib
import json
import os
import signal
import stat
import subprocess
import sys
import tempfile
import time
from datetime import datetime

PROTOCOL = 'PRIVATE_V2_AST_LOCAL_HANDOFF.v1'
CAP = 16 << 20

class IntegerLexeme(int):
    def __new__(cls, raw):
        value = int.__new__(cls, raw)
        value.raw = raw
        return value

class FloatLexeme:
    def __init__(self, raw):
        self.raw = raw

def canonical(value):
    # Preserve exact Go-emitted number lexemes, including exponent spelling and
    # negative zero in nested packet evidence. No evidence normalization.
    def encode(item):
        if isinstance(item, (IntegerLexeme, FloatLexeme)):
            return item.raw
        if isinstance(item, dict):
            return '{' + ','.join(encode(k) + ':' + encode(v) for k, v in item.items()) + '}'
        if isinstance(item, list):
            return '[' + ','.join(encode(v) for v in item) + ']'
        return json.dumps(item, ensure_ascii=False, allow_nan=False, separators=(',', ':'))
    return encode(value).replace('&', '\\u0026').replace('<', '\\u003c').replace('>', '\\u003e').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()

def digest(raw):
    return 'sha256:' + hashlib.sha256(raw).hexdigest()

def directory(path):
    if not os.path.isabs(path) or os.path.normpath(path) != path or path.startswith(('/tmp/', '/private/tmp/')):
        raise ValueError('canonical private outside-tmp directory required')
    fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY)
    try:
        for part in path.split('/')[1:]:
            new = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = new
        info = os.fstat(fd)
        if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
            raise ValueError('private current-owner directory required')
        return fd
    except BaseException:
        os.close(fd)
        raise

def read(fd, name, cap=CAP, executable=False, retain=False):
    if os.path.basename(name) != name:
        raise ValueError('direct child required')
    # Link-based no-replace publication is ready only after temporary unlink.
    # Bound this wait and never open/read the two-link prospective file.
    deadline = time.monotonic() + 0.1
    while True:
        before = os.stat(name, dir_fd=fd, follow_symlinks=False)
        try:
            temp = os.stat('.publish-' + name, dir_fd=fd, follow_symlinks=False)
        except FileNotFoundError:
            temp = None
        if temp is None or (before.st_dev, before.st_ino) != (temp.st_dev, temp.st_ino):
            break
        if time.monotonic() >= deadline:
            raise ValueError('unfinished publication')
        time.sleep(0.001)
    f = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
    retained = False
    try:
        opened = os.fstat(f)
        if (opened.st_dev, opened.st_ino) != (before.st_dev, before.st_ino):
            raise ValueError('file substituted before read')
        before = opened
        if not stat.S_ISREG(before.st_mode) or before.st_uid != os.getuid() or before.st_nlink != 1 or stat.S_IMODE(before.st_mode) != (0o700 if executable else 0o600):
            raise ValueError('private single-link regular file required')
        raw = bytearray()
        while len(raw) <= cap:
            block = os.read(f, min(65536, cap + 1 - len(raw)))
            if not block:
                break
            raw.extend(block)
        after = os.stat(name, dir_fd=fd, follow_symlinks=False)
        if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns) or os.fstat(f).st_ino != before.st_ino or len(raw) != before.st_size or not raw or len(raw) > cap:
            raise ValueError('partial/changed/over-limit file')
        if retain:
            retained = True
            return bytes(raw), f
        return bytes(raw)
    finally:
        if not retained:
            os.close(f)

def publish(fd, name, raw):
    if os.path.basename(name) != name or not raw or len(raw) > CAP:
        raise ValueError('publication bound/selector')
    temp = '.publish-' + name
    f = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    try:
        with os.fdopen(f, 'wb') as out:
            out.write(raw)
            out.flush()
            os.fsync(out.fileno())
        os.link(temp, name, src_dir_fd=fd, dst_dir_fd=fd, follow_symlinks=False)
    finally:
        os.unlink(temp, dir_fd=fd)
    os.fsync(fd)

def decode(raw):
    value = json.loads(raw, parse_int=IntegerLexeme, parse_float=FloatLexeme)
    if canonical(value) != raw:
        raise ValueError('canonical JSON required')
    return value

def request(fd, sequence):
    if not 1 <= sequence <= 4096:
        raise ValueError('sequence bound')
    prep = decode(read(fd, 'preparation.json'))
    frame = decode(read(fd, f'request-{sequence:06d}.json'))
    b = frame['Binding']
    if b['Protocol'] != PROTOCOL or any(b[k] != prep[k] for k in ('Invocation', 'OwnerNonce', 'PreparationSHA256')) or b['Sequence'] != sequence or digest(canonical(frame['Request'])) != b['RequestSHA256']:
        raise ValueError('request binding refused')
    if datetime.fromisoformat(b['Deadline'].replace('Z', '+00:00')).timestamp() <= time.time():
        raise ValueError('expired handoff')
    if 'terminal.json' in os.listdir(fd):
        raise ValueError('terminal handoff')
    return frame

def respond(mailbox, sequence, payload_path):
    fd = directory(mailbox)
    try:
        frame = request(fd, sequence)
        pfd = directory(os.path.dirname(payload_path))
        try:
            payload = read(pfd, os.path.basename(payload_path), 1 << 20)
        finally:
            os.close(pfd)
        # Do not decode, normalize, repair, or silently replace model bytes.
        result = dict(Binding=frame['Binding'], PayloadSHA256=digest(payload), PayloadByteLength=len(payload), Payload=base64.b64encode(payload).decode())
        raw = canonical(result)
        if len(raw) > frame['Binding']['ResponseAllowance']:
            raise ValueError('response frame exceeds allowance')
        publish(fd, f'response-{sequence:06d}.json', raw)
        return {'ResponseSHA256': digest(raw), 'ResponseByteLength': len(raw)}
    finally:
        os.close(fd)

def config(path, pin):
    fd = directory(os.path.dirname(path))
    try:
        raw = read(fd, os.path.basename(path), 64 << 10)
    finally:
        os.close(fd)
    if digest(raw) != pin:
        raise ValueError('reviewed config digest mismatch')
    c = decode(raw)
    if c['schema'] != 'private-variable-scope-v2-caller.v4' or c['purpose'] != 'PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL':
        raise ValueError('only local handoff v4; v3 retired')
    return c

def launch(args):
    c = config(args.config, args.config_sha256)
    fd = directory(args.control)
    try:
        if os.listdir(fd):
            raise ValueError('fresh empty control directory required')
        # Control artifacts must not pollute any worker input/output root.
        roots = [c['code_root'], c['scope']['root']]
        roots += [c['operation_directory'], *c['producer_outputs'], *c['post_outputs'], *c['protected_roots'], c['automation']['Directory']]
        for root in roots:
            if os.path.commonpath((root, args.control)) in (root, args.control):
                raise ValueError('control overlaps worker roots')
        worker_fd = directory(os.path.dirname(args.worker))
        try:
            binary = read(worker_fd, os.path.basename(args.worker), 64 << 20, executable=True)
        finally:
            os.close(worker_fd)
        if digest(binary) != args.worker_sha256:
            raise ValueError('reviewed worker digest mismatch')
        publish(fd, 'launch.json', canonical(dict(ConfigSHA256=args.config_sha256, WorkerSHA256=args.worker_sha256, Invocation=c['automation']['Invocation'])))
        command = [sys.executable, os.path.abspath(__file__), '_supervise', '--config', args.config, '--config-sha256', args.config_sha256, '--worker', args.worker, '--worker-sha256', args.worker_sha256, '--control', args.control]
        child = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True, close_fds=True, env={'PATH': '/usr/bin:/bin'})
        publish(fd, 'supervisor.json', canonical(dict(PID=child.pid, Invocation=c['automation']['Invocation'])))
        return {'SupervisorPID': child.pid, 'Control': args.control}
    finally:
        os.close(fd)

def supervise(args):
    c = config(args.config, args.config_sha256)
    fd = directory(args.control)
    # No ambient environment is read or retained. This is the existing entry's
    # explicit sentinel/config, not model settings or acquisition environment.
    env = {'PATH': '/usr/bin:/bin', 'GOWORK': 'off', 'GOENV': 'off', 'GOFLAGS': '', 'GO111MODULE': 'on', 'LSP_TRACE_PRIVATE_V2_CALLER': 'execute-once', 'LSP_TRACE_PRIVATE_V2_CALLER_CONFIG': args.config, 'LSP_TRACE_PRIVATE_V2_CALLER_CONFIG_SHA256': args.config_sha256}
    worker = None
    snapshot_directory = None
    try:
        worker_fd = directory(os.path.dirname(args.worker))
        executable_fd = None
        try:
            binary, executable_fd = read(worker_fd, os.path.basename(args.worker), 64 << 20, executable=True, retain=True)
            if digest(binary) != args.worker_sha256:
                raise ValueError('worker changed before launch')
            # Darwin refuses exec of /dev/fd (including shebangs). Materialize
            # only the already verified bytes into a fresh private snapshot;
            # never reopen args.worker or fall back to it after a launch error.
            if sys.platform == 'darwin':
                snapshot_directory = tempfile.mkdtemp(prefix='.exec-', dir=args.control)
                executable = os.path.join(snapshot_directory, 'worker')
                snapshot_fd = os.open(executable, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
                try:
                    view = memoryview(binary)
                    while view:
                        n = os.write(snapshot_fd, view)
                        if n <= 0:
                            raise ValueError('partial worker snapshot')
                        view = view[n:]
                    os.fsync(snapshot_fd)
                    os.fchmod(snapshot_fd, 0o700)
                    os.lseek(snapshot_fd, 0, os.SEEK_SET)
                    copied = bytearray()
                    while len(copied) <= 64 << 20:
                        block = os.read(snapshot_fd, 65536)
                        if not block:
                            break
                        copied.extend(block)
                    info = os.fstat(snapshot_fd)
                    if digest(copied) != args.worker_sha256 or info.st_nlink != 1 or not os.path.samestat(info, os.stat(executable, follow_symlinks=False)):
                        raise ValueError('worker snapshot mismatch')
                finally:
                    os.close(snapshot_fd)
                inherited = ()
            elif sys.platform == 'linux':
                os.lseek(executable_fd, 0, os.SEEK_SET)
                executable = '/dev/fd/' + str(executable_fd)
                probe_fd = os.open(executable, os.O_RDONLY)
                try:
                    if not os.path.samestat(os.fstat(executable_fd), os.fstat(probe_fd)):
                        raise ValueError('descriptor execution unavailable')
                finally:
                    os.close(probe_fd)
                inherited = (executable_fd,)
            else:
                raise ValueError('verified object execution unsupported')
            worker = subprocess.Popen([executable, '-test.run=^TestPrivateVariableScopeRealV2$', '-test.count=1', '-test.v'], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, env=env, start_new_session=True, close_fds=True, pass_fds=inherited)
        finally:
            if executable_fd is not None:
                os.close(executable_fd)
            os.close(worker_fd)
        publish(fd, 'worker.json', canonical(dict(PID=worker.pid, Invocation=c['automation']['Invocation'])))
        os.set_blocking(worker.stdout.fileno(), False)
        output = bytearray()
        deadline = time.monotonic() + c['operation_timeout_ms'] / 1000 + 5
        stopped = False
        while worker.poll() is None:
            try:
                chunk = os.read(worker.stdout.fileno(), 65536)
            except BlockingIOError:
                chunk = b''
            output.extend(chunk[:max(0, (1 << 20) - len(output))])
            if 'stop.json' in os.listdir(fd) or time.monotonic() >= deadline or len(output) >= 1 << 20:
                stopped = True
                os.killpg(worker.pid, signal.SIGTERM)
                try:
                    worker.wait(timeout=c['shutdown_timeout_ms'] / 1000 + 2)
                except subprocess.TimeoutExpired:
                    os.killpg(worker.pid, signal.SIGKILL)
                break
            time.sleep(0.01)
        code = worker.wait(timeout=3)
        while len(output) < 1 << 20:
            chunk = os.read(worker.stdout.fileno(), min(65536, (1 << 20) - len(output)))
            if not chunk:
                break
            output.extend(chunk)
        if output:
            publish(fd, 'worker-output.txt', bytes(output))
        publish(fd, 'exit.json', canonical(dict(ExitCode=code, Stopped=stopped, Joined=True)))
    finally:
        if worker is not None:
            if worker.poll() is None:
                os.killpg(worker.pid, signal.SIGKILL)
                worker.wait()
            worker.stdout.close()
        if snapshot_directory is not None:
            try:
                os.unlink(os.path.join(snapshot_directory, 'worker'))
            finally:
                os.rmdir(snapshot_directory)
        os.close(fd)

def main():
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest='action', required=True)
    for name in ('launch', '_supervise'):
        s = sub.add_parser(name)
        for key in ('config', 'config-sha256', 'worker', 'worker-sha256', 'control'):
            s.add_argument('--' + key, required=True)
    for name in ('read', 'respond', 'status'):
        s = sub.add_parser(name)
        s.add_argument('--mailbox', required=True)
        if name != 'status':
            s.add_argument('--sequence', type=int, required=True)
        if name == 'respond':
            s.add_argument('--payload', required=True)
    s = sub.add_parser('stop')
    s.add_argument('--control', required=True)
    a = p.parse_args()
    if a.action == 'launch':
        result = launch(a)
    elif a.action == '_supervise':
        supervise(a)
        return
    elif a.action == 'respond':
        result = respond(a.mailbox, a.sequence, a.payload)
    elif a.action == 'stop':
        fd = directory(a.control)
        try:
            publish(fd, 'stop.json', canonical({'StopRequested': True}))
        finally:
            os.close(fd)
        result = {'StopRequested': True}
    else:
        fd = directory(a.mailbox)
        try:
            if a.action == 'read':
                result = request(fd, a.sequence)
            else:
                names = os.listdir(fd)
                result = {'Terminal': decode(read(fd, 'terminal.json')) if 'terminal.json' in names else None, 'Requests': sorted(n for n in names if n.startswith('request-')), 'Responses': sorted(n for n in names if n.startswith('response-'))}
        finally:
            os.close(fd)
    print(canonical(result).decode())

if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError) as error:
        # Only our constant validation reasons are safe; OS messages may include paths.
        reason = str(error) if isinstance(error, ValueError) else type(error).__name__
        print('private handoff refused: ' + reason, file=sys.stderr)
        sys.exit(1)
