package handler

import "github.com/gin-gonic/gin"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, apiError{Code: code, Message: message})
}

func respondMessage(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}
