import json
import subprocess
import time
from pathlib import Path

binary = "/Users/schwa/.local/bin/lsp-trace-mcp"
root = "/Users/schwa/dev/lsp-trace"
out_path = Path(root) / ".pi/evidence/structural-context-runner/live-responses.ndjson"
err_path = Path(root) / ".pi/evidence/structural-context-runner/live-stderr.log"

proc = subprocess.Popen(
    [binary, "--enable-live-lsp", "--bootstrap-config", str(Path(root) / ".lsp-trace-mcp-bootstrap.json"), "--tool-profile", "full"],
    cwd=root,
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=open(err_path, "wb"),
    text=True,
    bufsize=1,
)
next_id = 1
records = []

def call(method, params):
    global next_id
    request_id = next_id
    next_id += 1
    proc.stdin.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params}, separators=(",", ":")) + "\n")
    proc.stdin.flush()
    while True:
        line = proc.stdout.readline()
        if not line:
            raise RuntimeError(f"MCP process ended before response {request_id}")
        response = json.loads(line)
        records.append(response)
        if response.get("id") == request_id:
            return response

def tool(name, arguments):
    return call("tools/call", {"name": name, "arguments": arguments})

try:
    call("initialize", {})
    def find_sessions(value):
        if isinstance(value, dict):
            if isinstance(value.get("Sessions"), list):
                return value["Sessions"]
            for child in value.values():
                found = find_sessions(child)
                if found is not None:
                    return found
        elif isinstance(value, list):
            for child in value:
                found = find_sessions(child)
                if found is not None:
                    return found
        return None

    session_id = None
    generation = None
    for _ in range(100):
        response = tool("lsp_session_v1_list", {"detail": "full", "uri": "file:///Users/schwa/dev/lsp-trace"})
        sessions = find_sessions(response) or []
        ready = [s for s in sessions if s.get("routing", {}).get("alias") == "project" and s.get("routing", {}).get("readiness") == "READY"]
        if ready:
            session_id = ready[0]["SessionID"]
            generation = ready[0]["Generation"]
            break
        time.sleep(0.1)
    if session_id is None:
        raise RuntimeError("repo project session did not become READY")
    common = {
        "session_id": session_id,
        "generation": generation,
        "analysis": {"kind": "NEIGHBORHOOD"},
        "up_depth": 0,
        "down_depth": 0,
        "max_nodes": 5,
    }
    runner = tool("lsp_trace_v2_structural_context", {**common, "symbol": "Runner"})
    field = tool("lsp_trace_v2_structural_context", {**common, "symbol": "nearest_outward_consumer"})
    summary = {"session_id": session_id, "generation": generation, "runner": runner, "field": field}
    out_path.write_text("\n".join(json.dumps(record, separators=(",", ":")) for record in records + [summary]) + "\n")
    print(json.dumps(summary, separators=(",", ":")))
finally:
    if proc.stdin:
        proc.stdin.close()
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)
