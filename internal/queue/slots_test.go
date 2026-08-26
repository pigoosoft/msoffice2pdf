package queue

import (
	"testing"
	"time"
)

func TestSlotLimiterThirdWaitsUntilRelease(t *testing.T) {
	s := newSlotLimiter(2, 1)
	s.acquire()
	s.acquire()
	done := make(chan struct{})
	go func() {
		s.acquire()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("third acquire must wait")
	case <-time.After(30 * time.Millisecond):
	}
	s.release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("release did not unblock")
	}
}

func TestSlotLimiterSnapshotInflight(t *testing.T) {
	s := newSlotLimiter(4, 1)
	inflight, max, min := s.snapshot()
	if inflight != 0 || max != 4 || min != 1 {
		t.Fatalf("idle snapshot %d %d %d", inflight, max, min)
	}
	s.acquire()
	s.acquire()
	inflight, max, min = s.snapshot()
	if inflight != 2 || max != 4 || min != 1 {
		t.Fatalf("busy snapshot %d %d %d", inflight, max, min)
	}
	s.release()
	inflight, _, _ = s.snapshot()
	if inflight != 1 {
		t.Fatalf("after release %d", inflight)
	}
}

func TestSlotLimiterSetCurrentBroadcasts(t *testing.T) {
	s := newSlotLimiter(4, 1)
	if old, ok := s.setCurrent(1); !ok || old != 4 {
		t.Fatalf("old=%d ok=%v", old, ok)
	}
	if s.current() != 1 {
		t.Fatal(s.current())
	}
}

func TestSlotLimiterStopUnblocks(t *testing.T) {
	s := newSlotLimiter(1, 1)
	s.acquire()
	done := make(chan struct{})
	go func() {
		if s.acquire() {
			t.Error("want false after stop")
		}
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	s.wakeAll()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not unblock")
	}
}
