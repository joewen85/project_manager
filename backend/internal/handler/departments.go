package handler

import (
	"net/http"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type departmentRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	UserIDs     []uint `json:"userIds"`
}

func (h *Handler) ListDepartments(c *gin.Context) {
	var items []model.Department
	if err := h.DB.Preload("Users").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	item := model.Department{Name: req.Name, Description: req.Description}
	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if len(req.UserIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
		h.DB.Model(&item).Association("Users").Replace(&users)
	}
	h.DB.Preload("Users").First(&item, item.ID)
	c.JSON(http.StatusCreated, item)
}
