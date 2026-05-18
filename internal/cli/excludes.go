package cli

// mergeExcludes concatenates exclude entries from config and CLI flags,
// deduplicating exact-match basenames. Order is preserved: config first,
// then CLI flags.
func mergeExcludes(fromConfig, fromCLI []string) []string {
	if len(fromConfig) == 0 && len(fromCLI) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(fromConfig)+len(fromCLI))
	out := make([]string, 0, len(fromConfig)+len(fromCLI))
	for _, src := range [][]string{fromConfig, fromCLI} {
		for _, e := range src {
			if e == "" {
				continue
			}
			if _, ok := seen[e]; ok {
				continue
			}
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}
