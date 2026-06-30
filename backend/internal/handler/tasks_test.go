package handler

import (
	"project-manager/backend/internal/model"
	"testing"
)

func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		in   string
		want model.TaskStatus
	}{
		{in: "queued", want: model.TaskQueued},
		{in: "processing", want: model.TaskProcessing},
		{in: "reviewing", want: model.TaskReviewing},
		{in: "completed", want: model.TaskCompleted},
		{in: "unknown", want: model.TaskPending},
	}

	for _, tc := range cases {
		if got := normalizeStatus(tc.in); got != tc.want {
			t.Fatalf("input %s expect %s got %s", tc.in, tc.want, got)
		}
	}
}

func TestParseExplicitTaskStatus(t *testing.T) {
	cases := []struct {
		in     string
		want   model.TaskStatus
		wantOK bool
	}{
		{in: "pending", want: model.TaskPending, wantOK: true},
		{in: "queued", want: model.TaskQueued, wantOK: true},
		{in: "processing", want: model.TaskProcessing, wantOK: true},
		{in: "reviewing", want: model.TaskReviewing, wantOK: true},
		{in: "completed", want: model.TaskCompleted, wantOK: true},
		{in: " processing ", want: model.TaskProcessing, wantOK: true},
		{in: "unknown", want: model.TaskPending, wantOK: false},
		{in: "", want: model.TaskPending, wantOK: false},
	}

	for _, tc := range cases {
		got, ok := parseExplicitTaskStatus(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Fatalf("input %q expect (%s,%v) got (%s,%v)", tc.in, tc.want, tc.wantOK, got, ok)
		}
	}
}
