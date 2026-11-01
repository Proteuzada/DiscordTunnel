package watch

import (
	"os"
	"path/filepath"
	"strings"
)

func RunningPIDs(prefixes []string, excludeNames []string) map[uint32]string {
	out := make(map[uint32]string)
	// Enumeration via tasklist is coarse; redirector proc_watcher handles live PIDs.
	// This helper is reserved for future snapshots.
	return out
}

func NormalizePrefixes(prefixes []string) []string {
	var out []string
	for _, p := range prefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasSuffix(p, string(os.PathSeparator)) {
			p += string(os.PathSeparator)
		}
		out = append(out, filepath.Clean(p)+string(os.PathSeparator))
	}
	return out
}
