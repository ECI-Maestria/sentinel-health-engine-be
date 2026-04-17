package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
	"go.uber.org/zap"
)

// AlertsHandler handles HTTP requests for alert analytics.
type AlertsHandler struct {
	repo   *cosmosdb.AlertsRepository
	logger *zap.Logger
}

// NewAlertsHandler creates a new AlertsHandler.
func NewAlertsHandler(repo *cosmosdb.AlertsRepository, logger *zap.Logger) *AlertsHandler {
	return &AlertsHandler{repo: repo, logger: logger}
}

// GetHistory handles GET /v1/patients/:id/alerts/history
// Query params: from (optional), to (optional), severity (optional: WARNING|CRITICAL)
func (h *AlertsHandler) GetHistory(c *gin.Context) {
	patientID := c.Param("id")
	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	severity := c.Query("severity")

	alerts, err := h.repo.GetHistory(c.Request.Context(), patientID, from, to, severity)
	if err != nil {
		h.logger.Error("failed to get alerts history",
			zap.String("patientId", patientID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve alerts history"})
		return
	}

	if alerts == nil {
		alerts = []cosmosdb.AlertRecord{}
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

// GetStats handles GET /v1/patients/:id/alerts/stats
// Query params: from (optional), to (optional)
func (h *AlertsHandler) GetStats(c *gin.Context) {
	patientID := c.Param("id")
	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, err := h.repo.GetStats(c.Request.Context(), patientID, from, to)
	if err != nil {
		h.logger.Error("failed to get alert stats",
			zap.String("patientId", patientID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve alert stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Acknowledge handles PATCH /v1/patients/:id/alerts/:alertId/acknowledge
func (h *AlertsHandler) Acknowledge(c *gin.Context) {
	patientID := c.Param("id")
	alertID := c.Param("alertId")

	if err := h.repo.AcknowledgeAlert(c.Request.Context(), patientID, alertID); err != nil {
		h.logger.Error("failed to acknowledge alert",
			zap.String("patientId", patientID),
			zap.String("alertId", alertID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acknowledge alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}
