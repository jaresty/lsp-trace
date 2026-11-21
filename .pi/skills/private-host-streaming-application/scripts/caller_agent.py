"""Caller-side application-inventory V3 choreography; no model or process runtime.

The host supplies actual observed capabilities/identities and ordinary tool
callbacks. Frames must first pass the existing Go bundle/receipt verifier;
this module's checks are additional invocation/order checks, NOT custody.
A service instance is invocation-local and must never be resumed or replayed.
"""
from dataclasses import dataclass
import json
import time


class UnsupportedHost(ValueError):
    pass


@dataclass(frozen=True)
class Capabilities:
    caller: bool
    owned_concurrent_process: bool
    role_launch: bool = False
    role_completion: bool = False
    role_result: bool = False


def select_mode(capabilities):
    if not capabilities.caller:
        raise UnsupportedHost("CALLER_AGENT_REQUIRED: invoke from an existing agent, not a bare shell")
    if not capabilities.owned_concurrent_process:
        raise UnsupportedHost("OWNED_PROCESS_REQUIRED: host must retain/cancel/join the foreground coordinator while servicing frames")
    if capabilities.role_launch and capabilities.role_completion and capabilities.role_result:
        return "SUBAGENTS"
    return "SAME_THREAD"


def bind_actors(mode, caller_identity, role_identities=None):
    def valid(value):
        return isinstance(value, str) and 0 < len(value) <= 256
    if mode == "SAME_THREAD" and valid(caller_identity):
        if role_identities is not None:
            raise ValueError("fallback cannot relabel actors")
        return {"interpreter": caller_identity, "reviewer": caller_identity}
    if mode == "SUBAGENTS" and isinstance(role_identities, dict):
        if (set(role_identities) == {"interpreter", "reviewer"}
                and all(valid(v) for v in role_identities.values())
                and role_identities["interpreter"] != role_identities["reviewer"]):
            return dict(role_identities)
    raise ValueError("actual role identities required")


def _object(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    value = json.loads(raw.decode("utf-8"), object_pairs_hook=pairs,
                       parse_constant=lambda value: (_ for _ in ()).throw(ValueError("non-JSON number")))
    if not isinstance(value, dict):
        raise ValueError("JSON object required")
    return value


class Service:
    """Single coordinator, serial exchanges; not safe for concurrent callers.

    `author(role, complete_frame_bytes)` is the calling model or its existing
    subagent, not code in this module. `submit(role, sequence, response)` invokes
    existing respond and returns only after observed success (raises otherwise).
    A submission/authoring failure retires the instance; it never retries.
    """
    def __init__(self, binding, actors, role_prompts, deadline_ns, response_limit,
                 clock=time.time_ns):
        required = {"ApplicationSHA256", "RunID", "ConfigSHA256", "Grant", "SourceRevision"}
        if (set(binding) != required or any(not isinstance(v, str) or not v for v in binding.values())
                or set(actors) != {"interpreter", "reviewer"}
                or any(not isinstance(v, str) or not 0 < len(v) <= 256 for v in actors.values())
                or set(role_prompts) != set(actors)
                or type(deadline_ns) is not int or deadline_ns <= clock()
                or type(response_limit) is not int or not 1 <= response_limit <= 65536):
            raise ValueError("invocation binding unavailable")
        self.binding = dict(binding)
        self.actors = dict(actors)
        self.prompts = dict(role_prompts)
        self.deadline = deadline_ns
        self.response_limit = response_limit
        self.clock = clock
        self.sequence = 0
        self.grouped = False
        self.reviewed = False
        self.retired = False

    def run(self, wait, read_verified_bundle, author, submit, verify_terminal):
        """Drive existing HOST operations using caller-owned ordinary-tool adapters.

        wait(role, after, absolute_deadline) returns a decoded actual HOST event;
        read_verified_bundle performs complete Go-contract bundle verification.
        verify_terminal must observe process exit and reopen/verify publication,
        or raise. Its return is relayed unchanged, never replaced by a success DTO.
        These callbacks are integration requirements, not tools invented here.
        """
        if self.retired or self.sequence != 0:
            raise ValueError("service spent")
        try:
            for _ in range(17):  # <=16 exchanges plus one terminal observation
                if self.clock() >= self.deadline:
                    raise ValueError("original deadline expired")
                role = "reviewer" if self.grouped else "interpreter"
                event = wait(role, self.sequence, self.deadline)
                if (not isinstance(event, dict)
                        or event.get("RunID") != self.binding["RunID"]
                        or event.get("ApplicationSHA256") != self.binding["ApplicationSHA256"]):
                    raise ValueError("event binding refused")
                if event.get("Event") == "TERMINAL":
                    if not self.reviewed:
                        raise ValueError("terminal before separate review")
                    return verify_terminal(event)
                if (event.get("Event") != "NEEDS_HOST" or self.retired
                        or type(event.get("GlobalSequence")) is not int
                        or event["GlobalSequence"] != self.sequence + 1
                        or not isinstance(event.get("Bundle"), dict)):
                    raise ValueError("host event refused; no retry")
                raw = read_verified_bundle(event["Bundle"])
                self.service(raw, author, submit)
            raise ValueError("exchange bound exhausted")
        finally:
            self.retired = True

    def service(self, raw, author, submit):
        if self.retired:
            raise ValueError("service spent")
        # Any attempted exchange retires on failure, including invalid frames.
        self.retired = True
        if not isinstance(raw, bytes) or len(raw) > 1 << 20:
            raise ValueError("frame bound")
        frame = _object(raw)
        operation = frame.get("Operation")
        role = {"Describe": "interpreter", "GroupDecision": "interpreter", "Reviewer": "reviewer"}.get(operation)
        sequence = frame.get("Sequence")
        end = frame.get("DeadlineUnixNano")
        if (frame.get("Protocol") != "PRIVATE_APPLICATION_INVENTORY_V3"
                or role is None or frame.get("Role") != role
                or frame.get("LaunchID") != self.actors[role]
                or frame.get("PromptSHA256") != self.prompts[role]
                or any(frame.get(k) != v for k, v in self.binding.items())
                or type(sequence) is not int or sequence != self.sequence + 1 or sequence > 16
                or type(end) is not int or self.clock() >= min(end, self.deadline)
                or self.reviewed or (operation == "Reviewer" and not self.grouped)
                or (operation != "Reviewer" and self.grouped)):
            raise ValueError("frame binding/order/deadline refused")
        response = author(role, raw)
        if (not isinstance(response, bytes) or not response.endswith(b"\n")
                or len(response) > self.response_limit):
            raise ValueError("bounded caller JSON plus LF required")
        _object(response)
        if self.clock() >= min(end, self.deadline):
            raise ValueError("original deadline expired")
        # Spend before the effect. An uncertain return never restores this slot.
        self.sequence = sequence
        submit(role, sequence, response)
        self.grouped = self.grouped or operation == "GroupDecision"
        self.reviewed = operation == "Reviewer"
        self.retired = self.reviewed
        return operation
