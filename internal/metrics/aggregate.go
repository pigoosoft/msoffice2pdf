package metrics

import (
	"time"

	"msoffice2pdf/internal/domain"
)

type RangeSpec struct {
	Name        string
	Window      time.Duration
	Bucket      time.Duration // 0 = raw points
	BucketLabel string
}

func ParseRange(s string) (RangeSpec, bool) {
	switch s {
	case "1h":
		return RangeSpec{Name: "1h", Window: time.Hour, Bucket: 0, BucketLabel: "raw"}, true
	case "24h":
		return RangeSpec{Name: "24h", Window: 24 * time.Hour, Bucket: time.Minute, BucketLabel: "1m"}, true
	case "7d":
		return RangeSpec{Name: "7d", Window: 7 * 24 * time.Hour, Bucket: 5 * time.Minute, BucketLabel: "5m"}, true
	default:
		return RangeSpec{}, false
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func minPositiveUint64(a, b uint64) uint64 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// mergeLoadPeak keeps queue/worker/log/heap peaks and the worst ram/disk in a write window.
func mergeLoadPeak(dst *domain.PressureSample, src domain.PressureSample) {
	dst.Pending = maxInt64(dst.Pending, src.Pending)
	dst.Queued = maxInt64(dst.Queued, src.Queued)
	dst.Converting = maxInt64(dst.Converting, src.Converting)
	dst.Failed = maxInt64(dst.Failed, src.Failed)
	dst.ChannelLen = maxInt(dst.ChannelLen, src.ChannelLen)
	dst.WorkersCur = maxInt(dst.WorkersCur, src.WorkersCur)
	dst.WorkersMax = maxInt(dst.WorkersMax, src.WorkersMax)
	dst.WorkersMin = maxInt(dst.WorkersMin, src.WorkersMin)
	dst.LogBacklogBytes = maxInt64(dst.LogBacklogBytes, src.LogBacklogBytes)
	dst.HeapAlloc = maxUint64(dst.HeapAlloc, src.HeapAlloc)
	dst.RamAvail = minPositiveUint64(dst.RamAvail, src.RamAvail)
	dst.DiskFreeMin = minPositiveUint64(dst.DiskFreeMin, src.DiskFreeMin)
	if src.DegradeReason != "" {
		dst.DegradeReason = src.DegradeReason
	}
	if src.SampledAt.After(dst.SampledAt) {
		dst.SampledAt = src.SampledAt
	}
}

func mergeMax(dst *domain.PressureSample, src domain.PressureSample) {
	dst.Pending = maxInt64(dst.Pending, src.Pending)
	dst.Queued = maxInt64(dst.Queued, src.Queued)
	dst.Converting = maxInt64(dst.Converting, src.Converting)
	dst.Failed = maxInt64(dst.Failed, src.Failed)
	dst.ChannelLen = maxInt(dst.ChannelLen, src.ChannelLen)
	dst.WorkersCur = maxInt(dst.WorkersCur, src.WorkersCur)
	dst.WorkersMax = maxInt(dst.WorkersMax, src.WorkersMax)
	dst.WorkersMin = maxInt(dst.WorkersMin, src.WorkersMin)
	dst.LogBacklogBytes = maxInt64(dst.LogBacklogBytes, src.LogBacklogBytes)
	dst.HeapAlloc = maxUint64(dst.HeapAlloc, src.HeapAlloc)
	dst.RamAvail = maxUint64(dst.RamAvail, src.RamAvail)
	dst.DiskFreeMin = maxUint64(dst.DiskFreeMin, src.DiskFreeMin)
	if src.DegradeReason != "" {
		dst.DegradeReason = src.DegradeReason
	}
}

// Aggregate buckets samples (must be sorted by SampledAt ascending) using per-field max.
// DegradeReason is the last non-empty reason in the bucket. Bucket==0 returns samples unchanged.
func Aggregate(samples []domain.PressureSample, bucket time.Duration) []domain.PressureSample {
	if bucket <= 0 || len(samples) == 0 {
		return samples
	}
	type acc struct {
		out domain.PressureSample
		n   int
	}
	order := make([]time.Time, 0)
	m := map[int64]*acc{}
	for _, s := range samples {
		t := s.SampledAt.UTC().Truncate(bucket)
		key := t.UnixNano()
		a, ok := m[key]
		if !ok {
			a = &acc{out: domain.PressureSample{SampledAt: t}}
			m[key] = a
			order = append(order, t)
		}
		if a.n == 0 {
			a.out.Pending = s.Pending
			a.out.Queued = s.Queued
			a.out.Converting = s.Converting
			a.out.Failed = s.Failed
			a.out.ChannelLen = s.ChannelLen
			a.out.WorkersCur = s.WorkersCur
			a.out.WorkersMax = s.WorkersMax
			a.out.WorkersMin = s.WorkersMin
			a.out.LogBacklogBytes = s.LogBacklogBytes
			a.out.HeapAlloc = s.HeapAlloc
			a.out.RamAvail = s.RamAvail
			a.out.DiskFreeMin = s.DiskFreeMin
			a.out.DegradeReason = s.DegradeReason
		} else {
			mergeMax(&a.out, s)
		}
		a.n++
	}
	out := make([]domain.PressureSample, 0, len(order))
	for _, t := range order {
		out = append(out, m[t.UnixNano()].out)
	}
	return out
}
