package handler

import "net/http"

import "github.com/gin-gonic/gin"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, apiError{Code: code, Message: message})
}

func respondDBError(c *gin.Context, status int, code string, err error) {
	if err == nil {
		respondError(c, status, code, "unknown database error")
		return
	}
	respondError(c, status, code, err.Error())
}

func respondMessage(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}

func respondValidationError(c *gin.Context, err error) {
	if err == nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request")
		return
	}
	respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
}
