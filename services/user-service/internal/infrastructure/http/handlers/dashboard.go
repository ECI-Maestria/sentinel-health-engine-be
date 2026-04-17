package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appuser "github.com/sentinel-health-engine/user-service/internal/application/user"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// DashboardPatientResponse is the API representation of a patient entry in the doctor dashboard.
type DashboardPatientResponse struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	FullName       string `json:"fullName"`
	IsActive       bool   `json:"isActive"`
	DeviceCount    int    `json:"deviceCount"`
	CaretakerCount int    `json:"caretakerCount"`
	CreatedAt      string `json:"createdAt"`
}

// DashboardHandler handles the doctor dashboard endpoint.
type DashboardHandler struct {
	getDashboard *appuser.GetDoctorDashboardUseCase
}

// NewDashboardHandler constructs a DashboardHandler.
func NewDashboardHandler(getDashboard *appuser.GetDoctorDashboardUseCase) *DashboardHandler {
	return &DashboardHandler{getDashboard: getDashboard}
}

// Register mounts dashboard routes. All require DOCTOR role.
func (h *DashboardHandler) Register(rg *gin.RouterGroup) {
	doctorRoutes := rg.Group("", middleware.RequireAuth(), middleware.RequireRole("DOCTOR"))
	doctorRoutes.GET("/doctor/dashboard", h.getDoctorDashboard)
}

// getDoctorDashboard handles GET /v1/doctor/dashboard
func (h *DashboardHandler) getDoctorDashboard(c *gin.Context) {
	patients, err := h.getDashboard.Execute(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]DashboardPatientResponse, 0, len(patients))
	for _, dp := range patients {
		response = append(response, DashboardPatientResponse{
			ID:             dp.User.ID(),
			Email:          dp.User.Email(),
			FirstName:      dp.User.FirstName(),
			LastName:       dp.User.LastName(),
			FullName:       dp.User.FullName(),
			IsActive:       dp.User.IsActive(),
			DeviceCount:    dp.DeviceCount,
			CaretakerCount: dp.CaretakerCount,
			CreatedAt:      dp.User.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"patients": response})
}
