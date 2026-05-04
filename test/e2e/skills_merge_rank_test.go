package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/skillsusage"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// 「相似性合并」在本仓库中的对应行为：
// 1) catalog 解析 agent frontmatter 时对 skills: 列表去重（dedupeSkillIDs），保留首次出现顺序；
// 2) skills digest 通过 _usage.jsonl 聚合用量，RankSkillIDs 在高用量并列时按最近使用时间、再按 id 字典序排序（热度合并视图）。

func appendSkillUsageEvent(t *testing.T, skillsRoot, skillID string, unixSec int64) {
	t.Helper()
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillsRoot, skillsusage.LogFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ev := skillsusage.Event{TimeUnix: unixSec, SkillID: skillID, Action: "e2e"}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ev); err != nil {
		t.Fatal(err)
	}
}

func mkdirSkillSKILLmd(t *testing.T, skillsRoot, skillID, summary string) {
	t.Helper()
	dir := filepath.Join(skillsRoot, skillID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + skillID + "\ndescription: " + summary + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_skills_catalog_frontmatter_dedupesPreserveFirstSeenOrder(t *testing.T) {
	raw := []byte(`---
name: Dedup agent
skills:
  - alpha-e2e
  - beta-e2e
  - alpha-e2e
  - gamma-e2e
---
Body.
`)
	ag, err := catalog.ParseAgentMarkdown("dedup-agent", raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha-e2e", "beta-e2e", "gamma-e2e"}
	if len(ag.ReferencedSkillIDs) != len(want) {
		t.Fatalf("ReferencedSkillIDs=%v want %v", ag.ReferencedSkillIDs, want)
	}
	for i := range want {
		if ag.ReferencedSkillIDs[i] != want[i] {
			t.Fatalf("index %d: got %q want %q", i, ag.ReferencedSkillIDs[i], want[i])
		}
	}
}

func TestE2E_skills_digest_hotRanking_prefersHigherUsageCount(t *testing.T) {
	tmp := t.TempDir()
	skillsRoot := filepath.Join(tmp, "skills")
	mkdirSkillSKILLmd(t, skillsRoot, "hot-low", "L")
	mkdirSkillSKILLmd(t, skillsRoot, "hot-high", "H")
	if err := skillsusage.Record(skillsRoot, "hot-low", "probe"); err != nil {
		t.Fatal(err)
	}
	if err := skillsusage.Record(skillsRoot, "hot-high", "probe"); err != nil {
		t.Fatal(err)
	}
	if err := skillsusage.Record(skillsRoot, "hot-high", "probe"); err != nil {
		t.Fatal(err)
	}

	digest, err := preturn.SkillsDigestMarkdown(skillsRoot, preturn.DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ih := strings.Index(digest, "- hot-high:")
	il := strings.Index(digest, "- hot-low:")
	if ih < 0 || il < 0 || ih >= il {
		t.Fatalf("expected hot-high before hot-low in digest:\n%s", digest)
	}
}

func TestE2E_skills_digest_tieBreakLatestUsageThenLexicographicID(t *testing.T) {
	tmp := t.TempDir()
	skillsRoot := filepath.Join(tmp, "skills")
	mkdirSkillSKILLmd(t, skillsRoot, "tie-new", "N")
	mkdirSkillSKILLmd(t, skillsRoot, "tie-old", "O")
	// 各 2 次命中；last_used 更大者应排前（秒级时间戳显式写入 _usage.jsonl）
	appendSkillUsageEvent(t, skillsRoot, "tie-new", 1000)
	appendSkillUsageEvent(t, skillsRoot, "tie-old", 1000)
	appendSkillUsageEvent(t, skillsRoot, "tie-old", 2000)
	appendSkillUsageEvent(t, skillsRoot, "tie-new", 3000)

	digest, err := preturn.SkillsDigestMarkdown(skillsRoot, preturn.DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	in := strings.Index(digest, "- tie-new:")
	io := strings.Index(digest, "- tie-old:")
	if in < 0 || io < 0 || in >= io {
		t.Fatalf("expected tie-new (last=3000) before tie-old (last=2000):\n%s", digest)
	}

	// 同 count、同 last_used → id 字典序（RankSkillIDs 单元语义）
	counts := map[string]int64{"z-id": 1, "a-id": 1}
	last := map[string]int64{"z-id": 99, "a-id": 99}
	ranked := skillsusage.RankSkillIDs(counts, last, []string{"z-id", "a-id"})
	if len(ranked) != 2 || ranked[0] != "a-id" || ranked[1] != "z-id" {
		t.Fatalf("tie-break id asc: got %v", ranked)
	}
}

func TestE2E_skills_digest_allowlist_hidesUnreferencedSkills(t *testing.T) {
	tmp := t.TempDir()
	skillsRoot := filepath.Join(tmp, "skills")
	mkdirSkillSKILLmd(t, skillsRoot, "allowed-only", "x")
	mkdirSkillSKILLmd(t, skillsRoot, "secret-other", "y")

	digest, err := preturn.SkillsDigestMarkdown(skillsRoot, preturn.DefaultBudget(), []string{"allowed-only"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(digest, "allowed-only") {
		t.Fatalf("digest:\n%s", digest)
	}
	if strings.Contains(digest, "secret-other") {
		t.Fatalf("allowlist digest must not list secret-other:\n%s", digest)
	}
}

func TestE2E_skills_createViaTool_surfacesInDigestAndUsage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	skillsRoot := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	tool, err := builtin.InferWriteSkillFile(root)
	if err != nil {
		t.Fatal(err)
	}
	skillID := "e2e-created-skill"
	args, err := json.Marshal(map[string]string{
		"path":    "skills/" + skillID + "/SKILL.md",
		"content": "---\nname: E2E Created\ndescription: created by test\n---\nBody.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(ctx, string(args)); err != nil {
		t.Fatal(err)
	}

	digest, err := preturn.SkillsDigestMarkdown(skillsRoot, preturn.DefaultBudget(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(digest, skillID) {
		t.Fatalf("digest should list new skill:\n%s", digest)
	}

	counts, _, err := skillsusage.Aggregate(skillsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if counts[skillID] < 1 {
		t.Fatalf("usage log should record writes for %q: %#v", skillID, counts)
	}
}
