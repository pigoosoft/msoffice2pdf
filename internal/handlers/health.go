package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"msoffice2pdf/internal/db"
)

type HealthHandler struct {
	DB *gorm.DB
}

func (h *HealthHandler) Health(c *gin.Context) {
	if err := db.Ping(h.DB); err != nil {
		JSON(c, http.StatusServiceUnavailable, 50301, "database unavailable", gin.H{
			"status": "degraded",
			"db":     "down",
		})
		return
	}
	OK(c, gin.H{"status": "ok", "db": "up"})
}
