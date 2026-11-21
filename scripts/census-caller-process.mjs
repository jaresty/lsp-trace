// Thin, host-neutral process/byte transport. No census scheduling, model calls,
// native proof persistence, role decisions, or publication validation live here.
import { spawn } from 'node:child_process';

export class CensusCallerProcess {
  constructor(command, args, { cwd, env = process.env } = {}) {
    this.queue = [];
    this.buffer = Buffer.alloc(0);
    this.failure = undefined;
    this.pending = undefined;
    this.stderr = '';
    this.stdout = '';
    this.child = spawn(command, args, {
      cwd, env: { ...env, LSP_TRACE_CALLER_FD: '3' },
      stdio: ['ignore', 'pipe', 'pipe', 'pipe'], detached: false,
    });
    this.stream = this.child.stdio[3];
    this.closed = new Promise((resolve) => {
      this.child.once('close', (code, signal) => {
        this.exit = { code, signal };
        this.fail(new Error(`Census child closed (${code ?? signal}): ${this.stderr}`));
        resolve(this.exit);
      });
    });
    this.child.once('error', (error) => this.fail(error));
    this.stream.on('error', (error) => this.fail(error));
    this.stream.on('data', (chunk) => {
      try {
        this.buffer = Buffer.concat([this.buffer, chunk]);
        if (this.buffer.length > (1 << 20) + 8192) throw new Error('Caller frame buffer exceeded');
        while (this.buffer.length >= 4) {
          const n = this.buffer.readUInt32BE();
          if (!n || n > (1 << 20) + 4096) throw new Error('Caller frame size refused');
          if (this.buffer.length < n + 4) break;
          const event = JSON.parse(this.buffer.subarray(4, n + 4).toString('utf8'));
          this.buffer = this.buffer.subarray(n + 4);
          if (this.pending) { const next = this.pending; this.pending = undefined; next.resolve(event); }
          else {
            if (this.queue.length >= 2) throw new Error('Caller queue exceeded');
            this.queue.push(event);
          }
        }
      } catch (error) { this.fail(error); void this.stop(); }
    });
    for (const field of ['stdout', 'stderr']) {
      this.child[field].on('data', (chunk) => {
        this[field] += chunk.toString('utf8');
        if (Buffer.byteLength(this[field]) > 64 * 1024) {
          this[field] = this[field].slice(0, 4096);
          this.fail(new Error(`Census ${field} exceeded`)); void this.stop();
        }
      });
    }
  }
  fail(error) {
    this.failure ??= error;
    if (this.pending) { this.pending.reject(this.failure); this.pending = undefined; }
  }
  send(value) {
    if (this.failure || this.exit) throw this.failure ?? new Error('Census child closed');
    const body = Buffer.from(JSON.stringify(value));
    if (!body.length || body.length > 96 * 1024) throw new Error('Caller reply size refused');
    const header = Buffer.alloc(4); header.writeUInt32BE(body.length);
    this.stream.write(Buffer.concat([header, body]));
  }
  async next(signal) {
    if (this.pending) throw new Error('Only one caller receive may be active');
    if (signal?.aborted) { await this.stop(); throw new Error('Caller aborted'); }
    if (this.queue.length) return this.queue.shift();
    if (this.failure) throw this.failure;
    const abort = () => { this.fail(new Error('Caller aborted')); void this.stop(); };
    signal?.addEventListener('abort', abort, { once: true });
    try { return await new Promise((resolve, reject) => { this.pending = { resolve, reject }; }); }
    finally { signal?.removeEventListener('abort', abort); }
  }
  async stop() {
    if (this.stopping) return this.stopping;
    this.stopping = (async () => {
      // EOF wakes Go's bounded reader; TERM also cancels its controller context.
      // Keep the child owned until close. Never detach or unref it.
      this.stream.destroy();
      if (!this.exit) this.child.kill('SIGTERM');
      const kill = setTimeout(() => { if (!this.exit) this.child.kill('SIGKILL'); }, 2000);
      try { return await this.closed; } finally { clearTimeout(kill); }
    })();
    return this.stopping;
  }
}
