package pathutil

// ResolveForSession resolves a tool path against cwd, optionally allowing memory roots.
func ResolveForSession(cwd string, memoryRoots []string, userPath string) (string, error) {
	if len(memoryRoots) == 0 {
		return ResolveUnderRoot(cwd, userPath)
	}
	return ResolveUnderAllowedRoots(cwd, memoryRoots, userPath)
}
