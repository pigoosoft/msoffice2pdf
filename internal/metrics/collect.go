package metrics

import (
	"runtime"
	"time"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/reslimit"
)

type Limits struct {
	MemLimitBytes      uint64 `json:"mem_limit_bytes"`
	DiskMinFreeBytes   uint64 `json:"disk_min_free_bytes"`
	LogBacklogMaxBytes uint64 `json:"log_backlog_max_bytes"`
}

func LimitsFrom(cfg config.ConverterConfig) Limits {
	mem := uint64(cfg.MemLimitMB) * 1024 * 1024
	if cfg.MemLimitMB == 0 {
		mem = reslimit.AutoMemLimitBytes()
	}
	return Limits{
		MemLimitBytes:      mem,
		DiskMinFreeBytes:   uint64(cfg.DiskMinFreeMB) * 1024 * 1024,
		LogBacklogMaxBytes: uint64(cfg.LogBacklogMaxMB) * 1024 * 1024,
	}
}

func countStatus(uploads *repo.UploadRepo, status string) int64 {
	if uploads == nil {
		return 0
	}
	n, err := uploads.CountByStatus(status)
	if err != nil {
		return 0
	}
	return n
}

func minDiskFree(dirs []string) uint64 {
	var min uint64
	var any bool
	for _, dir := range dirs {
		free, err := reslimit.DiskFreeBytes(dir)
		if err != nil {
			continue
		}
		if !any || free < min {
			min = free
			any = true
		}
	}
	if !any {
		return 0
	}
	return min
}

// Collect takes one snapshot. Partial probe failures leave the corresponding field at 0.
func Collect(now time.Time, q *queue.Queue, uploads *repo.UploadRepo) domain.PressureSample {
	s := domain.PressureSample{SampledAt: now.UTC()}
	s.Pending = countStatus(uploads, domain.UploadStatusPending)
	s.Queued = countStatus(uploads, domain.UploadStatusQueued)
	s.Converting = countStatus(uploads, domain.UploadStatusConverting)
	s.Failed = countStatus(uploads, domain.UploadStatusFailed)
	if q != nil {
		s.ChannelLen = q.ChannelLen()
		s.WorkersCur, s.WorkersMax, s.WorkersMin = q.SlotSnapshot()
		s.DegradeReason = q.ResourceTripReason()
		s.DiskFreeMin = minDiskFree(q.WatchDirs())
	}
	s.LogBacklogBytes = applog.BacklogBytes()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.HeapAlloc = ms.Alloc
	if _, avail, err := reslimit.SystemMemory(); err == nil {
		s.RamAvail = avail
	}
	return s
}
