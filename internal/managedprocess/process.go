// Package managedprocess owns fail-closed child process construction and cleanup.
// Its observations are local process-management evidence, never native containment
// or Stage 2 readiness evidence.
package managedprocess

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"lsp-trace/internal/containment"
)

const (
	localEvidence              = "hermetic process-management observation only"
	LocalDarwinSupervisionOnly = "LOCAL_DARWIN_SUPERVISION_ONLY"
	localDarwinLimitations     = "local Darwin process-group supervision does not prove anti-escape, owner-death cleanup, descendant containment, or VERIFIED"
)

type Options struct {
	StderrLimit int
	GracePeriod time.Duration
}

type Spec struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

type StartKind string

const (
	StartUnavailable StartKind = "UNAVAILABLE"
	StartFailed      StartKind = "FAILED"
	StartStarted     StartKind = "STARTED"
)

type StartObservation struct {
	Kind        StartKind
	Reason      string
	Err         error
	Evidence    string
	Limitations string
}

type DeathKind string

const (
	DeathExited   DeathKind = "EXITED"
	DeathSignaled DeathKind = "SIGNALED"
	DeathUnknown  DeathKind = "UNKNOWN"
)

type StderrObservation struct {
	Bytes     []byte
	Truncated bool
}

type ReapKind string

const (
	ReapComplete ReapKind = "COMPLETE"
	ReapFailed   ReapKind = "FAILED"
)

type ReapObservation struct {
	Kind     ReapKind
	Err      error
	Evidence string
}

type DeathObservation struct {
	Kind     DeathKind
	ExitCode int
	Err      error
	Stderr   StderrObservation
	Reap     ReapObservation
	Evidence string
}

type SurvivorKind string

const (
	SurvivorRunning SurvivorKind = "RUNNING"
	SurvivorDead    SurvivorKind = "DEAD"
)

type SurvivorObservation struct {
	Kind     SurvivorKind
	Evidence string
}

type ResourceKind string

const (
	ResourcesClosed      ResourceKind = "CLOSED"
	ResourcesCloseFailed ResourceKind = "CLOSE_FAILED"
)

type ResourceObservation struct {
	Kind     ResourceKind
	Err      error
	Evidence string
}

type TeardownPhase string

const (
	PhaseInterrupt TeardownPhase = "INTERRUPT"
	PhaseWait      TeardownPhase = "WAIT"
	PhaseKill      TeardownPhase = "KILL"
	PhaseReap      TeardownPhase = "REAP"
)

type GroupCensusObservation struct {
	ProcessGroupID int
	Members        int
	Limit          int
	Bounded        bool
	Truncated      bool
	Err            error
	Evidence       string
}

type TeardownObservation struct {
	Phases []TeardownPhase
	Death  DeathObservation
	Census GroupCensusObservation
}

type commandFactory func(context.Context, Spec) (*exec.Cmd, error)

type Manager struct {
	available   bool
	localDarwin bool
	options     Options
	factory     commandFactory
}

// New consumes only the sealed production gate. Current containment gates are
// unavailable, so Start returns before command construction or pipe creation.
func New(gate containment.RuntimeGate, options Options) *Manager {
	return &Manager{
		available: gate.Snapshot().Classification != containment.Unavailable,
		options:   normalizedOptions(options),
		factory:   commandFromSpec,
	}
}

// NewLocalDarwinSupervisor explicitly opts into local process-group supervision.
// It is not a containment gate and cannot make production startup available.
func NewLocalDarwinSupervisor(options Options) (*Manager, error) {
	if !localDarwinSupported() {
		return nil, errors.New("managedprocess: local Darwin supervision unavailable")
	}
	return &Manager{available: true, localDarwin: true, options: normalizedOptions(options), factory: commandFromSpec}, nil
}

// newHermeticManager exercises process mechanics without representing a
// production containment authorization path.
func newHermeticManager(options Options) *Manager {
	return &Manager{available: true, localDarwin: localDarwinSupported(), options: normalizedOptions(options), factory: commandFromSpec}
}

func normalizedOptions(options Options) Options {
	if options.StderrLimit < 0 {
		options.StderrLimit = 0
	}
	if options.GracePeriod <= 0 {
		options.GracePeriod = 100 * time.Millisecond
	}
	return options
}

func commandFromSpec(ctx context.Context, spec Spec) (*exec.Cmd, error) {
	if spec.Path == "" {
		return nil, errors.New("managedprocess: empty command path")
	}
	path := spec.Path
	if path == "__managedprocess_helper__" {
		path = os.Args[0]
	}
	cmd := exec.CommandContext(ctx, path, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = append([]string(nil), spec.Env...)
	}
	return cmd, nil
}

func (m *Manager) Start(ctx context.Context, spec Spec) (*Process, StartObservation) {
	evidence := localEvidence
	limitations := ""
	if m != nil && m.localDarwin {
		evidence = LocalDarwinSupervisionOnly
		limitations = localDarwinLimitations
	}
	if m == nil || !m.available {
		return nil, StartObservation{Kind: StartUnavailable, Reason: "containment unavailable", Evidence: evidence, Limitations: limitations}
	}
	if m.factory == nil {
		return nil, StartObservation{Kind: StartFailed, Reason: "command factory unavailable", Evidence: evidence, Limitations: limitations}
	}
	cmd, err := m.factory(ctx, spec)
	if err != nil {
		return nil, StartObservation{Kind: StartFailed, Reason: "command construction failed", Err: err, Evidence: evidence, Limitations: limitations}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, StartObservation{Kind: StartFailed, Reason: "stdin pipe failed", Err: err, Evidence: evidence, Limitations: limitations}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, StartObservation{Kind: StartFailed, Reason: "stdout pipe failed", Err: err, Evidence: evidence, Limitations: limitations}
	}
	stderr := &boundedBuffer{limit: m.options.StderrLimit}
	cmd.Stderr = stderr
	if m.localDarwin {
		if err := configureLocalDarwin(cmd); err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			return nil, StartObservation{Kind: StartUnavailable, Reason: "local Darwin supervision unavailable", Err: err, Evidence: LocalDarwinSupervisionOnly, Limitations: localDarwinLimitations}
		}
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, StartObservation{Kind: StartFailed, Reason: "process start failed", Err: err, Evidence: evidence, Limitations: limitations}
	}
	p := &Process{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr, gracePeriod: m.options.GracePeriod, done: make(chan struct{}), evidence: evidence, localDarwin: m.localDarwin}
	go p.reap()
	return p, StartObservation{Kind: StartStarted, Evidence: evidence, Limitations: limitations}
}

type Process struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      *boundedBuffer
	gracePeriod time.Duration
	done        chan struct{}
	evidence    string
	localDarwin bool

	waitOnce  sync.Once
	death     DeathObservation
	closeOnce sync.Once
	resources ResourceObservation
}

func (p *Process) Stdin() io.WriteCloser { return p.stdin }
func (p *Process) Stdout() io.ReadCloser { return p.stdout }

func (p *Process) reap() {
	err := p.cmd.Wait()
	p.death = deathFromWait(p.cmd, err, p.stderr.snapshot())
	if p.localDarwin {
		p.death.Evidence = p.evidence
		p.death.Reap.Evidence = p.evidence
	}
	close(p.done)
}

func (p *Process) Wait() DeathObservation {
	<-p.done
	return p.death
}

func (p *Process) Observe() SurvivorObservation {
	select {
	case <-p.done:
		return SurvivorObservation{Kind: SurvivorDead, Evidence: localEvidence}
	default:
		return SurvivorObservation{Kind: SurvivorRunning, Evidence: localEvidence}
	}
}

func (p *Process) Teardown(ctx context.Context) TeardownObservation {
	phases := []TeardownPhase{PhaseInterrupt}
	census := GroupCensusObservation{}
	if p.localDarwin {
		census = censusLocalDarwinGroup(p.cmd.Process.Pid, 64)
	}
	select {
	case <-p.done:
		phases = append(phases, PhaseReap)
		return TeardownObservation{Phases: phases, Death: p.death, Census: census}
	default:
	}
	if p.localDarwin {
		_ = signalLocalDarwinGroup(p.cmd.Process.Pid, os.Interrupt)
	} else {
		_ = p.cmd.Process.Signal(os.Interrupt)
	}
	phases = append(phases, PhaseWait)
	timer := time.NewTimer(p.gracePeriod)
	defer timer.Stop()
	select {
	case <-p.done:
	case <-ctx.Done():
		phases = append(phases, PhaseKill)
		if p.localDarwin {
			_ = signalLocalDarwinGroup(p.cmd.Process.Pid, os.Kill)
		} else {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	case <-timer.C:
		phases = append(phases, PhaseKill)
		if p.localDarwin {
			_ = signalLocalDarwinGroup(p.cmd.Process.Pid, os.Kill)
		} else {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	}
	phases = append(phases, PhaseReap)
	return TeardownObservation{Phases: phases, Death: p.death, Census: census}
}

func (p *Process) Close() ResourceObservation {
	p.closeOnce.Do(func() {
		var errs []error
		if p.stdin != nil {
			errs = appendIf(errs, p.stdin.Close())
		}
		if p.stdout != nil {
			errs = appendIf(errs, p.stdout.Close())
		}
		if err := errors.Join(errs...); err != nil {
			p.resources = ResourceObservation{Kind: ResourcesCloseFailed, Err: err, Evidence: p.evidence}
		} else {
			p.resources = ResourceObservation{Kind: ResourcesClosed, Evidence: p.evidence}
		}
	})
	return p.resources
}

func appendIf(errs []error, err error) []error {
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return append(errs, err)
	}
	return errs
}

func deathFromWait(cmd *exec.Cmd, err error, stderr StderrObservation) DeathObservation {
	observation := DeathObservation{Kind: DeathUnknown, ExitCode: -1, Err: err, Stderr: stderr, Evidence: localEvidence}
	observation.Reap = ReapObservation{Kind: ReapComplete, Evidence: localEvidence}
	if cmd.ProcessState != nil {
		observation.ExitCode = cmd.ProcessState.ExitCode()
		if observation.ExitCode >= 0 {
			observation.Kind = DeathExited
		} else {
			observation.Kind = DeathSignaled
		}
	} else {
		observation.Reap = ReapObservation{Kind: ReapFailed, Err: err, Evidence: localEvidence}
	}
	return observation
}

type boundedBuffer struct {
	mu        sync.Mutex
	limit     int
	bytes     []byte
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - len(b.bytes)
	if remaining > 0 {
		n := len(p)
		if n > remaining {
			n = remaining
		}
		b.bytes = append(b.bytes, p[:n]...)
	}
	if len(p) > remaining {
		b.truncated = true
	}
	return len(p), nil
}

func (b *boundedBuffer) snapshot() StderrObservation {
	b.mu.Lock()
	defer b.mu.Unlock()
	return StderrObservation{Bytes: append([]byte(nil), b.bytes...), Truncated: b.truncated}
}
