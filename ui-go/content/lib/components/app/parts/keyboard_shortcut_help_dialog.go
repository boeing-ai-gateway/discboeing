package parts

import "strings"

func shortcutKeyGroupID(keys []string) string {
	return strings.Join(keys, "+")
}
