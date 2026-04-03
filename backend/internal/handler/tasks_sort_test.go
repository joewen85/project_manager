package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPrioritySortClause(t *testing.T) {
	tests := []struct {
		name   string
		order  string
		expect string
	}{
		{
			name:   "high first default",
			order:  "",
			expect: "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc",
		},
		{
			name:   "high first explicit",
			order:  "high",
			expect: "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc",
		},
		{
			name:   "medium first",
			order:  "medium",
			expect: "CASE tasks.priority WHEN 'medium' THEN 0 WHEN 'high' THEN 1 WHEN '' THEN 1 WHEN 'low' THEN 2 ELSE 1 END, tasks.created_at desc",
		},
		{
			name:   "low first",
			order:  "low",
			expect: "CASE tasks.priority WHEN 'low' THEN 0 WHEN 'medium' THEN 1 WHEN 'high' THEN 2 WHEN '' THEN 2 ELSE 2 END, tasks.created_at desc",
		},
		{
			name:   "asc compatibility",
			order:  "asc",
			expect: "CASE tasks.priority WHEN 'low' THEN 0 WHEN 'medium' THEN 1 WHEN 'high' THEN 2 WHEN '' THEN 2 ELSE 2 END, tasks.created_at desc",
		},
		{
			name:   "desc compatibility",
			order:  "desc",
			expect: "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := prioritySortClause(testCase.order)
			if got != testCase.expect {
				t.Fatalf("unexpected clause: %s", got)
			}
		})
	}
}

func TestParseTaskSortPriority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?sortBy=priority&sortOrder=high", nil)

	clause := parseTaskSort(ctx)
	expect := "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc"
	if clause != expect {
		t.Fatalf("unexpected sort clause: %s", clause)
	}
}

func TestStatusSortClause(t *testing.T) {
	tests := []struct {
		name   string
		order  string
		expect string
	}{
		{
			name:   "status asc",
			order:  "asc",
			expect: "CASE tasks.status WHEN 'pending' THEN 0 WHEN 'queued' THEN 1 WHEN 'processing' THEN 2 WHEN 'completed' THEN 3 ELSE 4 END, tasks.created_at desc",
		},
		{
			name:   "status desc",
			order:  "desc",
			expect: "CASE tasks.status WHEN 'completed' THEN 0 WHEN 'processing' THEN 1 WHEN 'queued' THEN 2 WHEN 'pending' THEN 3 ELSE 4 END, tasks.created_at desc",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := statusSortClause(testCase.order)
			if got != testCase.expect {
				t.Fatalf("unexpected clause: %s", got)
			}
		})
	}
}
