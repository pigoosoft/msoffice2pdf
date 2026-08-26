package queue

import (
	"runtime"
	"time"

	"log/slog"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/reslimit"
)

func (q *Queue) pressureLoop() {
	defer q.pressureWG.Done()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	q.applyPressure("start")
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-t.C:
			q.applyPressure("poll")
		}
	}
}

func (q *Queue) applyPressure(why string) {
	want, reason := q.desiredWorkers()
	if old, changed := q.slots.setCurrent(want); changed {
		if want < old {
			slog.Warn("converter: degrade concurrent converts",
				"from", old, "to", want, "reason", reason, "trigger", why)
			return
		}
		slog.Info("converter: restore concurrent converts",
			"from", old, "to", want, "reason", reason, "trigger", why)
	}
}

func (q *Queue) desiredWorkers() (n int, reason string) {
	max := q.cfg.WorkerCount
	min := q.cfg.MinWorkers
	if min < 1 {
		min = 1
	}
	if min > max {
		min = max
	}

	backlog := uint64(applog.BacklogBytes())
	backlogMax := uint64(q.cfg.LogBacklogMaxMB) * 1024 * 1024
	if backlogMax > 0 && backlog > backlogMax {
		return min, "log_backlog"
	}

	memLimit := uint64(q.cfg.MemLimitMB) * 1024 * 1024
	if memLimit == 0 {
		memLimit = reslimit.AutoMemLimitBytes()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if memLimit > 0 && ms.Alloc > memLimit {
		return min, "heap"
	}
	if _, avail, err := reslimit.SystemMemory(); err == nil && avail < 256*1024*1024 {
		return min, "ram"
	}

	diskMin := uint64(q.cfg.DiskMinFreeMB) * 1024 * 1024
	if diskMin > 0 {
		for _, dir := range q.watchDirs {
			free, err := reslimit.DiskFreeBytes(dir)
			if err != nil {
				continue
			}
			if free < diskMin {
				return min, "disk"
			}
		}
	}

	cur := q.slots.current()
	if cur < max {
		return cur + 1, "healthy"
	}
	return max, "ok"
}
