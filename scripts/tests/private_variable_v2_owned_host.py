#!/usr/bin/env python3
"""Owned HOST emulator process, no model/provider. Not a production fallback."""
import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import time

spec = importlib.util.spec_from_file_location('handoff', sys.argv[1])
h = importlib.util.module_from_spec(spec)
spec.loader.exec_module(h)
mailbox, payload_dir = sys.argv[2:4]
# Existing Go-owned fixture layout only. Never discover a real configuration.
base = Path(mailbox).parent
config_path = base / 'config.json'
base_fd = h.directory(str(base))
try:
    config_pin = h.digest(h.read(base_fd, 'config.json', 64 << 10))
finally:
    os.close(base_fd)
fd = h.directory(mailbox)
try:
    sequence = 1
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        names = os.listdir(fd)
        if 'terminal.json' in names:
            status = h.decode(h.read(fd, 'terminal.json'))['Status']
            if status != 'VERIFIED_PAIR_PRIVATE_V2_AST':
                raise ValueError('owned terminal failed')
            print('OWNED_HOST_TERMINAL_PASS', flush=True)
            break
        if f'request-{sequence:06d}.json' not in names:
            time.sleep(0.005)
            continue
        export_dir = base / f'export-{sequence:06d}'
        command = [sys.executable, sys.argv[1], 'read', '--mailbox', mailbox, '--sequence', str(sequence),
                   '--export-dir', str(export_dir), '--config', str(config_path), '--config-sha256', config_pin,
                   '--control', str(base / 'control')]
        exported = subprocess.run(command, capture_output=True, check=True, timeout=5)
        if len(exported.stdout) > h.EXPORT_RECEIPT_CAP + 1:
            raise ValueError('owned export stdout ceiling')
        receipt = h.json.loads(exported.stdout)
        efd = h.directory(str(export_dir))
        try:
            manifest_raw = h.read(efd, receipt['Manifest'], h.EXPORT_MANIFEST_CAP)
            if (h.digest(manifest_raw), len(manifest_raw)) != (receipt['ManifestSHA256'], receipt['ManifestByteLength']):
                raise ValueError('owned manifest mismatch')
            manifest = h.decode(manifest_raw)
            if (manifest['Frame']['SHA256'], manifest['Frame']['ByteLength'], len(manifest['Parts'])) != (receipt['FrameSHA256'], receipt['FrameByteLength'], receipt['PartCount']):
                raise ValueError('owned manifest frame identity')
            complete = bytearray()
            for index, part in enumerate(manifest['Parts'], 1):
                if part['Name'] != f'part-{index:06d}.txt' or part['Start'] != len(complete):
                    raise ValueError('owned part order')
                raw = h.read(efd, part['Name'], h.EXPORT_PART_BYTES)
                raw.decode('utf-8', errors='strict')
                if (len(raw), h.digest(raw), len(complete) + len(raw)) != (part['ByteLength'], part['SHA256'], part['End']):
                    raise ValueError('owned part mismatch')
                complete.extend(raw)
            if (len(complete), h.digest(complete)) != (receipt['FrameByteLength'], receipt['FrameSHA256']) or bytes(complete) != h.read(efd, 'frame.json'):
                raise ValueError('owned complete evidence mismatch')
            frame = h.decode(bytes(complete))
            request_raw = h.read(efd, 'request.json')
            if bytes(complete)[manifest['Request']['Start']:manifest['Request']['End']] != request_raw or (h.digest(request_raw), len(request_raw)) != (manifest['Request']['SHA256'], manifest['Request']['ByteLength']) or h.digest(request_raw) != frame['Binding']['RequestSHA256'] or frame['Binding'] != manifest['Binding']:
                raise ValueError('owned request binding mismatch')
        finally:
            os.close(efd)
        print(f'OWNED_HOST_EXPORT_PASS sequence={sequence} parts={receipt["PartCount"]}', flush=True)
        r = frame['Request']
        if r['Stage'] == 'INTERPRET':
            q = r['Bodies'][0]
            body = q['Body']
            payload = dict(Disposition='DESCRIBED', Label='positive accumulation', Description='Starts at zero, adds positive inputs, and returns total.', Limitations='Own AST only; runtime unknown', Alternatives='May not run', RejectionCondition='No positive predicate', Claims=[dict(Text='Accumulates positive values', Citations=[dict(Handle=q['Handle'], Start=0, End=len(body.encode()), Quote=body)])])
        elif r['Stage'].startswith('REVIEW_'):
            # Complete request packet, own bodies, attempted revision and all prior
            # revisions are received, not merely the chosen citation list.
            if r['Stage'] == 'REVIEW_TECHNICAL' and not r['Packet']:
                raise ValueError('review evidence absent')
            payload = dict(Digest=r['RevisionDigest'], Verdict='APPROVE', Findings='Owned emulator, no semantic-quality claim')
        else:
            payload = []
        pfd = h.directory(payload_dir)
        name = f'payload-{sequence:06d}.json'
        try:
            h.publish(pfd, name, h.canonical(payload))
        finally:
            os.close(pfd)
        returned = subprocess.run([sys.executable, sys.argv[1], 'respond', '--mailbox', mailbox, '--sequence', str(sequence),
                                   '--payload', os.path.join(payload_dir, name)], capture_output=True, check=True, timeout=5)
        response_raw = h.read(fd, f'response-{sequence:06d}.json')
        response = h.decode(response_raw)
        if h.json.loads(returned.stdout) != dict(ResponseSHA256=h.digest(response_raw), ResponseByteLength=len(response_raw)) or response['Binding'] != frame['Binding']:
            raise ValueError('owned response receipt mismatch')
        exact = h.canonical(payload)
        if h.base64.b64decode(response['Payload']) != exact or response['PayloadSHA256'] != h.digest(exact) or response['PayloadByteLength'] != len(exact):
            raise ValueError('owned exact response mismatch')
        print(f'OWNED_HOST_RETURN_PASS sequence={sequence}', flush=True)
        sequence += 1
    else:
        raise ValueError('owned host deadline')
finally:
    os.close(fd)
