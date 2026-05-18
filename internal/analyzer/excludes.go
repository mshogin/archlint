package analyzer

// MatchesExclude reports whether dirName matches any entry in exclude.
// Match is exact basename comparison (no glob expansion).
func MatchesExclude(dirName string, exclude []string) bool {
	for _, e := range exclude {
		if e == dirName {
			return true
		}
	}
	return false
}
