package preturn

import "strings"

// MemoryTreeListingMarkdown returns a plain digest of memory/**/*.md (paths + sizes only). Empty when no tree.
func MemoryTreeListingMarkdown(instructionRoot string, budget Budget) string {
	budget = CoalesceBudget(budget)
	return strings.TrimSpace(memoryFolderTreeDigest(instructionRoot, budget.MemoryFolderMaxRunes))
}

// MemoryRecallSection returns markdown listing memory/**/*.md under instructionRoot (recall/discoverability).
// Empty string if there is no memory/ tree. Budget.MemoryFolderMaxRunes caps the listing size.
func MemoryRecallSection(instructionRoot string, budget Budget) string {
	tree := MemoryTreeListingMarkdown(instructionRoot, budget)
	if tree == "" {
		return ""
	}
	return "## Memory recall (instruction root)\n\n" + tree +
		"\n\n_Use `read_file` with paths under `memory/` (e.g. `memory/<yyyy-mm>/<file>.md` or shorthand `memory/<yyyy-mm-dd>.md`)._"
}
