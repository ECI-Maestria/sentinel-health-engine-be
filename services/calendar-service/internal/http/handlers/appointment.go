// Package handlers contains Gin HTTP handlers for the calendar-service.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sentinel-health-engine/calendar-service/internal/domain"
	"github.com/sentinel-health-engine/calendar-service/internal/http/middleware"
	"github.com/sentinel-health-engine/calendar-service/internal/postgres"
)

// AppointmentHandler handles HTTP requests for appointments.
type AppointmentHandler struct {
	repo   *postgres.AppointmentRepository
	logger *zap.Logger
}

// NewAppointmentHandler creates a new AppointmentHandler.
func NewAppointmentHandler(repo *postgres.AppointmentRepository, logger *zap.Logger) *AppointmentHandler {
	return &AppointmentHandler{repo: repo, logger: logger}
}

// appointmentResponse is the JSON shape returned for a single appointment.
type appointmentResponse struct {
	ID             string  `json:"id"`
	PatientID      string  `json:"patientId"`
	DoctorID       string  `json:"doctorId"`
	Title          string  `json:"title"`
	ScheduledAt    string  `json:"scheduledAt"`
	Location       string  `json:"location"`
	Notes          string  `json:"notes"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

func toAppointmentResponse(a *domain.Appointment) appointmentResponse {
	return appointmentResponse{
		ID:          a.ID,
		PatientID:   a.PatientID,
		DoctorID:    a.DoctorID,
		Title:       a.Title,
		ScheduledAt: a.ScheduledAt.UTC().Format(time.RFC3339),
		Location:    a.Location,
		Notes:       a.Notes,
		Status:      string(a.Status),
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// Create handles POST /v1/patients/:id/appointments
func (h *AppointmentHandler) Create(c *gin.Context) {
	patientID := c.Param("id")
	doctorID := middleware.CurrentUserID(c)

	var req struct {
		Title       string `json:"title"       binding:"required"`
		ScheduledAt string `json:"scheduledAt" binding:"required"`
		Location    string `json:"location"`
		Notes       string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scheduledAt must be RFC3339"})
		return
	}

	appt, err := domain.NewAppointment(patientID, doctorID, req.Title, scheduledAt, req.Location, req.Notes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Save(c.Request.Context(), appt); err != nil {
		h.logger.Error("create appointment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create appointment"})
		return
	}

	c.JSON(http.StatusCreated, toAppointmentResponse(appt))
}

// List handles GET /v1/patients/:id/appointments
// Optional query params:
//
//	?period=day|week|month|year  — filter by time period
//	?date=YYYY-MM-DD             — reference date for the period (default: today)
func (h *AppointmentHandler) List(c *gin.Context) {
	patientID := c.Param("id")

	from, to, err := periodBounds(c.Query("period"), c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	appointments, err := h.repo.ListByPatient(c.Request.Context(), patientID)
	if err != nil {
		h.logger.Error("list appointments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list appointments"})
		return
	}

	resp := make([]appointmentResponse, 0, len(appointments))
	for _, a := range appointments {
		if inRange(a.ScheduledAt.UTC(), from, to) {
			resp = append(resp, toAppointmentResponse(a))
		}
	}

	c.JSON(http.StatusOK, gin.H{"appointments": resp})
}

// GetByID handles GET /v1/patients/:id/appointments/:apptId
func (h *AppointmentHandler) GetByID(c *gin.Context) {
	apptID := c.Param("apptId")

	appt, err := h.repo.FindByID(c.Request.Context(), apptID)
	if err != nil {
		h.logger.Error("get appointment by id", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve appointment"})
		return
	}
	if appt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "appointment not found"})
		return
	}

	c.JSON(http.StatusOK, toAppointmentResponse(appt))
}

// Update handles PUT /v1/patients/:id/appointments/:apptId
func (h *AppointmentHandler) Update(c *gin.Context) {
	apptID := c.Param("apptId")

	appt, err := h.repo.FindByID(c.Request.Context(), apptID)
	if err != nil {
		h.logger.Error("find appointment for update", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve appointment"})
		return
	}
	if appt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "appointment not found"})
		return
	}

	var req struct {
		Title       *string `json:"title"`
		ScheduledAt *string `json:"scheduledAt"`
		Location    *string `json:"location"`
		Notes       *string `json:"notes"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != nil {
		appt.Title = *req.Title
	}
	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scheduledAt must be RFC3339"})
			return
		}
		appt.ScheduledAt = t.UTC()
	}
	if req.Location != nil {
		appt.Location = *req.Location
	}
	if req.Notes != nil {
		appt.Notes = *req.Notes
	}
	if req.Status != nil {
		appt.Status = domain.AppointmentStatus(*req.Status)
	}

	if err := h.repo.Update(c.Request.Context(), appt); err != nil {
		h.logger.Error("update appointment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update appointment"})
		return
	}

	c.JSON(http.StatusOK, toAppointmentResponse(appt))
}

// Delete handles DELETE /v1/patients/:id/appointments/:apptId
func (h *AppointmentHandler) Delete(c *gin.Context) {
	apptID := c.Param("apptId")

	appt, err := h.repo.FindByID(c.Request.Context(), apptID)
	if err != nil {
		h.logger.Error("find appointment for delete", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve appointment"})
		return
	}
	if appt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "appointment not found"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), apptID); err != nil {
		h.logger.Error("delete appointment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete appointment"})
		return
	}

	c.Status(http.StatusNoContent)
}
