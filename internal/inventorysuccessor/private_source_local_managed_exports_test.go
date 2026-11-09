//go:build darwin || linux

package inventorysuccessor

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

type ControlledPipeProbeForTest struct {
	Files      map[string]*os.File
	Closed     map[string]int
	CloseFault error
}
type controlledPipeIOForTest struct {
	*controlledLocalIOV1
	probe *ControlledPipeProbeForTest
}

func ControlledPipeIOForTest(c PrivateSourceLocalConfigV1, p *ControlledPipeProbeForTest) PrivateSourceLocalConfigV1 {
	p.Files = make(map[string]*os.File)
	p.Closed = make(map[string]int)
	c.io = &controlledPipeIOForTest{c.io.(*controlledLocalIOV1), p}
	return c
}
func (c *controlledPipeIOForTest) observeLocalPipeFD(name string, f *os.File) error {
	c.probe.Files[name] = f
	return nil
}
func (c *controlledPipeIOForTest) observeEffect(event string) error {
	e := c.controlledLocalIOV1.observeEffect(event)
	if strings.HasPrefix(event, "pipe-close:") {
		name := strings.TrimPrefix(event, "pipe-close:")
		c.probe.Closed[name]++
		if name == "stdout-write" {
			e = errors.Join(e, c.probe.CloseFault)
		}
	}
	return e
}
func ControlledPipeAssertForTest(t *testing.T, p *ControlledPipeProbeForTest) {
	t.Helper()
	if len(p.Files) != 4 {
		t.Errorf("ASSERT_PIPE_FOUR_CREATED: got=%d", len(p.Files))
	}
	for name, f := range p.Files {
		_, e := f.Stat()
		if !errors.Is(e, os.ErrClosed) {
			t.Errorf("ASSERT_PIPE_ALL_FOUR_OWNED_CLOSED: %s error=%v", name, e)
		}
		if p.Closed[name] != 1 {
			t.Errorf("ASSERT_PIPE_EXACT_ONE_OWNED_CLOSE: %s attempts=%d", name, p.Closed[name])
		}
	}
	if !t.Failed() {
		t.Log("ASSERT_PIPE_ALL_FOUR_OWNED_CLOSED: PASS; ASSERT_PIPE_EXACT_ONE_OWNED_CLOSE: PASS")
	}
}
func ControlledPipeDisposeForTest(p *ControlledPipeProbeForTest) {
	for _, f := range p.Files {
		if _, e := f.Stat(); e == nil {
			f.Close()
		}
	}
}

type ControlledMetadataProbeForTest struct {
	Fault                      string
	Attempts, Debits           map[string]int
	LegacyStats, Opens, Closes int
}
type controlledMetadataIOForTest struct {
	*controlledLocalIOV1
	p *ControlledMetadataProbeForTest
}

func ControlledMetadataIOForTest(c PrivateSourceLocalConfigV1, p *ControlledMetadataProbeForTest) PrivateSourceLocalConfigV1 {
	base := c.io.(*controlledLocalIOV1)
	prior := base.observe
	p.Attempts = map[string]int{}
	p.Debits = map[string]int{}
	base.observe = func(event string) error {
		switch event {
		case "stat":
			p.LegacyStats++
		case "open":
			p.Opens++
		case "close":
			p.Closes++
		}
		if strings.HasPrefix(event, "managed-metadata-debit:") {
			p.Debits[strings.TrimPrefix(event, "managed-metadata-debit:")]++
		}
		return prior(event)
	}
	c.io = &controlledMetadataIOForTest{base, p}
	return c
}
func (c *controlledMetadataIOForTest) managedPinFstat(kind string, fd uintptr, st *syscall.Stat_t) error {
	c.p.Attempts[kind]++
	if c.p.Debits[kind] != c.p.Attempts[kind] {
		return errors.New("managed metadata attempt lacked prior debit")
	}
	if c.p.Fault == kind {
		return syscall.EINTR
	} // controlled attempt error, not delivered kernel signal
	return singleLocalDirectoryFstatV1(fd, st) // actual held descriptor; never fabricated metadata
}
func ControlledMetadataAssertForTest(t *testing.T, p *ControlledMetadataProbeForTest) {
	t.Helper()
	base, dep := 12, 2
	opens := 7
	if p.Fault == "base" {
		base, dep, opens = 1, 0, 1
	}
	if p.Fault == "dependency" {
		dep = 1
	}
	if p.Attempts["base"] != base || p.Attempts["dependency"] != dep || p.LegacyStats != 0 || p.Opens != opens || p.Closes != opens || p.Debits["base"] != base || p.Debits["dependency"] != dep {
		t.Errorf("ASSERT_METADATA_SINGLE_RAW_ATTEMPT: fault=%s raw=%v debits=%v File.Stat=%d opens=%d closes=%d", p.Fault, p.Attempts, p.Debits, p.LegacyStats, p.Opens, p.Closes)
	} else {
		t.Log("ASSERT_METADATA_SINGLE_RAW_ATTEMPT: PASS; raw real-FD success or one terminal controlled EINTR; every attempt prepaid")
	}
}

func ControlledManagedLocalQuoteForTest(t *testing.T, c PrivateSourceLocalConfigV1) [14]uint64 {
	t.Helper()
	q, e := sourceLocalConfigChargeV1(c)
	if e != nil {
		t.Fatal(e)
	}
	return q.vector()
}

// Fixture disposal is not production retirement or an Observe receipt. Failed
// lifecycle/source jobs remain PENDING; only controlled fixture FDs are closed.
func ControlledManagedCloseForTest(m PrivateSourceLocalManagedV1) {
	if m.state == nil {
		return
	}
	g := m.state
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	for _, f := range g.files {
		if f != nil {
			f.Close()
		}
	}
	g.stage.cleanupLocalStageV1()
}
