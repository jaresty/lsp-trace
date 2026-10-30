#!/usr/bin/env python3
"""Owned HOST emulator process, no model/provider. Not a production fallback."""
import importlib.util
import os
import sys
import time

spec = importlib.util.spec_from_file_location('handoff', sys.argv[1])
h = importlib.util.module_from_spec(spec)
spec.loader.exec_module(h)
mailbox, payload_dir = sys.argv[2:4]
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
        frame = h.request(fd, sequence)
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
        h.respond(mailbox, sequence, os.path.join(payload_dir, name))
        sequence += 1
    else:
        raise ValueError('owned host deadline')
finally:
    os.close(fd)
