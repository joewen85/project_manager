package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type pageResult[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func parsePage(c *gin.Context) (page int, pageSize int) {
	page = 1
	pageSize = 10

	if value := c.Query("page"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if value := c.Query("pageSize"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}

	return page, pageSize
}

func parseRFC3339(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseSort(c *gin.Context, defaultClause string, allowed map[string]string) string {
	sortBy := strings.TrimSpace(c.Query("sortBy"))
	column, ok := allowed[sortBy]
	if !ok {
		return defaultClause
	}

	order := strings.ToLower(strings.TrimSpace(c.Query("sortOrder")))
	if order != "asc" {
		order = "desc"
	}
	return column + " " + order
}
