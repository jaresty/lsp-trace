package managedprocess

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

var ErrPrivateProviderWindow = errors.New("managedprocess: private provider window unavailable")

// Fixed maxima are prepaid before constructing a backend. Profiles below quote
// the actual owner; these constants confer no process or funding authority.
const PrivateProviderStderrReadLimit = 64 << 10
const PrivateProviderStderrScratch = 4096
const PrivateProviderCensusReadLimit = 4096

// PrivateProviderWindowProfile is a read-only quote of this exact backend. It
// grants no process, containment, funding, or production authority.
type PrivateProviderWindowProfile struct {
	owner                                           *Manager
	Available, LocalDarwin                          bool
	StderrRetention                                 int
	StderrReadLimit, StderrScratch, CensusReadLimit int
	GracePeriod                                     time.Duration
}

func (m *Manager) PrivateProviderWindowProfile() PrivateProviderWindowProfile {
	if m == nil {
		return PrivateProviderWindowProfile{}
	}
	return PrivateProviderWindowProfile{owner: m, Available: m.available, LocalDarwin: m.localDarwin, StderrRetention: m.options.StderrLimit, StderrReadLimit: PrivateProviderStderrReadLimit, StderrScratch: PrivateProviderStderrScratch, CensusReadLimit: PrivateProviderCensusReadLimit, GracePeriod: m.options.GracePeriod}
}

type privateProviderWindowKey struct{}
type privateProviderWindow struct {
	ctx                                                                context.Context
	manager                                                            *Manager
	profile                                                            PrivateProviderWindowProfile
	deadline                                                           time.Time
	mu                                                                 sync.Mutex
	stderr, censusBytes, censusLines, censusMembers, censusInvocations int
}

// PrivateProviderWindowContext binds the actual backend before command creation.
// Cleanup never renews this allowance. Unsupported platforms refuse pre-effect.
func (m *Manager) PrivateProviderWindowContext(ctx context.Context, p PrivateProviderWindowProfile, deadline time.Time) (context.Context, error) {
	if ctx == nil || ctx.Err() != nil || p != m.PrivateProviderWindowProfile() || !p.Available || !p.LocalDarwin || !time.Now().Before(deadline) {
		return nil, ErrPrivateProviderWindow
	}
	if b := privateProviderWindowFrom(ctx); b != nil {
		if b.manager != m || b.profile != p || !b.deadline.Equal(deadline) {
			return nil, ErrPrivateProviderWindow
		}
		return ctx, nil
	}
	return context.WithValue(ctx, privateProviderWindowKey{}, &privateProviderWindow{ctx: ctx, manager: m, profile: p, deadline: deadline, stderr: PrivateProviderStderrReadLimit, censusBytes: PrivateProviderCensusReadLimit, censusLines: 128, censusMembers: 64, censusInvocations: 2}), nil
}
func privateProviderWindowFrom(ctx context.Context) *privateProviderWindow {
	if ctx == nil {
		return nil
	}
	b, _ := ctx.Value(privateProviderWindowKey{}).(*privateProviderWindow)
	return b
}
func (b *privateProviderWindow) valid(m *Manager) bool {
	return b != nil && b.manager == m && b.profile == m.PrivateProviderWindowProfile() && time.Now().Before(b.deadline)
}

// PrivateProviderWindowReadStderr exposes the same pre-read admission used by
// the production drainer. It reads only the caller-owned reader/buffer; it starts
// no process and exposes no retained process content, reset, or window authority.
func (m *Manager) PrivateProviderWindowReadStderr(ctx context.Context, p PrivateProviderWindowProfile, deadline time.Time, src io.Reader, dst []byte) (int, error) {
	if ctx == nil || ctx.Err() != nil || src == nil || p != m.PrivateProviderWindowProfile() {
		return 0, ErrPrivateProviderWindow
	}
	b := privateProviderWindowFrom(ctx)
	d, ok := ctx.Deadline()
	if !ok || !d.Equal(deadline) || !b.valid(m) || b.profile != p || !b.deadline.Equal(deadline) {
		return 0, ErrPrivateProviderWindow
	}
	return b.readStderr(src, dst)
}
func (b *privateProviderWindow) readStderr(src io.Reader, dst []byte) (int, error) {
	b.mu.Lock()
	n := len(dst)
	if n > PrivateProviderStderrScratch {
		n = PrivateProviderStderrScratch
	}
	if n > b.stderr {
		n = b.stderr
	}
	if !time.Now().Before(b.deadline) || b.ctx.Err() != nil {
		n = 0
	}
	b.stderr -= n
	b.mu.Unlock()
	if n == 0 {
		return 0, ErrPrivateProviderWindow
	}
	got, e := src.Read(dst[:n])
	if got < 0 || got > n {
		return 0, ErrPrivateProviderWindow
	}
	if e != nil {
		if errors.Is(e, io.EOF) {
			return got, io.EOF
		}
		return got, ErrPrivateProviderWindow
	}
	return got, nil
}

// copyStderr clips before every pipe read, reserves before delivery, and never
// refunds ambiguous/short reads. Its sole join belongs to Process.reap.
func (b *privateProviderWindow) copyStderr(src io.ReadCloser, dst io.Writer) error {
	defer src.Close()
	var storage [PrivateProviderStderrScratch]byte
	for {
		got, err := b.readStderr(src, storage[:])
		if got > 0 {
			written, e := dst.Write(storage[:got])
			if e != nil {
				return e
			}
			if written != got {
				return io.ErrShortWrite
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// beginCensus returns a finite residual before /bin/ps construction. The parser
// consumes byte, line, and member residuals, including whitespace-only output.
func (b *privateProviderWindow) beginCensus() (context.Context, context.CancelFunc, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.censusInvocations == 0 || !time.Now().Before(b.deadline) {
		return nil, nil, false
	}
	b.censusInvocations--
	d := time.Now().Add(100 * time.Millisecond)
	if b.deadline.Before(d) {
		d = b.deadline
	}
	ctx, cancel := context.WithDeadline(context.Background(), d)
	return ctx, cancel, true
}
func (b *privateProviderWindow) censusRead(p []byte, src io.Reader) (int, error) {
	b.mu.Lock()
	n := len(p)
	if n > b.censusBytes {
		n = b.censusBytes
	}
	// One byte cannot contain more than one new line or member boundary.
	// Scanner read-ahead therefore cannot deliver an unpriced extra line.
	if n > 1 {
		n = 1
	}
	if !time.Now().Before(b.deadline) || b.censusLines == 0 || b.censusMembers == 0 {
		n = 0
	}
	b.censusBytes -= n
	b.mu.Unlock()
	if n == 0 {
		return 0, ErrPrivateProviderWindow
	}
	got, err := src.Read(p[:n])
	if got < 0 || got > n {
		return 0, ErrPrivateProviderWindow
	}
	return got, err
}
func (b *privateProviderWindow) censusLine(member bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.censusLines == 0 || member && b.censusMembers == 0 {
		return false
	}
	b.censusLines--
	if member {
		b.censusMembers--
	}
	return true
}

func (p *Process) censusPrivateWindow() GroupCensusObservation {
	if p.privateWindow == nil {
		return censusLocalDarwinGroup(p.cmd.Process.Pid, 64)
	}
	return censusLocalDarwinGroupPrivateWindow(p.cmd.Process.Pid, p.privateWindow)
}
func parsePrivateCensus(src io.Reader, b *privateProviderWindow) GroupCensusObservation {
	result := GroupCensusObservation{Limit: 64, Bounded: true, Evidence: LocalDarwinSupervisionOnly}
	scanner := bufio.NewScanner(privateCensusReader{b, src})
	scanner.Buffer(make([]byte, 256), 4096)
	for scanner.Scan() {
		member := strings.TrimSpace(scanner.Text()) != ""
		if !b.censusLine(member) {
			result.Truncated = true
			result.Err = ErrPrivateProviderWindow
			return result
		}
		if member {
			result.Members++
		}
	}
	result.Err = scanner.Err()
	if result.Err != nil {
		result.Truncated = true
	}
	return result
}

type privateCensusReader struct {
	b *privateProviderWindow
	r io.Reader
}

func (r privateCensusReader) Read(p []byte) (int, error) { return r.b.censusRead(p, r.r) }
