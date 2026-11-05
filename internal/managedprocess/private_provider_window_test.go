package managedprocess

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPrivateProviderWindowSelf(t *testing.T) {
	if os.Getenv("PRIVATE_WINDOW_CONTROLLED_SELF") != "1" {
		return
	}
	_, _ = io.WriteString(os.Stderr, "controlled-self\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}
func TestPrivateProviderWindowManagedSelfJoinedReapCensus(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("PRIVATE_WINDOW_DARWIN_SELF_UNAVAILABLE: compile-only on this platform")
	}
	m := newHermeticManager(Options{StderrLimit: 1024, GracePeriod: 10 * time.Millisecond})
	deadline := time.Now().Add(8 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	ctx, err := m.PrivateProviderWindowContext(ctx, m.PrivateProviderWindowProfile(), deadline)
	if err != nil {
		t.Fatal(err)
	}
	p, started := m.Start(ctx, Spec{Path: "__managedprocess_helper__", Args: []string{"-test.run=^TestPrivateProviderWindowSelf$"}, Env: []string{"PRIVATE_WINDOW_CONTROLLED_SELF=1", "GOENV=off", "GOFLAGS=-mod=readonly", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off"}})
	if started.Kind != StartStarted {
		t.Fatalf("start: %+v", started)
	}
	t.Cleanup(func() { _ = p.Stdin().Close(); _ = p.Teardown(ctx); _ = p.Close(); _ = p.Wait() })
	_ = p.Stdin().Close()
	death := p.Wait()
	if death.Reap.Kind != ReapComplete {
		t.Fatal("ASSERT_WINDOW_SOLE_REAP_JOIN")
	}
	reconciled := p.ReconcileReap(ctx)
	if !reconciled.Census.Bounded || reconciled.Census.Truncated || reconciled.Census.Err != nil || reconciled.Census.Members != 0 {
		t.Fatalf("ASSERT_WINDOW_POST_REAP_CENSUS: %+v", reconciled.Census)
	}
	if p.Close().Kind != ResourcesClosed {
		t.Fatal("ASSERT_WINDOW_SELF_RESOURCES_CLOSED")
	}
	if len(death.Stderr.Bytes) > 1024 || death.StderrDelivery.Err != nil {
		t.Fatalf("ASSERT_WINDOW_SELF_STDERR_JOIN: %+v", death.StderrDelivery)
	}
}
func TestPrivateProviderWindowCensusBlankLinesAndClip(t *testing.T) {
	b := &privateProviderWindow{deadline: time.Now().Add(time.Second), censusBytes: 4096, censusLines: 4, censusMembers: 2, censusInvocations: 2}
	result := parsePrivateCensus(strings.NewReader("\n\n\n\n\n"), b)
	if !result.Truncated || result.Err == nil {
		t.Fatal("ASSERT_WINDOW_CENSUS_BLANK_LINES")
	}
	probe := &privateWindowReadProbe{}
	b.censusLines, b.censusMembers = 4, 2
	b.censusBytes = 3
	_, _ = b.censusRead(make([]byte, 100), probe)
	if probe.maximum <= 0 || probe.maximum > 3 {
		t.Fatalf("ASSERT_WINDOW_CENSUS_READ_CLIPPED: %d", probe.maximum)
	}
	probe.maximum = 0
	b.censusLines = 0
	_, _ = b.censusRead(make([]byte, 100), probe)
	if probe.maximum != 0 {
		t.Fatal("ASSERT_WINDOW_CENSUS_LINE_PREBOUND")
	}
	b.censusLines = 4
	b.censusInvocations = 0
	if _, _, ok := b.beginCensus(); ok {
		t.Fatal("ASSERT_WINDOW_CENSUS_INVOCATIONS")
	}
	b.censusInvocations = 1
	b.deadline = time.Now().Add(-time.Second)
	if _, _, ok := b.beginCensus(); ok {
		t.Fatal("ASSERT_WINDOW_CENSUS_ORIGINAL_DEADLINE")
	}
}

type privateWindowReadProbe struct{ maximum int }

func (p *privateWindowReadProbe) Read(b []byte) (int, error) { p.maximum = len(b); return 0, io.EOF }
func TestPrivateProviderWindowStderrDeliveryNotRetention(t *testing.T) {
	b := &privateProviderWindow{deadline: time.Now().Add(time.Second), stderr: 3}
	var delivered bytes.Buffer
	err := b.copyStderr(io.NopCloser(strings.NewReader("abcdef")), &delivered)
	if err == nil || delivered.String() != "abc" {
		t.Fatalf("ASSERT_WINDOW_STDERR_DELIVERY_PREBOUND: %q err=%v", delivered.String(), err)
	}
}
