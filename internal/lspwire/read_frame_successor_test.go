package lspwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestOrdinaryReadWithFrameCharacterizationUnchanged(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	var stages []EventStage
	r := NewReaderObserved(bytes.NewReader(frame), Limits{}, func(e Event) { stages = append(stages, e.Stage) })
	m, observation, err := r.ReadWithFrame()
	if err != nil {
		t.Fatalf("ReadWithFrame: %v", err)
	}
	if m.Method != "x" {
		t.Fatalf("method=%q want x", m.Method)
	}
	sum := sha256.Sum256(frame)
	if observation.FrameBytes != int64(len(frame)) || observation.FrameSHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("observation=%+v frame len=%d", observation, len(frame))
	}
	bodyAt, decodeAt := -1, -1
	for i, stage := range stages {
		if stage == EventBodyRead {
			bodyAt = i
		}
		if stage == EventDecode {
			decodeAt = i
		}
	}
	if bodyAt < 0 || decodeAt < 0 || bodyAt >= decodeAt {
		t.Fatalf("stages=%v: body read must precede decode", stages)
	}
}

func TestSuccessorCaptureIsExplicitAndOnceBeforeDecode(t *testing.T) {
	frame := append(successorHeader(30), successorMessage()...)
	capture := &successorCapture{}
	o := testSuccessorOptions()
	o.Capture = capture
	r, err := NewSuccessorIngressReader(bytes.NewReader(frame), o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFrame(); err != nil {
		t.Fatal(err)
	}
	if len(capture.frames) != 1 || !bytes.Equal(capture.frames[0], frame) {
		t.Fatalf("captures=%q want one original frame", capture.frames)
	}
}
