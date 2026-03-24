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
		{in: "completed", want: model.TaskCompleted},
		{in: "unknown", want: model.TaskPending},
	}

	for _, tc := range cases {
		if got := normalizeStatus(tc.in); got != tc.want {
			t.Fatalf("input %s expect %s got %s", tc.in, tc.want, got)
		}
	}
}
