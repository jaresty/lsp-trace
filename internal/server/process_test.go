package server

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	mode := os.Getenv("LSP_TRACE_HELPER")
	if mode == "" {
		return
	}
	if mode == "graceful" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestStopIsIdempotentAfterGracefulExit(t *testing.T) {
	p, err := Start(context.Background(), os.Args[0], []string{"-test.run=TestHelperProcess"}, []string{"LSP_TRACE_HELPER=graceful"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(5 * time.Second); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestStopKillsUnresponsiveProcessAfterGrace(t *testing.T) {
	p, err := Start(context.Background(), os.Args[0], []string{"-test.run=TestHelperProcess"}, []string{"LSP_TRACE_HELPER=stubborn"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := p.Stop(25 * time.Millisecond); err == nil {
		t.Fatal("expected killed-process error")
	}
	elapsed := time.Since(start)
	if elapsed < 20*time.Millisecond || elapsed > time.Second {
		t.Fatalf("elapsed = %s", elapsed)
	}
}
