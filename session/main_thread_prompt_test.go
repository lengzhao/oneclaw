package session

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/prompts"
)

func TestMainThreadSystemRender(t *testing.T) {
	got, err := prompts.Render(prompts.NameMainThreadSystem, MainThreadSystemData{
		CWD:                   "/tmp/proj",
		Platform:              "darwin",
		Shell:                 "/bin/zsh",
		TasksFilePath:         "/tmp/proj/tasks.json",
		TaskLines:             []string{"- **[in_progress]** `ts_x` — item"},
		TasksOmitted:          0,
		MemoryPromptBlock:     "## File-based memory\n\nstub",
		SkillLines:            []string{"- pdf: extract PDFs"},
		AppendedSystemContext: "Extra line.",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		"# Intro",
		"# System",
		"# Rules precedence",
		"# agentMd",
		"# Doing tasks",
		"## Task list (persisted)",
		"tasks.json",
		"- **[in_progress]**",
		"# Actions",
		"# Memory",
		"## File-based memory",
		"Primary working directory: /tmp/proj",
		"# Skills and discovery",
		"## Skills",
		"invoke_skill",
		"- pdf:",
		"# Additional system context",
		"Extra line.",
	} {
		if !strings.Contains(got, sub) {
			t.Fatalf("main_thread_system missing %q\n---\n%s", sub, got)
		}
	}
}

func TestMainThreadOmitsTaskBlockWhenNoTasks(t *testing.T) {
	got, err := prompts.Render(prompts.NameMainThreadSystem, MainThreadSystemData{
		CWD: "/p", Platform: "linux", Shell: "bash",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Task list (persisted)") {
		t.Fatalf("expected no task list section, got:\n%s", got)
	}
}

func TestMainThreadOmitsSkillsWhenNoLines(t *testing.T) {
	got, err := prompts.Render(prompts.NameMainThreadSystem, MainThreadSystemData{
		CWD: "/p", Platform: "linux", Shell: "bash",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Skills and discovery") {
		t.Fatalf("expected no skills section, got:\n%s", got)
	}
}
