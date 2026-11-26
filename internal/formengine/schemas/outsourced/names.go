package outsourced

import (
	"sort"
	"strings"
)

// Names lists the schemas this package embeds, sorted, without the .json
// suffix. The Developer module's schema browser uses it.
func Names() []string {
	entries, _ := schemaFS.ReadDir(".")
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasSuffix(n, ".json") {
			out = append(out, strings.TrimSuffix(n, ".json"))
		}
	}
	sort.Strings(out)
	return out
}
