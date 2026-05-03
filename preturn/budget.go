package preturn

// Budget caps injected text slices (FR-FLOW-03 baseline).
type Budget struct {
	// MemoryMaxRunes caps content injected from lengzhao/memory (SQLite) recall when wired; not applied to MEMORY.md file snapshot.
	MemoryMaxRunes int
	// MemoryFolderMaxRunes caps the memory/ tree listing injected for memory_extractor.
	MemoryFolderMaxRunes int
	SkillsMaxRunes       int
}

// DefaultBudget returns conservative defaults when zero means unlimited logic elsewhere.
func DefaultBudget() Budget {
	return Budget{
		MemoryMaxRunes:       8000,
		MemoryFolderMaxRunes: 6000,
		SkillsMaxRunes:       4000,
	}
}

// CoalesceBudget fills zero or negative fields from [DefaultBudget].
func CoalesceBudget(b Budget) Budget {
	d := DefaultBudget()
	if b.MemoryMaxRunes <= 0 {
		b.MemoryMaxRunes = d.MemoryMaxRunes
	}
	if b.MemoryFolderMaxRunes <= 0 {
		b.MemoryFolderMaxRunes = d.MemoryFolderMaxRunes
	}
	if b.SkillsMaxRunes <= 0 {
		b.SkillsMaxRunes = d.SkillsMaxRunes
	}
	return b
}
