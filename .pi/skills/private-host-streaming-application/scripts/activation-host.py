#!/usr/bin/env python3
"""Foreground deterministic OS lifecycle owner; never launches roles or models.

The caller must keep stdin's private lifecycle pipe open, read one <=512-byte
stdout packet through EOF, and retain this foreground session until exit.
Closing lifecycle stdin cancels. It is NOT coordinator or role-response stdin.
No detach/daemon/restart mode exists. Standard library only, Darwin/Linux.
"""
import ctypes
import errno
import fcntl
import hashlib
import platform
import itertools
import json
import os
import re
import selectors
import signal
import stat
import subprocess
import sys
import time

DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
KEYS = {"Schema", "Event", "ApplicationSHA256", "RunID", "UnitID", "ActivationSHA256", "Activation"}


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def strict(raw):
    def pairs(values):
        result = {}
        for key, value in values:
            if key in result:
                raise ValueError("duplicate key")
            result[key] = value
        return result
    return json.loads(raw.decode("utf-8"), object_pairs_hook=pairs)


def held_directory(path):
    if not os.path.isabs(path) or os.path.normpath(path) != path:
        raise ValueError("canonical directory required")
    parts = path.split("/")[1:]
    if len(parts) > 64:
        raise ValueError("directory depth")
    fd = os.open("/", os.O_RDONLY | os.O_DIRECTORY)
    try:
        for part in parts:
            if not part:
                continue
            next_fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = next_fd
        return fd
    except BaseException:
        os.close(fd)
        raise


def private_read(path, limit):
    parent = held_directory(os.path.dirname(path))
    try:
        return private_read_at(parent, os.path.basename(path), limit)
    finally:
        os.close(parent)


def private_read_at(parent, name, limit):
    # All relative components, including objects/shard, are held/no-follow.
    parts = name.split("/")
    if len(parts) > 8 or any(p in ("", ".", "..") for p in parts):
        raise ValueError("relative metadata path")
    owned = []
    try:
        for part in parts[:-1]:
            parent = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
            owned.append(parent)
        return private_leaf(parent, parts[-1], limit)
    finally:
        for fd in reversed(owned):
            os.close(fd)


def private_leaf(parent, name, limit):
    fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
    try:
        before = os.fstat(fd)
        if not stat.S_ISREG(before.st_mode) or stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1 or before.st_uid != os.getuid() or not 0 < before.st_size <= limit:
            raise ValueError("private metadata")
        raw = os.read(fd, limit + 1)
        after = os.fstat(fd)
        visible = os.stat(name, dir_fd=parent, follow_symlinks=False)
        identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
        if len(raw) != before.st_size or identity(before) != identity(after) or identity(after) != identity(visible):
            raise ValueError("metadata changed")
        return raw
    finally:
        os.close(fd)


def validate(raw, request, run, app, root_fd=None):
    p = strict(raw)
    if len(raw) > 512 or set(p) != KEYS or p["Schema"] != "private-application-startup.v1" or p["Event"] != "ACTIVATION_READY" or p["ApplicationSHA256"] != app or p["RunID"] != run or type(p["Activation"]) is not int or not 1 <= p["Activation"] <= 16:
        raise ValueError("startup binding")
    if not all(isinstance(p[k], str) and DIGEST.fullmatch(p[k]) for k in ("UnitID", "ActivationSHA256")):
        raise ValueError("startup digest")
    root = request["Configuration"]["Store"]
    if root_fd is None:
        raise ValueError("held root required")
    visible = os.lstat(root)
    held = os.fstat(root_fd)
    if stat.S_ISLNK(visible.st_mode) or (visible.st_dev, visible.st_ino) != (held.st_dev, held.st_ino):
        raise ValueError("root identity changed")
    activation = private_read_at(root_fd, "activation-%02d.json" % p["Activation"], 4096)
    ac = strict(activation)
    mailbox = os.path.join(root, "units", p["UnitID"][7:], "cohort-mailbox")
    if digest(activation) != p["ActivationSHA256"] or set(ac) != {"Mailbox", "UnitID", "Base"} or ac["UnitID"] != p["UnitID"] or ac["Mailbox"] != mailbox or type(ac["Base"]) is not int or not 0 <= ac["Base"] <= 14:
        raise ValueError("activation binding")
    return p


def anonymous_pipe(fd, access):
    """Held-FD domain check, no pathname probes and one bounded fstatfs call.

    Darwin's 64-bit statfs ABI (sys/mount.h) is 2168 bytes. Anonymous
    pipes have device zero and fstatfs EINVAL; an unlinked vnode FIFO still
    has a filesystem. Linux LP64 statfs is 120 bytes and exposes PIPEFS_MAGIC.
    Other ABIs/platforms and every other query failure refuse, never guess.
    """
    st = os.fstat(fd)
    if not stat.S_ISFIFO(st.st_mode) or st.st_nlink != 0 or fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_ACCMODE != access:
        return False
    if ctypes.sizeof(ctypes.c_void_p) != 8:
        return False
    libc = ctypes.CDLL(None, use_errno=True)
    if sys.platform == "darwin" and platform.machine() in ("arm64", "x86_64"):
        # Aligned, exactly-sized storage for the verified 64-bit Darwin ABI.
        storage = (ctypes.c_uint64 * 271)()
        query = getattr(libc, "fstatfs$INODE64", None)
        if query is None:
            query = libc.fstatfs
    elif sys.platform == "linux" and platform.machine() in ("aarch64", "x86_64"):
        storage = (ctypes.c_long * 15)()
        query = libc.fstatfs
    else:
        return False
    query.argtypes = (ctypes.c_int, ctypes.c_void_p)
    query.restype = ctypes.c_int
    ctypes.set_errno(0)
    result = query(fd, ctypes.byref(storage))
    error = ctypes.get_errno()
    if sys.platform == "darwin":
        return result == -1 and error == errno.EINVAL and st.st_dev == 0
    return result == 0 and storage[0] == 0x50495045


def exclusive_stdio():
    """Bounded entry admission, before any metadata/log/child allocation.

    /dev/fd enumeration is native on the supported Darwin host. More than 256
    entries, unavailable enumeration, and all packet endpoint aliases refuse.
    close_fds in the bridge prevents inheritance; this public raw entry still
    independently rejects unsupported callers. No RLIMIT-sized scan is used.
    """
    try:
        packet = os.fstat(1)
        if not anonymous_pipe(0, os.O_RDONLY) or not anonymous_pipe(1, os.O_WRONLY):
            return False
        identity = lambda s: (s.st_dev, s.st_ino, stat.S_IFMT(s.st_mode))
        with os.scandir("/dev/fd") as directory:
            names = [entry.name for entry in itertools.islice(directory, 257)]
        if len(names) > 256:
            return False
        for name in names:
            if not name.isdecimal() or int(name) == 1:
                continue
            try:
                other = os.fstat(int(name))
            except OSError as error:
                if error.errno == 9:  # scandir's now-closed own descriptor
                    continue
                return False
            if identity(other) == identity(packet):
                return False
        return True
    except (OSError, AttributeError, TypeError, ValueError):
        return False


def main():
    # All outer work, including metadata validation and cleanup, shares this
    # ORIGINAL HOST watchdog. No fresh watchdog begins after packet delivery.
    started = time.monotonic()
    end = started + 270
    inherited_deadline = os.environ.get("LSP_HOST_ABSOLUTE_DEADLINE_NS")
    if inherited_deadline is not None:
        if not inherited_deadline.isdecimal() or len(inherited_deadline) > 20:
            return 125
        end = min(end, int(inherited_deadline) / 1e9)
    if not exclusive_stdio():
        return 125
    if len(sys.argv) != 6:
        return 125
    tool, request_path, run, app, logdir = sys.argv[1:]
    if not DIGEST.fullmatch(run) or not DIGEST.fullmatch(app):
        return 125
    os.umask(0o077)
    os.mkdir(logdir, 0o700)  # create-only, caller-nominated private log directory
    request = strict(private_read(request_path, 65536))
    config = request["Configuration"]
    root = config["Store"]
    root_fd = held_directory(root)
    root_stat = os.fstat(root_fd)
    if stat.S_IMODE(root_stat.st_mode) != 0o700 or root_stat.st_uid != os.getuid():
        return 125
    if config["ApprovalSHA256"] != app or strict(private_read_at(root_fd, "header.json", 65536)) != config:
        return 125
    ticket_raw = private_read_at(root_fd, "objects/" + run[7:9] + "/data-" + run + ".json", 65536)
    if digest(ticket_raw) != run or strict(ticket_raw)["ApplicationSHA256"] != app:
        return 125
    # Descriptor domain/access and packet aliases were admitted before side effects.
    os.set_blocking(1, False)
    os.set_blocking(0, False)
    wake_r, wake_w = os.pipe2(os.O_NONBLOCK | os.O_CLOEXEC) if hasattr(os, "pipe2") else os.pipe()
    os.set_blocking(wake_r, False)
    os.set_blocking(wake_w, False)
    state = {"cancel": False, "exited": False}
    def handled(sig, frame):
        state["exited" if sig == signal.SIGCHLD else "cancel"] = True
    previous_wakeup = signal.set_wakeup_fd(wake_w)
    for sig in (signal.SIGCHLD, signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, handled)
    reader, writer = os.pipe()
    child = None
    sel = selectors.DefaultSelector()
    log_fd = held_directory(logdir)
    def log_open(name):
        return os.fdopen(os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=log_fd), "wb", buffering=0)
    terminal = log_open("terminal.json")
    diagnostic = log_open("diagnostic.log")
    packet = bytearray()
    delivered = False
    reason = "UNKNOWN"
    code = 125
    try:
        # start_new_session provides an exact owned group. No poll()/wait() may
        # reap its leader before group cleanup: an unreaped PID cannot be reused.
        child = subprocess.Popen([tool, "private-host-application", "run", request_path, run, "--startup-notify-fd", str(writer)], stdin=subprocess.DEVNULL, stdout=terminal, stderr=diagnostic, pass_fds=(writer,), start_new_session=True)
        os.close(writer)
        writer = -1
        os.set_blocking(reader, False)
        sel.register(reader, selectors.EVENT_READ, "packet")
        sel.register(wake_r, selectors.EVENT_READ, "signal")
        sel.register(0, selectors.EVENT_READ, "lifecycle")
        while not state["cancel"] and time.monotonic() < end - 2:
            events = sel.select(max(0, end - 2 - time.monotonic()))
            for key, _ in events:
                if key.data == "signal":
                    try:
                        os.read(wake_r, 4096)
                    except BlockingIOError:
                        pass
                elif key.data == "lifecycle":
                    if os.read(0, 1) == b"":
                        state["cancel"] = True
                    else:
                        raise ValueError("lifecycle lease is not an input stream")
                else:
                    part = os.read(reader, 513 - len(packet))
                    packet.extend(part)
                    if len(packet) > 512:
                        raise ValueError("oversized packet")
                    if part == b"":
                        sel.unregister(reader)
                        os.close(reader)
                        reader = -1
                        if packet:
                            validate(bytes(packet), request, run, app, root_fd)
                            if time.monotonic() >= end - 2 or state["cancel"]:
                                raise ValueError("late packet")
                            if os.write(1, packet) != len(packet):
                                raise ValueError("short packet relay")
                            delivered = True
                        os.close(1)  # one bounded relay then EOF, never terminal stdout
                        reason = "ACTIVATION_READY" if delivered else "EOF_BEFORE_ACTIVATION"
            if state["exited"] and reader < 0:
                break
        if state["cancel"]:
            reason = "HOST_CANCELLED"
        elif time.monotonic() >= end - 2:
            reason = "HOST_WATCHDOG"
    except (OSError, ValueError, KeyError, TypeError):
        reason = "STARTUP_TRANSPORT_REFUSED"
    finally:
        if child is not None:
            # Signal only while the owned leader is unreaped. Never consult a
            # PID file or kill a numerical PID obtained from an earlier run.
            if not state["exited"]:
                try:
                    os.killpg(child.pid, signal.SIGTERM)
                except ProcessLookupError:
                    pass
                grace = min(end - 1, time.monotonic() + 1)
                # One bounded grace sleep, not a polling loop or new deadline.
                time.sleep(max(0, grace - time.monotonic()))
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            except PermissionError:
                # Darwin may refuse a group containing only its zombie leader.
                # This is not taken as cleanup success: join, then observe ESRCH
                # without ever signaling the now-reusable numerical identity.
                if not state["exited"]:
                    raise
            # After group kill the direct child is always joined. The syscall
            # itself is OS scheduling dependent, not a hard-kernel timing claim.
            child_code = child.wait()
            group_gone = False
            try:
                os.killpg(child.pid, 0)  # observation only, cannot kill a reused PID
            except ProcessLookupError:
                group_gone = True
            except PermissionError:
                pass
            code = child_code if child_code >= 0 else 128 - child_code
            if not group_gone:
                reason = "GROUP_CLEANUP_UNKNOWN"
                code = 125
            if reason in ("HOST_CANCELLED", "HOST_WATCHDOG", "STARTUP_TRANSPORT_REFUSED"):
                code = 125
        else:
            child_code = None
            group_gone = False
        for fd in (reader, writer):
            if fd >= 0:
                os.close(fd)
        sel.close()
        terminal.close()
        diagnostic.close()
        signal.set_wakeup_fd(previous_wakeup)
        os.close(wake_r)
        os.close(wake_w)
        receipt = {"Schema": "private-host-process-receipt.v1", "Status": reason, "CoordinatorExit": child_code, "WrapperExit": code, "ActivationRelayed": delivered, "Joined": child is not None, "GroupGoneObserved": group_gone, "ElapsedMS": int((time.monotonic() - started) * 1000)}
        with log_open("process.json") as f:
            f.write(json.dumps(receipt, separators=(",", ":")).encode("ascii"))
        os.close(log_fd)
        os.close(root_fd)
    return code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError) as error:
        # Fixed type/errno/line only; never raw metadata, paths or error strings.
        tb = error.__traceback__
        while tb.tb_next is not None:
            tb = tb.tb_next
        os.write(2, ("HOST_WRAPPER_ERROR type=%s errno=%s line=%d\n" % (type(error).__name__, getattr(error, "errno", 0), tb.tb_lineno)).encode("ascii"))
        sys.exit(125)
