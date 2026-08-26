package metrics

import (
	"testing"
	"time"

	"msoffice2pdf/internal/domain"
)

func TestParseRange(t *testing.T) {
	if _, ok := ParseRange(""); ok {
		t.Fatal("empty")
	}
	if _, ok := ParseRange("2h"); ok {
		t.Fatal("2h")
	}
	s, ok := ParseRange("1h")
	if !ok || s.BucketLabel != "raw" || s.Bucket != 0 {
		t.Fatalf("%#v", s)
	}
	s, ok = ParseRange("24h")
	if !ok || s.Bucket != time.Minute {
		t.Fatalf("%#v", s)
	}
	s, ok = ParseRange("7d")
	if !ok || s.Bucket != 5*time.Minute {
		t.Fatalf("%#v", s)
	}
}

func TestAggregateMinuteMaxKeepsSpike(t *testing.T) {
	base := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	samples := []domain.PressureSample{
		{SampledAt: base, Queued: 1, DiskFreeMin: 100},
		{SampledAt: base.Add(10 * time.Second), Queued: 50, DegradeReason: "disk", DiskFreeMin: 10},
		{SampledAt: base.Add(20 * time.Second), Queued: 3, DiskFreeMin: 90},
		{SampledAt: base.Add(time.Minute), Queued: 2, DiskFreeMin: 80},
	}
	out := Aggregate(samples, time.Minute)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Queued != 50 || out[0].DegradeReason != "disk" || out[0].DiskFreeMin != 100 {
		t.Fatalf("bucket0 %#v", out[0])
	}
	if !out[0].SampledAt.Equal(base) {
		t.Fatalf("start %v", out[0].SampledAt)
	}
	if out[1].Queued != 2 {
		t.Fatalf("bucket1 %#v", out[1])
	}
}

func TestAggregateFiveMinuteMax(t *testing.T) {
	base := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	samples := []domain.PressureSample{
		{SampledAt: base.Add(time.Minute), WorkersCur: 1},
		{SampledAt: base.Add(3 * time.Minute), WorkersCur: 4},
		{SampledAt: base.Add(6 * time.Minute), WorkersCur: 2},
	}
	out := Aggregate(samples, 5*time.Minute)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].WorkersCur != 4 || !out[0].SampledAt.Equal(base) {
		t.Fatalf("%#v", out[0])
	}
	if out[1].WorkersCur != 2 || !out[1].SampledAt.Equal(base.Add(5*time.Minute)) {
		t.Fatalf("%#v", out[1])
	}
}

func TestMergeLoadPeakKeepsConvertingSpikeAndDiskDip(t *testing.T) {
	acc := domain.PressureSample{SampledAt: time.Unix(1, 0).UTC(), Converting: 0, WorkersCur: 0, DiskFreeMin: 9_000, RamAvail: 8_000}
	mergeLoadPeak(&acc, domain.PressureSample{SampledAt: time.Unix(2, 0).UTC(), Converting: 4, WorkersCur: 4, DiskFreeMin: 1_000, RamAvail: 2_000, DegradeReason: "disk"})
	mergeLoadPeak(&acc, domain.PressureSample{SampledAt: time.Unix(3, 0).UTC(), Converting: 1, WorkersCur: 1, DiskFreeMin: 8_000, RamAvail: 7_000})
	if acc.Converting != 4 || acc.WorkersCur != 4 {
		t.Fatalf("peak %#v", acc)
	}
	if acc.DiskFreeMin != 1_000 || acc.RamAvail != 2_000 || acc.DegradeReason != "disk" {
		t.Fatalf("worst %#v", acc)
	}
	if !acc.SampledAt.Equal(time.Unix(3, 0).UTC()) {
		t.Fatalf("sampled_at %v", acc.SampledAt)
	}
}

func TestAggregateRawUnchanged(t *testing.T) {
	samples := []domain.PressureSample{{Pending: 1}, {Pending: 2}}
	out := Aggregate(samples, 0)
	if len(out) != 2 || out[1].Pending != 2 {
		t.Fatalf("%#v", out)
	}
}
