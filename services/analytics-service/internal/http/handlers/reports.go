package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
	"github.com/sentinel-health-engine/analytics-service/internal/pdf"
	"go.uber.org/zap"
)

// reportRequest is the JSON body for the report generation endpoint.
type reportRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ReportsHandler handles PDF report generation.
type ReportsHandler struct {
	vitalsRepo *cosmosdb.VitalsRepository
	alertsRepo *cosmosdb.AlertsRepository
	logger     *zap.Logger
}

// NewReportsHandler creates a new ReportsHandler.
func NewReportsHandler(
	vitalsRepo *cosmosdb.VitalsRepository,
	alertsRepo *cosmosdb.AlertsRepository,
	logger *zap.Logger,
) *ReportsHandler {
	return &ReportsHandler{
		vitalsRepo: vitalsRepo,
		alertsRepo: alertsRepo,
		logger:     logger,
	}
}

// Generate handles POST /v1/patients/:id/reports/generate
// Body: {"from":"2024-01-01","to":"2024-01-31"}
// Role restriction (DOCTOR or CARETAKER) is enforced by the route middleware.
func (h *ReportsHandler) Generate(c *gin.Context) {
	patientID := c.Param("id")

	var req reportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Default time range: last 30 days
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if req.From != "" {
		parsed, err := parseFlexibleDate(req.From)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid 'from' date: %v", err)})
			return
		}
		from = parsed
	}
	if req.To != "" {
		parsed, err := parseFlexibleDate(req.To)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid 'to' date: %v", err)})
			return
		}
		to = parsed
	}

	ctx := c.Request.Context()

	vitals, err := h.vitalsRepo.GetHistory(ctx, patientID, from, to)
	if err != nil {
		h.logger.Error("failed to fetch vitals for report",
			zap.String("patientId", patientID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve vitals"})
		return
	}

	alerts, err := h.alertsRepo.GetHistory(ctx, patientID, from, to, "")
	if err != nil {
		h.logger.Error("failed to fetch alerts for report",
			zap.String("patientId", patientID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve alerts"})
		return
	}

	// Use patientID as the name placeholder; a richer implementation could look up
	// the actual patient name from a user service.
	patientName := fmt.Sprintf("Patient %s", patientID)

	pdfBytes, err := pdf.GeneratePatientReport(patientName, from, to, vitals, alerts)
	if err != nil {
		h.logger.Error("failed to generate PDF report",
			zap.String("patientId", patientID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	filename := fmt.Sprintf("report-%s-%s.pdf", patientID, time.Now().UTC().Format("20060102"))

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
