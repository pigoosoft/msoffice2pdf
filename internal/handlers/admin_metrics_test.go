package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminMetricsHistoryInvalidRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AdminMetricsHandler{}
	r := gin.New()
	r.GET("/history", h.History)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/history?range=2h", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
	var env Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != CodeBadRequest {
		t.Fatalf("code %d", env.Code)
	}
}
