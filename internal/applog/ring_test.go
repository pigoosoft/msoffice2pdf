package applog

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRingCapacityDropsOldest(t *testing.T) {
	r := NewRing(3)
	h := r.Handler(nil)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
		rec.AddAttrs(slog.String("action", "a"), slog.Int("n", i))
		_ = h.Handle(ctx, rec)
	}
	all := r.Snapshot(slog.LevelDebug, "", "")
	if len(all) != 3 {
		t.Fatalf("len=%d want 3", len(all))
	}
}

func TestRingFilterLevelUIDAction(t *testing.T) {
	r := NewRing(100)
	h := r.Handler(nil)
	ctx := context.Background()
	mk := func(level slog.Level, uid, action, msg string) {
		rec := slog.NewRecord(time.Now(), level, msg, 0)
		rec.AddAttrs(slog.String("uid", uid), slog.String("action", action))
		_ = h.Handle(ctx, rec)
	}
	mk(slog.LevelInfo, "u1", "upload", "ok")
	mk(slog.LevelError, "u2", "convert", "fail")
	mk(slog.LevelWarn, "u1", "convert", "slow")

	got := r.Snapshot(slog.LevelWarn, "u1", "convert")
	if len(got) != 1 || got[0].Action != "convert" || got[0].UID != "u1" {
		t.Fatalf("got=%+v", got)
	}
}

func TestRingSetCapacityShrinks(t *testing.T) {
	r := NewRing(1000)
	h := r.Handler(nil)
	for i := 0; i < 50; i++ {
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "x", 0)
		_ = h.Handle(context.Background(), rec)
	}
	r.SetCapacity(10)
	if r.Capacity() != 10 {
		t.Fatalf("cap=%d", r.Capacity())
	}
	if n := len(r.Snapshot(slog.LevelDebug, "", "")); n != 10 {
		t.Fatalf("len=%d", n)
	}
}
