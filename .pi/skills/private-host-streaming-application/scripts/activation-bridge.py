#!/usr/bin/env python3
"""Finite ordinary-HOST file bridge. No model, provider, service or SDK API.

prepare CONTROL TOOL REQUEST RUN APP starts the original 270s HOST clock.
owner CONTROL is ONE foreground command in the ordinary background OS-owner
Agent. await CONTROL is ONE finite parent command. cancel CONTROL sends one
bound CANCEL record. Only the parent launches/observes semantic Agents.
All paths and the executable are same-user, trusted, immutable inputs.
"""
import hashlib
import importlib.util
import json
import os
import selectors
import signal
import stat
import subprocess
import sys
import time
from datetime import datetime

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("activation_host", os.path.join(os.path.dirname(os.path.abspath(__file__)), "activation-host.py"))
host = importlib.util.module_from_spec(spec)
spec.loader.exec_module(host)
SCHEMA = "private-host-bridge.v1"


def encoded(value):
    raw = json.dumps(value, separators=(",", ":"), sort_keys=True).encode("utf-8")
    if len(raw) > 65536:
        raise ValueError("metadata bound")
    return raw


def file_digest(path):
    # The executable is an explicit trusted immutable input, not an arbitrary
    # packet path. A finite 128MiB cap bounds hashing outside owned-Go cost.
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        before = os.fstat(fd)
        if not stat.S_ISREG(before.st_mode) or before.st_size > 128 * 1024 * 1024:
            raise ValueError("executable bound")
        result = hashlib.sha256()
        left = before.st_size
        while left:
            raw = os.read(fd, min(65536, left))
            if not raw:
                raise ValueError("short executable")
            result.update(raw)
            left -= len(raw)
        identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
        if identity(os.fstat(fd)) != identity(before) or identity(os.stat(path, follow_symlinks=False)) != identity(before):
            raise ValueError("executable changed")
        return "sha256:" + result.hexdigest()
    finally:
        os.close(fd)


def private_root(path):
    fd = host.held_directory(path)
    s = os.fstat(fd)
    if stat.S_IMODE(s.st_mode) != 0o700 or s.st_uid != os.getuid():
        os.close(fd)
        raise ValueError("private root")
    return fd


def publish(fd, name, value):
    # No partial pathname is admitted: only the second hard-link is a commit.
    # Both fixed names remain, so the reader requires exactly these two links.
    raw = encoded(value)
    pending = name + ".pending"
    out = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    try:
        if os.write(out, raw) != len(raw):
            raise ValueError("short publication")
        os.fsync(out)
    finally:
        os.close(out)
    os.link(pending, name, src_dir_fd=fd, dst_dir_fd=fd, follow_symlinks=False)
    os.fsync(fd)


def committed(fd, name):
    pending = os.stat(name + ".pending", dir_fd=fd, follow_symlinks=False)
    ready = os.stat(name, dir_fd=fd, follow_symlinks=False)
    if (pending.st_dev, pending.st_ino) != (ready.st_dev, ready.st_ino) or ready.st_nlink != 2 or not stat.S_ISREG(ready.st_mode) or stat.S_IMODE(ready.st_mode) != 0o600 or ready.st_uid != os.getuid() or not 0 < ready.st_size <= 65536:
        raise ValueError("commit identity")
    source = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
    try:
        raw = os.read(source, 65537)
        after = os.fstat(source)
        if len(raw) != ready.st_size or (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns) != (ready.st_dev, ready.st_ino, ready.st_size, ready.st_mtime_ns, ready.st_ctime_ns):
            raise ValueError("commit changed")
        return host.strict(raw)
    finally:
        os.close(source)


def fifo(fd, name, flags):
    handle = os.open(name, flags | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
    s = os.fstat(handle)
    if not stat.S_ISFIFO(s.st_mode) or s.st_uid != os.getuid() or stat.S_IMODE(s.st_mode) != 0o600 or s.st_nlink != 1:
        os.close(handle)
        raise ValueError("private fifo")
    return handle


def binding(fd):
    b = host.strict(host.private_read_at(fd, "binding.json", 65536))
    if b["Schema"] != SCHEMA or not host.DIGEST.fullmatch(b["RunID"]) or not host.DIGEST.fullmatch(b["ApplicationSHA256"]) or type(b["DeadlineNS"]) is not int:
        raise ValueError("binding")
    return b


def prepare(control, tool, request, run, app):
    if not host.DIGEST.fullmatch(run) or not host.DIGEST.fullmatch(app):
        raise ValueError("identity")
    start = time.monotonic_ns()
    raw = host.private_read(request, 65536)
    r = host.strict(raw)
    root = private_root(r["Configuration"]["Store"])
    try:
        s = os.fstat(root)
        b = dict(Schema=SCHEMA, Tool=tool, ToolSHA256=file_digest(tool), Request=request,
                 RequestSHA256=host.digest(raw), RunID=run, ApplicationSHA256=app,
                 DeadlineNS=start + 270000000000, StoreDevice=s.st_dev, StoreInode=s.st_ino)
        if r["Configuration"]["ApprovalSHA256"] != app:
            raise ValueError("application")
        parent = host.held_directory(os.path.dirname(control))
        try:
            os.mkdir(os.path.basename(control), 0o700, dir_fd=parent)
        finally:
            os.close(parent)
        fd = private_root(control)
        try:
            out = os.open("binding.json", os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
            with os.fdopen(out, "wb") as f:
                f.write(encoded(b))
                f.flush()
                os.fsync(f.fileno())
            for name in ("cancel.fifo", "event.fifo"):
                os.mkfifo(name, 0o600, dir_fd=fd)
            os.fsync(fd)
        finally:
            os.close(fd)
        return b
    finally:
        os.close(root)


def owner(control, fd, b):
    end = b["DeadlineNS"] / 1e9
    if not 2 < end - time.monotonic() <= 270:
        raise ValueError("original deadline")
    raw = host.private_read(b["Request"], 65536)
    if host.digest(raw) != b["RequestSHA256"] or file_digest(b["Tool"]) != b["ToolSHA256"]:
        raise ValueError("input changed")
    request = host.strict(raw)
    root = private_root(request["Configuration"]["Store"])
    root_s = os.fstat(root)
    if (root_s.st_dev, root_s.st_ino) != (b["StoreDevice"], b["StoreInode"]):
        os.close(root)
        raise ValueError("root changed")
    # This single create-only claim prevents a second owner/replay.
    claim = os.open("owner.claim", os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    os.close(claim)
    cancel = fifo(fd, "cancel.fifo", os.O_RDONLY)
    event = fifo(fd, "event.fifo", os.O_RDWR)
    wake_r, wake_w = os.pipe()
    os.set_blocking(wake_r, False)
    os.set_blocking(wake_w, False)
    previous = signal.set_wakeup_fd(wake_w)
    stopped = [False]
    def handled(sig, frame):
        if sig != signal.SIGCHLD:
            stopped[0] = True
    for sig in (signal.SIGCHLD, signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, handled)
    diagnostic = os.fdopen(os.open("bridge-diagnostic.log", os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd), "wb", buffering=0)
    child = None
    packet = bytearray()
    relayed = False
    reason = "UNKNOWN"
    joined = False
    wrapper_exit = None
    try:
        env = dict(os.environ, LSP_HOST_ABSOLUTE_DEADLINE_NS=str(b["DeadlineNS"]))
        child = subprocess.Popen([sys.executable, os.path.join(os.path.dirname(os.path.abspath(__file__)), "activation-host.py"), b["Tool"], b["Request"], b["RunID"], b["ApplicationSHA256"], os.path.join(control, "wrapper")], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=diagnostic, close_fds=True, env=env)
        os.set_blocking(child.stdout.fileno(), False)
        with selectors.DefaultSelector() as sel:
            sel.register(child.stdout, selectors.EVENT_READ, "packet")
            sel.register(cancel, selectors.EVENT_READ, "cancel")
            sel.register(wake_r, selectors.EVENT_READ, "signal")
            while not stopped[0] and time.monotonic() < end - 2:
                for key, _ in sel.select(max(0, end - 2 - time.monotonic())):
                    if key.data == "cancel":
                        data = os.read(cancel, 513)
                        if data:
                            c = host.strict(data)
                            if len(data) > 512 or c != {"Schema": SCHEMA, "Event": "CANCEL", "RunID": b["RunID"], "ApplicationSHA256": b["ApplicationSHA256"]}:
                                raise ValueError("cancel binding")
                        reason = "HOST_CANCELLED"
                        stopped[0] = True
                    elif key.data == "signal":
                        os.read(wake_r, 4096)
                        # waitid WNOWAIT observes exit without reaping/reusing PID.
                        observed = os.waitid(os.P_PID, child.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
                        if observed is not None:
                            stopped[0] = True
                            reason = "WRAPPER_EXITED"
                    else:
                        data = os.read(child.stdout.fileno(), 513 - len(packet))
                        packet.extend(data)
                        if len(packet) > 512:
                            raise ValueError("oversized packet")
                        if not data:
                            sel.unregister(child.stdout)
                            if packet:
                                host.validate(bytes(packet), request, b["RunID"], b["ApplicationSHA256"], root)
                                if time.monotonic() >= end - 2:
                                    raise ValueError("late packet")
                                publish(fd, "startup.json", dict(Schema=SCHEMA, Event="ACTIVATION_READY", RunID=b["RunID"], ApplicationSHA256=b["ApplicationSHA256"], DeadlineNS=b["DeadlineNS"], RequestSHA256=b["RequestSHA256"], ToolSHA256=b["ToolSHA256"], Packet=host.strict(bytes(packet))))
                                relayed = True
                                os.write(event, b"R")
                if stopped[0]:
                    break
            if not stopped[0]:
                reason = "HOST_WATCHDOG"
    except (OSError, ValueError, KeyError, TypeError):
        reason = "BRIDGE_REFUSED"
    finally:
        if child is not None:
            child.stdin.close()  # sole lifecycle writer: normal cancellation
            try:
                # No reset: cleanup consumes only the original remaining time.
                wrapper_exit = child.wait(timeout=max(0.001, end - time.monotonic()))
                joined = True
            except subprocess.TimeoutExpired:
                # No detached replacement owner/watchdog. SIGKILL cannot run
                # finally; do not fabricate CLI/group cleanup in this case.
                child.kill()
                wrapper_exit = child.wait()
                reason = "WRAPPER_CLEANUP_UNKNOWN"
            child.stdout.close()
        result = dict(Schema=SCHEMA, Event="OWNER_COMMAND_FINISHED", RunID=b["RunID"], ApplicationSHA256=b["ApplicationSHA256"], Status=reason, WrapperExit=wrapper_exit, WrapperJoined=joined, ActivationCommitted=relayed, OwnerAgentReturn="UNOBSERVED")
        publish(fd, "completion.json", result)
        os.write(event, b"D")
        diagnostic.close()
        signal.set_wakeup_fd(previous)
        for handle in (cancel, event, wake_r, wake_w, root):
            os.close(handle)
    return result


def await_startup(fd, b):
    event = fifo(fd, "event.fifo", os.O_RDONLY)
    try:
        # Only the post-fsync event admits a committed path. A visible link
        # alone is not evidence that directory sync succeeded. If the owner
        # already returned and its FIFO event was lost, parent completion/result
        # observation is required; this helper never invents readiness.
        with selectors.DefaultSelector() as sel:
            sel.register(event, selectors.EVENT_READ)
            for _ in range(3):  # one READY and one DONE, no unbounded polling
                remaining = b["DeadlineNS"] / 1e9 - time.monotonic()
                if remaining <= 0 or not sel.select(remaining):
                    break
                notice = os.read(event, 2)
                if b"R" in notice:
                    return committed(fd, "startup.json")
                if b"D" in notice:
                    return committed(fd, "completion.json")
                if not notice:
                    return dict(Schema=SCHEMA, Event="OWNER_RETURN_REQUIRED", OwnerAgentReturn="UNOBSERVED")
        return dict(Schema=SCHEMA, Event="DEADLINE", OwnerAgentReturn="UNOBSERVED")
    finally:
        os.close(event)


# SOURCE startup uses the existing foreground/FIFO transport, not activation
# packets from the unrelated application family. Capability is explicit caller
# delegation, NOT model authentication or a grant. No executable hash or job IO.
def source_encoded(value):
    return json.dumps(value, ensure_ascii=False, allow_nan=False, separators=(',', ':')).replace('&', '\\u0026').replace('<', '\\u003c').replace('>', '\\u003e').replace('\u2028', '\\u2028').replace('\u2029', '\\u2029').encode()


def source_deadline(value):
    d = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if d.tzinfo is None:
        raise ValueError('SOURCE deadline timezone')
    return d.timestamp()


def source_capability(path, pin):
    raw = host.private_read(path, 4096)
    c = host.strict(raw)
    if (host.digest(raw) != pin or set(c) != {'Schema', 'ConfigSHA256', 'ConfigPin', 'Invocation',
            'ProducerSession', 'ReviewerSession', 'RoleID', 'Mailbox', 'Control', 'SourceTotalMS', 'Nonce', 'LaunchDeadline', 'ApproveParentStart'}
            or c['Schema'] != 'private-source-start-capability.v1' or c['ApproveParentStart'] is not True
            or not all(isinstance(c[k], str) and host.DIGEST.fullmatch(c[k]) for k in ('ConfigSHA256', 'ConfigPin'))
            or not isinstance(c['Nonce'], str) or not __import__('re').fullmatch('[0-9a-f]{64}', c['Nonce'])
            or any(not isinstance(c[k], str) or not 0 < len(c[k]) <= 256 for k in
                   ('Invocation', 'ProducerSession', 'ReviewerSession', 'RoleID'))
            or type(c['SourceTotalMS']) is not int or c['SourceTotalMS'] not in (15000, 95000)
            or c['ProducerSession'] == c['ReviewerSession']):
        raise ValueError('SOURCE startup capability')
    source_deadline(c['LaunchDeadline'])
    return c


def source_live(fd, capability):
    if time.time() >= source_deadline(capability['LaunchDeadline']):
        raise ValueError('SOURCE startup expired')
    for name in ('stop.json', 'actual-exit.json', 'source-completion.json'):
        try:
            os.stat(name, dir_fd=fd, follow_symlinks=False)
        except FileNotFoundError:
            continue
        raise ValueError('SOURCE startup cancelled')


def source_receive(fd, end):
    raw = bytearray()
    with selectors.DefaultSelector() as sel:
        sel.register(fd, selectors.EVENT_READ)
        for _ in range(513):
            left = end - time.monotonic()
            if left <= 0 or not sel.select(left):
                raise ValueError('SOURCE startup expired')
            part = os.read(fd, 513 - len(raw))
            if not part:
                raise ValueError('SOURCE startup EOF')
            raw.extend(part)
            if len(raw) > 512:
                raise ValueError('SOURCE startup size')
            if raw.endswith(b'\n'):
                return bytes(raw)
    raise ValueError('SOURCE startup bound')


def source_send(fd, raw):
    if len(raw) > 512 or os.write(fd, raw) != len(raw):
        raise ValueError('SOURCE startup short write')


def source_ready(raw, capability):
    r = host.strict(raw)
    if (set(r) != {'Schema', 'Invocation', 'ConfigPin', 'SourceTemplatePin', 'ControlReceipt', 'LaunchDeadline'}
            or r['Schema'] != 'private-connected-source-ready.v2'
            or any(r[k] != capability[k] for k in ('Invocation', 'ConfigPin', 'LaunchDeadline'))
            or not all(isinstance(r[k], str) and host.DIGEST.fullmatch(r[k]) for k in ('SourceTemplatePin', 'ControlReceipt'))):
        raise ValueError('SOURCE connected identity')
    return r


def source_ack(raw, ready_raw, capability, pin):
    expected = dict(Schema='private-source-connected-ack.v1', CapabilitySHA256=pin,
                    DriverReadySHA256=host.digest(ready_raw), Nonce=capability['Nonce'], ProducerWaiterActive=True)
    ack = host.strict(raw)
    if ack != expected or ack.get('ProducerWaiterActive') is not True:
        raise ValueError('SOURCE handler identity')


def source_put(fd, name, raw):
    # Existing helper publication convention: create-only, single-link final.
    pending = '.publish-' + name
    out = os.open(pending, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    try:
        if os.write(out, raw) != len(raw):
            raise ValueError('SOURCE metadata short write')
        os.fsync(out)
    finally:
        os.close(out)
    os.link(pending, name, src_dir_fd=fd, dst_dir_fd=fd, follow_symlinks=False)
    os.unlink(pending, dir_fd=fd)
    os.fsync(fd)


def source_parent_join(fd, c, capability, pin, ready_raw, ack_raw):
    source_live(fd, capability)
    r = source_ready(ready_raw, capability)
    source_ack(ack_raw, ready_raw, capability, pin)
    parent = source_encoded(dict(Schema='private-parent-start-ready.v2', Invocation=c['Invocation'],
        DriverReadySHA256=host.digest(ready_raw), ProducerSession=c['ProducerSession'],
        ReviewerSession=c['ReviewerSession'], ProducerWaiterActive=True))
    start = source_encoded(dict(Operation='START_CONFIRM', Invocation=c['Invocation'], ProducerWaiterActive=True,
        DriverReadySHA256=host.digest(ready_raw), ParentReadySHA256=host.digest(parent),
        SourceTemplatePin=r['SourceTemplatePin']))
    source_put(fd, 'parent-ready.json', parent)
    source_live(fd, capability)  # cancellation/expiry during publication cannot START
    source_put(fd, 'start.json', start)


def source_owner(tool, config_path, config_pin, cap_path, cap_pin):
    capability = source_capability(cap_path, cap_pin)
    raw = host.private_read(config_path, 65536)
    c = host.strict(raw)
    if (host.digest(raw) != config_pin or config_pin != capability['ConfigSHA256']
            or c['Pin'] != capability['ConfigPin'] or c['Schema'] != 'private-source-start.v2'
            or any(c[k] != capability[k] for k in ('Invocation', 'ProducerSession', 'ReviewerSession', 'LaunchDeadline'))
            or c['Source']['Host']['Mailbox']['InterpreterRole'] != capability['RoleID']
            or c['Source']['Host']['Mailbox']['Directory'] != capability['Mailbox']
            or os.path.join(c['Base'], 'control') != capability['Control']):
        raise ValueError('SOURCE config capability binding')
    timing = c['Source'].get('Timing')
    total = 15000 if timing is None else sum(timing[k] for k in ('CaptureMS', 'PreparationMS', 'HOSTMS', 'PublicationCleanupMS'))
    if total != capability['SourceTotalMS']:
        raise ValueError('SOURCE selected total binding')
    fd = private_root(os.path.join(c['Base'], 'control'))
    handles, child, logs = [], None, []
    previous = {}
    result = dict(Schema='private-source-owner-result.v1', StartPublication='NOT_ATTEMPTED',
                  GoConsumption='UNOBSERVED', Joined=False, Exit=None)
    try:
        source_live(fd, capability)
        end = time.monotonic() + source_deadline(capability['LaunchDeadline']) - time.time()
        claim = os.open('source-owner.claim', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
        os.close(claim)
        for name in ('source-ready.fifo', 'source-ack.fifo'):
            os.mkfifo(name, 0o600, dir_fd=fd)
            handles.append(fifo(fd, name, os.O_RDWR))
        reader, writer = os.pipe()
        handles.extend((reader, writer))
        def cancel(sig, frame):
            raise InterruptedError('SOURCE owner cancelled')
        for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            previous[sig] = signal.signal(sig, cancel)
        for name in ('source-output.log', 'source-diagnostic.log'):
            logs.append(os.fdopen(os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd), 'wb', buffering=0))
        # Caller supplies startup authority. No second model, SDK, source read,
        # executable digest, or new accounting owner is created by this bridge.
        child = subprocess.Popen([tool, 'private-host-application', 'start-source', config_path,
            '--startup-notify-fd', str(writer)], stdin=subprocess.DEVNULL, stdout=logs[0], stderr=logs[1],
            pass_fds=(writer,), close_fds=True, start_new_session=True)
        os.close(writer)
        handles.remove(writer)
        print(source_encoded(dict(Event='SOURCE_HOST_ENTRY', Control=os.path.join(c['Base'], 'control'))).decode(), flush=True)
        ready_raw = source_receive(reader, end)
        source_ready(ready_raw, capability)
        source_live(fd, capability)
        source_send(handles[0], ready_raw)
        ack = source_receive(handles[1], end)
        result['StartPublication'] = 'ATTEMPTED_UNCONFIRMED'
        source_parent_join(fd, c, capability, cap_pin, ready_raw, ack)
        result['StartPublication'] = 'PUBLISHED'
        source_send(handles[0], b'{"Event":"START_COMMITTED"}\n')
        # After START, the existing Go original SOURCE clock owns completion.
        # This foreground owner remains joined; no replacement watchdog starts.
        result['Exit'] = child.wait()
        result['Joined'] = True
        return result
    finally:
        # A second signal cannot interrupt the already-entered owned cleanup.
        # No reusable numerical PID is signalled after child.wait has reaped it.
        for sig in previous:
            signal.signal(sig, signal.SIG_IGN)
        try:
            if child is not None and not result['Joined']:
                try:
                    os.killpg(child.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                try:
                    result['Exit'] = child.wait(timeout=max(.001, end - time.monotonic()))
                except subprocess.TimeoutExpired:
                    os.killpg(child.pid, signal.SIGKILL)
                    result['Exit'] = child.wait()
                result['Joined'] = True
            if child is not None:
                source_put(fd, 'source-completion.json', source_encoded(result))
        finally:
            close_error = None
            for handle in handles + [fd]:
                try:
                    os.close(handle)
                except OSError as error:
                    close_error = error
            for log in logs:
                try:
                    log.close()
                except OSError as error:
                    close_error = error
            for sig, handler in previous.items():
                signal.signal(sig, handler)
            if close_error is not None:
                raise close_error


def source_handler(control, capability_path, capability_pin):
    capability = source_capability(capability_path, capability_pin)
    if control != capability['Control']:
        raise ValueError('SOURCE handler control binding')
    fd = private_root(control)
    ready, ack = None, None
    try:
        source_live(fd, capability)
        end = time.monotonic() + source_deadline(capability['LaunchDeadline']) - time.time()
        ready = fifo(fd, 'source-ready.fifo', os.O_RDONLY)
        ack = fifo(fd, 'source-ack.fifo', os.O_WRONLY)
        raw = source_receive(ready, end)
        source_ready(raw, capability)
        source_live(fd, capability)
        source_send(ack, source_encoded(dict(Schema='private-source-connected-ack.v1',
            CapabilitySHA256=capability_pin, DriverReadySHA256=host.digest(raw),
            Nonce=capability['Nonce'], ProducerWaiterActive=True)) + b'\n')
        if host.strict(source_receive(ready, end)) != {'Event': 'START_COMMITTED'}:
            raise ValueError('SOURCE start acknowledgement')
        return capability
    finally:
        for handle in (ready, ack, fd):
            if handle is not None:
                os.close(handle)


def main():
    os.umask(0o077)
    if len(sys.argv) == 7 and sys.argv[1] == 'source':
        result = source_owner(*sys.argv[2:])
        print(source_encoded(result).decode(), flush=True)
        if result['Exit'] != 0:
            sys.exit(125)
        return
    if len(sys.argv) == 7 and sys.argv[1] == "prepare":
        result = prepare(*sys.argv[2:])
    elif len(sys.argv) == 3:
        action, control = sys.argv[1:]
        fd = private_root(control)
        try:
            b = binding(fd)
            if action == "owner":
                result = owner(control, fd, b)
            elif action == "await":
                result = await_startup(fd, b)
            elif action == "cancel":
                cancel = fifo(fd, "cancel.fifo", os.O_WRONLY)
                try:
                    raw = encoded(dict(Schema=SCHEMA, Event="CANCEL", RunID=b["RunID"], ApplicationSHA256=b["ApplicationSHA256"]))
                    if os.write(cancel, raw) != len(raw):
                        raise ValueError("short cancel")
                finally:
                    os.close(cancel)
                result = dict(Schema=SCHEMA, Event="CANCEL_SENT", OwnerAgentReturn="UNOBSERVED")
            else:
                raise ValueError("action")
        finally:
            os.close(fd)
    else:
        raise ValueError("arguments")
    print(encoded(result).decode("utf-8"))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError):
        print('{"Schema":"private-host-bridge.v1","Event":"REFUSED","OwnerAgentReturn":"UNOBSERVED"}')
        sys.exit(125)
