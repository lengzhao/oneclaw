package preturn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, skillsRoot, id, body string) {
	t.Helper()
	dir := filepath.Join(skillsRoot, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsDigest_unrestricted(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, skillsRoot, "alpha", "# A\n")
	writeSkill(t, skillsRoot, "beta", "# B\n")
	out, err := SkillsDigestMarkdown(skillsRoot, DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("want both skills: %q", out)
	}
}

func TestSkillsDigest_catalogRestrict(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, skillsRoot, "alpha", "# A\n")
	writeSkill(t, skillsRoot, "beta", "# B\n")
	out, err := SkillsDigestMarkdown(skillsRoot, DefaultBudget(), []string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Fatalf("want alpha only: %q", out)
	}
	if !strings.Contains(out, "catalog allowlist") {
		t.Fatalf("want allowlist header: %q", out)
	}
}

func TestSkillsDigest_frontmatterDescription(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: x\ndescription: Hello from YAML\n---\n\n# Ignored Heading\n"
	writeSkill(t, skillsRoot, "with-fm", body)
	writeSkill(t, skillsRoot, "no-fm", "# Title Only\n")
	out, err := SkillsDigestMarkdown(skillsRoot, DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- with-fm: Hello from YAML") {
		t.Fatalf("want YAML description in digest: %q", out)
	}
	if !strings.Contains(out, "- no-fm: Title Only") {
		t.Fatalf("want heading fallback: %q", out)
	}
}

func TestSkillsDigest_catalogRestrictMissing(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, skillsRoot, "alpha", "# A\n")
	out, err := SkillsDigestMarkdown(skillsRoot, DefaultBudget(), []string{"gamma"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not installed") {
		t.Fatalf("want hint: %q", out)
	}
}
