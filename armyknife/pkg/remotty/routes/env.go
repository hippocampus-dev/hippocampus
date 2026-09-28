package routes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// POSIX.1-2017 XBD §3.235 Name: https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap03.html#tag_03_235
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Env(patterns []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := map[string]struct{}{}
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if !envNamePattern.MatchString(name) {
				continue
			}
			for _, pattern := range patterns {
				matched, err := path.Match(pattern, name)
				if err != nil {
					continue
				}
				if matched {
					names[name] = struct{}{}
					break
				}
			}
		}

		sorted := make([]string, 0, len(names))
		for name := range names {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		for _, name := range sorted {
			value, ok := os.LookupEnv(name)
			if !ok {
				continue
			}
			_, _ = io.WriteString(w, fmt.Sprintf("export %s='%s'\n", name, strings.ReplaceAll(value, "'", `'\''`)))
		}
	}
}
