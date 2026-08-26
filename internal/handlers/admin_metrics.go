package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/metrics"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
)

type AdminMetricsHandler struct {
	Queue   *queue.Queue
	Uploads *repo.UploadRepo
	Samples *repo.PressureSampleRepo
	Conv    config.ConverterConfig
}

type metricsLimitsView struct {
	MemLimitBytes      uint64 `json:"mem_limit_bytes"`
	DiskMinFreeBytes   uint64 `json:"disk_min_free_bytes"`
	LogBacklogMaxBytes uint64 `json:"log_backlog_max_bytes"`
}

type metricsSampleView struct {
	SampledAt       time.Time `json:"sampled_at"`
	Pending         int64     `json:"pending"`
	Queued          int64     `json:"queued"`
	Converting      int64     `json:"converting"`
	Failed          int64     `json:"failed"`
	ChannelLen      int       `json:"channel_len"`
	WorkersCur      int       `json:"workers_cur"`
	WorkersMax      int       `json:"workers_max"`
	WorkersMin      int       `json:"workers_min"`
	LogBacklogBytes int64     `json:"log_backlog_bytes"`
	HeapAlloc       uint64    `json:"heap_alloc"`
	RamAvail        uint64    `json:"ram_avail"`
	DiskFreeMin     uint64    `json:"disk_free_min"`
	DegradeReason   string    `json:"degrade_reason"`
}

type metricsCurrentView struct {
	metricsSampleView
	Limits metricsLimitsView `json:"limits"`
}

func toSampleView(s domain.PressureSample) metricsSampleView {
	return metricsSampleView{
		SampledAt:       s.SampledAt,
		Pending:         s.Pending,
		Queued:          s.Queued,
		Converting:      s.Converting,
		Failed:          s.Failed,
		ChannelLen:      s.ChannelLen,
		WorkersCur:      s.WorkersCur,
		WorkersMax:      s.WorkersMax,
		WorkersMin:      s.WorkersMin,
		LogBacklogBytes: s.LogBacklogBytes,
		HeapAlloc:       s.HeapAlloc,
		RamAvail:        s.RamAvail,
		DiskFreeMin:     s.DiskFreeMin,
		DegradeReason:   s.DegradeReason,
	}
}

func (h *AdminMetricsHandler) Current(c *gin.Context) {
	s := metrics.Collect(time.Now(), h.Queue, h.Uploads)
	lim := metrics.LimitsFrom(h.Conv)
	OK(c, metricsCurrentView{
		metricsSampleView: toSampleView(s),
		Limits: metricsLimitsView{
			MemLimitBytes:      lim.MemLimitBytes,
			DiskMinFreeBytes:   lim.DiskMinFreeBytes,
			LogBacklogMaxBytes: lim.LogBacklogMaxBytes,
		},
	})
}

func (h *AdminMetricsHandler) History(c *gin.Context) {
	spec, ok := metrics.ParseRange(c.Query("range"))
	if !ok {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid range")
		return
	}
	if h.Samples == nil {
		OK(c, gin.H{"range": spec.Name, "bucket": spec.BucketLabel, "points": []metricsSampleView{}})
		return
	}
	since := time.Now().Add(-spec.Window)
	rows, err := h.Samples.ListSince(since)
	if err != nil {
		Fail(c, http.StatusInternalServerError, CodeInternal, "metrics history failed")
		return
	}
	rows = metrics.Aggregate(rows, spec.Bucket)
	points := make([]metricsSampleView, 0, len(rows))
	for _, r := range rows {
		points = append(points, toSampleView(r))
	}
	OK(c, gin.H{"range": spec.Name, "bucket": spec.BucketLabel, "points": points})
}
