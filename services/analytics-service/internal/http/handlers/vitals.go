package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
	"go.uber.org/zap"
)

// VitalsHandler handles HTTP requests for vital signs analytics.
type VitalsHandler struct {
	repo   *cosmosdb.VitalsRepository
	logger *zap.Logger
}

// NewVitalsHandler creates a new VitalsHandler.
func NewVitalsHandler(repo *cosmosdb.VitalsRepository, logger *zap.Logger) *VitalsHandler {
	return &VitalsHandler{repo: repo, logger: logger}
}

// GetHistory handles GET /v1/patients/:id/vitals/history
// Query params: from (optional), to (optional)
func (h *VitalsHandler) GetHistory(c *gin.Context) {
	patientID := c.Param("id")
	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	readings, err := h.repo.GetHistory(c.Request.Context(), patientID, from, to)
	if err != nil {
		h.logger.Error("failed to get vitals history", zap.String("patientId", patientID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve vitals history"})
		return
	}

	if readings == nil {
		readings = []cosmosdb.VitalReading{}
	}

	c.JSON(http.StatusOK, gin.H{"readings": readings})
}

// GetLatest handles GET /v1/patients/:id/vitals/latest
func (h *VitalsHandler) GetLatest(c *gin.Context) {
	patientID := c.Param("id")

	reading, err := h.repo.GetLatest(c.Request.Context(), patientID)
	if err != nil {
		h.logger.Error("failed to get latest vital", zap.String("patientId", patientID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve latest vital"})
		return
	}

	if reading == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no vitals found for patient"})
		return
	}

	c.JSON(http.StatusOK, reading)
}

// GetSummary handles GET /v1/patients/:id/vitals/summary
// Query params: from (optional), to (optional)
func (h *VitalsHandler) GetSummary(c *gin.Context) {
	patientID := c.Param("id")
	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	readings, err := h.repo.GetHistory(c.Request.Context(), patientID, from, to)
	if err != nil {
		h.logger.Error("failed to get vitals for summary", zap.String("patientId", patientID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve vitals"})
		return
	}

	if len(readings) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"heartRate": gin.H{"min": 0, "max": 0, "avg": 0},
			"spO2":      gin.H{"min": 0, "max": 0, "avg": 0},
			"count":     0,
		})
		return
	}

	// Compute in-memory statistics
	hrMin := readings[0].HeartRate
	hrMax := readings[0].HeartRate
	hrSum := 0

	spo2Min := readings[0].SpO2
	spo2Max := readings[0].SpO2
	spo2Sum := 0.0

	for _, r := range readings {
		if r.HeartRate < hrMin {
			hrMin = r.HeartRate
		}
		if r.HeartRate > hrMax {
			hrMax = r.HeartRate
		}
		hrSum += r.HeartRate

		if r.SpO2 < spo2Min {
			spo2Min = r.SpO2
		}
		if r.SpO2 > spo2Max {
			spo2Max = r.SpO2
		}
		spo2Sum += r.SpO2
	}

	n := len(readings)
	hrAvg := float64(hrSum) / float64(n)
	spo2Avg := spo2Sum / float64(n)

	c.JSON(http.StatusOK, gin.H{
		"heartRate": gin.H{
			"min": hrMin,
			"max": hrMax,
			"avg": round2(hrAvg),
		},
		"spO2": gin.H{
			"min": spo2Min,
			"max": spo2Max,
			"avg": round2(spo2Avg),
		},
		"count": n,
	})
}

// parseDateRange reads "from" and "to" query parameters. Each can be a date
// (2006-01-02) or RFC3339 timestamp. Defaults: from = 30 days ago, to = now.
func parseDateRange(c *gin.Context) (from, to time.Time, err error) {
	now := time.Now().UTC()
	from = now.AddDate(0, 0, -30)
	to = now

	if f := c.Query("from"); f != "" {
		from, err = parseFlexibleDate(f)
		if err != nil {
			err = fmt.Errorf("invalid 'from' parameter: %w", err)
			return
		}
	}
	if t := c.Query("to"); t != "" {
		to, err = parseFlexibleDate(t)
		if err != nil {
			err = fmt.Errorf("invalid 'to' parameter: %w", err)
			return
		}
	}
	return
}

// parseFlexibleDate accepts either a date (2006-01-02) or an RFC3339 timestamp.
func parseFlexibleDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q: use YYYY-MM-DD or RFC3339", s)
}

// round2 rounds a float64 to 2 decimal places.
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
