package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"project-manager/backend/internal/ai"
)

type fakeStreamingAIClient struct {
	deltas []string
}

func (f fakeStreamingAIClient) Chat(context.Context, []ai.Message) (string, error) {
	return strings.Join(f.deltas, ""), nil
}

func (f fakeStreamingAIClient) ChatStream(_ context.Context, _ []ai.Message, onDelta func(string) error) (string, error) {
	for _, delta := range f.deltas {
		if err := onDelta(delta); err != nil {
			return "", err
		}
	}
	return strings.Join(f.deltas, ""), nil
}

func TestAIParseSuggestedTasksValid(t *testing.T) {
	sources := []aiSourceRef{{Type: "project", ID: 1, Label: "P1"}}
	raw := `{"tasks":[
		{"title":"需求确认","description":"梳理范围","priority":"HIGH","isMilestone":false,"relativeStartDay":0,"durationDays":2},
		{"title":"交付执行","description":"推进核心任务","priority":"weird","isMilestone":true,"relativeStartDay":-3,"durationDays":0}
	]}`
	tasks, ok := aiParseSuggestedTasks(raw, sources)
	if !ok {
		t.Fatal("expected ok=true for valid payload")
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Priority != "high" {
		t.Errorf("expected priority normalised to high, got %q", tasks[0].Priority)
	}
	if tasks[1].Priority != "medium" {
		t.Errorf("expected invalid priority coerced to medium, got %q", tasks[1].Priority)
	}
	if tasks[1].RelativeStartDay != 0 {
		t.Errorf("expected negative start clamped to 0, got %d", tasks[1].RelativeStartDay)
	}
	if tasks[1].DurationDays != 1 {
		t.Errorf("expected non-positive duration coerced to 1, got %d", tasks[1].DurationDays)
	}
	if len(tasks[0].SourceRefs) != 1 {
		t.Errorf("expected sources attached, got %d", len(tasks[0].SourceRefs))
	}
}

func TestAIParseSuggestedTasksStripsCodeFence(t *testing.T) {
	raw := "```json\n{\"tasks\":[{\"title\":\"计划\",\"priority\":\"low\",\"durationDays\":3}]}\n```"
	tasks, ok := aiParseSuggestedTasks(raw, nil)
	if !ok || len(tasks) != 1 {
		t.Fatalf("expected 1 task from fenced JSON, ok=%v len=%d", ok, len(tasks))
	}
	if tasks[0].Title != "计划" {
		t.Errorf("unexpected title %q", tasks[0].Title)
	}
}

func TestAIParseSuggestedTasksRejectsGarbage(t *testing.T) {
	cases := []string{"", "not json", `{"tasks":[]}`, `{"tasks":[{"title":"  "}]}`}
	for _, raw := range cases {
		if _, ok := aiParseSuggestedTasks(raw, nil); ok {
			t.Errorf("expected ok=false for %q", raw)
		}
	}
}

func TestAIComposeNarrativeStreamResultEmitsDeltas(t *testing.T) {
	h := &Handler{AIClient: fakeStreamingAIClient{deltas: []string{"周", "报", "正文"}}}
	var got []string
	out, used := h.aiComposeNarrativeStreamResult(context.Background(), "system", "context", "fallback", func(delta string) error {
		got = append(got, delta)
		return nil
	})
	if !used {
		t.Fatal("expected model to be used")
	}
	if out != "周报正文" {
		t.Fatalf("unexpected output %q", out)
	}
	if strings.Join(got, "") != "周报正文" || len(got) != 3 {
		t.Fatalf("unexpected deltas: %#v", got)
	}
}

func TestLoadAIPromptsDefaultsWhenNoDir(t *testing.T) {
	set := loadAIPrompts("")
	if set.weeklyReport != defaultAIWeeklyReportSystemPrompt ||
		set.riskSummary != defaultAIRiskSummarySystemPrompt ||
		set.taskBreakdown != defaultAITaskBreakdownSystemPrompt {
		t.Fatal("expected built-in defaults when no prompt dir is given")
	}
}

func TestLoadAIPromptsFileOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, aiWeeklyReportPromptFile), []byte("  custom weekly  "), 0o644); err != nil {
		t.Fatal(err)
	}
	set := loadAIPrompts(dir)
	if set.weeklyReport != "custom weekly" {
		t.Errorf("expected trimmed file override, got %q", set.weeklyReport)
	}
	// Untouched files keep the built-in default.
	if set.riskSummary != defaultAIRiskSummarySystemPrompt {
		t.Errorf("expected default risk summary when file absent")
	}
}

func TestResolveAIPromptDirPrefersConfigured(t *testing.T) {
	if got := resolveAIPromptDir("  /custom/path  "); got != "  /custom/path  " {
		t.Errorf("expected configured path returned verbatim, got %q", got)
	}
}
