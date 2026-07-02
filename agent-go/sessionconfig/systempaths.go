package sessionconfig

import "path/filepath"

var discboeingSystemRoots = []string{
	"/opt/discboeing",
	"/usr/local/share/discboeing",
	"/usr/share/discboeing",
}

func discboeingSystemPaths(rel string) []string {
	paths := make([]string, 0, len(discboeingSystemRoots))
	for _, root := range discboeingSystemRoots {
		paths = append(paths, filepath.Join(root, rel))
	}
	return paths
}
