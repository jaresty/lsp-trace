//go:build darwin || linux

package main

import (
	"encoding/json"
	inv "lsp-trace/internal/inventorysuccessor"
	"testing"
	"time"
)

func TestConnectedHOSTTimingStartBindingsV2(t *testing.T) {
	c := startConfigFixtureV2(t)
	p := inv.PrivateSourceTimingV2{Mode: inv.PrivateSourceHOST60V2, CaptureMS: 15000, PreparationMS: 15000, HOSTMS: 60000, PublicationCleanupMS: 5000}
	legacyPin := privateSourceConfigPin(c.Source)
	c.Source.Timing = &p
	c.Source.Host.Mailbox.TimeoutMS = 60000
	c.Source.Host.Mailbox.OverallTimeoutMS = 60000
	c.Pin = privateStartConfigPin(c)
	raw, _ := json.Marshal(c)
	if _, e := decodePrivateStartConfig(raw); e != nil {
		t.Fatal("ASSERT_HOST_PROFILE_TEMPLATE", e)
	}
	if privateSourceConfigPin(c.Source) == legacyPin {
		t.Fatal("ASSERT_HOST_PROFILE_PIN")
	}
	now := time.Now()
	got, e := bindPrivateSourceStart(c.Source, now, now.Add(95*time.Second))
	if e != nil || got.TimingFrame == nil || !got.Deadline.Equal(now.Add(95*time.Second)) || !got.TimingFrame.CaptureDeadline.Equal(now.Add(15*time.Second)) || got.Pin == "" {
		t.Fatal("ASSERT_HOST_TOTAL_BOUND_ONCE", e)
	}
	if _, e = bindPrivateSourceStart(got, now.Add(time.Second), now.Add(200*time.Second)); e == nil {
		t.Fatal("ASSERT_HOST_NO_RENEWAL")
	}
	if _, e = bindPrivateSourceStart(c.Source, now, now.Add(95*time.Second-time.Nanosecond)); e == nil {
		t.Fatal("ASSERT_HOST_FULL_TOTAL_BEFORE_ISSUANCE")
	}
	forged := got
	frame := *got.TimingFrame
	frame.TotalDeadline = frame.TotalDeadline.Add(time.Nanosecond)
	forged.TimingFrame = &frame
	forged.Pin = privateSourceConfigPin(forged)
	raw, _ = json.Marshal(forged)
	if _, e = decodePrivateSourceHostConfig(raw); e == nil {
		t.Fatal("ASSERT_HOST_FRAME_FORGERY")
	}
	c.Source.Host.Mailbox.TimeoutMS = 5000
	c.Pin = privateStartConfigPin(c)
	raw, _ = json.Marshal(c)
	if _, e = decodePrivateStartConfig(raw); e == nil {
		t.Fatal("ASSERT_HOST_INCOMPATIBLE_PROFILE")
	}
	t.Log("ASSERT_HOST_PROFILE_TEMPLATE PASS; ASSERT_HOST_TOTAL_BOUND_ONCE PASS; ASSERT_HOST_NO_RENEWAL PASS; source remains unissued")
}
