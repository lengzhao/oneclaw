package preturn

// MemoryRecallSection returns markdown listing memory/**/*.md under instructionRoot (recall/discoverability).
// Empty string if there is no memory/ tree. Budget.MemoryFolderMaxRunes caps the listing size.
func MemoryRecallSection(instructionRoot string, budget Budget) string {
	budget = CoalesceBudget(budget)
	tree := memoryFolderTreeDigest(instructionRoot, budget.MemoryFolderMaxRunes)
	if tree == "" {
		return ""
	}
	return "## Memory recall (instruction root)\n\n" + tree +
		"\n\n_To read these files use tool `read_memory_month` with path `memory/<yyyy-mm>/<file>.md` (instruction root). `read_file` only sees the workspace._"
}
