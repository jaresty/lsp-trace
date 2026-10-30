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
EXPORT_PART_BYTES = 20000
EXPORT_MAX_PARTS = (CAP + EXPORT_PART_BYTES - 4) // (EXPORT_PART_BYTES - 3)
EXPORT_MANIFEST_CAP = 256 << 10
EXPORT_RECEIPT_CAP = 8192

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

def request(fd, sequence, retain=False, cap=CAP):
    if not 1 <= sequence <= 4096:
        raise ValueError('sequence bound')
    prep = decode(read(fd, 'preparation.json'))
    raw = read(fd, f'request-{sequence:06d}.json', cap)
    frame = decode(raw)
    b = frame['Binding']
    if b['Protocol'] != PROTOCOL or any(b[k] != prep[k] for k in ('Invocation', 'OwnerNonce', 'PreparationSHA256')) or b['Sequence'] != sequence or digest(canonical(frame['Request'])) != b['RequestSHA256']:
        raise ValueError('request binding refused')
    if datetime.fromisoformat(b['Deadline'].replace('Z', '+00:00')).timestamp() <= time.time():
        raise ValueError('expired handoff')
    if 'terminal.json' in os.listdir(fd):
        raise ValueError('terminal handoff')
    return (frame, raw, prep) if retain else frame

def export_request(fd, mailbox, sequence, destination, config_path, config_pin, control):
    """Artifact affordance only. No inference, authority reconstruction or retry."""
    automation, protected, expected = export_config(config_path, config_pin)
    maximum = automation['Limits']['MaxRequestBytes']
    if not isinstance(maximum, int) or isinstance(maximum, bool) or not 1 <= maximum <= CAP:
        raise ValueError('export request ceiling')
    if automation['Directory'] != mailbox:
        raise ValueError('export mailbox config mismatch')
    # Retain the one exact validated read; never reopen the request to export it.
    frame, raw, prep = request(fd, sequence, retain=True, cap=maximum)
    b = frame['Binding']
    if b['Invocation'] != automation['Invocation'] or prep['Limits'] != automation['Limits']:
        raise ValueError('export config binding mismatch')
    if expected is not None and (prep.get('ScopeSHA256'), b['PreparationSHA256']) != expected:
        raise ValueError('Census export scope/preparation mismatch')
    prefix = b'{"Binding":' + canonical(b) + b',"Request":'
    if list(frame) != ['Binding', 'Request'] or not raw.startswith(prefix):
        raise ValueError('export frame shape')
    request_raw = raw[len(prefix):-1]
    if digest(request_raw) != b['RequestSHA256']:
        raise ValueError('export exact request mismatch')
    # Canonical absolute no-follow paths have no lexical aliases. Roots are
    # limited to this supplied pinned config/control, not omniscient discovery.
    if not isinstance(destination, str) or len(destination.encode()) > 2048 or os.path.normpath(destination) != destination:
        raise ValueError('export path bound')
    roots = [mailbox, control, *protected, config_path]
    for root in roots:
        if not os.path.isabs(root) or os.path.normpath(root) != root:
            raise ValueError('export root shape')
        if os.path.commonpath((root, destination)) in (root, destination):
            raise ValueError('export overlaps supplied roots')
    control_fd = directory(control)
    parent_fd = None
    export_fd = None
    try:
        def fresh():
            if datetime.fromisoformat(b['Deadline'].replace('Z', '+00:00')).timestamp() <= time.time():
                raise ValueError('expired handoff')
            if any(n in os.listdir(fd) for n in ('terminal.json', f'response-{sequence:06d}.json')) or 'stop.json' in os.listdir(control_fd):
                raise ValueError('export closed/cancelled handoff')
        fresh()
        parent_fd = directory(os.path.dirname(destination))
        # Existing directory aliases (e.g. case-insensitive spellings) must not
        # evade lexical overlap checks. Observe identities, never source contents.
        ancestors = set()
        ancestor_fd = os.dup(parent_fd)
        try:
            for _ in range(2049):  # destination is at most 2048 UTF-8 bytes
                info = os.fstat(ancestor_fd)
                identity = (info.st_dev, info.st_ino)
                if identity in ancestors:
                    break
                ancestors.add(identity)
                next_fd = os.open('..', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=ancestor_fd)
                os.close(ancestor_fd)
                ancestor_fd = next_fd
            else:
                raise ValueError('export ancestor ceiling')
        finally:
            os.close(ancestor_fd)
        for root in roots:
            try:
                info = os.stat(root, follow_symlinks=False)
            except FileNotFoundError:
                continue
            if stat.S_ISLNK(info.st_mode) or (stat.S_ISDIR(info.st_mode) and (info.st_dev, info.st_ino) in ancestors):
                raise ValueError('export aliases supplied root')
        name = os.path.basename(destination)
        # mkdir is the exclusive attempt claim. Keep partial files on any error;
        # the existing directory then refuses every repeated export attempt.
        os.mkdir(name, mode=0o700, dir_fd=parent_fd)
        export_fd = directory(destination)
        os.fsync(parent_fd)
        def put(name, content):
            fresh()
            publish(export_fd, name, content)
        put('frame.json', raw)
        put('request.json', request_raw)
        parts = []
        start = 0
        # Slice bytes, backing up at most three UTF-8 continuation bytes.
        # No decoding/re-encoding of the evidence or newline/Unicode changes.
        raw.decode('utf-8', errors='strict')
        while start < len(raw):
            end = min(start + EXPORT_PART_BYTES, len(raw))
            while end < len(raw) and raw[end] & 0xc0 == 0x80:
                end -= 1
            part = raw[start:end]
            name = f'part-{len(parts) + 1:06d}.txt'
            parts.append(dict(Name=name, Start=start, End=end, ByteLength=len(part), SHA256=digest(part)))
            if len(parts) > EXPORT_MAX_PARTS:
                raise ValueError('export part count')
            put(name, part)
            start = end
        manifest = dict(Schema='private-v2-host-export.v1', Binding=b,
                        Frame=dict(Name='frame.json', SHA256=digest(raw), ByteLength=len(raw)),
                        Request=dict(Name='request.json', SHA256=digest(request_raw), ByteLength=len(request_raw), Start=len(prefix), End=len(raw) - 1),
                        PartBytes=EXPORT_PART_BYTES, Parts=parts,
                        Provenance='Exact validated local frame bytes; caller-asserted source provenance; no provider-byte custody or execution authority')
        encoded = canonical(manifest)
        if len(encoded) > EXPORT_MANIFEST_CAP:
            raise ValueError('export manifest ceiling')
        receipt = dict(Schema='private-v2-host-export-receipt.v1', Directory=destination,
                       Manifest='manifest.json', ManifestSHA256=digest(encoded), ManifestByteLength=len(encoded),
                       FrameSHA256=digest(raw), FrameByteLength=len(raw), RequestSHA256=b['RequestSHA256'],
                       RequestByteLength=len(request_raw), PartCount=len(parts), Sequence=sequence,
                       Deadline=b['Deadline'])
        if len(canonical(receipt)) > EXPORT_RECEIPT_CAP:
            raise ValueError('export receipt ceiling')
        # Last publication is the only complete manifest; no partial manifest.
        put('manifest.json', encoded)
        fresh()
        return receipt
    finally:
        for opened in (export_fd, parent_fd, control_fd):
            if opened is not None:
                os.close(opened)

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

def pinned_config(path, pin):
    fd = directory(os.path.dirname(path))
    try:
        raw = read(fd, os.path.basename(path), 64 << 10)
    finally:
        os.close(fd)
    if digest(raw) != pin:
        raise ValueError('reviewed config digest mismatch')
    return decode(raw)

def legacy_config(c):
    if c.get('schema') != 'private-variable-scope-v2-caller.v4' or c.get('purpose') != 'PRIVATE_VARIABLE_SCOPE_V2_AUTOMATIC_PROVISIONAL':
        raise ValueError('only local handoff v4; v3 retired')
    return c

def config(path, pin):
    # Launch/supervise remain v4-only. Census export metadata is never relabelled
    # as a worker config or used to recreate an acquisition/preparation owner.
    return legacy_config(pinned_config(path, pin))

def export_config(path, pin):
    c = pinned_config(path, pin)
    if c.get('Schema') == 'private-census-inventory-host-export.v1':
        if (c.get('Profile') != 'PRIVATE_CENSUS_AST_V2' or c.get('Protocol') != PROTOCOL
                or c.get('Authority') != 0 or isinstance(c.get('Authority'), bool)
                or c.get('Accepted') is not False or c.get('Qualified') is not False
                or c.get('FeatureIdentity') != 'UNRESOLVED' or c.get('SourceCompleteness') != 'UNKNOWN'
                or c.get('AbsenceInference') != 'PROHIBITED'
                or c['Automation']['ScopeSHA256'] != c['ScopeSHA256']
                or not isinstance(c.get('Roots'), list) or not 2 <= len(c['Roots']) <= 16):
            raise ValueError('private Census export family/ceiling refused')
        return c['Automation'], c['Roots'], (c['ScopeSHA256'], c['PreparationSHA256'])
    c = legacy_config(c)
    roots = [c['code_root'], c['scope']['root'], c['operation_directory'],
             *c['producer_outputs'], *c['post_outputs'], *c['protected_roots'],
             c['report_path'], c['attempt_marker_path']]
    return c['automation'], roots, None

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
        if name == 'read':
            s.add_argument('--export-dir')
            s.add_argument('--config')
            s.add_argument('--config-sha256')
            s.add_argument('--control')
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
                if a.export_dir:
                    if not all((a.config, a.config_sha256, a.control)):
                        raise ValueError('export requires pinned config and control')
                    try:
                        result = export_request(fd, a.mailbox, a.sequence, a.export_dir, a.config, a.config_sha256, a.control)
                    except (OSError, ValueError, KeyError, TypeError):
                        # Parser/date errors may include untrusted request text.
                        # Never print it or publish a completed export receipt.
                        raise ValueError('export refused; preserve diagnostics; do not retry') from None
                else:
                    if any((a.config, a.config_sha256, a.control)):
                        raise ValueError('export options require export directory')
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
