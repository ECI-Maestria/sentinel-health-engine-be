package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appdoctor "github.com/sentinel-health-engine/user-service/internal/application/doctor"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/middleware"
)

// DoctorHandler handles doctor management endpoints (Doctor-only).
type DoctorHandler struct {
	create *appdoctor.CreateDoctorUseCase
	list   *appdoctor.ListDoctorsUseCase
}

func NewDoctorHandler(create *appdoctor.CreateDoctorUseCase, list *appdoctor.ListDoctorsUseCase) *DoctorHandler {
	return &DoctorHandler{create: create, list: list}
}

// Register mounts doctor routes. All require DOCTOR role.
func (h *DoctorHandler) Register(rg *gin.RouterGroup) {
	doctorRoutes := rg.Group("", middleware.RequireAuth(), middleware.RequireRole("DOCTOR"))
	doctorRoutes.POST("/doctors", h.createDoctor)
	doctorRoutes.GET("/doctors", h.listDoctors)
}

// createDoctor handles POST /v1/doctors
func (h *DoctorHandler) createDoctor(c *gin.Context) {
	var body struct {
		FirstName string `json:"firstName" binding:"required"`
		LastName  string `json:"lastName"  binding:"required"`
		Email     string `json:"email"     binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.create.Execute(c.Request.Context(), appdoctor.CreateDoctorCommand{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Email:     body.Email,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(result.Doctor))
}

// listDoctors handles GET /v1/doctors
func (h *DoctorHandler) listDoctors(c *gin.Context) {
	doctors, err := h.list.Execute(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := make([]UserResponse, 0, len(doctors))
	for _, d := range doctors {
		response = append(response, toUserResponse(d))
	}
	c.JSON(http.StatusOK, gin.H{"doctors": response})
}
