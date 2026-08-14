package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/service"
)

func (h *PdfHandler) Events(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	// Accept repeated ?fileid=a&fileid=b and/or comma-separated ?fileid=a,b,c
	raw := c.QueryArray("fileid")
	fileIDs := expandFileIDs(raw)
	if len(fileIDs) == 0 {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "fileid required")
		return
	}
	maxN := h.SSE.MaxFileIDs
	if maxN <= 0 {
		maxN = 50
	}
	if len(fileIDs) > maxN {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "too many fileid")
		return
	}

	// Preflight: every fileid must resolve and be accessible (no half-open stream).
	snapshots := make([]map[string]interface{}, 0, len(fileIDs))
	for _, id := range fileIDs {
		data, err := h.Svc.Status(user, id)
		if err != nil {
			if mapServiceError(c, err) {
				return
			}
			Fail(c, http.StatusInternalServerError, 50001, "status failed")
			return
		}
		snapshots = append(snapshots, data)
	}

	// Bypass http.Server.WriteTimeout for this long-lived response.
	rc := http.NewResponseController(c.Writer)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("sse: clear write deadline failed", "err", err)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}

	lastFP := make(map[string]string, len(fileIDs))
	allTerminal := true
	for i, id := range fileIDs {
		if err := writeSSE(c, "status", snapshots[i]); err != nil {
			return
		}
		lastFP[id] = service.StatusFingerprint(snapshots[i])
		if !service.IsTerminalStatusMap(snapshots[i]) {
			allTerminal = false
		}
	}
	if allTerminal {
		_ = writeSSE(c, "done", map[string]interface{}{
			"reason":  "all_terminal",
			"fileids": fileIDs,
		})
		return
	}

	maxDur := h.SSE.MaxDuration
	if maxDur <= 0 {
		maxDur = 5 * time.Minute
	}
	pollEvery := h.SSE.PollInterval
	if pollEvery <= 0 {
		pollEvery = time.Second
	}
	hbEvery := h.SSE.HeartbeatInterval
	if hbEvery <= 0 {
		hbEvery = 15 * time.Second
	}

	deadline := time.Now().Add(maxDur)
	pollTick := time.NewTicker(pollEvery)
	hbTick := time.NewTicker(hbEvery)
	defer pollTick.Stop()
	defer hbTick.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-hbTick.C:
			if err := writeSSE(c, "ping", map[string]interface{}{}); err != nil {
				return
			}
		case <-pollTick.C:
			pending := make([]string, 0)
			allTerminal := true
			for _, id := range fileIDs {
				data, err := h.Svc.Status(user, id)
				if err != nil {
					slog.Warn("sse: status poll failed", "fileid", id, "err", err)
					allTerminal = false
					pending = append(pending, id)
					continue
				}
				fp := service.StatusFingerprint(data)
				if fp != lastFP[id] {
					if err := writeSSE(c, "status", data); err != nil {
						return
					}
					lastFP[id] = fp
				}
				if !service.IsTerminalStatusMap(data) {
					allTerminal = false
					pending = append(pending, id)
				}
			}
			if allTerminal {
				_ = writeSSE(c, "done", map[string]interface{}{
					"reason":  "all_terminal",
					"fileids": fileIDs,
				})
				return
			}
			if time.Now().After(deadline) {
				_ = writeSSE(c, "done", map[string]interface{}{
					"reason":  "max_duration",
					"fileids": fileIDs,
					"pending": pending,
				})
				return
			}
		}
	}
}

// expandFileIDs splits each query value on commas, trims space, and dedupes.
func expandFileIDs(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		for _, part := range strings.Split(s, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func writeSSE(c *gin.Context, event string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
