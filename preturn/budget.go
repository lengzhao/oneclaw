package preturn

// Budget caps injected file slices (FR-FLOW-03 baseline).
type Budget struct {
	MemoryMaxRunes int
	SkillsMaxRunes int
}

// DefaultBudget returns conservative defaults when zero means unlimited logic elsewhere.
func DefaultBudget() Budget {
	return Budget{
		MemoryMaxRunes: 8000,
		SkillsMaxRunes: 4000,
	}
}
